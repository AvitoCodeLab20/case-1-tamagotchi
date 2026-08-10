package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth/authtest"
)

const (
	testEmail    = "player@avito.test"
	testPassword = "correct-horse-battery"
	testName     = "Игрок"
)

type fixture struct {
	service  *auth.Service
	users    *authtest.UserRepository
	sessions *authtest.SessionRepository
}

func TestRegisterSignsTheUserIn(t *testing.T) {
	suite := newFixture(t)

	pair, err := suite.service.Register(context.Background(), auth.RegisterInput{
		Email:       "  Player@Avito.TEST ",
		DisplayName: "  Игрок  ",
		Password:    testPassword,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// The address is normalised on the way in so the unique index and every
	// later lookup agree on one spelling.
	if pair.User.Email != testEmail {
		t.Errorf("Email = %q, want %q", pair.User.Email, testEmail)
	}
	if pair.User.DisplayName != testName {
		t.Errorf("DisplayName = %q, want %q", pair.User.DisplayName, testName)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("Register() returned an empty token")
	}

	userID, err := suite.service.ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if userID != pair.User.ID {
		t.Errorf("token subject = %s, want %s", userID, pair.User.ID)
	}
}

func TestRegisterRejectsDuplicateEmailRegardlessOfCase(t *testing.T) {
	suite := newFixture(t)
	suite.register(t)

	_, err := suite.service.Register(context.Background(), auth.RegisterInput{
		Email:       "PLAYER@AVITO.TEST",
		DisplayName: "Другой игрок",
		Password:    testPassword,
	})
	if !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("Register() error = %v, want ErrEmailTaken", err)
	}
}

func TestRegisterValidatesInput(t *testing.T) {
	testCases := []struct {
		name      string
		input     auth.RegisterInput
		wantField string
	}{
		{
			name:      "empty email",
			input:     auth.RegisterInput{Email: "", DisplayName: testName, Password: testPassword},
			wantField: "email",
		},
		{
			name:      "malformed email",
			input:     auth.RegisterInput{Email: "player-at-avito", DisplayName: testName, Password: testPassword},
			wantField: "email",
		},
		{
			name:      "email with a display part",
			input:     auth.RegisterInput{Email: "Player <p@avito.test>", DisplayName: testName, Password: testPassword},
			wantField: "email",
		},
		{
			name:      "display name too short",
			input:     auth.RegisterInput{Email: testEmail, DisplayName: "И", Password: testPassword},
			wantField: "display_name",
		},
		{
			name:      "password too short",
			input:     auth.RegisterInput{Email: testEmail, DisplayName: testName, Password: "short"},
			wantField: "password",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			suite := newFixture(t)

			_, err := suite.service.Register(context.Background(), testCase.input)

			validationError := auth.ValidationError{}
			if !errors.As(err, &validationError) {
				t.Fatalf("Register() error = %v, want a ValidationError", err)
			}
			if validationError.Field != testCase.wantField {
				t.Errorf("field = %q, want %q", validationError.Field, testCase.wantField)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	suite := newFixture(t)
	registered := suite.register(t)

	pair, err := suite.service.Login(context.Background(), auth.LoginInput{
		Email:    "PLAYER@avito.test",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if pair.User.ID != registered.User.ID {
		t.Errorf("user id = %s, want %s", pair.User.ID, registered.User.ID)
	}
	if pair.User.LastLoginAt == nil {
		t.Error("LastLoginAt = nil, want the sign-in timestamp")
	}
	if pair.RefreshToken == registered.RefreshToken {
		t.Error("Login() reused the refresh token from registration")
	}
}

// TestLoginHidesWhetherTheEmailExists is the anti-enumeration guarantee: a
// wrong password and an unknown address must be indistinguishable to a client.
func TestLoginHidesWhetherTheEmailExists(t *testing.T) {
	suite := newFixture(t)
	suite.register(t)

	unknown := suite.service
	_, unknownErr := unknown.Login(context.Background(), auth.LoginInput{
		Email:    "nobody@avito.test",
		Password: testPassword,
	})
	_, wrongPasswordErr := suite.service.Login(context.Background(), auth.LoginInput{
		Email:    testEmail,
		Password: "not-the-right-password",
	})

	if !errors.Is(unknownErr, auth.ErrInvalidCredentials) {
		t.Errorf("unknown email error = %v, want ErrInvalidCredentials", unknownErr)
	}
	if !errors.Is(wrongPasswordErr, auth.ErrInvalidCredentials) {
		t.Errorf("wrong password error = %v, want ErrInvalidCredentials", wrongPasswordErr)
	}
	if unknownErr.Error() != wrongPasswordErr.Error() {
		t.Errorf("the two failures differ: %q vs %q", unknownErr, wrongPasswordErr)
	}
}

func TestLoginRejectsBlockedUser(t *testing.T) {
	suite := newFixture(t)
	registered := suite.register(t)
	suite.users.SetStatus(registered.User.ID, auth.StatusBlocked)

	_, err := suite.service.Login(context.Background(), auth.LoginInput{Email: testEmail, Password: testPassword})
	if !errors.Is(err, auth.ErrUserNotActive) {
		t.Errorf("Login() error = %v, want ErrUserNotActive", err)
	}
}

func TestRefreshRotatesTheSession(t *testing.T) {
	suite := newFixture(t)
	registered := suite.register(t)

	refreshed, err := suite.service.Refresh(context.Background(), registered.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.RefreshToken == registered.RefreshToken {
		t.Fatal("Refresh() returned the same refresh token, so the session was not rotated")
	}
	if count := suite.sessions.ActiveCount(registered.User.ID); count != 1 {
		t.Errorf("active sessions = %d, want 1", count)
	}

	// The rotated-out token must be dead immediately.
	if _, err = suite.service.Refresh(context.Background(), registered.RefreshToken); !errors.Is(err, auth.ErrInvalidRefreshToken) {
		t.Errorf("replaying the old token error = %v, want ErrInvalidRefreshToken", err)
	}
}

// TestRefreshReuseRevokesEverySession covers the leak response: replaying a
// rotated-out token means the token store is compromised, so every session of
// that user is closed.
func TestRefreshReuseRevokesEverySession(t *testing.T) {
	suite := newFixture(t)
	registered := suite.register(t)

	rotated, err := suite.service.Refresh(context.Background(), registered.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if _, err = suite.service.Refresh(context.Background(), registered.RefreshToken); !errors.Is(err, auth.ErrInvalidRefreshToken) {
		t.Fatalf("replay error = %v, want ErrInvalidRefreshToken", err)
	}

	if count := suite.sessions.ActiveCount(registered.User.ID); count != 0 {
		t.Errorf("active sessions after replay = %d, want 0", count)
	}
	// The token the honest client holds is revoked too: it has to sign in again.
	if _, err = suite.service.Refresh(context.Background(), rotated.RefreshToken); !errors.Is(err, auth.ErrInvalidRefreshToken) {
		t.Errorf("the current token error = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestRefreshRejectsUnknownAndExpiredTokens(t *testing.T) {
	suite := newFixture(t)
	registered := suite.register(t)

	if _, err := suite.service.Refresh(context.Background(), "a-token-nobody-issued"); !errors.Is(err, auth.ErrInvalidRefreshToken) {
		t.Errorf("unknown token error = %v, want ErrInvalidRefreshToken", err)
	}

	suite.sessions.Expire(registered.User.ID)

	if _, err := suite.service.Refresh(context.Background(), registered.RefreshToken); !errors.Is(err, auth.ErrInvalidRefreshToken) {
		t.Errorf("expired token error = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestRefreshRejectsBlockedUser(t *testing.T) {
	suite := newFixture(t)
	registered := suite.register(t)
	suite.users.SetStatus(registered.User.ID, auth.StatusBlocked)

	if _, err := suite.service.Refresh(context.Background(), registered.RefreshToken); !errors.Is(err, auth.ErrUserNotActive) {
		t.Errorf("Refresh() error = %v, want ErrUserNotActive", err)
	}
}

func TestLogoutIsIdempotent(t *testing.T) {
	suite := newFixture(t)
	registered := suite.register(t)

	for range 2 {
		if err := suite.service.Logout(context.Background(), registered.RefreshToken); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
	}
	if err := suite.service.Logout(context.Background(), "a-token-nobody-issued"); err != nil {
		t.Errorf("Logout() with an unknown token error = %v, want nil", err)
	}

	if count := suite.sessions.ActiveCount(registered.User.ID); count != 0 {
		t.Errorf("active sessions = %d, want 0", count)
	}
	if _, err := suite.service.Refresh(context.Background(), registered.RefreshToken); !errors.Is(err, auth.ErrInvalidRefreshToken) {
		t.Errorf("refresh after logout error = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestLogoutAllClosesEverySession(t *testing.T) {
	suite := newFixture(t)
	registered := suite.register(t)

	// A second device.
	if _, err := suite.service.Login(context.Background(), auth.LoginInput{Email: testEmail, Password: testPassword}); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if count := suite.sessions.ActiveCount(registered.User.ID); count != 2 {
		t.Fatalf("active sessions = %d, want 2", count)
	}

	if err := suite.service.LogoutAll(context.Background(), registered.User.ID); err != nil {
		t.Fatalf("LogoutAll() error = %v", err)
	}
	if count := suite.sessions.ActiveCount(registered.User.ID); count != 0 {
		t.Errorf("active sessions = %d, want 0", count)
	}
}

// TestAccessTokenSurvivesLogout documents the trade-off of stateless
// verification: revoking a session kills refreshing, but an already issued
// access token stays valid until it expires.
func TestAccessTokenSurvivesLogout(t *testing.T) {
	suite := newFixture(t)
	registered := suite.register(t)

	if err := suite.service.Logout(context.Background(), registered.RefreshToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	if _, err := suite.service.ParseAccessToken(registered.AccessToken); err != nil {
		t.Errorf("ParseAccessToken() error = %v, want the token to remain valid until it expires", err)
	}
}

func TestUserByIDRejectsBlockedUser(t *testing.T) {
	suite := newFixture(t)
	registered := suite.register(t)

	if _, err := suite.service.UserByID(context.Background(), registered.User.ID); err != nil {
		t.Fatalf("UserByID() error = %v", err)
	}

	suite.users.SetStatus(registered.User.ID, auth.StatusDeleted)

	if _, err := suite.service.UserByID(context.Background(), registered.User.ID); !errors.Is(err, auth.ErrUserNotActive) {
		t.Errorf("UserByID() error = %v, want ErrUserNotActive", err)
	}
}

func TestLoginPropagatesRepositoryFailure(t *testing.T) {
	suite := newFixture(t)
	suite.register(t)

	unavailable := errors.New("database unavailable")
	suite.users.FailWith = unavailable

	_, err := suite.service.Login(context.Background(), auth.LoginInput{Email: testEmail, Password: testPassword})
	if !errors.Is(err, unavailable) {
		t.Errorf("Login() error = %v, want the repository failure", err)
	}
	// An infrastructure failure must not be reported as bad credentials.
	if errors.Is(err, auth.ErrInvalidCredentials) {
		t.Error("Login() reported a database failure as invalid credentials")
	}
}

func TestNewServiceValidatesArguments(t *testing.T) {
	issuer, err := auth.NewTokenIssuer([]byte("test-signing-key-that-is-long-enough"), "avito-tamagotchi", time.Hour)
	if err != nil {
		t.Fatalf("NewTokenIssuer() error = %v", err)
	}

	hasher, err := auth.NewPasswordHasher(bcrypt.MinCost)
	if err != nil {
		t.Fatalf("NewPasswordHasher() error = %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// A refresh token no longer-lived than an access token defeats the point of
	// having two tokens.
	_, err = auth.NewService(authtest.NewUserRepository(), authtest.NewSessionRepository(), issuer, hasher, time.Minute, logger)
	if err == nil {
		t.Error("NewService() error = nil, want a TTL ordering error")
	}

	_, err = auth.NewService(nil, authtest.NewSessionRepository(), issuer, hasher, 24*time.Hour, logger)
	if err == nil {
		t.Error("NewService() error = nil, want a nil repository error")
	}
}

func newFixture(t *testing.T) fixture {
	t.Helper()

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

	service, err := auth.NewService(users, sessions, issuer, hasher, 24*time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return fixture{service: service, users: users, sessions: sessions}
}

func (suite fixture) register(t *testing.T) auth.TokenPair {
	t.Helper()

	pair, err := suite.service.Register(context.Background(), auth.RegisterInput{
		Email:       testEmail,
		DisplayName: testName,
		Password:    testPassword,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	return pair
}
