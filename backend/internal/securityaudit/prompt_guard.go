package securityaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

type GuardEvaluator struct {
	scanner  PromptScanner
	repo     JobRepository
	metrics  Metrics
	clock    Clock
	capacity *promptCapacity
	rules    *PromptGuardRuleStore
	cache    PromptDecisionCache
	flight   singleflight.Group
}

func NewGuardEvaluator(scanner PromptScanner, repo JobRepository, metrics Metrics) *GuardEvaluator {
	return newGuardEvaluatorWithCapacity(scanner, repo, metrics, newPromptCapacity(
		defaultPromptSyncGlobalLimit,
		defaultPromptSyncNodeLimit,
		defaultPromptAsyncGlobalLimit,
		defaultPromptAsyncNodeLimit,
	))
}

func newGuardEvaluatorWithDependencies(scanner PromptScanner, repo JobRepository, metrics Metrics, capacity *promptCapacity, rules *PromptGuardRuleStore, cache PromptDecisionCache) *GuardEvaluator {
	return &GuardEvaluator{scanner: scanner, repo: repo, metrics: metrics, clock: realClock{}, capacity: capacity, rules: rules, cache: cache}
}

func newGuardEvaluator(scanner PromptScanner, repo JobRepository, metrics Metrics, globalLimit, perNodeLimit int) *GuardEvaluator {
	return newGuardEvaluatorWithCapacity(scanner, repo, metrics, newPromptCapacity(
		globalLimit,
		perNodeLimit,
		defaultPromptAsyncGlobalLimit,
		defaultPromptAsyncNodeLimit,
	))

}

func newGuardEvaluatorWithCapacity(scanner PromptScanner, repo JobRepository, metrics Metrics, capacity *promptCapacity) *GuardEvaluator {
	return newGuardEvaluatorWithDependencies(scanner, repo, metrics, capacity, nil, nil)
}

