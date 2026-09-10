-- Run only after approving the preview and independent opening evidence.
-- Required psql variables: user_id, email, amount, event_type, evidence.
-- Example event_type: registration_bonus or initial_balance.
\set ON_ERROR_STOP on
BEGIN;
CREATE TEMP TABLE opening_backfill_input ON COMMIT DROP AS
SELECT :'user_id'::bigint AS user_id, :'email'::text AS email,
       :'amount'::numeric(20,8) AS amount, :'event_type'::text AS event_type,
       :'evidence'::text AS evidence;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM opening_backfill_input WHERE amount<0 OR evidence='' OR
 event_type NOT IN ('registration_bonus','initial_balance')) THEN
 RAISE EXCEPTION 'Invalid opening evidence'; END IF;
 IF (SELECT COUNT(*) FROM users u JOIN opening_backfill_input i ON i.user_id=u.id AND lower(i.email)=lower(u.email))<>1 THEN
 RAISE EXCEPTION 'User identity mismatch'; END IF;
END $$;
SELECT u.id FROM users u JOIN opening_backfill_input i ON i.user_id=u.id FOR UPDATE OF u;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM user_balance_ledgers l JOIN opening_backfill_input i ON i.user_id=l.user_id
 WHERE l.source_type='user_creation' AND (l.amount<>i.amount OR l.event_type<>i.event_type)) THEN
 RAISE EXCEPTION 'Conflicting opening record'; END IF;
END $$;
INSERT INTO user_balance_ledgers(user_id,event_type,amount,balance_before,balance_after,source_type,source_id,description,metadata,created_at)
SELECT u.id,i.event_type,i.amount,0,i.amount,'user_creation',u.id::text,'Historical opening record',
 jsonb_build_object('notes',i.evidence,'backfilled_at',NOW()),u.created_at
FROM users u JOIN opening_backfill_input i ON i.user_id=u.id
WHERE NOT EXISTS(SELECT 1 FROM user_balance_ledgers l WHERE l.user_id=u.id AND l.source_type='user_creation');
COMMIT;
