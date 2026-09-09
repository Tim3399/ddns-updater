package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type updateForcerFunc func(ctx context.Context) []error

func (f updateForcerFunc) ForceUpdate(ctx context.Context) []error {
	return f(ctx)
}

func TestForcedUpdateRequiresPOST(t *testing.T) {
	t.Parallel()

	called := false
	runner := updateForcerFunc(func(_ context.Context) []error {
		called = true
		return nil
	})
	handler := newHandler(context.Background(), "", nil, runner)

	request := httptest.NewRequest(http.MethodGet, "/update", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
	assert.Equal(t, http.MethodPost, response.Header().Get("Allow"))
	assert.False(t, called)
}

func TestForcedUpdatePOST(t *testing.T) {
	t.Parallel()

	called := false
	runner := updateForcerFunc(func(_ context.Context) []error {
		called = true
		return nil
	})
	handler := newHandler(context.Background(), "", nil, runner)

	request := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	require.True(t, called)
	assert.Equal(t, http.StatusAccepted, response.Code)
}