func (g *GuardEvaluator) Evaluate(ctx context.Context, cfg ActiveConfig, snapshot PromptSnapshot) (*PromptDecision, error) {
	if g == nil {
		return nil, &GuardError{Code: ErrorCodeUnavailable}
	}
	start := g.clock.Now()
	baseFields := snapshotLogFields(snapshot)
	baseFields["config_version"] = cfg.ConfigVersion
	ruleSet, ruleErr := g.ruleSet(ctx)
	if ruleErr != nil {
		LogWarn(EventLocalRuleLoadFailed, mergeLogFields(baseFields, map[string]any{"status": "degraded", "error_code": "local_rule_load_failed"}))
	}
	if ruleSet != nil {
		if match, ok := ruleSet.Match(snapshot.ScanText); ok {
			return g.finishLocalRule(ctx, cfg, snapshot, match, ruleSet.Digest(), start)
		}
	}
	if !cfg.GuardEnabled {
		if g.metrics != nil {
			g.metrics.Observe(DecisionAllow, g.clock.Now().Sub(start))
		}
		LogInfo(EventGuardDisabled, mergeLogFields(baseFields, map[string]any{"status": "disabled", "upstream_dispatched": false, "billing_preconsumed": false}))
		return &PromptDecision{Kind: DecisionAllow, AllowNextStage: true}, nil
	}
	if g.scanner == nil {
		if g.metrics != nil {
			g.metrics.Observe(DecisionUnavailable, 0)
		}
		logGuardFailure(snapshot, cfg, DecisionUnavailable, ErrorCodeUnavailable, "", 0)
		return nil, &GuardError{Code: ErrorCodeUnavailable}
	}
	endpoints := cfg.EnabledEndpoints()
	if len(endpoints) == 0 {
		if g.metrics != nil {
			g.metrics.Observe(DecisionUnavailable, g.clock.Now().Sub(start))
		}
		logGuardFailure(snapshot, cfg, DecisionUnavailable, ErrorCodeUnavailable, "", g.clock.Now().Sub(start))
		return nil, &GuardError{Code: ErrorCodeUnavailable}
	}
	timeout := time.Duration(endpoints[0].TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = DefaultTimeoutMS * time.Millisecond
	}
	evalCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	inputLimit := minimumInputLimit(endpoints)
	chunks := SplitRunes(snapshot.ScanText, inputLimit)
	if len(chunks) == 0 {
		if g.metrics != nil {
			g.metrics.Observe(DecisionAllow, g.clock.Now().Sub(start))
		}
		return &PromptDecision{Kind: DecisionAllow, AllowNextStage: true}, nil
	}
	cacheKey := promptDecisionCacheKey(cfg, snapshot, ruleDigest(ruleSet))
	if cached, ok := g.cachedDecision(ctx, cacheKey); ok {
		cached.Result.LatencyMS = int(g.clock.Now().Sub(start).Milliseconds())
		if cached.Result.LatencyMS < 0 {
			cached.Result.LatencyMS = 0
		}
		if g.metrics != nil {
			g.metrics.Observe(cached.Kind, g.clock.Now().Sub(start))
		}
		g.recordDecision(ctx, cfg, snapshot, cached, baseFields, true)
		return cached, nil
	}
	LogInfo(EventEvaluationStarted, mergeLogFields(baseFields, map[string]any{"chunk_total": len(chunks), "status": "started"}))
	results := make([]*NormalizedResult, 0, len(chunks))
	for index, chunk := range chunks {
		chunkStarted := g.clock.Now()
		LogInfo(EventChunkStarted, mergeLogFields(baseFields, map[string]any{
			"chunk_index": index + 1, "chunk_total": len(chunks),
			"chunk_chars": len([]rune(chunk)), "input_chars": snapshot.PromptLength, "input_limit": inputLimit,
			"status": "started",
		}))
		result, err := g.scanChunk(evalCtx, cfg, endpoints, chunk)
		if err != nil {
			code := guardErrorCode(err)
			LogWarn(EventChunkFailed, mergeLogFields(baseFields, map[string]any{
				"chunk_index": index + 1, "chunk_total": len(chunks),
				"chunk_chars": len([]rune(chunk)), "input_chars": snapshot.PromptLength, "input_limit": inputLimit,
				"latency_ms": g.clock.Now().Sub(chunkStarted).Milliseconds(), "error_code": code, "status": "failed",
			}))
			kind := DecisionUnavailable
			if code == ErrorCodeInvalidResponse {
				kind = DecisionInvalid
			}
			if g.metrics != nil {
				g.metrics.Observe(kind, g.clock.Now().Sub(start))
				var guardErr *GuardError
				if errors.As(err, &guardErr) && guardErr.Timeout {
					g.metrics.IncTimeout()
				}
			}
			logGuardFailure(snapshot, cfg, kind, code, "", g.clock.Now().Sub(start))
			return nil, err
		}
		result.ChunkTotal = len(chunks)
		results = append(results, result)
		LogInfo(EventChunkCompleted, mergeLogFields(baseFields, map[string]any{
			"chunk_index": index + 1, "chunk_total": len(chunks),
			"chunk_chars": len([]rune(chunk)), "input_chars": snapshot.PromptLength, "input_limit": inputLimit,
			"guard_endpoint_id": result.GuardEndpointID, "action": result.Action,
			"latency_ms": g.clock.Now().Sub(chunkStarted).Milliseconds(), "status": "completed",
		}))
		if result.Action == ActionBlock {
			break
		}
	}
	aggregated, err := AggregateResults(results, g.clock.Now().Sub(start))
	if err != nil {
		if g.metrics != nil {
			g.metrics.Observe(DecisionInvalid, g.clock.Now().Sub(start))
		}
		logGuardFailure(snapshot, cfg, DecisionInvalid, ErrorCodeInvalidResponse, "", g.clock.Now().Sub(start))
		return nil, &GuardError{Code: ErrorCodeInvalidResponse, Cause: err}
	}
	aggregated.ChunkTotal = len(chunks)
	kind := DecisionAllow
	if aggregated.Action == ActionWarn {
		kind = DecisionFlag
	}
	if aggregated.Action == ActionBlock {
		kind = DecisionBlock
	}
	decision := &PromptDecision{Kind: kind, Result: aggregated, AllowNextStage: kind == DecisionAllow || kind == DecisionFlag}
	if kind == DecisionBlock {
		decision.ErrorCode = ErrorCodeBlocked
	}
	if g.cache != nil {
		if err := g.cache.Set(ctx, cacheKey, decision, promptDecisionCacheTTL(decision)); err != nil {
			LogWarn(EventDecisionCacheFailed, mergeLogFields(baseFields, map[string]any{"error_code": "decision_cache_write_failed", "status": "degraded"}))
		}
	}
	if g.metrics != nil {
		g.metrics.Observe(kind, g.clock.Now().Sub(start))
	}
	LogInfo(EventChunksAggregated, mergeLogFields(baseFields, map[string]any{
		"decision":   kind,
		"risk_level": aggregated.RiskLevel, "action": aggregated.Action, "chunk_total": aggregated.ChunkTotal,
		"latency_ms": aggregated.LatencyMS, "guard_endpoint_id": aggregated.GuardEndpointID, "stage": snapshot.Stage,
		"status": "completed",
	}))
	if g.repo != nil {
		if _, recordErr := g.repo.RecordBlocking(ctx, snapshot.Redacted(), cfg.ConfigVersion, aggregated, cfg.StorePassEvents); recordErr != nil {
			if g.metrics != nil {
				g.metrics.IncRecordFailed()
			}
			LogWarn(EventResultRecordFailed, mergeLogFields(baseFields, map[string]any{
				"decision": kind, "error_code": "result_record_failed", "stage": snapshot.Stage,
				"status": "failed",
			}))
		}
	}
	if kind == DecisionBlock {
		LogWarn(EventGuardBlocked, mergeLogFields(baseFields, map[string]any{
			"guard_endpoint_id": aggregated.GuardEndpointID,
			"decision":          kind, "risk_level": aggregated.RiskLevel, "action": aggregated.Action, "chunk_total": aggregated.ChunkTotal,
			"latency_ms": aggregated.LatencyMS, "status": "blocked", "error_code": ErrorCodeBlocked,
			"stage": snapshot.Stage, "upstream_dispatched": false, "billing_preconsumed": false,
		}))
	} else {
		LogInfo(EventGuardAllowed, mergeLogFields(baseFields, map[string]any{
			"decision": kind, "risk_level": aggregated.RiskLevel, "action": aggregated.Action,
			"guard_endpoint_id": aggregated.GuardEndpointID, "chunk_total": aggregated.ChunkTotal,
			"latency_ms": aggregated.LatencyMS, "stage": snapshot.Stage, "status": "allowed",
		}))
	}
	return decision, nil
}

