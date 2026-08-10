package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/activity"
)

type ActivityRepository struct {
	db ActivityQuerier
}

func NewActivityRepository(db ActivityQuerier) *ActivityRepository {
	return &ActivityRepository{
		db: db,
	}
}

func (repository *ActivityRepository) ListActive(ctx context.Context) ([]activity.Type, error) {
	rows, err := repository.db.Query(ctx, `
		SELECT
			code,
			title,
			description,
			category,
			base_experience,
			daily_limit,
			cooldown_seconds,
			is_active,
			created_at,
			updated_at
		FROM activity_types
		WHERE is_active = TRUE
		ORDER BY code
	`)
	if err != nil {
		return nil, fmt.Errorf("list activity types: %w", err)
	}
	defer rows.Close()

	activities := make([]activity.Type, 0)

	for rows.Next() {
		var item activity.Type

		if err := rows.Scan(
			&item.Code,
			&item.Title,
			&item.Description,
			&item.Category,
			&item.BaseExperience,
			&item.DailyLimit,
			&item.CooldownSeconds,
			&item.IsActive,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan activity type: %w", err)
		}

		activities = append(activities, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity types: %w", err)
	}

	return activities, nil
}

func (repository *ActivityRepository) ByCode(
	ctx context.Context,
	code string,
) (activity.Type, error) {
	const query = `
		SELECT
			code,
			title,
			description,
			category,
			base_experience,
			daily_limit,
			cooldown_seconds,
			is_active,
			created_at,
			updated_at
		FROM activity_types
		WHERE code = $1
	`

	var activityType activity.Type

	err := repository.db.QueryRow(ctx, query, code).Scan(
		&activityType.Code,
		&activityType.Title,
		&activityType.Description,
		&activityType.Category,
		&activityType.BaseExperience,
		&activityType.DailyLimit,
		&activityType.CooldownSeconds,
		&activityType.IsActive,
		&activityType.CreatedAt,
		&activityType.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return activity.Type{}, activity.ErrTypeNotFound
	}
	if err != nil {
		return activity.Type{}, fmt.Errorf(
			"select activity type by code: %w",
			err,
		)
	}

	return activityType, nil
}
