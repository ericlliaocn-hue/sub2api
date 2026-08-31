package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromotionOperationsMigrationAddsAuditableCommissionFlow(t *testing.T) {
	content, err := FS.ReadFile("232_promotion_operations.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "commission_freeze_days")
	require.Contains(t, sql, "commission_rate NUMERIC(10,4) NULL")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS promotion_attribution_events")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS promotion_commission_ledger")
	require.Contains(t, sql, "UNIQUE (payment_order_id)")
	require.Contains(t, sql, "reversed_amount")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS promotion_commission_settlements")
	require.Contains(t, sql, "status IN ('draft', 'paid', 'cancelled')")
}
