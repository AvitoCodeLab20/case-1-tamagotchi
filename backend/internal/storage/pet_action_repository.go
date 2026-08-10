package storage

import (
	"context"
	"errors"

	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/activity"
)

type PetActionRepository struct {
	db Querier
}

func NewPetActionRepository(db Querier) *PetActionRepository {
	return &PetActionRepository{
		db: db,
	}
}

func (repository *PetActionRepository) Create(
	ctx context.Context,
	action activity.Action,
) (activity.Action, error) {
	const query = `
		INSERT INTO pet_actions (
			user_id,
			pet_id,
			activity_code,
			experience_awarded,
			state_delta,
			idempotency_key,
			occurred_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	err := repository.db.QueryRow(
		ctx,
		query,
		action.UserID,
		action.PetID,
		action.ActivityCode,
		action.ExperienceAwarded,
		action.StateDelta,
		action.IdempotencyKey,
		action.OccurredAt,
	).Scan(
		&action.ID,
		&action.CreatedAt,
	)
	if err != nil {
		return activity.Action{}, err
	}

	return action, nil
}

func (repository *PetActionRepository) ByIdempotencyKey(
	ctx context.Context,
	userID uuid.UUID,
	key uuid.UUID,
) (activity.Action, error) {
	const query = `
		SELECT
			id,
			user_id,
			pet_id,
			activity_code,
			experience_awarded,
			state_delta,
			idempotency_key,
			occurred_at,
			created_at
		FROM pet_actions
		WHERE user_id = $1
		  AND idempotency_key = $2
	`

	var action activity.Action

	err := repository.db.QueryRow(
		ctx,
		query,
		userID,
		key,
	).Scan(
		&action.ID,
		&action.UserID,
		&action.PetID,
		&action.ActivityCode,
		&action.ExperienceAwarded,
		&action.StateDelta,
		&action.IdempotencyKey,
		&action.OccurredAt,
		&action.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return activity.Action{}, activity.ErrActionNotFound
	}
	if err != nil {
		return activity.Action{}, err
	}

	return action, nil
}

func (repository *PetActionRepository) LastAction(
	ctx context.Context,
	userID uuid.UUID,
	activityCode string,
) (activity.Action, error) {
	const query = `
		SELECT
			id,
			user_id,
			pet_id,
			activity_code,
			experience_awarded,
			state_delta,
			idempotency_key,
			occurred_at,
			created_at
		FROM pet_actions
		WHERE user_id = $1
		  AND activity_code = $2
		ORDER BY occurred_at DESC
		LIMIT 1
	`

	var action activity.Action

	err := repository.db.QueryRow(
		ctx,
		query,
		userID,
		activityCode,
	).Scan(
		&action.ID,
		&action.UserID,
		&action.PetID,
		&action.ActivityCode,
		&action.ExperienceAwarded,
		&action.StateDelta,
		&action.IdempotencyKey,
		&action.OccurredAt,
		&action.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return activity.Action{}, activity.ErrActionNotFound
	}
	if err != nil {
		return activity.Action{}, err
	}

	return action, nil
}

func (repository *PetActionRepository) CountActionsSince(
	ctx context.Context,
	userID uuid.UUID,
	activityCode string,
	since time.Time,
) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM pet_actions
		WHERE user_id = $1
		  AND activity_code = $2
		  AND occurred_at >= $3
	`

	var count int

	err := repository.db.QueryRow(
		ctx,
		query,
		userID,
		activityCode,
		since,
	).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
