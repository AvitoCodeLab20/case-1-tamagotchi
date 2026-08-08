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
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/leaderboard"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/rewards"
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

func TestLeaderboardRepositoryAggregatesWeeklyExperience(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewLeaderboardRepository(pool)
	activeUser := createUser(t, pool)
	blockedUser := createUser(t, pool)
	if _, err := pool.Exec(context.Background(), `UPDATE users SET status = 'blocked' WHERE id = $1`, blockedUser.ID); err != nil {
		t.Fatalf("block user: %v", err)
	}
	startsAt := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	endsAt := startsAt.AddDate(0, 0, 7)
	activePetID := createPet(t, pool, activeUser.ID)
	blockedPetID := createPet(t, pool, blockedUser.ID)
	insertPetAction(t, pool, activeUser.ID, activePetID, 20, startsAt.Add(time.Hour))
	insertPetAction(t, pool, activeUser.ID, activePetID, 30, startsAt.Add(2*time.Hour))
	insertPetAction(t, pool, activeUser.ID, activePetID, 100, startsAt.Add(-time.Second))
	insertPetAction(t, pool, blockedUser.ID, blockedPetID, 500, startsAt.Add(time.Hour))
	participants, err := repository.WeeklyParticipants(context.Background(), startsAt, endsAt)
	if err != nil {
		t.Fatalf("WeeklyParticipants() error = %v", err)
	}
	if len(participants) != 1 || participants[0].UserID != activeUser.ID || participants[0].WeeklyExperience != 50 {
		t.Fatalf("participants = %+v, want active user with 50 XP", participants)
	}
}

