package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// createAccountWithInitialRateVersion persists the account row and its first
// rate version (default 1.0, or the manual rate provided at creation) in the
// caller's transaction. The creation path enqueues its own scheduler outbox,
// so the version transition skips the outbox write.
func (r *accountRepository) createAccountWithInitialRateVersion(ctx context.Context, client *dbent.Client, account *service.Account) error {
	if err := createAccountRecord(ctx, client, account); err != nil {
		return err
	}
	change := initialUpstreamRateVersionChange(account)
	_, err := r.applyUpstreamRateVersionChangeInTx(ctx, client, change)
	return err
}

// initialUpstreamRateVersionChange builds the first rate version for a new
// account: default 1.0 when no manual rate was provided, otherwise manual.
func initialUpstreamRateVersionChange(account *service.Account) service.UpstreamRateVersionChange {
	change := service.UpstreamRateVersionChange{
		AccountID:      account.ID,
		RateMultiplier: 1.0,
		Source:         domain.UpstreamRateSourceDefault,
		ChangeReason:   domain.UpstreamRateChangeAccountCreated,
		SkipOutbox:     true,
	}
	if account.RateMultiplier != nil && *account.RateMultiplier >= 0 {
		change.RateMultiplier = *account.RateMultiplier
		change.Source = domain.UpstreamRateSourceManual
		change.ChangeReason = domain.UpstreamRateChangeManualUpdate
	}
	return change
}

// CreateWithUpstreamCostConfig atomically creates the account, its manual
// price versions (when configured), and its first rate version, then points
// the account at the current rate version. The scheduler outbox is written in
// the same transaction so the routing snapshot cannot observe a half-created
// account.
func (r *accountRepository) CreateWithUpstreamCostConfig(ctx context.Context, account *service.Account, costInputs []service.UpstreamCostVersionInput, createdBy int64) error {
	if account == nil {
		return service.ErrAccountNilInput
	}
	ctx, client, tx, ownsTx, err := beginAccountWriteTx(ctx, r.client)
	if err != nil {
		return err
	}
	if ownsTx {
		defer func() { _ = tx.Rollback() }()
	}

	if err := r.createAccountWithInitialRateVersion(ctx, client, account); err != nil {
		return err
	}
	if len(costInputs) > 0 {
		profiles, err := createUpstreamCostVersionRows(ctx, client, account.ID, costInputs, createdBy)
		if err != nil {
			return err
		}
		profilesJSON, err := json.Marshal(profiles)
		if err != nil {
			return fmt.Errorf("marshal upstream cost profiles: %w", err)
		}
		if _, err := client.ExecContext(ctx, `
			UPDATE accounts
			SET extra = COALESCE(extra, '{}'::jsonb) || jsonb_build_object('upstream_cost_profiles', $2::jsonb),
				updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`, account.ID, profilesJSON); err != nil {
			return err
		}
	}
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &account.ID, nil, buildSchedulerGroupPayload(account.GroupIDs)); err != nil {
		return err
	}
	if ownsTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// ReplaceUpstreamCostProfiles persists a new price version for every changed
// or new model and replaces the account's active profile set (and the enabled
// flag when provided) in one transaction. Unchanged models keep their current
// version. Models absent from costInputs are removed from the active set.
//
// When costInputs is nil the price set is left untouched — only the enabled
// flag is updated. A non-nil empty slice explicitly clears the active set.
func (r *accountRepository) ReplaceUpstreamCostProfiles(ctx context.Context, accountID int64, costInputs []service.UpstreamCostVersionInput, enabled *bool, createdBy int64) error {
	ctx, client, tx, ownsTx, err := beginAccountWriteTx(ctx, r.client)
	if err != nil {
		return err
	}
	if ownsTx {
		defer func() { _ = tx.Rollback() }()
	}

	if costInputs == nil {
		if enabled == nil {
			return nil
		}
		// 只更新开关，保留现有价格集合。
		encoded, err := json.Marshal(*enabled)
		if err != nil {
			return err
		}
		logger.L().Info("upstream_cost_config_toggle_only",
			zap.Int64("account_id", accountID),
			zap.Bool("enabled", *enabled),
		)
		if _, err := client.ExecContext(ctx, `
			UPDATE accounts
			SET extra = jsonb_set(COALESCE(extra, '{}'::jsonb), '{upstream_cost_enabled}', $2::jsonb, TRUE),
				updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`, accountID, encoded); err != nil {
			return err
		}
		if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			return err
		}
		if ownsTx {
			if err := tx.Commit(); err != nil {
				return err
			}
		}
		return nil
	}

	currentProfiles, err := lockUpstreamCostProfiles(ctx, client, accountID)
	if err != nil {
		return err
	}

	nextProfiles := make(map[string]any, len(costInputs))
	for i := range costInputs {
		costInputs[i].AccountID = accountID
		model := strings.ToLower(strings.TrimSpace(costInputs[i].Model))
		if existing, ok := currentProfiles[model]; ok && upstreamCostProfileUnchanged(existing, costInputs[i]) {
			nextProfiles[model] = existing
			continue
		}
		item, err := insertUpstreamCostVersionRow(ctx, client, costInputs[i], createdBy)
		if err != nil {
			return err
		}
		nextProfiles[model] = item.ExtraSnapshot()
	}

	logger.L().Info("upstream_cost_config_replaced",
		zap.Int64("account_id", accountID),
		zap.Int("profile_count", len(nextProfiles)),
		zap.Any("enabled", derefBool(enabled)),
	)

	profilesJSON, err := json.Marshal(nextProfiles)
	if err != nil {
		return fmt.Errorf("marshal upstream cost profiles: %w", err)
	}
	var enabledArg any
	if enabled != nil {
		encoded, err := json.Marshal(*enabled)
		if err != nil {
			return err
		}
		enabledArg = encoded
	}
	extraExpression := `jsonb_set(COALESCE(extra, '{}'::jsonb), '{upstream_cost_profiles}', $2::jsonb, TRUE)`
	extraArgs := []any{accountID, profilesJSON}
	if enabledArg != nil {
		extraExpression = `jsonb_set(` + extraExpression + `, '{upstream_cost_enabled}', $3::jsonb, TRUE)`
		extraArgs = append(extraArgs, enabledArg)
	}
	if _, err := client.ExecContext(ctx, `
		UPDATE accounts
		SET extra = `+extraExpression+`,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, extraArgs...); err != nil {
		return err
	}
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
		return err
	}
	if ownsTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// beginAccountWriteTx opens a transaction for a repository write, reusing a
// caller-owned ent transaction when present and tolerating a tx-bound client.
// Returns the derived context, the client to execute against, the owned tx (or
// nil), and whether the caller must commit.
func beginAccountWriteTx(ctx context.Context, client *dbent.Client) (context.Context, *dbent.Client, *dbent.Tx, bool, error) {
	if contextTx := dbent.TxFromContext(ctx); contextTx != nil {
		return ctx, contextTx.Client(), nil, false, nil
	}
	tx, err := client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return nil, nil, nil, false, err
	}
	if tx != nil {
		return dbent.NewTxContext(ctx, tx), tx.Client(), tx, true, nil
	}
	return ctx, client, nil, false, nil
}

// lockUpstreamCostProfiles reads the account's active manual profiles under a
// row lock so concurrent profile edits cannot interleave.
func lockUpstreamCostProfiles(ctx context.Context, client *dbent.Client, accountID int64) (map[string]any, error) {
	var rawProfiles []byte
	if err := scanSingleRow(ctx, client, `
		SELECT COALESCE(extra -> 'upstream_cost_profiles', '{}'::jsonb)
		FROM accounts
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, []any{accountID}, &rawProfiles); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAccountNotFound
		}
		return nil, err
	}
	profiles := make(map[string]any)
	if len(rawProfiles) > 0 {
		if err := json.Unmarshal(rawProfiles, &profiles); err != nil {
			return nil, fmt.Errorf("decode upstream cost profiles: %w", err)
		}
	}
	return profiles, nil
}

