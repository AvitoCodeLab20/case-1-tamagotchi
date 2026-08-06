package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/leaderboard"
)

var ErrUserNotRegistered = errors.New("leaderboard: user is not registered")

type user struct {
	displayName string
	active      bool
}

type experienceEvent struct {
	userID     uuid.UUID
	experience int64
	occurredAt time.Time
}

type Repository struct {
	mutex  sync.RWMutex
	users  map[uuid.UUID]user
	events []experienceEvent
}

func NewRepository() *Repository {
	return &Repository{users: make(map[uuid.UUID]user)}
}

func (repository *Repository) SetUser(userID uuid.UUID, displayName string, active bool) {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	repository.users[userID] = user{displayName: displayName, active: active}
}

func (repository *Repository) AddExperience(userID uuid.UUID, experience int64, occurredAt time.Time) error {
	repository.mutex.Lock()
	defer repository.mutex.Unlock()
	if _, exists := repository.users[userID]; !exists {
		return ErrUserNotRegistered
	}
	if experience <= 0 {
		return nil
	}
	repository.events = append(repository.events, experienceEvent{
		userID:     userID,
		experience: experience,
		occurredAt: occurredAt,
	})
	return nil
}

func (repository *Repository) WeeklyParticipants(
	ctx context.Context,
	startsAt time.Time,
	endsAt time.Time,
) ([]leaderboard.Participant, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	repository.mutex.RLock()
	defer repository.mutex.RUnlock()
	participants := make(map[uuid.UUID]leaderboard.Participant)
	for _, event := range repository.events {
		if event.occurredAt.Before(startsAt) || !event.occurredAt.Before(endsAt) {
			continue
		}
		currentUser, exists := repository.users[event.userID]
		if !exists || !currentUser.active {
			continue
		}
		participant := participants[event.userID]
		participant.UserID = event.userID
		participant.DisplayName = currentUser.displayName
		participant.WeeklyExperience += event.experience
		if event.occurredAt.After(participant.ReachedAt) {
			participant.ReachedAt = event.occurredAt
		}
		participants[event.userID] = participant
	}
	result := make([]leaderboard.Participant, 0, len(participants))
	for _, participant := range participants {
		result = append(result, participant)
	}
	return result, nil
}

var _ leaderboard.Repository = (*Repository)(nil)
