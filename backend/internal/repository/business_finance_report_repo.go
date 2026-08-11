package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type financeUsageRow struct {
	Key       string
	Name      string
	GroupID   int64
	ChannelID int64
	AccountID int64
	Model     string
	Requests  int64
	Tokens    int64
	Revenue   float64
	Direct    float64
}

type financeCostComponent struct {
	service.FinanceCostComponent
	Scope map[string]any
	Rate  string
	From  time.Time
	To    *time.Time
}

func (r *businessFinanceRepository) GetBusinessFinanceReport(ctx context.Context, filter service.FinanceReportFilter) (*service.FinanceReport, error) {
	rows, err := r.queryFinanceUsageRows(ctx, filter)
	if err != nil {
		return nil, err
	}
	components, err := r.loadFinanceCostComponents(ctx, filter.StartTime, filter.EndTime)
	if err != nil {
		return nil, err
	}

	var totalRequests, totalTokens int64
	var totalRevenue, totalDirect float64
	for _, row := range rows {
		totalRequests += row.Requests
		totalTokens += row.Tokens
		totalRevenue += row.Revenue
		totalDirect += row.Direct
	}

	componentOutput := make([]service.FinanceCostComponent, 0, len(components))
	for _, component := range components {
		componentOutput = append(componentOutput, component.FinanceCostComponent)
	}

	var totalOperating float64
	for _, row := range rows {
		totalOperating += allocateFinanceOperatingCost(components, row, rows, totalRequests, totalTokens, totalRevenue, totalDirect, filter.Dimension)
	}
	summary := financeMetric(totalRequests, totalTokens, totalRevenue, totalDirect, totalOperating)
	outputRows := aggregateFinanceRows(rows, components, totalRequests, totalTokens, totalRevenue, totalDirect, filter.Dimension)
	trendRows, err := r.queryFinanceTrendRows(ctx, filter)
	if err != nil {
		return nil, err
	}
	trend := aggregateFinanceTrendRows(trendRows, components, totalRequests, totalTokens, totalRevenue, totalDirect)

	alerts := buildFinanceRiskAlerts(outputRows, filter.Dimension, filter.MinMargin)
	return &service.FinanceReport{
		StartTime: filter.StartTime, EndTime: filter.EndTime, Dimension: filter.Dimension,
		Summary: summary, Rows: outputRows, Trend: trend, Components: componentOutput, Alerts: alerts,
	}, nil
}

