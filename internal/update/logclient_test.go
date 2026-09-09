package update

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qdm12/ddns-updater/internal/update/mock_update"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_LogClient(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		requestMethod      string
		requetsHeaders     http.Header
		requestBodyNil     bool
		requestBodyString  string
		requestLineRegex   string
		responseStatusCode int
		responseBodyNil    bool
		responseBodyString string
		responseLineRegex  string
	}{
		"PUT with headers and body": {
			requestMethod: http.MethodPut,
			requetsHeaders: http.Header{
				"Key1": []string{"value 1", "value 2"},
				"Key2": []string{"value 3"},
			},
			requestBodyString: "request body",
			requestLineRegex: "PUT http://127.0.0.1:[0-9]{0,5} | " +
				"headers: Key1: value 1,value 2; Key2: value 3 | " +
				"body: \\[REDACTED\\]",
			responseStatusCode: http.StatusAccepted,
			responseBodyString: "response body",
			responseLineRegex: "202 Accepted | " +
				"headers: Content-Length: 13; Content-Type: text/plain; charset=utf-8; Date: .+ | " +
				"body: \\[REDACTED\\]",
		},
		"simple GET": {
			requestMethod:      http.MethodGet,
			requestBodyNil:     true,
			requestLineRegex:   "GET http://127.0.0.1:[0-9]{0,5}",
			responseStatusCode: http.StatusOK,
			responseBodyString: "response body",
			responseLineRegex: "200 OK | " +
				"headers: Content-Length: 13; Content-Type: text/plain; charset=utf-8; Date: .+ | " +
				"body: \\[REDACTED\\]",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			handler := http.HandlerFunc(func(rw http.ResponseWriter, request *http.Request) {
				assert.Equal(t, testCase.requestMethod, request.Method)
				for key, expectedValues := range testCase.requetsHeaders {
					values := request.Header[key]
					assert.Equal(t, expectedValues, values)
				}

				if testCase.requestBodyNil {
					require.Equal(t, http.NoBody, request.Body)
				} else {
					require.NotNil(t, request.Body)
					b, err := io.ReadAll(request.Body)
					require.NoError(t, err)
					assert.Equal(t, testCase.requestBodyString, string(b))
				}

				rw.WriteHeader(testCase.responseStatusCode)
				if testCase.responseBodyNil {
					return
				}
				_, err := rw.Write([]byte(testCase.responseBodyString))
				require.NoError(t, err)
			})
			server := httptest.NewServer(handler)
			defer server.Close()

			client := server.Client()

			logger := mock_update.NewMockDebugLogger(ctrl)
			logger.EXPECT().Debug(gomock.AssignableToTypeOf("")).
				DoAndReturn(func(s string) {
					assert.Regexp(t, testCase.requestLineRegex, s)
				})
			logger.EXPECT().Debug(gomock.AssignableToTypeOf("")).
				DoAndReturn(func(s string) {
					assert.Regexp(t, testCase.responseLineRegex, s)
				})

			logClient := makeLogClient(client, logger)
			assert.Same(t, logClient, client)

			ctx := context.Background()
			var requestBody io.Reader
			if !testCase.requestBodyNil {
				requestBody = bytes.NewBufferString(testCase.requestBodyString)
			}
			request, err := http.NewRequestWithContext(ctx,
				testCase.requestMethod, server.URL, requestBody)
			require.NoError(t, err)
			request.Header = testCase.requetsHeaders

			response, err := logClient.Do(request)
			require.NoError(t, err)
			defer require.NoError(t, response.Body.Close())

			assert.Equal(t, testCase.responseStatusCode, response.StatusCode)
			if testCase.responseBodyNil {
				assert.Nil(t, response.Body)
				return
			}
			b, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			assert.Equal(t, testCase.responseBodyString, string(b))
		})
	}
}

func TestRequestToStringRedactsSecrets(t *testing.T) {
	t.Parallel()

	request, err := http.NewRequest(http.MethodPost,
		"https://user:password@example.com/path?token=secret-token&api_key=secret-key&safe=value",
		strings.NewReader("sensitive request body"))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer super-secret")
	request.Header.Set("Cookie", "session=session-secret")
	request.Header.Set("X-Api-Key", "header-secret")

	line := requestToString(request)

	for _, secret := range []string{
		"user", "password", "secret-token", "secret-key",
		"sensitive request body", "super-secret", "session-secret", "header-secret",
	} {
		assert.NotContains(t, line, secret)
	}
	assert.Contains(t, line, "safe=value")
	assert.Contains(t, line, redactedValue)
}

func TestResponseToStringRedactsSecrets(t *testing.T) {
	t.Parallel()

	response := &http.Response{
		Status: "200 OK",
		Header: http.Header{
			"Set-Cookie": []string{"session=response-secret"},
			"X-Test":     []string{"safe"},
		},
		Body: io.NopCloser(strings.NewReader("sensitive response body")),
	}

	line := responseToString(response)
	assert.NotContains(t, line, "response-secret")
	assert.NotContains(t, line, "sensitive response body")
	assert.Contains(t, line, "X-Test: safe")
	assert.Contains(t, line, redactedValue)
}
