package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/activity"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/storage"
)

func TestActivityTypeRepositoryByCode(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewActivityTypeRepository(pool)

	got, err := repository.ByCode(context.Background(), "feed")
	if err != nil {
		t.Fatalf("ByCode() error = %v", err)
	}

	if got.Code != "feed" {
		t.Errorf("Code = %q, want %q", got.Code, "feed")
	}

	if got.Title != "Покормить" {
		t.Errorf("Title = %q, want %q", got.Title, "Покормить")
	}

	if got.Category != "care" {
		t.Errorf("Category = %q, want %q", got.Category, "care")
	}

	if got.BaseExperience != 10 {
		t.Errorf("BaseExperience = %d, want 10", got.BaseExperience)
	}

	if got.DailyLimit == nil {
		t.Fatal("DailyLimit = nil, want 3")
	}

	if *got.DailyLimit != 3 {
		t.Errorf("DailyLimit = %d, want 3", *got.DailyLimit)
	}

	if got.CooldownSeconds != 14400 {
		t.Errorf("CooldownSeconds = %d, want 14400", got.CooldownSeconds)
	}

	if !got.IsActive {
		t.Error("IsActive = false, want true")
	}
}

func TestActivityTypeRepositoryMissingRow(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewActivityTypeRepository(pool)

	_, err := repository.ByCode(
		context.Background(),
		"does_not_exist",
	)

	if !errors.Is(err, activity.ErrActivityTypeNotFound) {
		t.Errorf(
			"ByCode() error = %v, want ErrActivityTypeNotFound",
			err,
		)
	}
}
