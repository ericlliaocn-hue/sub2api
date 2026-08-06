-- OIOIO 创作中心独立业务 Schema。
--
-- 这组表只保存创作中心的供应商、模型路由和任务投影；用户身份、API Key、
-- 余额与扣费仍由 Sub2API 现有表和服务负责。迁移只新增对象，不修改或删除
-- 任何现有运营数据。

CREATE SCHEMA IF NOT EXISTS creation;

CREATE TABLE IF NOT EXISTS creation.provider (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(96) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL,
    protocol VARCHAR(64) NOT NULL DEFAULT 'openai_compatible',
    base_url TEXT,
    secret_ref VARCHAR(255),
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS creation.provider_group (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES creation.provider(id),
    code VARCHAR(96) NOT NULL,
    name VARCHAR(160) NOT NULL,
    platform VARCHAR(64) NOT NULL DEFAULT 'openai',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    priority INTEGER NOT NULL DEFAULT 100,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider_id, code)
);

CREATE TABLE IF NOT EXISTS creation.provider_account (
    id BIGSERIAL PRIMARY KEY,
    provider_group_id BIGINT NOT NULL REFERENCES creation.provider_group(id),
    name VARCHAR(160) NOT NULL,
    secret_ref VARCHAR(255),
    credentials_ciphertext TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    rate_limit JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_error_code VARCHAR(96),
    last_error_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS creation.model (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(160) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS creation.model_route (
    id BIGSERIAL PRIMARY KEY,
    model_id BIGINT NOT NULL REFERENCES creation.model(id),
    provider_group_id BIGINT NOT NULL REFERENCES creation.provider_group(id),
    upstream_model VARCHAR(160) NOT NULL,
    endpoint VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    priority INTEGER NOT NULL DEFAULT 100,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (model_id, provider_group_id, upstream_model, endpoint)
);

CREATE TABLE IF NOT EXISTS creation.model_pricing (
    id BIGSERIAL PRIMARY KEY,
    model_route_id BIGINT NOT NULL REFERENCES creation.model_route(id),
    unit VARCHAR(32) NOT NULL DEFAULT 'request',
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    amount NUMERIC(20, 8) NOT NULL DEFAULT 0,
    rules JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS creation.generation_task (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT,
    model_id BIGINT REFERENCES creation.model(id),
    model_route_id BIGINT REFERENCES creation.model_route(id),
    kind VARCHAR(32) NOT NULL,
    status VARCHAR(40) NOT NULL DEFAULT 'queued',
    prompt TEXT NOT NULL DEFAULT '',
    request JSONB NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key VARCHAR(255),
    provider_task_id VARCHAR(255),
    estimated_cost NUMERIC(20, 8) NOT NULL DEFAULT 0,
    actual_cost NUMERIC(20, 8),
    hold_reference VARCHAR(255),
    error_code VARCHAR(96),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    settled_at TIMESTAMPTZ,
    UNIQUE (user_id, api_key_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS creation.generation_attempt (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL REFERENCES creation.generation_task(id) ON DELETE CASCADE,
    attempt_no INTEGER NOT NULL,
    provider_account_id BIGINT REFERENCES creation.provider_account(id),
    status VARCHAR(40) NOT NULL DEFAULT 'leased',
    external_task_id VARCHAR(255),
    request JSONB NOT NULL DEFAULT '{}'::jsonb,
    response JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_code VARCHAR(96),
    error_message TEXT,
    lease_token VARCHAR(128),
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (task_id, attempt_no)
);

CREATE TABLE IF NOT EXISTS creation.media_asset (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL REFERENCES creation.generation_task(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    kind VARCHAR(32) NOT NULL,
    mime_type VARCHAR(128),
    object_key TEXT,
    media_url TEXT,
    poster_url TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS creation.task_event (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL REFERENCES creation.generation_task(id) ON DELETE CASCADE,
    event_type VARCHAR(64) NOT NULL,
    status VARCHAR(40),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS creation_provider_group_provider_idx
    ON creation.provider_group (provider_id, status, priority);
CREATE INDEX IF NOT EXISTS creation_provider_account_group_idx
    ON creation.provider_account (provider_group_id, status);
CREATE INDEX IF NOT EXISTS creation_model_route_model_idx
    ON creation.model_route (model_id, status, priority);
CREATE INDEX IF NOT EXISTS creation_model_pricing_route_idx
    ON creation.model_pricing (model_route_id, status, effective_from DESC);
CREATE INDEX IF NOT EXISTS creation_generation_task_user_idx
    ON creation.generation_task (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS creation_generation_task_status_idx
    ON creation.generation_task (status, created_at);
CREATE INDEX IF NOT EXISTS creation_generation_task_provider_idx
    ON creation.generation_task (provider_task_id);
CREATE INDEX IF NOT EXISTS creation_generation_attempt_lease_idx
    ON creation.generation_attempt (status, lease_expires_at);
CREATE INDEX IF NOT EXISTS creation_media_asset_user_idx
    ON creation.media_asset (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS creation_task_event_task_idx
    ON creation.task_event (task_id, created_at);
