package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type UserBalanceLedgerItem struct {
	ID            int64           `json:"id"`
	EventType     string          `json:"event_type"`
	Amount        float64         `json:"amount"`
	BalanceBefore float64         `json:"balance_before"`
	BalanceAfter  float64         `json:"balance_after"`
	BonusBefore   float64         `json:"bonus_before"`
	BonusAfter    float64         `json:"bonus_after"`
	SourceType    string          `json:"source_type"`
	SourceID      string          `json:"source_id"`
	Description   string          `json:"description"`
	Metadata      json.RawMessage `json:"metadata"`
	CreatedAt     time.Time       `json:"created_at"`
}

type UserBalanceLedgerPage struct {
	Items             []UserBalanceLedgerItem `json:"items"`
	Total             int                     `json:"total"`
	Page              int                     `json:"page"`
	PageSize          int                     `json:"page_size"`
	ActiveBonus       float64                 `json:"active_bonus"`
	NextBonusExpiryAt *time.Time              `json:"next_bonus_expiry_at,omitempty"`
}

func (s *PaymentService) ListUserBalanceLedger(ctx context.Context, userID int64, page, pageSize int) (_ *UserBalanceLedgerPage, err error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	exec := tx.Client()
	if _, err = ExpireRechargeBonusLots(ctx, exec, userID); err != nil {
		return nil, fmt.Errorf("expire recharge bonus: %w", err)
	}

	result := &UserBalanceLedgerPage{Items: make([]UserBalanceLedgerItem, 0), Page: page, PageSize: pageSize}
	if err = ScanBalanceRow(ctx, exec, `SELECT COUNT(*) FROM user_balance_ledgers WHERE user_id=$1`, []any{userID}, &result.Total); err != nil {
		return nil, err
	}
	if result.ActiveBonus, err = ActiveRechargeBonusTotal(ctx, exec, userID); err != nil {
		return nil, err
	}
	var nextExpiry sql.NullTime
	if scanErr := ScanBalanceRow(ctx, exec, `
		SELECT MIN(expires_at) FROM recharge_bonus_grants
		WHERE user_id=$1 AND status='active' AND remaining_amount>0 AND expires_at>NOW()
	`, []any{userID}, &nextExpiry); scanErr == nil && nextExpiry.Valid {
		result.NextBonusExpiryAt = &nextExpiry.Time
	}

	rows, err := exec.QueryContext(ctx, `
		SELECT id,event_type,amount,balance_before,balance_after,bonus_before,bonus_after,
		       source_type,source_id,description,metadata,created_at
		FROM user_balance_ledgers
		WHERE user_id=$1
		ORDER BY created_at DESC,id DESC
		LIMIT $2 OFFSET $3
	`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item UserBalanceLedgerItem
		if err = rows.Scan(&item.ID, &item.EventType, &item.Amount, &item.BalanceBefore, &item.BalanceAfter,
			&item.BonusBefore, &item.BonusAfter, &item.SourceType, &item.SourceID, &item.Description,
			&item.Metadata, &item.CreatedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		result.Items = append(result.Items, item)
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
