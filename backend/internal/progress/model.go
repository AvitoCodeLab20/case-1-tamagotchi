package progress

import "time"

type UserStreak struct {
	CurrentDays    int
	LongestDays    int
	LastActiveDate *time.Time
	UpdatedAt      time.Time
}

type Progress struct {
	Level                   int
	Experience              int64
	RequiredTotalExperience int64
	Title                   string
	UserStreak              UserStreak
}

type Level struct {
	Level                   int
	RequiredTotalExperience int64
	Title                   string
	CreatedAt               time.Time
}
