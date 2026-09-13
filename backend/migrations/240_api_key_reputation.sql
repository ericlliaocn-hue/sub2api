-- API Key 信誉分（Phase D）
--
-- 现有的自动封禁是「用户」级：content_moderation_logs 在窗口内累计 flagged
-- 次数达阈值后禁用 users 行。本表是「Key」级的一层，对应方案 §6.2 的处置顺序
-- ——先罚肇事 Key（限速 → 打回观察池 → 封 Key），升级后才轮到封用户。
--
-- 分数是派生值：由后台任务按滚动窗口从 prompt_audit_events 与
-- content_moderation_logs 重算，本表只做物化与制裁状态记录，不是事实来源。
-- 因此表可以随时清空重建，唯一会丢的是「已制裁」标记。

CREATE TABLE IF NOT EXISTS api_key_reputation (
    id              BIGSERIAL PRIMARY KEY,
    api_key_id      BIGINT NOT NULL UNIQUE REFERENCES api_keys(id) ON DELETE CASCADE,
    -- 100 = 干净，0 = 最差。
    score           INT NOT NULL DEFAULT 100,
    -- 窗口内的命中拆分，供管理端解释分数怎么来的。
    severe_hits     INT NOT NULL DEFAULT 0,
    total_hits      INT NOT NULL DEFAULT 0,
    last_event_at   TIMESTAMPTZ,
    scored_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- none | demoted | disabled。记录已执行的制裁，避免每轮重复触发。
    sanction        VARCHAR(20) NOT NULL DEFAULT 'none',
    sanctioned_at   TIMESTAMPTZ,
    sanction_reason TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 管理端「最差的 N 个 Key」列表。
CREATE INDEX IF NOT EXISTS idx_api_key_reputation_score
    ON api_key_reputation(score);

-- 找出待制裁的 Key：绝大多数行是 sanction='none' 且 score=100，部分索引让
-- 这个查询只扫真正有问题的那一小撮。
CREATE INDEX IF NOT EXISTS idx_api_key_reputation_pending_sanction
    ON api_key_reputation(score)
    WHERE sanction = 'none' AND score < 100;

COMMENT ON TABLE api_key_reputation IS
    'Per-API-key reputation derived from moderation and prompt-audit hits (Phase D).';
COMMENT ON COLUMN api_key_reputation.score IS
    'Derived 0-100 score; recomputed by the reputation job, not authoritative.';
COMMENT ON COLUMN api_key_reputation.sanction IS
    'Sanction already applied by the reputation job: none | demoted | disabled.';
