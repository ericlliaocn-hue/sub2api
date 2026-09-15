-- User-level sub-pool placement. Keys inherit this; new keys stop needing a
-- per-key click. A group default covers everyone without a personal row.
-- Graduation ignores keys whose user has a default, so an admin pin sticks.

CREATE TABLE IF NOT EXISTS user_sub_pool_defaults (
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    sub_pool_id BIGINT NOT NULL REFERENCES sub_pools(id) ON DELETE CASCADE,
    operator    VARCHAR(64) NOT NULL DEFAULT 'system',
    note        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_user_sub_pool_defaults_group
    ON user_sub_pool_defaults(group_id, sub_pool_id);

COMMENT ON TABLE user_sub_pool_defaults IS
    'Admin-owned default sub-pool for a user inside one group; all of that user''s keys inherit it';

CREATE TABLE IF NOT EXISTS group_sub_pool_defaults (
    group_id    BIGINT PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
    sub_pool_id BIGINT NOT NULL REFERENCES sub_pools(id) ON DELETE CASCADE,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE group_sub_pool_defaults IS
    'Default sub-pool for new keys in a group when the user has no personal default';
