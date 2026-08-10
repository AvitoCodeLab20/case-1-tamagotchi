package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/activity"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/storage"
	"github.com/google/uuid"
)

func TestPetActionRepositoryCreate(t *testing.T) {
	pool := newPool(t)

	actionRepository := storage.NewPetActionRepository(pool)
	petRepository := storage.NewPetRepository(pool)

	user := createUser(t, pool)

	pet, err := petRepository.ByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() error = %v", err)
	}

	key := uuid.New()

	got, err := actionRepository.Create(
		context.Background(),
		activity.Action{
			UserID:            user.ID,
			PetID:             pet.ID,
			ActivityCode:      "feed",
			ExperienceAwarded: 10,
			StateDelta: activity.StateDelta{
				Hunger: 10,
			},
			IdempotencyKey: key,
			OccurredAt:     time.Now().UTC(),
		},
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got.ID == 0 {
		t.Error("ID = 0")
	}

	if got.UserID != user.ID {
		t.Errorf("UserID = %s, want %s", got.UserID, user.ID)
	}

	if got.PetID != pet.ID {
		t.Errorf("PetID = %s, want %s", got.PetID, pet.ID)
	}

	if got.ActivityCode != "feed" {
		t.Errorf("ActivityCode = %q, want %q", got.ActivityCode, "feed")
	}

	if got.ExperienceAwarded != 10 {
		t.Errorf("ExperienceAwarded = %d, want 10", got.ExperienceAwarded)
	}

	if got.IdempotencyKey != key {
		t.Errorf("IdempotencyKey = %s, want %s", got.IdempotencyKey, key)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestPetActionRepositoryByIdempotencyKey(t *testing.T) {
	pool := newPool(t)

	actionRepository := storage.NewPetActionRepository(pool)
	petRepository := storage.NewPetRepository(pool)

	user := createUser(t, pool)

	pet, err := petRepository.ByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() error = %v", err)
	}

	key := uuid.New()

	created, err := actionRepository.Create(
		context.Background(),
		activity.Action{
			UserID:            user.ID,
			PetID:             pet.ID,
			ActivityCode:      "feed",
			ExperienceAwarded: 10,
			StateDelta: activity.StateDelta{
				Hunger: 10,
			},
			IdempotencyKey: key,
			OccurredAt:     time.Now().UTC(),
		},
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := actionRepository.ByIdempotencyKey(
		context.Background(),
		user.ID,
		key,
	)
	if err != nil {
		t.Fatalf("ByIdempotencyKey() error = %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}

	if got.UserID != user.ID {
		t.Errorf("UserID = %s, want %s", got.UserID, user.ID)
	}

	if got.PetID != pet.ID {
		t.Errorf("PetID = %s, want %s", got.PetID, pet.ID)
	}

	if got.ActivityCode != "feed" {
		t.Errorf("ActivityCode = %q, want %q", got.ActivityCode, "feed")
	}

	if got.ExperienceAwarded != 10 {
		t.Errorf("ExperienceAwarded = %d, want 10", got.ExperienceAwarded)
	}

	if got.StateDelta.Hunger != 10 {
		t.Errorf("StateDelta.Hunger = %d, want 10", got.StateDelta.Hunger)
	}

	if got.IdempotencyKey != key {
		t.Errorf("IdempotencyKey = %s, want %s", got.IdempotencyKey, key)
	}
}

func TestPetActionRepositoryByIdempotencyKeyMissing(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewPetActionRepository(pool)

	user := createUser(t, pool)

	_, err := repository.ByIdempotencyKey(
		context.Background(),
		user.ID,
		uuid.New(),
	)

	if !errors.Is(err, activity.ErrActionNotFound) {
		t.Errorf(
			"ByIdempotencyKey() error = %v, want ErrActionNotFound",
			err,
		)
	}
}

func TestPetActionRepositoryLastAction(t *testing.T) {
	pool := newPool(t)
	actionRepository := storage.NewPetActionRepository(pool)
	petRepository := storage.NewPetRepository(pool)

	user := createUser(t, pool)

	pet, err := petRepository.ByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() error = %v", err)
	}

	now := time.Now().UTC()

	first, err := actionRepository.Create(context.Background(), activity.Action{
		UserID:            user.ID,
		PetID:             pet.ID,
		ActivityCode:      "feed",
		ExperienceAwarded: 10,
		StateDelta:        activity.StateDelta{Hunger: 10},
		IdempotencyKey:    uuid.New(),
		OccurredAt:        now.Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}

	second, err := actionRepository.Create(context.Background(), activity.Action{
		UserID:            user.ID,
		PetID:             pet.ID,
		ActivityCode:      "feed",
		ExperienceAwarded: 10,
		StateDelta:        activity.StateDelta{Hunger: 10},
		IdempotencyKey:    uuid.New(),
		OccurredAt:        now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}

	got, err := actionRepository.LastAction(
		context.Background(),
		user.ID,
		"feed",
	)
	if err != nil {
		t.Fatalf("LastAction() error = %v", err)
	}

	if got.ID != second.ID {
		t.Errorf("ID = %d, want %d", got.ID, second.ID)
	}

	if got.ID == first.ID {
		t.Error("LastAction() returned older action")
	}
}

func TestPetActionRepositoryCountActionsSince(t *testing.T) {
	pool := newPool(t)
	actionRepository := storage.NewPetActionRepository(pool)
	petRepository := storage.NewPetRepository(pool)

	user := createUser(t, pool)

	pet, err := petRepository.ByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() error = %v", err)
	}

	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)

	times := []time.Time{
		now.Add(-48 * time.Hour),
		now.Add(-12 * time.Hour),
		now.Add(-6 * time.Hour),
		now.Add(-time.Hour),
	}

	for _, occurredAt := range times {
		_, err := actionRepository.Create(context.Background(), activity.Action{
			UserID:            user.ID,
			PetID:             pet.ID,
			ActivityCode:      "feed",
			ExperienceAwarded: 10,
			StateDelta:        activity.StateDelta{Hunger: 10},
			IdempotencyKey:    uuid.New(),
			OccurredAt:        occurredAt,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	count, err := actionRepository.CountActionsSince(
		context.Background(),
		user.ID,
		"feed",
		since,
	)
	if err != nil {
		t.Fatalf("CountActionsSince() error = %v", err)
	}

	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}
