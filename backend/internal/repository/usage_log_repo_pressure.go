package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

const usagePressureActorLimit = 8

const usagePressureWindowsSQL = `
-- usage_pressure_windows
SELECT
	COUNT(*) FILTER (WHERE created_at >= $2) AS r5,
	COUNT(DISTINCT user_id) FILTER (WHERE created_at >= $2 AND user_id > 0) AS u5,
	COUNT(DISTINCT account_id) FILTER (WHERE created_at >= $2 AND account_id > 0) AS a5,
	COALESCE(SUM(actual_cost) FILTER (WHERE created_at >= $2), 0) AS b5,
	COUNT(*) FILTER (WHERE created_at >= $3) AS r15,
	COUNT(DISTINCT user_id) FILTER (WHERE created_at >= $3 AND user_id > 0) AS u15,
	COUNT(DISTINCT account_id) FILTER (WHERE created_at >= $3 AND account_id > 0) AS a15,
	COALESCE(SUM(actual_cost) FILTER (WHERE created_at >= $3), 0) AS b15,
	COUNT(*) FILTER (WHERE created_at >= $4) AS r60,
	COUNT(DISTINCT user_id) FILTER (WHERE created_at >= $4 AND user_id > 0) AS u60,
	COUNT(DISTINCT account_id) FILTER (WHERE created_at >= $4 AND account_id > 0) AS a60,
	COALESCE(SUM(actual_cost) FILTER (WHERE created_at >= $4), 0) AS b60,
	COUNT(*) AS r_today,
	COUNT(DISTINCT user_id) FILTER (WHERE user_id > 0) AS u_today,
	COUNT(DISTINCT account_id) FILTER (WHERE account_id > 0) AS a_today,
	COALESCE(SUM(actual_cost), 0) AS b_today
FROM usage_logs
WHERE created_at >= $1 AND created_at < $5
`

const usagePressurePeakHourSQL = `
-- usage_pressure_peak_hour
SELECT
	(date_trunc('hour', timezone($3, created_at)) AT TIME ZONE $3) AS hour_at,
	COUNT(*) AS requests,
	COUNT(DISTINCT user_id) FILTER (WHERE user_id > 0) AS users,
	COALESCE(SUM(actual_cost), 0) AS billed
FROM usage_logs
WHERE created_at >= $1 AND created_at < $2
GROUP BY 1
ORDER BY billed DESC, requests DESC, hour_at ASC
LIMIT 1
`

const usagePressureUsersSQL = `
-- usage_pressure_users
SELECT
	u.user_id AS id,
	COALESCE(NULLIF(us.email, ''), NULLIF(us.username, ''), '') AS name,
	COUNT(*) AS requests,
	COALESCE(SUM(u.actual_cost), 0) AS billed,
	MAX(u.created_at) AS last_at
FROM usage_logs u
LEFT JOIN users us ON us.id = u.user_id
WHERE u.created_at >= $1 AND u.created_at < $2
	AND u.user_id > 0
GROUP BY u.user_id, us.email, us.username
ORDER BY billed DESC, requests DESC, id ASC
LIMIT %d
`

const usagePressureAccountsSQL = `
-- usage_pressure_accounts
SELECT
	u.account_id AS id,
	COALESCE(NULLIF(a.name, ''), '') AS name,
	COUNT(*) AS requests,
	COALESCE(SUM(u.actual_cost), 0) AS billed,
	MAX(u.created_at) AS last_at
FROM usage_logs u
LEFT JOIN accounts a ON a.id = u.account_id
WHERE u.created_at >= $1 AND u.created_at < $2
	AND u.account_id > 0
GROUP BY u.account_id, a.name
ORDER BY billed DESC, requests DESC, id ASC
LIMIT %d
`

// GetUsagePressureSnapshot aggregates live 5/15/60m burn plus today's peak hour.
func (r *usageLogRepository) GetUsagePressureSnapshot(ctx context.Context, now, todayStart time.Time, tz string) (*usagestats.UsagePressureSnapshot, error) {
	if tz == "" || tz == "Local" {
		tz = "Asia/Shanghai"
	}
	if now.IsZero() {
		now = time.Now()
	}
	if todayStart.IsZero() || todayStart.After(now) {
		todayStart = now.Add(-24 * time.Hour)
	}

	t5 := now.Add(-5 * time.Minute)
	t15 := now.Add(-15 * time.Minute)
	t60 := now.Add(-60 * time.Minute)

	snap := &usagestats.UsagePressureSnapshot{
		Users:    []usagestats.UsagePressureActor{},
		Accounts: []usagestats.UsagePressureActor{},
	}

	var w5, w15, w60, today usagestats.UsagePressureWindow
	err := scanSingleRow(ctx, r.sql, usagePressureWindowsSQL, []any{todayStart, t5, t15, t60, now},
		&w5.Requests, &w5.Users, &w5.Accounts, &w5.Billed,
		&w15.Requests, &w15.Users, &w15.Accounts, &w15.Billed,
		&w60.Requests, &w60.Users, &w60.Accounts, &w60.Billed,
		&today.Requests, &today.Users, &today.Accounts, &today.Billed,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	snap.Windows = usagestats.UsagePressureWindows{M5: w5, M15: w15, M60: w60}
	snap.Today = today

	var peak usagestats.UsagePressurePeakHour
	err = scanSingleRow(ctx, r.sql, usagePressurePeakHourSQL, []any{todayStart, now, tz},
		&peak.Hour, &peak.Requests, &peak.Users, &peak.Billed,
	)
	switch {
	case err == nil:
		snap.PeakHour = &peak
	case !errors.Is(err, sql.ErrNoRows):
		return nil, err
	}

	users, err := r.listUsagePressureActors(ctx, fmt.Sprintf(usagePressureUsersSQL, usagePressureActorLimit), t15, now)
	if err != nil {
		return nil, err
	}
	snap.Users = users

	accounts, err := r.listUsagePressureActors(ctx, fmt.Sprintf(usagePressureAccountsSQL, usagePressureActorLimit), t15, now)
	if err != nil {
		return nil, err
	}
	snap.Accounts = accounts

	return snap, nil
}

func (r *usageLogRepository) listUsagePressureActors(ctx context.Context, query string, start, end time.Time) ([]usagestats.UsagePressureActor, error) {
	rows, err := r.sql.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]usagestats.UsagePressureActor, 0)
	for rows.Next() {
		var actor usagestats.UsagePressureActor
		if err = rows.Scan(&actor.ID, &actor.Name, &actor.Requests, &actor.Billed, &actor.LastAt); err != nil {
			return nil, err
		}
		out = append(out, actor)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
