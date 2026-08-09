package dailysummary

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	ByUserIDAndDate(
		ctx context.Context,
		userID uuid.UUID,
		summaryDate time.Time,
	) (DailySummary, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
		now:        time.Now,
	}
}

func (service *Service) Current(
	ctx context.Context,
	userID uuid.UUID,
) (DailySummary, error) {
	now := service.now().UTC()

	today := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		0,
		0,
		0,
		0,
		time.UTC,
	)

	return service.repository.ByUserIDAndDate(
		ctx,
		userID,
		today,
	)
}

func (service *Service) ByDate(
	ctx context.Context,
	userID uuid.UUID,
	summaryDate time.Time,
) (DailySummary, error) {
	return service.repository.ByUserIDAndDate(ctx, userID, summaryDate)
}