func (g *GuardEvaluator) ruleSet(ctx context.Context) (*PromptGuardRuleSet, error) {
	if g == nil || g.rules == nil {
		return nil, nil
	}
	rulesCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	return g.rules.Snapshot(rulesCtx)
}

func ruleDigest(set *PromptGuardRuleSet) string {
	if set == nil {
		return "none"
	}
	return set.Digest()
}

func (g *GuardEvaluator) cachedDecision(ctx context.Context, key string) (*PromptDecision, bool) {
	if g == nil || g.cache == nil {
		return nil, false
	}
	decision, ok, err := g.cache.Get(ctx, key)
	if err != nil {
		if g.metrics != nil {
			g.metrics.IncCacheMiss()
		}
		return nil, false
	}
	if g.metrics != nil {
		if ok {
			g.metrics.IncCacheHit()
		} else {
			g.metrics.IncCacheMiss()
		}
	}
	return decision, ok
}

func (g *GuardEvaluator) finishLocalRule(ctx context.Context, cfg ActiveConfig, snapshot PromptSnapshot, match PromptGuardRuleMatch, digest string, start time.Time) (*PromptDecision, error) {
	rule := match.Rule
	decisionKind := DecisionBlock
	action := ActionBlock
	decision := EventCritical
	if rule.Action == PromptGuardRuleActionFlag {
		decisionKind = DecisionFlag
		action = ActionWarn
		decision = EventFlag
	}
	result := &NormalizedResult{
		Decision: decision, RiskLevel: rule.Severity, Action: action, Safety: "Unsafe",
		Categories: []string{rule.Category}, MatchedScanners: []string{"local_rule"},
		ScannerScores:   map[string]float64{rule.Category: 1},
		ScannerEvidence: map[string]string{rule.Category: rule.Name, "local_rule": rule.Name},
		ScannerBackend:  "local_rules", ScannerVersion: digest, PolicyID: rule.RuleKey,
		PolicyVersion: 1, ChunkTotal: 1, LatencyMS: int(g.clock.Now().Sub(start).Milliseconds()),
	}
	if result.LatencyMS < 0 {
		result.LatencyMS = 0
	}
	promptDecision := &PromptDecision{Kind: decisionKind, Result: result, AllowNextStage: decisionKind != DecisionBlock}
	if decisionKind == DecisionBlock {
		promptDecision.ErrorCode = ErrorCodeBlocked
	}
	if g.metrics != nil {
		g.metrics.Observe(decisionKind, g.clock.Now().Sub(start))
		if decisionKind == DecisionBlock {
			g.metrics.IncLocalRuleBlock()
		}
	}
	baseFields := snapshotLogFields(snapshot)
	baseFields["config_version"] = cfg.ConfigVersion
	baseFields["rule_key"] = rule.RuleKey
	baseFields["rule_id"] = rule.ID
	baseFields["local_rule"] = true
	g.recordDecision(ctx, cfg, snapshot, promptDecision, baseFields, false)
	if decisionKind == DecisionBlock {
		LogWarn(EventLocalRuleBlocked, mergeLogFields(baseFields, map[string]any{
			"decision": decisionKind, "risk_level": result.RiskLevel, "action": result.Action,
			"error_code": ErrorCodeBlocked, "upstream_dispatched": false, "billing_preconsumed": false, "status": "blocked",
		}))
	} else {
		LogInfo(EventLocalRuleFlagged, mergeLogFields(baseFields, map[string]any{"decision": decisionKind, "risk_level": result.RiskLevel, "action": result.Action, "status": "flagged"}))
	}
	if g.cache != nil {
		if err := g.cache.Set(ctx, promptDecisionCacheKey(cfg, snapshot, digest), promptDecision, promptDecisionCacheTTL(promptDecision)); err != nil {
			LogWarn(EventDecisionCacheFailed, mergeLogFields(baseFields, map[string]any{"error_code": "decision_cache_write_failed", "status": "degraded"}))
		}
	}
	return promptDecision, nil
}

