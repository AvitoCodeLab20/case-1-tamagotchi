package rewards

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusIssued   = "issued"
	StatusRedeemed = "redeemed"
	StatusExpired  = "expired"
	StatusRevoked  = "revoked"

	SourceLevel       = "level"
	SourceStreak      = "streak"
	SourceLeaderboard = "leaderboard"
)

type Benefit struct {
	Type                      string `json:"type"`
	Percent                   *int   `json:"percent,omitempty"`
	AmountRUB                 *int   `json:"amount_rub,omitempty"`
	MaxAmountRUB              *int   `json:"max_amount_rub,omitempty"`
	DurationDays              *int   `json:"duration_days,omitempty"`
	CategorySelectionRequired bool   `json:"category_selection_required"`
}

type Reward struct {
	ID          uuid.UUID
	Code        string
	Title       string
	Description string
	Source      string
	Status      string
	Benefit     Benefit
	IssuedAt    time.Time
	ExpiresAt   time.Time
	RedeemedAt  *time.Time
}

type RewardOption struct {
	Code        string
	Title       string
	Description string
	Benefit     Benefit
}

type LeaderboardAward struct {
	ID           uuid.UUID
	WeekStartsAt time.Time
	Tier         int
	Status       string
	SelectBefore time.Time
	Options      []RewardOption
}

type Inventory struct {
	Rewards           []Reward
	LeaderboardAwards []LeaderboardAward
}

type RedeemCommand struct {
	UserID         uuid.UUID
	RewardID       uuid.UUID
	IdempotencyKey uuid.UUID
	RequestHash    []byte
	CategoryCode   string
	ListingID      string
	RedeemedAt     time.Time
}

type SelectAwardCommand struct {
	UserID         uuid.UUID
	AwardID        uuid.UUID
	IdempotencyKey uuid.UUID
	RequestHash    []byte
	OptionCode     string
	SelectedAt     time.Time
	ExpiresAt      time.Time
}

type IssueCommand struct {
	UserID       uuid.UUID
	TriggerType  string
	TriggerValue int
	IssuanceKey  string
	OccurredAt   time.Time
	ExpiresAt    time.Time
}

func (reward Reward) ValidateRedemption(categoryCode, listingID string) error {
	categoryCode = strings.TrimSpace(categoryCode)
	listingID = strings.TrimSpace(listingID)
	if reward.Benefit.CategorySelectionRequired && categoryCode == "" {
		return ValidationError{Field: "category_code", Message: "category code is required for this reward"}
	}
	if !reward.Benefit.CategorySelectionRequired && categoryCode != "" {
		return ValidationError{Field: "category_code", Message: "category code is not supported for this reward"}
	}
	if rewardRequiresListing(reward.Benefit.Type) && listingID == "" {
		return ValidationError{Field: "listing_id", Message: "listing id is required for this reward"}
	}
	if !rewardRequiresListing(reward.Benefit.Type) && listingID != "" {
		return ValidationError{Field: "listing_id", Message: "listing id is not supported for this reward"}
	}
	return nil
}

func rewardRequiresListing(benefitType string) bool {
	switch benefitType {
	case "promotion_discount", "promotion_certificate", "price_highlight", "xl_listing", "xl_listing_with_badge":
		return true
	default:
		return false
	}
}
