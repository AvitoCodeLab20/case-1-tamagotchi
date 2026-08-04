package storage_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/storage"
)

// databaseURLEnv points these tests at a migrated PostgreSQL instance. Without
// it they skip, so `go test ./...` stays runnable with no infrastructure.
//
//	make up && TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5433/tamagotchi?sslmode=disable' make test-integration
const databaseURLEnv = "TEST_DATABASE_URL"

// newPool connects to the test database, or skips the test when it is absent.
func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv(databaseURLEnv)
	if databaseURL == "" {
		t.Skipf("%s is not set, skipping the database integration tests", databaseURLEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to %s: %v", databaseURLEnv, err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping the test database: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

// createUser inserts a user with a unique address and removes it afterwards, so
// the tests can run repeatedly against the same database.
func createUser(t *testing.T, pool *pgxpool.Pool) auth.User {
	t.Helper()

	repository := storage.NewUserRepository(pool)

	user, err := repository.Create(context.Background(), auth.CreateUserParams{
		Email:        "storage-" + uuid.NewString() + "@avito.test",
		DisplayName:  "Игрок",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Cleanup(func() {
		// ON DELETE CASCADE takes the refresh sessions with it.
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
			t.Errorf("clean up user %s: %v", user.ID, err)
		}
	})

	return user
}

// TestUserRepositoryCreateAndRead also proves that a google/uuid value survives
// a round trip through pgx and the UUID column.
func TestUserRepositoryCreateAndRead(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewUserRepository(pool)
	created := createUser(t, pool)

	if created.ID == uuid.Nil {
		t.Fatal("Create() returned a nil UUID")
	}
	if created.Status != auth.StatusActive {
		t.Errorf("status = %q, want %q", created.Status, auth.StatusActive)
	}
	if created.LastLoginAt != nil {
		t.Errorf("LastLoginAt = %v, want nil for a fresh account", created.LastLoginAt)
	}

	byID, err := repository.ByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}
	if byID.ID != created.ID || byID.Email != created.Email {
		t.Errorf("ByID() = %+v, want %+v", byID, created)
	}
}

func TestUserRepositoryLookupIsCaseInsensitive(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewUserRepository(pool)
	created := createUser(t, pool)

	upper := ""
	for _, char := range created.Email {
		if char >= 'a' && char <= 'z' {
			char -= 'a' - 'A'
		}
		upper += string(char)
	}

	credentials, err := repository.CredentialsByEmail(context.Background(), upper)
	if err != nil {
		t.Fatalf("CredentialsByEmail() error = %v", err)
	}
	if credentials.User.ID != created.ID {
		t.Errorf("user id = %s, want %s", credentials.User.ID, created.ID)
	}
	if credentials.PasswordHash == "" {
		t.Error("CredentialsByEmail() returned an empty password hash")
	}
}

// TestUserRepositoryRejectsDuplicateEmail proves that the unique index, not
// application code, is what enforces one account per address.
func TestUserRepositoryRejectsDuplicateEmail(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewUserRepository(pool)
	created := createUser(t, pool)

	_, err := repository.Create(context.Background(), auth.CreateUserParams{
		Email:        created.Email,
		DisplayName:  "Другой игрок",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
	})
	if !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("Create() error = %v, want ErrEmailTaken", err)
	}

	// The index is on lower(email), so a different spelling collides too.
	_, err = repository.Create(context.Background(), auth.CreateUserParams{
		Email:        "STORAGE" + created.Email[7:],
		DisplayName:  "Третий игрок",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
	})
	if !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("Create() with a different case error = %v, want ErrEmailTaken", err)
	}
}

func TestUserRepositoryMissingRows(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewUserRepository(pool)
	missing := uuid.New()

	if _, err := repository.ByID(context.Background(), missing); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("ByID() error = %v, want ErrUserNotFound", err)
	}
	if _, err := repository.CredentialsByEmail(context.Background(), "nobody@avito.test"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("CredentialsByEmail() error = %v, want ErrUserNotFound", err)
	}
	if err := repository.MarkLoggedIn(context.Background(), missing, time.Now()); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("MarkLoggedIn() error = %v, want ErrUserNotFound", err)
	}
}

