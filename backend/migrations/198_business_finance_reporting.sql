-- Business finance reporting extensions.
-- These fields only extend the independent management layer; the gateway
-- billing path and existing rate_multiplier columns remain unchanged.

ALTER TABLE business_cost_configs
    ADD COLUMN IF NOT EXISTS frequency VARCHAR(20) NOT NULL DEFAULT 'monthly';

ALTER TABLE business_cost_configs
    DROP CONSTRAINT IF EXISTS business_cost_configs_frequency_check;

ALTER TABLE business_cost_configs
    ADD CONSTRAINT business_cost_configs_frequency_check
    CHECK (frequency IN ('one_time', 'daily', 'monthly', 'yearly'));

CREATE INDEX IF NOT EXISTS business_cost_configs_reporting_idx
    ON business_cost_configs (enabled, effective_from, effective_to, frequency);

CREATE INDEX IF NOT EXISTS business_expenses_reporting_idx
    ON business_expenses (status, occurred_at, period_start, period_end);

CREATE INDEX IF NOT EXISTS usage_logs_business_finance_reporting_idx
    ON usage_logs (created_at, group_id, channel_id, account_id, model);
