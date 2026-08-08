package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/rewards"
)

type RewardRepository struct {
	pool *pgxpool.Pool
}

type rowScanner interface {
	Scan(dest ...any) error
}

func NewRewardRepository(pool *pgxpool.Pool) *RewardRepository {
	return &RewardRepository{pool: pool}
}

func (repository *RewardRepository) ListInventory(
	ctx context.Context,
	userID uuid.UUID,
	status string,
	now time.Time,
) (rewards.Inventory, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return rewards.Inventory{}, fmt.Errorf("begin inventory transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `
		UPDATE user_rewards
		SET status = 'expired'
		WHERE user_id = $1 AND status = 'issued' AND expires_at <= $2`, userID, now); err != nil {
		return rewards.Inventory{}, fmt.Errorf("expire rewards: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE leaderboard_awards
		SET status = 'expired'
		WHERE user_id = $1 AND status = 'selection_pending' AND select_before <= $2`, userID, now); err != nil {
		return rewards.Inventory{}, fmt.Errorf("expire leaderboard awards: %w", err)
	}

	rewardRows, err := tx.Query(ctx, `
		SELECT
			user_reward.id,
			definition.code,
			definition.title,
			definition.description,
			definition.trigger_type,
			user_reward.status,
			definition.payload,
			user_reward.issued_at,
			user_reward.expires_at,
			user_reward.redeemed_at
		FROM user_rewards AS user_reward
		JOIN reward_definitions AS definition ON definition.id = user_reward.reward_id
		WHERE user_reward.user_id = $1 AND ($2 = '' OR user_reward.status = $2)
		ORDER BY user_reward.issued_at DESC, user_reward.id`, userID, status)
	if err != nil {
		return rewards.Inventory{}, fmt.Errorf("query reward inventory: %w", err)
	}

	inventory := rewards.Inventory{Rewards: make([]rewards.Reward, 0), LeaderboardAwards: make([]rewards.LeaderboardAward, 0)}
	for rewardRows.Next() {
		reward, scanErr := scanReward(rewardRows)
		if scanErr != nil {
			rewardRows.Close()
			return rewards.Inventory{}, scanErr
		}
		inventory.Rewards = append(inventory.Rewards, reward)
	}
	if err = rewardRows.Err(); err != nil {
		rewardRows.Close()
		return rewards.Inventory{}, fmt.Errorf("iterate reward inventory: %w", err)
	}
	rewardRows.Close()

	awardRows, err := tx.Query(ctx, `
		SELECT
			award.id,
			week.starts_at,
			award.tier,
			award.status,
			award.select_before,
			definition.code,
			definition.title,
			definition.description,
			definition.payload
		FROM leaderboard_awards AS award
		JOIN leaderboard_weeks AS week ON week.id = award.week_id
		JOIN leaderboard_reward_options AS option ON option.tier = award.tier
		JOIN reward_definitions AS definition ON definition.id = option.reward_id
		WHERE award.user_id = $1 AND award.status = 'selection_pending'
		ORDER BY award.select_before, award.id, option.sort_order`, userID)
	if err != nil {
		return rewards.Inventory{}, fmt.Errorf("query leaderboard award inventory: %w", err)
	}

	awardIndexes := make(map[uuid.UUID]int)
	for awardRows.Next() {
		awardID := uuid.Nil
		award := rewards.LeaderboardAward{}
		option := rewards.RewardOption{}
		payload := []byte(nil)
		if err = awardRows.Scan(
			&awardID,
			&award.WeekStartsAt,
			&award.Tier,
			&award.Status,
			&award.SelectBefore,
			&option.Code,
			&option.Title,
			&option.Description,
			&payload,
		); err != nil {
			awardRows.Close()
			return rewards.Inventory{}, fmt.Errorf("scan leaderboard award inventory: %w", err)
		}
		if err = json.Unmarshal(payload, &option.Benefit); err != nil {
			awardRows.Close()
			return rewards.Inventory{}, fmt.Errorf("decode leaderboard reward benefit: %w", err)
		}
		index, exists := awardIndexes[awardID]
		if !exists {
			award.ID = awardID
			award.Options = make([]rewards.RewardOption, 0, 3)
			inventory.LeaderboardAwards = append(inventory.LeaderboardAwards, award)
			index = len(inventory.LeaderboardAwards) - 1
			awardIndexes[awardID] = index
		}
		inventory.LeaderboardAwards[index].Options = append(inventory.LeaderboardAwards[index].Options, option)
	}
	if err = awardRows.Err(); err != nil {
		awardRows.Close()
		return rewards.Inventory{}, fmt.Errorf("iterate leaderboard award inventory: %w", err)
	}
	awardRows.Close()

	if err = tx.Commit(ctx); err != nil {
		return rewards.Inventory{}, fmt.Errorf("commit inventory transaction: %w", err)
	}
	return inventory, nil
}

