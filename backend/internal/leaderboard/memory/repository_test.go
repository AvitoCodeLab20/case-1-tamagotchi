package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRepositoryAggregatesWeeklyExperience(t *testing.T) {
	t.Parallel()
	repository := NewRepository()
	activeUserID := uuid.New()
	inactiveUserID := uuid.New()
	repository.SetUser(activeUserID, "Active", true)
	repository.SetUser(inactiveUserID, "Inactive", false)
	startsAt := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	endsAt := startsAt.AddDate(0, 0, 7)
	addExperience(t, repository, activeUserID, 20, startsAt.Add(time.Hour))
	addExperience(t, repository, activeUserID, 30, startsAt.Add(2*time.Hour))
	addExperience(t, repository, activeUserID, 100, startsAt.Add(-time.Second))
	addExperience(t, repository, inactiveUserID, 500, startsAt.Add(time.Hour))
	participants, err := repository.WeeklyParticipants(context.Background(), startsAt, endsAt)
	if err != nil {
		t.Fatalf("WeeklyParticipants() error = %v", err)
	}
	if len(participants) != 1 {
		t.Fatalf("len(participants) = %d, want 1", len(participants))
	}
	participant := participants[0]
	if participant.UserID != activeUserID || participant.WeeklyExperience != 50 {
		t.Fatalf("participant = %+v, want active user with 50 XP", participant)
	}
	if !participant.ReachedAt.Equal(startsAt.Add(2 * time.Hour)) {
		t.Fatalf("ReachedAt = %s, want latest included event", participant.ReachedAt)
	}
}

func TestRepositoryUsesCurrentUserState(t *testing.T) {
	t.Parallel()
	repository := NewRepository()
	userID := uuid.New()
	repository.SetUser(userID, "Old name", true)
	addExperience(t, repository, userID, 25, time.Now())
	repository.SetUser(userID, "New name", false)
	participants, err := repository.WeeklyParticipants(
		context.Background(),
		time.Now().Add(-time.Hour),
		time.Now().Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("WeeklyParticipants() error = %v", err)
	}
	if len(participants) != 0 {
		t.Fatalf("len(participants) = %d, want 0 for inactive user", len(participants))
	}
}

func TestRepositoryRejectsUnknownUser(t *testing.T) {
	t.Parallel()
	repository := NewRepository()
	err := repository.AddExperience(uuid.New(), 10, time.Now())
	if !errors.Is(err, ErrUserNotRegistered) {
		t.Fatalf("AddExperience() error = %v, want ErrUserNotRegistered", err)
	}
}

func TestRepositoryRespectsCancelledContext(t *testing.T) {
	t.Parallel()
	repository := NewRepository()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := repository.WeeklyParticipants(ctx, time.Now(), time.Now().Add(time.Hour))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WeeklyParticipants() error = %v, want context.Canceled", err)
	}
}

func addExperience(
	t *testing.T,
	repository *Repository,
	userID uuid.UUID,
	experience int64,
	occurredAt time.Time,
) {
	t.Helper()
	if err := repository.AddExperience(userID, experience, occurredAt); err != nil {
		t.Fatalf("AddExperience() error = %v", err)
	}
}
