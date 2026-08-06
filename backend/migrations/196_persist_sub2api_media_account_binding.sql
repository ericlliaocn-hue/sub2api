-- Keep the Sub2API runtime account binding separate from the optional
-- creation.provider_account binding. The media gateway currently routes
-- through public.accounts, while the creation schema may later gain its own
-- provider registry.
ALTER TABLE creation.generation_attempt
    ADD COLUMN IF NOT EXISTS sub2api_account_id BIGINT
        REFERENCES public.accounts(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS creation_generation_attempt_sub2api_account_idx
    ON creation.generation_attempt (sub2api_account_id, updated_at DESC);
