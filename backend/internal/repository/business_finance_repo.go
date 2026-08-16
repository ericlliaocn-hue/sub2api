package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type businessFinanceRepository struct {
	db *sql.DB
}

func NewBusinessFinanceRepository(db *sql.DB) service.BusinessFinanceRepository {
	return &businessFinanceRepository{db: db}
}

const businessCostConfigColumns = `
	id, code, name, category, amount, currency, exchange_rate_to_billing_unit, allocation_method, frequency, scope,
	effective_from, effective_to, enabled, notes, created_by, created_at, updated_at`

func (r *businessFinanceRepository) ListCostConfigs(ctx context.Context) ([]service.CostConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+businessCostConfigColumns+` FROM business_cost_configs ORDER BY effective_from DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]service.CostConfig, 0)
	for rows.Next() {
		item, err := scanCostConfig(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *businessFinanceRepository) CreateCostConfig(ctx context.Context, input service.CostConfigInput, createdBy int64) (*service.CostConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO business_cost_configs
			(code, name, category, amount, currency, exchange_rate_to_billing_unit, allocation_method, frequency, scope, effective_from, effective_to, enabled, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NULLIF($14, 0))
		RETURNING `+businessCostConfigColumns,
		input.Code, input.Name, input.Category, input.Amount, input.Currency, input.ExchangeRate, input.AllocationMethod,
		input.Frequency, jsonValue(input.Scope), input.EffectiveFrom, input.EffectiveTo, input.Enabled, input.Notes, createdBy,
	)
	return scanCostConfig(row)
}

func (r *businessFinanceRepository) UpdateCostConfig(ctx context.Context, id int64, input service.CostConfigInput) (*service.CostConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE business_cost_configs
		SET code = $2, name = $3, category = $4, amount = $5, currency = $6,
			exchange_rate_to_billing_unit = $7, allocation_method = $8, frequency = $9, scope = $10, effective_from = $11, effective_to = $12,
			enabled = $13, notes = $14, updated_at = NOW()
		WHERE id = $1
		RETURNING `+businessCostConfigColumns,
		id, input.Code, input.Name, input.Category, input.Amount, input.Currency, input.ExchangeRate, input.AllocationMethod,
		input.Frequency, jsonValue(input.Scope), input.EffectiveFrom, input.EffectiveTo, input.Enabled, input.Notes,
	)
	return scanCostConfig(row)
}

func (r *businessFinanceRepository) DisableCostConfig(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE business_cost_configs SET enabled = FALSE, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *businessFinanceRepository) DeleteCostConfig(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM business_cost_configs WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *businessFinanceRepository) ListExpenses(ctx context.Context, filter service.ExpenseListFilter) ([]service.ExpenseEntry, int, error) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 5)
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if filter.Category != "" {
		add("category = $%d", filter.Category)
	}
	if filter.Status != "" {
		add("status = $%d", filter.Status)
	}
	if filter.StartTime != nil {
		add("COALESCE(period_end, occurred_at + interval '1 microsecond') > $%d", *filter.StartTime)
	}
	if filter.EndTime != nil {
		add("COALESCE(period_start, occurred_at) < $%d", *filter.EndTime)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM business_expenses WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append([]any{}, args...)
	limitPosition := len(listArgs) + 1
	offsetPosition := len(listArgs) + 2
	listArgs = append(listArgs, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, category, name, amount, currency, occurred_at, period_start, period_end,
		       exchange_rate_to_billing_unit, allocation_method, scope, status, notes, created_by, created_at, updated_at
		FROM business_expenses
		WHERE `+whereSQL+`
		ORDER BY occurred_at DESC, id DESC
		LIMIT $`+fmt.Sprint(limitPosition)+` OFFSET $`+fmt.Sprint(offsetPosition), listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]service.ExpenseEntry, 0)
	for rows.Next() {
		item, err := scanExpense(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *businessFinanceRepository) CreateExpense(ctx context.Context, input service.ExpenseInput, createdBy int64) (*service.ExpenseEntry, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO business_expenses
			(category, name, amount, currency, exchange_rate_to_billing_unit, occurred_at, period_start, period_end, allocation_method, scope, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULLIF($12, 0))
		RETURNING id, category, name, amount, currency, occurred_at, period_start, period_end,
		          exchange_rate_to_billing_unit, allocation_method, scope, status, notes, created_by, created_at, updated_at`,
		input.Category, input.Name, input.Amount, input.Currency, input.ExchangeRate, input.OccurredAt,
		input.PeriodStart, input.PeriodEnd, input.AllocationMethod, jsonValue(input.Scope), input.Notes, createdBy,
	)
	return scanExpense(row)
}

func (r *businessFinanceRepository) UpdateExpense(ctx context.Context, id int64, input service.ExpenseInput) (*service.ExpenseEntry, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE business_expenses
		SET category = $2, name = $3, amount = $4, currency = $5, exchange_rate_to_billing_unit = $6, occurred_at = $7,
			period_start = $8, period_end = $9, allocation_method = $10, scope = $11,
			notes = $12, updated_at = NOW()
		WHERE id = $1 AND status = 'active'
		RETURNING id, category, name, amount, currency, occurred_at, period_start, period_end,
		          exchange_rate_to_billing_unit, allocation_method, scope, status, notes, created_by, created_at, updated_at`,
		id, input.Category, input.Name, input.Amount, input.Currency, input.ExchangeRate, input.OccurredAt,
		input.PeriodStart, input.PeriodEnd, input.AllocationMethod, jsonValue(input.Scope), input.Notes,
	)
	return scanExpense(row)
}

