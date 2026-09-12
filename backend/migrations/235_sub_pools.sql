-- Sub-pools: an internal isolation layer beneath a user-visible group.
--
-- A group stays the single product/pricing unit users see. Underneath it, a
-- sub-pool binds a small number of API keys to a small number of upstream
-- accounts, so that one abusive key can only burn the accounts of its own pool
-- and the blast radius stays attributable to a handful of keys.
--
-- Keys with sub_pool_id IS NULL keep the pre-existing behaviour (the whole
-- account_groups membership of the group is schedulable), so enabling this
-- feature is opt-in per group.

CREATE TABLE IF NOT EXISTS sub_pools (
    id             BIGSERIAL PRIMARY KEY,
    group_id       BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    name           VARCHAR(100) NOT NULL,
    description    TEXT,
    -- formal: long-lived pool backed by regular accounts.
    -- probe:  observation pool backed by disposable accounts; new keys land here first.
    kind           VARCHAR(20) NOT NULL DEFAULT 'formal',
    -- healthy: schedulable and accepting new key bindings.
    -- cooling: upstream accounts are flagged/overloaded; clean keys get migrated out.
    -- closed:  schedulable for already-bound keys but no longer accepting new ones.
    status         VARCHAR(20) NOT NULL DEFAULT 'healthy',
    -- Soft cap on bound keys. 0 means unlimited.
    key_soft_limit INT NOT NULL DEFAULT 8,
    cooling_until  TIMESTAMPTZ,
    cooling_reason TEXT,
    sort_order     INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

-- Name uniqueness follows the soft-delete convention used by groups.
CREATE UNIQUE INDEX IF NOT EXISTS idx_sub_pools_group_name_unique
    ON sub_pools(group_id, name)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_sub_pools_group_status
    ON sub_pools(group_id, status, kind)
    WHERE deleted_at IS NULL;

COMMENT ON TABLE sub_pools IS
    'Internal account/key isolation pools under a user-visible group';
COMMENT ON COLUMN sub_pools.kind IS 'formal | probe (probe pools use disposable accounts)';
COMMENT ON COLUMN sub_pools.status IS 'healthy | cooling | closed';
COMMENT ON COLUMN sub_pools.key_soft_limit IS 'Soft cap on bound API keys; 0 means unlimited';

-- Upstream accounts attached to a sub-pool. group_id is denormalised so that an
-- account can be constrained to at most one sub-pool per group.
CREATE TABLE IF NOT EXISTS sub_pool_accounts (
    sub_pool_id BIGINT NOT NULL REFERENCES sub_pools(id) ON DELETE CASCADE,
    account_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    -- primary: normal member. standby: only used when no primary is schedulable.
    role        VARCHAR(20) NOT NULL DEFAULT 'primary',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (sub_pool_id, account_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sub_pool_accounts_group_account_unique
    ON sub_pool_accounts(group_id, account_id);

CREATE INDEX IF NOT EXISTS idx_sub_pool_accounts_account
    ON sub_pool_accounts(account_id);

COMMENT ON TABLE sub_pool_accounts IS
    'Upstream accounts attached to a sub-pool; an account joins at most one sub-pool per group';
COMMENT ON COLUMN sub_pool_accounts.role IS 'primary | standby';

-- Current binding lives on the API key so the hot scheduling path needs no join.
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS sub_pool_id BIGINT REFERENCES sub_pools(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_api_keys_sub_pool_id
    ON api_keys(sub_pool_id)
    WHERE sub_pool_id IS NOT NULL;

COMMENT ON COLUMN api_keys.sub_pool_id IS
    'Sub-pool this key is currently bound to; NULL keeps whole-group scheduling';

-- Append-only binding history. Attribution after an incident needs to know which
-- pool a key was in at the time, not just where it sits now.
CREATE TABLE IF NOT EXISTS api_key_sub_pool_bindings (
    id          BIGSERIAL PRIMARY KEY,
    api_key_id  BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    sub_pool_id BIGINT NOT NULL REFERENCES sub_pools(id) ON DELETE CASCADE,
    group_id    BIGINT NOT NULL,
    bound_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    unbound_at  TIMESTAMPTZ,
    -- initial | probe_graduation | cooling_migration | admin_manual | pool_removed
    reason      VARCHAR(40) NOT NULL,
    -- 'system' or 'admin:<user_id>'
    operator    VARCHAR(64) NOT NULL DEFAULT 'system',
    note        TEXT
);

CREATE INDEX IF NOT EXISTS idx_api_key_sub_pool_bindings_key
    ON api_key_sub_pool_bindings(api_key_id, bound_at DESC);

CREATE INDEX IF NOT EXISTS idx_api_key_sub_pool_bindings_pool
    ON api_key_sub_pool_bindings(sub_pool_id, bound_at DESC);

-- One open binding per key at a time.
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_key_sub_pool_bindings_open_unique
    ON api_key_sub_pool_bindings(api_key_id)
    WHERE unbound_at IS NULL;

COMMENT ON TABLE api_key_sub_pool_bindings IS
    'Append-only history of API key to sub-pool bindings, used for incident attribution';
COMMENT ON COLUMN api_key_sub_pool_bindings.reason IS
    'initial | probe_graduation | cooling_migration | admin_manual | pool_removed';

-- Per-group switch. Groups keep whole-group scheduling until an admin turns this on.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS sub_pool_enabled BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.sub_pool_enabled IS
    'Whether API keys in this group are scheduled through sub-pools instead of the whole group';
