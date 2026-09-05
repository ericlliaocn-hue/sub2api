package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration231GeneralizesRechargeBonusLotsForAdminCampaigns(t *testing.T) {
	content, err := FS.ReadFile("231_admin_expiring_bonus_grants.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ALTER COLUMN payment_order_id DROP NOT NULL")
	require.Contains(t, sql, "source_type VARCHAR(40) NOT NULL DEFAULT 'payment_order'")
	require.Contains(t, sql, "source_id VARCHAR(128) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "granted_by BIGINT REFERENCES users(id) ON DELETE SET NULL")
	require.Contains(t, sql, "recharge_bonus_grants_source_user_unique_idx")
	require.Contains(t, sql, "WHERE source_id <> ''")
	require.Contains(t, sql, "recharge_bonus_grants_expiry_sweep_idx")
}
