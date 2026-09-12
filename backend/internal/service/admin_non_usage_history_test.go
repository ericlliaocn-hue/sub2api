package service

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Uses an isolated schema, never application tables. Set HISTORY_TEST_DSN to run.
func TestNonUsageHistoryPostgres(t *testing.T) {
	dsn := os.Getenv("HISTORY_TEST_DSN")
	if dsn == "" {
		t.Skip("HISTORY_TEST_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TEMP TABLE redeem_codes(id bigint,code text,type text,value numeric,used_at timestamptz,created_at timestamptz,notes text,group_id bigint,validity_days int,used_by bigint);
 CREATE TEMP TABLE user_affiliate_ledger(id bigint,user_id bigint,action text,amount numeric,created_at timestamptz);
 CREATE TEMP TABLE user_balance_ledgers(id bigint,user_id bigint,event_type text,amount numeric,created_at timestamptz,description text,source_type text,source_id text,metadata jsonb);
 INSERT INTO redeem_codes VALUES(1,'PAY-1','balance',10,now(),now(),'paid',NULL,0,1);
 INSERT INTO user_balance_ledgers VALUES
 (1,1,'balance_redeem',10,now(),'','redeem_code','PAY-1','{}'),
 (2,1,'registration_bonus',5,now(),'','user_creation','1','{}'),
 (3,1,'recharge_bonus_granted',3,now(),'','payment_order','1','{}'),
 (4,1,'usage_charge',-1,now(),'','usage_request','x','{}'),
 (5,1,'batch_hold',-1,now(),'','','','{}'),
 (6,1,'batch_capture',-1,now(),'','','','{}'),
 (7,1,'batch_release',1,now(),'','','','{}'),
 (8,2,'registration_bonus',5,now(),'','user_creation','2','{}'),
 (9,1,'payment_refund',-10,now(),'','payment_order','1','{}');
 INSERT INTO user_affiliate_ledger VALUES(2,1,'transfer',2,now()),(3,1,'accrue',3,now());`)
	require.NoError(t, err)
	var count int
	var amount float64
	require.NoError(t, tx.QueryRow(nonUsageHistorySQL+`SELECT count(*),sum(amount) FROM history WHERE ($2='' OR kind=$2)`, 1, "").Scan(&count, &amount))
	require.Equal(t, 5, count)
	require.Equal(t, 10.0, amount)
	require.NoError(t, tx.QueryRow(nonUsageHistorySQL+`SELECT count(*) FROM history WHERE ($2='' OR kind=$2)`, 1, "registration_bonus").Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, tx.QueryRow(nonUsageHistorySQL+`SELECT count(*) FROM history WHERE ($2='' OR kind=$2)`, 1, "usage_charge").Scan(&count))
	require.Zero(t, count)
}

func TestNonUsageEntitlementTriggers(t *testing.T) {
	dsn := os.Getenv("HISTORY_TEST_DSN")
	if dsn == "" {
		t.Skip("HISTORY_TEST_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE SCHEMA history_trigger_test; SET LOCAL search_path=history_trigger_test,public;
 CREATE TABLE users(id bigint primary key,balance numeric);
 INSERT INTO users VALUES(1,10);
 CREATE TABLE user_balance_ledgers(id bigserial,user_id bigint,event_type text,amount numeric,balance_before numeric,balance_after numeric,source_type text,source_id text,description text,metadata jsonb);
 CREATE TABLE user_subscriptions(id bigint,user_id bigint,group_id bigint,starts_at timestamptz,expires_at timestamptz,status text,deleted_at timestamptz,daily_usage_usd numeric);
 CREATE TABLE recharge_bonus_grants(id bigint,user_id bigint,expires_at timestamptz,remaining_amount numeric);`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../migrations/238_non_usage_history.sql")
	require.NoError(t, err)
	_, err = tx.Exec(string(migration))
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO user_subscriptions VALUES(1,1,1,now(),now()+interval '1 day','active',NULL,0);
 UPDATE user_subscriptions SET daily_usage_usd=1;
 UPDATE user_subscriptions SET expires_at=expires_at+interval '1 day';
 DELETE FROM user_subscriptions;
 INSERT INTO recharge_bonus_grants VALUES(1,1,now(),3);
 UPDATE recharge_bonus_grants SET remaining_amount=2;
 UPDATE recharge_bonus_grants SET expires_at=expires_at+interval '1 day';`)
	require.NoError(t, err)
	var count int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM user_balance_ledgers`).Scan(&count))
	require.Equal(t, 4, count)
	var balance float64
	require.NoError(t, tx.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 10.0, balance)
}
