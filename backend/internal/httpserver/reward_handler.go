package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/rewards"
)

type rewardService interface {
	ListInventory(ctx context.Context, userID uuid.UUID, status string) (rewards.Inventory, error)
	Redeem(
		ctx context.Context,
		userID uuid.UUID,
		rewardID uuid.UUID,
		idempotencyKey uuid.UUID,
		categoryCode string,
		listingID string,
	) (rewards.Reward, error)
	SelectAward(
		ctx context.Context,
		userID uuid.UUID,
		awardID uuid.UUID,
		idempotencyKey uuid.UUID,
		optionCode string,
	) (rewards.Reward, error)
}

type rewardInventoryResponse struct {
	Rewards           []rewardResponse           `json:"rewards"`
	LeaderboardAwards []leaderboardAwardResponse `json:"leaderboard_awards"`
}

type rewardResponse struct {
	ID          string                `json:"id"`
	Code        string                `json:"code"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Source      string                `json:"source"`
	Status      string                `json:"status"`
	Benefit     rewardBenefitResponse `json:"benefit"`
	IssuedAt    time.Time             `json:"issued_at"`
	ExpiresAt   time.Time             `json:"expires_at"`
	RedeemedAt  *time.Time            `json:"redeemed_at"`
}

type rewardBenefitResponse struct {
	Type                      string `json:"type"`
	Percent                   *int   `json:"percent,omitempty"`
	AmountRUB                 *int   `json:"amount_rub,omitempty"`
	MaxAmountRUB              *int   `json:"max_amount_rub,omitempty"`
	DurationDays              *int   `json:"duration_days,omitempty"`
	CategorySelectionRequired bool   `json:"category_selection_required"`
}

type leaderboardAwardResponse struct {
	ID           string                 `json:"id"`
	WeekStartsAt time.Time              `json:"week_starts_at"`
	Tier         int                    `json:"tier"`
	Status       string                 `json:"status"`
	SelectBefore time.Time              `json:"select_before"`
	Options      []rewardOptionResponse `json:"options"`
}

type rewardOptionResponse struct {
	Code        string                `json:"code"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Benefit     rewardBenefitResponse `json:"benefit"`
}

type redeemRewardRequest struct {
	CategoryCode string `json:"category_code"`
	ListingID    string `json:"listing_id"`
}

type selectLeaderboardAwardRequest struct {
	OptionCode string `json:"option_code"`
}

func listRewardsHandler(
	authentication authService,
	service rewardService,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := activeUserID(response, request, authentication, logger, "list rewards")
		if !ok {
			return
		}
		inventory, err := service.ListInventory(request.Context(), userID, request.URL.Query().Get("status"))
		if err != nil {
			writeRewardError(response, logger, "list rewards", err)
			return
		}
		writeJSON(response, http.StatusOK, newRewardInventoryResponse(inventory))
	}
}

func redeemRewardHandler(
	authentication authService,
	service rewardService,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := activeUserID(response, request, authentication, logger, "redeem reward")
		if !ok {
			return
		}
		rewardID, ok := pathUUID(response, request, "reward_id")
		if !ok {
			return
		}
		idempotencyKey, ok := idempotencyKey(response, request)
		if !ok {
			return
		}
		payload := redeemRewardRequest{}
		if err := decodeJSON(response, request, &payload); err != nil {
			writeError(response, http.StatusBadRequest, codeBadRequest, err.Error())
			return
		}
		reward, err := service.Redeem(
			request.Context(), userID, rewardID, idempotencyKey, payload.CategoryCode, payload.ListingID,
		)
		if err != nil {
			writeRewardError(response, logger, "redeem reward", err)
			return
		}
		writeJSON(response, http.StatusOK, newRewardResponse(reward))
	}
}

func selectLeaderboardAwardHandler(
	authentication authService,
	service rewardService,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		userID, ok := activeUserID(response, request, authentication, logger, "select leaderboard award")
		if !ok {
			return
		}
		awardID, ok := pathUUID(response, request, "award_id")
		if !ok {
			return
		}
		idempotencyKey, ok := idempotencyKey(response, request)
		if !ok {
			return
		}
		payload := selectLeaderboardAwardRequest{}
		if err := decodeJSON(response, request, &payload); err != nil {
			writeError(response, http.StatusBadRequest, codeBadRequest, err.Error())
			return
		}
		reward, err := service.SelectAward(
			request.Context(), userID, awardID, idempotencyKey, payload.OptionCode,
		)
		if err != nil {
			writeRewardError(response, logger, "select leaderboard award", err)
			return
		}
		writeJSON(response, http.StatusOK, newRewardResponse(reward))
	}
}

