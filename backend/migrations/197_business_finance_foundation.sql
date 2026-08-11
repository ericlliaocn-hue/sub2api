-- Business finance foundation.
--
-- These tables are intentionally separate from the gateway, billing and
-- usage-log tables. They provide an auditable management layer for operating
-- costs without changing the existing request charging path.

CREATE TABLE IF NOT EXISTS business_cost_configs (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    category VARCHAR(40) NOT NULL,
    amount NUMERIC(20, 8) NOT NULL CHECK (amount >= 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'CNY',
    allocation_method VARCHAR(32) NOT NULL DEFAULT 'revenue_share',
    scope JSONB NOT NULL DEFAULT '{}'::jsonb,
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMPTZ,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    notes TEXT NOT NULL DEFAULT '',
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT business_cost_configs_effective_range_check
        CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE INDEX IF NOT EXISTS business_cost_configs_category_idx
    ON business_cost_configs (category, enabled, effective_from DESC);

CREATE TABLE IF NOT EXISTS business_expenses (
    id BIGSERIAL PRIMARY KEY,
    category VARCHAR(40) NOT NULL,
    name VARCHAR(128) NOT NULL,
    amount NUMERIC(20, 8) NOT NULL CHECK (amount >= 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'CNY',
    occurred_at TIMESTAMPTZ NOT NULL,
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    allocation_method VARCHAR(32) NOT NULL DEFAULT 'revenue_share',
    scope JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    notes TEXT NOT NULL DEFAULT '',
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT business_expenses_period_range_check
        CHECK (period_end IS NULL OR period_start IS NULL OR period_end > period_start),
    CONSTRAINT business_expenses_status_check
        CHECK (status IN ('active', 'void'))
);

CREATE INDEX IF NOT EXISTS business_expenses_occurred_at_idx
    ON business_expenses (occurred_at DESC);

CREATE INDEX IF NOT EXISTS business_expenses_category_status_idx
    ON business_expenses (category, status, occurred_at DESC);
