package activity

import (
	"context"
	"errors"
	"time"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/pet"
	"github.com/google/uuid"
)

var (
	ErrActivityInactive = errors.New("activity is inactive")
	ErrCooldown         = errors.New("activity is on cooldown")
	ErrDailyLimit       = errors.New("daily limit reached")
)

type PerformActionParams struct {
	UserID         uuid.UUID
	ActivityCode   string
	IdempotencyKey uuid.UUID
}

func (service *Service) validateAction(
	ctx context.Context,
	actionRepository ActionRepository,
	params PerformActionParams,
	activityType Type,
	now time.Time,
) error {
	lastAction, err := actionRepository.LastAction(
		ctx,
		params.UserID,
		params.ActivityCode,
	)

	if err == nil {
		cooldown := time.Duration(
			activityType.CooldownSeconds,
		) * time.Second

		if now.Before(lastAction.OccurredAt.Add(cooldown)) {
			return ErrCooldown
		}
	} else if !errors.Is(err, ErrActionNotFound) {
		return err
	}

	if activityType.DailyLimit == nil {
		return nil
	}

	startOfDay := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		0,
		0,
		0,
		0,
		time.UTC,
	)

	count, err := actionRepository.CountActionsSince(
		ctx,
		params.UserID,
		params.ActivityCode,
		startOfDay,
	)
	if err != nil {
		return err
	}

	if count >= *activityType.DailyLimit {
		return ErrDailyLimit
	}

	return nil
}

func (service *Service) PerformAction(
	ctx context.Context,
	params PerformActionParams,
) (Action, error) {
	existingAction, err := service.actionRepository.ByIdempotencyKey(
		ctx,
		params.UserID,
		params.IdempotencyKey,
	)
	if err == nil {
		return existingAction, nil
	}

	if !errors.Is(err, ErrActionNotFound) {
		return Action{}, err
	}

	activityType, err := service.typeRepository.ByCode(
		ctx,
		params.ActivityCode,
	)
	if err != nil {
		return Action{}, err
	}

	if !activityType.IsActive {
		return Action{}, ErrActivityInactive
	}

	now := time.Now().UTC()
	stateDelta := stateDeltaForActivity(params.ActivityCode)

	var result Action

	err = service.transactionManager.WithinTransaction(
		ctx,
		func(
			petRepository PetRepository,
			actionRepository ActionRepository,
		) error {
			if err := petRepository.LockByUserID(
				ctx,
				params.UserID,
			); err != nil {
				return err
			}

			existingAction, err := actionRepository.ByIdempotencyKey(
				ctx,
				params.UserID,
				params.IdempotencyKey,
			)
			if err == nil {
				result = existingAction
				return nil
			}

			if !errors.Is(err, ErrActionNotFound) {
				return err
			}

			if err := service.validateAction(
				ctx,
				actionRepository,
				params,
				activityType,
				now,
			); err != nil {
				return err
			}

			updatedPet, err := petRepository.ApplyAction(
				ctx,
				params.UserID,
				pet.ApplyActionParams{
					Experience: activityType.BaseExperience,
					Health:     stateDelta.Health,
					Hunger:     stateDelta.Hunger,
					Happiness:  stateDelta.Happiness,
					Energy:     stateDelta.Energy,
					OccurredAt: now,
				},
			)
			if err != nil {
				return err
			}

			action := Action{
				UserID:            params.UserID,
				PetID:             updatedPet.ID,
				ActivityCode:      activityType.Code,
				ExperienceAwarded: activityType.BaseExperience,
				StateDelta:        stateDelta,
				IdempotencyKey:    params.IdempotencyKey,
				OccurredAt:        now,
			}

			result, err = actionRepository.Create(ctx, action)

			return err
		},
	)
	if err != nil {
		return Action{}, err
	}

	return result, nil
}

func stateDeltaForActivity(activityCode string) StateDelta {
	switch activityCode {
	case "feed":
		return StateDelta{
			Hunger: 20,
		}

	case "play":
		return StateDelta{
			Happiness: 20,
		}

	case "rest":
		return StateDelta{
			Energy: 20,
		}

	default:
		return StateDelta{}
	}
}
