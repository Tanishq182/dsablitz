CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email CITEXT NOT NULL UNIQUE,
    password_hash TEXT,
    handle CITEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    avatar_url TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT users_status_check CHECK (status IN ('active', 'disabled', 'deleted')),
    CONSTRAINT users_handle_length_check CHECK (char_length(handle::TEXT) BETWEEN 3 AND 32),
    CONSTRAINT users_display_name_length_check CHECK (char_length(display_name) BETWEEN 1 AND 80)
);

CREATE TABLE oauth_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_account_id TEXT NOT NULL,
    provider_email CITEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT oauth_accounts_provider_check CHECK (provider IN ('google', 'github')),
    CONSTRAINT oauth_accounts_provider_account_unique UNIQUE (provider, provider_account_id),
    CONSTRAINT oauth_accounts_user_provider_unique UNIQUE (user_id, provider)
);

CREATE TABLE friendships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    addressee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    responded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT friendships_distinct_users_check CHECK (requester_id <> addressee_id),
    CONSTRAINT friendships_status_check CHECK (status IN ('pending', 'accepted', 'declined', 'blocked'))
);

CREATE TABLE rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    host_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'waiting',
    max_players SMALLINT NOT NULL DEFAULT 2,
    duration_seconds INTEGER NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT rooms_status_check CHECK (status IN ('waiting', 'ready', 'in_battle', 'closed', 'expired')),
    CONSTRAINT rooms_max_players_check CHECK (max_players = 2),
    CONSTRAINT rooms_duration_seconds_check CHECK (duration_seconds IN (120, 300)),
    CONSTRAINT rooms_code_length_check CHECK (char_length(code) BETWEEN 4 AND 16)
);

CREATE TABLE room_players (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seat_number SMALLINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'joined',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT room_players_status_check CHECK (status IN ('joined', 'ready', 'left', 'kicked')),
    CONSTRAINT room_players_seat_number_check CHECK (seat_number BETWEEN 1 AND 2),
    CONSTRAINT room_players_room_user_unique UNIQUE (room_id, user_id),
    CONSTRAINT room_players_room_seat_unique UNIQUE (room_id, seat_number)
);

CREATE TABLE battles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID REFERENCES rooms(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'created',
    duration_seconds INTEGER NOT NULL,
    question_count INTEGER NOT NULL DEFAULT 0,
    winner_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT battles_status_check CHECK (status IN ('created', 'countdown', 'active', 'finished', 'aborted')),
    CONSTRAINT battles_duration_seconds_check CHECK (duration_seconds IN (120, 300)),
    CONSTRAINT battles_question_count_check CHECK (question_count >= 0),
    CONSTRAINT battles_time_order_check CHECK (ended_at IS NULL OR started_at IS NULL OR ended_at >= started_at)
);

CREATE TABLE battle_players (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    battle_id UUID NOT NULL REFERENCES battles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seat_number SMALLINT NOT NULL,
    rating_before INTEGER NOT NULL,
    rating_after INTEGER,
    score INTEGER NOT NULL DEFAULT 0,
    correct_count INTEGER NOT NULL DEFAULT 0,
    incorrect_count INTEGER NOT NULL DEFAULT 0,
    max_streak INTEGER NOT NULL DEFAULT 0,
    result TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT battle_players_seat_number_check CHECK (seat_number BETWEEN 1 AND 2),
    CONSTRAINT battle_players_rating_before_check CHECK (rating_before >= 0),
    CONSTRAINT battle_players_rating_after_check CHECK (rating_after IS NULL OR rating_after >= 0),
    CONSTRAINT battle_players_score_check CHECK (score >= 0),
    CONSTRAINT battle_players_counts_check CHECK (correct_count >= 0 AND incorrect_count >= 0 AND max_streak >= 0),
    CONSTRAINT battle_players_result_check CHECK (result IS NULL OR result IN ('win', 'loss', 'draw', 'abandoned')),
    CONSTRAINT battle_players_battle_user_unique UNIQUE (battle_id, user_id),
    CONSTRAINT battle_players_battle_seat_unique UNIQUE (battle_id, seat_number)
);

