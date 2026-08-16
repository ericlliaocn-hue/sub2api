//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreateWithUpstreamCostConfig_AtomicallyCreatesAccountVersionsAndProfiles(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)

	account := &service.Account{
		Name:        "upstream-cost-create",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
		Extra:       map[string]any{},
	}
	input := service.UpstreamCostVersionInput{
		Model:              "gpt-5.6-luna",
		ShortPrices:        service.UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		LongPrices:         service.UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		DeclaredMultiplier: 0.035,
		BalanceUnitCost:    1,
	}
	require.NoError(t, repo.CreateWithUpstreamCostConfig(ctx, account, []service.UpstreamCostVersionInput{input}, 7))

	// 账号存在。
	got, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, account.ID, got.ID)

	// 初始倍率版本：default 1.0，version_no = 1。
	var rateVersionID, versionNo int64
	var source, reason string
	var rate float64
	require.NoError(t, scanSingleRow(ctx, tx.Client(), `
		SELECT id, version_no, rate_multiplier, source, change_reason
		FROM account_upstream_rate_versions
		WHERE account_id = $1
	`, []any{account.ID}, &rateVersionID, &versionNo, &rate, &source, &reason))
	require.Equal(t, int64(1), versionNo)
	require.Equal(t, 1.0, rate)
	require.Equal(t, string(domain.UpstreamRateSourceDefault), source)
	require.Equal(t, string(domain.UpstreamRateChangeAccountCreated), reason)

	// 当前版本指针指向初始版本。
	var activeID sql.NullInt64
	require.NoError(t, scanSingleRow(ctx, tx.Client(), "SELECT active_upstream_rate_version_id FROM accounts WHERE id = $1", []any{account.ID}, &activeID))
	require.True(t, activeID.Valid)
	require.Equal(t, rateVersionID, activeID.Int64)

	// 上游价格版本与 extra 快照。
	var priceVersionID int64
	require.NoError(t, scanSingleRow(ctx, tx.Client(), `
		SELECT id FROM upstream_cost_versions WHERE account_id = $1 AND model = 'gpt-5.6-luna'
	`, []any{account.ID}, &priceVersionID))
	require.Greater(t, priceVersionID, int64(0))

	got, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	profiles, ok := got.Extra[service.UpstreamCostProfilesExtraKey].(map[string]any)
	require.True(t, ok)
	require.Contains(t, profiles, "gpt-5.6-luna")
}

func TestCreateWithUpstreamCostConfig_ManualInitialRateVersion(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)

	manualRate := 2.5
	account := &service.Account{
		Name:           "upstream-cost-manual-rate",
		Platform:       service.PlatformOpenAI,
		Type:           service.AccountTypeAPIKey,
		Credentials:    map[string]any{"api_key": "sk-test"},
		RateMultiplier: &manualRate,
		Extra:          map[string]any{},
	}
	require.NoError(t, repo.CreateWithUpstreamCostConfig(ctx, account, nil, 0))

	var rate float64
	var source string
	require.NoError(t, scanSingleRow(ctx, tx.Client(), `
		SELECT rate_multiplier, source FROM account_upstream_rate_versions WHERE account_id = $1
	`, []any{account.ID}, &rate, &source))
	require.Equal(t, manualRate, rate)
	require.Equal(t, string(domain.UpstreamRateSourceManual), source)
}

func TestReplaceUpstreamCostProfiles_ChangedCreatesNewVersionUnchangedKeeps(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)

	account := &service.Account{
		Name:        "upstream-cost-replace",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
		Extra:       map[string]any{},
	}
	input := service.UpstreamCostVersionInput{
		Model:              "gpt-5.6-luna",
		ShortPrices:        service.UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		LongPrices:         service.UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		DeclaredMultiplier: 0.035,
		BalanceUnitCost:    1,
	}
	require.NoError(t, repo.CreateWithUpstreamCostConfig(ctx, account, []service.UpstreamCostVersionInput{input}, 0))

	// 相同内容：不产生新价格版本。
	require.NoError(t, repo.ReplaceUpstreamCostProfiles(ctx, account.ID, []service.UpstreamCostVersionInput{input}, nil, 0))
	var count int
	require.NoError(t, scanSingleRow(ctx, tx.Client(), "SELECT COUNT(*) FROM upstream_cost_versions WHERE account_id = $1", []any{account.ID}, &count))
	require.Equal(t, 1, count)

	// 价格变化：产生新价格版本，旧版本保留（append-only）。
	changed := input
	changed.ShortPrices = service.UpstreamCostPrices{Input: 2, CacheRead: 0.2, CacheWrite: 2, Output: 12}
	changed.LongPrices = changed.ShortPrices
	require.NoError(t, repo.ReplaceUpstreamCostProfiles(ctx, account.ID, []service.UpstreamCostVersionInput{changed}, nil, 0))
	require.NoError(t, scanSingleRow(ctx, tx.Client(), "SELECT COUNT(*) FROM upstream_cost_versions WHERE account_id = $1", []any{account.ID}, &count))
	require.Equal(t, 2, count)

	// extra 快照指向最新版本。
	got, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	raw := got.Extra[service.UpstreamCostProfilesExtraKey].(map[string]any)["gpt-5.6-luna"]
	encoded, err := json.Marshal(raw)
	require.NoError(t, err)
	var snapshot map[string]any
	require.NoError(t, json.Unmarshal(encoded, &snapshot))
	require.Equal(t, float64(2), snapshot["short_prices"].(map[string]any)["input"])
}
