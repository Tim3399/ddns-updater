package hetznercloud

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_handleErrorResponse(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		statusCode int
		body       string
		errMessage string
	}{
		"error_with_code_and_message": {
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"code":"unauthorized","message":"invalid token"}}`,
			errMessage: "HTTP status is not valid: 401: code: unauthorized: message: invalid token",
		},
		"error_with_details": {
			statusCode: http.StatusUnprocessableEntity,
			body:       `{"error":{"code":"invalid_input","message":"invalid input","details":"ttl is too low"}}`,
			errMessage: "HTTP status is not valid: 422: code: invalid_input: message: invalid input: details: ttl is too low",
		},
		"body_not_json": {
			statusCode: http.StatusBadGateway,
			body:       "bad\ngateway",
			errMessage: "HTTP status is not valid: 502: badgateway",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			response := &http.Response{
				StatusCode: testCase.statusCode,
				Body:       io.NopCloser(strings.NewReader(testCase.body)),
			}

			err := handleErrorResponse(response)

			assert.ErrorContains(t, err, testCase.errMessage)
		})
	}
}
