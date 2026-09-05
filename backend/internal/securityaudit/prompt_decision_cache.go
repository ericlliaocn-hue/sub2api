package securityaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	promptDecisionCachePrefix   = "sub2api:prompt_guard:decision:v1:"
	promptDecisionBlockTTL      = 24 * time.Hour
	promptDecisionFlagTTL       = 10 * time.Minute
	promptDecisionAllowTTL      = 5 * time.Minute
	maxPromptDecisionCacheBytes = 64 * 1024
)

type PromptDecisionCache interface {
	Get(ctx context.Context, key string) (*PromptDecision, bool, error)
	Set(ctx context.Context, key string, decision *PromptDecision, ttl time.Duration) error
}

type redisPromptDecisionCache struct {
	client *redis.Client
}

func newPromptDecisionCache(payload *RedisPayloadStore) PromptDecisionCache {
	if payload == nil || payload.client == nil {
		return nil
	}
	return &redisPromptDecisionCache{client: payload.client}
}

func (c *redisPromptDecisionCache) Get(ctx context.Context, key string) (*PromptDecision, bool, error) {
	if c == nil || c.client == nil || strings.TrimSpace(key) == "" {
		return nil, false, nil
	}
	raw, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(raw) == 0 || len(raw) > maxPromptDecisionCacheBytes {
		return nil, false, fmt.Errorf("prompt decision cache entry exceeds limit")
	}
	var decision PromptDecision
	if err := json.Unmarshal(raw, &decision); err != nil {
		return nil, false, err
	}
	if decision.Kind == DecisionUnavailable || decision.Kind == DecisionInvalid || decision.Result == nil {
		return nil, false, nil
	}
	return &decision, true, nil
}

func (c *redisPromptDecisionCache) Set(ctx context.Context, key string, decision *PromptDecision, ttl time.Duration) error {
	if c == nil || c.client == nil || decision == nil || decision.Result == nil || strings.TrimSpace(key) == "" {
		return nil
	}
	if decision.Kind == DecisionUnavailable || decision.Kind == DecisionInvalid {
		return nil
	}
	if ttl <= 0 {
		ttl = promptDecisionAllowTTL
	}
	raw, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	if len(raw) > maxPromptDecisionCacheBytes {
		return fmt.Errorf("prompt decision cache entry exceeds limit")
	}
	return c.client.Set(ctx, key, raw, ttl).Err()
}

func promptDecisionCacheKey(cfg ActiveConfig, snapshot PromptSnapshot, ruleDigest string) string {
	scannerDigest := sha256.Sum256([]byte(strings.Join(cfg.Scanners, ",")))
	promptHash := strings.TrimSpace(snapshot.PromptHash)
	// PromptHash is populated by snapshot extraction, but keep the cache safe
	// for callers constructing snapshots directly: an empty hash must never make
	// different prompt text share a decision.
	if promptHash == "" {
		fallback := sha256.Sum256([]byte(snapshot.ScanText))
		promptHash = hex.EncodeToString(fallback[:])
	}
	parts := []string{
		fmt.Sprintf("config=%d", cfg.ConfigVersion),
		"rules=" + ruleDigest,
		"model=" + snapshot.Model,
		"scanners=" + hex.EncodeToString(scannerDigest[:]),
		"prompt=" + promptHash,
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return promptDecisionCachePrefix + hex.EncodeToString(digest[:])
}

func promptDecisionCacheTTL(decision *PromptDecision) time.Duration {
	if decision == nil {
		return 0
	}
	switch decision.Kind {
	case DecisionBlock:
		return promptDecisionBlockTTL
	case DecisionFlag:
		return promptDecisionFlagTTL
	case DecisionAllow:
		return promptDecisionAllowTTL
	default:
		return 0
	}
}
