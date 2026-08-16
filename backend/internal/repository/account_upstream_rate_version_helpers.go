package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func resolveUpstreamRateChangeReason(
	change service.UpstreamRateVersionChange,
	current *service.UpstreamRateVersion,
) domain.UpstreamRateVersionChangeReason {
	if current != nil && current.Source != change.Source {
		switch change.Source {
		case domain.UpstreamRateSourceManual:
			return domain.UpstreamRateChangeManualTakeover
		case domain.UpstreamRateSourceUpstreamProbe:
			return domain.UpstreamRateChangeProbeTakeover
		}
	}
	return change.ChangeReason
}

func probeSnapshotVersionPayload(snapshot *service.UpstreamBillingProbeSnapshot) (map[string]any, error) {
	if snapshot == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal upstream billing probe snapshot: %w", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, fmt.Errorf("decode upstream billing probe snapshot: %w", err)
	}
	return payload, nil
}

func probeSnapshotObservedAt(snapshot *service.UpstreamBillingProbeSnapshot) *time.Time {
	if snapshot == nil || snapshot.Data == nil {
		return nil
	}
	value, ok := snapshot.Data["observed_at"].(string)
	if !ok || value == "" {
		return nil
	}
	observedAt, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	observedAt = observedAt.UTC()
	return &observedAt
}

// loadActiveUpstreamRateVersions returns the current (effective_to IS NULL)
// rate version per account id. Accounts without a version are absent.
func loadActiveUpstreamRateVersions(ctx context.Context, client *dbent.Client, accountIDs []int64) (map[int64]*service.UpstreamRateVersion, error) {
	result := make(map[int64]*service.UpstreamRateVersion)
	accountIDs = uniquePositiveInt64s(accountIDs)
	if len(accountIDs) == 0 {
		return result, nil
	}
	for start := 0; start < len(accountIDs); start += postgresParameterBatchSize {
		end := start + postgresParameterBatchSize
		if end > len(accountIDs) {
			end = len(accountIDs)
		}
		rows, err := client.QueryContext(ctx, `
			SELECT id, account_id, version_no, rate_multiplier, source,
				effective_from, effective_to, observed_at, snapshot,
				change_reason, created_by, created_at
			FROM account_upstream_rate_versions
			WHERE account_id = ANY($1) AND effective_to IS NULL
		`, pq.Array(accountIDs[start:end]))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			version, err := scanUpstreamRateVersionRow(rows)
			if err != nil {
				_ = rows.Close()
				return nil, err
			}
			result[version.AccountID] = version
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}
	return result, nil
}

// scanUpstreamRateVersionRow scans one row of the shared version column shape
// (see lockCurrentUpstreamRateVersion for the SELECT list).
func scanUpstreamRateVersionRow(row interface{ Scan(...any) error }) (*service.UpstreamRateVersion, error) {
	var (
		version      service.UpstreamRateVersion
		source       string
		effectiveTo  sql.NullTime
		observedAt   sql.NullTime
		snapshotJSON []byte
		createdBy    sql.NullInt64
		createdAt    time.Time
	)
	if err := row.Scan(
		&version.ID, &version.AccountID, &version.VersionNo, &version.RateMultiplier, &source,
		&version.EffectiveFrom, &effectiveTo, &observedAt, &snapshotJSON,
		&version.ChangeReason, &createdBy, &createdAt,
	); err != nil {
		return nil, err
	}
	version.Source = domain.UpstreamRateVersionSource(source)
	version.EffectiveTo = nullTimePtr(effectiveTo)
	version.ObservedAt = nullTimePtr(observedAt)
	version.CreatedBy = scanNullableInt64(createdBy)
	version.CreatedAt = createdAt
	snapshot, err := decodeUpstreamRateSnapshot(snapshotJSON)
	if err != nil {
		return nil, err
	}
	version.Snapshot = snapshot
	version.RateMultiplier = normalizeUpstreamRateMultiplier(version.RateMultiplier)
	return &version, nil
}
