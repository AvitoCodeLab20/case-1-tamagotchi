package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestBearerToken(t *testing.T) {
	testCases := []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{name: "standard", header: "Bearer abc.def.ghi", wantToken: "abc.def.ghi", wantOK: true},
		// RFC 7235 makes the scheme case-insensitive.
		{name: "lowercase scheme", header: "bearer abc.def.ghi", wantToken: "abc.def.ghi", wantOK: true},
		{name: "padded token", header: "Bearer   abc.def.ghi  ", wantToken: "abc.def.ghi", wantOK: true},
		{name: "missing header"},
		{name: "wrong scheme", header: "Basic dXNlcjpwYXNz"},
		{name: "scheme only", header: "Bearer"},
		{name: "empty token", header: "Bearer "},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			if testCase.header != "" {
				request.Header.Set("Authorization", testCase.header)
			}

			token, ok := bearerToken(request)

			if ok != testCase.wantOK {
				t.Fatalf("ok = %t, want %t", ok, testCase.wantOK)
			}
			if token != testCase.wantToken {
				t.Errorf("token = %q, want %q", token, testCase.wantToken)
			}
		})
	}
}

func TestUserIDFromContext(t *testing.T) {
	if _, ok := userIDFromContext(context.Background()); ok {
		t.Error("a bare context reported a user id")
	}

	userID := uuid.New()

	got, ok := userIDFromContext(withUserID(context.Background(), userID))
	if !ok {
		t.Fatal("the stored user id was not found")
	}
	if got != userID {
		t.Errorf("user id = %s, want %s", got, userID)
	}
}

func TestChainRunsMiddlewareInOrder(t *testing.T) {
	order := []string{}

	record := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				order = append(order, name)
				next.ServeHTTP(response, request)
			})
		}
	}

	handler := chain(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { order = append(order, "handler") }),
		record("first"),
		record("second"),
	)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))

	want := []string{"first", "second", "handler"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}

	for index, name := range want {
		if order[index] != name {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestClientIPFallsBackToRemoteAddr(t *testing.T) {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.4:44321"

	if ip := clientIP(request); ip != "198.51.100.4" {
		t.Errorf("clientIP() = %q, want the host part", ip)
	}

	// A forged forwarding header must not change the limiter key, otherwise the
	// budget resets on every request.
	request.Header.Set("X-Forwarded-For", "10.0.0.1")

	if ip := clientIP(request); ip != "198.51.100.4" {
		t.Errorf("clientIP() = %q, want X-Forwarded-For to be ignored", ip)
	}
}
