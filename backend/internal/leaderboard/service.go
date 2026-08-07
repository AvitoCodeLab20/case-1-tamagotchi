package leaderboard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const DefaultLimit = 50

var ErrInvalidLimit = errors.New("leaderboard: limit must be positive")

type Repository interface {
	WeeklyParticipants(ctx context.Context, startsAt, endsAt time.Time) ([]Participant, error)
}

type Board struct {
	Period            Period
	Status            string
	ParticipantsCount int
	Entries           []Entry
	Me                *Entry
	NextTier          *NextTierProgress
}

type NextTierProgress struct {
	Tier               int
	ExperienceRequired int64
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("leaderboard: repository is required")
	}

	return &Service{repository: repository, now: time.Now}, nil
}

func (service *Service) Current(ctx context.Context, userID uuid.UUID, limit int) (Board, error) {
	if limit < 1 {
		return Board{}, ErrInvalidLimit
	}

	period, err := CurrentPeriod(service.now())
	if err != nil {
		return Board{}, err
	}

	participants, err := service.repository.WeeklyParticipants(ctx, period.StartsAt, period.EndsAt)
	if err != nil {
		return Board{}, fmt.Errorf("load weekly participants: %w", err)
	}

	allEntries := Rank(participants)
	visibleCount := min(limit, len(allEntries))
	visibleEntries := make([]Entry, visibleCount)
	copy(visibleEntries, allEntries[:visibleCount])

	board := Board{
		Period:            period,
		Status:            "in_progress",
		ParticipantsCount: len(allEntries),
		Entries:           visibleEntries,
	}

	for index := range allEntries {
		if allEntries[index].UserID == userID {
			currentUser := allEntries[index]
			board.Me = &currentUser
			board.NextTier = nextTierProgress(currentUser, allEntries)

			break
		}
	}

	return board, nil
}

func nextTierProgress(currentUser Entry, entries []Entry) *NextTierProgress {
	nextTier := 0
	switch currentUser.PrizeTier {
	case 0:
		nextTier = 15
	case 15:
		nextTier = 10
	case 10:
		nextTier = 5
	case 5:
		return nil
	}

	boundary := percentageBoundary(len(entries), nextTier)
	if boundary == 0 || boundary > len(entries) {
		return nil
	}

	required := entries[boundary-1].WeeklyExperience - currentUser.WeeklyExperience
	if required < 0 {
		required = 0
	}

	return &NextTierProgress{Tier: nextTier, ExperienceRequired: required}
}
