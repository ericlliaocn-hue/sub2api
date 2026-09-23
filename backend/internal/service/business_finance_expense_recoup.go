package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// ExpenseRecoupWindow is the usage window used to judge whether an
// account-scoped expense has recouped. Period dates win over occurred_at;
// an open end means "until now" so a long-lived number keeps accumulating.
func ExpenseRecoupWindow(item ExpenseEntry, now time.Time) (accountID int64, start, end time.Time, ok bool) {
	accountID, ok = FinanceScopeAccountID(item.Scope)
	if !ok {
		return 0, time.Time{}, time.Time{}, false
	}
	start = item.OccurredAt
	if item.PeriodStart != nil && !item.PeriodStart.IsZero() {
		start = *item.PeriodStart
	}
	end = now
	if item.PeriodEnd != nil && !item.PeriodEnd.IsZero() {
		end = *item.PeriodEnd
	}
	if start.IsZero() || !end.After(start) {
		return 0, time.Time{}, time.Time{}, false
	}
	return accountID, start, end, true
}

func ExpenseBillingCost(item ExpenseEntry) float64 {
	rate := item.ExchangeRate
	if rate == 0 {
		rate = 1
	}
	return item.Amount * rate
}

func ApplyExpenseRecoup(item *ExpenseEntry, accountID int64, billed float64, requests int64, accountName string) {
	if item == nil {
		return
	}
	if accountID <= 0 {
		accountID, _ = FinanceScopeAccountID(item.Scope)
	}
	cost := ExpenseBillingCost(*item)
	item.Recoup = &ExpenseRecoup{
		AccountID:   accountID,
		AccountName: strings.TrimSpace(accountName),
		Billed:      billed,
		Requests:    requests,
		Cost:        cost,
		Profit:      billed - cost,
	}
}

func FinanceScopeAccountID(scope map[string]any) (int64, bool) {
	if len(scope) == 0 {
		return 0, false
	}
	id, ok := parseScopeInt64(scope["account_id"])
	return id, ok && id > 0
}

// FinanceScopeAccountIDs returns every account bound on an expense, including
// the merged sibling list stored in scope.account_ids.
func FinanceScopeAccountIDs(scope map[string]any) []int64 {
	if len(scope) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, 4)
	out := make([]int64, 0, 4)
	add := func(id int64) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if id, ok := FinanceScopeAccountID(scope); ok {
		add(id)
	}
	switch typed := scope["account_ids"].(type) {
	case []any:
		for _, value := range typed {
			if id, ok := parseScopeInt64(value); ok {
				add(id)
			}
		}
	case []int64:
		for _, id := range typed {
			add(id)
		}
	case []float64:
		for _, id := range typed {
			if parsed, ok := parseScopeInt64(id); ok {
				add(parsed)
			}
		}
	}
	return out
}

func parseScopeInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), typed >= 0
	case int32:
		return int64(typed), typed >= 0
	case int64:
		return typed, typed >= 0
	case uint:
		return int64(typed), true
	case uint64:
		if typed > uint64(1<<63-1) {
			return 0, false
		}
		return int64(typed), true
	case float64:
		if typed < 0 || typed != float64(int64(typed)) {
			return 0, false
		}
		return int64(typed), true
	case json.Number:
		parsed, err := strconv.ParseInt(typed.String(), 10, 64)
		return parsed, err == nil && parsed >= 0
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed, err == nil && parsed >= 0
	default:
		return 0, false
	}
}
