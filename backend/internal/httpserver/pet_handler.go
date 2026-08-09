package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/progress"
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
	CreatedAt         time.Time  `json:"created_at"`
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
		CreatedAt:         value.CreatedAt,
	}
}

type getPetResponse struct {
	Pet        petResponse        `json:"pet"`
	Level      levelResponse      `json:"level"`
	UserStreak userStreakResponse `json:"user_streak"`
}

type levelResponse struct {
	Level                   int       `json:"level"`
	RequiredTotalExperience int64     `json:"required_total_experience"`
	Title                   string    `json:"title"`
	CreatedAt               time.Time `json:"created_at"`
}

func petHandler(
	petService petService,
	progressService progressService,
	logger *slog.Logger,
) http.HandlerFunc {
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

		petValue, err := petService.Get(request.Context(), userID)
		if err != nil {
			if errors.Is(err, pet.ErrNotFound) {
				writeError(
					response,
					http.StatusNotFound,
					codeNotFound,
					"pet not found",
				)
				return
			}

			writeInternalError(response, logger, "get pet", err)
			return
		}

		progressValue, err := progressService.Get(request.Context(), userID)
		if err != nil {
			if errors.Is(err, progress.ErrNotFound) {
				writeError(
					response,
					http.StatusNotFound,
					codeNotFound,
					"pet progress not found",
				)
				return
			}

			writeInternalError(response, logger, "get pet progress", err)
			return
		}

		writeJSON(response, http.StatusOK, getPetResponse{
			Pet: newPetResponse(petValue),
			Level: levelResponse{
				Level:                   progressValue.Level,
				RequiredTotalExperience: progressValue.RequiredTotalExperience,
				Title:                   progressValue.Title,
				CreatedAt:               progressValue.LevelCreatedAt,
			},
			UserStreak: newUserStreakResponse(progressValue.UserStreak),
		})
	}
}
