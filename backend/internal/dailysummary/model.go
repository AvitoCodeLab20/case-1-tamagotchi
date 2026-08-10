package dailysummary

import (
	"github.com/google/uuid"
	"time"
)

type PetState struct {
	Level      int   `json:"level"`
	Experience int64 `json:"experience"`
	Health     int   `json:"health"`
	Hunger     int   `json:"hunger"`
	Happiness  int   `json:"happiness"`
	Energy     int   `json:"energy"`
}

type UpsertParams struct {
	UserID           uuid.UUID
	SummaryDate      time.Time
	ExperienceEarned int
	LevelsGained     int
	StateBefore      PetState
	StateAfter       PetState
	GeneratedAt      time.Time
}

type DailySummary struct {
	ID               int64
	SummaryDate      time.Time
	ActionCount      int
	ExperienceEarned int
	LevelsGained     int
	StateBefore      PetState
	StateAfter       PetState
	GeneratedAt      time.Time
}
