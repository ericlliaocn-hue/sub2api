package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

var upstreamRateVersionColumns = []string{
	"id", "account_id", "version_no", "rate_multiplier", "source",
	"effective_from", "effective_to", "observed_at", "snapshot",
	"change_reason", "created_by", "created_at",
}

func TestApplyUpstreamRateVersionChangeRejectsInvalidInputBeforeOpeningTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := newAccountRepositoryWithSQL(client, db, nil)

	_, err = repo.ApplyUpstreamRateVersionChange(context.Background(), service.UpstreamRateVersionChange{
		AccountID:      7,
		RateMultiplier: 1,
		Source:         "unsupported",
		ChangeReason:   domain.UpstreamRateChangeManualUpdate,
	})
	require.ErrorIs(t, err, service.ErrUpstreamRateVersionInvalidSource)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUpstreamRateVersionChangeCreatesInitialVersionAndOutbox(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	effectiveFrom := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)
	observedAt := effectiveFrom.Add(-time.Minute)
	mock.ExpectBegin()
	expectAccountRateLock(mock, 7)
	expectNoCurrentUpstreamRateVersion(mock, 7)
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(version_no\), 0\) \+ 1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"next_version"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("INSERT INTO account_upstream_rate_versions")+`.*RETURNING id, created_at`).
		WithArgs(int64(7), int64(1), 1.25, string(domain.UpstreamRateSourceManual), effectiveFrom, observedAt, sqlmock.AnyArg(), string(domain.UpstreamRateChangeManualUpdate), int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(41), effectiveFrom.Add(time.Second)))
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*`+regexp.QuoteMeta("active_upstream_rate_version_id")+`.*`+regexp.QuoteMeta("rate_multiplier")+`.*WHERE id = \$3`).
		WithArgs(int64(41), 1.25, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectAccountChangedOutbox(mock, 7)
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	createdBy := int64(99)
	result, err := repo.ApplyUpstreamRateVersionChange(context.Background(), service.UpstreamRateVersionChange{
		AccountID:      7,
		RateMultiplier: 1.25,
		Source:         domain.UpstreamRateSourceManual,
		EffectiveFrom:  effectiveFrom,
		ObservedAt:     &observedAt,
		Snapshot:       map[string]any{"mode": "manual"},
		ChangeReason:   domain.UpstreamRateChangeManualUpdate,
		CreatedBy:      &createdBy,
	})

	require.NoError(t, err)
	require.True(t, result.Changed)
	require.Equal(t, int64(41), result.Version.ID)
	require.Equal(t, int64(1), result.Version.VersionNo)
	require.Equal(t, domain.UpstreamRateSourceManual, result.Version.Source)
	require.Equal(t, domain.UpstreamRateChangeManualUpdate, result.Version.ChangeReason)
	require.Equal(t, map[string]any{"mode": "manual"}, result.Version.Snapshot)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUpstreamRateVersionChangeSameValueAndSourceRefreshesOnlyObservation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	effectiveFrom := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)
	observedAt := effectiveFrom.Add(time.Hour)
	mock.ExpectBegin()
	expectAccountRateLock(mock, 8)
	expectCurrentUpstreamRateVersion(mock, 8, 51, 3, 0.8, string(domain.UpstreamRateSourceUpstreamProbe), effectiveFrom, []byte(`{"old":true}`))
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE account_upstream_rate_versions")+`.*observed_at.*snapshot`).
		WithArgs(int64(51), observedAt, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	result, err := repo.ApplyUpstreamRateVersionChange(context.Background(), service.UpstreamRateVersionChange{
		AccountID:      8,
		RateMultiplier: 0.8,
		Source:         domain.UpstreamRateSourceUpstreamProbe,
		EffectiveFrom:  effectiveFrom.Add(2 * time.Hour),
		ObservedAt:     &observedAt,
		Snapshot:       map[string]any{"old": false, "new": true},
		ChangeReason:   domain.UpstreamRateChangeProbeChanged,
	})

	require.NoError(t, err)
	require.False(t, result.Changed)
	require.Equal(t, int64(51), result.Version.ID)
	require.Equal(t, map[string]any{"old": false, "new": true}, result.Version.Snapshot)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUpstreamRateVersionChangeSameValueSourceTakeoverCreatesVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	effectiveFrom := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)
	newEffectiveFrom := effectiveFrom.Add(time.Hour)
	mock.ExpectBegin()
	expectAccountRateLock(mock, 9)
	expectCurrentUpstreamRateVersion(mock, 9, 61, 4, 0.8, string(domain.UpstreamRateSourceManual), effectiveFrom, []byte(`{}`))
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE account_upstream_rate_versions")+`.*effective_to`).
		WithArgs(newEffectiveFrom, int64(61)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("INSERT INTO account_upstream_rate_versions")+`.*RETURNING id, created_at`).
		WithArgs(int64(9), int64(5), 0.8, string(domain.UpstreamRateSourceUpstreamProbe), newEffectiveFrom, nil, sqlmock.AnyArg(), string(domain.UpstreamRateChangeProbeTakeover), nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(62), newEffectiveFrom.Add(time.Second)))
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*active_upstream_rate_version_id`).
		WithArgs(int64(62), 0.8, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectAccountChangedOutbox(mock, 9)
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	result, err := repo.ApplyUpstreamRateVersionChange(context.Background(), service.UpstreamRateVersionChange{
		AccountID:      9,
		RateMultiplier: 0.8,
		Source:         domain.UpstreamRateSourceUpstreamProbe,
		EffectiveFrom:  newEffectiveFrom,
		ChangeReason:   domain.UpstreamRateChangeProbeChanged,
	})

	require.NoError(t, err)
	require.True(t, result.Changed)
	require.Equal(t, int64(5), result.Version.VersionNo)
	require.Equal(t, domain.UpstreamRateChangeProbeTakeover, result.Version.ChangeReason)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUpstreamRateVersionChangeRollsBackWhenOutboxFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	expectAccountRateLock(mock, 10)
	expectNoCurrentUpstreamRateVersion(mock, 10)
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(version_no\), 0\) \+ 1`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"next_version"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("INSERT INTO account_upstream_rate_versions") + `.*RETURNING id, created_at`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(71), time.Now()))
	mock.ExpectExec(`(?s)` + regexp.QuoteMeta("UPDATE accounts") + `.*active_upstream_rate_version_id`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).WillReturnError(errors.New("outbox failed"))
	mock.ExpectRollback()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	_, err = repo.ApplyUpstreamRateVersionChange(context.Background(), service.UpstreamRateVersionChange{
		AccountID:      10,
		RateMultiplier: 1,
		Source:         domain.UpstreamRateSourceManual,
		ChangeReason:   domain.UpstreamRateChangeManualUpdate,
	})

	require.EqualError(t, err, "outbox failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectAccountRateLock(mock sqlmock.Sqlmock, accountID int64) {
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("SELECT active_upstream_rate_version_id, rate_multiplier") + `.*` + regexp.QuoteMeta("FOR UPDATE")).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"active_upstream_rate_version_id", "rate_multiplier"}).AddRow(nil, 1.0))
}

func expectNoCurrentUpstreamRateVersion(mock sqlmock.Sqlmock, accountID int64) {
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("SELECT id, account_id, version_no, rate_multiplier, source") + `.*` + regexp.QuoteMeta("account_upstream_rate_versions") + `.*` + regexp.QuoteMeta("FOR UPDATE")).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows(upstreamRateVersionColumns))
}

func expectCurrentUpstreamRateVersion(mock sqlmock.Sqlmock, accountID, versionID, versionNo int64, rate float64, source string, effectiveFrom time.Time, snapshot []byte) {
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("SELECT id, account_id, version_no, rate_multiplier, source") + `.*` + regexp.QuoteMeta("account_upstream_rate_versions") + `.*` + regexp.QuoteMeta("FOR UPDATE")).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows(upstreamRateVersionColumns).AddRow(
			versionID, accountID, versionNo, rate, source, effectiveFrom, nil, nil, snapshot,
			string(domain.UpstreamRateChangeManualUpdate), nil, effectiveFrom,
		))
}

func expectAccountChangedOutbox(mock sqlmock.Sqlmock, accountID int64) {
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).
		WithArgs(service.SchedulerOutboxEventAccountChanged, accountID, nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

// Phase 4: probe → manual 同值生成 manual_takeover 版本（source 变化即使数值相同）。
func TestApplyUpstreamRateVersionChangeProbeToManualSameValueTakeover(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	effectiveFrom := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)
	newEffectiveFrom := effectiveFrom.Add(time.Hour)
	mock.ExpectBegin()
	expectAccountRateLock(mock, 15)
	expectCurrentUpstreamRateVersion(mock, 15, 81, 2, 1.5, string(domain.UpstreamRateSourceUpstreamProbe), effectiveFrom, []byte(`{}`))
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE account_upstream_rate_versions")+`.*effective_to`).
		WithArgs(newEffectiveFrom, int64(81)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("INSERT INTO account_upstream_rate_versions")+`.*RETURNING id, created_at`).
		WithArgs(int64(15), int64(3), 1.5, string(domain.UpstreamRateSourceManual), newEffectiveFrom, nil, sqlmock.AnyArg(), string(domain.UpstreamRateChangeManualTakeover), nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(82), newEffectiveFrom.Add(time.Second)))
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*active_upstream_rate_version_id`).
		WithArgs(int64(82), 1.5, int64(15)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectAccountChangedOutbox(mock, 15)
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	result, err := repo.ApplyUpstreamRateVersionChange(context.Background(), service.UpstreamRateVersionChange{
		AccountID:      15,
		RateMultiplier: 1.5,
		Source:         domain.UpstreamRateSourceManual,
		EffectiveFrom:  newEffectiveFrom,
		ChangeReason:   domain.UpstreamRateChangeManualUpdate,
	})

	require.NoError(t, err)
	require.True(t, result.Changed)
	require.Equal(t, domain.UpstreamRateChangeManualTakeover, result.Version.ChangeReason)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Phase 4: 探测快照完整保留原始 Data，observed_at 从探测 Data 解析。
func TestProbeSnapshotVersionPayloadKeepsFullDataAndObservedAt(t *testing.T) {
	observedAt := time.Date(2026, 8, 16, 8, 30, 0, 0, time.UTC)
	snapshot := &service.UpstreamBillingProbeSnapshot{
		Status:        service.UpstreamBillingProbeStatusOK,
		LastAttemptAt: observedAt.Add(time.Minute),
		Data: map[string]any{
			"resolved_rate_multiplier": 0.8,
			"observed_at":              observedAt.Format(time.RFC3339Nano),
			"nested":                   map[string]any{"peak": true},
		},
	}

	payload, err := probeSnapshotVersionPayload(snapshot)
	require.NoError(t, err)
	require.NotNil(t, payload)
	require.Equal(t, string(service.UpstreamBillingProbeStatusOK), payload["status"])
	data, ok := payload["data"].(map[string]any)
	require.True(t, ok, "full probe Data must be preserved in the version snapshot")
	require.Equal(t, 0.8, data["resolved_rate_multiplier"])
	require.Equal(t, map[string]any{"peak": true}, data["nested"])

	gotObservedAt := probeSnapshotObservedAt(snapshot)
	require.NotNil(t, gotObservedAt)
	require.Equal(t, observedAt, *gotObservedAt)
}
