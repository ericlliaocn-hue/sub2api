ALTER TABLE recharge_bonus_grants
    ALTER COLUMN payment_order_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(40) NOT NULL DEFAULT 'payment_order',
    ADD COLUMN IF NOT EXISTS source_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS granted_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS notes VARCHAR(255) NOT NULL DEFAULT '';

UPDATE recharge_bonus_grants
SET source_type = 'payment_order',
    source_id = payment_order_id::text
WHERE payment_order_id IS NOT NULL
  AND (source_id = '' OR source_type = 'payment_order');

CREATE UNIQUE INDEX IF NOT EXISTS recharge_bonus_grants_source_user_unique_idx
    ON recharge_bonus_grants (source_type, source_id, user_id)
    WHERE source_id <> '';

CREATE INDEX IF NOT EXISTS recharge_bonus_grants_expiry_sweep_idx
    ON recharge_bonus_grants (expires_at, user_id)
    WHERE status = 'active' AND remaining_amount > 0;

COMMENT ON COLUMN recharge_bonus_grants.source_type IS 'Grant origin such as payment_order or admin_campaign';
COMMENT ON COLUMN recharge_bonus_grants.source_id IS 'Origin-scoped idempotency identifier';
COMMENT ON COLUMN recharge_bonus_grants.granted_by IS 'Administrator user id for manual campaign grants';
