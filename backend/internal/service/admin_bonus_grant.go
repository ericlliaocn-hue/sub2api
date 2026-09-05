package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

const (
	maxAdminBonusGrantUsers  = 500
	maxAdminBonusGrantAmount = 999_999_999_999
)

type AdminBonusGrantInput struct {
	UserIDs     []int64
	Amount      float64
	ExpiresAt   time.Time
	CampaignID  string
	Notes       string
	OperationID string
	GrantedBy   int64
}

type AdminBonusGrantResult struct {
	Affected     int       `json:"affected"`
	Amount       float64   `json:"amount"`
	TotalGranted float64   `json:"total_granted"`
	ExpiresAt    time.Time `json:"expires_at"`
	CampaignID   string    `json:"campaign_id"`
	OperationID  string    `json:"operation_id"`
	Recovered    bool      `json:"recovered"`
}

type normalizedAdminBonusGrant struct {
	AdminBonusGrantInput
	SourceID string
}

func normalizeAdminBonusGrant(input AdminBonusGrantInput, now time.Time) (*normalizedAdminBonusGrant, error) {
	if len(input.UserIDs) == 0 || len(input.UserIDs) > maxAdminBonusGrantUsers {
		return nil, infraerrors.BadRequest("INVALID_BONUS_GRANT_USERS", fmt.Sprintf("user_ids must contain between 1 and %d users", maxAdminBonusGrantUsers))
	}
	if math.IsNaN(input.Amount) || math.IsInf(input.Amount, 0) || input.Amount <= 0 || input.Amount > maxAdminBonusGrantAmount || !decimal.NewFromFloat(input.Amount).Equal(decimal.NewFromFloat(input.Amount).Round(8)) {
		return nil, infraerrors.BadRequest("INVALID_BONUS_GRANT_AMOUNT", "bonus amount must be positive, no greater than 999999999999, and use at most 8 decimal places")
	}
	if input.ExpiresAt.IsZero() || !input.ExpiresAt.After(now) {
		return nil, infraerrors.BadRequest("INVALID_BONUS_GRANT_EXPIRY", "bonus expiration time must be in the future")
	}
	if input.ExpiresAt.After(now.AddDate(10, 0, 0)) {
		return nil, infraerrors.BadRequest("INVALID_BONUS_GRANT_EXPIRY", "bonus expiration time cannot be more than 10 years in the future")
	}

	campaignID := strings.TrimSpace(input.CampaignID)
	if campaignID == "" || utf8.RuneCountInString(campaignID) > 100 {
		return nil, infraerrors.BadRequest("INVALID_BONUS_GRANT_CAMPAIGN", "campaign_id is required and cannot exceed 100 characters")
	}
	notes := strings.TrimSpace(input.Notes)
	if utf8.RuneCountInString(notes) > 255 {
		return nil, infraerrors.BadRequest("INVALID_BONUS_GRANT_NOTES", "notes cannot exceed 255 characters")
	}
	operationID := strings.TrimSpace(input.OperationID)
	if operationID == "" || utf8.RuneCountInString(operationID) > 64 {
		return nil, infraerrors.BadRequest("INVALID_BONUS_GRANT_OPERATION", "idempotency key is required and cannot exceed 64 characters")
	}
	if input.GrantedBy <= 0 {
		return nil, infraerrors.BadRequest("INVALID_BONUS_GRANT_ACTOR", "administrator identity is required")
	}

	userIDs := append([]int64(nil), input.UserIDs...)
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	unique := userIDs[:0]
	for _, userID := range userIDs {
		if userID <= 0 {
			return nil, infraerrors.BadRequest("INVALID_BONUS_GRANT_USERS", "user_ids must contain positive user ids")
		}
		if len(unique) == 0 || unique[len(unique)-1] != userID {
			unique = append(unique, userID)
		}
	}

	expiresAt := input.ExpiresAt.UTC().Truncate(time.Second)
	return &normalizedAdminBonusGrant{
		AdminBonusGrantInput: AdminBonusGrantInput{
			UserIDs: unique, Amount: decimal.NewFromFloat(input.Amount).Round(8).InexactFloat64(),
			ExpiresAt: expiresAt, CampaignID: campaignID, Notes: notes,
			OperationID: operationID, GrantedBy: input.GrantedBy,
		},
		SourceID: fmt.Sprintf("%d:%s", input.GrantedBy, operationID),
	}, nil
}