CREATE TABLE questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_type TEXT NOT NULL,
    difficulty SMALLINT NOT NULL,
    title TEXT NOT NULL,
    prompt TEXT NOT NULL,
    options JSONB,
    correct_answer TEXT NOT NULL,
    explanation TEXT,
    time_limit_sec INTEGER NOT NULL,
    tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    source TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT questions_type_check CHECK (question_type IN ('mcq', 'complexity_prediction', 'pattern_recognition', 'numeric_answer', 'algorithm_ordering')),
    CONSTRAINT questions_difficulty_check CHECK (difficulty BETWEEN 1 AND 5),
    CONSTRAINT questions_time_limit_sec_check CHECK (time_limit_sec > 0),
    CONSTRAINT questions_options_shape_check CHECK (options IS NULL OR jsonb_typeof(options) = 'array'),
    CONSTRAINT questions_title_length_check CHECK (char_length(title) BETWEEN 1 AND 200)
);

CREATE TABLE submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    battle_id UUID NOT NULL REFERENCES battles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE RESTRICT,
    raw_answer JSONB NOT NULL,
    normalized_answer TEXT,
    is_correct BOOLEAN NOT NULL,
    response_time_ms INTEGER NOT NULL,
    score_awarded INTEGER NOT NULL DEFAULT 0,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT submissions_raw_answer_shape_check CHECK (jsonb_typeof(raw_answer) IN ('string', 'number', 'boolean', 'array', 'object')),
    CONSTRAINT submissions_response_time_check CHECK (response_time_ms >= 0),
    CONSTRAINT submissions_score_awarded_check CHECK (score_awarded >= 0)
);

CREATE TABLE user_stats (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    rating INTEGER NOT NULL DEFAULT 1000,
    battles_played INTEGER NOT NULL DEFAULT 0,
    battles_won INTEGER NOT NULL DEFAULT 0,
    battles_lost INTEGER NOT NULL DEFAULT 0,
    battles_drawn INTEGER NOT NULL DEFAULT 0,
    questions_attempted INTEGER NOT NULL DEFAULT 0,
    questions_correct INTEGER NOT NULL DEFAULT 0,
    current_streak INTEGER NOT NULL DEFAULT 0,
    best_streak INTEGER NOT NULL DEFAULT 0,
    total_score BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT user_stats_rating_check CHECK (rating >= 0),
    CONSTRAINT user_stats_counts_check CHECK (
        battles_played >= 0
        AND battles_won >= 0
        AND battles_lost >= 0
        AND battles_drawn >= 0
        AND questions_attempted >= 0
        AND questions_correct >= 0
        AND current_streak >= 0
        AND best_streak >= 0
        AND total_score >= 0
    ),
    CONSTRAINT user_stats_battle_total_check CHECK (battles_played >= battles_won + battles_lost + battles_drawn),
    CONSTRAINT user_stats_question_total_check CHECK (questions_attempted >= questions_correct)
);

CREATE TABLE question_stats (
    question_id UUID PRIMARY KEY REFERENCES questions(id) ON DELETE CASCADE,
    times_served INTEGER NOT NULL DEFAULT 0,
    times_answered INTEGER NOT NULL DEFAULT 0,
    times_correct INTEGER NOT NULL DEFAULT 0,
    average_response_time_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT question_stats_counts_check CHECK (
        times_served >= 0
        AND times_answered >= 0
        AND times_correct >= 0
        AND (average_response_time_ms IS NULL OR average_response_time_ms >= 0)
    ),
    CONSTRAINT question_stats_answered_total_check CHECK (times_served >= times_answered),
    CONSTRAINT question_stats_correct_total_check CHECK (times_answered >= times_correct)
);

