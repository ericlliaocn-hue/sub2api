package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestResolvePoolForNewKey(t *testing.T) {
	probe := SubPool{ID: 2, Kind: domain.SubPoolKindProbe, Status: domain.SubPoolStatusHealthy, AccountIDs: []int64{34}}
	formal := SubPool{ID: 1, Kind: domain.SubPoolKindFormal, Status: domain.SubPoolStatusHealthy, AccountIDs: []int64{349}}
	pools := []SubPool{formal, probe}

	t.Run("user pin wins", func(t *testing.T) {
		user := int64(2)
		group := int64(1)
		got := resolvePoolForNewKey(pools, &user, &group)
		require.NotNil(t, got)
		require.Equal(t, int64(2), got.ID)
	})

	t.Run("group default when user has none", func(t *testing.T) {
		group := int64(1)
		got := resolvePoolForNewKey(pools, nil, &group)
		require.NotNil(t, got)
		require.Equal(t, int64(1), got.ID)
	})

	t.Run("falls back to probe when nothing pinned", func(t *testing.T) {
		got := resolvePoolForNewKey(pools, nil, nil)
		require.NotNil(t, got)
		require.Equal(t, int64(2), got.ID)
	})
}
