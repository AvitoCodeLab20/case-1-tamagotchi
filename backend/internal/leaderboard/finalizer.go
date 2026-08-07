package leaderboard

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const awardSelectionLifetime = 7 * 24 * time.Hour

type FinalizationRepository interface {
	WeeklyParticipants(ctx context.Context, startsAt, endsAt time.Time) ([]Participant, error)
	SaveFinalizedWeek(
		ctx context.Context,
		period Period,
		entries []Entry,
		finalizedAt time.Time,
		selectBefore time.Time,
	) error
}

type Finalizer struct {
	repository FinalizationRepository
}

func NewFinalizer(repository FinalizationRepository) (*Finalizer, error) {
	if repository == nil {
		return nil, errors.New("leaderboard: finalization repository is required")
	}
	return &Finalizer{repository: repository}, nil
}

func (finalizer *Finalizer) FinalizePreviousWeek(ctx context.Context, now time.Time) error {
	currentPeriod, err := CurrentPeriod(now)
	if err != nil {
		return err
	}
	period := Period{
		StartsAt: currentPeriod.StartsAt.AddDate(0, 0, -daysInWeek),
		EndsAt:   currentPeriod.StartsAt,
		Timezone: currentPeriod.Timezone,
	}
	participants, err := finalizer.repository.WeeklyParticipants(ctx, period.StartsAt, period.EndsAt)
	if err != nil {
		return fmt.Errorf("load previous weekly participants: %w", err)
	}
	if err = finalizer.repository.SaveFinalizedWeek(
		ctx,
		period,
		Rank(participants),
		now,
		period.EndsAt.Add(awardSelectionLifetime),
	); err != nil {
		return fmt.Errorf("save finalized leaderboard week: %w", err)
	}
	return nil
}
