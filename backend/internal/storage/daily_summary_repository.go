package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/dailysummary"
)

type DailySummaryRepository struct {
	db Querier
}

func NewDailySummaryRepository(
	db Querier,
) *DailySummaryRepository {
	return &DailySummaryRepository{
		db: db,
	}
}

func (repository *DailySummaryRepository) ByUserIDAndDate(
	ctx context.Context,
	userID uuid.UUID,
	summaryDate time.Time,
) (dailysummary.DailySummary, error) {
	const query = `
		SELECT
			id,
			summary_date,
			action_count,
			experience_earned,
			levels_gained,
			state_before,
			state_after,
			generated_at
		FROM daily_summaries
		WHERE user_id = $1
		  AND summary_date = $2::date
	`

	var result dailysummary.DailySummary
	var stateBeforeJSON []byte
	var stateAfterJSON []byte

	err := repository.db.QueryRow(
		ctx,
		query,
		userID,
		summaryDate,
	).Scan(
		&result.ID,
		&result.SummaryDate,
		&result.ActionCount,
		&result.ExperienceEarned,
		&result.LevelsGained,
		&stateBeforeJSON,
		&stateAfterJSON,
		&result.GeneratedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dailysummary.DailySummary{},
				dailysummary.ErrNotFound
		}

		return dailysummary.DailySummary{},
			fmt.Errorf("select daily summary: %w", err)
	}

	if err := json.Unmarshal(
		stateBeforeJSON,
		&result.StateBefore,
	); err != nil {
		return dailysummary.DailySummary{},
			fmt.Errorf("decode state before: %w", err)
	}

	if err := json.Unmarshal(
		stateAfterJSON,
		&result.StateAfter,
	); err != nil {
		return dailysummary.DailySummary{},
			fmt.Errorf("decode state after: %w", err)
	}

	return result, nil
}

func (repository *DailySummaryRepository) UpsertAction(
	ctx context.Context,
	params dailysummary.UpsertParams,
) error {
	stateBeforeJSON, err := json.Marshal(params.StateBefore)
	if err != nil {
		return fmt.Errorf("encode state before: %w", err)
	}

	stateAfterJSON, err := json.Marshal(params.StateAfter)
	if err != nil {
		return fmt.Errorf("encode state after: %w", err)
	}

	const query = `
		INSERT INTO daily_summaries (
			user_id,
			summary_date,
			action_count,
			experience_earned,
			levels_gained,
			state_before,
			state_after,
			generated_at
		)
		VALUES (
			$1,
			$2::date,
			1,
			$3,
			$4,
			$5::jsonb,
			$6::jsonb,
			$7
		)
		ON CONFLICT (user_id, summary_date)
		DO UPDATE SET
			action_count =
				daily_summaries.action_count + 1,
			experience_earned =
				daily_summaries.experience_earned
				+ EXCLUDED.experience_earned,
			levels_gained =
				daily_summaries.levels_gained
				+ EXCLUDED.levels_gained,
			state_after = EXCLUDED.state_after,
			generated_at = EXCLUDED.generated_at
	`

	_, err = repository.db.Exec(
		ctx,
		query,
		params.UserID,
		params.SummaryDate,
		params.ExperienceEarned,
		params.LevelsGained,
		stateBeforeJSON,
		stateAfterJSON,
		params.GeneratedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert daily summary: %w", err)
	}

	return nil
}
