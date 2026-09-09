package hetznercloud

import (
	"context"
	"net/http"
	"net/netip"
	"strconv"
	"testing"

	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateReportsHetznerCloudTransientAPIErrors(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		statusCode int
		code       string
		message    string
	}{
		"rate limited": {
			statusCode: http.StatusTooManyRequests,
			code:       "rate_limit_exceeded",
			message:    "too many requests",
		},
		"server error": {
			statusCode: http.StatusInternalServerError,
			code:       "server_error",
			message:    "temporary failure",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(testCase.statusCode)
				_, _ = w.Write([]byte(`{"error":{"code":"` + testCase.code + `","message":"` + testCase.message + `"}}`))
			})

			provider := &Provider{
				domain:    "example.com",
				owner:     "home",
				ipVersion: ipversion.IP4,
				token:     "token",
			}

			_, err := provider.Update(context.Background(), newProviderTestClient(t, handler),
				netip.MustParseAddr("203.0.113.2"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), strconv.Itoa(testCase.statusCode))
			assert.Contains(t, err.Error(), testCase.message)
		})
	}
}
