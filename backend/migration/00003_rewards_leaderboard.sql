-- +goose Up
ALTER TABLE reward_definitions
DROP CONSTRAINT reward_definitions_trigger_type_check;

ALTER TABLE reward_definitions
ADD CONSTRAINT reward_definitions_trigger_type_check
CHECK (trigger_type IN ('level', 'streak', 'achievement', 'leaderboard'));

ALTER TABLE user_rewards
ADD COLUMN issuance_key TEXT;

UPDATE user_rewards
SET issuance_key = 'legacy:' || id::text
WHERE issuance_key IS NULL;

ALTER TABLE user_rewards
ALTER COLUMN issuance_key SET NOT NULL;

ALTER TABLE user_rewards
DROP CONSTRAINT user_rewards_user_id_reward_id_key;

ALTER TABLE user_rewards
ADD CONSTRAINT user_rewards_business_event_key
UNIQUE (user_id, reward_id, issuance_key);

ALTER TABLE reward_redemptions
ALTER COLUMN external_reference DROP NOT NULL;

ALTER TABLE reward_redemptions
ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE,
ADD COLUMN idempotency_key UUID,
ADD COLUMN request_hash BYTEA,
ADD COLUMN redemption_context JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE reward_redemptions AS redemption
SET
    user_id = reward.user_id,
    idempotency_key = redemption.id,
    request_hash = digest(redemption.external_reference, 'sha256')
FROM user_rewards AS reward
WHERE reward.id = redemption.user_reward_id;

ALTER TABLE reward_redemptions
ALTER COLUMN user_id SET NOT NULL,
ALTER COLUMN idempotency_key SET NOT NULL,
ALTER COLUMN request_hash SET NOT NULL;

ALTER TABLE reward_redemptions
ADD CONSTRAINT reward_redemptions_user_idempotency_key
UNIQUE (user_id, idempotency_key);

CREATE TABLE reward_categories (
    code TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE leaderboard_weeks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    starts_at TIMESTAMPTZ NOT NULL UNIQUE,
    ends_at TIMESTAMPTZ NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'Europe/Moscow' CHECK (timezone = 'Europe/Moscow'),
    status TEXT NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'final')),
    participants_count INTEGER NOT NULL DEFAULT 0 CHECK (participants_count >= 0),
    top_5_max_rank INTEGER CHECK (top_5_max_rank > 0),
    top_10_max_rank INTEGER CHECK (top_10_max_rank > 0),
    top_15_max_rank INTEGER CHECK (top_15_max_rank > 0),
    finalized_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at > starts_at),
    CHECK (
        (status = 'in_progress' AND finalized_at IS NULL)
        OR (status = 'final' AND finalized_at IS NOT NULL)
    )
);

CREATE TABLE leaderboard_results (
    week_id UUID NOT NULL REFERENCES leaderboard_weeks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,
    weekly_experience BIGINT NOT NULL CHECK (weekly_experience > 0),
    rank INTEGER NOT NULL CHECK (rank > 0),
    reached_at TIMESTAMPTZ NOT NULL,
    prize_tier INTEGER CHECK (prize_tier IN (5, 10, 15)),
    PRIMARY KEY (week_id, user_id),
    UNIQUE (week_id, user_id, prize_tier)
);

CREATE INDEX leaderboard_results_order_idx
ON leaderboard_results (week_id, weekly_experience DESC, reached_at, user_id);

CREATE INDEX pet_actions_weekly_leaderboard_idx
ON pet_actions (occurred_at, user_id)
INCLUDE (experience_awarded);

CREATE TABLE leaderboard_reward_options (
    tier INTEGER NOT NULL CHECK (tier IN (5, 10, 15)),
    option_code TEXT NOT NULL,
    reward_id UUID NOT NULL REFERENCES reward_definitions(id),
    sort_order SMALLINT NOT NULL CHECK (sort_order BETWEEN 1 AND 3),
    PRIMARY KEY (tier, option_code),
    UNIQUE (tier, sort_order),
    UNIQUE (tier, reward_id)
);

CREATE TABLE leaderboard_awards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    week_id UUID NOT NULL REFERENCES leaderboard_weeks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tier INTEGER NOT NULL CHECK (tier IN (5, 10, 15)),
    status TEXT NOT NULL DEFAULT 'selection_pending'
        CHECK (status IN ('selection_pending', 'selected', 'expired')),
    select_before TIMESTAMPTZ NOT NULL,
    selected_option_code TEXT,
    user_reward_id UUID UNIQUE REFERENCES user_rewards(id),
    idempotency_key UUID,
    request_hash BYTEA,
    selected_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (week_id, user_id),
    FOREIGN KEY (week_id, user_id, tier)
        REFERENCES leaderboard_results(week_id, user_id, prize_tier),
    FOREIGN KEY (tier, selected_option_code)
        REFERENCES leaderboard_reward_options(tier, option_code),
    CHECK (select_before > created_at),
    CHECK (
        (status = 'selection_pending'
            AND selected_option_code IS NULL
            AND user_reward_id IS NULL
            AND selected_at IS NULL)
        OR (status = 'selected'
            AND selected_option_code IS NOT NULL
            AND user_reward_id IS NOT NULL
            AND idempotency_key IS NOT NULL
            AND request_hash IS NOT NULL
            AND selected_at IS NOT NULL)
        OR (status = 'expired'
            AND selected_option_code IS NULL
            AND user_reward_id IS NULL
            AND selected_at IS NULL)
    )
);

CREATE UNIQUE INDEX leaderboard_awards_user_idempotency_idx
ON leaderboard_awards (user_id, idempotency_key)
WHERE idempotency_key IS NOT NULL;

CREATE INDEX leaderboard_awards_inventory_idx
ON leaderboard_awards (user_id, status, select_before);

-- +goose Down
DROP TABLE IF EXISTS leaderboard_awards;
DROP TABLE IF EXISTS leaderboard_reward_options;
DROP INDEX IF EXISTS pet_actions_weekly_leaderboard_idx;
DROP TABLE IF EXISTS leaderboard_results;
DROP TABLE IF EXISTS leaderboard_weeks;
DROP TABLE IF EXISTS reward_categories;

UPDATE reward_redemptions
SET external_reference = COALESCE(external_reference, 'rollback:' || id::text);

ALTER TABLE reward_redemptions
DROP CONSTRAINT reward_redemptions_user_idempotency_key,
DROP COLUMN redemption_context,
DROP COLUMN request_hash,
DROP COLUMN idempotency_key,
DROP COLUMN user_id;

ALTER TABLE reward_redemptions
ALTER COLUMN external_reference SET NOT NULL;

ALTER TABLE user_rewards
DROP CONSTRAINT user_rewards_business_event_key;

ALTER TABLE user_rewards
ADD CONSTRAINT user_rewards_user_id_reward_id_key
UNIQUE (user_id, reward_id);

ALTER TABLE user_rewards
DROP COLUMN issuance_key;

ALTER TABLE reward_definitions
DROP CONSTRAINT reward_definitions_trigger_type_check;

ALTER TABLE reward_definitions
ADD CONSTRAINT reward_definitions_trigger_type_check
CHECK (trigger_type IN ('level', 'streak', 'achievement'));