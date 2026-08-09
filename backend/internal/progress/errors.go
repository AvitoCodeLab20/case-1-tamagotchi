package progress

import "errors"

var (
	ErrNotFound       = errors.New("progress not found")
	ErrLevelNotFound  = errors.New("level not found")
	ErrStreakNotFound = errors.New("user streak not found")
)
