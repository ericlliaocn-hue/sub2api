package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func lunaCostVersion(id int64, model string) service.UpstreamCostVersion {
	return service.UpstreamCostVersion{
		ID: id, Model: model,
		ShortPrices:          service.UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		LongContextThreshold: 0,
		LongPrices:           service.UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		DeclaredMultiplier:   0.035,
		BalanceUnitCost:      1,
		EffectiveFrom:        time.Now(),
	}
}

func lunaCostInput() service.UpstreamCostVersionInput {
	return service.UpstreamCostVersionInput{
		Model:              "gpt-5.6-luna",
		ShortPrices:        service.UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		LongPrices:         service.UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		DeclaredMultiplier: 0.035,
		BalanceUnitCost:    1,
	}
}

func TestReplaceUpstreamCostProfilesCreatesVersionForChangedProfile(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	existing := lunaCostVersion(1, "gpt-5.6-luna")
	existingJSON, err := json.Marshal(map[string]any{"gpt-5.6-luna": existing.ExtraSnapshot()})
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("SELECT COALESCE(extra -> 'upstream_cost_profiles', '{}'::jsonb)") + `.*FOR UPDATE`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"profiles"}).AddRow(existingJSON))
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("INSERT INTO upstream_cost_versions")+`.*RETURNING id, effective_from, created_at`).
		WithArgs(int64(9), "gpt-5.6-luna", 1.5, 0.2, 1.5, 8.0, 0, 1.5, 0.2, 1.5, 8.0, 0.035, 1.0, "", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "effective_from", "created_at"}).AddRow(int64(2), time.Now(), time.Now()))
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*upstream_cost_profiles`).
		WithArgs(int64(9), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectAccountChangedOutbox(mock, 9)
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	input := lunaCostInput()
	input.ShortPrices = service.UpstreamCostPrices{Input: 1.5, CacheRead: 0.2, CacheWrite: 1.5, Output: 8}
	input.LongPrices = service.UpstreamCostPrices{Input: 1.5, CacheRead: 0.2, CacheWrite: 1.5, Output: 8}
	err = repo.ReplaceUpstreamCostProfiles(context.Background(), 9, []service.UpstreamCostVersionInput{input}, nil, 7)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReplaceUpstreamCostProfilesKeepsVersionWhenUnchanged(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	existing := lunaCostVersion(1, "gpt-5.6-luna")
	existingJSON, err := json.Marshal(map[string]any{"gpt-5.6-luna": existing.ExtraSnapshot()})
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("SELECT COALESCE(extra -> 'upstream_cost_profiles', '{}'::jsonb)") + `.*FOR UPDATE`).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"profiles"}).AddRow(existingJSON))
	// Identical billing content: no INSERT of a new price version.
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*upstream_cost_profiles`).
		WithArgs(int64(10), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectAccountChangedOutbox(mock, 10)
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	err = repo.ReplaceUpstreamCostProfiles(context.Background(), 10, []service.UpstreamCostVersionInput{lunaCostInput()}, nil, 0)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUpstreamRateVersionChangeSkipOutboxOnCreatePath(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	expectAccountRateLock(mock, 12)
	expectNoCurrentUpstreamRateVersion(mock, 12)
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(version_no\), 0\) \+ 1`).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"next_version"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("INSERT INTO account_upstream_rate_versions")+`.*RETURNING id, created_at`).
		WithArgs(int64(12), int64(1), 1.0, "default", sqlmock.AnyArg(), nil, sqlmock.AnyArg(), "account_created", nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(91), time.Now()))
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*active_upstream_rate_version_id`).
		WithArgs(int64(91), 1.0, int64(12)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// SkipOutbox: no scheduler outbox write on the create path.
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	change := initialUpstreamRateVersionChange(&service.Account{ID: 12})
	require.Equal(t, "default", string(change.Source))
	require.Equal(t, "account_created", string(change.ChangeReason))
	require.True(t, change.SkipOutbox)

	result, err := repo.ApplyUpstreamRateVersionChange(context.Background(), change)
	require.NoError(t, err)
	require.True(t, result.Changed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// 修复：只切换 upstream_cost_enabled 开关时不得清空已有价格配置。
func TestReplaceUpstreamCostProfiles_ToggleOnlyKeepsProfiles(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	enabled := false
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*upstream_cost_enabled`).
		WithArgs(int64(21), []byte("false")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectAccountChangedOutbox(mock, 21)
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	// costInputs == nil：只更新 enabled，不清空 profiles。
	err = repo.ReplaceUpstreamCostProfiles(context.Background(), 21, nil, &enabled, 0)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
