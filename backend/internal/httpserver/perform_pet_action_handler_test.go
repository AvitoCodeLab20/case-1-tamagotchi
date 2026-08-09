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
)

type performActionServiceStub struct {
	result activity.PerformActionResult
	err    error

	calls      int
	lastParams activity.PerformActionParams
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

	service := &performActionServiceStub{
		result: activity.PerformActionResult{
			Action: activity.Action{
				ID:                42,
				UserID:            userID,
				ActivityCode:      "feed",
				ExperienceAwarded: 10,
				StateDelta: activity.StateDelta{
					Hunger: 20,
				},
				IdempotencyKey: idempotencyKey,
				OccurredAt:     occurredAt,
				CreatedAt:      createdAt,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := performPetActionHandler(service, logger)

	request := httptest.NewRequest(
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
}

func TestPerformPetActionHandlerEmptyActivityCode(t *testing.T) {
	service := &performActionServiceStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := performPetActionHandler(service, logger)

	request := httptest.NewRequest(
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
	handler := performPetActionHandler(service, logger)

	request := httptest.NewRequest(
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
			handler := performPetActionHandler(service, logger)

			request := httptest.NewRequest(
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
		})
	}
}

func TestPerformPetActionHandlerWithoutUserID(t *testing.T) {
	service := &performActionServiceStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := performPetActionHandler(service, logger)

	request := httptest.NewRequest(
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
