package activity

import "errors"

var ErrActivityTypeNotFound = errors.New("activity type not found")

type ActivityType struct {
	Code            string
	Title           string
	Description     string
	Category        string
	BaseExperience  int
	DailyLimit      *int
	CooldownSeconds int
	IsActive        bool
}