// upstreamCostProfileUnchanged reports whether the stored profile snapshot has
// the same billing content as the incoming input. Notes are intentionally
// ignored: only price-affecting fields generate a new version.
func upstreamCostProfileUnchanged(stored any, input service.UpstreamCostVersionInput) bool {
	current, ok := decodeUpstreamCostProfileSnapshot(stored)
	if !ok {
		return false
	}
	return current.ShortPrices == input.ShortPrices &&
		current.LongContextThreshold == input.LongContextThreshold &&
		current.LongPrices == input.LongPrices &&
		current.DeclaredMultiplier == input.DeclaredMultiplier &&
		current.BalanceUnitCost == input.BalanceUnitCost
}

func decodeUpstreamCostProfileSnapshot(stored any) (service.UpstreamCostVersion, bool) {
	data, err := json.Marshal(stored)
	if err != nil {
		return service.UpstreamCostVersion{}, false
	}
	var profile service.UpstreamCostVersion
	if err := json.Unmarshal(data, &profile); err != nil || profile.ID <= 0 {
		return service.UpstreamCostVersion{}, false
	}
	return profile, true
}

// createUpstreamCostVersionRows inserts one immutable price version per input
// and returns the accounts.extra profile map keyed by model.
func createUpstreamCostVersionRows(ctx context.Context, client *dbent.Client, accountID int64, inputs []service.UpstreamCostVersionInput, createdBy int64) (map[string]any, error) {
	profiles := make(map[string]any, len(inputs))
	for i := range inputs {
		inputs[i].AccountID = accountID
		item, err := insertUpstreamCostVersionRow(ctx, client, inputs[i], createdBy)
		if err != nil {
			return nil, err
		}
		profiles[item.Model] = item.ExtraSnapshot()
	}
	return profiles, nil
}

func insertUpstreamCostVersionRow(ctx context.Context, client *dbent.Client, input service.UpstreamCostVersionInput, createdBy int64) (*service.UpstreamCostVersion, error) {
	item := &service.UpstreamCostVersion{
		AccountID: input.AccountID, Model: input.Model,
		ShortPrices: input.ShortPrices, LongContextThreshold: input.LongContextThreshold, LongPrices: input.LongPrices,
		DeclaredMultiplier: input.DeclaredMultiplier, BalanceUnitCost: input.BalanceUnitCost, Notes: input.Notes,
	}
	if err := scanSingleRow(ctx, client, `
		INSERT INTO upstream_cost_versions (
			account_id, model,
			short_input_price, short_cache_read_price, short_cache_write_price, short_output_price,
			long_context_threshold, long_input_price, long_cache_read_price, long_cache_write_price, long_output_price,
			declared_multiplier, balance_unit_cost, notes, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NULLIF($15, 0)
		)
		RETURNING id, effective_from, created_at
	`, []any{
		input.AccountID, input.Model,
		input.ShortPrices.Input, input.ShortPrices.CacheRead, input.ShortPrices.CacheWrite, input.ShortPrices.Output,
		input.LongContextThreshold, input.LongPrices.Input, input.LongPrices.CacheRead, input.LongPrices.CacheWrite, input.LongPrices.Output,
		input.DeclaredMultiplier, input.BalanceUnitCost, input.Notes, createdBy,
	}, &item.ID, &item.EffectiveFrom, &item.CreatedAt); err != nil {
		return nil, err
	}
	if createdBy > 0 {
		item.CreatedBy = &createdBy
	}
	return item, nil
}

func derefBool(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}
