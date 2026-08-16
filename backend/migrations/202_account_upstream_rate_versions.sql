-- Versioned upstream billing rate snapshots.
--
-- This migration only creates the persistence shape. Account creation,
-- manual changes, and probe takeover transactions are implemented later.

CREATE TABLE IF NOT EXISTS account_upstream_rate_versions (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL
        REFERENCES accounts(id) ON DELETE RESTRICT,
    version_no BIGINT NOT NULL
        CONSTRAINT account_upstream_rate_versions_version_no_positive CHECK (version_no > 0),
    rate_multiplier NUMERIC(20, 10) NOT NULL
        CONSTRAINT account_upstream_rate_versions_rate_multiplier_non_negative CHECK (rate_multiplier >= 0),
    source VARCHAR(32) NOT NULL
        CONSTRAINT account_upstream_rate_versions_source_check
        CHECK (source IN ('default', 'manual', 'upstream_probe')),
    effective_from TIMESTAMPTZ NOT NULL,
    effective_to TIMESTAMPTZ NULL,
    observed_at TIMESTAMPTZ NULL,
    snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    change_reason VARCHAR(64) NOT NULL,
    created_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT account_upstream_rate_versions_effective_window_check
        CHECK (effective_to IS NULL OR effective_to > effective_from),
    CONSTRAINT account_upstream_rate_versions_account_version_key
        UNIQUE (account_id, version_no)
);

CREATE INDEX IF NOT EXISTS account_upstream_rate_versions_account_effective_idx
    ON account_upstream_rate_versions (account_id, effective_from DESC, id DESC);

CREATE UNIQUE INDEX IF NOT EXISTS account_upstream_rate_versions_account_current_uidx
    ON account_upstream_rate_versions (account_id)
    WHERE effective_to IS NULL;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS active_upstream_rate_version_id BIGINT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'accounts_active_upstream_rate_version_fkey'
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_active_upstream_rate_version_fkey
            FOREIGN KEY (active_upstream_rate_version_id)
            REFERENCES account_upstream_rate_versions(id)
            ON DELETE SET NULL;
    END IF;
END
$$;

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS account_rate_version_id BIGINT,
    ADD COLUMN IF NOT EXISTS account_rate_source VARCHAR(32),
    ADD COLUMN IF NOT EXISTS account_rate_snapshot JSONB;

ALTER TABLE usage_logs
    ALTER COLUMN account_rate_multiplier TYPE NUMERIC(20, 10)
    USING account_rate_multiplier::numeric;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'usage_logs_account_rate_version_fkey'
    ) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_account_rate_version_fkey
            FOREIGN KEY (account_rate_version_id)
            REFERENCES account_upstream_rate_versions(id)
            ON DELETE SET NULL;
    END IF;
END
$$;
