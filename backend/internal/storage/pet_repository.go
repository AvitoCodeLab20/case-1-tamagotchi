package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
)

type PetRepository struct {
	db Querier
}

func NewPetRepository(db Querier) *PetRepository {
	return &PetRepository{db: db}
}

func (repository *PetRepository) ByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (pet.Pet, error) {
	const query = `
		SELECT
			id,
			user_id,
			name,
			species,
			level,
			experience,
			health,
			hunger,
			happiness,
			energy,
			state_version,
			last_interaction_at,
			created_at,
			updated_at
		FROM pets
		WHERE user_id = $1`

	result := pet.Pet{}

	err := repository.db.QueryRow(ctx, query, userID).Scan(
		&result.ID,
		&result.UserID,
		&result.Name,
		&result.Species,
		&result.Level,
		&result.Experience,
		&result.Health,
		&result.Hunger,
		&result.Happiness,
		&result.Energy,
		&result.StateVersion,
		&result.LastInteractionAt,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pet.Pet{}, pet.ErrNotFound
		}

		return pet.Pet{}, fmt.Errorf("select pet by user id: %w", err)
	}

	return result, nil
}

func (repository *PetRepository) ApplyAction(
	ctx context.Context,
	userID uuid.UUID,
	params pet.ApplyActionParams,
) (pet.Pet, error) {
	const query = `
		UPDATE pets AS p
		SET
			experience = p.experience + $2,

			level = COALESCE((
				SELECT MAX(l.level)
				FROM levels AS l
				WHERE l.required_total_experience <= p.experience + $2
			), p.level),

			health = LEAST(100, GREATEST(0, p.health + $3)),
			hunger = LEAST(100, GREATEST(0, p.hunger + $4)),
			happiness = LEAST(100, GREATEST(0, p.happiness + $5)),
			energy = LEAST(100, GREATEST(0, p.energy + $6)),

			state_version = p.state_version + 1,
			last_interaction_at = $7

		WHERE p.user_id = $1

		RETURNING
			id,
			user_id,
			name,
			species,
			level,
			experience,
			health,
			hunger,
			happiness,
			energy,
			state_version,
			last_interaction_at,
			created_at,
			updated_at
	`

	var result pet.Pet

	err := repository.db.QueryRow(
		ctx,
		query,
		userID,
		params.Experience,
		params.Health,
		params.Hunger,
		params.Happiness,
		params.Energy,
		params.OccurredAt,
	).Scan(
		&result.ID,
		&result.UserID,
		&result.Name,
		&result.Species,
		&result.Level,
		&result.Experience,
		&result.Health,
		&result.Hunger,
		&result.Happiness,
		&result.Energy,
		&result.StateVersion,
		&result.LastInteractionAt,
		&result.CreatedAt,
		&result.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return pet.Pet{}, pet.ErrNotFound
	}
	if err != nil {
		return pet.Pet{}, err
	}

	return result, nil
}

func (repository *PetRepository) LockByUserID(
	ctx context.Context,
	userID uuid.UUID,
) error {
	const query = `
		SELECT id
		FROM pets
		WHERE user_id = $1
		FOR UPDATE
	`

	var petID uuid.UUID

	err := repository.db.QueryRow(
		ctx,
		query,
		userID,
	).Scan(&petID)
	if err != nil {
		return fmt.Errorf("lock pet by user id: %w", err)
	}

	return nil
}
