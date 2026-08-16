package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func listBulkUpdatedAccountIDs(ctx context.Context, queryer sqlQueryer, ids []int64, apiKeyOnly bool) ([]int64, error) {
	query := `
		SELECT id
		FROM accounts
		WHERE id = ANY($1) AND deleted_at IS NULL
	`
	args := []any{pq.Array(ids)}
	if apiKeyOnly {
		query += " AND type = $2"
		args = append(args, service.AccountTypeAPIKey)
	}
	rows, err := queryer.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	accountIDs := make([]int64, 0, len(ids))
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			return nil, err
		}
		accountIDs = append(accountIDs, accountID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accountIDs, nil
}
