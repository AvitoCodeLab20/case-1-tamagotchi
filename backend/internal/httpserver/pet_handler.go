package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
)

type petService interface {
	Get(ctx context.Context, userID uuid.UUID) (pet.Pet, error)
}

type petResponse struct {
	ID                uuid.UUID  `json:"id"`
	Name              string     `json:"name"`
	Species           string     `json:"species"`
	Level             int        `json:"level"`
	Experience        int64      `json:"experience"`
	Health            int        `json:"health"`
	Hunger            int        `json:"hunger"`
	Happiness         int        `json:"happiness"`
	Energy            int        `json:"energy"`
	StateVersion      int64      `json:"state_version"`
	LastInteractionAt *time.Time `json:"last_interaction_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func newPetResponse(value pet.Pet) petResponse {
	return petResponse{
		ID:                value.ID,
		Name:              value.Name,
		Species:           value.Species,
		Level:             value.Level,
		Experience:        value.Experience,
		Health:            value.Health,
		Hunger:            value.Hunger,
		Happiness:         value.Happiness,
		Energy:            value.Energy,
		StateVersion:      value.StateVersion,
		LastInteractionAt: value.LastInteractionAt,
		UpdatedAt:         value.UpdatedAt,
	}
}

func petHandler(service petService, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := userIDFromContext(request.Context())
		if !ok {
			writeInternalError(
				response,
				logger,
				"get pet",
				errors.New("user id is missing from context"),
			)
			return
		}

		value, err := service.Get(request.Context(), userID)
		if err != nil {
			if errors.Is(err, pet.ErrNotFound) {
				writeError(
					response,
					http.StatusNotFound,
					codePetNotFound,
					"pet not found",
				)
				return
			}

			writeInternalError(response, logger, "get pet", err)
			return
		}

		writeJSON(response, http.StatusOK, newPetResponse(value))
	}
}
