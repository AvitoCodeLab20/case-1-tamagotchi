package storage

import (
	"context"
	"errors"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/activity"
	"github.com/jackc/pgx/v5"
)

type ActivityTypeRepository struct {
	db Querier
}

func NewActivityTypeRepository(db Querier) *ActivityTypeRepository {
	return &ActivityTypeRepository{
		db: db,
	}
}

func (repository *ActivityTypeRepository) ByCode(
	ctx context.Context,
	code string,
) (activity.ActivityType, error) {
	const query = `
		SELECT
			code,
			title,
			description,
			category,
			base_experience,
			daily_limit,
			cooldown_seconds,
			is_active
		FROM activity_types
		WHERE code = $1
	`

	var activityType activity.ActivityType

	err := repository.db.QueryRow(
		ctx,
		query,
		code,
	).Scan(
		&activityType.Code,
		&activityType.Title,
		&activityType.Description,
		&activityType.Category,
		&activityType.BaseExperience,
		&activityType.DailyLimit,
		&activityType.CooldownSeconds,
		&activityType.IsActive,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return activity.ActivityType{}, activity.ErrActivityTypeNotFound
	}
	if err != nil {
		return activity.ActivityType{}, err
	}

	return activityType, nil
}
