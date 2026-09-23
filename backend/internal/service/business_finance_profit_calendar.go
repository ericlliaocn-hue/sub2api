package service

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	profitCalendarTimezone    = "Asia/Shanghai"
	profitCalendarMaxDays     = 93
	profitCalendarDefaultDays = 7
)

// ProfitCalendar is the import-day P&L board: one row per purchased identity
// (duplicate emails / scoped siblings are merged), bucketed by first import day.
type ProfitCalendar struct {
	StartTime time.Time             `json:"start_time"`
	EndTime   time.Time             `json:"end_time"`
	Timezone  string                `json:"timezone"`
	Days      []ProfitCalendarDay   `json:"days"`
	Summary   ProfitCalendarSummary `json:"summary"`
}

type ProfitCalendarSummary struct {
	Accounts        int     `json:"accounts"`
	Recorded        int     `json:"recorded"`
	Unrecorded      int     `json:"unrecorded"`
	Recouped        int     `json:"recouped"`
	Short           int     `json:"short"`
	Cost            float64 `json:"cost"`
	OfficialTokens  int64   `json:"official_tokens"`
	OfficialBilling float64 `json:"official_billing"`
	UserBilling     float64 `json:"user_billing"`
	Profit          float64 `json:"profit"`
}

type ProfitCalendarDay struct {
	Date            string              `json:"date"`
	Rows            []ProfitCalendarRow `json:"rows"`
	Cost            float64             `json:"cost"`
	OfficialTokens  int64               `json:"official_tokens"`
	OfficialBilling float64             `json:"official_billing"`
	UserBilling     float64             `json:"user_billing"`
	Profit          float64             `json:"profit"`
	Recorded        int                 `json:"recorded"`
	Unrecorded      int                 `json:"unrecorded"`
}

type ProfitCalendarRow struct {
	AccountIDs      []int64   `json:"account_ids"`
	AccountName     string    `json:"account_name"`
	ImportedAt      time.Time `json:"imported_at"`
	Status          string    `json:"status"`
	Schedulable     bool      `json:"schedulable"`
	ExpenseID       *int64    `json:"expense_id,omitempty"`
	Cost            float64   `json:"cost"`
	CostRecorded    bool      `json:"cost_recorded"`
	Requests        int64     `json:"requests"`
	OfficialTokens  int64     `json:"official_tokens"`
	OfficialBilling float64   `json:"official_billing"`
	UserBilling     float64   `json:"user_billing"`
	Profit          float64   `json:"profit"`
}

type ProfitCalendarAccount struct {
	ID          int64
	Name        string
	Status      string
	Schedulable bool
	Type        string
	CreatedAt   time.Time
}

type ProfitCalendarExpense struct {
	ID           int64
	Name         string
	Amount       float64
	ExchangeRate float64
	OccurredAt   time.Time
	Scope        map[string]any
}

type ProfitCalendarUsage struct {
	AccountID       int64
	Requests        int64
	OfficialTokens  int64
	OfficialBilling float64
	UserBilling     float64
}

func ProfitCalendarLocation() *time.Location {
	loc, err := time.LoadLocation(profitCalendarTimezone)
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

func NormalizeProfitCalendarRange(start, end time.Time) (time.Time, time.Time, error) {
	loc := ProfitCalendarLocation()
	if start.IsZero() && end.IsZero() {
		today := profitCalendarDayStart(time.Now().In(loc), loc)
		end = today.AddDate(0, 0, 1)
		start = end.AddDate(0, 0, -profitCalendarDefaultDays)
		return start.UTC(), end.UTC(), nil
	}
	if start.IsZero() || end.IsZero() {
		return time.Time{}, time.Time{}, fmt.Errorf("start_time and end_time are required together")
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end_time must be after start_time")
	}
	if end.Sub(start) > time.Duration(profitCalendarMaxDays)*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("profit calendar range cannot exceed %d days", profitCalendarMaxDays)
	}
	return start.UTC(), end.UTC(), nil
}

