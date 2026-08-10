package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/dailysummary"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/storage"
)

func TestDailySummaryRepositoryByUserIDAndDate(t *testing.T) {
	pool := newPool(t)
	user := createUser(t, pool)
	repository := storage.NewDailySummaryRepository(pool)
	ctx := context.Background()

	summaryDate := time.Date(
		2026, 8, 9,
		0, 0, 0, 0,
		time.UTC,
	)
	generatedAt := time.Date(
		2026, 8, 9,
		18, 30, 0, 0,
		time.UTC,
	)

	const stateBefore = `{
		"level": 1,
		"experience": 90,
		"health": 100,
		"hunger": 70,
		"happiness": 80,
		"energy": 60
	}`

	const stateAfter = `{
		"level": 2,
		"experience": 115,
		"health": 100,
		"hunger": 90,
		"happiness": 85,
		"energy": 55
	}`

	_, err := pool.Exec(
		ctx,
		`
			INSERT INTO daily_summaries (
				user_id,
				summary_date,
				action_count,
				experience_earned,
				levels_gained,
				state_before,
				state_after,
				generated_at
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5,
				$6::jsonb,
				$7::jsonb,
				$8
			)
		`,
		user.ID,
		summaryDate,
		3,
		25,
		1,
		stateBefore,
		stateAfter,
		generatedAt,
	)
	if err != nil {
		t.Fatalf("insert daily summary: %v", err)
	}

	got, err := repository.ByUserIDAndDate(
		ctx,
		user.ID,
		summaryDate,
	)
	if err != nil {
		t.Fatalf("ByUserIDAndDate() error = %v", err)
	}

	if got.ID == 0 {
		t.Error("ID = 0, want non-zero")
	}

	if !got.SummaryDate.Equal(summaryDate) {
		t.Errorf(
			"SummaryDate = %s, want %s",
			got.SummaryDate,
			summaryDate,
		)
	}

	if got.ActionCount != 3 {
		t.Errorf("ActionCount = %d, want 3", got.ActionCount)
	}

	if got.ExperienceEarned != 25 {
		t.Errorf(
			"ExperienceEarned = %d, want 25",
			got.ExperienceEarned,
		)
	}

	if got.LevelsGained != 1 {
		t.Errorf("LevelsGained = %d, want 1", got.LevelsGained)
	}

	wantStateBefore := dailysummary.PetState{
		Level:      1,
		Experience: 90,
		Health:     100,
		Hunger:     70,
		Happiness:  80,
		Energy:     60,
	}

	if got.StateBefore != wantStateBefore {
		t.Errorf(
			"StateBefore = %+v, want %+v",
			got.StateBefore,
			wantStateBefore,
		)
	}

	wantStateAfter := dailysummary.PetState{
		Level:      2,
		Experience: 115,
		Health:     100,
		Hunger:     90,
		Happiness:  85,
		Energy:     55,
	}

	if got.StateAfter != wantStateAfter {
		t.Errorf(
			"StateAfter = %+v, want %+v",
			got.StateAfter,
			wantStateAfter,
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

func TestDailySummaryRepositoryNotFound(t *testing.T) {
	pool := newPool(t)
	user := createUser(t, pool)
	repository := storage.NewDailySummaryRepository(pool)

	summaryDate := time.Date(
		2026, 8, 9,
		0, 0, 0, 0,
		time.UTC,
	)

	_, err := repository.ByUserIDAndDate(
		context.Background(),
		user.ID,
		summaryDate,
	)

	if !errors.Is(err, dailysummary.ErrNotFound) {
		t.Errorf(
			"ByUserIDAndDate() error = %v, want %v",
			err,
			dailysummary.ErrNotFound,
		)
	}
}

func TestDailySummaryRepositoryUpsertAction(t *testing.T) {
	pool := newPool(t)
	user := createUser(t, pool)
	repository := storage.NewDailySummaryRepository(pool)
	ctx := context.Background()

	summaryDate := time.Date(
		2026, time.August, 9,
		0, 0, 0, 0,
		time.UTC,
	)

	firstGeneratedAt := time.Date(
		2026, time.August, 9,
		12, 0, 0, 0,
		time.UTC,
	)

	initialState := dailysummary.PetState{
		Level:      1,
		Experience: 90,
		Health:     100,
		Hunger:     70,
		Happiness:  80,
		Energy:     60,
	}

	firstStateAfter := dailysummary.PetState{
		Level:      1,
		Experience: 110,
		Health:     100,
		Hunger:     90,
		Happiness:  80,
		Energy:     60,
	}

	err := repository.UpsertAction(
		ctx,
		dailysummary.UpsertParams{
			UserID:           user.ID,
			SummaryDate:      summaryDate,
			ExperienceEarned: 20,
			LevelsGained:     0,
			StateBefore:      initialState,
			StateAfter:       firstStateAfter,
			GeneratedAt:      firstGeneratedAt,
		},
	)
	if err != nil {
		t.Fatalf("first UpsertAction() error = %v", err)
	}

	created, err := repository.ByUserIDAndDate(
		ctx,
		user.ID,
		summaryDate,
	)
	if err != nil {
		t.Fatalf("read created summary: %v", err)
	}

	if created.ActionCount != 1 {
		t.Errorf(
			"created ActionCount = %d, want 1",
			created.ActionCount,
		)
	}

	if created.ExperienceEarned != 20 {
		t.Errorf(
			"created ExperienceEarned = %d, want 20",
			created.ExperienceEarned,
		)
	}

	if created.LevelsGained != 0 {
		t.Errorf(
			"created LevelsGained = %d, want 0",
			created.LevelsGained,
		)
	}

	if created.StateBefore != initialState {
		t.Errorf(
			"created StateBefore = %+v, want %+v",
			created.StateBefore,
			initialState,
		)
	}

	if created.StateAfter != firstStateAfter {
		t.Errorf(
			"created StateAfter = %+v, want %+v",
			created.StateAfter,
			firstStateAfter,
		)
	}

	if !created.GeneratedAt.Equal(firstGeneratedAt) {
		t.Errorf(
			"created GeneratedAt = %s, want %s",
			created.GeneratedAt,
			firstGeneratedAt,
		)
	}

	secondGeneratedAt := time.Date(
		2026, time.August, 9,
		18, 30, 0, 0,
		time.UTC,
	)

	secondStateAfter := dailysummary.PetState{
		Level:      2,
		Experience: 130,
		Health:     100,
		Hunger:     90,
		Happiness:  95,
		Energy:     40,
	}

	err = repository.UpsertAction(
		ctx,
		dailysummary.UpsertParams{
			UserID:      user.ID,
			SummaryDate: summaryDate.Add(18 * time.Hour),

			ExperienceEarned: 20,
			LevelsGained:     1,
			StateBefore: dailysummary.PetState{
				Level:      999,
				Experience: 999,
				Health:     1,
				Hunger:     1,
				Happiness:  1,
				Energy:     1,
			},

			StateAfter:  secondStateAfter,
			GeneratedAt: secondGeneratedAt,
		},
	)
	if err != nil {
		t.Fatalf("second UpsertAction() error = %v", err)
	}

	updated, err := repository.ByUserIDAndDate(
		ctx,
		user.ID,
		summaryDate,
	)
	if err != nil {
		t.Fatalf("read updated summary: %v", err)
	}

	if updated.ID != created.ID {
		t.Errorf(
			"updated ID = %d, want original ID %d",
			updated.ID,
			created.ID,
		)
	}

	if updated.ActionCount != 2 {
		t.Errorf(
			"updated ActionCount = %d, want 2",
			updated.ActionCount,
		)
	}

	if updated.ExperienceEarned != 40 {
		t.Errorf(
			"updated ExperienceEarned = %d, want 40",
			updated.ExperienceEarned,
		)
	}

	if updated.LevelsGained != 1 {
		t.Errorf(
			"updated LevelsGained = %d, want 1",
			updated.LevelsGained,
		)
	}

	if updated.StateBefore != initialState {
		t.Errorf(
			"updated StateBefore = %+v, want original %+v",
			updated.StateBefore,
			initialState,
		)
	}

	if updated.StateAfter != secondStateAfter {
		t.Errorf(
			"updated StateAfter = %+v, want %+v",
			updated.StateAfter,
			secondStateAfter,
		)
	}

	if !updated.GeneratedAt.Equal(secondGeneratedAt) {
		t.Errorf(
			"updated GeneratedAt = %s, want %s",
			updated.GeneratedAt,
			secondGeneratedAt,
		)
	}

	var rowCount int

	err = pool.QueryRow(
		ctx,
		`
			SELECT COUNT(*)
			FROM daily_summaries
			WHERE user_id = $1
			  AND summary_date = $2::date
		`,
		user.ID,
		summaryDate,
	).Scan(&rowCount)
	if err != nil {
		t.Fatalf("count daily summaries: %v", err)
	}

	if rowCount != 1 {
		t.Errorf("row count = %d, want 1", rowCount)
	}
}
