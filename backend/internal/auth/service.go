package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// UserRepository stores accounts. Create must translate a unique-email
// violation into ErrEmailTaken, and the lookups must return ErrUserNotFound
// when no row matches.
type UserRepository interface {
	Create(ctx context.Context, params CreateUserParams) (User, error)
	CredentialsByEmail(ctx context.Context, email string) (Credentials, error)
	ByID(ctx context.Context, userID uuid.UUID) (User, error)
	MarkLoggedIn(ctx context.Context, userID uuid.UUID, loggedInAt time.Time) error
}

// SessionRepository stores refresh sessions. ByTokenHash must return
// ErrSessionNotFound when no row matches, including for revoked or expired
// rows only if they were deleted; revoked rows are returned so the service can
// detect token reuse.
type SessionRepository interface {
	Create(ctx context.Context, session RefreshSession) error
	ByTokenHash(ctx context.Context, tokenHash []byte) (RefreshSession, error)
	Revoke(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error
}

// Service implements registration and the token lifecycle.
type Service struct {
	users      UserRepository
	sessions   SessionRepository
	tokens     *TokenIssuer
	hasher     *PasswordHasher
	refreshTTL time.Duration
	logger     *slog.Logger
	now        func() time.Time
}

// NewService wires the auth service.
func NewService(
	users UserRepository,
	sessions SessionRepository,
	tokens *TokenIssuer,
	hasher *PasswordHasher,
	refreshTTL time.Duration,
	logger *slog.Logger,
) (*Service, error) {
	switch {
	case users == nil || sessions == nil:
		return nil, errors.New("auth: repositories must not be nil")
	case tokens == nil || hasher == nil:
		return nil, errors.New("auth: token issuer and password hasher must not be nil")
	case refreshTTL <= tokens.TTL():
		return nil, errors.New("auth: refresh token TTL must be longer than the access token TTL")
	case logger == nil:
		return nil, errors.New("auth: logger must not be nil")
	}

	return &Service{
		users:      users,
		sessions:   sessions,
		tokens:     tokens,
		hasher:     hasher,
		refreshTTL: refreshTTL,
		logger:     logger,
		now:        time.Now,
	}, nil
}

// Register creates an account and signs the user in straight away, so the
// client does not have to follow registration with a login round trip.
func (service *Service) Register(ctx context.Context, input RegisterInput) (TokenPair, error) {
	email := normaliseEmail(input.Email)
	displayName := strings.TrimSpace(input.DisplayName)

	if err := validateEmail(email); err != nil {
		return TokenPair{}, err
	}
	if err := validateDisplayName(displayName); err != nil {
		return TokenPair{}, err
	}
	if err := ValidatePassword(input.Password); err != nil {
		return TokenPair{}, err
	}

	passwordHash, err := service.hasher.Hash(input.Password)
	if err != nil {
		return TokenPair{}, err
	}

	user, err := service.users.Create(ctx, CreateUserParams{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return TokenPair{}, err
	}

	service.logger.Info("user registered", "user_id", user.ID)

	return service.issueTokenPair(ctx, user)
}

// Login verifies a password and starts a new session.
func (service *Service) Login(ctx context.Context, input LoginInput) (TokenPair, error) {
	email := normaliseEmail(input.Email)

	credentials, err := service.users.CredentialsByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Spend the same time as a real bcrypt comparison so an attacker
			// cannot enumerate registered emails by measuring the response.
			service.hasher.VerifyPlaceholder()

			return TokenPair{}, ErrInvalidCredentials
		}

		return TokenPair{}, err
	}

	if err = service.hasher.Verify(credentials.PasswordHash, input.Password); err != nil {
		return TokenPair{}, err
	}
	if !credentials.User.IsActive() {
		return TokenPair{}, ErrUserNotActive
	}

	loggedInAt := service.now().UTC()
	if err = service.users.MarkLoggedIn(ctx, credentials.User.ID, loggedInAt); err != nil {
		// The sign-in itself succeeded; a failed bookkeeping update must not
		// cost the user their session.
		service.logger.Warn("update last login timestamp", "user_id", credentials.User.ID, "error", err)
	} else {
		credentials.User.LastLoginAt = &loggedInAt
	}

	service.logger.Info("user logged in", "user_id", credentials.User.ID)

	return service.issueTokenPair(ctx, credentials.User)
}

