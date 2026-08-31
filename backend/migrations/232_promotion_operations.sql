-- 推广运营闭环：负责人配置、归因审计、佣金冻结与人工结算。
-- 保留既有 promotion_* 主表和 ?source=CODE 链接，历史归因无需迁移。

ALTER TABLE promotion_promoters
    ADD COLUMN IF NOT EXISTS commission_freeze_days INTEGER NOT NULL DEFAULT 7;

ALTER TABLE promotion_promoters
    DROP CONSTRAINT IF EXISTS promotion_promoters_commission_freeze_days_check;
ALTER TABLE promotion_promoters
    ADD CONSTRAINT promotion_promoters_commission_freeze_days_check
    CHECK (commission_freeze_days >= 0 AND commission_freeze_days <= 365);

ALTER TABLE promotion_channels
    ADD COLUMN IF NOT EXISTS commission_rate NUMERIC(10,4) NULL;

ALTER TABLE promotion_channels
    DROP CONSTRAINT IF EXISTS promotion_channels_commission_rate_check;
ALTER TABLE promotion_channels
    ADD CONSTRAINT promotion_channels_commission_rate_check
    CHECK (commission_rate IS NULL OR (commission_rate >= 0 AND commission_rate <= 100));

CREATE TABLE IF NOT EXISTS promotion_attribution_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requested_code VARCHAR(64) NOT NULL,
    channel_id BIGINT NULL REFERENCES promotion_channels(id) ON DELETE SET NULL,
    outcome VARCHAR(24) NOT NULL,
    detail VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT promotion_attribution_events_outcome_check
        CHECK (outcome IN ('attributed', 'already_attributed', 'invalid_code', 'channel_disabled'))
);

CREATE INDEX IF NOT EXISTS idx_promotion_attribution_events_created
    ON promotion_attribution_events(created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_promotion_attribution_events_user
    ON promotion_attribution_events(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS promotion_commission_settlements (
    id BIGSERIAL PRIMARY KEY,
    promoter_id BIGINT NOT NULL REFERENCES promotion_promoters(id) ON DELETE RESTRICT,
    period_end TIMESTAMPTZ NOT NULL,
    amount NUMERIC(20,8) NOT NULL CHECK (amount >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    notes TEXT NOT NULL DEFAULT '',
    paid_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT promotion_commission_settlements_status_check
        CHECK (status IN ('draft', 'paid', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_promotion_settlements_promoter
    ON promotion_commission_settlements(promoter_id, created_at DESC);

CREATE TABLE IF NOT EXISTS promotion_commission_ledger (
    id BIGSERIAL PRIMARY KEY,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    channel_id BIGINT NOT NULL REFERENCES promotion_channels(id) ON DELETE RESTRICT,
    promoter_id BIGINT NOT NULL REFERENCES promotion_promoters(id) ON DELETE RESTRICT,
    settlement_id BIGINT NULL REFERENCES promotion_commission_settlements(id) ON DELETE SET NULL,
    base_amount NUMERIC(20,8) NOT NULL CHECK (base_amount > 0),
    commission_rate NUMERIC(10,4) NOT NULL CHECK (commission_rate > 0 AND commission_rate <= 100),
    amount NUMERIC(20,8) NOT NULL CHECK (amount > 0),
    reversed_amount NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (reversed_amount >= 0),
    currency VARCHAR(16) NOT NULL DEFAULT 'BILLING',
    status VARCHAR(24) NOT NULL,
    frozen_until TIMESTAMPTZ NULL,
    channel_code_snapshot VARCHAR(64) NOT NULL,
    channel_name_snapshot VARCHAR(128) NOT NULL,
    promoter_name_snapshot VARCHAR(128) NOT NULL,
    reversed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT promotion_commission_ledger_order_unique UNIQUE (payment_order_id),
    CONSTRAINT promotion_commission_ledger_reversal_check CHECK (reversed_amount <= amount),
    CONSTRAINT promotion_commission_ledger_status_check
        CHECK (status IN ('frozen', 'available', 'settled', 'reversed'))
);

CREATE INDEX IF NOT EXISTS idx_promotion_commission_promoter_status
    ON promotion_commission_ledger(promoter_id, status, frozen_until, created_at);
CREATE INDEX IF NOT EXISTS idx_promotion_commission_channel_created
    ON promotion_commission_ledger(channel_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_promotion_commission_settlement
    ON promotion_commission_ledger(settlement_id)
    WHERE settlement_id IS NOT NULL;

COMMENT ON TABLE promotion_attribution_events IS '推广注册归因尝试审计，包含无效和重复归因';
COMMENT ON TABLE promotion_commission_ledger IS '推广渠道佣金不可变订单快照及退款冲正状态';
COMMENT ON TABLE promotion_commission_settlements IS '推广成员人工结算批次，不代表系统自动打款';
