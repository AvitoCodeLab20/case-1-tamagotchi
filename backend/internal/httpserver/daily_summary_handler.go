package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/dailysummary"
)

type dailySummaryService interface {
	Current(
		ctx context.Context,
		userID uuid.UUID,
	) (dailysummary.DailySummary, error)
	ByDate(
		ctx context.Context,
		userID uuid.UUID,
		summaryDate time.Time,
	) (dailysummary.DailySummary, error)
}

type petStateResponse struct {
	Level      int   `json:"level"`
	Experience int64 `json:"experience"`
	Health     int   `json:"health"`
	Hunger     int   `json:"hunger"`
	Happiness  int   `json:"happiness"`
	Energy     int   `json:"energy"`
}

type dailySummaryResponse struct {
	ID               int64            `json:"id"`
	SummaryDate      string           `json:"summary_date"`
	ActionCount      int              `json:"action_count"`
	ExperienceEarned int              `json:"experience_earned"`
	LevelsGained     int              `json:"levels_gained"`
	StateBefore      petStateResponse `json:"state_before"`
	StateAfter       petStateResponse `json:"state_after"`
	GeneratedAt      time.Time        `json:"generated_at"`
}

func newPetStateResponse(value dailysummary.PetState) petStateResponse {
	return petStateResponse{
		Level:      value.Level,
		Experience: value.Experience,
		Health:     value.Health,
		Hunger:     value.Hunger,
		Happiness:  value.Happiness,
		Energy:     value.Energy,
	}
}

func newDailySummaryResponse(
	value dailysummary.DailySummary,
) dailySummaryResponse {
	return dailySummaryResponse{
		ID:               value.ID,
		SummaryDate:      value.SummaryDate.Format(time.DateOnly),
		ActionCount:      value.ActionCount,
		ExperienceEarned: value.ExperienceEarned,
		LevelsGained:     value.LevelsGained,
		StateBefore:      newPetStateResponse(value.StateBefore),
		StateAfter:       newPetStateResponse(value.StateAfter),
		GeneratedAt:      value.GeneratedAt,
	}
}

func currentDailySummaryHandler(
	service dailySummaryService,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := userIDFromContext(request.Context())
		if !ok {
			writeInternalError(
				response,
				logger,
				"get current daily summary",
				errors.New("user id is missing from context"),
			)
			return
		}

		value, err := service.Current(request.Context(), userID)
		if errors.Is(err, dailysummary.ErrNotFound) {
			writeError(
				response,
				http.StatusNotFound,
				codeNotFound,
				"daily summary not found",
			)
			return
		}
		if err != nil {
			writeInternalError(
				response,
				logger,
				"get current daily summary",
				err,
			)
			return
		}

		writeJSON(response, http.StatusOK, newDailySummaryResponse(value))
	}
}

func dailySummaryByDateHandler(
	service dailySummaryService,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := userIDFromContext(request.Context())
		if !ok {
			writeInternalError(
				response,
				logger,
				"get daily summary by date",
				errors.New("user id is missing from context"),
			)
			return
		}

		summaryDate, err := time.Parse(
			time.DateOnly,
			request.PathValue("summary_date"),
		)
		if err != nil {
			writeFieldError(
				response,
				"summary_date",
				"summary_date must be in YYYY-MM-DD format",
			)
			return
		}

		value, err := service.ByDate(
			request.Context(),
			userID,
			summaryDate,
		)
		if errors.Is(err, dailysummary.ErrNotFound) {
			writeError(
				response,
				http.StatusNotFound,
				codeNotFound,
				"daily summary not found",
			)
			return
		}
		if err != nil {
			writeInternalError(
				response,
				logger,
				"get daily summary by date",
				err,
			)
			return
		}

		writeJSON(response, http.StatusOK, newDailySummaryResponse(value))
	}
}
