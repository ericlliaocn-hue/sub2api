-- Phase 5: batch image jobs persist the account rate version snapshot fixed at
-- job creation time, so settlement writes the same snapshot into usage_logs
-- even when the account's active version changes while the batch is running.

ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS account_rate_version_id BIGINT,
    ADD COLUMN IF NOT EXISTS account_rate_source VARCHAR(32),
    ADD COLUMN IF NOT EXISTS account_rate_snapshot JSONB;
