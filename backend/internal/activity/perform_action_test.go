package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/dailysummary"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/progress"
)

type fakeDailySummaryRepository struct {
	calls  int
	params dailysummary.UpsertParams
	err    error
}

func (repository *fakeDailySummaryRepository) UpsertAction(
	_ context.Context,
	params dailysummary.UpsertParams,
) error {
	repository.calls++
	repository.params = params

	return repository.err
}

type fakeProgressRepository struct {
	level     progress.Level
	levelErr  error
	streak    progress.UserStreak
	streakErr error

	advanceCalls int
}

func (repository *fakeProgressRepository) LevelByNumber(
	ctx context.Context,
	levelNumber int,
) (progress.Level, error) {
	return repository.level, repository.levelErr
}

func (repository *fakeProgressRepository) StreakByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (progress.UserStreak, error) {
	return repository.streak, repository.streakErr
}

func (repository *fakeProgressRepository) AdvanceStreak(
	ctx context.Context,
	userID uuid.UUID,
	activeAt time.Time,
) (progress.UserStreak, error) {
	repository.advanceCalls++

	return repository.streak, repository.streakErr
}

type fakeTransactionManager struct {
	petRepository          PetRepository
	actionRepository       ActionRepository
	progressRepository     ProgressRepository
	dailySummaryRepository DailySummaryRepository
	calls                  int
	err                    error
}

func (manager *fakeTransactionManager) WithinTransaction(
	_ context.Context,
	fn func(
		PetRepository,
		ActionRepository,
		ProgressRepository,
		DailySummaryRepository,
	) error,
) error {
	manager.calls++

	if manager.err != nil {
		return manager.err
	}

	return fn(
		manager.petRepository,
		manager.actionRepository,
		manager.progressRepository,
		manager.dailySummaryRepository,
	)
}

type fakePetRepository struct {
	pet pet.Pet
	err error

	applyCalls int
	lockCalls  int
	lockErr    error
}

func (repository *fakePetRepository) ByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (pet.Pet, error) {
	return repository.pet, repository.err
}

func (repository *fakePetRepository) ApplyAction(
	ctx context.Context,
	userID uuid.UUID,
	params pet.ApplyActionParams,
) (pet.Pet, error) {
	repository.applyCalls++

	return repository.pet, repository.err
}

func (repository *fakePetRepository) LockByUserID(
	ctx context.Context,
	userID uuid.UUID,
) error {
	repository.lockCalls++

	return repository.lockErr
}

type fakeTypeRepository struct {
	activityType Type
	err          error
}

func (repository *fakeTypeRepository) ByCode(
	ctx context.Context,
	code string,
) (Type, error) {
	return repository.activityType, repository.err
}

type fakeActionRepository struct {
	lastAction Action
	lastErr    error

	count    int
	countErr error

	existingAction Action
	existingErr    error
}

func (repository *fakeActionRepository) Create(
	ctx context.Context,
	action Action,
) (Action, error) {
	return action, nil
}

func (repository *fakeActionRepository) LastAction(
	ctx context.Context,
	userID uuid.UUID,
	activityCode string,
) (Action, error) {
	return repository.lastAction, repository.lastErr
}

func (repository *fakeActionRepository) CountActionsSince(
	ctx context.Context,
	userID uuid.UUID,
	activityCode string,
	since time.Time,
) (int, error) {
	return repository.count, repository.countErr
}

func (repository *fakeActionRepository) ByIdempotencyKey(
	ctx context.Context,
	userID uuid.UUID,
	key uuid.UUID,
) (Action, error) {
	if repository.existingErr != nil {
		return Action{}, repository.existingErr
	}

	if repository.existingAction.ID != 0 {
		return repository.existingAction, nil
	}

	return Action{}, ErrActionNotFound
}

func TestValidateActionSuccess(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	dailyLimit := 3

	typeRepository := &fakeTypeRepository{
		activityType: Type{
			Code:            "feed",
			IsActive:        true,
			CooldownSeconds: 3600,
			DailyLimit:      &dailyLimit,
		},
	}

	actionRepository := &fakeActionRepository{
		lastErr: ErrActionNotFound,
		count:   0,
	}

	service := NewService(
		nil,
		typeRepository,
		actionRepository,
		nil, nil, nil,
	)

	err := service.validateAction(
		context.Background(),
		actionRepository,
		PerformActionParams{
			UserID:       uuid.New(),
			ActivityCode: "feed",
		},
		typeRepository.activityType,
		now,
	)
	if err != nil {
		t.Fatalf("validateAction() error = %v", err)
	}

	if err != nil {
		t.Fatalf("validateAction() error = %v", err)
	}

}

