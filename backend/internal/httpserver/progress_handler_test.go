package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/progress"
)

type progressServiceStub struct {
	result progress.Progress
	err    error

	calls      int
	lastUserID uuid.UUID
}

func (stub *progressServiceStub) Get(
	_ context.Context,
	userID uuid.UUID,
) (progress.Progress, error) {
	stub.calls++
	stub.lastUserID = userID

	return stub.result, stub.err
}

func TestProgressHandler(t *testing.T) {
	userID := uuid.New()

	lastActiveDate := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 8, 8, 14, 30, 0, 0, time.UTC)

	service := &progressServiceStub{
		result: progress.Progress{
			Level:                   3,
			Experience:              320,
			RequiredTotalExperience: 250,
			Title:                   "Заботливый хозяин",
			UserStreak: progress.UserStreak{
				CurrentDays:    2,
				LongestDays:    5,
				LastActiveDate: &lastActiveDate,
				UpdatedAt:      updatedAt,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := progressHandler(service, logger)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/progress",
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

	if service.calls != 1 {
		t.Fatalf("Get calls = %d, want 1", service.calls)
	}

	if service.lastUserID != userID {
		t.Errorf(
			"UserID = %s, want %s",
			service.lastUserID,
			userID,
		)
	}

	var got progressResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Level != 3 {
		t.Errorf("Level = %d, want 3", got.Level)
	}

	if got.Experience != 320 {
		t.Errorf("Experience = %d, want 320", got.Experience)
	}

	if got.RequiredTotalExperience != 250 {
		t.Errorf(
			"RequiredTotalExperience = %d, want 250",
			got.RequiredTotalExperience,
		)
	}

	if got.Title != "Заботливый хозяин" {
		t.Errorf(
			"Title = %q, want %q",
			got.Title,
			"Заботливый хозяин",
		)
	}

	if got.UserStreak.CurrentDays != 2 {
		t.Errorf(
			"CurrentDays = %d, want 2",
			got.UserStreak.CurrentDays,
		)
	}

	if got.UserStreak.LongestDays != 5 {
		t.Errorf(
			"LongestDays = %d, want 5",
			got.UserStreak.LongestDays,
		)
	}

	if got.UserStreak.LastActiveDate == nil ||
		*got.UserStreak.LastActiveDate != "2026-08-08" {
		t.Errorf(
			"LastActiveDate = %v, want 2026-08-08",
			got.UserStreak.LastActiveDate,
		)
	}

	if !got.UserStreak.UpdatedAt.Equal(updatedAt) {
		t.Errorf(
			"UpdatedAt = %s, want %s",
			got.UserStreak.UpdatedAt,
			updatedAt,
		)
	}
}

func TestProgressHandlerNotFound(t *testing.T) {
	service := &progressServiceStub{
		err: progress.ErrNotFound,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := progressHandler(service, logger)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/progress",
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
}

func TestProgressHandlerServiceError(t *testing.T) {
	service := &progressServiceStub{
		err: errors.New("database error"),
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := progressHandler(service, logger)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/progress",
		nil,
	)
	request = request.WithContext(
		withUserID(request.Context(), uuid.New()),
	)

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
}
