package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/progress"
)

type petServiceStub struct {
	result pet.Pet
	err    error

	calls      int
	lastUserID uuid.UUID
}

func (stub *petServiceStub) Get(
	_ context.Context,
	userID uuid.UUID,
) (pet.Pet, error) {
	stub.calls++
	stub.lastUserID = userID

	return stub.result, stub.err
}

func TestPetHandler(t *testing.T) {
	userID := uuid.New()
	petID := uuid.New()

	lastInteractionAt := time.Date(
		2026, 8, 8, 12, 30, 0, 0, time.UTC,
	)
	petCreatedAt := time.Date(
		2026, 8, 1, 9, 0, 0, 0, time.UTC,
	)
	petUpdatedAt := time.Date(
		2026, 8, 8, 12, 30, 1, 0, time.UTC,
	)
	levelCreatedAt := time.Date(
		2026, 7, 1, 9, 0, 0, 0, time.UTC,
	)
	lastActiveDate := time.Date(
		2026, 8, 8, 0, 0, 0, 0, time.UTC,
	)
	streakUpdatedAt := time.Date(
		2026, 8, 8, 12, 30, 2, 0, time.UTC,
	)

	petService := &petServiceStub{
		result: pet.Pet{
			ID:                petID,
			UserID:            userID,
			Name:              "Авитоша",
			Species:           "avito_pet",
			Level:             2,
			Experience:        110,
			Health:            100,
			Hunger:            80,
			Happiness:         90,
			Energy:            70,
			StateVersion:      3,
			LastInteractionAt: &lastInteractionAt,
			CreatedAt:         petCreatedAt,
			UpdatedAt:         petUpdatedAt,
		},
	}

	progressService := &progressServiceStub{
		result: progress.Progress{
			Level:                   2,
			Experience:              110,
			RequiredTotalExperience: 100,
			Title:                   "Знакомство",
			LevelCreatedAt:          levelCreatedAt,
			UserStreak: progress.UserStreak{
				CurrentDays:    3,
				LongestDays:    5,
				LastActiveDate: &lastActiveDate,
				UpdatedAt:      streakUpdatedAt,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := petHandler(petService, progressService, logger)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/v1/pet",
		nil,
	)
	request = request.WithContext(
		withUserID(request.Context(), userID),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			response.Code,
			http.StatusOK,
			response.Body.String(),
		)
	}

	if petService.calls != 1 {
		t.Errorf("pet Get calls = %d, want 1", petService.calls)
	}

	if petService.lastUserID != userID {
		t.Errorf(
			"pet UserID = %s, want %s",
			petService.lastUserID,
			userID,
		)
	}

	if progressService.calls != 1 {
		t.Errorf(
			"progress Get calls = %d, want 1",
			progressService.calls,
		)
	}

	if progressService.lastUserID != userID {
		t.Errorf(
			"progress UserID = %s, want %s",
			progressService.lastUserID,
			userID,
		)
	}

	var got getPetResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Pet.ID != petID {
		t.Errorf("Pet.ID = %s, want %s", got.Pet.ID, petID)
	}

	if got.Pet.Name != "Авитоша" {
		t.Errorf("Pet.Name = %q, want %q", got.Pet.Name, "Авитоша")
	}

	if got.Pet.Experience != 110 {
		t.Errorf("Pet.Experience = %d, want 110", got.Pet.Experience)
	}

	if !got.Pet.CreatedAt.Equal(petCreatedAt) {
		t.Errorf(
			"Pet.CreatedAt = %s, want %s",
			got.Pet.CreatedAt,
			petCreatedAt,
		)
	}

	if got.Level.Level != 2 {
		t.Errorf("Level.Level = %d, want 2", got.Level.Level)
	}

	if got.Level.RequiredTotalExperience != 100 {
		t.Errorf(
			"Level.RequiredTotalExperience = %d, want 100",
			got.Level.RequiredTotalExperience,
		)
	}

	if got.Level.Title != "Знакомство" {
		t.Errorf(
			"Level.Title = %q, want %q",
			got.Level.Title,
			"Знакомство",
		)
	}

	if !got.Level.CreatedAt.Equal(levelCreatedAt) {
		t.Errorf(
			"Level.CreatedAt = %s, want %s",
			got.Level.CreatedAt,
			levelCreatedAt,
		)
	}

	if got.UserStreak.CurrentDays != 3 {
		t.Errorf(
			"UserStreak.CurrentDays = %d, want 3",
			got.UserStreak.CurrentDays,
		)
	}

	if got.UserStreak.LongestDays != 5 {
		t.Errorf(
			"UserStreak.LongestDays = %d, want 5",
			got.UserStreak.LongestDays,
		)
	}

	if got.UserStreak.LastActiveDate == nil ||
		*got.UserStreak.LastActiveDate != "2026-08-08" {
		t.Errorf(
			"UserStreak.LastActiveDate = %v, want 2026-08-08",
			got.UserStreak.LastActiveDate,
		)
	}

	if !got.UserStreak.UpdatedAt.Equal(streakUpdatedAt) {
		t.Errorf(
			"UserStreak.UpdatedAt = %s, want %s",
			got.UserStreak.UpdatedAt,
			streakUpdatedAt,
		)
	}
}

func TestPetHandlerPetNotFound(t *testing.T) {
	petService := &petServiceStub{
		err: pet.ErrNotFound,
	}
	progressService := &progressServiceStub{}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := petHandler(petService, progressService, logger)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/v1/pet",
		nil,
	)
	request = request.WithContext(
		withUserID(request.Context(), uuid.New()),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			response.Code,
			http.StatusNotFound,
			response.Body.String(),
		)
	}

	var got errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Error.Code != codeNotFound {
		t.Errorf(
			"error code = %q, want %q",
			got.Error.Code,
			codeNotFound,
		)
	}

	if progressService.calls != 0 {
		t.Errorf(
			"progress Get calls = %d, want 0",
			progressService.calls,
		)
	}
}
