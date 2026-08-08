package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type readinessStub struct {
	err error
}

func (stub readinessStub) Ping(context.Context) error {
	return stub.err
}

func TestHealth(t *testing.T) {
	suite := newSuite(t)

	response := suite.request(t, http.MethodGet, "/healthz", nil, "")

	if response.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if strings.TrimSpace(response.Body.String()) != `{"status":"ok"}` {
		t.Errorf("body = %q", response.Body.String())
	}
}

func TestReadinessUnavailable(t *testing.T) {
	suite := newSuite(t, withDatabaseError(errors.New("database unavailable")))

	response := suite.request(t, http.MethodGet, "/readyz", nil, "")

	if response.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestNewRequiresDependencies(t *testing.T) {
	if _, err := New(Options{Address: ":0"}); err == nil {
		t.Error("New() error = nil, want a missing dependency error")
	}
}

// TestUnknownRouteIsNotFound guards against a router change silently exposing
// an endpoint under a wildcard pattern.
func TestUnknownRouteIsNotFound(t *testing.T) {
	suite := newSuite(t)

	response := suite.request(t, http.MethodGet, "/api/v1/auth/nonexistent", nil, "")

	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestAuthRoutesRejectWrongMethod(t *testing.T) {
	suite := newSuite(t)

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/login", nil)
	suite.handler.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
