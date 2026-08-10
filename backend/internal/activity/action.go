package activity

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/progress"
)

var (
	ErrActionNotFound      = errors.New("action not found")
	ErrIdempotencyConflict = errors.New("idempotency key belongs to another action")
)

type StateDelta struct {
	Health    int `json:"health,omitempty"`
	Hunger    int `json:"hunger,omitempty"`
	Happiness int `json:"happiness,omitempty"`
	Energy    int `json:"energy,omitempty"`
}

type Action struct {
	ID                int64
	UserID            uuid.UUID
	PetID             uuid.UUID
	ActivityCode      string
	ExperienceAwarded int
	StateDelta        StateDelta
	IdempotencyKey    uuid.UUID
	OccurredAt        time.Time
	CreatedAt         time.Time
}

type PerformActionResult struct {
	Action     Action
	Pet        pet.Pet
	Level      progress.Level
	UserStreak progress.UserStreak
}
