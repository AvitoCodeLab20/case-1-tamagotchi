package rewards

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repositoryStub struct {
	inventory    Inventory
	reward       Reward
	issued       []Reward
	err          error
	redeem       RedeemCommand
	selection    SelectAwardCommand
	issueCommand IssueCommand
}

func (stub *repositoryStub) ListInventory(
	context.Context,
	uuid.UUID,
	string,
	time.Time,
) (Inventory, error) {
	return stub.inventory, stub.err
}

func (stub *repositoryStub) Redeem(_ context.Context, command RedeemCommand) (Reward, error) {
	stub.redeem = command
	return stub.reward, stub.err
}

func (stub *repositoryStub) SelectAward(_ context.Context, command SelectAwardCommand) (Reward, error) {
	stub.selection = command
	return stub.reward, stub.err
}

func (stub *repositoryStub) Issue(_ context.Context, command IssueCommand) ([]Reward, error) {
	stub.issueCommand = command
	return stub.issued, stub.err
}

func TestListInventoryValidatesStatus(t *testing.T) {
	t.Parallel()
	service, err := NewService(&repositoryStub{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	_, err = service.ListInventory(context.Background(), uuid.New(), "wrong")
	validationError := ValidationError{}
	if !errors.As(err, &validationError) || validationError.Field != "status" {
		t.Fatalf("ListInventory() error = %v, want status validation error", err)
	}
}

func TestRedeemBuildsStableIdempotentCommand(t *testing.T) {
	t.Parallel()
	repository := &repositoryStub{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	userID := uuid.New()
	rewardID := uuid.New()
	key := uuid.New()
	if _, err = service.Redeem(context.Background(), userID, rewardID, key, " category ", " listing "); err != nil {
		t.Fatalf("Redeem() error = %v", err)
	}
	if repository.redeem.UserID != userID || repository.redeem.RewardID != rewardID {
		t.Fatalf("Redeem() command = %+v", repository.redeem)
	}
	if repository.redeem.CategoryCode != "category" || repository.redeem.ListingID != "listing" {
		t.Fatalf("Redeem() context = %q and %q", repository.redeem.CategoryCode, repository.redeem.ListingID)
	}
	if repository.redeem.RedeemedAt != now || len(repository.redeem.RequestHash) != sha256.Size {
		t.Fatalf("Redeem() time/hash = %s/%x", repository.redeem.RedeemedAt, repository.redeem.RequestHash)
	}
}

func TestIssueLevelUsesProgressionCycle(t *testing.T) {
	t.Parallel()
	repository := &repositoryStub{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	occurredAt := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	if _, err = service.IssueLevel(context.Background(), uuid.New(), 5, 2, occurredAt); err != nil {
		t.Fatalf("IssueLevel() error = %v", err)
	}
	if repository.issueCommand.IssuanceKey != "level:2:5" || repository.issueCommand.TriggerType != SourceLevel {
		t.Fatalf("IssueLevel() command = %+v", repository.issueCommand)
	}
	if repository.issueCommand.ExpiresAt != occurredAt.Add(rewardLifetime) {
		t.Fatalf("ExpiresAt = %s", repository.issueCommand.ExpiresAt)
	}
}

func TestIssueStreakUsesStartDate(t *testing.T) {
	t.Parallel()
	repository := &repositoryStub{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	startedAt := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	if _, err = service.IssueStreak(context.Background(), uuid.New(), 7, startedAt, startedAt.AddDate(0, 0, 6)); err != nil {
		t.Fatalf("IssueStreak() error = %v", err)
	}
	if repository.issueCommand.IssuanceKey != "streak:2026-08-01:7" {
		t.Fatalf("IssuanceKey = %q", repository.issueCommand.IssuanceKey)
	}
}

func TestRewardValidatesRedemptionContext(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		reward    Reward
		category  string
		listing   string
		wantField string
	}{
		{
			name: "category required", reward: Reward{Benefit: Benefit{
				Type: "category_discount", CategorySelectionRequired: true,
			}}, wantField: "category_code",
		},
		{
			name: "listing required", reward: Reward{Benefit: Benefit{
				Type: "promotion_certificate",
			}}, wantField: "listing_id",
		},
		{
			name: "valid category", reward: Reward{Benefit: Benefit{
				Type: "category_discount", CategorySelectionRequired: true,
			}}, category: "cars",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.reward.ValidateRedemption(testCase.category, testCase.listing)
			if testCase.wantField == "" {
				if err != nil {
					t.Fatalf("ValidateRedemption() error = %v", err)
				}
				return
			}
			validationError := ValidationError{}
			if !errors.As(err, &validationError) || validationError.Field != testCase.wantField {
				t.Fatalf("ValidateRedemption() error = %v, want field %q", err, testCase.wantField)
			}
		})
	}
}