func (s *adminServiceImpl) GrantExpiringBonus(ctx context.Context, input AdminBonusGrantInput) (*AdminBonusGrantResult, error) {
	if s == nil || s.entClient == nil {
		return nil, errors.New("admin bonus grant service unavailable")
	}
	normalized, err := normalizeAdminBonusGrant(input, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	exec := tx.Client()

	recovered, err := recoverAdminBonusGrant(ctx, exec, normalized)
	if err != nil {
		return nil, err
	}
	if recovered {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return adminBonusGrantResult(normalized, true), nil
	}

	balances, err := lockAdminBonusGrantUsers(ctx, exec, normalized.UserIDs)
	if err != nil {
		return nil, err
	}
	if len(balances) != len(normalized.UserIDs) {
		return nil, infraerrors.BadRequest("BONUS_GRANT_USER_NOT_FOUND", "one or more selected users do not exist or were deleted")
	}

	for _, userID := range normalized.UserIDs {
		balanceBefore := balances[userID]
		bonusBefore, err := ActiveRechargeBonusTotal(ctx, exec, userID)
		if err != nil {
			return nil, err
		}
		var grantID int64
		if err := ScanBalanceRow(ctx, exec, `
			INSERT INTO recharge_bonus_grants (
				user_id,payment_order_id,campaign_id,payment_amount,base_credited_amount,
				granted_amount,remaining_amount,currency,expires_at,status,
				source_type,source_id,granted_by,notes
			) VALUES ($1,NULL,$2,0,0,$3,$3,'USD',$4,'active','admin_campaign',$5,$6,$7)
			RETURNING id
		`, []any{userID, normalized.CampaignID, normalized.Amount, normalized.ExpiresAt, normalized.SourceID, normalized.GrantedBy, normalized.Notes}, &grantID); err != nil {
			return nil, err
		}
		var balanceAfter float64
		if err := ScanBalanceRow(ctx, exec, `
			UPDATE users SET balance=balance+$1,updated_at=NOW()
			WHERE id=$2 AND deleted_at IS NULL RETURNING balance
		`, []any{normalized.Amount, userID}, &balanceAfter); err != nil {
			return nil, err
		}
		if err := RecordBalanceLedger(ctx, exec, BalanceLedgerEntry{
			UserID: userID, EventType: "admin_bonus_granted", Amount: normalized.Amount,
			BalanceBefore: balanceBefore, BalanceAfter: balanceAfter,
			BonusBefore: bonusBefore, BonusAfter: bonusBefore + normalized.Amount,
			SourceType: "admin_campaign", SourceID: fmt.Sprintf("%s:%d", normalized.SourceID, userID),
			Description: "管理员活动赠送余额",
			Metadata: map[string]any{
				"grant_id": grantID, "campaign_id": normalized.CampaignID,
				"expires_at": normalized.ExpiresAt, "granted_by": normalized.GrantedBy,
				"operation_id": normalized.OperationID, "notes": normalized.Notes,
			},
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.invalidateGrantedBonusUsers(normalized.UserIDs)
	return adminBonusGrantResult(normalized, false), nil
}

func lockAdminBonusGrantUsers(ctx context.Context, exec BalanceSQLExecutor, userIDs []int64) (map[int64]float64, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT id,balance FROM users
		WHERE id=ANY($1) AND deleted_at IS NULL
		ORDER BY id FOR UPDATE
	`, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	balances := make(map[int64]float64, len(userIDs))
	for rows.Next() {
		var userID int64
		var balance float64
		if err := rows.Scan(&userID, &balance); err != nil {
			return nil, err
		}
		balances[userID] = balance
	}
	return balances, rows.Err()
}

func recoverAdminBonusGrant(ctx context.Context, exec BalanceSQLExecutor, input *normalizedAdminBonusGrant) (bool, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT user_id,granted_amount,expires_at,campaign_id,notes,granted_by
		FROM recharge_bonus_grants
		WHERE source_type='admin_campaign' AND source_id=$1
		ORDER BY user_id FOR UPDATE
	`, input.SourceID)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	existing := make(map[int64]struct{}, len(input.UserIDs))
	for rows.Next() {
		var userID, grantedBy int64
		var amount float64
		var expiresAt time.Time
		var campaignID, notes string
		if err := rows.Scan(&userID, &amount, &expiresAt, &campaignID, &notes, &grantedBy); err != nil {
			return false, err
		}
		if !decimal.NewFromFloat(amount).Equal(decimal.NewFromFloat(input.Amount)) ||
			!expiresAt.UTC().Truncate(time.Second).Equal(input.ExpiresAt) || campaignID != input.CampaignID ||
			notes != input.Notes || grantedBy != input.GrantedBy {
			return false, infraerrors.Conflict("BONUS_GRANT_IDEMPOTENCY_CONFLICT", "idempotency key was already used with different grant details")
		}
		existing[userID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if len(existing) == 0 {
		return false, nil
	}
	if len(existing) != len(input.UserIDs) {
		return false, infraerrors.Conflict("BONUS_GRANT_IDEMPOTENCY_CONFLICT", "idempotency key was already used with a different user selection")
	}
	for _, userID := range input.UserIDs {
		if _, ok := existing[userID]; !ok {
			return false, infraerrors.Conflict("BONUS_GRANT_IDEMPOTENCY_CONFLICT", "idempotency key was already used with a different user selection")
		}
	}
	return true, nil
}

func adminBonusGrantResult(input *normalizedAdminBonusGrant, recovered bool) *AdminBonusGrantResult {
	total := decimal.NewFromFloat(input.Amount).Mul(decimal.NewFromInt(int64(len(input.UserIDs)))).Round(8).InexactFloat64()
	return &AdminBonusGrantResult{
		Affected: len(input.UserIDs), Amount: input.Amount, TotalGranted: total,
		ExpiresAt: input.ExpiresAt, CampaignID: input.CampaignID,
		OperationID: input.OperationID, Recovered: recovered,
	}
}

func (s *adminServiceImpl) invalidateGrantedBonusUsers(userIDs []int64) {
	for _, userID := range userIDs {
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(context.Background(), userID)
		}
	}
	if s.billingCacheService == nil {
		return
	}
	ids := append([]int64(nil), userIDs...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for _, userID := range ids {
			if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
				return
			}
		}
	}()
}

var _ BalanceSQLExecutor = (*sql.DB)(nil)
