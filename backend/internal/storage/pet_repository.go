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
