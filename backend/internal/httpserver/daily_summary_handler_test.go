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

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/dailysummary"
)

type dailySummaryHandlerServiceStub struct {
	result dailysummary.DailySummary
	err    error

	calls       int
	userID      uuid.UUID
	summaryDate time.Time
}

func (stub *dailySummaryHandlerServiceStub) Current(
	_ context.Context,
	userID uuid.UUID,
) (dailysummary.DailySummary, error) {
	stub.calls++
	stub.userID = userID

	return stub.result, stub.err
}

func (stub *dailySummaryHandlerServiceStub) ByDate(
	_ context.Context,
	userID uuid.UUID,
	summaryDate time.Time,
) (dailysummary.DailySummary, error) {
	stub.calls++
	stub.userID = userID
	stub.summaryDate = summaryDate

	return stub.result, stub.err
}

func TestCurrentDailySummaryHandler(t *testing.T) {
	userID := uuid.New()

	generatedAt := time.Date(
		2026,
		time.August,
		9,
		15,
		30,
		0,
		0,
		time.UTC,
	)

	service := &dailySummaryHandlerServiceStub{
		result: dailysummary.DailySummary{
			ID: 42,
			SummaryDate: time.Date(
				2026,
				time.August,
				9,
				0,
				0,
				0,
				0,
				time.UTC,
			),
			ActionCount:      3,
			ExperienceEarned: 35,
			LevelsGained:     1,
			StateBefore: dailysummary.PetState{
				Level:      1,
				Experience: 90,
				Health:     80,
				Hunger:     70,
				Happiness:  60,
				Energy:     50,
			},
			StateAfter: dailysummary.PetState{
				Level:      2,
				Experience: 125,
				Health:     90,
				Hunger:     90,
				Happiness:  80,
				Energy:     40,
			},
			GeneratedAt: generatedAt,
		},
	}

	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	handler := currentDailySummaryHandler(service, logger)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/daily-summaries/current",
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
		t.Errorf(
			"Current() calls = %d, want 1",
			service.calls,
		)
	}

	if service.userID != userID {
		t.Errorf(
			"Current() userID = %s, want %s",
			service.userID,
			userID,
		)
	}

	var got dailySummaryResponse

	if err := json.Unmarshal(
		response.Body.Bytes(),
		&got,
	); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.ID != 42 {
		t.Errorf("ID = %d, want 42", got.ID)
	}

	if got.SummaryDate != "2026-08-09" {
		t.Errorf(
			"SummaryDate = %q, want %q",
			got.SummaryDate,
			"2026-08-09",
		)
	}

	if got.ActionCount != 3 {
		t.Errorf(
			"ActionCount = %d, want 3",
			got.ActionCount,
		)
	}

	if got.ExperienceEarned != 35 {
		t.Errorf(
			"ExperienceEarned = %d, want 35",
			got.ExperienceEarned,
		)
	}

	if got.LevelsGained != 1 {
		t.Errorf(
			"LevelsGained = %d, want 1",
			got.LevelsGained,
		)
	}

	if got.StateBefore.Experience != 90 {
		t.Errorf(
			"StateBefore.Experience = %d, want 90",
			got.StateBefore.Experience,
		)
	}

	if got.StateAfter.Level != 2 {
		t.Errorf(
			"StateAfter.Level = %d, want 2",
			got.StateAfter.Level,
		)
	}

	if !got.GeneratedAt.Equal(generatedAt) {
		t.Errorf(
			"GeneratedAt = %s, want %s",
			got.GeneratedAt,
			generatedAt,
		)
	}
}

func TestCurrentDailySummaryHandlerNotFound(t *testing.T) {
	service := &dailySummaryHandlerServiceStub{
		err: dailysummary.ErrNotFound,
	}

	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	handler := currentDailySummaryHandler(service, logger)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/daily-summaries/current",
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

	if err := json.Unmarshal(
		response.Body.Bytes(),
		&got,
	); err != nil {
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

func TestCurrentDailySummaryHandlerInternalError(t *testing.T) {
	service := &dailySummaryHandlerServiceStub{
		err: errors.New("database error"),
	}

	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	handler := currentDailySummaryHandler(service, logger)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/daily-summaries/current",
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

	var got errorResponse

	if err := json.Unmarshal(
		response.Body.Bytes(),
		&got,
	); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Error.Code != codeInternalError {
		t.Errorf(
			"error code = %q, want %q",
			got.Error.Code,
			codeInternalError,
		)
	}
}

func TestCurrentDailySummaryHandlerWithoutUserID(t *testing.T) {
	service := &dailySummaryHandlerServiceStub{}

	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	handler := currentDailySummaryHandler(service, logger)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/daily-summaries/current",
		nil,
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

	if service.calls != 0 {
		t.Errorf(
			"Current() calls = %d, want 0",
			service.calls,
		)
	}
}

func TestDailySummaryByDateHandler(t *testing.T) {
	userID := uuid.New()
	generatedAt := time.Date(2026, time.August, 9, 15, 0, 0, 0, time.UTC)
	service := &dailySummaryHandlerServiceStub{
		result: dailysummary.DailySummary{
			ID:               42,
			SummaryDate:      time.Date(2026, time.August, 8, 0, 0, 0, 0, time.UTC),
			ActionCount:      2,
			ExperienceEarned: 20,
			LevelsGained:     1,
			StateBefore: dailysummary.PetState{
				Level:      1,
				Experience: 90,
				Health:     80,
				Hunger:     70,
				Happiness:  60,
				Energy:     50,
			},
			StateAfter: dailysummary.PetState{
				Level:      2,
				Experience: 110,
				Health:     80,
				Hunger:     90,
				Happiness:  60,
				Energy:     50,
			},
			GeneratedAt: generatedAt,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := dailySummaryByDateHandler(service, logger)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/daily-summaries/2026-08-08",
		nil,
	)
	request.SetPathValue("summary_date", "2026-08-08")
	request = request.WithContext(withUserID(request.Context(), userID))
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
		t.Errorf("ByDate() calls = %d, want 1", service.calls)
	}
	if service.userID != userID {
		t.Errorf("UserID = %s, want %s", service.userID, userID)
	}

	wantDate := time.Date(2026, time.August, 8, 0, 0, 0, 0, time.UTC)
	if !service.summaryDate.Equal(wantDate) {
		t.Errorf(
			"SummaryDate argument = %s, want %s",
			service.summaryDate,
			wantDate,
		)
	}

	var got dailySummaryResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.ID != 42 {
		t.Errorf("ID = %d, want 42", got.ID)
	}
	if got.SummaryDate != "2026-08-08" {
		t.Errorf("SummaryDate = %q, want %q", got.SummaryDate, "2026-08-08")
	}
	if got.ActionCount != 2 {
		t.Errorf("ActionCount = %d, want 2", got.ActionCount)
	}
	if got.ExperienceEarned != 20 {
		t.Errorf("ExperienceEarned = %d, want 20", got.ExperienceEarned)
	}
	if got.LevelsGained != 1 {
		t.Errorf("LevelsGained = %d, want 1", got.LevelsGained)
	}
	if got.StateBefore.Experience != 90 {
		t.Errorf("StateBefore.Experience = %d, want 90", got.StateBefore.Experience)
	}
	if got.StateAfter.Level != 2 {
		t.Errorf("StateAfter.Level = %d, want 2", got.StateAfter.Level)
	}
	if !got.GeneratedAt.Equal(generatedAt) {
		t.Errorf("GeneratedAt = %s, want %s", got.GeneratedAt, generatedAt)
	}
}
