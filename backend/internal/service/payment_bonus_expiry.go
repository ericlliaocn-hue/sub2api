package service

import (
	"context"
	"fmt"
)

// ExpireDueRechargeBonuses eagerly reclaims expired bonus lots in bounded user batches.
// Request-time expiry remains the final enforcement path; this sweep keeps displayed
// balances and caches current even when a user is idle.
func (s *PaymentService) ExpireDueRechargeBonuses(ctx context.Context, limit int) ([]int64, float64, error) {
	if s == nil || s.entClient == nil {
		return nil, 0, nil
	}
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	rows, err := s.entClient.QueryContext(ctx, `
		SELECT DISTINCT user_id
		FROM recharge_bonus_grants
		WHERE status='active' AND remaining_amount>0 AND expires_at<=NOW()
		ORDER BY user_id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, 0, err
	}
	userIDs := make([]int64, 0, limit)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}

	expiredUsers := make([]int64, 0, len(userIDs))
	totalExpired := 0.0
	for _, userID := range userIDs {
		if err := ctx.Err(); err != nil {
			return expiredUsers, totalExpired, err
		}
		tx, err := s.entClient.Tx(ctx)
		if err != nil {
			return expiredUsers, totalExpired, err
		}
		expired, expireErr := ExpireRechargeBonusLots(ctx, tx.Client(), userID)
		if expireErr != nil {
			_ = tx.Rollback()
			return expiredUsers, totalExpired, fmt.Errorf("expire bonus for user %d: %w", userID, expireErr)
		}
		if err := tx.Commit(); err != nil {
			return expiredUsers, totalExpired, err
		}
		if expired > balanceEpsilon {
			expiredUsers = append(expiredUsers, userID)
			totalExpired += expired
		}
	}
	return expiredUsers, totalExpired, nil
}
