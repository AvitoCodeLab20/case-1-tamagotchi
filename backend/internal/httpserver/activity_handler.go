package httpserver

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/activity"
)

type activityService interface {
	ListTypes(ctx context.Context) ([]activity.Type, error)

	PerformAction(
		ctx context.Context,
		params activity.PerformActionParams,
	) (activity.PerformActionResult, error)
}
type activityTypeResponse struct {
	Code            string `json:"code"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Category        string `json:"category"`
	BaseExperience  int    `json:"base_experience"`
	DailyLimit      *int   `json:"daily_limit"`
	CooldownSeconds int    `json:"cooldown_seconds"`
}

type activityTypesResponse struct {
	ActivityTypes []activityTypeResponse `json:"activity_types"`
}

func activityTypesHandler(
	service activityService,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		types, err := service.ListTypes(request.Context())
		if err != nil {
			writeInternalError(response, logger, "list activity types", err)
			return
		}

		items := make([]activityTypeResponse, 0, len(types))

		for _, activityType := range types {
			items = append(items, activityTypeResponse{
				Code:            activityType.Code,
				Title:           activityType.Title,
				Description:     activityType.Description,
				Category:        activityType.Category,
				BaseExperience:  activityType.BaseExperience,
				DailyLimit:      activityType.DailyLimit,
				CooldownSeconds: activityType.CooldownSeconds,
			})
		}

		writeJSON(response, http.StatusOK, activityTypesResponse{
			ActivityTypes: items,
		})
	}
}