func TestValidateActionCooldown(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

	actionRepository := &fakeActionRepository{
		lastAction: Action{
			OccurredAt: now.Add(-30 * time.Minute),
		},
	}

	service := &Service{}

	err := service.validateAction(
		context.Background(),
		actionRepository,
		PerformActionParams{
			UserID:       uuid.New(),
			ActivityCode: "feed",
		},
		Type{
			Code:            "feed",
			IsActive:        true,
			CooldownSeconds: 3600,
		},
		now,
	)

	if !errors.Is(err, ErrCooldown) {
		t.Errorf(
			"validateAction() error = %v, want %v",
			err,
			ErrCooldown,
		)
	}
}

func TestValidateActionCooldownExpired(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

	actionRepository := &fakeActionRepository{
		lastAction: Action{
			OccurredAt: now.Add(-2 * time.Hour),
		},
	}

	service := &Service{}

	err := service.validateAction(
		context.Background(),
		actionRepository,
		PerformActionParams{
			UserID:       uuid.New(),
			ActivityCode: "feed",
		},
		Type{
			Code:            "feed",
			IsActive:        true,
			CooldownSeconds: 3600,
		},
		now,
	)

	if err != nil {
		t.Fatalf("validateAction() error = %v, want nil", err)
	}
}

func TestValidateActionDailyLimit(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	dailyLimit := 3

	actionRepository := &fakeActionRepository{
		lastErr: ErrActionNotFound,
		count:   3,
	}

	service := &Service{}

	err := service.validateAction(
		context.Background(),
		actionRepository,
		PerformActionParams{
			UserID:       uuid.New(),
			ActivityCode: "feed",
		},
		Type{
			Code:       "feed",
			IsActive:   true,
			DailyLimit: &dailyLimit,
		},
		now,
	)

	if !errors.Is(err, ErrDailyLimit) {
		t.Errorf(
			"validateAction() error = %v, want %v",
			err,
			ErrDailyLimit,
		)
	}
}

func TestValidateActionNoPreviousActions(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

	actionRepository := &fakeActionRepository{
		lastErr: ErrActionNotFound,
	}

	service := &Service{}

	err := service.validateAction(
		context.Background(),
		actionRepository,
		PerformActionParams{
			UserID:       uuid.New(),
			ActivityCode: "play",
		},
		Type{
			Code:     "play",
			IsActive: true,
		},
		now,
	)

	if err != nil {
		t.Fatalf("validateAction() error = %v, want nil", err)
	}
}

func TestPerformActionSuccess(t *testing.T) {
	userID := uuid.New()
	petID := uuid.New()
	idempotencyKey := uuid.New()

	typeRepository := &fakeTypeRepository{
		activityType: Type{
			Code:           "feed",
			BaseExperience: 10,
			IsActive:       true,
		},
	}

	actionRepository := &fakeActionRepository{
		lastErr: ErrActionNotFound,
	}

	petRepository := &fakePetRepository{
		pet: pet.Pet{
			ID:     petID,
			UserID: userID,
			Level:  1,
		},
	}

	progressRepository := &fakeProgressRepository{
		level: progress.Level{
			Level:                   1,
			RequiredTotalExperience: 0,
			Title:                   "Новичок",
		},
		streak: progress.UserStreak{
			CurrentDays: 1,
			LongestDays: 1,
		},
	}
	dailySummaryRepository := &fakeDailySummaryRepository{}

	transactionManager := &fakeTransactionManager{
		petRepository:          petRepository,
		actionRepository:       actionRepository,
		progressRepository:     progressRepository,
		dailySummaryRepository: dailySummaryRepository,
	}

	service := NewService(
		nil,
		typeRepository,
		actionRepository,
		petRepository,
		progressRepository,
		transactionManager,
	)

	got, err := service.PerformAction(
		context.Background(),
		PerformActionParams{
			UserID:         userID,
			ActivityCode:   "feed",
			IdempotencyKey: idempotencyKey,
		},
	)
	if err != nil {
		t.Fatalf("PerformAction() error = %v", err)
	}

	if got.Action.UserID != userID {
		t.Errorf(
			"Action.UserID = %v, want %v",
			got.Action.UserID,
			userID,
		)
	}

	if got.Action.PetID != petID {
		t.Errorf(
			"Action.PetID = %v, want %v",
			got.Action.PetID,
			petID,
		)
	}

	if got.Action.ActivityCode != "feed" {
		t.Errorf(
			"Action.ActivityCode = %q, want %q",
			got.Action.ActivityCode,
			"feed",
		)
	}

	if got.Action.ExperienceAwarded != 10 {
		t.Errorf(
			"Action.ExperienceAwarded = %d, want 10",
			got.Action.ExperienceAwarded,
		)
	}

	if got.Action.StateDelta.Hunger != 20 {
		t.Errorf(
			"Action.StateDelta.Hunger = %d, want 20",
			got.Action.StateDelta.Hunger,
		)
	}

	if got.Pet.ID != petID {
		t.Errorf(
			"Pet.ID = %v, want %v",
			got.Pet.ID,
			petID,
		)
	}

	if got.Level.Level != 1 {
		t.Errorf(
			"Level.Level = %d, want 1",
			got.Level.Level,
		)
	}

	if got.UserStreak.CurrentDays != 1 {
		t.Errorf(
			"UserStreak.CurrentDays = %d, want 1",
			got.UserStreak.CurrentDays,
		)
	}

	if petRepository.applyCalls != 1 {
		t.Errorf(
			"ApplyAction() called %d times, want 1",
			petRepository.applyCalls,
		)
	}

	if progressRepository.advanceCalls != 1 {
		t.Errorf(
			"AdvanceStreak() called %d times, want 1",
			progressRepository.advanceCalls,
		)
	}

	if transactionManager.calls != 1 {
		t.Errorf(
			"WithinTransaction() called %d times, want 1",
			transactionManager.calls,
		)
	}

	if dailySummaryRepository.calls != 1 {
		t.Errorf(
			"UpsertAction() called %d times, want 1",
			dailySummaryRepository.calls,
		)
	}

	if dailySummaryRepository.params.UserID != userID {
		t.Errorf(
			"UpsertAction UserID = %v, want %v",
			dailySummaryRepository.params.UserID,
			userID,
		)
	}

	if dailySummaryRepository.params.ExperienceEarned != 10 {
		t.Errorf(
			"UpsertAction ExperienceEarned = %d, want 10",
			dailySummaryRepository.params.ExperienceEarned,
		)
	}
}

