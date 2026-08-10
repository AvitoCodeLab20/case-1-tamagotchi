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

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/activity"
)

type activityServiceStub struct {
	types            []activity.Type
	err              error
	actionResult     activity.PerformActionResult
	performActionErr error
}

func (stub *activityServiceStub) ListTypes(
	_ context.Context,
) ([]activity.Type, error) {
	return stub.types, stub.err
}

func (stub *activityServiceStub) PerformAction(
	_ context.Context,
	_ activity.PerformActionParams,
) (activity.PerformActionResult, error) {
	return stub.actionResult, stub.performActionErr
}
func TestActivityTypesHandler(t *testing.T) {
	dailyLimit := 3

	service := &activityServiceStub{
		types: []activity.Type{
			{
				Code:            "feed",
				Title:           "Покормить",
				Description:     "Покормить питомца",
				Category:        "pet",
				BaseExperience:  10,
				DailyLimit:      &dailyLimit,
				CooldownSeconds: 3600,
			},
			{
				Code:            "play",
				Title:           "Поиграть",
				Description:     "Поиграть с питомцем",
				Category:        "pet",
				BaseExperience:  15,
				DailyLimit:      nil,
				CooldownSeconds: 7200,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := activityTypesHandler(service, logger)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/v1/activity-types",
		nil,
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

	var got activityTypesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(got.ActivityTypes) != 2 {
		t.Fatalf(
			"activity types count = %d, want 2",
			len(got.ActivityTypes),
		)
	}

	first := got.ActivityTypes[0]

	if first.Code != "feed" {
		t.Errorf("Code = %q, want %q", first.Code, "feed")
	}

	if first.Title != "Покормить" {
		t.Errorf("Title = %q, want %q", first.Title, "Покормить")
	}

	if first.Description != "Покормить питомца" {
		t.Errorf(
			"Description = %q, want %q",
			first.Description,
			"Покормить питомца",
		)
	}

	if first.Category != "pet" {
		t.Errorf("Category = %q, want %q", first.Category, "pet")
	}

	if first.BaseExperience != 10 {
		t.Errorf("BaseExperience = %d, want 10", first.BaseExperience)
	}

	if first.DailyLimit == nil || *first.DailyLimit != 3 {
		t.Errorf("DailyLimit = %v, want 3", first.DailyLimit)
	}

	if first.CooldownSeconds != 3600 {
		t.Errorf("CooldownSeconds = %d, want 3600", first.CooldownSeconds)
	}

	if got.ActivityTypes[1].DailyLimit != nil {
		t.Errorf(
			"second DailyLimit = %v, want nil",
			got.ActivityTypes[1].DailyLimit,
		)
	}
}

func TestActivityTypesHandlerServiceError(t *testing.T) {
	service := &activityServiceStub{
		err: errors.New("database error"),
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := activityTypesHandler(service, logger)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/v1/activity-types",
		nil,
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Errorf(
			"status = %d, want %d; body = %s",
			response.Code,
			http.StatusInternalServerError,
			response.Body.String(),
		)
	}
}
