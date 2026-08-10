// Package authtest provides in-memory implementations of the auth
// repositories. They let the service and the HTTP layer be exercised end to end
// without a database, in the same spirit as net/http/httptest.
package authtest

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth"
)

// UserRepository is an in-memory auth.UserRepository.
type UserRepository struct {
	mutex sync.Mutex
	users map[uuid.UUID]auth.Credentials

	// FailWith, when set, is returned by every method. It is how a test
	// exercises the "database is down" branch.
	FailWith error
}

// NewUserRepository builds an empty repository.
func NewUserRepository() *UserRepository {
	return &UserRepository{users: make(map[uuid.UUID]auth.Credentials)}
}

// Create inserts a user, rejecting a duplicate address case-insensitively just
// like the users_email_lower_uidx index does.
func (repository *UserRepository) Create(_ context.Context, params auth.CreateUserParams) (auth.User, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	if repository.FailWith != nil {
		return auth.User{}, repository.FailWith
	}

	for _, existing := range repository.users {
		if strings.EqualFold(existing.User.Email, params.Email) {
			return auth.User{}, auth.ErrEmailTaken
		}
	}

	user := auth.User{
		ID:          uuid.New(),
		Email:       params.Email,
		DisplayName: params.DisplayName,
		Status:      auth.StatusActive,
		CreatedAt:   time.Now().UTC(),
	}
	repository.users[user.ID] = auth.Credentials{User: user, PasswordHash: params.PasswordHash}

	return user, nil
}

// CredentialsByEmail looks a user up by address.
func (repository *UserRepository) CredentialsByEmail(_ context.Context, email string) (auth.Credentials, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	if repository.FailWith != nil {
		return auth.Credentials{}, repository.FailWith
	}

	for _, credentials := range repository.users {
		if strings.EqualFold(credentials.User.Email, email) {
			return credentials, nil
		}
	}

	return auth.Credentials{}, auth.ErrUserNotFound
}

// ByID looks a user up by identifier.
func (repository *UserRepository) ByID(_ context.Context, userID uuid.UUID) (auth.User, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	if repository.FailWith != nil {
		return auth.User{}, repository.FailWith
	}

	credentials, found := repository.users[userID]
	if !found {
		return auth.User{}, auth.ErrUserNotFound
	}

	return credentials.User, nil
}

// MarkLoggedIn records the sign-in timestamp.
func (repository *UserRepository) MarkLoggedIn(_ context.Context, userID uuid.UUID, loggedInAt time.Time) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	if repository.FailWith != nil {
		return repository.FailWith
	}

	credentials, found := repository.users[userID]
	if !found {
		return auth.ErrUserNotFound
	}

	credentials.User.LastLoginAt = &loggedInAt
	repository.users[userID] = credentials

	return nil
}

// SetStatus changes an account status so a test can block or delete a user.
func (repository *UserRepository) SetStatus(userID uuid.UUID, status string) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	credentials, found := repository.users[userID]
	if !found {
		return
	}

	credentials.User.Status = status
	repository.users[userID] = credentials
}

// SessionRepository is an in-memory auth.SessionRepository.
type SessionRepository struct {
	mutex    sync.Mutex
	sessions map[uuid.UUID]auth.RefreshSession

	// FailWith, when set, is returned by every method.
	FailWith error
}

// NewSessionRepository builds an empty repository.
func NewSessionRepository() *SessionRepository {
	return &SessionRepository{sessions: make(map[uuid.UUID]auth.RefreshSession)}
}

// Create stores a refresh session.
func (repository *SessionRepository) Create(_ context.Context, session auth.RefreshSession) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	if repository.FailWith != nil {
		return repository.FailWith
	}

	repository.sessions[session.ID] = session

	return nil
}

// ByTokenHash finds a session by digest, returning revoked rows as the
// PostgreSQL repository does.
func (repository *SessionRepository) ByTokenHash(_ context.Context, tokenHash []byte) (auth.RefreshSession, error) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	if repository.FailWith != nil {
		return auth.RefreshSession{}, repository.FailWith
	}

	for _, session := range repository.sessions {
		if string(session.TokenHash) == string(tokenHash) {
			return session, nil
		}
	}

	return auth.RefreshSession{}, auth.ErrSessionNotFound
}

// Revoke closes one session, keeping the first revocation timestamp.
func (repository *SessionRepository) Revoke(_ context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	if repository.FailWith != nil {
		return repository.FailWith
	}

	session, found := repository.sessions[sessionID]
	if !found || session.RevokedAt != nil {
		return nil
	}

	session.RevokedAt = &revokedAt
	repository.sessions[sessionID] = session

	return nil
}

// RevokeAllForUser closes every open session of a user.
func (repository *SessionRepository) RevokeAllForUser(_ context.Context, userID uuid.UUID, revokedAt time.Time) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	if repository.FailWith != nil {
		return repository.FailWith
	}

	for id, session := range repository.sessions {
		if session.UserID == userID && session.RevokedAt == nil {
			session.RevokedAt = &revokedAt
			repository.sessions[id] = session
		}
	}

	return nil
}

// ActiveCount reports how many sessions of a user are still open.
func (repository *SessionRepository) ActiveCount(userID uuid.UUID) int {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	count := 0

	for _, session := range repository.sessions {
		if session.UserID == userID && session.RevokedAt == nil {
			count++
		}
	}

	return count
}

// Expire back-dates the expiry of every session of a user so a test can reach
// the "refresh token expired" branch without waiting.
func (repository *SessionRepository) Expire(userID uuid.UUID) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()

	for id, session := range repository.sessions {
		if session.UserID == userID {
			session.ExpiresAt = time.Now().UTC().Add(-time.Hour)
			repository.sessions[id] = session
		}
	}
}
