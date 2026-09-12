package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbreputation "github.com/Wei-Shaw/sub2api/ent/apikeyreputation"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type apiKeyReputationRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewAPIKeyReputationRepository(client *dbent.Client, sqlDB *sql.DB) service.APIKeyReputationRepository {
	return &apiKeyReputationRepository{client: client, sql: sqlDB}
}

// reputationPenaltyQuery scores both moderation sources in one pass.
//
// Weights encode how much each signal is worth as evidence, not how bad the
// content was. A prompt the guard rated critical, or content the moderation API
// blocked, is a deliberate act. A "flag"/observe hit is a fuzzy scanner being
// nervous, so it is worth little on its own but accumulates.
//
// The linear decay means a hit at the far edge of the window contributes almost
// nothing, which is what lets a key recover without a separate forgiveness job.
const reputationPenaltyQuery = `
WITH events AS (
    SELECT api_key_id,
           created_at,
           CASE
               WHEN decision = 'critical' THEN 25
               WHEN risk_level IN ('high', 'critical') THEN 12
               WHEN decision = 'flag' THEN 4
               ELSE 0
           END AS weight
    FROM prompt_audit_events
    WHERE api_key_id IS NOT NULL AND created_at >= $1

    UNION ALL

    SELECT api_key_id,
           created_at,
           CASE
               WHEN action IN ('block', 'keyword_block', 'cyber_policy') THEN 20
               ELSE 8
           END AS weight
    FROM content_moderation_logs
    WHERE api_key_id IS NOT NULL AND created_at >= $1 AND flagged
)
SELECT api_key_id,
       LEAST(100, ROUND(SUM(
           weight * GREATEST(0, 1 - EXTRACT(EPOCH FROM (NOW() - created_at)) / $2)
       )))::int AS penalty,
       COUNT(*) FILTER (WHERE weight >= 20)::int AS severe_hits,
       COUNT(*)::int AS total_hits,
       MAX(created_at) AS last_event_at
FROM events
WHERE weight > 0
GROUP BY api_key_id`

func (r *apiKeyReputationRepository) AggregatePenalties(ctx context.Context, since time.Time, window time.Duration) ([]service.ReputationPenalty, error) {
	windowSeconds := window.Seconds()
	if windowSeconds <= 0 {
		return nil, fmt.Errorf("reputation window must be positive")
	}

	rows, err := r.sql.QueryContext(ctx, reputationPenaltyQuery, since, windowSeconds)
	if err != nil {
		return nil, fmt.Errorf("query reputation penalties: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.ReputationPenalty, 0)
	for rows.Next() {
		var (
			item        service.ReputationPenalty
			lastEventAt sql.NullTime
		)
		if err := rows.Scan(&item.APIKeyID, &item.Penalty, &item.SevereHits, &item.TotalHits, &lastEventAt); err != nil {
			return nil, fmt.Errorf("scan reputation penalty: %w", err)
		}
		if lastEventAt.Valid {
			t := lastEventAt.Time
			item.LastEventAt = &t
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *apiKeyReputationRepository) Upsert(ctx context.Context, rep *service.APIKeyReputation) error {
	if rep == nil || rep.APIKeyID <= 0 {
		return fmt.Errorf("api key id is required")
	}
	// The sanction columns are intentionally not part of the conflict update:
	// scoring and sanctioning are separate decisions, and a re-score must never
	// silently clear a ban.
	return r.client.APIKeyReputation.Create().
		SetAPIKeyID(rep.APIKeyID).
		SetScore(rep.Score).
		SetSevereHits(rep.SevereHits).
		SetTotalHits(rep.TotalHits).
		SetNillableLastEventAt(rep.LastEventAt).
		SetScoredAt(rep.ScoredAt).
		OnConflictColumns(dbreputation.FieldAPIKeyID).
		UpdateScore().
		UpdateSevereHits().
		UpdateTotalHits().
		UpdateLastEventAt().
		UpdateScoredAt().
		SetUpdatedAt(timezone.Now()).
		Exec(ctx)
}

func reputationEntityToService(row *dbent.APIKeyReputation) *service.APIKeyReputation {
	if row == nil {
		return nil
	}
	return &service.APIKeyReputation{
		APIKeyID:       row.APIKeyID,
		Score:          row.Score,
		SevereHits:     row.SevereHits,
		TotalHits:      row.TotalHits,
		LastEventAt:    row.LastEventAt,
		ScoredAt:       row.ScoredAt,
		Sanction:       row.Sanction,
		SanctionedAt:   row.SanctionedAt,
		SanctionReason: row.SanctionReason,
	}
}

func (r *apiKeyReputationRepository) Get(ctx context.Context, apiKeyID int64) (*service.APIKeyReputation, error) {
	row, err := r.client.APIKeyReputation.Query().
		Where(dbreputation.APIKeyIDEQ(apiKeyID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return reputationEntityToService(row), nil
}

func (r *apiKeyReputationRepository) ListWorst(ctx context.Context, limit int) ([]service.APIKeyReputation, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.client.APIKeyReputation.Query().
		Where(dbreputation.ScoreLT(100)).
		Order(dbent.Asc(dbreputation.FieldScore), dbent.Desc(dbreputation.FieldTotalHits)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.APIKeyReputation, 0, len(rows))
	for _, row := range rows {
		out = append(out, *reputationEntityToService(row))
	}
	return out, nil
}

func (r *apiKeyReputationRepository) MarkSanctioned(ctx context.Context, apiKeyID int64, sanction, reason string) error {
	_, err := r.client.APIKeyReputation.Update().
		Where(dbreputation.APIKeyIDEQ(apiKeyID)).
		SetSanction(sanction).
		SetSanctionedAt(timezone.Now()).
		SetSanctionReason(reason).
		SetUpdatedAt(timezone.Now()).
		Save(ctx)
	return err
}

func (r *apiKeyReputationRepository) ClearSanction(ctx context.Context, apiKeyID int64) error {
	_, err := r.client.APIKeyReputation.Update().
		Where(dbreputation.APIKeyIDEQ(apiKeyID)).
		SetSanction(service.ReputationSanctionNone).
		ClearSanctionedAt().
		ClearSanctionReason().
		SetUpdatedAt(timezone.Now()).
		Save(ctx)
	return err
}

// ResetScoresNotIn restores keys whose last hit has aged out of the window.
// Sanctions are left in place: serving a ban is an operator decision to undo,
// not something that expires because the evidence got old.
func (r *apiKeyReputationRepository) ResetScoresNotIn(ctx context.Context, keepAPIKeyIDs []int64) error {
	query := `
		UPDATE api_key_reputation
		SET score = 100, severe_hits = 0, total_hits = 0, scored_at = NOW(), updated_at = NOW()
		WHERE score < 100`
	args := []any{}
	if len(keepAPIKeyIDs) > 0 {
		query += ` AND api_key_id <> ALL($1)`
		args = append(args, pq.Array(keepAPIKeyIDs))
	}
	if _, err := r.sql.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("reset aged-out reputation scores: %w", err)
	}
	return nil
}
