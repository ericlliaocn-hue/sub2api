package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type subPoolUsageRepository struct {
	sql sqlExecutor
}

// NewSubPoolUsageRepository serves the sub-pool attribution queries. It is a
// narrow interface on purpose: UsageLogRepository is already very wide and has
// many test doubles, and these two queries have nothing to do with billing.
func NewSubPoolUsageRepository(_ *dbent.Client, sqlDB *sql.DB) service.SubPoolUsageRepository {
	return &subPoolUsageRepository{sql: sqlDB}
}

// PeerActivity aggregates each key's calls since its own binding started, so a
// peer who joined the pool yesterday is not credited with last month's traffic.
func (r *subPoolUsageRepository) PeerActivity(ctx context.Context, keys []service.SubPoolPeerWindow, todayStart, weekStart time.Time) (map[int64]service.SubPoolPeerActivity, error) {
	out := make(map[int64]service.SubPoolPeerActivity, len(keys))
	if len(keys) == 0 {
		return out, nil
	}

	values := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)*2+2)
	args = append(args, todayStart, weekStart)
	for _, k := range keys {
		values = append(values, fmt.Sprintf("($%d::bigint, $%d::timestamptz)", len(args)+1, len(args)+2))
		args = append(args, k.APIKeyID, k.Since)
	}

	query := fmt.Sprintf(`
		SELECT k.api_key_id,
		       MIN(u.created_at) AS first_call_at,
		       MAX(u.created_at) AS last_call_at,
		       COUNT(*) FILTER (WHERE u.created_at >= $1) AS today_count,
		       COUNT(*) FILTER (WHERE u.created_at >= $2) AS week_count
		FROM (VALUES %s) AS k(api_key_id, since)
		JOIN usage_logs u
		  ON u.api_key_id = k.api_key_id
		 AND u.created_at >= k.since
		GROUP BY k.api_key_id`, strings.Join(values, ", "))

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sub-pool peer activity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			apiKeyID    int64
			firstCallAt sql.NullTime
			lastCallAt  sql.NullTime
			todayCount  int64
			weekCount   int64
		)
		if err := rows.Scan(&apiKeyID, &firstCallAt, &lastCallAt, &todayCount, &weekCount); err != nil {
			return nil, fmt.Errorf("scan sub-pool peer activity: %w", err)
		}
		activity := service.SubPoolPeerActivity{
			APIKeyID:   apiKeyID,
			TodayCalls: todayCount,
			WeekCalls:  weekCount,
		}
		if firstCallAt.Valid {
			t := firstCallAt.Time
			activity.FirstCallAt = &t
		}
		if lastCallAt.Valid {
			t := lastCallAt.Time
			activity.LastCallAt = &t
		}
		out[apiKeyID] = activity
	}
	return out, rows.Err()
}

// TopKeysByAccount answers the attribution question after an incident: which
// keys drove traffic through the burned account in the window.
func (r *subPoolUsageRepository) TopKeysByAccount(ctx context.Context, accountID int64, start, end time.Time, limit int) ([]service.SubPoolAccountKeyUsage, error) {
	if limit <= 0 {
		limit = 20
	}
	const query = `
		SELECT u.api_key_id,
		       u.user_id,
		       COUNT(*) AS calls,
		       MIN(u.created_at) AS first_call_at,
		       MAX(u.created_at) AS last_call_at
		FROM usage_logs u
		WHERE u.account_id = $1
		  AND u.created_at >= $2
		  AND u.created_at < $3
		GROUP BY u.api_key_id, u.user_id
		ORDER BY calls DESC
		LIMIT $4`

	rows, err := r.sql.QueryContext(ctx, query, accountID, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("query top keys by account: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.SubPoolAccountKeyUsage, 0, limit)
	for rows.Next() {
		var (
			item        service.SubPoolAccountKeyUsage
			firstCallAt sql.NullTime
			lastCallAt  sql.NullTime
		)
		if err := rows.Scan(&item.APIKeyID, &item.UserID, &item.Calls, &firstCallAt, &lastCallAt); err != nil {
			return nil, fmt.Errorf("scan top keys by account: %w", err)
		}
		if firstCallAt.Valid {
			t := firstCallAt.Time
			item.FirstCallAt = &t
		}
		if lastCallAt.Valid {
			t := lastCallAt.Time
			item.LastCallAt = &t
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// CountViolationsSince is the "did this key misbehave during probation" check.
//
// It counts only unambiguous hits — a flagged moderation log, or a prompt-audit
// event that was blocked or scored high/critical. Warnings and medium scores are
// left out deliberately: probation should filter out abusers, not everyone who
// ever tripped a fuzzy scanner.
func (r *subPoolUsageRepository) CountViolationsSince(ctx context.Context, apiKeyID int64, since time.Time) (int64, error) {
	const query = `
		SELECT
		  (SELECT COUNT(*) FROM content_moderation_logs
		    WHERE api_key_id = $1 AND created_at >= $2 AND flagged) +
		  (SELECT COUNT(*) FROM prompt_audit_events
		    WHERE api_key_id = $1 AND created_at >= $2
		      AND (decision = 'block' OR risk_level IN ('high', 'critical')))`

	count, err := r.scanSingleCount(ctx, query, apiKeyID, since)
	if err != nil {
		return 0, fmt.Errorf("count sub-pool probation violations: %w", err)
	}
	return count, nil
}

// MaxDailyCallsSince returns the busiest single day of the key since it entered
// the pool. Days are bucketed in the site timezone so the number matches what
// the peer board shows users.
func (r *subPoolUsageRepository) MaxDailyCallsSince(ctx context.Context, apiKeyID int64, since time.Time) (int64, error) {
	const query = `
		SELECT COALESCE(MAX(daily.calls), 0)
		FROM (
		    SELECT COUNT(*) AS calls
		    FROM usage_logs
		    WHERE api_key_id = $1 AND created_at >= $2
		    GROUP BY date_trunc('day', created_at AT TIME ZONE $3)
		) AS daily`

	peak, err := r.scanSingleCount(ctx, query, apiKeyID, since, timezone.Name())
	if err != nil {
		return 0, fmt.Errorf("query sub-pool probation peak usage: %w", err)
	}
	return peak, nil
}

// scanSingleCount runs a query that yields exactly one aggregate row.
// sqlExecutor deliberately exposes only QueryContext, so the row is read here
// rather than widening an interface that many repositories share.
func (r *subPoolUsageRepository) scanSingleCount(ctx context.Context, query string, args ...any) (int64, error) {
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	var count int64
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}
	}
	return count, rows.Err()
}
