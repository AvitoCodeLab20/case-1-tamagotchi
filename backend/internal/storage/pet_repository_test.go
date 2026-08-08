package storage_test

import (
	"context"
	"errors"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/storage"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestPetRepositoryByUserID(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewPetRepository(pool)

	user := createUser(t, pool)

	got, err := repository.ByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() error = %v", err)
	}

	if got.UserID != user.ID {
		t.Errorf("UserID = %s, want %s", got.UserID, user.ID)
	}

	if got.ID == uuid.Nil {
		t.Error("ID = nil UUID")
	}
}
func TestPetRepositoryMissingRow(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewPetRepository(pool)

	_, err := repository.ByUserID(context.Background(), uuid.New())
	if !errors.Is(err, pet.ErrNotFound) {
		t.Errorf("ByUserID() error = %v, want ErrNotFound", err)
	}
}

func TestPetAutomaticallyCreatedWithUser(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewPetRepository(pool)

	user := createUser(t, pool)

	got, err := repository.ByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() error = %v", err)
	}

	if got.UserID != user.ID {
		t.Errorf("UserID = %s, want %s", got.UserID, user.ID)
	}

	if got.Name != "Авитоша" {
		t.Errorf("Name = %q, want %q", got.Name, "Авитоша")
	}

	if got.Species != "avito_pet" {
		t.Errorf("Species = %q, want %q", got.Species, "avito_pet")
	}

	if got.Level != 1 {
		t.Errorf("Level = %d, want 1", got.Level)
	}

	if got.Experience != 0 {
		t.Errorf("Experience = %d, want 0", got.Experience)
	}

	if got.Health != 100 {
		t.Errorf("Health = %d, want 100", got.Health)
	}

	if got.Hunger != 100 {
		t.Errorf("Hunger = %d, want 100", got.Hunger)
	}

	if got.Happiness != 100 {
		t.Errorf("Happiness = %d, want 100", got.Happiness)
	}

	if got.Energy != 100 {
		t.Errorf("Energy = %d, want 100", got.Energy)
	}
}

func TestPetRepositoryApplyAction(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewPetRepository(pool)

	user := createUser(t, pool)

	got, err := repository.ApplyAction(
		context.Background(),
		user.ID,
		pet.ApplyActionParams{
			Experience: 10,
			Hunger:     10,
			Energy:     -30,
			OccurredAt: time.Now().UTC(),
		},
	)
	if err != nil {
		t.Fatalf("ApplyAction() error = %v", err)
	}

	if got.Experience != 10 {
		t.Errorf("Experience = %d, want 10", got.Experience)
	}

	if got.Level != 1 {
		t.Errorf("Level = %d, want 1", got.Level)
	}

	if got.Hunger != 100 {
		t.Errorf("Hunger = %d, want 100", got.Hunger)
	}

	if got.Energy != 70 {
		t.Errorf("Energy = %d, want 70", got.Energy)
	}

	if got.StateVersion != 2 {
		t.Errorf("StateVersion = %d, want 2", got.StateVersion)
	}
}

func TestPetRepositoryApplyActionLevelUp(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewPetRepository(pool)

	user := createUser(t, pool)

	_, err := pool.Exec(
		context.Background(),
		`
			UPDATE pets
			SET experience = 95
			WHERE user_id = $1
		`,
		user.ID,
	)
	if err != nil {
		t.Fatalf("prepare pet: %v", err)
	}

	got, err := repository.ApplyAction(
		context.Background(),
		user.ID,
		pet.ApplyActionParams{
			Experience: 10,
			OccurredAt: time.Now().UTC(),
		},
	)
	if err != nil {
		t.Fatalf("ApplyAction() error = %v", err)
	}

	if got.Experience != 105 {
		t.Errorf("Experience = %d, want 105", got.Experience)
	}

	if got.Level != 2 {
		t.Errorf("Level = %d, want 2", got.Level)
	}
}
