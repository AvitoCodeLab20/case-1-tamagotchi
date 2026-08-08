package activity

import (
	"errors"
	"time"
)

var ErrTypeNotFound = errors.New("activity type not found")

type Type struct {
	Code            string
	Title           string
	Description     string
	Category        string
	BaseExperience  int
	DailyLimit      *int
	CooldownSeconds int
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
