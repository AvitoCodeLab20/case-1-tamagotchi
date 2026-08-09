package progress

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	ByUserID(ctx context.Context, userID uuid.UUID) (Progress, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (service *Service) Get(
	ctx context.Context,
	userID uuid.UUID,
) (Progress, error) {
	return service.repository.ByUserID(ctx, userID)
}
