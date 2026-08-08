package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/activity"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
)

type performPetActionRequest struct {
	ActivityCode string `json:"activity_code"`
}

type performPetActionResponse struct {
	PetAction petActionResponse `json:"pet_action"`
}

type petActionResponse struct {
	ID                int64     `json:"id"`
	ActivityCode      string    `json:"activity_code"`
	ExperienceAwarded int       `json:"experience_awarded"`
	StateDelta        any       `json:"state_delta"`
	OccurredAt        time.Time `json:"occurred_at"`
	CreatedAt         time.Time `json:"created_at"`
}

func newPetActionResponse(value activity.Action) petActionResponse {
	return petActionResponse{
		ID:                value.ID,
		ActivityCode:      value.ActivityCode,
		ExperienceAwarded: value.ExperienceAwarded,
		StateDelta:        value.StateDelta,
		OccurredAt:        value.OccurredAt,
		CreatedAt:         value.CreatedAt,
	}
}

func performPetActionHandler(
	service activityService,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := userIDFromContext(request.Context())
		if !ok {
			writeInternalError(
				response,
				logger,
				"perform pet action",
				errors.New("user id is missing from context"),
			)
			return
		}

		var body performPetActionRequest
		if err := decodeJSON(response, request, &body); err != nil {
			writeError(
				response,
				http.StatusBadRequest,
				codeBadRequest,
				err.Error(),
			)
			return
		}

		body.ActivityCode = strings.TrimSpace(body.ActivityCode)
		if body.ActivityCode == "" {
			writeFieldError(
				response,
				"activity_code",
				"activity code is required",
			)
			return
		}

		idempotencyKeyValue := strings.TrimSpace(
			request.Header.Get("Idempotency-Key"),
		)
		if idempotencyKeyValue == "" {
			writeFieldError(
				response,
				"Idempotency-Key",
				"Idempotency-Key header is required",
			)
			return
		}

		idempotencyKey, err := uuid.Parse(idempotencyKeyValue)
		if err != nil || idempotencyKey == uuid.Nil {
			writeFieldError(
				response,
				"Idempotency-Key",
				"Idempotency-Key must be a valid UUID",
			)
			return
		}

		actionResult, err := service.PerformAction(
			request.Context(),
			activity.PerformActionParams{
				UserID:         userID,
				ActivityCode:   body.ActivityCode,
				IdempotencyKey: idempotencyKey,
			},
		)
		if err != nil {
			switch {
			case errors.Is(err, activity.ErrTypeNotFound):
				writeError(
					response,
					http.StatusNotFound,
					codeInvalidActivityCode,
					"activity type not found",
				)

			case errors.Is(err, activity.ErrActivityInactive):
				writeError(
					response,
					http.StatusConflict,
					codeActivityNotActive,
					"activity is not active",
				)

			case errors.Is(err, activity.ErrCooldown):
				writeError(
					response,
					http.StatusConflict,
					codeCooldownActive,
					"activity cooldown is active",
				)

			case errors.Is(err, activity.ErrDailyLimit):
				writeError(
					response,
					http.StatusConflict,
					codeDailyLimitReached,
					"daily activity limit reached",
				)

			case errors.Is(err, pet.ErrNotFound):
				writeError(
					response,
					http.StatusNotFound,
					codePetNotFound,
					"pet not found",
				)

			default:
				writeInternalError(
					response,
					logger,
					"perform pet action",
					err,
				)
			}

			return
		}

		writeJSON(response, http.StatusOK, performPetActionResponse{
			PetAction: newPetActionResponse(actionResult),
		})
	}
}