func (g *GuardEvaluator) recordDecision(ctx context.Context, cfg ActiveConfig, snapshot PromptSnapshot, decision *PromptDecision, baseFields map[string]any, cacheHit bool) {
	if decision == nil || decision.Result == nil {
		return
	}
	if cacheHit {
		LogInfo(EventDecisionCacheHit, mergeLogFields(baseFields, map[string]any{"cache_hit": true, "decision": decision.Kind, "status": "hit"}))
	}
	if g.repo == nil {
		return
	}
	if _, recordErr := g.repo.RecordBlocking(ctx, snapshot.Redacted(), cfg.ConfigVersion, decision.Result, cfg.StorePassEvents); recordErr != nil {
		if g.metrics != nil {
			g.metrics.IncRecordFailed()
		}
		LogWarn(EventResultRecordFailed, mergeLogFields(baseFields, map[string]any{"decision": decision.Kind, "error_code": "result_record_failed", "stage": snapshot.Stage, "status": "failed"}))
	}
}

func logGuardFailure(snapshot PromptSnapshot, cfg ActiveConfig, kind DecisionKind, code, guardEndpointID string, latency time.Duration) {
	fields := snapshotLogFields(snapshot)
	fields["config_version"] = cfg.ConfigVersion
	LogWarn(EventGuardFailed, mergeLogFields(fields, map[string]any{
		"decision": kind, "guard_endpoint_id": guardEndpointID, "latency_ms": latency.Milliseconds(),
		"status": "failed", "error_code": code, "upstream_dispatched": false, "billing_preconsumed": false,
	}))
}

