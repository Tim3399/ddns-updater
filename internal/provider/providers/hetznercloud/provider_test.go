package hetznercloud

import (
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Provider_Update(t *testing.T) {
	t.Parallel()

	// exchange is an HTTP request expected by the test and its response.
	type exchange struct {
		method         string
		path           string
		requestBody    string
		responseStatus int
		responseBody   string
	}

	const actionSuccess = `{"action":{"id":42,"status":"success"}}`

	testCases := map[string]struct {
		ipVersion  ipversion.IPVersion
		ttl        uint32
		ip         netip.Addr
		exchanges  []exchange
		errMessage string
	}{
		"ipv4_rrset_updated": {
			ipVersion: ipversion.IP4,
			ip:        netip.MustParseAddr("203.0.113.2"),
			exchanges: []exchange{{
				method:         http.MethodGet,
				path:           "/v1/zones/example.com/rrsets/home/A",
				responseStatus: http.StatusOK,
				responseBody:   `{"rrset":{"records":[{"value":"203.0.113.1"}]}}`,
			}, {
				method:         http.MethodPost,
				path:           "/v1/zones/example.com/rrsets/home/A/actions/set_records",
				requestBody:    `{"records":[{"value":"203.0.113.2"}]}`,
				responseStatus: http.StatusCreated,
				responseBody:   actionSuccess,
			}},
		},
		"ipv6_rrset_updated": {
			ipVersion: ipversion.IP6,
			ip:        netip.MustParseAddr("2001:db8::2"),
			exchanges: []exchange{{
				method:         http.MethodGet,
				path:           "/v1/zones/example.com/rrsets/home/AAAA",
				responseStatus: http.StatusOK,
				responseBody:   `{"rrset":{"records":[{"value":"2001:db8::1"}]}}`,
			}, {
				method:         http.MethodPost,
				path:           "/v1/zones/example.com/rrsets/home/AAAA/actions/set_records",
				requestBody:    `{"records":[{"value":"2001:db8::2"}]}`,
				responseStatus: http.StatusCreated,
				responseBody:   actionSuccess,
			}},
		},
		"ipv4_rrset_created": {
			ipVersion: ipversion.IP4,
			ttl:       60,
			ip:        netip.MustParseAddr("203.0.113.2"),
			exchanges: []exchange{{
				method:         http.MethodGet,
				path:           "/v1/zones/example.com/rrsets/home/A",
				responseStatus: http.StatusNotFound,
			}, {
				method:         http.MethodPost,
				path:           "/v1/zones/example.com/rrsets",
				requestBody:    `{"name":"home","type":"A","ttl":60,"records":[{"value":"203.0.113.2"}]}`,
				responseStatus: http.StatusCreated,
				responseBody:   actionSuccess,
			}},
		},
		"ipv6_rrset_created": {
			ipVersion: ipversion.IP6,
			ttl:       60,
			ip:        netip.MustParseAddr("2001:db8::2"),
			exchanges: []exchange{{
				method:         http.MethodGet,
				path:           "/v1/zones/example.com/rrsets/home/AAAA",
				responseStatus: http.StatusNotFound,
			}, {
				method:         http.MethodPost,
				path:           "/v1/zones/example.com/rrsets",
				requestBody:    `{"name":"home","type":"AAAA","ttl":60,"records":[{"value":"2001:db8::2"}]}`,
				responseStatus: http.StatusCreated,
				responseBody:   actionSuccess,
			}},
		},
		"rrset_up_to_date": {
			ipVersion: ipversion.IP4,
			ip:        netip.MustParseAddr("203.0.113.2"),
			exchanges: []exchange{{
				method:         http.MethodGet,
				path:           "/v1/zones/example.com/rrsets/home/A",
				responseStatus: http.StatusOK,
				responseBody:   `{"rrset":{"records":[{"value":"203.0.113.2"}]}}`,
			}},
		},
		// An RRSet with several values is left as is when one of them is the IP address.
		"rrset_with_other_values_up_to_date": {
			ipVersion: ipversion.IP4,
			ip:        netip.MustParseAddr("203.0.113.2"),
			exchanges: []exchange{{
				method:         http.MethodGet,
				path:           "/v1/zones/example.com/rrsets/home/A",
				responseStatus: http.StatusOK,
				responseBody:   `{"rrset":{"records":[{"value":"192.0.2.10"},{"value":"203.0.113.2"}]}}`,
			}},
		},
		"unauthorized": {
			ipVersion: ipversion.IP4,
			ip:        netip.MustParseAddr("203.0.113.2"),
			exchanges: []exchange{{
				method:         http.MethodGet,
				path:           "/v1/zones/example.com/rrsets/home/A",
				responseStatus: http.StatusUnauthorized,
				responseBody:   `{"error":{"code":"unauthorized","message":"invalid token"}}`,
			}},
			errMessage: "getting record id: HTTP status is not valid: 401",
		},
		"rate_limited": {
			ipVersion: ipversion.IP4,
			ip:        netip.MustParseAddr("203.0.113.2"),
			exchanges: []exchange{{
				method:         http.MethodGet,
				path:           "/v1/zones/example.com/rrsets/home/A",
				responseStatus: http.StatusTooManyRequests,
				responseBody:   `{"error":{"code":"rate_limit_exceeded","message":"too many requests"}}`,
			}},
			errMessage: "getting record id: HTTP status is not valid: 429",
		},
		"server_error": {
			ipVersion: ipversion.IP4,
			ip:        netip.MustParseAddr("203.0.113.2"),
			exchanges: []exchange{{
				method:         http.MethodGet,
				path:           "/v1/zones/example.com/rrsets/home/A",
				responseStatus: http.StatusInternalServerError,
				responseBody:   `{"error":{"code":"server_error","message":"temporary failure"}}`,
			}},
			errMessage: "getting record id: HTTP status is not valid: 500",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			exchangeIndex := 0
			client := &http.Client{
				Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					if exchangeIndex >= len(testCase.exchanges) {
						return nil, errors.New("unexpected request")
					}
					exchange := testCase.exchanges[exchangeIndex]
					exchangeIndex++

					assert.Equal(t, exchange.method, request.Method)
					assert.Equal(t, "https://api.hetzner.cloud"+exchange.path, request.URL.String())
					assert.Equal(t, "Bearer token", request.Header.Get("Authorization"))
					if exchange.requestBody != "" {
						requestBody, err := io.ReadAll(request.Body)
						require.NoError(t, err)
						assert.JSONEq(t, exchange.requestBody, string(requestBody))
					}

					return &http.Response{
						StatusCode: exchange.responseStatus,
						Body:       io.NopCloser(strings.NewReader(exchange.responseBody)),
					}, nil
				}),
			}
			provider := &Provider{
				domain:    "example.com",
				owner:     "home",
				ipVersion: testCase.ipVersion,
				token:     "token",
				ttl:       testCase.ttl,
			}

			newIP, err := provider.Update(t.Context(), client, testCase.ip)

			assert.Equal(t, len(testCase.exchanges), exchangeIndex)
			if testCase.errMessage != "" {
				assert.ErrorContains(t, err, testCase.errMessage)
				assert.False(t, newIP.IsValid())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, testCase.ip, newIP)
		})
	}
}
