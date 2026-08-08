package leaderboard

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCurrentPeriod(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 6, 18, 45, 0, 0, time.UTC)
	period, err := CurrentPeriod(now)
	if err != nil {
		t.Fatalf("CurrentPeriod() error = %v", err)
	}

	assertTimeEqual(t, period.StartsAt, time.Date(2026, time.August, 3, 0, 0, 0, 0, period.StartsAt.Location()))
	assertTimeEqual(t, period.EndsAt, time.Date(2026, time.August, 10, 0, 0, 0, 0, period.EndsAt.Location()))
	if period.Timezone != moscowTimezone {
		t.Fatalf("Timezone = %q, want %q", period.Timezone, moscowTimezone)
	}
}

func TestCurrentPeriodAtMondayBoundary(t *testing.T) {
	t.Parallel()

	moscow, err := time.LoadLocation(moscowTimezone)
	if err != nil {
		t.Fatalf("load Moscow timezone: %v", err)
	}

	now := time.Date(2026, time.August, 10, 0, 0, 0, 0, moscow)
	period, err := CurrentPeriod(now)
	if err != nil {
		t.Fatalf("CurrentPeriod() error = %v", err)
	}

	assertTimeEqual(t, period.StartsAt, now)
}

func TestRankUsesCompetitionPlacesAndStableTieOrder(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	participants := []Participant{
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000005"), WeeklyExperience: 0, ReachedAt: baseTime},
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000004"), WeeklyExperience: 80, ReachedAt: baseTime},
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), WeeklyExperience: 100, ReachedAt: baseTime.Add(time.Minute)},
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WeeklyExperience: 120, ReachedAt: baseTime},
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), WeeklyExperience: 100, ReachedAt: baseTime},
	}

	entries := Rank(participants)
	if len(entries) != 4 {
		t.Fatalf("len(Rank()) = %d, want 4", len(entries))
	}

	wantUserIDs := []uuid.UUID{participants[3].UserID, participants[4].UserID, participants[2].UserID, participants[1].UserID}
	wantRanks := []int{1, 2, 2, 4}
	for index := range entries {
		if entries[index].UserID != wantUserIDs[index] {
			t.Errorf("entry %d user = %s, want %s", index, entries[index].UserID, wantUserIDs[index])
		}
		if entries[index].Rank != wantRanks[index] {
			t.Errorf("entry %d rank = %d, want %d", index, entries[index].Rank, wantRanks[index])
		}
	}
}

func TestRankKeepsBoundaryTieInHigherPrizeTier(t *testing.T) {
	t.Parallel()

	participants := make([]Participant, 100)
	for index := range participants {
		participants[index] = Participant{
			UserID:           uuid.MustParse("00000000-0000-0000-0000-" + formatUUIDSuffix(index+1)),
			WeeklyExperience: int64(1000 - index),
			ReachedAt:        time.Unix(int64(index), 0),
		}
	}

	participants[5].WeeklyExperience = participants[4].WeeklyExperience
	entries := Rank(participants)

	if entries[4].Rank != 5 || entries[5].Rank != 5 {
		t.Fatalf("boundary tie ranks = %d and %d, want 5 and 5", entries[4].Rank, entries[5].Rank)
	}
	if entries[4].PrizeTier != 5 || entries[5].PrizeTier != 5 {
		t.Fatalf("boundary tie tiers = %d and %d, want 5 and 5", entries[4].PrizeTier, entries[5].PrizeTier)
	}
	if entries[6].PrizeTier != 10 {
		t.Fatalf("place %d tier = %d, want 10", entries[6].Rank, entries[6].PrizeTier)
	}
}

func TestRankDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	participants := []Participant{
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WeeklyExperience: 10},
		{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), WeeklyExperience: 20},
	}

	_ = Rank(participants)
	if participants[0].WeeklyExperience != 10 || participants[1].WeeklyExperience != 20 {
		t.Fatal("Rank() mutated its input")
	}
}

func formatUUIDSuffix(value int) string {
	return fmt.Sprintf("%012d", value)
}

func assertTimeEqual(t *testing.T, got, want time.Time) {
	t.Helper()

	if !got.Equal(want) {
		t.Fatalf("time = %s, want %s", got, want)
	}
}
