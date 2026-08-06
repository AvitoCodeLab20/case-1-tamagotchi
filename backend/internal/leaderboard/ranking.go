package leaderboard

import (
	"errors"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

const (
	moscowTimezone = "Europe/Moscow"
	daysInWeek     = 7
)

var ErrMoscowTimezoneUnavailable = errors.New("leaderboard: Europe/Moscow timezone is unavailable")

type Participant struct {
	UserID           uuid.UUID
	DisplayName      string
	WeeklyExperience int64
	ReachedAt        time.Time
}

type Entry struct {
	Participant
	Rank      int
	PrizeTier int
}

type Period struct {
	StartsAt time.Time
	EndsAt   time.Time
	Timezone string
}

func CurrentPeriod(now time.Time) (Period, error) {
	location, err := time.LoadLocation(moscowTimezone)
	if err != nil {
		return Period{}, ErrMoscowTimezoneUnavailable
	}

	localNow := now.In(location)
	daysSinceMonday := (int(localNow.Weekday()) + daysInWeek - int(time.Monday)) % daysInWeek
	startOfToday := time.Date(
		localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location,
	)
	startsAt := startOfToday.AddDate(0, 0, -daysSinceMonday)

	return Period{
		StartsAt: startsAt,
		EndsAt:   startsAt.AddDate(0, 0, daysInWeek),
		Timezone: moscowTimezone,
	}, nil
}

func Rank(participants []Participant) []Entry {
	eligible := make([]Participant, 0, len(participants))
	for _, participant := range participants {
		if participant.WeeklyExperience > 0 {
			eligible = append(eligible, participant)
		}
	}

	sort.SliceStable(eligible, func(leftIndex, rightIndex int) bool {
		left := eligible[leftIndex]
		right := eligible[rightIndex]

		if left.WeeklyExperience != right.WeeklyExperience {
			return left.WeeklyExperience > right.WeeklyExperience
		}
		if !left.ReachedAt.Equal(right.ReachedAt) {
			return left.ReachedAt.Before(right.ReachedAt)
		}

		return left.UserID.String() < right.UserID.String()
	})

	entries := make([]Entry, len(eligible))
	currentRank := 0
	for index, participant := range eligible {
		if index == 0 || participant.WeeklyExperience != eligible[index-1].WeeklyExperience {
			currentRank = index + 1
		}

		entries[index] = Entry{
			Participant: participant,
			Rank:        currentRank,
			PrizeTier:   prizeTier(currentRank, len(eligible)),
		}
	}

	return entries
}

func prizeTier(rank, participantCount int) int {
	if rank <= 0 || participantCount <= 0 {
		return 0
	}

	switch {
	case rank <= percentageBoundary(participantCount, 5):
		return 5
	case rank <= percentageBoundary(participantCount, 10):
		return 10
	case rank <= percentageBoundary(participantCount, 15):
		return 15
	default:
		return 0
	}
}

func percentageBoundary(participantCount, percentage int) int {
	return int(math.Ceil(float64(participantCount*percentage) / 100))
}
