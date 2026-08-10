package activity

import (
	"context"
	"errors"
	"time"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/dailysummary"
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
) (PerformActionResult, error) {
	existingAction, err := service.actionRepository.ByIdempotencyKey(
		ctx,
		params.UserID,
		params.IdempotencyKey,
	)
	if err == nil {
		if existingAction.ActivityCode != params.ActivityCode {
			return PerformActionResult{}, ErrIdempotencyConflict
		}

		return service.loadActionResult(
			ctx,
			existingAction,
			service.petRepository,
			service.progressRepository,
		)
	}

	if !errors.Is(err, ErrActionNotFound) {
		return PerformActionResult{}, err
	}

	activityType, err := service.typeRepository.ByCode(
		ctx,
		params.ActivityCode,
	)
	if err != nil {
		return PerformActionResult{}, err
	}

	if !activityType.IsActive {
		return PerformActionResult{}, ErrActivityInactive
	}

	now := time.Now().UTC()
	stateDelta := stateDeltaForActivity(params.ActivityCode)

	var result PerformActionResult

	err = service.transactionManager.WithinTransaction(
		ctx,
		func(
			petRepository PetRepository,
			actionRepository ActionRepository,
			progressRepository ProgressRepository,
			dailySummaryRepository DailySummaryRepository,
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
				if existingAction.ActivityCode != params.ActivityCode {
					return ErrIdempotencyConflict
				}

				result, err = service.loadActionResult(
					ctx,
					existingAction,
					petRepository,
					progressRepository,
				)

				return err
			}

			if !errors.Is(err, ErrActionNotFound) {
				return err
			}

			petBefore, err := petRepository.ByUserID(
				ctx,
				params.UserID,
			)
			if err != nil {
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

			createdAction, err := actionRepository.Create(ctx, action)
			if err != nil {
				return err
			}

			level, err := progressRepository.LevelByNumber(
				ctx,
				updatedPet.Level,
			)
			if err != nil {
				return err
			}

			userStreak, err := progressRepository.AdvanceStreak(
				ctx,
				params.UserID,
				now,
			)
			if err != nil {
				return err
			}

			levelsGained := updatedPet.Level - petBefore.Level

			if err := dailySummaryRepository.UpsertAction(
				ctx,
				dailysummary.UpsertParams{
					UserID:           params.UserID,
					SummaryDate:      now,
					ExperienceEarned: createdAction.ExperienceAwarded,
					LevelsGained:     levelsGained,
					StateBefore:      dailySummaryPetState(petBefore),
					StateAfter:       dailySummaryPetState(updatedPet),
					GeneratedAt:      now,
				},
			); err != nil {
				return err
			}

			result = PerformActionResult{
				Action:     createdAction,
				Pet:        updatedPet,
				Level:      level,
				UserStreak: userStreak,
			}

			return nil
		},
	)

	if err != nil {
		return PerformActionResult{}, err
	}

	return result, nil
}
func (service *Service) loadActionResult(
	ctx context.Context,
	action Action,
	petRepository PetRepository,
	progressRepository ProgressRepository,
) (PerformActionResult, error) {
	currentPet, err := petRepository.ByUserID(
		ctx,
		action.UserID,
	)
	if err != nil {
		return PerformActionResult{}, err
	}

	level, err := progressRepository.LevelByNumber(
		ctx,
		currentPet.Level,
	)
	if err != nil {
		return PerformActionResult{}, err
	}

	userStreak, err := progressRepository.StreakByUserID(
		ctx,
		action.UserID,
	)
	if err != nil {
		return PerformActionResult{}, err
	}

	return PerformActionResult{
		Action:     action,
		Pet:        currentPet,
		Level:      level,
		UserStreak: userStreak,
	}, nil
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

func dailySummaryPetState(value pet.Pet) dailysummary.PetState {
	return dailysummary.PetState{
		Level:      value.Level,
		Experience: value.Experience,
		Health:     value.Health,
		Hunger:     value.Hunger,
		Happiness:  value.Happiness,
		Energy:     value.Energy,
	}
}
