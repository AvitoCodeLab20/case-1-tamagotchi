package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/rewards"
)

type rewardStub struct {
	inventory  rewards.Inventory
	reward     rewards.Reward
	err        error
	status     string
	userID     uuid.UUID
	resourceID uuid.UUID
	key        uuid.UUID
	value      string
}

func (stub *rewardStub) ListInventory(
	_ context.Context,
	userID uuid.UUID,
	status string,
) (rewards.Inventory, error) {
	stub.userID = userID
	stub.status = status
	return stub.inventory, stub.err
}

func (stub *rewardStub) Redeem(
	_ context.Context,
	userID uuid.UUID,
	rewardID uuid.UUID,
	idempotencyKey uuid.UUID,
	categoryCode string,
	listingID string,
) (rewards.Reward, error) {
	stub.userID = userID
	stub.resourceID = rewardID
	stub.key = idempotencyKey
	stub.value = categoryCode + ":" + listingID
	return stub.reward, stub.err
}

func (stub *rewardStub) SelectAward(
	_ context.Context,
	userID uuid.UUID,
	awardID uuid.UUID,
	idempotencyKey uuid.UUID,
	optionCode string,
) (rewards.Reward, error) {
	stub.userID = userID
	stub.resourceID = awardID
	stub.key = idempotencyKey
	stub.value = optionCode
	return stub.reward, stub.err
}

func withRewards(service rewardService) suiteOption {
	return func(cfg *suiteConfig) { cfg.rewards = service }
}

func TestListRewardsEndpoint(t *testing.T) {
	now := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	stub := &rewardStub{inventory: rewards.Inventory{Rewards: []rewards.Reward{{
		ID: uuid.New(), Code: "streak_7_delivery_15", Title: "Reward", Description: "Description",
		Source: rewards.SourceStreak, Status: rewards.StatusIssued,
		Benefit: rewards.Benefit{Type: "delivery_discount"}, IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}}}}
	s := newSuite(t, withRewards(stub))
	registered := s.registerUser(t)
	response := s.request(t, http.MethodGet, "/api/v1/rewards?status=issued",
		map[string]string{"Authorization": "Bearer " + registered.AccessToken}, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	result := rewardInventoryResponse{}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result.Rewards) != 1 || stub.status != rewards.StatusIssued {
		t.Fatalf("response = %+v, status filter = %q", result, stub.status)
	}
}

func TestRedeemRewardEndpoint(t *testing.T) {
	rewardID := uuid.New()
	key := uuid.New()
	stub := &rewardStub{reward: rewards.Reward{
		ID: rewardID, Code: "level_20_promotion_300", Source: rewards.SourceLevel,
		Status: rewards.StatusRedeemed, Benefit: rewards.Benefit{Type: "promotion_certificate"},
		IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}}
	s := newSuite(t, withRewards(stub))
	registered := s.registerUser(t)
	response := s.request(t, http.MethodPost, "/api/v1/rewards/"+rewardID.String()+"/redeem",
		map[string]string{
			"Authorization":   "Bearer " + registered.AccessToken,
			"Idempotency-Key": key.String(),
		}, `{"listing_id":"listing-1"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	if stub.resourceID != rewardID || stub.key != key || stub.value != ":listing-1" {
		t.Fatalf("service arguments = resource %s, key %s, value %q", stub.resourceID, stub.key, stub.value)
	}
}

func TestSelectLeaderboardAwardEndpointMapsConflict(t *testing.T) {
	stub := &rewardStub{err: rewards.ErrSelectionExpired}
	s := newSuite(t, withRewards(stub))
	registered := s.registerUser(t)
	response := s.request(t, http.MethodPost, "/api/v1/leaderboard/awards/"+uuid.NewString()+"/select",
		map[string]string{
			"Authorization":   "Bearer " + registered.AccessToken,
			"Idempotency-Key": uuid.NewString(),
		}, `{"option_code":"leaderboard_5_autoteka"}`)
	if response.Code != http.StatusConflict || decodeError(t, response).Code != codeSelectionExpired {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
}

func TestRewardEndpointsValidateIdentifiers(t *testing.T) {
	s := newSuite(t)
	registered := s.registerUser(t)
	testCases := []struct {
		path    string
		headers map[string]string
		body    string
	}{
		{path: "/api/v1/rewards/not-a-uuid/redeem", headers: map[string]string{
			"Authorization": "Bearer " + registered.AccessToken,
		}, body: `{}`},
		{path: "/api/v1/rewards/" + uuid.NewString() + "/redeem", headers: map[string]string{
			"Authorization": "Bearer " + registered.AccessToken,
		}, body: `{}`},
	}
	for _, testCase := range testCases {
		response := s.request(t, http.MethodPost, testCase.path, testCase.headers, testCase.body)
		if response.Code != http.StatusBadRequest {
			t.Errorf("path %q status = %d, want %d", testCase.path, response.Code, http.StatusBadRequest)
		}
	}
}

func TestRewardEndpointMapsNotFound(t *testing.T) {
	s := newSuite(t, withRewards(&rewardStub{err: errors.Join(errors.New("repository"), rewards.ErrNotFound)}))
	registered := s.registerUser(t)
	response := s.request(t, http.MethodGet, "/api/v1/rewards",
		map[string]string{"Authorization": "Bearer " + registered.AccessToken}, "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
