CREATE TABLE IF NOT EXISTS recharge_bonus_grants (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE RESTRICT,
    campaign_id VARCHAR(100) NOT NULL,
    payment_amount NUMERIC(20,2) NOT NULL,
    base_credited_amount NUMERIC(20,2) NOT NULL,
    granted_amount NUMERIC(20,8) NOT NULL,
    remaining_amount NUMERIC(20,8) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT recharge_bonus_grants_order_unique UNIQUE (payment_order_id),
    CONSTRAINT recharge_bonus_grants_amounts_valid CHECK (
        granted_amount > 0 AND remaining_amount >= 0 AND remaining_amount <= granted_amount
    ),
    CONSTRAINT recharge_bonus_grants_status_valid CHECK (status IN ('active', 'consumed', 'expired', 'revoked'))
);

CREATE INDEX IF NOT EXISTS recharge_bonus_grants_user_fefo_idx
    ON recharge_bonus_grants (user_id, expires_at, id)
    WHERE status = 'active' AND remaining_amount > 0;

CREATE INDEX IF NOT EXISTS recharge_bonus_grants_campaign_user_idx
    ON recharge_bonus_grants (campaign_id, user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS recharge_bonus_hold_allocations (
    id BIGSERIAL PRIMARY KEY,
    batch_id VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    grant_id BIGINT NOT NULL REFERENCES recharge_bonus_grants(id) ON DELETE RESTRICT,
    allocated_amount NUMERIC(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT recharge_bonus_hold_allocations_unique UNIQUE (batch_id, grant_id),
    CONSTRAINT recharge_bonus_hold_allocations_amount_valid CHECK (allocated_amount > 0)
);

CREATE INDEX IF NOT EXISTS recharge_bonus_hold_allocations_batch_idx
    ON recharge_bonus_hold_allocations (batch_id, user_id, grant_id);

CREATE TABLE IF NOT EXISTS user_balance_ledgers (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type VARCHAR(40) NOT NULL,
    amount NUMERIC(20,8) NOT NULL,
    balance_before NUMERIC(20,8) NOT NULL,
    balance_after NUMERIC(20,8) NOT NULL,
    bonus_before NUMERIC(20,8) NOT NULL DEFAULT 0,
    bonus_after NUMERIC(20,8) NOT NULL DEFAULT 0,
    source_type VARCHAR(40) NOT NULL DEFAULT '',
    source_id VARCHAR(128) NOT NULL DEFAULT '',
    description VARCHAR(255) NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS user_balance_ledgers_user_created_idx
    ON user_balance_ledgers (user_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX IF NOT EXISTS user_balance_ledgers_source_unique_idx
    ON user_balance_ledgers (event_type, source_type, source_id)
    WHERE source_type <> '' AND source_id <> '';

COMMENT ON TABLE recharge_bonus_grants IS 'Expiring recharge bonus lots; consumed by earliest expiry first';
COMMENT ON TABLE recharge_bonus_hold_allocations IS 'Bonus lots allocated to asynchronous batch-image balance holds';
COMMENT ON TABLE user_balance_ledgers IS 'Unified immutable user balance change ledger';
