package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/leaderboard"
)

type LeaderboardRepository struct {
	pool *pgxpool.Pool
}

func NewLeaderboardRepository(pool *pgxpool.Pool) *LeaderboardRepository {
	return &LeaderboardRepository{pool: pool}
}

func (repository *LeaderboardRepository) WeeklyParticipants(
	ctx context.Context,
	startsAt time.Time,
	endsAt time.Time,
) ([]leaderboard.Participant, error) {
	const query = `
		SELECT
			action.user_id,
			account.display_name,
			SUM(action.experience_awarded)::bigint,
			MAX(action.occurred_at)
		FROM pet_actions AS action
		JOIN users AS account ON account.id = action.user_id
		WHERE action.occurred_at >= $1
			AND action.occurred_at < $2
			AND action.experience_awarded > 0
			AND account.status = 'active'
		GROUP BY action.user_id, account.display_name`

	rows, err := repository.pool.Query(ctx, query, startsAt, endsAt)
	if err != nil {
		return nil, fmt.Errorf("query weekly leaderboard: %w", err)
	}
	defer rows.Close()

	participants := make([]leaderboard.Participant, 0)
	for rows.Next() {
		participant := leaderboard.Participant{}
		if err = rows.Scan(
			&participant.UserID,
			&participant.DisplayName,
			&participant.WeeklyExperience,
			&participant.ReachedAt,
		); err != nil {
			return nil, fmt.Errorf("scan weekly leaderboard participant: %w", err)
		}
		participants = append(participants, participant)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate weekly leaderboard: %w", err)
	}
	return participants, nil
}

func (repository *LeaderboardRepository) SaveFinalizedWeek(
	ctx context.Context,
	period leaderboard.Period,
	entries []leaderboard.Entry,
	finalizedAt time.Time,
	selectBefore time.Time,
) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin leaderboard finalization transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	weekID := uuid.Nil
	err = tx.QueryRow(ctx, `
		INSERT INTO leaderboard_weeks (
			starts_at,
			ends_at,
			timezone,
			status,
			participants_count,
			top_5_max_rank,
			top_10_max_rank,
			top_15_max_rank,
			finalized_at
		)
		VALUES ($1, $2, $3, 'final', $4, $5, $6, $7, $8)
		ON CONFLICT (starts_at) DO NOTHING
		RETURNING id`,
		period.StartsAt,
		period.EndsAt,
		period.Timezone,
		len(entries),
		maxRankForTier(entries, 5),
		maxRankForTier(entries, 10),
		maxRankForTier(entries, 15),
		finalizedAt,
	).Scan(&weekID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("insert finalized leaderboard week: %w", err)
	}

	for _, entry := range entries {
		var prizeTier *int
		if entry.PrizeTier > 0 {
			prizeTier = &entry.PrizeTier
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO leaderboard_results (
				week_id, user_id, display_name, weekly_experience, rank, reached_at, prize_tier
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			weekID,
			entry.UserID,
			entry.DisplayName,
			entry.WeeklyExperience,
			entry.Rank,
			entry.ReachedAt,
			prizeTier,
		); err != nil {
			return fmt.Errorf("insert finalized leaderboard result: %w", err)
		}
		if entry.PrizeTier == 0 {
			continue
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO leaderboard_awards (week_id, user_id, tier, select_before)
			VALUES ($1, $2, $3, $4)`, weekID, entry.UserID, entry.PrizeTier, selectBefore); err != nil {
			return fmt.Errorf("insert leaderboard award: %w", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit leaderboard finalization: %w", err)
	}
	return nil
}

func maxRankForTier(entries []leaderboard.Entry, tier int) *int {
	maxRank := 0
	for _, entry := range entries {
		if entry.PrizeTier == tier && entry.Rank > maxRank {
			maxRank = entry.Rank
		}
	}
	if maxRank == 0 {
		return nil
	}
	return &maxRank
}

var _ leaderboard.Repository = (*LeaderboardRepository)(nil)
var _ leaderboard.FinalizationRepository = (*LeaderboardRepository)(nil)