func (r *businessFinanceRepository) VoidExpense(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE business_expenses SET status = 'void', updated_at = NOW() WHERE id = $1 AND status = 'active'`, id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

const upstreamCostVersionColumns = `
	v.id, v.account_id, a.name, v.model,
	v.short_input_price, v.short_cache_read_price, v.short_cache_write_price, v.short_output_price,
	v.long_context_threshold, v.long_input_price, v.long_cache_read_price, v.long_cache_write_price, v.long_output_price,
	v.declared_multiplier, v.balance_unit_cost, v.notes, v.effective_from, v.created_by, v.created_at`

func (r *businessFinanceRepository) ListUpstreamCostVersions(ctx context.Context, accountID int64, model string) ([]service.UpstreamCostVersion, error) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 2)
	if accountID > 0 {
		args = append(args, accountID)
		where = append(where, fmt.Sprintf("v.account_id = $%d", len(args)))
	}
	if model != "" {
		args = append(args, model)
		where = append(where, fmt.Sprintf("v.model = $%d", len(args)))
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+upstreamCostVersionColumns+`
		FROM upstream_cost_versions v
		JOIN accounts a ON a.id = v.account_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY v.effective_from DESC, v.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]service.UpstreamCostVersion, 0)
	for rows.Next() {
		item, err := scanUpstreamCostVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *businessFinanceRepository) CreateUpstreamCostVersion(ctx context.Context, input service.UpstreamCostVersionInput, createdBy int64) (*service.UpstreamCostVersion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var accountName, platform string
	if err := tx.QueryRowContext(ctx, `
		SELECT name, platform FROM accounts
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE`, input.AccountID).Scan(&accountName, &platform); err != nil {
		return nil, err
	}
	if platform != service.PlatformOpenAI {
		return nil, fmt.Errorf("manual upstream cost profile requires an OpenAI account")
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO upstream_cost_versions (
			account_id, model,
			short_input_price, short_cache_read_price, short_cache_write_price, short_output_price,
			long_context_threshold, long_input_price, long_cache_read_price, long_cache_write_price, long_output_price,
			declared_multiplier, balance_unit_cost, notes, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NULLIF($15, 0)
		)
		RETURNING id, effective_from, created_at`,
		input.AccountID, input.Model,
		input.ShortPrices.Input, input.ShortPrices.CacheRead, input.ShortPrices.CacheWrite, input.ShortPrices.Output,
		input.LongContextThreshold, input.LongPrices.Input, input.LongPrices.CacheRead, input.LongPrices.CacheWrite, input.LongPrices.Output,
		input.DeclaredMultiplier, input.BalanceUnitCost, input.Notes, createdBy,
	)
	item := &service.UpstreamCostVersion{
		AccountID: input.AccountID, AccountName: accountName, Model: input.Model,
		ShortPrices: input.ShortPrices, LongContextThreshold: input.LongContextThreshold, LongPrices: input.LongPrices,
		DeclaredMultiplier: input.DeclaredMultiplier, BalanceUnitCost: input.BalanceUnitCost, Notes: input.Notes,
	}
	if err := row.Scan(&item.ID, &item.EffectiveFrom, &item.CreatedAt); err != nil {
		return nil, err
	}
	if createdBy > 0 {
		item.CreatedBy = &createdBy
	}
	profileJSON, err := json.Marshal(item.ExtraSnapshot())
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts
		SET extra = jsonb_set(
			COALESCE(extra, '{}'::jsonb),
			'{upstream_cost_profiles}',
			COALESCE(extra->'upstream_cost_profiles', '{}'::jsonb) || jsonb_build_object($2::text, $3::jsonb),
			TRUE
		), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`, input.AccountID, input.Model, profileJSON); err != nil {
		return nil, err
	}
	if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &input.AccountID, nil, nil); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