func TestPerformActionIdempotency(t *testing.T) {
	userID := uuid.New()
	petID := uuid.New()
	idempotencyKey := uuid.New()

	existingAction := Action{
		ID:                42,
		UserID:            userID,
		PetID:             petID,
		ActivityCode:      "feed",
		ExperienceAwarded: 10,
		IdempotencyKey:    idempotencyKey,
	}

	actionRepository := &fakeActionRepository{
		existingAction: existingAction,
	}

	petRepository := &fakePetRepository{
		pet: pet.Pet{
			ID:     petID,
			UserID: userID,
			Level:  1,
		},
	}

	progressRepository := &fakeProgressRepository{
		level: progress.Level{
			Level: 1,
			Title: "Новичок",
		},
		streak: progress.UserStreak{
			CurrentDays: 1,
			LongestDays: 1,
		},
	}

	transactionManager := &fakeTransactionManager{
		petRepository:      petRepository,
		actionRepository:   actionRepository,
		progressRepository: progressRepository,
	}

	service := NewService(
		nil,
		nil,
		actionRepository,
		petRepository,
		progressRepository,
		transactionManager,
	)

	got, err := service.PerformAction(
		context.Background(),
		PerformActionParams{
			UserID:         userID,
			ActivityCode:   "feed",
			IdempotencyKey: idempotencyKey,
		},
	)
	if err != nil {
		t.Fatalf("PerformAction() error = %v", err)
	}

	if got.Action.ID != existingAction.ID {
		t.Errorf(
			"Action.ID = %d, want %d",
			got.Action.ID,
			existingAction.ID,
		)
	}

	if got.Pet.ID != petID {
		t.Errorf(
			"Pet.ID = %v, want %v",
			got.Pet.ID,
			petID,
		)
	}

	if got.Level.Level != 1 {
		t.Errorf(
			"Level.Level = %d, want 1",
			got.Level.Level,
		)
	}

	if got.UserStreak.CurrentDays != 1 {
		t.Errorf(
			"UserStreak.CurrentDays = %d, want 1",
			got.UserStreak.CurrentDays,
		)
	}

	if petRepository.applyCalls != 0 {
		t.Errorf(
			"ApplyAction() called %d times, want 0",
			petRepository.applyCalls,
		)
	}

	if progressRepository.advanceCalls != 0 {
		t.Errorf(
			"AdvanceStreak() called %d times, want 0",
			progressRepository.advanceCalls,
		)
	}

	if transactionManager.calls != 0 {
		t.Errorf(
			"WithinTransaction() called %d times, want 0",
			transactionManager.calls,
		)
	}
}
