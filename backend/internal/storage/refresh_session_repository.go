package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth"
)

// RefreshSessionRepository reads and writes the refresh_sessions table. It
// implements auth.SessionRepository.
type RefreshSessionRepository struct {
	db Querier
}

// NewRefreshSessionRepository builds the repository over a pool or a transaction.
func NewRefreshSessionRepository(db Querier) *RefreshSessionRepository {
	return &RefreshSessionRepository{db: db}
}

// Create opens a refresh session. Only the digest of the token is written; the
// plaintext exists solely in the response to the client.
func (repository *RefreshSessionRepository) Create(ctx context.Context, session auth.RefreshSession) error {
	const query = `
		INSERT INTO refresh_sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := repository.db.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.ExpiresAt,
		session.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert refresh session: %w", err)
	}

	return nil
}

// ByTokenHash finds a session by the digest of its token. Revoked and expired
// rows are returned as well, because the service has to tell "unknown token"
// apart from "replayed token" to react to a leak.
func (repository *RefreshSessionRepository) ByTokenHash(ctx context.Context, tokenHash []byte) (auth.RefreshSession, error) {
	const query = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_sessions
		WHERE token_hash = $1`

	session := auth.RefreshSession{}

	err := repository.db.QueryRow(ctx, query, tokenHash).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.RefreshSession{}, auth.ErrSessionNotFound
		}

		return auth.RefreshSession{}, fmt.Errorf("select refresh session: %w", err)
	}

	return session, nil
}

// Revoke closes a single session. The `revoked_at IS NULL` guard keeps the
// first revocation timestamp when two requests race.
func (repository *RefreshSessionRepository) Revoke(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	const query = `
		UPDATE refresh_sessions
		SET revoked_at = $2
		WHERE id = $1 AND revoked_at IS NULL`

	if _, err := repository.db.Exec(ctx, query, sessionID, revokedAt); err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}

	return nil
}

// RevokeAllForUser closes every open session of a user. It backs both an
// explicit "sign out everywhere" and the automatic response to a replayed token.
func (repository *RefreshSessionRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error {
	const query = `
		UPDATE refresh_sessions
		SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL`

	if _, err := repository.db.Exec(ctx, query, userID, revokedAt); err != nil {
		return fmt.Errorf("revoke user refresh sessions: %w", err)
	}

	return nil
}
