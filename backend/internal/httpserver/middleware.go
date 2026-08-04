package httpserver

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// contextKey is unexported so no other package can collide with our keys.
type contextKey struct{ name string }

var userIDContextKey = contextKey{name: "user-id"}

// tokenParser verifies an access token and returns the user it belongs to.
// The middleware needs nothing else from the auth service.
type tokenParser interface {
	ParseAccessToken(token string) (uuid.UUID, error)
}

// requireAuth rejects a request that does not carry a valid access token and
// puts the caller's user id into the request context.
//
// Verification is signature-only: no database round trip per request. The cost
// is that revoking access takes until the access token expires, which is what
// the short ACCESS_TOKEN_TTL is for.
func requireAuth(parser tokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			token, ok := bearerToken(request)
			if !ok {
				// RFC 6750 asks for this header on a 401 from a bearer-token API.
				response.Header().Set("WWW-Authenticate", `Bearer realm="tamagotchi"`)
				writeError(response, http.StatusUnauthorized, codeUnauthorized, "authorization header with a bearer token is required")

				return
			}

			userID, err := parser.ParseAccessToken(token)
			if err != nil {
				response.Header().Set("WWW-Authenticate", `Bearer realm="tamagotchi", error="invalid_token"`)
				writeError(response, http.StatusUnauthorized, codeUnauthorized, "access token is invalid or expired")

				return
			}

			next.ServeHTTP(response, request.WithContext(withUserID(request.Context(), userID)))
		})
	}
}

// bearerToken extracts the credential from an Authorization header.
func bearerToken(request *http.Request) (string, bool) {
	header := request.Header.Get("Authorization")
	if header == "" {
		return "", false
	}

	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}

	token = strings.TrimSpace(token)

	return token, token != ""
}

// withUserID stores the authenticated user id in the context.
func withUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// userIDFromContext reads the user id put there by requireAuth. The second
// result is false when the handler was mounted without the middleware.
func userIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDContextKey).(uuid.UUID)

	return userID, ok
}

// chain applies middlewares to a handler so the first listed one runs first.
func chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for index := len(middlewares) - 1; index >= 0; index-- {
		handler = middlewares[index](handler)
	}

	return handler
}
