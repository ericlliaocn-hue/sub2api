-- Per-user traffic capture for ops investigation.
--
-- This is NOT content moderation and NOT prompt audit. Those paths call
-- external APIs and were too expensive to leave on for the whole site.
-- This table only stores the prompt and upstream response for a short
-- explicit user list, and the feature has a kill switch in settings
-- (watched_traffic_config.enabled). When that flag is off, the gateway
-- takes the fast path and does not wrap the request or write here.

CREATE TABLE IF NOT EXISTS watched_traffic_logs (
    id             BIGSERIAL PRIMARY KEY,
    request_id     VARCHAR(64) NOT NULL DEFAULT '',
    user_id        BIGINT NOT NULL,
    user_email     VARCHAR(255) NOT NULL DEFAULT '',
    api_key_id     BIGINT,
    api_key_name   VARCHAR(128) NOT NULL DEFAULT '',
    group_id       BIGINT,
    group_name     VARCHAR(128) NOT NULL DEFAULT '',
    account_id     BIGINT,
    model          VARCHAR(256) NOT NULL DEFAULT '',
    endpoint       VARCHAR(256) NOT NULL DEFAULT '',
    status_code    INT NOT NULL DEFAULT 0,
    prompt_text    TEXT,
    response_text  TEXT,
    error_text     TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_watched_traffic_user_time
    ON watched_traffic_logs(user_id, created_at DESC, id DESC);

COMMENT ON TABLE watched_traffic_logs IS
    'Local prompt/response capture for explicitly watched users; independent of content moderation';