func (g *GuardEvaluator) scanChunk(ctx context.Context, cfg ActiveConfig, endpoints []ActiveEndpoint, chunk string) (*NormalizedResult, error) {
	var lastErr error
	capacity := g.capacity
	if capacity == nil {
		capacity = newPromptCapacity(defaultPromptSyncGlobalLimit, defaultPromptSyncNodeLimit, defaultPromptAsyncGlobalLimit, defaultPromptAsyncNodeLimit)
	}
	for index, endpoint := range capacity.OrderEndpoints(cfg.Strategy, endpoints) {
		key := guardScanFlightKey(cfg, endpoint, chunk)
		result, err := g.sharedScan(ctx, key, time.Duration(endpoint.TimeoutMS)*time.Millisecond, func(sharedCtx context.Context) (*NormalizedResult, error) {
			release, acquired := capacity.AcquireSync(sharedCtx, endpoint.ID)
			if !acquired {
				if g.metrics != nil {
					g.metrics.IncBulkheadFull()
				}
				return nil, &GuardError{Code: ErrorCodeUnavailable, Retryable: true, Timeout: errors.Is(sharedCtx.Err(), context.DeadlineExceeded), Cause: sharedCtx.Err()}
			}
			defer release()
			return callPromptScanner(sharedCtx, g.scanner, endpoint, chunk, cfg.Scanners)
		})
		if err == nil && result != nil {
			return result, nil
		}
		if err == nil {
			err = &GuardError{Code: ErrorCodeInvalidResponse, Retryable: false}
		}
		lastErr = err
		var guardErr *GuardError
		if !errors.As(err, &guardErr) || !guardErr.Retryable {
			return nil, err
		}
		if index < len(endpoints)-1 && g.metrics != nil {
			g.metrics.IncFailover()
		}
	}
	if lastErr == nil {
		lastErr = &GuardError{Code: ErrorCodeUnavailable}
	}
	return nil, lastErr
}

func (g *GuardEvaluator) sharedScan(ctx context.Context, key string, timeout time.Duration, fn func(context.Context) (*NormalizedResult, error)) (*NormalizedResult, error) {
	if g == nil || fn == nil {
		return nil, &GuardError{Code: ErrorCodeUnavailable}
	}
	if timeout <= 0 {
		timeout = DefaultTimeoutMS * time.Millisecond
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	resultCh := g.flight.DoChan(key, func() (any, error) {
		sharedCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return fn(sharedCtx)
	})
	var result singleflight.Result
	select {
	case result = <-resultCh:
	case <-ctx.Done():
		return nil, &GuardError{Code: ErrorCodeUnavailable, Retryable: true, Timeout: errors.Is(ctx.Err(), context.DeadlineExceeded), Cause: ctx.Err()}
	}
	if result.Err != nil {
		return nil, result.Err
	}
	value, ok := result.Val.(*NormalizedResult)
	if !ok || value == nil {
		return nil, &GuardError{Code: ErrorCodeInvalidResponse}
	}
	return cloneNormalizedResult(value), nil
}

func cloneNormalizedResult(result *NormalizedResult) *NormalizedResult {
	if result == nil {
		return nil
	}
	clone := *result
	clone.Categories = append([]string(nil), result.Categories...)
	clone.MatchedScanners = append([]string(nil), result.MatchedScanners...)
	clone.UnknownCategories = append([]string(nil), result.UnknownCategories...)
	clone.ScannerScores = make(map[string]float64, len(result.ScannerScores))
	for key, value := range result.ScannerScores {
		clone.ScannerScores[key] = value
	}
	clone.ScannerEvidence = make(map[string]string, len(result.ScannerEvidence))
	for key, value := range result.ScannerEvidence {
		clone.ScannerEvidence[key] = value
	}
	return &clone
}

func guardScanFlightKey(cfg ActiveConfig, endpoint ActiveEndpoint, chunk string) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(fmt.Sprintf("%d\x00%s\x00%s\x00", cfg.ConfigVersion, endpoint.ID, endpoint.Model)))
	_, _ = digest.Write([]byte(strings.Join(cfg.Scanners, ",")))
	_, _ = digest.Write([]byte("\x00"))
	_, _ = digest.Write([]byte(chunk))
	return "prompt_guard_scan:" + hex.EncodeToString(digest.Sum(nil))
}

func callPromptScanner(ctx context.Context, scanner PromptScanner, endpoint ActiveEndpoint, chunk string, scanners []string) (result *NormalizedResult, err error) {
	defer func() {
		if recover() != nil {
			result = nil
			err = &GuardError{Code: ErrorCodeUnavailable, Retryable: false}
		}
	}()
	return scanner.Scan(ctx, endpoint, chunk, scanners)
}

func minimumInputLimit(endpoints []ActiveEndpoint) int {
	limit := DefaultInputLimit
	for index, endpoint := range endpoints {
		value := endpoint.InputLimit
		if value <= 0 {
			value = DefaultInputLimit
		}
		if index == 0 || value < limit {
			limit = value
		}
	}
	return limit
}

func guardErrorCode(err error) string {
	var guardErr *GuardError
	if errors.As(err, &guardErr) && guardErr.Code != "" {
		return guardErr.Code
	}
	return ErrorCodeUnavailable
}

func pointerLogID(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
