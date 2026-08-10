package dailysummary

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repositoryStub struct {
	result DailySummary
	err    error

	userID      uuid.UUID
	summaryDate time.Time
}

func (stub *repositoryStub) ByUserIDAndDate(
	_ context.Context,
	userID uuid.UUID,
	summaryDate time.Time,
) (DailySummary, error) {
	stub.userID = userID
	stub.summaryDate = summaryDate

	return stub.result, stub.err
}

func TestCurrent(t *testing.T) {
	userID := uuid.New()
	want := DailySummary{ID: 42}
	repository := &repositoryStub{result: want}
	service := NewService(repository)
	service.now = func() time.Time {
		return time.Date(2026, time.August, 9, 23, 30, 0, 0, time.UTC)
	}

	got, err := service.Current(context.Background(), userID)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	if got.ID != want.ID {
		t.Errorf("ID = %d, want %d", got.ID, want.ID)
	}

	if repository.userID != userID {
		t.Errorf("UserID = %s, want %s", repository.userID, userID)
	}

	wantDate := time.Date(2026, time.August, 9, 0, 0, 0, 0, time.UTC)
	if !repository.summaryDate.Equal(wantDate) {
		t.Errorf(
			"SummaryDate = %s, want %s",
			repository.summaryDate,
			wantDate,
		)
	}
}

func TestByDate(t *testing.T) {
	userID := uuid.New()
	wantDate := time.Date(2026, time.August, 8, 0, 0, 0, 0, time.UTC)
	want := DailySummary{ID: 42}
	repository := &repositoryStub{result: want}
	service := NewService(repository)

	got, err := service.ByDate(context.Background(), userID, wantDate)
	if err != nil {
		t.Fatalf("ByDate() error = %v", err)
	}

	if got.ID != want.ID {
		t.Errorf("ID = %d, want %d", got.ID, want.ID)
	}
	if repository.userID != userID {
		t.Errorf("UserID = %s, want %s", repository.userID, userID)
	}
	if !repository.summaryDate.Equal(wantDate) {
		t.Errorf(
			"SummaryDate = %s, want %s",
			repository.summaryDate,
			wantDate,
		)
	}
}
