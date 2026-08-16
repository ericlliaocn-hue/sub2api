package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const upstreamRateMultiplierScale = 1e10

// ApplyUpstreamRateVersionChange is the single write path for account-level
// upstream multiplier transitions. It serializes on the account row, so a
// manual edit and a probe cannot create overlapping current versions.
func (r *accountRepository) ApplyUpstreamRateVersionChange(
	ctx context.Context,
	change service.UpstreamRateVersionChange,
) (*service.UpstreamRateVersionChangeResult, error) {
	if err := change.Validate(); err != nil {
		return nil, err
	}
	change.RateMultiplier = normalizeUpstreamRateMultiplier(change.RateMultiplier)
	if change.EffectiveFrom.IsZero() {
		change.EffectiveFrom = time.Now().UTC()
	} else {
		change.EffectiveFrom = change.EffectiveFrom.UTC()
	}

	if tx := dbent.TxFromContext(ctx); tx != nil {
		return r.applyUpstreamRateVersionChangeInTx(ctx, tx.Client(), change)
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := r.applyUpstreamRateVersionChangeInTx(dbent.NewTxContext(ctx, tx), tx.Client(), change)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	// The outbox is durable; this direct refresh only reduces visibility
	// latency for the current process when the method owns the transaction.
	r.syncSchedulerAccountSnapshot(ctx, change.AccountID)
	return result, nil
}

func (r *accountRepository) applyUpstreamRateVersionChangeInTx(
	ctx context.Context,
	client *dbent.Client,
	change service.UpstreamRateVersionChange,
) (*service.UpstreamRateVersionChangeResult, error) {
	if change.EffectiveFrom.IsZero() {
		change.EffectiveFrom = time.Now().UTC()
	} else {
		change.EffectiveFrom = change.EffectiveFrom.UTC()
	}
	var (
		activeVersionID sql.NullInt64
		compatRate      float64
	)
	if err := scanSingleRow(ctx, client, `
		SELECT active_upstream_rate_version_id, rate_multiplier
		FROM accounts
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, []any{change.AccountID}, &activeVersionID, &compatRate); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAccountNotFound
		}
		return nil, err
	}

	current, err := lockCurrentUpstreamRateVersion(ctx, client, change.AccountID)
	if err != nil {
		return nil, err
	}

	if current != nil && current.RateMultiplier == change.RateMultiplier && current.Source == change.Source {
		if err := refreshUpstreamRateVersionObservation(ctx, client, current.ID, change); err != nil {
			return nil, err
		}
		if change.ObservedAt != nil {
			observedAt := change.ObservedAt.UTC()
			current.ObservedAt = &observedAt
		}
		if change.Snapshot != nil {
			current.Snapshot = copyJSONMap(change.Snapshot)
		}
		return &service.UpstreamRateVersionChangeResult{Version: current, Changed: false}, nil
	}

	if current != nil && !change.EffectiveFrom.After(current.EffectiveFrom) {
		return nil, service.ErrUpstreamRateVersionEffectiveFromOrder
	}
	changeReason := resolveUpstreamRateChangeReason(change, current)

	nextVersionNo := int64(1)
	if current != nil {
		nextVersionNo = current.VersionNo + 1
	} else if err := scanSingleRow(ctx, client, `
		SELECT COALESCE(MAX(version_no), 0) + 1
		FROM account_upstream_rate_versions
		WHERE account_id = $1
	`, []any{change.AccountID}, &nextVersionNo); err != nil {
		return nil, err
	}

	if current != nil {
		result, err := client.ExecContext(ctx, `
			UPDATE account_upstream_rate_versions
			SET effective_to = $1
			WHERE id = $2 AND effective_to IS NULL
		`, change.EffectiveFrom, current.ID)
		if err != nil {
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected != 1 {
			return nil, fmt.Errorf("close current upstream rate version %d: affected %d rows", current.ID, affected)
		}
	}

	snapshot := change.Snapshot
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal upstream rate snapshot: %w", err)
	}
	var versionID int64
	var createdAt time.Time
	if err := scanSingleRow(ctx, client, `
		INSERT INTO account_upstream_rate_versions (
			account_id, version_no, rate_multiplier, source,
			effective_from, observed_at, snapshot, change_reason, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9)
		RETURNING id, created_at
	`, []any{change.AccountID, nextVersionNo, change.RateMultiplier, string(change.Source),
		change.EffectiveFrom, nullableTime(change.ObservedAt), snapshotJSON,
		string(changeReason), nullableInt64(change.CreatedBy)}, &versionID, &createdAt); err != nil {
		return nil, err
	}

	accountResult, err := client.ExecContext(ctx, `
		UPDATE accounts
		SET active_upstream_rate_version_id = $1,
			rate_multiplier = $2,
			updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL
	`, versionID, change.RateMultiplier, change.AccountID)
	if err != nil {
		return nil, err
	}
	accountAffected, err := accountResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	if accountAffected != 1 {
		return nil, service.ErrAccountNotFound
	}

	if !change.SkipOutbox {
		if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &change.AccountID, nil, change.OutboxPayload); err != nil {
			return nil, err
		}
	}

	return &service.UpstreamRateVersionChangeResult{
		Version: &service.UpstreamRateVersion{
			ID:             versionID,
			AccountID:      change.AccountID,
			VersionNo:      nextVersionNo,
			RateMultiplier: change.RateMultiplier,
			Source:         change.Source,
			EffectiveFrom:  change.EffectiveFrom,
			ObservedAt:     cloneTime(change.ObservedAt),
			Snapshot:       copyJSONMap(snapshot),
			ChangeReason:   changeReason,
			CreatedBy:      cloneInt64(change.CreatedBy),
			CreatedAt:      createdAt,
		},
		Changed: true,
	}, nil
}

func lockCurrentUpstreamRateVersion(ctx context.Context, client *dbent.Client, accountID int64) (*service.UpstreamRateVersion, error) {
	rows, err := client.QueryContext(ctx, `
		SELECT id, account_id, version_no, rate_multiplier, source,
			effective_from, effective_to, observed_at, snapshot,
			change_reason, created_by, created_at
		FROM account_upstream_rate_versions
		WHERE account_id = $1 AND effective_to IS NULL
		ORDER BY version_no DESC, id DESC
		LIMIT 1
		FOR UPDATE
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return scanUpstreamRateVersionRow(rows)
}

func refreshUpstreamRateVersionObservation(
	ctx context.Context,
	client *dbent.Client,
	versionID int64,
	change service.UpstreamRateVersionChange,
) error {
	var snapshotArg any
	if change.Snapshot != nil {
		encoded, err := json.Marshal(change.Snapshot)
		if err != nil {
			return fmt.Errorf("marshal upstream rate observation: %w", err)
		}
		snapshotArg = encoded
	}
	result, err := client.ExecContext(ctx, `
		UPDATE account_upstream_rate_versions
		SET observed_at = COALESCE($2::timestamptz, observed_at),
			snapshot = COALESCE($3::jsonb, snapshot)
		WHERE id = $1 AND effective_to IS NULL
	`, versionID, nullableTime(change.ObservedAt), snapshotArg)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("refresh upstream rate version %d: affected %d rows", versionID, affected)
	}
	return nil
}

func normalizeUpstreamRateMultiplier(value float64) float64 {
	return math.Round(value*upstreamRateMultiplierScale) / upstreamRateMultiplierScale
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func scanNullableInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func decodeUpstreamRateSnapshot(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("decode upstream rate snapshot: %w", err)
	}
	if snapshot == nil {
		return map[string]any{}, nil
	}
	return snapshot, nil
}