func profitCalendarDayStart(value time.Time, loc *time.Location) time.Time {
	local := value.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

func accountEmailKey(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	at := strings.LastIndex(normalized, "@")
	if at < 0 {
		return normalized
	}
	if dash := strings.Index(normalized[at:], "-"); dash >= 0 {
		return normalized[:at+dash]
	}
	return normalized
}

func BuildProfitCalendar(
	accounts []ProfitCalendarAccount,
	expenses []ProfitCalendarExpense,
	usage []ProfitCalendarUsage,
	start, end time.Time,
	loc *time.Location,
) *ProfitCalendar {
	if loc == nil {
		loc = ProfitCalendarLocation()
	}
	byID := make(map[int64]ProfitCalendarAccount, len(accounts))
	for _, account := range accounts {
		byID[account.ID] = account
	}

	parent := make(map[int64]int64, len(accounts))
	var find func(int64) int64
	find = func(id int64) int64 {
		if _, ok := parent[id]; !ok {
			parent[id] = id
		}
		if parent[id] != id {
			parent[id] = find(parent[id])
		}
		return parent[id]
	}
	union := func(a, b int64) {
		if a <= 0 || b <= 0 {
			return
		}
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	emailFirst := make(map[string]int64)
	for _, account := range accounts {
		find(account.ID)
		key := accountEmailKey(account.Name)
		if key == "" || !strings.Contains(key, "@") {
			continue
		}
		if first, ok := emailFirst[key]; ok {
			union(first, account.ID)
			continue
		}
		emailFirst[key] = account.ID
	}

	type boundExpense struct {
		expense    ProfitCalendarExpense
		accountIDs []int64
	}
	bound := make([]boundExpense, 0, len(expenses))
	for _, expense := range expenses {
		ids := FinanceScopeAccountIDs(expense.Scope)
		if len(ids) == 0 {
			continue
		}
		bound = append(bound, boundExpense{expense: expense, accountIDs: ids})
		for i := 1; i < len(ids); i++ {
			union(ids[0], ids[i])
		}
	}

	usageByID := make(map[int64]ProfitCalendarUsage, len(usage))
	for _, item := range usage {
		usageByID[item.AccountID] = item
	}

	groups := make(map[int64][]ProfitCalendarAccount)
	for id, account := range byID {
		root := find(id)
		groups[root] = append(groups[root], account)
	}

	expenseByRoot := make(map[int64]ProfitCalendarExpense)
	for _, item := range bound {
		root := find(item.accountIDs[0])
		current, exists := expenseByRoot[root]
		if !exists || item.expense.OccurredAt.Before(current.OccurredAt) {
			expenseByRoot[root] = item.expense
		}
	}

	rowsByDay := make(map[string][]ProfitCalendarRow)
	for _, members := range groups {
		sort.Slice(members, func(i, j int) bool {
			if members[i].CreatedAt.Equal(members[j].CreatedAt) {
				return members[i].ID < members[j].ID
			}
			return members[i].CreatedAt.Before(members[j].CreatedAt)
		})
		first := members[0]
		if first.CreatedAt.Before(start) || !first.CreatedAt.Before(end) {
			continue
		}
		ids := make([]int64, 0, len(members))
		var requests, tokens int64
		var official, user float64
		status := first.Status
		schedulable := false
		for _, member := range members {
			ids = append(ids, member.ID)
			if hit, ok := usageByID[member.ID]; ok {
				requests += hit.Requests
				tokens += hit.OfficialTokens
				official += hit.OfficialBilling
				user += hit.UserBilling
			}
			if member.Status == "active" && member.Schedulable {
				status = "active"
				schedulable = true
			} else if !schedulable && member.Status == "active" {
				status = "active"
			} else if !schedulable && status != "active" && member.Status != "" {
				status = member.Status
			}
		}
		row := ProfitCalendarRow{
			AccountIDs:      ids,
			AccountName:     first.Name,
			ImportedAt:      first.CreatedAt,
			Status:          status,
			Schedulable:     schedulable,
			Requests:        requests,
			OfficialTokens:  tokens,
			OfficialBilling: official,
			UserBilling:     user,
		}
		if expense, ok := expenseByRoot[find(first.ID)]; ok {
			id := expense.ID
			row.ExpenseID = &id
			row.CostRecorded = true
			row.Cost = ExpenseBillingCost(ExpenseEntry{Amount: expense.Amount, ExchangeRate: expense.ExchangeRate})
			if strings.TrimSpace(expense.Name) != "" {
				row.AccountName = expense.Name
			}
		}
		row.Profit = row.UserBilling - row.Cost
		day := profitCalendarDayStart(first.CreatedAt, loc).Format("2006-01-02")
		rowsByDay[day] = append(rowsByDay[day], row)
	}

	days := make([]ProfitCalendarDay, 0)
	for cursor := profitCalendarDayStart(start.In(loc), loc); cursor.Before(end); cursor = cursor.AddDate(0, 0, 1) {
		date := cursor.Format("2006-01-02")
		rows := rowsByDay[date]
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].ImportedAt.Equal(rows[j].ImportedAt) {
				return rows[i].AccountName < rows[j].AccountName
			}
			return rows[i].ImportedAt.Before(rows[j].ImportedAt)
		})
		day := ProfitCalendarDay{Date: date, Rows: rows}
		if day.Rows == nil {
			day.Rows = []ProfitCalendarRow{}
		}
		for _, row := range rows {
			day.Cost += row.Cost
			day.OfficialTokens += row.OfficialTokens
			day.OfficialBilling += row.OfficialBilling
			day.UserBilling += row.UserBilling
			day.Profit += row.Profit
			if row.CostRecorded {
				day.Recorded++
			} else {
				day.Unrecorded++
			}
		}
		days = append(days, day)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date > days[j].Date })

	summary := ProfitCalendarSummary{}
	for _, day := range days {
		summary.Cost += day.Cost
		summary.OfficialTokens += day.OfficialTokens
		summary.OfficialBilling += day.OfficialBilling
		summary.UserBilling += day.UserBilling
		summary.Profit += day.Profit
		summary.Recorded += day.Recorded
		summary.Unrecorded += day.Unrecorded
		for _, row := range day.Rows {
			summary.Accounts++
			if !row.CostRecorded {
				continue
			}
			if row.Profit >= 0 {
				summary.Recouped++
			} else {
				summary.Short++
			}
		}
	}

	return &ProfitCalendar{
		StartTime: start,
		EndTime:   end,
		Timezone:  profitCalendarTimezone,
		Days:      days,
		Summary:   summary,
	}
}
