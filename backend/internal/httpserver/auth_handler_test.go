package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth/authtest"
)

const (
	testEmail    = "player@avito.test"
	testPassword = "correct-horse-battery"
	testName     = "Игрок"

	registerBody = `{"email":"player@avito.test","display_name":"Игрок","password":"correct-horse-battery"}`
	loginBody    = `{"email":"player@avito.test","password":"correct-horse-battery"}`
)

// suite is a server wired to in-memory repositories, exercised through the real
// router so routing, middleware, and error mapping are all covered.
type suite struct {
	handler  http.Handler
	users    *authtest.UserRepository
	sessions *authtest.SessionRepository
}

type suiteOption func(*suiteConfig)

type suiteConfig struct {
	databaseErr error
}

func withDatabaseError(err error) suiteOption {
	return func(cfg *suiteConfig) { cfg.databaseErr = err }
}

func newSuite(t *testing.T, options ...suiteOption) suite {
	t.Helper()

	cfg := suiteConfig{}
	for _, option := range options {
		option(&cfg)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	issuer, err := auth.NewTokenIssuer([]byte("test-signing-key-that-is-long-enough"), "avito-tamagotchi", time.Hour)
	if err != nil {
		t.Fatalf("NewTokenIssuer() error = %v", err)
	}

	hasher, err := auth.NewPasswordHasher(bcrypt.MinCost)
	if err != nil {
		t.Fatalf("NewPasswordHasher() error = %v", err)
	}

	users := authtest.NewUserRepository()
	sessions := authtest.NewSessionRepository()

	service, err := auth.NewService(users, sessions, issuer, hasher, 24*time.Hour, logger)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	server, err := New(Options{
		Address:  ":0",
		Database: readinessStub{err: cfg.databaseErr},
		Auth:     service,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return suite{handler: server.Handler, users: users, sessions: sessions}
}

// request sends a request through the router. An empty body is sent without a
// Content-Type header.
func (s suite) request(t *testing.T, method, path string, headers map[string]string, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request := httptest.NewRequestWithContext(context.Background(), method, path, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	for name, value := range headers {
		request.Header.Set(name, value)
	}

	response := httptest.NewRecorder()
	s.handler.ServeHTTP(response, request)

	return response
}

// registerUser runs the register endpoint and returns the decoded token pair.
func (s suite) registerUser(t *testing.T) tokenResponse {
	t.Helper()

	response := s.request(t, http.MethodPost, "/api/v1/auth/register", nil, registerBody)
	if response.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", response.Code, response.Body)
	}

	return decodeTokens(t, response)
}

func decodeTokens(t *testing.T, response *httptest.ResponseRecorder) tokenResponse {
	t.Helper()

	tokens := tokenResponse{}
	if err := json.Unmarshal(response.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("decode token response: %v (body = %s)", err, response.Body)
	}

	return tokens
}

func decodeError(t *testing.T, response *httptest.ResponseRecorder) errorBody {
	t.Helper()

	envelope := errorResponse{}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error response: %v (body = %s)", err, response.Body)
	}

	return envelope.Error
}

func TestRegisterEndpoint(t *testing.T) {
	s := newSuite(t)

	response := s.request(t, http.MethodPost, "/api/v1/auth/register", nil, registerBody)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}

	tokens := decodeTokens(t, response)

	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("response is missing a token")
	}
	if tokens.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want Bearer", tokens.TokenType)
	}
	if tokens.ExpiresIn <= 0 {
		t.Errorf("expires_in = %d, want a positive number of seconds", tokens.ExpiresIn)
	}
	if tokens.User.Email != testEmail || tokens.User.DisplayName != testName {
		t.Errorf("user = %+v, want the registered account", tokens.User)
	}
	if tokens.User.Status != auth.StatusActive {
		t.Errorf("status = %q, want %q", tokens.User.Status, auth.StatusActive)
	}
}

// TestResponseNeverLeaksThePasswordHash is the guarantee that matters most in
// the serialisation layer.
func TestResponseNeverLeaksThePasswordHash(t *testing.T) {
	s := newSuite(t)

	for _, path := range []string{"/api/v1/auth/register", "/api/v1/auth/login"} {
		body := registerBody
		if strings.HasSuffix(path, "login") {
			body = loginBody
		}

		response := s.request(t, http.MethodPost, path, nil, body)

		for _, forbidden := range []string{"password", "$2a$", "$2b$"} {
			if strings.Contains(response.Body.String(), forbidden) {
				t.Errorf("%s response contains %q: %s", path, forbidden, response.Body)
			}
		}
	}
}

func TestRegisterEndpointRejectsDuplicateEmail(t *testing.T) {
	s := newSuite(t)
	s.registerUser(t)

	response := s.request(t, http.MethodPost, "/api/v1/auth/register", nil, registerBody)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
	if code := decodeError(t, response).Code; code != codeEmailTaken {
		t.Errorf("code = %q, want %q", code, codeEmailTaken)
	}
}

