package rewards

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const rewardLifetime = 30 * 24 * time.Hour

type Repository interface {
	ListInventory(ctx context.Context, userID uuid.UUID, status string, now time.Time) (Inventory, error)
	Redeem(ctx context.Context, command RedeemCommand) (Reward, error)
	SelectAward(ctx context.Context, command SelectAwardCommand) (Reward, error)
	Issue(ctx context.Context, command IssueCommand) ([]Reward, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("rewards: repository is required")
	}
	return &Service{repository: repository, now: time.Now}, nil
}

func (service *Service) ListInventory(
	ctx context.Context,
	userID uuid.UUID,
	status string,
) (Inventory, error) {
	if status != "" && !validStatus(status) {
		return Inventory{}, ValidationError{Field: "status", Message: "unsupported reward status"}
	}
	inventory, err := service.repository.ListInventory(ctx, userID, status, service.now())
	if err != nil {
		return Inventory{}, fmt.Errorf("list reward inventory: %w", err)
	}
	return inventory, nil
}

func (service *Service) Redeem(
	ctx context.Context,
	userID uuid.UUID,
	rewardID uuid.UUID,
	idempotencyKey uuid.UUID,
	categoryCode string,
	listingID string,
) (Reward, error) {
	categoryCode = strings.TrimSpace(categoryCode)
	listingID = strings.TrimSpace(listingID)
	reward, err := service.repository.Redeem(ctx, RedeemCommand{
		UserID:         userID,
		RewardID:       rewardID,
		IdempotencyKey: idempotencyKey,
		RequestHash:    redemptionHash(rewardID.String(), categoryCode, listingID),
		CategoryCode:   categoryCode,
		ListingID:      listingID,
		RedeemedAt:     service.now(),
	})
	if err != nil {
		return Reward{}, fmt.Errorf("redeem reward: %w", err)
	}
	return reward, nil
}

func (service *Service) SelectAward(
	ctx context.Context,
	userID uuid.UUID,
	awardID uuid.UUID,
	idempotencyKey uuid.UUID,
	optionCode string,
) (Reward, error) {
	optionCode = strings.TrimSpace(optionCode)
	if optionCode == "" {
		return Reward{}, ValidationError{Field: "option_code", Message: "option code is required"}
	}
	now := service.now()
	reward, err := service.repository.SelectAward(ctx, SelectAwardCommand{
		UserID:         userID,
		AwardID:        awardID,
		IdempotencyKey: idempotencyKey,
		RequestHash:    redemptionHash(awardID.String(), optionCode),
		OptionCode:     optionCode,
		SelectedAt:     now,
		ExpiresAt:      now.Add(rewardLifetime),
	})
	if err != nil {
		return Reward{}, fmt.Errorf("select leaderboard award: %w", err)
	}
	return reward, nil
}

func (service *Service) IssueLevel(
	ctx context.Context,
	userID uuid.UUID,
	level int,
	progressionCycle int,
	occurredAt time.Time,
) ([]Reward, error) {
	if level < 1 {
		return nil, ValidationError{Field: "level", Message: "level must be positive"}
	}
	if progressionCycle < 1 {
		return nil, ValidationError{Field: "progression_cycle", Message: "progression cycle must be positive"}
	}
	return service.issue(ctx, IssueCommand{
		UserID:       userID,
		TriggerType:  SourceLevel,
		TriggerValue: level,
		IssuanceKey:  fmt.Sprintf("level:%d:%d", progressionCycle, level),
		OccurredAt:   occurredAt,
		ExpiresAt:    occurredAt.Add(rewardLifetime),
	})
}

func (service *Service) IssueStreak(
	ctx context.Context,
	userID uuid.UUID,
	milestoneDays int,
	streakStartedOn time.Time,
	occurredAt time.Time,
) ([]Reward, error) {
	if milestoneDays < 1 {
		return nil, ValidationError{Field: "milestone_days", Message: "milestone days must be positive"}
	}
	return service.issue(ctx, IssueCommand{
		UserID:       userID,
		TriggerType:  SourceStreak,
		TriggerValue: milestoneDays,
		IssuanceKey:  fmt.Sprintf("streak:%s:%d", streakStartedOn.Format(time.DateOnly), milestoneDays),
		OccurredAt:   occurredAt,
		ExpiresAt:    occurredAt.Add(rewardLifetime),
	})
}

func (service *Service) issue(ctx context.Context, command IssueCommand) ([]Reward, error) {
	rewards, err := service.repository.Issue(ctx, command)
	if err != nil {
		return nil, fmt.Errorf("issue rewards: %w", err)
	}
	return rewards, nil
}

func validStatus(status string) bool {
	switch status {
	case StatusIssued, StatusRedeemed, StatusExpired, StatusRevoked:
		return true
	default:
		return false
	}
}

func redemptionHash(parts ...string) []byte {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return digest[:]
}