func activeUserID(
	response http.ResponseWriter,
	request *http.Request,
	authentication authService,
	logger *slog.Logger,
	operation string,
) (uuid.UUID, bool) {
	userID, ok := userIDFromContext(request.Context())
	if !ok {
		writeInternalError(response, logger, operation, errors.New("handler reached without requireAuth"))
		return uuid.Nil, false
	}
	if _, err := authentication.UserByID(request.Context(), userID); err != nil {
		writeAuthError(response, logger, operation+" user", err)
		return uuid.Nil, false
	}
	return userID, true
}

func pathUUID(response http.ResponseWriter, request *http.Request, name string) (uuid.UUID, bool) {
	value, err := uuid.Parse(request.PathValue(name))
	if err != nil {
		writeFieldError(response, name, name+" must be a UUID")
		return uuid.Nil, false
	}
	return value, true
}

func idempotencyKey(response http.ResponseWriter, request *http.Request) (uuid.UUID, bool) {
	value, err := uuid.Parse(request.Header.Get("Idempotency-Key"))
	if err != nil {
		writeFieldError(response, "Idempotency-Key", "Idempotency-Key must be a UUID")
		return uuid.Nil, false
	}
	return value, true
}

func writeRewardError(response http.ResponseWriter, logger *slog.Logger, operation string, err error) {
	validationError := rewards.ValidationError{}
	switch {
	case errors.As(err, &validationError):
		writeFieldError(response, validationError.Field, validationError.Message)
	case errors.Is(err, rewards.ErrNotFound):
		writeError(response, http.StatusNotFound, codeNotFound, "reward was not found")
	case errors.Is(err, rewards.ErrNotAvailable):
		writeError(response, http.StatusConflict, codeRewardNotAvailable, "reward is not available")
	case errors.Is(err, rewards.ErrSelectionExpired):
		writeError(response, http.StatusConflict, codeSelectionExpired, "reward selection has expired")
	case errors.Is(err, rewards.ErrIdempotencyConflict):
		writeError(response, http.StatusConflict, codeIdempotencyConflict, "idempotency key was reused")
	default:
		writeInternalError(response, logger, operation, err)
	}
}

func newRewardInventoryResponse(inventory rewards.Inventory) rewardInventoryResponse {
	result := rewardInventoryResponse{
		Rewards:           make([]rewardResponse, len(inventory.Rewards)),
		LeaderboardAwards: make([]leaderboardAwardResponse, len(inventory.LeaderboardAwards)),
	}
	for index, reward := range inventory.Rewards {
		result.Rewards[index] = newRewardResponse(reward)
	}
	for index, award := range inventory.LeaderboardAwards {
		result.LeaderboardAwards[index] = leaderboardAwardResponse{
			ID:           award.ID.String(),
			WeekStartsAt: award.WeekStartsAt,
			Tier:         award.Tier,
			Status:       award.Status,
			SelectBefore: award.SelectBefore,
			Options:      make([]rewardOptionResponse, len(award.Options)),
		}
		for optionIndex, option := range award.Options {
			result.LeaderboardAwards[index].Options[optionIndex] = rewardOptionResponse{
				Code: option.Code, Title: option.Title, Description: option.Description,
				Benefit: newRewardBenefitResponse(option.Benefit),
			}
		}
	}
	return result
}

func newRewardResponse(reward rewards.Reward) rewardResponse {
	return rewardResponse{
		ID:          reward.ID.String(),
		Code:        reward.Code,
		Title:       reward.Title,
		Description: reward.Description,
		Source:      reward.Source,
		Status:      reward.Status,
		Benefit:     newRewardBenefitResponse(reward.Benefit),
		IssuedAt:    reward.IssuedAt,
		ExpiresAt:   reward.ExpiresAt,
		RedeemedAt:  reward.RedeemedAt,
	}
}

func newRewardBenefitResponse(benefit rewards.Benefit) rewardBenefitResponse {
	return rewardBenefitResponse{
		Type:                      benefit.Type,
		Percent:                   benefit.Percent,
		AmountRUB:                 benefit.AmountRUB,
		MaxAmountRUB:              benefit.MaxAmountRUB,
		DurationDays:              benefit.DurationDays,
		CategorySelectionRequired: benefit.CategorySelectionRequired,
	}
}