func TestRegisterEndpointReportsTheInvalidField(t *testing.T) {
	testCases := []struct {
		name      string
		body      string
		wantField string
	}{
		{
			name:      "malformed email",
			body:      `{"email":"not-an-email","display_name":"Игрок","password":"correct-horse-battery"}`,
			wantField: "email",
		},
		{
			name:      "short password",
			body:      `{"email":"player@avito.test","display_name":"Игрок","password":"short"}`,
			wantField: "password",
		},
		{
			name:      "short display name",
			body:      `{"email":"player@avito.test","display_name":"И","password":"correct-horse-battery"}`,
			wantField: "display_name",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			s := newSuite(t)

			response := s.request(t, http.MethodPost, "/api/v1/auth/register", nil, testCase.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}

			body := decodeError(t, response)
			if body.Code != codeValidationFailed {
				t.Errorf("code = %q, want %q", body.Code, codeValidationFailed)
			}
			if body.Field != testCase.wantField {
				t.Errorf("field = %q, want %q", body.Field, testCase.wantField)
			}
		})
	}
}

func TestRegisterEndpointRejectsBadBodies(t *testing.T) {
	testCases := []struct {
		name        string
		body        string
		contentType string
	}{
		{name: "malformed JSON", body: `{"email":`},
		{name: "unknown field", body: `{"email":"a@b.co","display_name":"Ok","password":"correct-horse-battery","role":"admin"}`},
		{name: "two objects", body: registerBody + registerBody},
		{name: "wrong content type", body: registerBody, contentType: "text/plain"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			s := newSuite(t)

			headers := map[string]string{}
			if testCase.contentType != "" {
				headers["Content-Type"] = testCase.contentType
			}

			response := s.request(t, http.MethodPost, "/api/v1/auth/register", headers, testCase.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (body = %s)", response.Code, http.StatusBadRequest, response.Body)
			}
			if code := decodeError(t, response).Code; code != codeBadRequest {
				t.Errorf("code = %q, want %q", code, codeBadRequest)
			}
		})
	}
}

func TestRegisterEndpointRejectsOversizedBody(t *testing.T) {
	s := newSuite(t)

	oversized := `{"email":"player@avito.test","display_name":"` + strings.Repeat("и", maxRequestBodyBytes) + `","password":"correct-horse-battery"}`

	response := s.request(t, http.MethodPost, "/api/v1/auth/register", nil, oversized)
	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestLoginEndpoint(t *testing.T) {
	s := newSuite(t)
	registered := s.registerUser(t)

	response := s.request(t, http.MethodPost, "/api/v1/auth/login", nil, loginBody)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}

	tokens := decodeTokens(t, response)
	if tokens.User.ID != registered.User.ID {
		t.Errorf("user id = %q, want %q", tokens.User.ID, registered.User.ID)
	}
	if tokens.RefreshToken == registered.RefreshToken {
		t.Error("login reused the refresh token issued at registration")
	}
}

// TestLoginEndpointAnswersTheSameForUnknownEmailAndWrongPassword keeps the
// anti-enumeration property observable from outside.
func TestLoginEndpointAnswersTheSameForUnknownEmailAndWrongPassword(t *testing.T) {
	s := newSuite(t)
	s.registerUser(t)

	unknown := s.request(t, http.MethodPost, "/api/v1/auth/login", nil,
		`{"email":"nobody@avito.test","password":"correct-horse-battery"}`)
	wrongPassword := s.request(t, http.MethodPost, "/api/v1/auth/login", nil,
		`{"email":"player@avito.test","password":"wrong-password-here"}`)

	if unknown.Code != http.StatusUnauthorized || wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("statuses = %d and %d, want both %d", unknown.Code, wrongPassword.Code, http.StatusUnauthorized)
	}
	if unknown.Body.String() != wrongPassword.Body.String() {
		t.Errorf("responses differ: %q vs %q", unknown.Body, wrongPassword.Body)
	}
}

func TestLoginEndpointRejectsBlockedUser(t *testing.T) {
	s := newSuite(t)
	registered := s.registerUser(t)

	s.users.SetStatus(uuid.MustParse(registered.User.ID), auth.StatusBlocked)

	response := s.request(t, http.MethodPost, "/api/v1/auth/login", nil, loginBody)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if code := decodeError(t, response).Code; code != codeUserNotActive {
		t.Errorf("code = %q, want %q", code, codeUserNotActive)
	}
}

func TestRefreshEndpointRotatesTokens(t *testing.T) {
	s := newSuite(t)
	registered := s.registerUser(t)

	body := `{"refresh_token":"` + registered.RefreshToken + `"}`

	response := s.request(t, http.MethodPost, "/api/v1/auth/refresh", nil, body)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}

	rotated := decodeTokens(t, response)
	if rotated.RefreshToken == registered.RefreshToken {
		t.Fatal("refresh returned the same token")
	}

	// The consumed token is dead.
	replay := s.request(t, http.MethodPost, "/api/v1/auth/refresh", nil, body)
	if replay.Code != http.StatusUnauthorized {
		t.Errorf("replay status = %d, want %d", replay.Code, http.StatusUnauthorized)
	}
}

