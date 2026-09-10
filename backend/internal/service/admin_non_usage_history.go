package service

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// Keep legacy business records authoritative when the wallet contains a mirror.
// Exclude model accounting at the database boundary, before pagination.
const nonUsageHistorySQL = `WITH history AS (
 SELECT 'redeem:' || r.id AS identity, r.id, r.code, r.type AS kind,
 r.value AS amount, COALESCE(r.used_at,r.created_at) AS occurred_at,
 COALESCE(r.notes,'') AS notes,r.group_id,COALESCE(r.validity_days,0) AS validity_days
 FROM redeem_codes r WHERE r.used_by=$1
 UNION ALL
 SELECT 'affiliate:' || a.id, -a.id, 'AFF-' || a.id, 'affiliate_balance',
 a.amount, a.created_at, '',NULL::bigint,0 FROM user_affiliate_ledger a
 WHERE a.user_id=$1 AND a.action='transfer'
 UNION ALL
 SELECT 'ledger:' || l.id, -l.id, 'LEDGER-' || l.id, l.event_type,
 l.amount,l.created_at,concat_ws(' ',NULLIF(l.description,''),
 'source=' || l.source_type || ':' || l.source_id,
 CASE WHEN COALESCE(l.metadata->>'actor_user_id',l.metadata->>'granted_by') IS NOT NULL THEN 'actor=' || COALESCE(l.metadata->>'actor_user_id',l.metadata->>'granted_by') END,
 l.metadata->>'notes',l.metadata->>'previous_expires_at',l.metadata->>'expires_at',
 l.metadata->>'previous_status',l.metadata->>'status'),NULL::bigint,0
 FROM user_balance_ledgers l WHERE l.user_id=$1
 AND l.event_type NOT IN ('usage_charge','batch_hold','batch_capture','batch_release')
 AND NOT (l.source_type='redeem_code' AND EXISTS (
 SELECT 1 FROM redeem_codes r WHERE r.used_by=l.user_id
 AND (r.code=l.source_id OR r.id::text=l.source_id)))
 AND NOT (l.event_type='affiliate_balance' AND EXISTS (
 SELECT 1 FROM user_affiliate_ledger a WHERE a.user_id=l.user_id
 AND a.action='transfer' AND a.id::text=l.source_id))
) `

func (s *adminServiceImpl) listNonUsageBalanceHistory(ctx context.Context, userID int64, page, size int, kind string) ([]RedeemCode, int64, float64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: size}
	var total int64
	if err := ScanBalanceRow(ctx, s.entClient, nonUsageHistorySQL+`SELECT COUNT(*) FROM history WHERE ($2='' OR kind=$2)`, []any{userID, kind}, &total); err != nil {
		return nil, 0, 0, err
	}
	rows, err := s.entClient.QueryContext(ctx, nonUsageHistorySQL+`SELECT h.id,h.code,h.kind,h.amount,h.occurred_at,h.notes,h.group_id,h.validity_days,COALESCE(g.name,'') FROM history h LEFT JOIN groups g ON g.id=h.group_id WHERE ($2='' OR kind=$2) ORDER BY occurred_at DESC,identity DESC OFFSET $3 LIMIT $4`, userID, kind, params.Offset(), params.Limit())
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	result := make([]RedeemCode, 0)
	for rows.Next() {
		item := RedeemCode{UsedBy: &userID, Status: StatusUsed}
		var groupName string
		if err := rows.Scan(&item.ID, &item.Code, &item.Type, &item.Value, &item.CreatedAt, &item.Notes, &item.GroupID, &item.ValidityDays, &groupName); err != nil {
			return nil, 0, 0, err
		}
		if item.GroupID != nil {
			item.Group = &Group{ID: *item.GroupID, Name: groupName}
		}
		item.UsedAt = &item.CreatedAt
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}
	recharged, err := s.redeemCodeRepo.SumPositiveBalanceByUser(ctx, userID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("balance history totals: %w", err)
	}
	return result, total, recharged, nil
}
