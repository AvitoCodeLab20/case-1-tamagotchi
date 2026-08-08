package activity

import (
	"context"
	"time"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
	"github.com/google/uuid"
)

type Repository interface {
	ListActive(ctx context.Context) ([]Type, error)
}

type TypeRepository interface {
	ByCode(ctx context.Context, code string) (Type, error)
}

type PetRepository interface {
	ByUserID(
		ctx context.Context,
		userID uuid.UUID,
	) (pet.Pet, error)

	ApplyAction(
		ctx context.Context,
		userID uuid.UUID,
		params pet.ApplyActionParams,
	) (pet.Pet, error)

	LockByUserID(
		ctx context.Context,
		userID uuid.UUID,
	) error
}

type ActionRepository interface {
	Create(ctx context.Context, action Action) (Action, error)

	LastAction(
		ctx context.Context,
		userID uuid.UUID,
		activityCode string,
	) (Action, error)

	CountActionsSince(
		ctx context.Context,
		userID uuid.UUID,
		activityCode string,
		since time.Time,
	) (int, error)

	ByIdempotencyKey(
		ctx context.Context,
		userID uuid.UUID,
		key uuid.UUID,
	) (Action, error)
}

type Service struct {
	repository         Repository
	typeRepository     TypeRepository
	actionRepository   ActionRepository
	petRepository      PetRepository
	transactionManager TransactionManager
}

type TransactionManager interface {
	WithinTransaction(
		ctx context.Context,
		fn func(PetRepository, ActionRepository) error,
	) error
}

func NewService(
	repository Repository,
	typeRepository TypeRepository,
	actionRepository ActionRepository,
	petRepository PetRepository,
	transactionManager TransactionManager,
) *Service {
	return &Service{
		repository:         repository,
		typeRepository:     typeRepository,
		actionRepository:   actionRepository,
		petRepository:      petRepository,
		transactionManager: transactionManager,
	}
}

func (service *Service) ListTypes(ctx context.Context) ([]Type, error) {
	return service.repository.ListActive(ctx)
}