func TestRefreshEndpointRejectsUnknownToken(t *testing.T) {
	s := newSuite(t)

	response := s.request(t, http.MethodPost, "/api/v1/auth/refresh", nil, `{"refresh_token":"nobody-issued-this"}`)
	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestRefreshEndpointRequiresTheToken(t *testing.T) {
	s := newSuite(t)

	response := s.request(t, http.MethodPost, "/api/v1/auth/refresh", nil, `{"refresh_token":""}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if field := decodeError(t, response).Field; field != "refresh_token" {
		t.Errorf("field = %q, want refresh_token", field)
	}
}

func TestLogoutEndpoint(t *testing.T) {
	s := newSuite(t)
	registered := s.registerUser(t)

	body := `{"refresh_token":"` + registered.RefreshToken + `"}`

	response := s.request(t, http.MethodPost, "/api/v1/auth/logout", nil, body)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}

	// Logging out twice, and logging out an unknown token, both answer 204: the
	// endpoint must not become an oracle for which tokens exist.
	repeat := s.request(t, http.MethodPost, "/api/v1/auth/logout", nil, body)
	unknown := s.request(t, http.MethodPost, "/api/v1/auth/logout", nil, `{"refresh_token":"nobody-issued-this"}`)

	if repeat.Code != http.StatusNoContent || unknown.Code != http.StatusNoContent {
		t.Errorf("statuses = %d and %d, want both %d", repeat.Code, unknown.Code, http.StatusNoContent)
	}

	refresh := s.request(t, http.MethodPost, "/api/v1/auth/refresh", nil, body)
	if refresh.Code != http.StatusUnauthorized {
		t.Errorf("refresh after logout status = %d, want %d", refresh.Code, http.StatusUnauthorized)
	}
}

func TestCurrentUserEndpoint(t *testing.T) {
	s := newSuite(t)
	registered := s.registerUser(t)

	response := s.request(t, http.MethodGet, "/api/v1/auth/me",
		map[string]string{"Authorization": "Bearer " + registered.AccessToken}, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}

	user := userResponse{}
	if err := json.Unmarshal(response.Body.Bytes(), &user); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	if user.ID != registered.User.ID {
		t.Errorf("id = %q, want %q", user.ID, registered.User.ID)
	}
}

func TestCurrentUserEndpointRequiresAValidToken(t *testing.T) {
	s := newSuite(t)
	registered := s.registerUser(t)

	testCases := []struct {
		name   string
		header string
	}{
		{name: "no header"},
		{name: "wrong scheme", header: "Basic " + registered.AccessToken},
		{name: "no token", header: "Bearer "},
		{name: "garbage token", header: "Bearer not.a.jwt"},
		{name: "token from another key", header: "Bearer " + foreignToken(t)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			headers := map[string]string{}
			if testCase.header != "" {
				headers["Authorization"] = testCase.header
			}

			response := s.request(t, http.MethodGet, "/api/v1/auth/me", headers, "")
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
			if response.Header().Get("WWW-Authenticate") == "" {
				t.Error("WWW-Authenticate header is missing from the 401")
			}
		})
	}
}

func TestCurrentUserEndpointRejectsBlockedUser(t *testing.T) {
	s := newSuite(t)
	registered := s.registerUser(t)

	s.users.SetStatus(uuid.MustParse(registered.User.ID), auth.StatusBlocked)

	// The access token is still cryptographically valid, so this proves the
	// handler re-checks the account status rather than trusting the token.
	response := s.request(t, http.MethodGet, "/api/v1/auth/me",
		map[string]string{"Authorization": "Bearer " + registered.AccessToken}, "")
	if response.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestLogoutAllEndpoint(t *testing.T) {
	s := newSuite(t)
	registered := s.registerUser(t)

	if login := s.request(t, http.MethodPost, "/api/v1/auth/login", nil, loginBody); login.Code != http.StatusOK {
		t.Fatalf("login status = %d", login.Code)
	}

	response := s.request(t, http.MethodPost, "/api/v1/auth/logout-all",
		map[string]string{"Authorization": "Bearer " + registered.AccessToken}, `{}`)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}

	if count := s.sessions.ActiveCount(uuid.MustParse(registered.User.ID)); count != 0 {
		t.Errorf("active sessions = %d, want 0", count)
	}
}

func TestLogoutAllEndpointRequiresAuthentication(t *testing.T) {
	s := newSuite(t)

	response := s.request(t, http.MethodPost, "/api/v1/auth/logout-all", nil, `{}`)
	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

// foreignToken mints a well-formed token signed with a key the server does not
// trust.
func foreignToken(t *testing.T) string {
	t.Helper()

	issuer, err := auth.NewTokenIssuer([]byte("a-completely-different-signing-key-32"), "avito-tamagotchi", time.Hour)
	if err != nil {
		t.Fatalf("NewTokenIssuer() error = %v", err)
	}

	token, _, err := issuer.Issue(uuid.New())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	return token
}
