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

// emailUniqueIndex is the index that enforces one account per address,
// case-insensitively.
const emailUniqueIndex = "users_email_lower_uidx"

// UserRepository reads and writes the users table. It implements
// auth.UserRepository.
type UserRepository struct {
	db Querier
}

// NewUserRepository builds the repository over a pool or a transaction.
func NewUserRepository(db Querier) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts an account and returns it. A duplicate address surfaces as
// auth.ErrEmailTaken: the unique index is the single source of truth, so two
// concurrent registrations cannot both succeed.
func (repository *UserRepository) Create(ctx context.Context, params auth.CreateUserParams) (auth.User, error) {
	const query = `
		INSERT INTO users (email, display_name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, display_name, status, last_login_at, created_at`

	user := auth.User{}

	err := repository.db.QueryRow(ctx, query, params.Email, params.DisplayName, params.PasswordHash).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.Status,
		&user.LastLoginAt,
		&user.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err, emailUniqueIndex) {
			return auth.User{}, auth.ErrEmailTaken
		}

		return auth.User{}, fmt.Errorf("insert user: %w", err)
	}

	return user, nil
}

// CredentialsByEmail loads an account together with its password hash. The
// predicate matches the lower(email) expression index, so the lookup stays on
// the index.
func (repository *UserRepository) CredentialsByEmail(ctx context.Context, email string) (auth.Credentials, error) {
	const query = `
		SELECT id, email, display_name, status, last_login_at, created_at, password_hash
		FROM users
		WHERE lower(email) = lower($1)`

	credentials := auth.Credentials{}

	err := repository.db.QueryRow(ctx, query, email).Scan(
		&credentials.User.ID,
		&credentials.User.Email,
		&credentials.User.DisplayName,
		&credentials.User.Status,
		&credentials.User.LastLoginAt,
		&credentials.User.CreatedAt,
		&credentials.PasswordHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.Credentials{}, auth.ErrUserNotFound
		}

		return auth.Credentials{}, fmt.Errorf("select user by email: %w", err)
	}

	return credentials, nil
}

// ByID loads an account without its password hash.
func (repository *UserRepository) ByID(ctx context.Context, userID uuid.UUID) (auth.User, error) {
	const query = `
		SELECT id, email, display_name, status, last_login_at, created_at
		FROM users
		WHERE id = $1`

	user := auth.User{}

	err := repository.db.QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.Status,
		&user.LastLoginAt,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, auth.ErrUserNotFound
		}

		return auth.User{}, fmt.Errorf("select user by id: %w", err)
	}

	return user, nil
}

// MarkLoggedIn records the moment of a successful sign-in.
func (repository *UserRepository) MarkLoggedIn(ctx context.Context, userID uuid.UUID, loggedInAt time.Time) error {
	const query = `UPDATE users SET last_login_at = $2 WHERE id = $1`

	tag, err := repository.db.Exec(ctx, query, userID, loggedInAt)
	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrUserNotFound
	}

	return nil
}
