-- 获客来源归因：官网 / SEO / 邀请 / 其他。
-- 每个用户都有且只有一行归因；没有任何证据的默认官网。
-- 获客分类与邀请返利（user_affiliates.inviter_id）是两个维度，互不覆盖。

-- 1. 渠道类型收敛为四分类，并标记系统渠道。
UPDATE promotion_channels
SET channel_type = CASE
    WHEN LOWER(channel_type) IN ('official', 'seo', 'invite', 'other') THEN LOWER(channel_type)
    ELSE 'other'
END;

ALTER TABLE promotion_channels
    DROP CONSTRAINT IF EXISTS promotion_channels_channel_type_check;
ALTER TABLE promotion_channels
    ADD CONSTRAINT promotion_channels_channel_type_check
    CHECK (channel_type IN ('official', 'seo', 'invite', 'other'));

ALTER TABLE promotion_channels
    ADD COLUMN IF NOT EXISTS system BOOLEAN NOT NULL DEFAULT FALSE;

INSERT INTO promotion_channels (code, name, channel_type, enabled, notes, system)
VALUES
    ('OFFICIAL', '官网直接进入', 'official', TRUE, '系统渠道：无任何来源证据时的默认归因', TRUE),
    ('SEO', '搜索引擎', 'seo', TRUE, '系统渠道：referrer 为搜索引擎或 AI 搜索', TRUE),
    ('INVITE', '邀请链接', 'invite', TRUE, '系统渠道：通过 ?aff= 邀请链接进入', TRUE),
    ('MANUAL', '后台创建', 'other', TRUE, '系统渠道：管理员或 API 手工创建的账号', TRUE),
    ('EXTERNAL', '外站与付费流量', 'other', TRUE, '系统渠道：外站 referrer 或带付费点击参数但没有渠道码', TRUE)
ON CONFLICT (code) DO UPDATE SET system = TRUE;

-- 2. 归因表补充分类与证据。
ALTER TABLE promotion_user_attributions
    ADD COLUMN IF NOT EXISTS acquisition_class VARCHAR(16) NOT NULL DEFAULT 'official',
    ADD COLUMN IF NOT EXISTS landing_path TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS referrer TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS utm JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS evidence JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE promotion_user_attributions
    DROP CONSTRAINT IF EXISTS promotion_user_attributions_class_check;
ALTER TABLE promotion_user_attributions
    ADD CONSTRAINT promotion_user_attributions_class_check
    CHECK (acquisition_class IN ('official', 'seo', 'invite', 'other'));

CREATE INDEX IF NOT EXISTS idx_promotion_attributions_class_time
    ON promotion_user_attributions(acquisition_class, attributed_at);

-- 3. 审计事件补充分类，放开 outcome 枚举。
ALTER TABLE promotion_attribution_events
    ADD COLUMN IF NOT EXISTS acquisition_class VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE promotion_attribution_events
    DROP CONSTRAINT IF EXISTS promotion_attribution_events_outcome_check;
ALTER TABLE promotion_attribution_events
    ADD CONSTRAINT promotion_attribution_events_outcome_check
    CHECK (outcome IN ('attributed', 'already_attributed', 'invalid_code', 'channel_disabled', 'default_official', 'resolved'));

-- 4. 历史回填。
-- 4a. 已有归因行：分类跟随渠道类型。
UPDATE promotion_user_attributions a
SET acquisition_class = c.channel_type,
    evidence = CASE WHEN a.evidence = '{}'::jsonb THEN '{"reason":"backfill_channel_type"}'::jsonb ELSE a.evidence END
FROM promotion_channels c
WHERE c.id = a.channel_id;

-- 4b. 有邀请人但没有归因行：邀请。
INSERT INTO promotion_user_attributions (user_id, channel_id, acquisition_class, first_seen_at, attributed_at, evidence)
SELECT ua.user_id, c.id, 'invite', u.created_at, u.created_at, '{"reason":"backfill_inviter"}'::jsonb
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id
JOIN promotion_channels c ON c.code = 'INVITE'
WHERE ua.inviter_id IS NOT NULL
ON CONFLICT (user_id) DO NOTHING;

-- 4c. 管理员创建（创建流水带 actor_user_id）且没有归因行：后台创建。
INSERT INTO promotion_user_attributions (user_id, channel_id, acquisition_class, first_seen_at, attributed_at, evidence)
SELECT DISTINCT l.user_id, c.id, 'other', u.created_at, u.created_at, '{"reason":"backfill_admin_created"}'::jsonb
FROM user_balance_ledgers l
JOIN users u ON u.id = l.user_id
JOIN promotion_channels c ON c.code = 'MANUAL'
WHERE l.source_type = 'user_creation'
  AND COALESCE(NULLIF(l.metadata->>'actor_user_id', ''), '0') ~ '^[0-9]+$'
  AND (l.metadata->>'actor_user_id')::bigint > 0
ON CONFLICT (user_id) DO NOTHING;

-- 4d. 其余全部默认官网。
INSERT INTO promotion_user_attributions (user_id, channel_id, acquisition_class, first_seen_at, attributed_at, evidence)
SELECT u.id, c.id, 'official', u.created_at, u.created_at, '{"reason":"backfill_default_official"}'::jsonb
FROM users u
JOIN promotion_channels c ON c.code = 'OFFICIAL'
WHERE NOT EXISTS (SELECT 1 FROM promotion_user_attributions a WHERE a.user_id = u.id);

-- 5. 访问计数：按天聚合，不存单次访问。
CREATE TABLE IF NOT EXISTS acquisition_visits_daily (
    day DATE NOT NULL,
    acquisition_class VARCHAR(16) NOT NULL,
    channel_id BIGINT NOT NULL DEFAULT 0,
    engine VARCHAR(64) NOT NULL DEFAULT '',
    referrer_host VARCHAR(255) NOT NULL DEFAULT '',
    landing_path VARCHAR(512) NOT NULL DEFAULT '',
    visits BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT acquisition_visits_daily_class_check
        CHECK (acquisition_class IN ('official', 'seo', 'invite', 'other'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_acquisition_visits_daily_key
    ON acquisition_visits_daily(day, acquisition_class, channel_id, engine, referrer_host, landing_path);
CREATE INDEX IF NOT EXISTS idx_acquisition_visits_daily_day
    ON acquisition_visits_daily(day);

COMMENT ON COLUMN promotion_user_attributions.acquisition_class IS '获客分类：official 官网 / seo 搜索 / invite 邀请 / other 其他；与邀请返利分开记';
COMMENT ON COLUMN promotion_channels.system IS '系统渠道：不可删除、不可改编码与类型、不计佣金';
COMMENT ON TABLE acquisition_visits_daily IS '获客落地访问按天聚合，用于访问到注册的转化率';
