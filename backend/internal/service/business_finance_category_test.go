package service

import (
	"testing"
	"time"
)

func TestFinanceCategoryAcceptsDomainAndCompliance(t *testing.T) {
	t.Parallel()

	for _, category := range []string{"domain", "compliance"} {
		category := category
		t.Run(category, func(t *testing.T) {
			t.Parallel()

			costInput := CostConfigInput{
				Code:             "category-" + category,
				Name:             "category " + category,
				Category:         category,
				Amount:           1,
				Currency:         "CNY",
				ExchangeRate:     1,
				AllocationMethod: "revenue_share",
				Frequency:        "yearly",
				Scope:            map[string]any{},
				EffectiveFrom:    time.Now().UTC(),
			}
			if err := validateCostConfigInput(&costInput); err != nil {
				t.Fatalf("validate cost category %q: %v", category, err)
			}

			expenseInput := ExpenseInput{
				Name:             "expense " + category,
				Category:         category,
				Amount:           1,
				Currency:         "CNY",
				ExchangeRate:     1,
				OccurredAt:       time.Now().UTC(),
				AllocationMethod: "revenue_share",
				Scope:            map[string]any{},
			}
			if err := validateExpenseInput(&expenseInput); err != nil {
				t.Fatalf("validate expense category %q: %v", category, err)
			}
		})
	}
}
