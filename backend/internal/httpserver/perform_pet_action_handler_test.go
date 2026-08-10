package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/activity"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/progress"
)

type performActionServiceStub struct {
	result activity.PerformActionResult
	err    error

	calls      int
	lastParams activity.PerformActionParams
}

type petStatePublisherStub struct {
	calls  int
	userID uuid.UUID
	pet    pet.Pet
}

func (stub *petStatePublisherStub) Publish(userID uuid.UUID, value pet.Pet) {
	stub.calls++
	stub.userID = userID
	stub.pet = value
}

func (stub *performActionServiceStub) ListTypes(
	_ context.Context,
) ([]activity.Type, error) {
	return nil, nil
}

func (stub *performActionServiceStub) PerformAction(
	_ context.Context,
	params activity.PerformActionParams,
) (activity.PerformActionResult, error) {
	stub.calls++
	stub.lastParams = params

	return stub.result, stub.err
}

func TestPerformPetActionHandler(t *testing.T) {
	userID := uuid.New()
	idempotencyKey := uuid.New()
	occurredAt := time.Date(2026, 8, 8, 12, 30, 0, 0, time.UTC)
	createdAt := time.Date(2026, 8, 8, 12, 30, 1, 0, time.UTC)
	petCreatedAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	levelCreatedAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	lastActiveDate := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	streakUpdatedAt := time.Date(2026, 8, 8, 12, 30, 2, 0, time.UTC)
	petID := uuid.New()

	service := &performActionServiceStub{
		result: activity.PerformActionResult{
			Action: activity.Action{
				ID:                42,
				ActivityCode:      "feed",
				ExperienceAwarded: 10,
				StateDelta:        activity.StateDelta{Hunger: 20},
				OccurredAt:        occurredAt,
				CreatedAt:         createdAt,
			},
			Pet: pet.Pet{
				ID:                petID,
				Name:              "Авитоша",
				Species:           "avito_pet",
				Level:             2,
				Experience:        110,
				Health:            100,
				Hunger:            80,
				Happiness:         90,
				Energy:            70,
				StateVersion:      3,
				LastInteractionAt: &occurredAt,
				CreatedAt:         petCreatedAt,
				UpdatedAt:         occurredAt,
			},
			Level: progress.Level{
				Level:                   2,
				RequiredTotalExperience: 100,
				Title:                   "Знакомство",
				CreatedAt:               levelCreatedAt,
			},
			UserStreak: progress.UserStreak{
				CurrentDays:    3,
				LongestDays:    5,
				LastActiveDate: &lastActiveDate,
				UpdatedAt:      streakUpdatedAt,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := &petStatePublisherStub{}
	handler := performPetActionHandler(service, publisher, logger)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/v1/pet/actions",
		strings.NewReader(`{"activity_code":"feed"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey.String())
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

	if service.calls != 1 {
		t.Fatalf("PerformAction calls = %d, want 1", service.calls)
	}
	if publisher.calls != 1 {
		t.Fatalf("Publish calls = %d, want 1", publisher.calls)
	}
	if publisher.userID != userID {
		t.Errorf("published UserID = %s, want %s", publisher.userID, userID)
	}
	if publisher.pet.StateVersion != service.result.Pet.StateVersion {
		t.Errorf(
			"published StateVersion = %d, want %d",
			publisher.pet.StateVersion,
			service.result.Pet.StateVersion,
		)
	}

	if service.lastParams.UserID != userID {
		t.Errorf(
			"UserID = %s, want %s",
			service.lastParams.UserID,
			userID,
		)
	}

	if service.lastParams.ActivityCode != "feed" {
		t.Errorf(
			"ActivityCode = %q, want %q",
			service.lastParams.ActivityCode,
			"feed",
		)
	}

	if service.lastParams.IdempotencyKey != idempotencyKey {
		t.Errorf(
			"IdempotencyKey = %s, want %s",
			service.lastParams.IdempotencyKey,
			idempotencyKey,
		)
	}

	var got performPetActionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.PetAction.ID != 42 {
		t.Errorf("ID = %d, want 42", got.PetAction.ID)
	}

	if got.PetAction.ActivityCode != "feed" {
		t.Errorf(
			"ActivityCode = %q, want %q",
			got.PetAction.ActivityCode,
			"feed",
		)
	}

	if got.PetAction.ExperienceAwarded != 10 {
		t.Errorf(
			"ExperienceAwarded = %d, want 10",
			got.PetAction.ExperienceAwarded,
		)
	}

	if !got.PetAction.OccurredAt.Equal(occurredAt) {
		t.Errorf(
			"OccurredAt = %s, want %s",
			got.PetAction.OccurredAt,
			occurredAt,
		)
	}

	if !got.PetAction.CreatedAt.Equal(createdAt) {
		t.Errorf(
			"CreatedAt = %s, want %s",
			got.PetAction.CreatedAt,
			createdAt,
		)
	}

	if got.PetAction.StateDelta.Hunger != 20 {
		t.Errorf(
			"StateDelta.Hunger = %d, want 20",
			got.PetAction.StateDelta.Hunger,
		)
	}

	if got.Pet.ID != petID {
		t.Errorf("Pet.ID = %s, want %s", got.Pet.ID, petID)
	}

	if got.Pet.Experience != 110 {
		t.Errorf("Pet.Experience = %d, want 110", got.Pet.Experience)
	}

	if got.Pet.StateVersion != 3 {
		t.Errorf("Pet.StateVersion = %d, want 3", got.Pet.StateVersion)
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

func TestPerformPetActionHandlerEmptyActivityCode(t *testing.T) {
	service := &performActionServiceStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := performPetActionHandler(service, &petStatePublisherStub{}, logger)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/v1/pet/actions",
		strings.NewReader(`{"activity_code":"   "}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", uuid.NewString())
	request = request.WithContext(
		withUserID(request.Context(), uuid.New()),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			response.Code,
			http.StatusBadRequest,
			response.Body.String(),
		)
	}

	var got errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Error.Code != codeValidationFailed {
		t.Errorf(
			"error code = %q, want %q",
			got.Error.Code,
			codeValidationFailed,
		)
	}

	if got.Error.Field != "activity_code" {
		t.Errorf(
			"error field = %q, want %q",
			got.Error.Field,
			"activity_code",
		)
	}

	if service.calls != 0 {
		t.Errorf("PerformAction calls = %d, want 0", service.calls)
	}
}

func TestPerformPetActionHandlerInvalidIdempotencyKey(t *testing.T) {
	service := &performActionServiceStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := performPetActionHandler(service, &petStatePublisherStub{}, logger)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/v1/pet/actions",
		strings.NewReader(`{"activity_code":"feed"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "not-a-uuid")
	request = request.WithContext(
		withUserID(request.Context(), uuid.New()),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			response.Code,
			http.StatusBadRequest,
			response.Body.String(),
		)
	}

	var got errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Error.Code != codeValidationFailed {
		t.Errorf(
			"error code = %q, want %q",
			got.Error.Code,
			codeValidationFailed,
		)
	}

	if got.Error.Field != "Idempotency-Key" {
		t.Errorf(
			"error field = %q, want %q",
			got.Error.Field,
			"Idempotency-Key",
		)
	}

	if service.calls != 0 {
		t.Errorf("PerformAction calls = %d, want 0", service.calls)
	}
}

func TestPerformPetActionHandlerDomainErrors(t *testing.T) {
	testCases := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "activity type not found",
			serviceErr: activity.ErrTypeNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   codeInvalidActivityCode,
		},
		{
			name:       "activity inactive",
			serviceErr: activity.ErrActivityInactive,
			wantStatus: http.StatusConflict,
			wantCode:   codeActivityNotActive,
		},
		{
			name:       "cooldown active",
			serviceErr: activity.ErrCooldown,
			wantStatus: http.StatusConflict,
			wantCode:   codeCooldownActive,
		},
		{
			name:       "daily limit reached",
			serviceErr: activity.ErrDailyLimit,
			wantStatus: http.StatusConflict,
			wantCode:   codeDailyLimitReached,
		},
		{
			name:       "idempotency conflict",
			serviceErr: activity.ErrIdempotencyConflict,
			wantStatus: http.StatusConflict,
			wantCode:   codeIdempotencyConflict,
		},
		{
			name:       "pet not found",
			serviceErr: pet.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   codeNotFound,
		},
		{
			name:       "internal error",
			serviceErr: errors.New("database error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   codeInternalError,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := &performActionServiceStub{
				err: testCase.serviceErr,
			}

			logger := slog.New(
				slog.NewTextHandler(io.Discard, nil),
			)
			publisher := &petStatePublisherStub{}
			handler := performPetActionHandler(service, publisher, logger)

			request := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"/api/v1/pet/actions",
				strings.NewReader(`{"activity_code":"feed"}`),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(
				"Idempotency-Key",
				uuid.NewString(),
			)
			request = request.WithContext(
				withUserID(request.Context(), uuid.New()),
			)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != testCase.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					response.Code,
					testCase.wantStatus,
					response.Body.String(),
				)
			}

			var got errorResponse
			if err := json.Unmarshal(
				response.Body.Bytes(),
				&got,
			); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if got.Error.Code != testCase.wantCode {
				t.Errorf(
					"error code = %q, want %q",
					got.Error.Code,
					testCase.wantCode,
				)
			}

			if service.calls != 1 {
				t.Errorf(
					"PerformAction calls = %d, want 1",
					service.calls,
				)
			}
			if publisher.calls != 0 {
				t.Errorf("Publish calls = %d, want 0", publisher.calls)
			}
		})
	}
}

func TestPerformPetActionHandlerWithoutUserID(t *testing.T) {
	service := &performActionServiceStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := performPetActionHandler(service, &petStatePublisherStub{}, logger)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/v1/pet/actions",
		strings.NewReader(`{"activity_code":"feed"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", uuid.NewString())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			response.Code,
			http.StatusInternalServerError,
			response.Body.String(),
		)
	}

	if service.calls != 0 {
		t.Errorf("PerformAction calls = %d, want 0", service.calls)
	}
}
