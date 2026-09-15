package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type watchedTrafficRepository struct {
	db *sql.DB
}

func NewWatchedTrafficRepository(db *sql.DB) service.WatchedTrafficRepository {
	return &watchedTrafficRepository{db: db}
}

func (r *watchedTrafficRepository) Create(ctx context.Context, rec *service.WatchedTrafficRecord) error {
	if rec == nil {
		return nil
	}
	return r.db.QueryRowContext(ctx, `
INSERT INTO watched_traffic_logs (
    request_id, user_id, user_email, api_key_id, api_key_name,
    group_id, group_name, account_id, model, endpoint, status_code,
    prompt_text, response_text, error_text
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, $11,
    $12, $13, $14
) RETURNING id, created_at`,
		rec.RequestID,
		rec.UserID,
		rec.UserEmail,
		nullableInt64(rec.APIKeyID),
		rec.APIKeyName,
		nullableInt64(rec.GroupID),
		rec.GroupName,
		nullableInt64(rec.AccountID),
		rec.Model,
		rec.Endpoint,
		rec.StatusCode,
		rec.PromptText,
		rec.ResponseText,
		rec.ErrorText,
	).Scan(&rec.ID, &rec.CreatedAt)
}

func (r *watchedTrafficRepository) List(ctx context.Context, filter service.WatchedTrafficFilter) ([]service.WatchedTrafficRecord, *pagination.PaginationResult, error) {
	where := []string{"1=1"}
	args := make([]any, 0, 4)
	if filter.UserID != nil && *filter.UserID > 0 {
		args = append(args, *filter.UserID)
		where = append(where, fmt.Sprintf("user_id = $%d", len(args)))
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM watched_traffic_logs "+whereSQL, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count watched traffic logs: %w", err)
	}

	params := filter.Pagination
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
SELECT
    id, request_id, user_id, user_email, api_key_id, api_key_name,
    group_id, group_name, account_id, model, endpoint, status_code,
    prompt_text, response_text, error_text, created_at
FROM watched_traffic_logs
`+whereSQL+`
ORDER BY created_at DESC, id DESC
LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs)),
		queryArgs...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list watched traffic logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.WatchedTrafficRecord, 0)
	for rows.Next() {
		var item service.WatchedTrafficRecord
		var apiKeyID, groupID, accountID sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&item.RequestID,
			&item.UserID,
			&item.UserEmail,
			&apiKeyID,
			&item.APIKeyName,
			&groupID,
			&item.GroupName,
			&accountID,
			&item.Model,
			&item.Endpoint,
			&item.StatusCode,
			&item.PromptText,
			&item.ResponseText,
			&item.ErrorText,
			&item.CreatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan watched traffic log: %w", err)
		}
		if apiKeyID.Valid {
			v := apiKeyID.Int64
			item.APIKeyID = &v
		}
		if groupID.Valid {
			v := groupID.Int64
			item.GroupID = &v
		}
		if accountID.Valid {
			v := accountID.Int64
			item.AccountID = &v
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate watched traffic logs: %w", err)
	}
	return items, paginationResultFromTotal(total, params), nil
}