func TestLeaderboardRepositoryFinalizesWeekIdempotently(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewLeaderboardRepository(pool)
	startsAt := time.Now().UTC().AddDate(0, 0, -7).Truncate(time.Second)
	period := leaderboard.Period{
		StartsAt: startsAt,
		EndsAt:   startsAt.AddDate(0, 0, 7),
		Timezone: "Europe/Moscow",
	}
	users := []auth.User{createUser(t, pool), createUser(t, pool), createUser(t, pool)}
	entries := []leaderboard.Entry{
		{Participant: leaderboard.Participant{
			UserID: users[0].ID, DisplayName: users[0].DisplayName, WeeklyExperience: 300, ReachedAt: startsAt,
		}, Rank: 1, PrizeTier: 5},
		{Participant: leaderboard.Participant{
			UserID: users[1].ID, DisplayName: users[1].DisplayName, WeeklyExperience: 200, ReachedAt: startsAt,
		}, Rank: 2, PrizeTier: 10},
		{Participant: leaderboard.Participant{
			UserID: users[2].ID, DisplayName: users[2].DisplayName, WeeklyExperience: 100, ReachedAt: startsAt,
		}, Rank: 3, PrizeTier: 15},
	}
	finalizedAt := period.EndsAt.Add(time.Hour)
	selectBefore := period.EndsAt.AddDate(0, 0, 7)
	if err := repository.SaveFinalizedWeek(
		context.Background(), period, entries, finalizedAt, selectBefore,
	); err != nil {
		t.Fatalf("SaveFinalizedWeek() error = %v", err)
	}
	if err := repository.SaveFinalizedWeek(
		context.Background(), period, entries, finalizedAt, selectBefore,
	); err != nil {
		t.Fatalf("repeated SaveFinalizedWeek() error = %v", err)
	}
	resultCount := 0
	awardCount := 0
	if err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT COUNT(*) FROM leaderboard_results WHERE week_id = week.id),
			(SELECT COUNT(*) FROM leaderboard_awards WHERE week_id = week.id)
		FROM leaderboard_weeks AS week
		WHERE week.starts_at = $1`, period.StartsAt).Scan(&resultCount, &awardCount); err != nil {
		t.Fatalf("query finalized week: %v", err)
	}
	if resultCount != 3 || awardCount != 3 {
		t.Fatalf("result count = %d, award count = %d", resultCount, awardCount)
	}
}

func TestRewardRepositoryIssueListAndRedeem(t *testing.T) {
	pool := newPool(t)
	service, err := rewards.NewService(storage.NewRewardRepository(pool))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	user := createUser(t, pool)
	now := time.Now().UTC().Truncate(time.Millisecond)
	issued, err := service.IssueLevel(context.Background(), user.ID, 5, 1, now)
	if err != nil {
		t.Fatalf("IssueLevel() error = %v", err)
	}
	if len(issued) != 1 {
		t.Fatalf("len(issued) = %d, want 1", len(issued))
	}
	repeated, err := service.IssueLevel(context.Background(), user.ID, 5, 1, now)
	if err != nil {
		t.Fatalf("repeated IssueLevel() error = %v", err)
	}
	if len(repeated) != 1 || repeated[0].ID != issued[0].ID {
		t.Fatalf("repeated issue = %+v, want reward %s", repeated, issued[0].ID)
	}
	inventory, err := service.ListInventory(context.Background(), user.ID, rewards.StatusIssued)
	if err != nil {
		t.Fatalf("ListInventory() error = %v", err)
	}
	if len(inventory.Rewards) != 1 || len(inventory.LeaderboardAwards) != 0 {
		t.Fatalf("inventory = %+v", inventory)
	}
	key := uuid.New()
	redeemed, err := service.Redeem(context.Background(), user.ID, issued[0].ID, key, "", "")
	if err != nil {
		t.Fatalf("Redeem() error = %v", err)
	}
	if redeemed.Status != rewards.StatusRedeemed || redeemed.RedeemedAt == nil {
		t.Fatalf("redeemed = %+v", redeemed)
	}
	repeatedRedemption, err := service.Redeem(context.Background(), user.ID, issued[0].ID, key, "", "")
	if err != nil {
		t.Fatalf("repeated Redeem() error = %v", err)
	}
	if repeatedRedemption.ID != redeemed.ID {
		t.Fatalf("repeated reward id = %s, want %s", repeatedRedemption.ID, redeemed.ID)
	}
	streakRewards, err := service.IssueStreak(context.Background(), user.ID, 7, now, now)
	if err != nil {
		t.Fatalf("IssueStreak() error = %v", err)
	}
	_, err = service.Redeem(context.Background(), user.ID, streakRewards[0].ID, key, "", "")
	if !errors.Is(err, rewards.ErrIdempotencyConflict) {
		t.Fatalf("reused key error = %v, want ErrIdempotencyConflict", err)
	}
}

func TestRewardRepositoryValidatesCategory(t *testing.T) {
	pool := newPool(t)
	service, err := rewards.NewService(storage.NewRewardRepository(pool))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	user := createUser(t, pool)
	now := time.Now().UTC().Truncate(time.Millisecond)
	issued, err := service.IssueLevel(context.Background(), user.ID, 15, 1, now)
	if err != nil {
		t.Fatalf("IssueLevel() error = %v", err)
	}
	if len(issued) != 1 {
		t.Fatalf("len(issued) = %d, want 1", len(issued))
	}
	_, err = service.Redeem(context.Background(), user.ID, issued[0].ID, uuid.New(), "unknown", "")
	validationError := rewards.ValidationError{}
	if !errors.As(err, &validationError) || validationError.Field != "category_code" {
		t.Fatalf("Redeem() error = %v, want category validation error", err)
	}
	redeemed, err := service.Redeem(
		context.Background(), user.ID, issued[0].ID, uuid.New(), "electronics", "",
	)
	if err != nil {
		t.Fatalf("Redeem() with allowed category error = %v", err)
	}
	if redeemed.Status != rewards.StatusRedeemed {
		t.Fatalf("redeemed status = %q, want %q", redeemed.Status, rewards.StatusRedeemed)
	}
}

func TestRewardRepositorySelectsLeaderboardAward(t *testing.T) {
	pool := newPool(t)
	service, err := rewards.NewService(storage.NewRewardRepository(pool))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	user := createUser(t, pool)
	now := time.Now().UTC().Truncate(time.Millisecond)
	weekID := uuid.New()
	if _, err = pool.Exec(context.Background(), `
		INSERT INTO leaderboard_weeks (
			id, starts_at, ends_at, status, participants_count, top_5_max_rank, finalized_at
		) VALUES ($1, $2, $3, 'final', 1, 1, $3)`, weekID, now.AddDate(0, 0, -7), now); err != nil {
		t.Fatalf("insert leaderboard week: %v", err)
	}
	if _, err = pool.Exec(context.Background(), `
		INSERT INTO leaderboard_results (
			week_id, user_id, display_name, weekly_experience, rank, reached_at, prize_tier
		) VALUES ($1, $2, $3, 100, 1, $4, 5)`, weekID, user.ID, user.DisplayName, now); err != nil {
		t.Fatalf("insert leaderboard result: %v", err)
	}
	awardID := uuid.New()
	if _, err = pool.Exec(context.Background(), `
		INSERT INTO leaderboard_awards (id, week_id, user_id, tier, select_before)
		VALUES ($1, $2, $3, 5, $4)`, awardID, weekID, user.ID, now.AddDate(0, 0, 7)); err != nil {
		t.Fatalf("insert leaderboard award: %v", err)
	}
	inventory, err := service.ListInventory(context.Background(), user.ID, "")
	if err != nil {
		t.Fatalf("ListInventory() error = %v", err)
	}
	if len(inventory.LeaderboardAwards) != 1 || len(inventory.LeaderboardAwards[0].Options) != 3 {
		t.Fatalf("leaderboard awards = %+v", inventory.LeaderboardAwards)
	}
	key := uuid.New()
	selected, err := service.SelectAward(
		context.Background(), user.ID, awardID, key, "leaderboard_5_autoteka",
	)
	if err != nil {
		t.Fatalf("SelectAward() error = %v", err)
	}
	if selected.Code != "leaderboard_5_autoteka" || selected.Source != rewards.SourceLeaderboard {
		t.Fatalf("selected reward = %+v", selected)
	}
	repeated, err := service.SelectAward(
		context.Background(), user.ID, awardID, key, "leaderboard_5_autoteka",
	)
	if err != nil {
		t.Fatalf("repeated SelectAward() error = %v", err)
	}
	if repeated.ID != selected.ID {
		t.Fatalf("repeated reward id = %s, want %s", repeated.ID, selected.ID)
	}
}

func createPet(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) uuid.UUID {
	t.Helper()
	petID := uuid.Nil
	if err := pool.QueryRow(
		context.Background(),
		`INSERT INTO pets (user_id, name) VALUES ($1, 'Тестовый питомец') RETURNING id`,
		userID,
	).Scan(&petID); err != nil {
		t.Fatalf("create pet: %v", err)
	}
	return petID
}

func insertPetAction(
	t *testing.T,
	pool *pgxpool.Pool,
	userID uuid.UUID,
	petID uuid.UUID,
	experience int,
	occurredAt time.Time,
) {
	t.Helper()
	if _, err := pool.Exec(
		context.Background(),
		`INSERT INTO pet_actions (
			user_id, pet_id, activity_code, experience_awarded, idempotency_key, occurred_at
		) VALUES ($1, $2, 'feed', $3, $4, $5)`,
		userID,
		petID,
		experience,
		uuid.New(),
		occurredAt,
	); err != nil {
		t.Fatalf("insert pet action: %v", err)
	}
}

var _ leaderboard.Repository = (*storage.LeaderboardRepository)(nil)
