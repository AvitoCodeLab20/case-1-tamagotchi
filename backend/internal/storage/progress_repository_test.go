package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/progress"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/storage"
)

func TestProgressRepositoryByUserID(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewProgressRepository(pool)

	user := createUser(t, pool)

	got, err := repository.ByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() error = %v", err)
	}

	if got.Level != 1 {
		t.Errorf("Level = %d, want 1", got.Level)
	}

	if got.Experience != 0 {
		t.Errorf("Experience = %d, want 0", got.Experience)
	}

	if got.RequiredTotalExperience != 0 {
		t.Errorf(
			"RequiredTotalExperience = %d, want 0",
			got.RequiredTotalExperience,
		)
	}

	if got.Title != "Новичок" {
		t.Errorf("Title = %q, want %q", got.Title, "Новичок")
	}

	if got.UserStreak.CurrentDays != 0 {
		t.Errorf("CurrentDays = %d, want 0", got.UserStreak.CurrentDays)
	}

	if got.UserStreak.LongestDays != 0 {
		t.Errorf("LongestDays = %d, want 0", got.UserStreak.LongestDays)
	}

	if got.UserStreak.LastActiveDate != nil {
		t.Errorf(
			"LastActiveDate = %v, want nil",
			got.UserStreak.LastActiveDate,
		)
	}
}

func TestProgressRepositoryByUserIDWithStreak(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewProgressRepository(pool)

	user := createUser(t, pool)

	_, err := pool.Exec(
		context.Background(),
		`
			INSERT INTO user_streaks (
				user_id,
				current_days,
				longest_days,
				last_active_date
			)
			VALUES ($1, $2, $3, $4)
		`,
		user.ID,
		3,
		5,
		time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("insert user streak: %v", err)
	}

	got, err := repository.ByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() error = %v", err)
	}

	if got.UserStreak.CurrentDays != 3 {
		t.Errorf(
			"CurrentDays = %d, want 3",
			got.UserStreak.CurrentDays,
		)
	}

	if got.UserStreak.LongestDays != 5 {
		t.Errorf(
			"LongestDays = %d, want 5",
			got.UserStreak.LongestDays,
		)
	}

	if got.UserStreak.LastActiveDate == nil {
		t.Fatal("LastActiveDate = nil")
	}

	if got := got.UserStreak.LastActiveDate.Format(time.DateOnly); got != "2026-08-08" {
		t.Errorf("LastActiveDate = %s, want 2026-08-08", got)
	}
}

func TestProgressRepositoryMissingUser(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewProgressRepository(pool)

	_, err := repository.ByUserID(
		context.Background(),
		uuid.New(),
	)

	if !errors.Is(err, progress.ErrNotFound) {
		t.Errorf(
			"ByUserID() error = %v, want ErrNotFound",
			err,
		)
	}
}
