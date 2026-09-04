package securityaudit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisPromptDecisionCacheStoresDecisionOnlyWithTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := &redisPromptDecisionCache{client: client}
	key := "prompt-guard-cache-test"
	decision := &PromptDecision{
		Kind: DecisionBlock, ErrorCode: ErrorCodeBlocked,
		Result: &NormalizedResult{Decision: EventCritical, RiskLevel: RiskCritical, Action: ActionBlock},
	}
	require.NoError(t, cache.Set(context.Background(), key, decision, promptDecisionBlockTTL))
	value, hit, err := cache.Get(context.Background(), key)
	require.NoError(t, err)
	require.True(t, hit)
	require.Equal(t, DecisionBlock, value.Kind)
	raw, err := server.Get(key)
	require.NoError(t, err)
	require.NotContains(t, raw, "raw prompt")
	ttl, err := client.TTL(context.Background(), key).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, 23*time.Hour)
}

func TestPromptDecisionCacheKeyUsesScanTextWhenPromptHashMissing(t *testing.T) {
	cfg := ActiveConfig{ConfigVersion: 1, Scanners: []string{"pii"}}
	first := promptDecisionCacheKey(cfg, PromptSnapshot{ScanText: "first"}, "rules")
	second := promptDecisionCacheKey(cfg, PromptSnapshot{ScanText: "second"}, "rules")
	require.NotEqual(t, first, second)
	third := promptDecisionCacheKey(cfg, PromptSnapshot{ScanText: "first"}, "rules")
	require.Equal(t, first, third)
}