CREATE TABLE rating_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    battle_id UUID REFERENCES battles(id) ON DELETE SET NULL,
    rating_before INTEGER NOT NULL,
    rating_after INTEGER NOT NULL,
    delta INTEGER NOT NULL,
    reason TEXT NOT NULL DEFAULT 'battle_result',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT rating_history_rating_check CHECK (rating_before >= 0 AND rating_after >= 0),
    CONSTRAINT rating_history_delta_check CHECK (delta = rating_after - rating_before),
    CONSTRAINT rating_history_reason_check CHECK (reason IN ('battle_result', 'admin_adjustment', 'season_reset'))
);

CREATE UNIQUE INDEX idx_friendships_unique_pair
    ON friendships (LEAST(requester_id, addressee_id), GREATEST(requester_id, addressee_id));

CREATE INDEX idx_users_status_created_at ON users(status, created_at);

CREATE INDEX idx_oauth_accounts_user_id ON oauth_accounts(user_id);

CREATE INDEX idx_friendships_requester_status ON friendships(requester_id, status);
CREATE INDEX idx_friendships_addressee_status ON friendships(addressee_id, status);

CREATE INDEX idx_rooms_host_status ON rooms(host_user_id, status);
CREATE INDEX idx_rooms_status_created_at ON rooms(status, created_at);
CREATE INDEX idx_rooms_expires_at ON rooms(expires_at) WHERE expires_at IS NOT NULL;

CREATE INDEX idx_room_players_room_id ON room_players(room_id);
CREATE INDEX idx_room_players_user_id ON room_players(user_id);
CREATE INDEX idx_room_players_status ON room_players(status);

CREATE INDEX idx_battles_room_id ON battles(room_id) WHERE room_id IS NOT NULL;
CREATE INDEX idx_battles_status_created_at ON battles(status, created_at);
CREATE INDEX idx_battles_winner_user_id ON battles(winner_user_id) WHERE winner_user_id IS NOT NULL;

CREATE INDEX idx_battle_players_battle_id ON battle_players(battle_id);
CREATE INDEX idx_battle_players_user_id ON battle_players(user_id);
CREATE INDEX idx_battle_players_result ON battle_players(result) WHERE result IS NOT NULL;

CREATE INDEX idx_questions_active_difficulty_type ON questions(is_active, difficulty, question_type);
CREATE INDEX idx_questions_created_by ON questions(created_by) WHERE created_by IS NOT NULL;
CREATE INDEX idx_questions_tags_gin ON questions USING GIN(tags);

CREATE INDEX idx_submissions_battle_user_submitted_at ON submissions(battle_id, user_id, submitted_at);
CREATE INDEX idx_submissions_battle_question ON submissions(battle_id, question_id);
CREATE INDEX idx_submissions_user_submitted_at ON submissions(user_id, submitted_at);
CREATE INDEX idx_submissions_question_id ON submissions(question_id);

CREATE INDEX idx_user_stats_rating ON user_stats(rating DESC, user_id);

CREATE INDEX idx_question_stats_times_answered ON question_stats(times_answered DESC, question_id);

CREATE INDEX idx_rating_history_user_created_at ON rating_history(user_id, created_at DESC);
CREATE INDEX idx_rating_history_battle_id ON rating_history(battle_id) WHERE battle_id IS NOT NULL;

CREATE TRIGGER set_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER set_oauth_accounts_updated_at
BEFORE UPDATE ON oauth_accounts
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER set_friendships_updated_at
BEFORE UPDATE ON friendships
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER set_rooms_updated_at
BEFORE UPDATE ON rooms
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER set_room_players_updated_at
BEFORE UPDATE ON room_players
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER set_battles_updated_at
BEFORE UPDATE ON battles
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER set_battle_players_updated_at
BEFORE UPDATE ON battle_players
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER set_questions_updated_at
BEFORE UPDATE ON questions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER set_user_stats_updated_at
BEFORE UPDATE ON user_stats
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER set_question_stats_updated_at
BEFORE UPDATE ON question_stats
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
