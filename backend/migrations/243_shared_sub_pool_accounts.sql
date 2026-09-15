-- Allow one upstream account to back more than one sub-pool in the same group.
-- Isolation still holds per key: a key only sees the accounts of its own pool.
-- Shared accounts (e.g. a new channel both formal and probe should use) are
-- an explicit admin choice; the blast radius of that account is then the union
-- of the pools that list it.

DROP INDEX IF EXISTS idx_sub_pool_accounts_group_account_unique;

COMMENT ON TABLE sub_pool_accounts IS
    'Upstream accounts attached to a sub-pool; the same account may join multiple pools in one group';
