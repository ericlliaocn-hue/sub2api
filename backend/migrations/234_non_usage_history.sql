-- Capture entitlement changes only. Usage window counters are intentionally ignored.
CREATE OR REPLACE FUNCTION record_subscription_history() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE target_user bigint; balance_now numeric; payload jsonb; event_name text;
BEGIN
 IF TG_OP='UPDATE' AND OLD.starts_at IS NOT DISTINCT FROM NEW.starts_at
 AND OLD.expires_at IS NOT DISTINCT FROM NEW.expires_at
 AND OLD.status IS NOT DISTINCT FROM NEW.status
 AND OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN RETURN NEW; END IF;
 target_user := COALESCE(NEW.user_id,OLD.user_id);
 SELECT balance INTO balance_now FROM users WHERE id=target_user;
 IF NOT FOUND THEN RETURN COALESCE(NEW,OLD); END IF;
 event_name := CASE WHEN TG_OP='INSERT' THEN 'subscription_created'
 WHEN TG_OP='DELETE' THEN 'subscription_removed' ELSE 'subscription_changed' END;
 payload := jsonb_build_object('group_id',COALESCE(NEW.group_id,OLD.group_id),
 'previous_expires_at',CASE WHEN TG_OP='INSERT' THEN NULL ELSE OLD.expires_at END,
 'expires_at',CASE WHEN TG_OP='DELETE' THEN NULL ELSE NEW.expires_at END,
 'previous_status',CASE WHEN TG_OP='INSERT' THEN NULL ELSE OLD.status END,
 'status',CASE WHEN TG_OP='DELETE' THEN 'removed' ELSE NEW.status END);
 INSERT INTO user_balance_ledgers(user_id,event_type,amount,balance_before,balance_after,source_type,source_id,description,metadata)
 VALUES(target_user,event_name,0,balance_now,balance_now,'subscription',COALESCE(NEW.id,OLD.id)::text || ':' || gen_random_uuid()::text,'',payload);
 RETURN COALESCE(NEW,OLD);
END $$;
CREATE TRIGGER trg_subscription_non_usage_history AFTER INSERT OR UPDATE OR DELETE ON user_subscriptions
FOR EACH ROW EXECUTE FUNCTION record_subscription_history();

CREATE OR REPLACE FUNCTION record_bonus_expiry_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE balance_now numeric;
BEGIN
 IF OLD.expires_at IS NOT DISTINCT FROM NEW.expires_at THEN RETURN NEW; END IF;
 SELECT balance INTO balance_now FROM users WHERE id=NEW.user_id;
 INSERT INTO user_balance_ledgers(user_id,event_type,amount,balance_before,balance_after,source_type,source_id,description,metadata)
 VALUES(NEW.user_id,'bonus_expiry_changed',0,balance_now,balance_now,'bonus_grant',NEW.id::text || ':' || gen_random_uuid()::text,'',
 jsonb_build_object('previous_expires_at',OLD.expires_at,'expires_at',NEW.expires_at));
 RETURN NEW;
END $$;
CREATE TRIGGER trg_bonus_expiry_history AFTER UPDATE OF expires_at ON recharge_bonus_grants
FOR EACH ROW EXECUTE FUNCTION record_bonus_expiry_change();
