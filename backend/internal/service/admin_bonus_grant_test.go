package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAdminBonusGrantSortsAndDeduplicatesUsers(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	got, err := normalizeAdminBonusGrant(AdminBonusGrantInput{
		UserIDs: []int64{9, 3, 9}, Amount: 1.25,
		ExpiresAt: now.Add(24 * time.Hour), CampaignID: " weekend ",
		OperationID: "operation-1", GrantedBy: 7,
	}, now)
	require.NoError(t, err)
	require.Equal(t, []int64{3, 9}, got.UserIDs)
	require.Equal(t, "weekend", got.CampaignID)
	require.Equal(t, "7:operation-1", got.SourceID)
}

func TestNormalizeAdminBonusGrantRejectsExpiredAndOversizedBatches(t *testing.T) {
	now := time.Now().UTC()
	_, err := normalizeAdminBonusGrant(AdminBonusGrantInput{
		UserIDs: []int64{1}, Amount: 1, ExpiresAt: now,
		CampaignID: "weekend", OperationID: "operation-1", GrantedBy: 7,
	}, now)
	require.Error(t, err)

	userIDs := make([]int64, maxAdminBonusGrantUsers+1)
	for i := range userIDs {
		userIDs[i] = int64(i + 1)
	}
	_, err = normalizeAdminBonusGrant(AdminBonusGrantInput{
		UserIDs: userIDs, Amount: 1, ExpiresAt: now.Add(time.Hour),
		CampaignID: "weekend", OperationID: "operation-2", GrantedBy: 7,
	}, now)
	require.Error(t, err)
}

func TestNormalizeAdminBonusGrantRejectsAmountOutsideLedgerPrecision(t *testing.T) {
	now := time.Now().UTC()
	for _, amount := range []float64{0, maxAdminBonusGrantAmount + 1, 1.000000001} {
		_, err := normalizeAdminBonusGrant(AdminBonusGrantInput{
			UserIDs: []int64{1}, Amount: amount, ExpiresAt: now.Add(time.Hour),
			CampaignID: "weekend", OperationID: "operation-amount", GrantedBy: 7,
		}, now)
		require.Error(t, err)
	}
}

func TestRecoverAdminBonusGrantAcceptsExactCommittedRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	expiresAt := time.Date(2026, 8, 30, 15, 59, 0, 0, time.UTC)
	input := &normalizedAdminBonusGrant{
		AdminBonusGrantInput: AdminBonusGrantInput{
			UserIDs: []int64{3, 9}, Amount: 2.5, ExpiresAt: expiresAt,
			CampaignID: "weekend", Notes: "周末活动", OperationID: "operation-3", GrantedBy: 7,
		},
		SourceID: "7:operation-3",
	}
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT user_id,granted_amount,expires_at,campaign_id,notes,granted_by
		FROM recharge_bonus_grants
		WHERE source_type='admin_campaign' AND source_id=$1
		ORDER BY user_id FOR UPDATE
	`)).WithArgs(input.SourceID).WillReturnRows(
		sqlmock.NewRows([]string{"user_id", "granted_amount", "expires_at", "campaign_id", "notes", "granted_by"}).
			AddRow(int64(3), 2.5, expiresAt, "weekend", "周末活动", int64(7)).
			AddRow(int64(9), 2.5, expiresAt, "weekend", "周末活动", int64(7)),
	)

	recovered, err := recoverAdminBonusGrant(context.Background(), db, input)
	require.NoError(t, err)
	require.True(t, recovered)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecoverAdminBonusGrantRejectsChangedPayload(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	expiresAt := time.Date(2026, 8, 30, 15, 59, 0, 0, time.UTC)
	input := &normalizedAdminBonusGrant{
		AdminBonusGrantInput: AdminBonusGrantInput{
			UserIDs: []int64{3}, Amount: 2.5, ExpiresAt: expiresAt,
			CampaignID: "weekend", OperationID: "operation-4", GrantedBy: 7,
		},
		SourceID: "7:operation-4",
	}
	mock.ExpectQuery("SELECT user_id,granted_amount").WithArgs(input.SourceID).WillReturnRows(
		sqlmock.NewRows([]string{"user_id", "granted_amount", "expires_at", "campaign_id", "notes", "granted_by"}).
			AddRow(int64(3), 9.0, expiresAt, "weekend", "", int64(7)),
	)

	recovered, err := recoverAdminBonusGrant(context.Background(), db, input)
	require.Error(t, err)
	require.False(t, recovered)
	require.NoError(t, mock.ExpectationsWereMet())
}
