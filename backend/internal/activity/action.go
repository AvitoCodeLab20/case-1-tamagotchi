package activity

import (
	"time"
	"errors"
	"github.com/google/uuid"
)

var ErrActionNotFound = errors.New("action not found")

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
