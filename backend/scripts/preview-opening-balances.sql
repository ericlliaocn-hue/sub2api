-- Read-only candidate list. An opening amount must be established from a
-- contemporaneous record; current balance and current defaults are not evidence.
BEGIN READ ONLY;
SELECT u.id,u.email,u.created_at,u.balance,
       CASE WHEN EXISTS (SELECT 1 FROM user_balance_ledgers l
          WHERE l.user_id=u.id AND l.source_type='user_creation')
         THEN 'recorded' ELSE 'requires_opening_evidence' END AS opening_history,
       (SELECT MIN(a.id) FROM audit_logs a
         WHERE a.action='admin.users.create' AND a.status_code BETWEEN 200 AND 299
           AND a.created_at BETWEEN u.created_at-INTERVAL '5 seconds' AND u.created_at+INTERVAL '5 seconds') AS candidate_admin_audit_id
FROM users u WHERE u.deleted_at IS NULL ORDER BY u.id;
COMMIT;
