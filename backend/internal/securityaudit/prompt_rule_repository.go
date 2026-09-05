package securityaudit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type PostgreSQLRuleRepository struct {
	db *sql.DB
}

func NewPostgreSQLRuleRepository(db *sql.DB) *PostgreSQLRuleRepository {
	return &PostgreSQLRuleRepository{db: db}
}

func (r *PostgreSQLRuleRepository) List(ctx context.Context, includeDisabled bool) ([]PromptGuardRule, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("prompt guard rule database unavailable")
	}
	query := `
SELECT id, rule_key, name, category, severity, pattern_type, pattern, action, priority, enabled, created_at, updated_at, updated_by
FROM prompt_guard_rules`
	if !includeDisabled {
		query += ` WHERE enabled = TRUE`
	}
	query += ` ORDER BY priority ASC, id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list prompt guard rules: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]PromptGuardRule, 0)
	for rows.Next() {
		var rule PromptGuardRule
		if err := rows.Scan(&rule.ID, &rule.RuleKey, &rule.Name, &rule.Category, &rule.Severity,
			&rule.PatternType, &rule.Pattern, &rule.Action, &rule.Priority, &rule.Enabled,
			&rule.CreatedAt, &rule.UpdatedAt, &rule.UpdatedBy); err != nil {
			return nil, fmt.Errorf("scan prompt guard rule: %w", err)
		}
		result = append(result, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prompt guard rules: %w", err)
	}
	return result, nil
}

func (r *PostgreSQLRuleRepository) Create(ctx context.Context, input UpsertPromptGuardRuleRequest, actorID int64) (*PromptGuardRule, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("prompt guard rule database unavailable")
	}
	if err := ValidatePromptGuardRule(input); err != nil {
		return nil, err
	}
	input = normalizePromptGuardRuleRequest(input)
	var rule PromptGuardRule
	err := r.db.QueryRowContext(ctx, `
INSERT INTO prompt_guard_rules (rule_key, name, category, severity, pattern_type, pattern, action, priority, enabled, updated_by)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id, rule_key, name, category, severity, pattern_type, pattern, action, priority, enabled, created_at, updated_at, updated_by`,
		input.RuleKey, input.Name, input.Category, input.Severity, input.PatternType, input.Pattern,
		input.Action, input.Priority, input.Enabled, actorID).Scan(
		&rule.ID, &rule.RuleKey, &rule.Name, &rule.Category, &rule.Severity, &rule.PatternType,
		&rule.Pattern, &rule.Action, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt, &rule.UpdatedBy)
	if err != nil {
		return nil, fmt.Errorf("create prompt guard rule: %w", err)
	}
	return &rule, nil
}

func (r *PostgreSQLRuleRepository) Update(ctx context.Context, input UpsertPromptGuardRuleRequest, actorID int64) (*PromptGuardRule, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("prompt guard rule database unavailable")
	}
	if input.ID <= 0 {
		return nil, fmt.Errorf("%w: id", ErrPromptGuardRuleInvalid)
	}
	if err := ValidatePromptGuardRule(input); err != nil {
		return nil, err
	}
	input = normalizePromptGuardRuleRequest(input)
	var rule PromptGuardRule
	err := r.db.QueryRowContext(ctx, `
UPDATE prompt_guard_rules
SET rule_key=$2, name=$3, category=$4, severity=$5, pattern_type=$6, pattern=$7,
    action=$8, priority=$9, enabled=$10, updated_by=$11, updated_at=NOW()
WHERE id=$1
RETURNING id, rule_key, name, category, severity, pattern_type, pattern, action, priority, enabled, created_at, updated_at, updated_by`,
		input.ID, input.RuleKey, input.Name, input.Category, input.Severity, input.PatternType,
		input.Pattern, input.Action, input.Priority, input.Enabled, actorID).Scan(
		&rule.ID, &rule.RuleKey, &rule.Name, &rule.Category, &rule.Severity, &rule.PatternType,
		&rule.Pattern, &rule.Action, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt, &rule.UpdatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPromptGuardRuleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update prompt guard rule: %w", err)
	}
	return &rule, nil
}

func (r *PostgreSQLRuleRepository) Delete(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("prompt guard rule database unavailable")
	}
	if id <= 0 {
		return fmt.Errorf("%w: id", ErrPromptGuardRuleInvalid)
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM prompt_guard_rules WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete prompt guard rule: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrPromptGuardRuleNotFound
	}
	return nil
}

func normalizePromptGuardRuleRequest(input UpsertPromptGuardRuleRequest) UpsertPromptGuardRuleRequest {
	input.RuleKey = strings.TrimSpace(input.RuleKey)
	input.Name = strings.TrimSpace(input.Name)
	input.Category = NormalizeCategory(input.Category)
	input.Severity = RiskLevel(strings.ToLower(strings.TrimSpace(string(input.Severity))))
	input.PatternType = strings.ToLower(strings.TrimSpace(input.PatternType))
	input.Pattern = strings.TrimSpace(input.Pattern)
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	if input.Priority == 0 {
		input.Priority = 100
	}
	return input
}
