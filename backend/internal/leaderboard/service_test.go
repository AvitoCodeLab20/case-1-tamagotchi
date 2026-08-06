package leaderboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repositoryStub struct {
	participants []Participant
	err          error
	startsAt     time.Time
	endsAt       time.Time
}

func (stub *repositoryStub) WeeklyParticipants(_ context.Context, startsAt, endsAt time.Time) ([]Participant, error) {
	stub.startsAt = startsAt
	stub.endsAt = endsAt

	return stub.participants, stub.err
}

func TestCurrentBuildsLimitedBoardAndKeepsCurrentUser(t *testing.T) {
	t.Parallel()

	currentUserID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	repository := &repositoryStub{participants: []Participant{
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WeeklyExperience: 300},
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), WeeklyExperience: 200},
		{UserID: currentUserID, WeeklyExperience: 100},
	}}
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time {
		return time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	}

	board, err := service.Current(context.Background(), currentUserID, 2)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	if board.ParticipantsCount != 3 || len(board.Entries) != 2 {
		t.Fatalf("participants = %d, entries = %d, want 3 and 2", board.ParticipantsCount, len(board.Entries))
	}
	if board.Me == nil || board.Me.UserID != currentUserID || board.Me.Rank != 3 {
		t.Fatalf("Me = %+v, want current user at rank 3", board.Me)
	}
	if repository.startsAt.Weekday() != time.Monday || !repository.startsAt.Before(repository.endsAt) {
		t.Fatalf("repository period = [%s, %s), want a Monday week", repository.startsAt, repository.endsAt)
	}
}

func TestCurrentCalculatesExperienceToNextTier(t *testing.T) {
	t.Parallel()

	currentUserID := uuid.MustParse("00000000-0000-0000-0000-000000000020")
	participants := make([]Participant, 100)
	for index := range participants {
		participants[index] = Participant{
			UserID:           uuid.MustParse("00000000-0000-0000-0000-" + formatUUIDSuffix(index+1)),
			WeeklyExperience: int64(1000 - index*10),
		}
	}

	service, err := NewService(&repositoryStub{participants: participants})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	board, err := service.Current(context.Background(), currentUserID, DefaultLimit)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	if board.Me == nil || board.Me.PrizeTier != 0 {
		t.Fatalf("Me = %+v, want a non-prize participant", board.Me)
	}
	if board.NextTier == nil || board.NextTier.Tier != 15 || board.NextTier.ExperienceRequired != 50 {
		t.Fatalf("NextTier = %+v, want 50 XP to top 15%%", board.NextTier)
	}
}

func TestCurrentWrapsRepositoryError(t *testing.T) {
	t.Parallel()

	repositoryError := errors.New("database unavailable")
	service, err := NewService(&repositoryStub{err: repositoryError})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = service.Current(context.Background(), uuid.New(), DefaultLimit)
	if !errors.Is(err, repositoryError) {
		t.Fatalf("Current() error = %v, want wrapped repository error", err)
	}
}

func TestNewServiceRequiresRepository(t *testing.T) {
	t.Parallel()

	if _, err := NewService(nil); err == nil {
		t.Fatal("NewService(nil) error = nil")
	}
}

func TestCurrentRejectsInvalidLimit(t *testing.T) {
	t.Parallel()

	service, err := NewService(&repositoryStub{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, err = service.Current(context.Background(), uuid.New(), 0); !errors.Is(err, ErrInvalidLimit) {
		t.Fatalf("Current() error = %v, want ErrInvalidLimit", err)
	}
}