type financeRowScanner interface {
	Scan(dest ...any) error
}

func scanCostConfig(row financeRowScanner) (*service.CostConfig, error) {
	var item service.CostConfig
	var scopeRaw []byte
	var createdBy sql.NullInt64
	if err := row.Scan(
		&item.ID, &item.Code, &item.Name, &item.Category, &item.Amount, &item.Currency,
		&item.ExchangeRate, &item.AllocationMethod, &item.Frequency, &scopeRaw, &item.EffectiveFrom, &item.EffectiveTo,
		&item.Enabled, &item.Notes, &createdBy, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Scope = decodeJSONMap(scopeRaw)
	if createdBy.Valid {
		item.CreatedBy = &createdBy.Int64
	}
	return &item, nil
}

func scanExpense(row financeRowScanner) (*service.ExpenseEntry, error) {
	var item service.ExpenseEntry
	var scopeRaw []byte
	var createdBy sql.NullInt64
	if err := row.Scan(
		&item.ID, &item.Category, &item.Name, &item.Amount, &item.Currency, &item.OccurredAt,
		&item.PeriodStart, &item.PeriodEnd, &item.ExchangeRate, &item.AllocationMethod, &scopeRaw, &item.Status,
		&item.Notes, &createdBy, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Scope = decodeJSONMap(scopeRaw)
	if createdBy.Valid {
		item.CreatedBy = &createdBy.Int64
	}
	return &item, nil
}

func scanUpstreamCostVersion(row financeRowScanner) (*service.UpstreamCostVersion, error) {
	var item service.UpstreamCostVersion
	var createdBy sql.NullInt64
	if err := row.Scan(
		&item.ID, &item.AccountID, &item.AccountName, &item.Model,
		&item.ShortPrices.Input, &item.ShortPrices.CacheRead, &item.ShortPrices.CacheWrite, &item.ShortPrices.Output,
		&item.LongContextThreshold, &item.LongPrices.Input, &item.LongPrices.CacheRead, &item.LongPrices.CacheWrite, &item.LongPrices.Output,
		&item.DeclaredMultiplier, &item.BalanceUnitCost, &item.Notes, &item.EffectiveFrom, &createdBy, &item.CreatedAt,
	); err != nil {
		return nil, err
	}
	if createdBy.Valid {
		item.CreatedBy = &createdBy.Int64
	}
	return &item, nil
}

func jsonValue(value map[string]any) []byte {
	if value == nil {
		return []byte(`{}`)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func decodeJSONMap(raw []byte) map[string]any {
	value := map[string]any{}
	if len(raw) == 0 {
		return value
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{}
	}
	return value
}
