-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE levels (
    level INTEGER PRIMARY KEY CHECK (level > 0),
    required_total_experience BIGINT NOT NULL CHECK (required_total_experience >= 0),
    title TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    display_name TEXT NOT NULL CHECK (char_length(display_name) BETWEEN 2 AND 40),
    password_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked', 'deleted')),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX users_email_lower_uidx ON users (lower(email));

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE pets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 30),
    species TEXT NOT NULL DEFAULT 'avito_pet',
    level INTEGER NOT NULL DEFAULT 1 REFERENCES levels(level),
    experience BIGINT NOT NULL DEFAULT 0 CHECK (experience >= 0),
    health SMALLINT NOT NULL DEFAULT 100 CHECK (health BETWEEN 0 AND 100),
    hunger SMALLINT NOT NULL DEFAULT 100 CHECK (hunger BETWEEN 0 AND 100),
    happiness SMALLINT NOT NULL DEFAULT 100 CHECK (happiness BETWEEN 0 AND 100),
    energy SMALLINT NOT NULL DEFAULT 100 CHECK (energy BETWEEN 0 AND 100),
    state_version BIGINT NOT NULL DEFAULT 1 CHECK (state_version > 0),
    last_interaction_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX pets_leaderboard_idx ON pets (experience DESC, updated_at ASC);

CREATE TRIGGER pets_set_updated_at
BEFORE UPDATE ON pets
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE activity_types (
    code TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('care', 'avito_product')),
    base_experience INTEGER NOT NULL CHECK (base_experience > 0),
    daily_limit INTEGER CHECK (daily_limit IS NULL OR daily_limit > 0),
    cooldown_seconds INTEGER NOT NULL DEFAULT 0 CHECK (cooldown_seconds >= 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER activity_types_set_updated_at
BEFORE UPDATE ON activity_types
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE pet_actions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pet_id UUID NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    activity_code TEXT NOT NULL REFERENCES activity_types(code),
    experience_awarded INTEGER NOT NULL CHECK (experience_awarded >= 0),
    state_delta JSONB NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key UUID NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX pet_actions_pet_occurred_idx ON pet_actions (pet_id, occurred_at DESC);
CREATE INDEX pet_actions_user_day_idx ON pet_actions (user_id, occurred_at DESC);

CREATE TABLE user_streaks (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_days INTEGER NOT NULL DEFAULT 0 CHECK (current_days >= 0),
    longest_days INTEGER NOT NULL DEFAULT 0 CHECK (longest_days >= current_days),
    last_active_date DATE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE daily_summaries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    summary_date DATE NOT NULL,
    action_count INTEGER NOT NULL DEFAULT 0 CHECK (action_count >= 0),
    experience_earned INTEGER NOT NULL DEFAULT 0 CHECK (experience_earned >= 0),
    levels_gained INTEGER NOT NULL DEFAULT 0 CHECK (levels_gained >= 0),
    state_before JSONB NOT NULL DEFAULT '{}'::jsonb,
    state_after JSONB NOT NULL DEFAULT '{}'::jsonb,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, summary_date)
);

CREATE TABLE reward_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    reward_type TEXT NOT NULL CHECK (reward_type IN ('free_delivery', 'discount', 'autoteka', 'promotion', 'custom')),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    trigger_type TEXT NOT NULL CHECK (trigger_type IN ('level', 'streak', 'achievement')),
    trigger_value INTEGER NOT NULL CHECK (trigger_value > 0),
    active_from TIMESTAMPTZ,
    active_until TIMESTAMPTZ,
    total_issue_limit INTEGER CHECK (total_issue_limit IS NULL OR total_issue_limit > 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (active_until IS NULL OR active_from IS NULL OR active_until > active_from)
);

CREATE TRIGGER reward_definitions_set_updated_at
BEFORE UPDATE ON reward_definitions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_rewards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reward_id UUID NOT NULL REFERENCES reward_definitions(id),
    status TEXT NOT NULL DEFAULT 'issued' CHECK (status IN ('issued', 'redeemed', 'expired', 'revoked')),
    progress INTEGER NOT NULL DEFAULT 0 CHECK (progress >= 0),
    target INTEGER NOT NULL CHECK (target > 0),
    claim_token_hash BYTEA UNIQUE,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    redeemed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    UNIQUE (user_id, reward_id),
    CHECK (redeemed_at IS NULL OR status = 'redeemed'),
    CHECK (expires_at IS NULL OR expires_at > issued_at)
);

CREATE INDEX user_rewards_status_idx ON user_rewards (user_id, status, issued_at DESC);

CREATE TABLE reward_redemptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_reward_id UUID NOT NULL UNIQUE REFERENCES user_rewards(id),
    external_reference TEXT NOT NULL UNIQUE,
    redeemed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE refresh_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (expires_at > created_at)
);

CREATE INDEX refresh_sessions_user_active_idx
ON refresh_sessions (user_id, expires_at DESC)
WHERE revoked_at IS NULL;

CREATE VIEW leaderboard AS
SELECT
    p.user_id,
    u.display_name,
    p.level,
    p.experience,
    COALESCE(s.current_days, 0) AS current_streak,
    DENSE_RANK() OVER (ORDER BY p.experience DESC, COALESCE(s.current_days, 0) DESC) AS rank
FROM pets AS p
JOIN users AS u ON u.id = p.user_id
LEFT JOIN user_streaks AS s ON s.user_id = p.user_id
WHERE u.status = 'active';

-- +goose Down
DROP VIEW IF EXISTS leaderboard;
DROP TABLE IF EXISTS refresh_sessions;
DROP TABLE IF EXISTS reward_redemptions;
DROP TABLE IF EXISTS user_rewards;
DROP TABLE IF EXISTS reward_definitions;
DROP TABLE IF EXISTS daily_summaries;
DROP TABLE IF EXISTS user_streaks;
DROP TABLE IF EXISTS pet_actions;
DROP TABLE IF EXISTS activity_types;
DROP TABLE IF EXISTS pets;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS levels;
DROP FUNCTION IF EXISTS set_updated_at();
DROP EXTENSION IF EXISTS pgcrypto;