func (repository *RewardRepository) Redeem(
	ctx context.Context,
	command rewards.RedeemCommand,
) (rewards.Reward, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return rewards.Reward{}, fmt.Errorf("begin redemption transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if reward, found, loadErr := loadRedemption(ctx, tx, command); loadErr != nil {
		return rewards.Reward{}, loadErr
	} else if found {
		return commitReward(ctx, tx, reward)
	}

	reward, err := loadRewardForUpdate(ctx, tx, command.UserID, command.RewardID)
	if err != nil {
		return rewards.Reward{}, err
	}
	if existing, found, loadErr := loadRedemption(ctx, tx, command); loadErr != nil {
		return rewards.Reward{}, loadErr
	} else if found {
		return commitReward(ctx, tx, existing)
	}
	if reward.Status != rewards.StatusIssued || !command.RedeemedAt.Before(reward.ExpiresAt) {
		return rewards.Reward{}, rewards.ErrNotAvailable
	}
	if err = reward.ValidateRedemption(command.CategoryCode, command.ListingID); err != nil {
		return rewards.Reward{}, err
	}
	if command.CategoryCode != "" {
		categoryAllowed := false
		err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM reward_categories
				WHERE code = $1 AND is_active
			)`, command.CategoryCode).Scan(&categoryAllowed)
		if err != nil {
			return rewards.Reward{}, fmt.Errorf("validate reward category: %w", err)
		}
		if !categoryAllowed {
			return rewards.Reward{}, rewards.ValidationError{
				Field: "category_code", Message: "category is not available",
			}
		}
	}

	contextPayload, err := json.Marshal(map[string]string{
		"category_code": command.CategoryCode,
		"listing_id":    command.ListingID,
	})
	if err != nil {
		return rewards.Reward{}, fmt.Errorf("encode redemption context: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE user_rewards
		SET status = 'redeemed', redeemed_at = $2
		WHERE id = $1`, command.RewardID, command.RedeemedAt); err != nil {
		return rewards.Reward{}, fmt.Errorf("mark reward redeemed: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO reward_redemptions (
			user_reward_id, user_id, idempotency_key, request_hash, redemption_context
		) VALUES ($1, $2, $3, $4, $5)`,
		command.RewardID,
		command.UserID,
		command.IdempotencyKey,
		command.RequestHash,
		contextPayload,
	); err != nil {
		if isUniqueViolation(err, "") {
			return rewards.Reward{}, rewards.ErrIdempotencyConflict
		}
		return rewards.Reward{}, fmt.Errorf("insert reward redemption: %w", err)
	}
	reward.Status = rewards.StatusRedeemed
	reward.RedeemedAt = &command.RedeemedAt
	return commitReward(ctx, tx, reward)
}

func (repository *RewardRepository) SelectAward(
	ctx context.Context,
	command rewards.SelectAwardCommand,
) (rewards.Reward, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return rewards.Reward{}, fmt.Errorf("begin award selection transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	weekID := uuid.Nil
	tier := 0
	status := ""
	selectBefore := command.SelectedAt
	selectedRewardID := (*uuid.UUID)(nil)
	storedKey := (*uuid.UUID)(nil)
	storedHash := []byte(nil)
	err = tx.QueryRow(ctx, `
		SELECT week_id, tier, status, select_before, user_reward_id, idempotency_key, request_hash
		FROM leaderboard_awards
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`, command.AwardID, command.UserID).Scan(
		&weekID,
		&tier,
		&status,
		&selectBefore,
		&selectedRewardID,
		&storedKey,
		&storedHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rewards.Reward{}, rewards.ErrNotFound
		}
		return rewards.Reward{}, fmt.Errorf("lock leaderboard award: %w", err)
	}
	if status == "selected" {
		if storedKey == nil || *storedKey != command.IdempotencyKey || !bytes.Equal(storedHash, command.RequestHash) {
			return rewards.Reward{}, rewards.ErrIdempotencyConflict
		}
		if selectedRewardID == nil {
			return rewards.Reward{}, errors.New("selected leaderboard award has no reward")
		}
		reward, loadErr := loadReward(ctx, tx, command.UserID, *selectedRewardID)
		if loadErr != nil {
			return rewards.Reward{}, loadErr
		}
		return commitReward(ctx, tx, reward)
	}
	if status != "selection_pending" {
		return rewards.Reward{}, rewards.ErrNotAvailable
	}
	if !command.SelectedAt.Before(selectBefore) {
		return rewards.Reward{}, rewards.ErrSelectionExpired
	}

	definitionID := uuid.Nil
	err = tx.QueryRow(ctx, `
		SELECT reward_id
		FROM leaderboard_reward_options
		WHERE tier = $1 AND option_code = $2`, tier, command.OptionCode).Scan(&definitionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rewards.Reward{}, rewards.ValidationError{
				Field: "option_code", Message: "option is not available for this award",
			}
		}
		return rewards.Reward{}, fmt.Errorf("select leaderboard reward option: %w", err)
	}

	userRewardID := uuid.Nil
	err = tx.QueryRow(ctx, `
		INSERT INTO user_rewards (
			user_id, reward_id, target, issuance_key, issued_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		command.UserID,
		definitionID,
		tier,
		"leaderboard:"+weekID.String(),
		command.SelectedAt,
		command.ExpiresAt,
	).Scan(&userRewardID)
	if err != nil {
		return rewards.Reward{}, fmt.Errorf("issue selected leaderboard reward: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE leaderboard_awards
		SET
			status = 'selected',
			selected_option_code = $2,
			user_reward_id = $3,
			idempotency_key = $4,
			request_hash = $5,
			selected_at = $6
		WHERE id = $1`,
		command.AwardID,
		command.OptionCode,
		userRewardID,
		command.IdempotencyKey,
		command.RequestHash,
		command.SelectedAt,
	); err != nil {
		if isUniqueViolation(err, "") {
			return rewards.Reward{}, rewards.ErrIdempotencyConflict
		}
		return rewards.Reward{}, fmt.Errorf("complete leaderboard award selection: %w", err)
	}
	reward, err := loadReward(ctx, tx, command.UserID, userRewardID)
	if err != nil {
		return rewards.Reward{}, err
	}
	return commitReward(ctx, tx, reward)
}

func (repository *RewardRepository) Issue(
	ctx context.Context,
	command rewards.IssueCommand,
) ([]rewards.Reward, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin reward issuance transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `
		INSERT INTO user_rewards (
			user_id, reward_id, target, issuance_key, issued_at, expires_at
		)
		SELECT $1, definition.id, definition.trigger_value, $4, $5, $6
		FROM reward_definitions AS definition
		WHERE definition.trigger_type = $2
			AND definition.trigger_value = $3
			AND definition.is_active
			AND (definition.active_from IS NULL OR definition.active_from <= $5)
			AND (definition.active_until IS NULL OR definition.active_until > $5)
		ON CONFLICT (user_id, reward_id, issuance_key) DO NOTHING`,
		command.UserID,
		command.TriggerType,
		command.TriggerValue,
		command.IssuanceKey,
		command.OccurredAt,
		command.ExpiresAt,
	); err != nil {
		return nil, fmt.Errorf("insert issued rewards: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT
			user_reward.id,
			definition.code,
			definition.title,
			definition.description,
			definition.trigger_type,
			user_reward.status,
			definition.payload,
			user_reward.issued_at,
			user_reward.expires_at,
			user_reward.redeemed_at
		FROM user_rewards AS user_reward
		JOIN reward_definitions AS definition ON definition.id = user_reward.reward_id
		WHERE user_reward.user_id = $1
			AND user_reward.issuance_key = $2
			AND definition.trigger_type = $3
			AND definition.trigger_value = $4
		ORDER BY definition.code`,
		command.UserID,
		command.IssuanceKey,
		command.TriggerType,
		command.TriggerValue,
	)
	if err != nil {
		return nil, fmt.Errorf("query issued rewards: %w", err)
	}
	issued := make([]rewards.Reward, 0)
	for rows.Next() {
		reward, scanErr := scanReward(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		issued = append(issued, reward)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate issued rewards: %w", err)
	}
	rows.Close()
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit reward issuance: %w", err)
	}
	return issued, nil
}

func loadRedemption(
	ctx context.Context,
	tx pgx.Tx,
	command rewards.RedeemCommand,
) (rewards.Reward, bool, error) {
	requestHash := []byte(nil)
	row := tx.QueryRow(ctx, `
		SELECT
			redemption.request_hash,
			user_reward.id,
			definition.code,
			definition.title,
			definition.description,
			definition.trigger_type,
			user_reward.status,
			definition.payload,
			user_reward.issued_at,
			user_reward.expires_at,
			user_reward.redeemed_at
		FROM reward_redemptions AS redemption
		JOIN user_rewards AS user_reward ON user_reward.id = redemption.user_reward_id
		JOIN reward_definitions AS definition ON definition.id = user_reward.reward_id
		WHERE redemption.user_id = $1 AND redemption.idempotency_key = $2`,
		command.UserID,
		command.IdempotencyKey,
	)
	reward, err := scanRewardWithPrefix(row, &requestHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rewards.Reward{}, false, nil
		}
		return rewards.Reward{}, false, err
	}
	if reward.ID != command.RewardID || !bytes.Equal(requestHash, command.RequestHash) {
		return rewards.Reward{}, false, rewards.ErrIdempotencyConflict
	}
	return reward, true, nil
}

func loadRewardForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	rewardID uuid.UUID,
) (rewards.Reward, error) {
	return scanRewardRow(tx.QueryRow(ctx, `
		SELECT
			user_reward.id,
			definition.code,
			definition.title,
			definition.description,
			definition.trigger_type,
			user_reward.status,
			definition.payload,
			user_reward.issued_at,
			user_reward.expires_at,
			user_reward.redeemed_at
		FROM user_rewards AS user_reward
		JOIN reward_definitions AS definition ON definition.id = user_reward.reward_id
		WHERE user_reward.id = $1 AND user_reward.user_id = $2
		FOR UPDATE OF user_reward`, rewardID, userID))
}

func loadReward(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	rewardID uuid.UUID,
) (rewards.Reward, error) {
	return scanRewardRow(tx.QueryRow(ctx, `
		SELECT
			user_reward.id,
			definition.code,
			definition.title,
			definition.description,
			definition.trigger_type,
			user_reward.status,
			definition.payload,
			user_reward.issued_at,
			user_reward.expires_at,
			user_reward.redeemed_at
		FROM user_rewards AS user_reward
		JOIN reward_definitions AS definition ON definition.id = user_reward.reward_id
		WHERE user_reward.id = $1 AND user_reward.user_id = $2`, rewardID, userID))
}

