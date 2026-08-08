package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
	"github.com/google/uuid"
)

type fakeTransactionManager struct {
	petRepository    PetRepository
	actionRepository ActionRepository
	calls            int
	err              error
}

func (manager *fakeTransactionManager) WithinTransaction(
	ctx context.Context,
	fn func(PetRepository, ActionRepository) error,
) error {
	manager.calls++

	if manager.err != nil {
		return manager.err
	}

	return fn(manager.petRepository, manager.actionRepository)
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
		nil, nil,
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
		},
	}

	transactionManager := &fakeTransactionManager{
		petRepository:    petRepository,
		actionRepository: actionRepository,
	}

	service := NewService(
		nil,
		typeRepository,
		actionRepository,
		petRepository, transactionManager,
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

	if got.UserID != userID {
		t.Errorf("UserID = %v, want %v", got.UserID, userID)
	}

	if got.PetID != petID {
		t.Errorf("PetID = %v, want %v", got.PetID, petID)
	}

	if got.ActivityCode != "feed" {
		t.Errorf("ActivityCode = %q, want %q", got.ActivityCode, "feed")
	}

	if got.ExperienceAwarded != 10 {
		t.Errorf(
			"ExperienceAwarded = %d, want 10",
			got.ExperienceAwarded,
		)
	}

	if got.StateDelta.Hunger != 20 {
		t.Errorf(
			"StateDelta.Hunger = %d, want 20",
			got.StateDelta.Hunger,
		)
	}

	if petRepository.applyCalls != 1 {
		t.Errorf(
			"ApplyAction() called %d times, want 1",
			petRepository.applyCalls,
		)
	}

	if transactionManager.calls != 1 {
		t.Errorf(
			"WithinTransaction() called %d times, want 1",
			transactionManager.calls,
		)
	}
}

func TestPerformActionIdempotency(t *testing.T) {
	userID := uuid.New()
	idempotencyKey := uuid.New()

	existingAction := Action{
		ID:                42,
		UserID:            userID,
		ActivityCode:      "feed",
		ExperienceAwarded: 10,
		IdempotencyKey:    idempotencyKey,
	}

	actionRepository := &fakeActionRepository{
		existingAction: existingAction,
	}

	petRepository := &fakePetRepository{}

	transactionManager := &fakeTransactionManager{
		petRepository:    petRepository,
		actionRepository: actionRepository,
	}

	service := NewService(
		nil,
		nil,
		actionRepository,
		petRepository, transactionManager,
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

	if got.ID != existingAction.ID {
		t.Errorf("ID = %d, want %d", got.ID, existingAction.ID)
	}

	if petRepository.applyCalls != 0 {
		t.Errorf(
			"ApplyAction() called %d times, want 0",
			petRepository.applyCalls,
		)
	}

	if transactionManager.calls != 0 {
		t.Errorf(
			"WithinTransaction() called %d times, want 0",
			transactionManager.calls,
		)
	}
}
