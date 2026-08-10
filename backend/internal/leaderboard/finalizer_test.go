package leaderboard

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type finalizationRepositoryStub struct {
	participants []Participant
	period       Period
	entries      []Entry
	finalizedAt  time.Time
	selectBefore time.Time
}

func (stub *finalizationRepositoryStub) WeeklyParticipants(
	_ context.Context,
	startsAt time.Time,
	endsAt time.Time,
) ([]Participant, error) {
	stub.period.StartsAt = startsAt
	stub.period.EndsAt = endsAt
	return stub.participants, nil
}

func (stub *finalizationRepositoryStub) SaveFinalizedWeek(
	_ context.Context,
	period Period,
	entries []Entry,
	finalizedAt time.Time,
	selectBefore time.Time,
) error {
	stub.period = period
	stub.entries = entries
	stub.finalizedAt = finalizedAt
	stub.selectBefore = selectBefore
	return nil
}

func TestFinalizerFinalizesPreviousMoscowWeek(t *testing.T) {
	repository := &finalizationRepositoryStub{participants: []Participant{
		{UserID: uuid.New(), DisplayName: "Игрок", WeeklyExperience: 100},
	}}
	finalizer, err := NewFinalizer(repository)
	if err != nil {
		t.Fatalf("NewFinalizer() error = %v", err)
	}
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	if err = finalizer.FinalizePreviousWeek(context.Background(), now); err != nil {
		t.Fatalf("FinalizePreviousWeek() error = %v", err)
	}
	if repository.period.Timezone != moscowTimezone || repository.period.StartsAt.Weekday() != time.Monday {
		t.Fatalf("period = %+v", repository.period)
	}
	if !repository.period.EndsAt.Equal(repository.period.StartsAt.AddDate(0, 0, daysInWeek)) {
		t.Fatalf("period duration = %s", repository.period.EndsAt.Sub(repository.period.StartsAt))
	}
	if len(repository.entries) != 1 || repository.entries[0].PrizeTier != 5 {
		t.Fatalf("entries = %+v", repository.entries)
	}
	if !repository.finalizedAt.Equal(now) {
		t.Fatalf("finalizedAt = %s, want %s", repository.finalizedAt, now)
	}
	if !repository.selectBefore.Equal(repository.period.EndsAt.Add(awardSelectionLifetime)) {
		t.Fatalf("selectBefore = %s", repository.selectBefore)
	}
}
