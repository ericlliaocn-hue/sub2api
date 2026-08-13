CREATE TABLE IF NOT EXISTS promotion_promoters (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    contact VARCHAR(255) NOT NULL DEFAULT '',
    commission_rate NUMERIC(10,4) NOT NULL DEFAULT 0 CHECK (commission_rate >= 0 AND commission_rate <= 100),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS promotion_channels (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    channel_type VARCHAR(64) NOT NULL DEFAULT 'other',
    promoter_id BIGINT REFERENCES promotion_promoters(id) ON DELETE SET NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS promotion_user_attributions (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    channel_id BIGINT NOT NULL REFERENCES promotion_channels(id) ON DELETE RESTRICT,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attributed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_promotion_channels_promoter ON promotion_channels(promoter_id);
CREATE INDEX IF NOT EXISTS idx_promotion_attributions_channel ON promotion_user_attributions(channel_id);
CREATE INDEX IF NOT EXISTS idx_promotion_attributions_time ON promotion_user_attributions(attributed_at);
