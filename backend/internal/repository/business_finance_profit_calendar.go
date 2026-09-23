package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *businessFinanceRepository) GetProfitCalendar(ctx context.Context, start, end time.Time) (*service.ProfitCalendar, error) {
	accounts, err := r.listProfitCalendarAccounts(ctx, start, end)
	if err != nil {
		return nil, err
	}
	expenses, err := r.listProfitCalendarExpenses(ctx)
	if err != nil {
		return nil, err
	}

	inRange := make(map[int64]struct{}, len(accounts))
	for _, account := range accounts {
		inRange[account.ID] = struct{}{}
	}
	extraIDs := make([]int64, 0)
	seenExtra := make(map[int64]struct{})
	for _, expense := range expenses {
		ids := service.FinanceScopeAccountIDs(expense.Scope)
		touches := false
		for _, id := range ids {
			if _, ok := inRange[id]; ok {
				touches = true
				break
			}
		}
		if !touches {
			continue
		}
		for _, id := range ids {
			if _, ok := inRange[id]; ok {
				continue
			}
			if _, ok := seenExtra[id]; ok {
				continue
			}
			seenExtra[id] = struct{}{}
			extraIDs = append(extraIDs, id)
		}
	}
	if len(extraIDs) > 0 {
		extras, err := r.listProfitCalendarAccountsByIDs(ctx, extraIDs)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, extras...)
	}

	usageIDs := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		usageIDs = append(usageIDs, account.ID)
	}
	usage, err := r.listProfitCalendarUsage(ctx, usageIDs)
	if err != nil {
		return nil, err
	}
	return service.BuildProfitCalendar(accounts, expenses, usage, start, end, service.ProfitCalendarLocation()), nil
}

func (r *businessFinanceRepository) listProfitCalendarAccounts(ctx context.Context, start, end time.Time) ([]service.ProfitCalendarAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, status, schedulable, type, created_at
		FROM accounts
		WHERE deleted_at IS NULL
		  AND created_at >= $1 AND created_at < $2
		ORDER BY created_at, id`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProfitCalendarAccounts(rows)
}

func (r *businessFinanceRepository) listProfitCalendarAccountsByIDs(ctx context.Context, ids []int64) ([]service.ProfitCalendarAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, status, schedulable, type, created_at
		FROM accounts
		WHERE deleted_at IS NULL AND id = ANY($1)
		ORDER BY created_at, id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProfitCalendarAccounts(rows)
}

func (r *businessFinanceRepository) listProfitCalendarExpenses(ctx context.Context) ([]service.ProfitCalendarExpense, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, amount, exchange_rate_to_billing_unit, occurred_at, scope
		FROM business_expenses
		WHERE status = 'active' AND category = 'account_purchase'
		ORDER BY occurred_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]service.ProfitCalendarExpense, 0)
	for rows.Next() {
		var item service.ProfitCalendarExpense
		var scope []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Amount, &item.ExchangeRate, &item.OccurredAt, &scope); err != nil {
			return nil, err
		}
		if len(scope) > 0 {
			if err := json.Unmarshal(scope, &item.Scope); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *businessFinanceRepository) listProfitCalendarUsage(ctx context.Context, ids []int64) ([]service.ProfitCalendarUsage, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT account_id,
		       COUNT(*)::bigint,
		       COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0)::bigint,
		       COALESCE(SUM(total_cost), 0)::double precision,
		       COALESCE(SUM(actual_cost), 0)::double precision
		FROM usage_logs
		WHERE account_id = ANY($1)
		GROUP BY account_id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]service.ProfitCalendarUsage, 0)
	for rows.Next() {
		var item service.ProfitCalendarUsage
		if err := rows.Scan(&item.AccountID, &item.Requests, &item.OfficialTokens, &item.OfficialBilling, &item.UserBilling); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanProfitCalendarAccounts(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]service.ProfitCalendarAccount, error) {
	items := make([]service.ProfitCalendarAccount, 0)
	for rows.Next() {
		var item service.ProfitCalendarAccount
		if err := rows.Scan(&item.ID, &item.Name, &item.Status, &item.Schedulable, &item.Type, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