func scanRewardRow(row rowScanner) (rewards.Reward, error) {
	reward, err := scanReward(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return rewards.Reward{}, rewards.ErrNotFound
	}
	return reward, err
}

func scanReward(scanner rowScanner) (rewards.Reward, error) {
	return scanRewardWithPrefix(scanner)
}

func scanRewardWithPrefix(scanner rowScanner, prefix ...any) (rewards.Reward, error) {
	reward := rewards.Reward{}
	payload := []byte(nil)
	destinations := make([]any, 0, len(prefix)+10)
	destinations = append(destinations, prefix...)
	destinations = append(destinations,
		&reward.ID,
		&reward.Code,
		&reward.Title,
		&reward.Description,
		&reward.Source,
		&reward.Status,
		&payload,
		&reward.IssuedAt,
		&reward.ExpiresAt,
		&reward.RedeemedAt,
	)
	if err := scanner.Scan(destinations...); err != nil {
		return rewards.Reward{}, err
	}
	if err := json.Unmarshal(payload, &reward.Benefit); err != nil {
		return rewards.Reward{}, fmt.Errorf("decode reward benefit: %w", err)
	}
	return reward, nil
}

func commitReward(ctx context.Context, tx pgx.Tx, reward rewards.Reward) (rewards.Reward, error) {
	if err := tx.Commit(ctx); err != nil {
		return rewards.Reward{}, fmt.Errorf("commit reward transaction: %w", err)
	}
	return reward, nil
}

var _ rewards.Repository = (*RewardRepository)(nil)
