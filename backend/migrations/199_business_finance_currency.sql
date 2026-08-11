-- Store the conversion used to express an operating cost in the same unit as
-- usage_logs.actual_cost. Existing rows default to 1 for backwards compatibility.

ALTER TABLE business_cost_configs
    ADD COLUMN IF NOT EXISTS exchange_rate_to_billing_unit NUMERIC(20, 10) NOT NULL DEFAULT 1;

ALTER TABLE business_cost_configs
    DROP CONSTRAINT IF EXISTS business_cost_configs_exchange_rate_check;

ALTER TABLE business_cost_configs
    ADD CONSTRAINT business_cost_configs_exchange_rate_check
    CHECK (exchange_rate_to_billing_unit > 0);

ALTER TABLE business_expenses
    ADD COLUMN IF NOT EXISTS exchange_rate_to_billing_unit NUMERIC(20, 10) NOT NULL DEFAULT 1;

ALTER TABLE business_expenses
    DROP CONSTRAINT IF EXISTS business_expenses_exchange_rate_check;

ALTER TABLE business_expenses
    ADD CONSTRAINT business_expenses_exchange_rate_check
    CHECK (exchange_rate_to_billing_unit > 0);
