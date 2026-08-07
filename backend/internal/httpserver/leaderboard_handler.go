package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/leaderboard"
)

const maxLeaderboardLimit = 100

type leaderboardService interface {
	Current(ctx context.Context, userID uuid.UUID, limit int) (leaderboard.Board, error)
}

type leaderboardResponse struct {
	Period            leaderboardPeriodResponse  `json:"period"`
	Status            string                     `json:"status"`
	ParticipantsCount int                        `json:"participants_count"`
	Entries           []leaderboardEntryResponse `json:"entries"`
	Me                *leaderboardEntryResponse  `json:"me,omitempty"`
	NextTier          *nextTierProgressResponse  `json:"next_tier,omitempty"`
}

type leaderboardPeriodResponse struct {
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Timezone string    `json:"timezone"`
}

type leaderboardEntryResponse struct {
	UserID           string `json:"user_id"`
	DisplayName      string `json:"display_name"`
	Rank             int    `json:"rank"`
	WeeklyExperience int64  `json:"weekly_experience"`
	PrizeTier        *int   `json:"prize_tier"`
	IsCurrentUser    bool   `json:"is_current_user"`
}

type nextTierProgressResponse struct {
	Tier               int   `json:"tier"`
	ExperienceRequired int64 `json:"experience_required"`
}

func currentLeaderboardHandler(
	authentication authService,
	service leaderboardService,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := userIDFromContext(request.Context())
		if !ok {
			writeInternalError(response, logger, "current leaderboard", errors.New("handler reached without requireAuth"))

			return
		}

		if _, err := authentication.UserByID(request.Context(), userID); err != nil {
			writeAuthError(response, logger, "current leaderboard user", err)

			return
		}

		limit, err := leaderboardLimit(request)
		if err != nil {
			writeFieldError(response, "limit", err.Error())

			return
		}

		board, err := service.Current(request.Context(), userID, limit)
		if err != nil {
			writeInternalError(response, logger, "current leaderboard", err)

			return
		}

		writeJSON(response, http.StatusOK, newLeaderboardResponse(board, userID))
	}
}

func leaderboardLimit(request *http.Request) (int, error) {
	rawLimit := request.URL.Query().Get("limit")
	if rawLimit == "" {
		return leaderboard.DefaultLimit, nil
	}

	limit, err := strconv.Atoi(rawLimit)
	if err != nil || limit < 1 || limit > maxLeaderboardLimit {
		return 0, errors.New("limit must be an integer between 1 and 100")
	}

	return limit, nil
}

func newLeaderboardResponse(board leaderboard.Board, currentUserID uuid.UUID) leaderboardResponse {
	result := leaderboardResponse{
		Period: leaderboardPeriodResponse{
			StartsAt: board.Period.StartsAt,
			EndsAt:   board.Period.EndsAt,
			Timezone: board.Period.Timezone,
		},
		Status:            board.Status,
		ParticipantsCount: board.ParticipantsCount,
		Entries:           make([]leaderboardEntryResponse, len(board.Entries)),
	}

	for index, entry := range board.Entries {
		result.Entries[index] = newLeaderboardEntryResponse(entry, currentUserID)
	}
	if board.Me != nil {
		entry := newLeaderboardEntryResponse(*board.Me, currentUserID)
		result.Me = &entry
	}
	if board.NextTier != nil {
		result.NextTier = &nextTierProgressResponse{
			Tier:               board.NextTier.Tier,
			ExperienceRequired: board.NextTier.ExperienceRequired,
		}
	}

	return result
}

func newLeaderboardEntryResponse(entry leaderboard.Entry, currentUserID uuid.UUID) leaderboardEntryResponse {
	var prizeTier *int
	if entry.PrizeTier != 0 {
		value := entry.PrizeTier
		prizeTier = &value
	}

	return leaderboardEntryResponse{
		UserID:           entry.UserID.String(),
		DisplayName:      entry.DisplayName,
		Rank:             entry.Rank,
		WeeklyExperience: entry.WeeklyExperience,
		PrizeTier:        prizeTier,
		IsCurrentUser:    entry.UserID == currentUserID,
	}
}
