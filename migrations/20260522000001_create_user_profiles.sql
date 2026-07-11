-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS user_profiles (
    user_id     UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    bio         TEXT        NOT NULL DEFAULT '',
    links_json  JSONB       NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fast lookup
CREATE INDEX IF NOT EXISTS idx_user_profiles_user_id ON user_profiles(user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_profiles;
-- +goose StatementEnd