// Refresh exchanges a refresh token for a new pair and rotates the session, so
// a leaked token is usable at most once.
func (service *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	session, err := service.sessions.ByTokenHash(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return TokenPair{}, ErrInvalidRefreshToken
		}

		return TokenPair{}, err
	}

	now := service.now().UTC()

	if session.RevokedAt != nil {
		// The token was already rotated or logged out. Someone is replaying an
		// old token, which means either the client or the token store leaked:
		// drop every session of that user and make them sign in again.
		service.logger.Warn("refresh token reuse detected", "user_id", session.UserID, "session_id", session.ID)

		if err = service.sessions.RevokeAllForUser(ctx, session.UserID, now); err != nil {
			return TokenPair{}, err
		}

		return TokenPair{}, ErrInvalidRefreshToken
	}

	if !session.ExpiresAt.After(now) {
		return TokenPair{}, ErrInvalidRefreshToken
	}

	user, err := service.users.ByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return TokenPair{}, ErrInvalidRefreshToken
		}

		return TokenPair{}, err
	}
	if !user.IsActive() {
		return TokenPair{}, ErrUserNotActive
	}

	if err = service.sessions.Revoke(ctx, session.ID, now); err != nil {
		return TokenPair{}, err
	}

	return service.issueTokenPair(ctx, user)
}

// Logout revokes the session behind a refresh token. It is idempotent: an
// unknown or already revoked token is not an error, because a client that is
// signing out has nothing to do with the answer.
func (service *Service) Logout(ctx context.Context, refreshToken string) error {
	session, err := service.sessions.ByTokenHash(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}

		return err
	}
	if session.RevokedAt != nil {
		return nil
	}

	if err = service.sessions.Revoke(ctx, session.ID, service.now().UTC()); err != nil {
		return err
	}

	service.logger.Info("user logged out", "user_id", session.UserID)

	return nil
}

// LogoutAll revokes every session of a user. It backs "sign out on all
// devices" and is what a password change should call once that exists.
func (service *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	if err := service.sessions.RevokeAllForUser(ctx, userID, service.now().UTC()); err != nil {
		return err
	}

	service.logger.Info("all user sessions revoked", "user_id", userID)

	return nil
}

// ParseAccessToken verifies an access token without touching the database. The
// middleware uses it on every request, which is the point of a stateless JWT.
func (service *Service) ParseAccessToken(token string) (uuid.UUID, error) {
	claims, err := service.tokens.Parse(token)
	if err != nil {
		return uuid.Nil, err
	}

	return claims.UserID()
}

// UserByID loads the account behind an authenticated request.
func (service *Service) UserByID(ctx context.Context, userID uuid.UUID) (User, error) {
	user, err := service.users.ByID(ctx, userID)
	if err != nil {
		return User{}, err
	}
	if !user.IsActive() {
		return User{}, ErrUserNotActive
	}

	return user, nil
}

// issueTokenPair mints an access token and opens a refresh session for the user.
func (service *Service) issueTokenPair(ctx context.Context, user User) (TokenPair, error) {
	accessToken, accessExpiresAt, err := service.tokens.Issue(user.ID)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, tokenHash, err := newRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}

	now := service.now().UTC()
	refreshExpiresAt := now.Add(service.refreshTTL)

	session := RefreshSession{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: refreshExpiresAt,
		CreatedAt: now,
	}
	if err = service.sessions.Create(ctx, session); err != nil {
		return TokenPair{}, fmt.Errorf("open refresh session: %w", err)
	}

	return TokenPair{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshExpiresAt,
		User:                  user,
	}, nil
}
