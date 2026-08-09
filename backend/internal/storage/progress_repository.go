package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/progress"
)

type ProgressRepository struct {
	db Querier
}

func NewProgressRepository(db Querier) *ProgressRepository {
	return &ProgressRepository{db: db}
}

func (repository *ProgressRepository) scanStreak(
	row pgx.Row,
	operation string,
) (progress.UserStreak, error) {
	var result progress.UserStreak

	err := row.Scan(
		&result.CurrentDays,
		&result.LongestDays,
		&result.LastActiveDate,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return progress.UserStreak{}, progress.ErrStreakNotFound
	}
	if err != nil {
		return progress.UserStreak{}, fmt.Errorf("%s: %w", operation, err)
	}

	return result, nil
}

func (repository *ProgressRepository) ByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (progress.Progress, error) {
	const query = `
		SELECT
			p.level,
			p.experience,
			l.required_total_experience,
			l.title,
			COALESCE(s.current_days, 0),
			COALESCE(s.longest_days, 0),
			s.last_active_date,
			COALESCE(s.updated_at, p.created_at)
		FROM pets AS p
		JOIN levels AS l
			ON l.level = p.level
		LEFT JOIN user_streaks AS s
			ON s.user_id = p.user_id
		WHERE p.user_id = $1
	`

	var result progress.Progress

	err := repository.db.QueryRow(ctx, query, userID).Scan(
		&result.Level,
		&result.Experience,
		&result.RequiredTotalExperience,
		&result.Title,
		&result.UserStreak.CurrentDays,
		&result.UserStreak.LongestDays,
		&result.UserStreak.LastActiveDate,
		&result.UserStreak.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return progress.Progress{}, progress.ErrNotFound
		}

		return progress.Progress{}, fmt.Errorf("select progress by user id: %w", err)
	}

	return result, nil
}

func (repository *ProgressRepository) LevelByNumber(
	ctx context.Context,
	levelNumber int,
) (progress.Level, error) {
	const query = `
		SELECT
			level,
			required_total_experience,
			title,
			created_at
		FROM levels
		WHERE level = $1
	`

	var result progress.Level

	err := repository.db.QueryRow(ctx, query, levelNumber).Scan(
		&result.Level,
		&result.RequiredTotalExperience,
		&result.Title,
		&result.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return progress.Level{}, progress.ErrLevelNotFound
	}
	if err != nil {
		return progress.Level{}, fmt.Errorf(
			"select level by number: %w",
			err,
		)
	}

	return result, nil
}

func (repository *ProgressRepository) StreakByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (progress.UserStreak, error) {
	const query = `
		SELECT
			current_days,
			longest_days,
			last_active_date,
			updated_at
		FROM user_streaks
		WHERE user_id = $1
	`

	return repository.scanStreak(
		repository.db.QueryRow(ctx, query, userID),
		"select user streak",
	)
}

func (repository *ProgressRepository) AdvanceStreak(
	ctx context.Context,
	userID uuid.UUID,
	activeAt time.Time,
) (progress.UserStreak, error) {
	const query = `
		INSERT INTO user_streaks (
			user_id,
			current_days,
			longest_days,
			last_active_date
		)
		VALUES ($1, 1, 1, $2::date)
		ON CONFLICT (user_id) DO UPDATE
		SET
			current_days = CASE
				WHEN user_streaks.last_active_date = EXCLUDED.last_active_date
					THEN user_streaks.current_days
				WHEN user_streaks.last_active_date = EXCLUDED.last_active_date - 1
					THEN user_streaks.current_days + 1
				ELSE 1
			END,
			longest_days = GREATEST(
				user_streaks.longest_days,
				CASE
					WHEN user_streaks.last_active_date = EXCLUDED.last_active_date
						THEN user_streaks.current_days
					WHEN user_streaks.last_active_date = EXCLUDED.last_active_date - 1
						THEN user_streaks.current_days + 1
					ELSE 1
				END
			),
			last_active_date = EXCLUDED.last_active_date,
			updated_at = NOW()
		RETURNING
			current_days,
			longest_days,
			last_active_date,
			updated_at
	`

	activeDate := activeAt.UTC().Format(time.DateOnly)

	return repository.scanStreak(
		repository.db.QueryRow(ctx, query, userID, activeDate),
		"advance user streak",
	)
}
