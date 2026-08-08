package storage_test

import (
	"context"
	"testing"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/storage"
)

func TestActivityRepositoryListActive(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewActivityRepository(pool)

	got, err := repository.ListActive(context.Background())
	if err != nil {
		t.Fatalf("ListActive() error = %v", err)
	}

	if len(got) != 6 {
		t.Fatalf("ListActive() returned %d activities, want 6", len(got))
	}

	wantCodes := []string{
		"browse_listings",
		"feed",
		"play",
		"publish_listing",
		"rest",
		"save_listing",
	}

	for i, wantCode := range wantCodes {
		if got[i].Code != wantCode {
			t.Errorf("activity[%d].Code = %q, want %q", i, got[i].Code, wantCode)
		}
	}
}

func TestActivityRepositoryListActiveSkipsInactive(t *testing.T) {
	pool := newPool(t)
	repository := storage.NewActivityRepository(pool)
	ctx := context.Background()

	_, err := pool.Exec(
		ctx,
		`UPDATE activity_types SET is_active = FALSE WHERE code = 'feed'`,
	)
	if err != nil {
		t.Fatalf("disable feed activity: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			`UPDATE activity_types SET is_active = TRUE WHERE code = 'feed'`,
		)
		if err != nil {
			t.Errorf("restore feed activity: %v", err)
		}
	})

	got, err := repository.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive() error = %v", err)
	}

	if len(got) != 5 {
		t.Fatalf("ListActive() returned %d activities, want 5", len(got))
	}

	for _, activityType := range got {
		if activityType.Code == "feed" {
			t.Error("ListActive() returned inactive activity feed")
		}
	}
}