func TestUserRepositoryMarkLoggedIn(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewUserRepository(pool)
	created := createUser(t, pool)

	loggedInAt := time.Now().UTC().Truncate(time.Millisecond)
	if err := repository.MarkLoggedIn(context.Background(), created.ID, loggedInAt); err != nil {
		t.Fatalf("MarkLoggedIn() error = %v", err)
	}

	reloaded, err := repository.ByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}
	if reloaded.LastLoginAt == nil {
		t.Fatal("LastLoginAt = nil after MarkLoggedIn()")
	}
	if !reloaded.LastLoginAt.Equal(loggedInAt) {
		t.Errorf("LastLoginAt = %s, want %s", reloaded.LastLoginAt, loggedInAt)
	}
}

func TestRefreshSessionRepositoryLifecycle(t *testing.T) {
	pool := newPool(t)
	sessions := storage.NewRefreshSessionRepository(pool)
	user := createUser(t, pool)

	now := time.Now().UTC().Truncate(time.Millisecond)
	session := auth.RefreshSession{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: []byte("0123456789abcdef0123456789abcdef"),
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}

	if err := sessions.Create(context.Background(), session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	stored, err := sessions.ByTokenHash(context.Background(), session.TokenHash)
	if err != nil {
		t.Fatalf("ByTokenHash() error = %v", err)
	}
	if stored.ID != session.ID || stored.UserID != user.ID {
		t.Errorf("stored = %+v, want the created session", stored)
	}
	if stored.RevokedAt != nil {
		t.Errorf("RevokedAt = %v, want nil for a fresh session", stored.RevokedAt)
	}

	revokedAt := now.Add(time.Minute)
	if err = sessions.Revoke(context.Background(), session.ID, revokedAt); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	revoked, err := sessions.ByTokenHash(context.Background(), session.TokenHash)
	if err != nil {
		t.Fatalf("ByTokenHash() after revoke error = %v", err)
	}
	// A revoked row must still be readable: reuse detection depends on telling
	// a replayed token apart from an unknown one.
	if revoked.RevokedAt == nil {
		t.Fatal("RevokedAt = nil after Revoke()")
	}

	// Revoking twice keeps the first timestamp.
	if err = sessions.Revoke(context.Background(), session.ID, revokedAt.Add(time.Hour)); err != nil {
		t.Fatalf("second Revoke() error = %v", err)
	}

	reRead, err := sessions.ByTokenHash(context.Background(), session.TokenHash)
	if err != nil {
		t.Fatalf("ByTokenHash() error = %v", err)
	}
	if !reRead.RevokedAt.Equal(*revoked.RevokedAt) {
		t.Errorf("RevokedAt = %s, want the first revocation at %s", reRead.RevokedAt, revoked.RevokedAt)
	}
}

func TestRefreshSessionRepositoryRevokeAllForUser(t *testing.T) {
	pool := newPool(t)
	sessions := storage.NewRefreshSessionRepository(pool)
	user := createUser(t, pool)

	now := time.Now().UTC()
	hashes := [][]byte{
		[]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1"),
		[]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa2"),
	}

	for _, hash := range hashes {
		err := sessions.Create(context.Background(), auth.RefreshSession{
			ID:        uuid.New(),
			UserID:    user.ID,
			TokenHash: hash,
			ExpiresAt: now.Add(24 * time.Hour),
			CreatedAt: now,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	if err := sessions.RevokeAllForUser(context.Background(), user.ID, now.Add(time.Minute)); err != nil {
		t.Fatalf("RevokeAllForUser() error = %v", err)
	}

	for _, hash := range hashes {
		session, err := sessions.ByTokenHash(context.Background(), hash)
		if err != nil {
			t.Fatalf("ByTokenHash() error = %v", err)
		}
		if session.RevokedAt == nil {
			t.Errorf("session %s is still open after RevokeAllForUser()", session.ID)
		}
	}
}

func TestRefreshSessionRepositoryMissingRow(t *testing.T) {
	pool := newPool(t)
	sessions := storage.NewRefreshSessionRepository(pool)

	_, err := sessions.ByTokenHash(context.Background(), []byte("no-session-has-this-token-digest"))
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("ByTokenHash() error = %v, want ErrSessionNotFound", err)
	}
}
