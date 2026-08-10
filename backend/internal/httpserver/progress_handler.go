package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/progress"
)

func newUserStreakResponse(
	value progress.UserStreak,
) userStreakResponse {
	var lastActiveDate *string

	if value.LastActiveDate != nil {
		formatted := value.LastActiveDate.Format(time.DateOnly)
		lastActiveDate = &formatted
	}

	return userStreakResponse{
		CurrentDays:    value.CurrentDays,
		LongestDays:    value.LongestDays,
		LastActiveDate: lastActiveDate,
		UpdatedAt:      value.UpdatedAt,
	}
}

type progressService interface {
	Get(ctx context.Context, userID uuid.UUID) (progress.Progress, error)
}

type progressResponse struct {
	Level                   int                `json:"level"`
	Experience              int64              `json:"experience"`
	RequiredTotalExperience int64              `json:"required_total_experience"`
	Title                   string             `json:"title"`
	UserStreak              userStreakResponse `json:"user_streak"`
}

type userStreakResponse struct {
	CurrentDays    int       `json:"current_days"`
	LongestDays    int       `json:"longest_days"`
	LastActiveDate *string   `json:"last_active_date"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func newProgressResponse(value progress.Progress) progressResponse {
	return progressResponse{
		Level:                   value.Level,
		Experience:              value.Experience,
		RequiredTotalExperience: value.RequiredTotalExperience,
		Title:                   value.Title,
		UserStreak:              newUserStreakResponse(value.UserStreak),
	}
}

func progressHandler(
	service progressService,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := userIDFromContext(request.Context())
		if !ok {
			writeInternalError(
				response,
				logger,
				"get progress",
				errors.New("user id is missing from context"),
			)
			return
		}

		value, err := service.Get(request.Context(), userID)
		if err != nil {
			if errors.Is(err, progress.ErrNotFound) {
				writeError(
					response,
					http.StatusNotFound,
					codeNotFound,
					"progress not found",
				)
				return
			}

			writeInternalError(response, logger, "get progress", err)
			return
		}

		writeJSON(response, http.StatusOK, newProgressResponse(value))
	}
}
