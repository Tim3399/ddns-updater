package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_newHandler(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		rootURL      string
		method       string
		path         string
		updateForced bool
		status       int
		allowHeader  string
	}{
		"post_update": {
			rootURL:      "/",
			method:       http.MethodPost,
			path:         "/update",
			updateForced: true,
			status:       http.StatusAccepted,
		},
		"post_update_with_root_url": {
			rootURL:      "/ddns",
			method:       http.MethodPost,
			path:         "/ddns/update",
			updateForced: true,
			status:       http.StatusAccepted,
		},
		"get_update": {
			rootURL:     "/",
			method:      http.MethodGet,
			path:        "/update",
			status:      http.StatusMethodNotAllowed,
			allowHeader: http.MethodPost,
		},
		"get_update_with_root_url": {
			rootURL:     "/ddns",
			method:      http.MethodGet,
			path:        "/ddns/update",
			status:      http.StatusMethodNotAllowed,
			allowHeader: http.MethodPost,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			runner := NewMockUpdateForcer(ctrl)
			if testCase.updateForced {
				runner.EXPECT().ForceUpdate(t.Context()).Return(nil)
			}
			handler := newHandler(t.Context(), testCase.rootURL, nil, runner)
			request := httptest.NewRequestWithContext(t.Context(), testCase.method, testCase.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			assert.Equal(t, testCase.status, response.Code)
			assert.Equal(t, testCase.allowHeader, response.Header().Get("Allow"))
		})
	}
}