func (r *businessFinanceRepository) queryFinanceUsageRows(ctx context.Context, filter service.FinanceReportFilter) ([]financeUsageRow, error) {
	dimensionKey, dimensionName, joins, groupBy := financeDimensionSQL(filter.Dimension)
	query := fmt.Sprintf(`
		SELECT %s AS dimension_key, %s AS dimension_name,
		       COALESCE(ul.group_id, 0)::bigint,
		       COALESCE(ul.channel_id, 0)::bigint,
		       COALESCE(ul.account_id, 0)::bigint,
		       COALESCE(ul.model, ''),
		       COUNT(*)::bigint,
		       COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0)::bigint,
		       COALESCE(SUM(ul.actual_cost), 0)::double precision AS revenue,
		       COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0)::double precision AS direct_cost
		FROM usage_logs ul
		%s
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		  AND ($3 = 0 OR ul.group_id = $3)
		  AND ($4 = 0 OR ul.channel_id = $4)
		  AND ($5 = '' OR ul.model = $5)
		GROUP BY %s, ul.group_id, ul.channel_id, ul.account_id, ul.model
		ORDER BY revenue DESC, direct_cost DESC, dimension_key ASC`,
		dimensionKey, dimensionName, joins, groupBy)

	rows, err := r.db.QueryContext(ctx, query, filter.StartTime, filter.EndTime, filter.GroupID, filter.ChannelID, filter.Model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]financeUsageRow, 0)
	for rows.Next() {
		var item financeUsageRow
		if err := rows.Scan(&item.Key, &item.Name, &item.GroupID, &item.ChannelID, &item.AccountID, &item.Model, &item.Requests, &item.Tokens, &item.Revenue, &item.Direct); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *businessFinanceRepository) queryFinanceTrendRows(ctx context.Context, filter service.FinanceReportFilter) ([]financeUsageRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT TO_CHAR(DATE_TRUNC('day', ul.created_at AT TIME ZONE 'UTC'), 'YYYY-MM-DD') AS day,
		       TO_CHAR(DATE_TRUNC('day', ul.created_at AT TIME ZONE 'UTC'), 'YYYY-MM-DD') AS day_name,
		       COALESCE(ul.group_id, 0)::bigint,
		       COALESCE(ul.channel_id, 0)::bigint,
		       COALESCE(ul.account_id, 0)::bigint,
		       COALESCE(ul.model, ''),
		       COUNT(*)::bigint,
		       COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0)::bigint,
		       COALESCE(SUM(ul.actual_cost), 0)::double precision,
		       COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0)::double precision
		FROM usage_logs ul
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		  AND ($3 = 0 OR ul.group_id = $3)
		  AND ($4 = 0 OR ul.channel_id = $4)
		  AND ($5 = '' OR ul.model = $5)
		GROUP BY DATE_TRUNC('day', ul.created_at AT TIME ZONE 'UTC'), ul.group_id, ul.channel_id, ul.account_id, ul.model
		ORDER BY day ASC`, filter.StartTime, filter.EndTime, filter.GroupID, filter.ChannelID, filter.Model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]financeUsageRow, 0)
	for rows.Next() {
		var item financeUsageRow
		if err := rows.Scan(&item.Key, &item.Name, &item.GroupID, &item.ChannelID, &item.AccountID, &item.Model, &item.Requests, &item.Tokens, &item.Revenue, &item.Direct); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func financeDimensionSQL(dimension string) (key, name, joins, groupBy string) {
	switch dimension {
	case "channel":
		return "COALESCE(ul.channel_id, 0)::text", "COALESCE(ch.name, '未关联渠道')", "LEFT JOIN channels ch ON ch.id = ul.channel_id", "ul.channel_id, ch.name"
	case "model":
		return "COALESCE(ul.model, '')", "COALESCE(ul.model, '')", "", "ul.model"
	case "account":
		return "COALESCE(ul.account_id, 0)::text", "COALESCE(a.name, CASE WHEN ul.account_id IS NULL THEN '未关联账号' ELSE '#' || ul.account_id::text END)", "LEFT JOIN accounts a ON a.id = ul.account_id", "ul.account_id, a.name"
	default:
		return "COALESCE(ul.group_id, 0)::text", "COALESCE(g.name, '未分组')", "LEFT JOIN groups g ON g.id = ul.group_id", "ul.group_id, g.name"
	}
}

func (r *businessFinanceRepository) loadFinanceCostComponents(ctx context.Context, start, end time.Time) ([]financeCostComponent, error) {
	components := make([]financeCostComponent, 0)
	configRows, err := r.db.QueryContext(ctx, `
		SELECT category, name, amount::double precision, currency, exchange_rate_to_billing_unit::double precision,
		       allocation_method, frequency, scope,
		       effective_from, effective_to
		FROM business_cost_configs
		WHERE enabled = TRUE
		  AND effective_from < $2
		  AND (effective_to IS NULL OR effective_to > $1)`, start, end)
	if err != nil {
		return nil, err
	}
	for configRows.Next() {
		var category, name, currency, method, frequency string
		var amount, exchangeRate float64
		var scopeRaw []byte
		var from time.Time
		var to sql.NullTime
		if err := configRows.Scan(&category, &name, &amount, &currency, &exchangeRate, &method, &frequency, &scopeRaw, &from, &to); err != nil {
			configRows.Close()
			return nil, err
		}
		var toPtr *time.Time
		if to.Valid {
			value := to.Time
			toPtr = &value
		}
		rawAmount := amount
		factor := recurringAmount(frequency, from, toPtr, start, end)
		amount = rawAmount * exchangeRate * factor
		if amount > 0 {
			components = append(components, financeCostComponent{
				FinanceCostComponent: service.FinanceCostComponent{Category: category, Name: name, Amount: amount, OriginalAmount: rawAmount, Currency: currency, ExchangeRate: exchangeRate, AllocationMethod: method, Source: "config"},
				Scope:                decodeReportJSONMap(scopeRaw), Rate: frequency, From: from, To: toPtr,
			})
		}
	}
	if err := configRows.Err(); err != nil {
		configRows.Close()
		return nil, err
	}
	configRows.Close()

	expenseRows, err := r.db.QueryContext(ctx, `
		SELECT category, name, amount::double precision, currency, exchange_rate_to_billing_unit::double precision, allocation_method, scope
		FROM business_expenses
		WHERE status = 'active'
		  AND COALESCE(period_end, occurred_at + interval '1 microsecond') > $1
		  AND COALESCE(period_start, occurred_at) < $2`, start, end)
	if err != nil {
		return nil, err
	}
	for expenseRows.Next() {
		var category, name, currency, method string
		var amount, exchangeRate float64
		var scopeRaw []byte
		if err := expenseRows.Scan(&category, &name, &amount, &currency, &exchangeRate, &method, &scopeRaw); err != nil {
			expenseRows.Close()
			return nil, err
		}
		if amount > 0 {
			components = append(components, financeCostComponent{
				FinanceCostComponent: service.FinanceCostComponent{Category: category, Name: name, Amount: amount * exchangeRate, OriginalAmount: amount, Currency: currency, ExchangeRate: exchangeRate, AllocationMethod: method, Source: "expense"},
				Scope:                decodeReportJSONMap(scopeRaw),
			})
		}
	}
	if err := expenseRows.Err(); err != nil {
		expenseRows.Close()
		return nil, err
	}
	expenseRows.Close()

	// These two costs are derived from authoritative existing ledgers, so they
	// are visible in reports even when an operator has not manually entered a
	// duplicate expense row.
	var affiliateCost float64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)::double precision
		FROM user_affiliate_ledger
		WHERE action = 'accrue' AND created_at >= $1 AND created_at < $2`, start, end).Scan(&affiliateCost); err != nil {
		return nil, err
	}
	if affiliateCost > 0 {
		components = append(components, financeCostComponent{FinanceCostComponent: service.FinanceCostComponent{
			Category: "affiliate", Name: "实际返佣台账", Amount: affiliateCost, OriginalAmount: affiliateCost, Currency: "BILLING", ExchangeRate: 1, AllocationMethod: "revenue_share", Source: "affiliate_ledger",
		}})
	}

	var paymentFee float64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount * fee_rate / 100), 0)::double precision
		FROM payment_orders
		WHERE status = 'COMPLETED' AND COALESCE(paid_at, completed_at, created_at) >= $1
		  AND COALESCE(paid_at, completed_at, created_at) < $2`, start, end).Scan(&paymentFee); err != nil {
		return nil, err
	}
	if paymentFee > 0 {
		components = append(components, financeCostComponent{FinanceCostComponent: service.FinanceCostComponent{
			Category: "payment_fee", Name: "充值支付手续费", Amount: paymentFee, OriginalAmount: paymentFee, Currency: "BILLING", ExchangeRate: 1, AllocationMethod: "revenue_share", Source: "payment_orders",
		}})
	}
	return components, nil
}

func recurringAmount(frequency string, from time.Time, to *time.Time, start, end time.Time) float64 {
	overlapStart := from
	if overlapStart.Before(start) {
		overlapStart = start
	}
	overlapEnd := end
	if to != nil && to.Before(overlapEnd) {
		overlapEnd = *to
	}
	if !overlapEnd.After(overlapStart) {
		return 0
	}
	switch frequency {
	case "one_time":
		if !from.Before(start) && from.Before(end) {
			return 1
		}
		return 0
	case "daily":
		return overlapEnd.Sub(overlapStart).Hours() / 24
	case "yearly":
		return calendarFraction(overlapStart, overlapEnd, false)
	default:
		return calendarFraction(overlapStart, overlapEnd, true)
	}
}

func calendarFraction(start, end time.Time, monthly bool) float64 {
	if !end.After(start) {
		return 0
	}
	current := start
	var total float64
	for current.Before(end) {
		var next time.Time
		if monthly {
			next = time.Date(current.Year(), current.Month()+1, 1, 0, 0, 0, 0, current.Location())
		} else {
			next = time.Date(current.Year()+1, 1, 1, 0, 0, 0, 0, current.Location())
		}
		segmentEnd := next
		if segmentEnd.After(end) {
			segmentEnd = end
		}
		periodStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, current.Location())
		periodEnd := next
		if !monthly {
			periodStart = time.Date(current.Year(), 1, 1, 0, 0, 0, 0, current.Location())
		}
		total += segmentEnd.Sub(current).Seconds() / periodEnd.Sub(periodStart).Seconds()
		current = segmentEnd
	}
	return total
}

func aggregateFinanceRows(rows []financeUsageRow, components []financeCostComponent, totalRequests, totalTokens int64, totalRevenue, totalDirect float64, dimension string) []service.FinanceReportRow {
	result := make([]service.FinanceReportRow, 0, len(rows))
	indexes := make(map[string]int, len(rows))
	for _, row := range rows {
		operating := allocateFinanceOperatingCost(components, row, rows, totalRequests, totalTokens, totalRevenue, totalDirect, dimension)
		metric := financeMetric(row.Requests, row.Tokens, row.Revenue, row.Direct, operating)
		index, ok := indexes[row.Key]
		if !ok {
			indexes[row.Key] = len(result)
			result = append(result, service.FinanceReportRow{Key: row.Key, Name: row.Name, FinanceMetric: metric})
			continue
		}
		addFinanceMetric(&result[index].FinanceMetric, metric)
	}
	return result
}

func aggregateFinanceTrendRows(rows []financeUsageRow, components []financeCostComponent, totalRequests, totalTokens int64, totalRevenue, totalDirect float64) []service.FinanceTrendPoint {
	result := make([]service.FinanceTrendPoint, 0, len(rows))
	indexes := make(map[string]int, len(rows))
	for _, row := range rows {
		operating := allocateFinanceOperatingCost(components, row, rows, totalRequests, totalTokens, totalRevenue, totalDirect, "trend")
		metric := financeMetric(row.Requests, row.Tokens, row.Revenue, row.Direct, operating)
		index, ok := indexes[row.Key]
		if !ok {
			indexes[row.Key] = len(result)
			result = append(result, service.FinanceTrendPoint{Date: row.Key, FinanceMetric: metric})
			continue
		}
		addFinanceMetric(&result[index].FinanceMetric, metric)
	}
	return result
}

func addFinanceMetric(target *service.FinanceMetric, value service.FinanceMetric) {
	target.Requests += value.Requests
	target.Tokens += value.Tokens
	target.Revenue += value.Revenue
	target.DirectCost += value.DirectCost
	target.OperatingCost += value.OperatingCost
	target.TotalCost += value.TotalCost
	target.GrossProfit += value.GrossProfit
	target.OperatingProfit += value.OperatingProfit
	if target.Revenue > 0 {
		target.GrossMargin = target.GrossProfit / target.Revenue
		target.OperatingMargin = target.OperatingProfit / target.Revenue
	}
	if target.DirectCost > 0 {
		target.CostMultiplier = target.TotalCost / target.DirectCost
	}
}

func allocateFinanceOperatingCost(components []financeCostComponent, row financeUsageRow, allRows []financeUsageRow, totalRequests, totalTokens int64, totalRevenue, totalDirect float64, dimension string) float64 {
	var total float64
	for _, component := range components {
		if !financeScopeMatches(component.Scope, row, dimension) {
			continue
		}
		share := 0.0
		switch component.AllocationMethod {
		case "request_share":
			if totalRequests > 0 {
				share = float64(row.Requests) / float64(totalRequests)
			}
		case "token_share":
			if totalTokens > 0 {
				share = float64(row.Tokens) / float64(totalTokens)
			}
		case "account_average":
			rowCount := countFinanceAllocationRows(allRows, component.Scope, dimension)
			if rowCount > 0 {
				share = 1 / float64(rowCount)
			}
		case "direct":
			if totalDirect > 0 {
				share = row.Direct / totalDirect
			}
		default:
			if totalRevenue > 0 {
				share = row.Revenue / totalRevenue
			}
		}
		total += component.Amount * share
	}
	return total
}

func financeScopeMatches(scope map[string]any, row financeUsageRow, dimension string) bool {
	if len(scope) == 0 {
		return true
	}
	for scopeKey, value := range scope {
		var rowValue string
		switch scopeKey {
		case "group_id":
			rowValue = strconv.FormatInt(row.GroupID, 10)
		case "channel_id":
			rowValue = strconv.FormatInt(row.ChannelID, 10)
		case "account_id":
			rowValue = strconv.FormatInt(row.AccountID, 10)
		case "model":
			rowValue = row.Model
		default:
			return false
		}
		if normalizeReportScopeValue(value) != rowValue {
			return false
		}
	}
	return true
}

func countFinanceAllocationRows(rows []financeUsageRow, scope map[string]any, dimension string) int {
	count := 0
	for _, row := range rows {
		if financeScopeMatches(scope, row, dimension) {
			count++
		}
	}
	return count
}

func normalizeReportScopeValue(value any) string {
	switch typed := value.(type) {
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case json.Number:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func financeMetric(requests, tokens int64, revenue, direct, operating float64) service.FinanceMetric {
	total := direct + operating
	metric := service.FinanceMetric{
		Requests: requests, Tokens: tokens, Revenue: revenue,
		DirectCost: direct, OperatingCost: operating, TotalCost: total,
		GrossProfit: revenue - direct, OperatingProfit: revenue - total,
	}
	if revenue > 0 {
		metric.GrossMargin = metric.GrossProfit / revenue
		metric.OperatingMargin = metric.OperatingProfit / revenue
	}
	if direct > 0 {
		metric.CostMultiplier = total / direct
	}
	return metric
}

func buildFinanceRiskAlerts(rows []service.FinanceReportRow, dimension string, minMargin float64) []service.FinanceRiskAlert {
	alerts := make([]service.FinanceRiskAlert, 0)
	for _, row := range rows {
		if row.Revenue <= 0 && row.TotalCost <= 0 {
			continue
		}
		severity, reason := "", ""
		if row.OperatingProfit < 0 {
			severity, reason = "critical", "经营亏损"
		} else if row.OperatingMargin < minMargin {
			severity, reason = "warning", "经营毛利率低于阈值"
		} else if row.CostMultiplier >= 1.2 {
			severity, reason = "warning", "综合成本倍率偏高"
		}
		if severity != "" {
			alerts = append(alerts, service.FinanceRiskAlert{Severity: severity, Dimension: dimension, Key: row.Key, Name: row.Name, Reason: reason, OperatingProfit: row.OperatingProfit, OperatingMargin: row.OperatingMargin, CostMultiplier: row.CostMultiplier})
		}
	}
	return alerts
}

func decodeReportJSONMap(raw []byte) map[string]any {
	value := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &value)
	}
	return value
}

func (r *businessFinanceRepository) GetBusinessFinanceGrowth(ctx context.Context, filter service.FinanceReportFilter) (*service.FinanceGrowthReport, error) {
	var report service.FinanceGrowthReport
	report.StartTime, report.EndTime = filter.StartTime, filter.EndTime
	queries := []struct {
		dest      *int64
		query     string
		withRange bool
	}{
		{&report.NewUsers, `SELECT COUNT(*) FROM users WHERE created_at >= $1 AND created_at < $2`, true},
		{&report.ActiveUsers, `SELECT COUNT(DISTINCT user_id) FROM usage_logs WHERE created_at >= $1 AND created_at < $2`, true},
		{&report.OnlineUsers, `SELECT COUNT(DISTINCT user_id) FROM usage_logs WHERE created_at >= NOW() - interval '15 minutes'`, false},
		{&report.PayingUsers, `SELECT COUNT(DISTINCT user_id) FROM payment_orders WHERE status = 'COMPLETED' AND COALESCE(paid_at, completed_at, created_at) >= $1 AND COALESCE(paid_at, completed_at, created_at) < $2`, true},
	}
	for _, item := range queries {
		var err error
		if item.withRange {
			err = r.db.QueryRowContext(ctx, item.query, filter.StartTime, filter.EndTime).Scan(item.dest)
		} else {
			err = r.db.QueryRowContext(ctx, item.query).Scan(item.dest)
		}
		if err != nil {
			return nil, err
		}
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0)::double precision FROM payment_orders WHERE status = 'COMPLETED' AND COALESCE(paid_at, completed_at, created_at) >= $1 AND COALESCE(paid_at, completed_at, created_at) < $2`, filter.StartTime, filter.EndTime).Scan(&report.RechargeAmount); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(actual_cost), 0)::double precision FROM usage_logs WHERE created_at >= $1 AND created_at < $2`, filter.StartTime, filter.EndTime).Scan(&report.Revenue); err != nil {
		return nil, err
	}
	components, err := r.loadFinanceCostComponents(ctx, filter.StartTime, filter.EndTime)
	if err != nil {
		return nil, err
	}
	for _, component := range components {
		switch component.Category {
		case "marketing":
			report.MarketingCost += component.Amount
		case "affiliate":
			report.AffiliateCost += component.Amount
		}
	}
	if report.NewUsers > 0 {
		report.CAC = report.MarketingCost / float64(report.NewUsers)
	}
	if report.PayingUsers > 0 {
		report.LTV = report.Revenue / float64(report.PayingUsers)
	}
	if report.MarketingCost > 0 {
		report.ROI = (report.Revenue - report.MarketingCost - report.AffiliateCost) / report.MarketingCost
	}

	rows, err := r.db.QueryContext(ctx, `
		WITH cohort AS (
			SELECT id, COALESCE(NULLIF(signup_source, ''), 'unknown') AS source
			FROM users WHERE created_at >= $1 AND created_at < $2
		), usage AS (
			SELECT user_id, SUM(actual_cost)::double precision AS revenue
			FROM usage_logs WHERE created_at >= $1 AND created_at < $2 GROUP BY user_id
		), payments AS (
			SELECT user_id, SUM(amount)::double precision AS recharge
			FROM payment_orders
			WHERE status = 'COMPLETED' AND COALESCE(paid_at, completed_at, created_at) >= $1
			  AND COALESCE(paid_at, completed_at, created_at) < $2
			GROUP BY user_id
		)
		SELECT c.source, COUNT(*)::bigint,
		       COUNT(u.user_id)::bigint, COUNT(p.user_id)::bigint,
		       COALESCE(SUM(u.revenue), 0)::double precision,
		       COALESCE(SUM(p.recharge), 0)::double precision
		FROM cohort c LEFT JOIN usage u ON u.user_id = c.id LEFT JOIN payments p ON p.user_id = c.id
		GROUP BY c.source ORDER BY COUNT(*) DESC, c.source`, filter.StartTime, filter.EndTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item service.FinanceGrowthSource
		if err := rows.Scan(&item.Source, &item.NewUsers, &item.ActiveUsers, &item.PayingUsers, &item.Revenue, &item.Recharge); err != nil {
			return nil, err
		}
		report.BySource = append(report.BySource, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &report, nil
}
