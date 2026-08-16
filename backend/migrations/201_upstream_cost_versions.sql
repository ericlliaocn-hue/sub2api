-- Versioned manual upstream cost profiles.
--
-- The active profile is also copied to accounts.extra for request-path reads.
-- This table is append-only so historical usage rows can retain the exact
-- version that was active when a request started.

CREATE TABLE IF NOT EXISTS upstream_cost_versions (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    model VARCHAR(100) NOT NULL,
    short_input_price NUMERIC(20, 10) NOT NULL CHECK (short_input_price >= 0),
    short_cache_read_price NUMERIC(20, 10) NOT NULL CHECK (short_cache_read_price >= 0),
    short_cache_write_price NUMERIC(20, 10) NOT NULL CHECK (short_cache_write_price >= 0),
    short_output_price NUMERIC(20, 10) NOT NULL CHECK (short_output_price >= 0),
    long_context_threshold INTEGER NOT NULL DEFAULT 0 CHECK (long_context_threshold >= 0),
    long_input_price NUMERIC(20, 10) NOT NULL CHECK (long_input_price >= 0),
    long_cache_read_price NUMERIC(20, 10) NOT NULL CHECK (long_cache_read_price >= 0),
    long_cache_write_price NUMERIC(20, 10) NOT NULL CHECK (long_cache_write_price >= 0),
    long_output_price NUMERIC(20, 10) NOT NULL CHECK (long_output_price >= 0),
    declared_multiplier NUMERIC(20, 10) NOT NULL CHECK (declared_multiplier >= 0),
    balance_unit_cost NUMERIC(20, 10) NOT NULL DEFAULT 1 CHECK (balance_unit_cost > 0),
    notes TEXT NOT NULL DEFAULT '',
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS upstream_cost_versions_account_model_effective_idx
    ON upstream_cost_versions (account_id, model, effective_from DESC, id DESC);

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS upstream_cost NUMERIC(20, 10),
    ADD COLUMN IF NOT EXISTS upstream_cost_snapshot JSONB;

ALTER TABLE usage_logs
    DROP CONSTRAINT IF EXISTS usage_logs_upstream_cost_non_negative;

ALTER TABLE usage_logs
    ADD CONSTRAINT usage_logs_upstream_cost_non_negative
    CHECK (upstream_cost IS NULL OR upstream_cost >= 0);
