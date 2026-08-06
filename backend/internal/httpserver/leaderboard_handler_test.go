package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/leaderboard"
)

type leaderboardStub struct {
	board  leaderboard.Board
	err    error
	userID uuid.UUID
	limit  int
}

func (stub *leaderboardStub) Current(
	_ context.Context,
	userID uuid.UUID,
	limit int,
) (leaderboard.Board, error) {
	stub.userID = userID
	stub.limit = limit

	return stub.board, stub.err
}

func withLeaderboard(service leaderboardService) suiteOption {
	return func(cfg *suiteConfig) { cfg.leaderboard = service }
}

func TestCurrentLeaderboardEndpoint(t *testing.T) {
	currentUserID := uuid.Nil
	stub := &leaderboardStub{}
	s := newSuite(t, withLeaderboard(stub))
	registered := s.registerUser(t)
	currentUserID = uuid.MustParse(registered.User.ID)

	periodStart := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.FixedZone("Europe/Moscow", 3*60*60))
	stub.board = leaderboard.Board{
		Period: leaderboard.Period{
			StartsAt: periodStart,
			EndsAt:   periodStart.AddDate(0, 0, 7),
			Timezone: "Europe/Moscow",
		},
		Status:            "in_progress",
		ParticipantsCount: 1,
		Entries: []leaderboard.Entry{{
			Participant: leaderboard.Participant{
				UserID:           currentUserID,
				DisplayName:      "Игрок",
				WeeklyExperience: 120,
			},
			Rank:      1,
			PrizeTier: 5,
		}},
	}

	response := s.request(t, http.MethodGet, "/api/v1/leaderboard/current?limit=25",
		map[string]string{"Authorization": "Bearer " + registered.AccessToken}, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}

	result := leaderboardResponse{}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if stub.userID != currentUserID || stub.limit != 25 {
		t.Fatalf("service called with user %s and limit %d", stub.userID, stub.limit)
	}
	if len(result.Entries) != 1 || !result.Entries[0].IsCurrentUser {
		t.Fatalf("entries = %+v, want the current user", result.Entries)
	}
	if result.Entries[0].PrizeTier == nil || *result.Entries[0].PrizeTier != 5 {
		t.Fatalf("prize tier = %v, want 5", result.Entries[0].PrizeTier)
	}
}

func TestCurrentLeaderboardEndpointRequiresAuthentication(t *testing.T) {
	s := newSuite(t)

	response := s.request(t, http.MethodGet, "/api/v1/leaderboard/current", nil, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestCurrentLeaderboardEndpointValidatesLimit(t *testing.T) {
	s := newSuite(t)
	registered := s.registerUser(t)

	for _, limit := range []string{"0", "101", "wrong"} {
		response := s.request(t, http.MethodGet, "/api/v1/leaderboard/current?limit="+limit,
			map[string]string{"Authorization": "Bearer " + registered.AccessToken}, "")
		if response.Code != http.StatusBadRequest {
			t.Errorf("limit %q status = %d, want %d", limit, response.Code, http.StatusBadRequest)
		}
		if field := decodeError(t, response).Field; field != "limit" {
			t.Errorf("limit %q field = %q, want limit", limit, field)
		}
	}
}

func TestCurrentLeaderboardEndpointMapsServiceFailure(t *testing.T) {
	s := newSuite(t, withLeaderboard(&leaderboardStub{err: errors.New("database unavailable")}))
	registered := s.registerUser(t)

	response := s.request(t, http.MethodGet, "/api/v1/leaderboard/current",
		map[string]string{"Authorization": "Bearer " + registered.AccessToken}, "")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}
