package securityaudit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scriptedScanner struct {
	mu      sync.Mutex
	calls   []string
	block   <-chan struct{}
	entered chan<- struct{}
}

func (s *scriptedScanner) Scan(ctx context.Context, endpoint ActiveEndpoint, _ string, _ []string) (*NormalizedResult, error) {
	s.mu.Lock()
	s.calls = append(s.calls, endpoint.ID)
	s.mu.Unlock()
	if s.entered != nil {
		select {
		case s.entered <- struct{}{}:
		default:
		}
	}
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return nil, &GuardError{Code: ErrorCodeUnavailable, Retryable: true, Timeout: true, Cause: ctx.Err()}
		}
	}
	if endpoint.ID == "bad" {
		return nil, &GuardError{Code: ErrorCodeUnavailable, Retryable: true}
	}
	if endpoint.ID == "invalid" {
		return nil, &GuardError{Code: ErrorCodeInvalidResponse}
	}
	return &NormalizedResult{Decision: EventPass, RiskLevel: RiskLow, Action: ActionAllow, Safety: "Safe", ScannerScores: map[string]float64{}, ScannerEvidence: map[string]string{}, GuardEndpointID: endpoint.ID}, nil
}

func guardConfig(endpoints ...ActiveEndpoint) ActiveConfig {
	return ActiveConfig{RiskControlEnabled: true, Enabled: true, GuardEnabled: true, BlockingEnabled: true, ConfigVersion: 2, Scanners: AllScannerIDs, Endpoints: endpoints}
}

func TestNewGuardEvaluatorUsesDedicatedGuardConcurrencyDefaults(t *testing.T) {
	evaluator := NewGuardEvaluator(&scriptedScanner{}, nil, nil)
	require.NotNil(t, evaluator.capacity)
	require.Equal(t, defaultPromptSyncGlobalLimit, evaluator.capacity.globalLimit)
	require.Equal(t, defaultPromptSyncNodeLimit, evaluator.capacity.nodeLimit)
	require.Equal(t, defaultPromptAsyncGlobalLimit, evaluator.capacity.asyncGlobalLimit)
	require.Equal(t, defaultPromptAsyncNodeLimit, evaluator.capacity.asyncNodeLimit)
}

func TestGuardEvaluatorDisabledStillAppliesLocalRuleAndSkipsGuard(t *testing.T) {
	rules := &promptRuleTestRepo{rules: []PromptGuardRule{{
		ID: 1, RuleKey: "local", Name: "local", Category: "jailbreak", Severity: RiskHigh,
		PatternType: PromptGuardRulePatternLiteral, Pattern: "danger", Action: PromptGuardRuleActionBlock, Priority: 1, Enabled: true,
	}}}
	evaluator := newGuardEvaluatorWithDependencies(nil, nil, NewAtomicMetrics(), nil, NewPromptGuardRuleStore(rules), nil)
	cfg := guardConfig()
	cfg.GuardEnabled = false
	decision, err := evaluator.Evaluate(context.Background(), cfg, PromptSnapshot{ScanText: "danger", PromptLength: 6})
	require.NoError(t, err)
	require.Equal(t, DecisionBlock, decision.Kind)
	decision, err = evaluator.Evaluate(context.Background(), cfg, PromptSnapshot{ScanText: "clean", PromptLength: 5})
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, decision.Kind)
}

type promptDecisionTestCache struct {
	mu     sync.Mutex
	values map[string]*PromptDecision
	gets   int
	sets   int
}

func (c *promptDecisionTestCache) Get(_ context.Context, key string) (*PromptDecision, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets++
	value, ok := c.values[key]
	return value, ok, nil
}

func (c *promptDecisionTestCache) Set(_ context.Context, key string, decision *PromptDecision, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = map[string]*PromptDecision{}
	}
	c.values[key] = decision
	c.sets++
	return nil
}

func TestGuardEvaluatorLocalRuleBlocksBeforeGuard(t *testing.T) {
	rules := &promptRuleTestRepo{rules: []PromptGuardRule{{
		ID: 1, RuleKey: "sexual-literature", Name: "sexual literature", Category: "sexual_content_or_sexual_acts",
		Severity: RiskCritical, PatternType: PromptGuardRulePatternLiteral, Pattern: "色情小说",
		Action: PromptGuardRuleActionBlock, Priority: 1, Enabled: true,
	}}}
	evaluator := newGuardEvaluatorWithDependencies(nil, nil, NewAtomicMetrics(), nil, NewPromptGuardRuleStore(rules), nil)
	decision, err := evaluator.Evaluate(context.Background(), guardConfig(), PromptSnapshot{
		RequestID: "local-rule", PromptHash: "hash", ScanText: "请写色情小说", PromptLength: 7,
	})
	require.NoError(t, err)
	require.Equal(t, DecisionBlock, decision.Kind)
	require.False(t, decision.AllowNextStage)
	require.Equal(t, "sexual-literature", decision.Result.PolicyID)
	require.Equal(t, "local_rules", decision.Result.ScannerBackend)
}

func TestGuardEvaluatorDecisionCacheAvoidsRepeatedGuardCall(t *testing.T) {
	var calls int
	scanner := PromptScannerFunc(func(context.Context, ActiveEndpoint, string, []string) (*NormalizedResult, error) {
		calls++
		return &NormalizedResult{Decision: EventPass, RiskLevel: RiskLow, Action: ActionAllow,
			ScannerScores: map[string]float64{}, ScannerEvidence: map[string]string{}}, nil
	})
	cache := &promptDecisionTestCache{}
	evaluator := newGuardEvaluatorWithDependencies(scanner, nil, NewAtomicMetrics(), nil, nil, cache)
	cfg := guardConfig(ActiveEndpoint{ID: "guard", Model: DefaultGuardModel, Enabled: true, TimeoutMS: 1000, InputLimit: 100})
	snapshot := PromptSnapshot{PromptHash: "same", ScanText: "hello", PromptLength: 5}
	first, err := evaluator.Evaluate(context.Background(), cfg, snapshot)
	require.NoError(t, err)
	second, err := evaluator.Evaluate(context.Background(), cfg, snapshot)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, first.Kind)
	require.Equal(t, DecisionAllow, second.Kind)
	require.Equal(t, 1, calls)
	require.Equal(t, 1, cache.sets)
	require.Equal(t, 2, cache.gets)
}

func TestGuardEvaluatorMergesSameChunkInFlight(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	scanner := &scriptedScanner{block: release, entered: entered}
	evaluator := newGuardEvaluator(scanner, nil, NewAtomicMetrics(), 2, 2)
	cfg := guardConfig(ActiveEndpoint{ID: "guard", Model: DefaultGuardModel, Enabled: true, TimeoutMS: 2000, InputLimit: 100})
	snapshot := PromptSnapshot{PromptHash: "same-flight", ScanText: "same", PromptLength: 4}
	done := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := evaluator.Evaluate(context.Background(), cfg, snapshot)
			done <- err
		}()
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("guard scanner was not entered")
	}
	time.Sleep(30 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("duplicate request completed before shared scan release: %v", err)
	default:
	}
	close(release)
	require.NoError(t, <-done)
	require.NoError(t, <-done)
	scanner.mu.Lock()
	callCount := len(scanner.calls)
	scanner.mu.Unlock()
	require.Equal(t, 1, callCount)
}

func TestGuardEvaluatorLeastInflightDistributesConcurrentScans(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan string, 2)
	var mu sync.Mutex
	var calls []string
	scanner := PromptScannerFunc(func(ctx context.Context, endpoint ActiveEndpoint, _ string, _ []string) (*NormalizedResult, error) {
		mu.Lock()
		calls = append(calls, endpoint.ID)
		mu.Unlock()
		entered <- endpoint.ID
		select {
		case <-release:
			return integrationResult(EventPass), nil
		case <-ctx.Done():
			return nil, &GuardError{Code: ErrorCodeUnavailable, Retryable: true, Timeout: true, Cause: ctx.Err()}
		}
	})
	evaluator := newGuardEvaluator(scanner, nil, NewAtomicMetrics(), 4, 2)
	cfg := guardConfig(
		ActiveEndpoint{ID: "first", Enabled: true, TimeoutMS: 1000, InputLimit: 100},
		ActiveEndpoint{ID: "second", Enabled: true, TimeoutMS: 1000, InputLimit: 100},
	)
	cfg.Strategy = PromptAuditStrategyLeastInflight
	done := make(chan error, 2)
	go func() {
		_, err := evaluator.Evaluate(context.Background(), cfg, PromptSnapshot{PromptHash: "one", ScanText: "one", PromptLength: 3})
		done <- err
	}()
	seen := []string{<-entered}
	go func() {
		_, err := evaluator.Evaluate(context.Background(), cfg, PromptSnapshot{PromptHash: "two", ScanText: "two", PromptLength: 3})
		done <- err
	}()
	seen = append(seen, <-entered)
	close(release)
	require.NoError(t, <-done)
	require.NoError(t, <-done)
	mu.Lock()
	defer mu.Unlock()
	require.ElementsMatch(t, []string{"first", "second"}, calls)
	require.ElementsMatch(t, []string{"first", "second"}, seen)
}

func TestGuardEvaluatorOrderedFailoverAndInvalidTerminal(t *testing.T) {
	scanner := &scriptedScanner{}
	metrics := NewAtomicMetrics()
	evaluator := newGuardEvaluator(scanner, nil, metrics, 4, 2)
	snapshot := PromptSnapshot{RequestID: "r", ScanText: "hello", PromptLength: 5}
	decision, err := evaluator.Evaluate(context.Background(), guardConfig(
		ActiveEndpoint{ID: "bad", Enabled: true, TimeoutMS: 1000, InputLimit: 100},
		ActiveEndpoint{ID: "good", Enabled: true, TimeoutMS: 1000, InputLimit: 100},
	), snapshot)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, decision.Kind)
	require.Equal(t, int64(1), metrics.Snapshot().Failovers)
	_, err = evaluator.Evaluate(context.Background(), guardConfig(
		ActiveEndpoint{ID: "invalid", Enabled: true, TimeoutMS: 1000, InputLimit: 100},
		ActiveEndpoint{ID: "good", Enabled: true, TimeoutMS: 1000, InputLimit: 100},
	), snapshot)
	var guardErr *GuardError
	require.ErrorAs(t, err, &guardErr)
	require.Equal(t, ErrorCodeInvalidResponse, guardErr.Code)
	snapshotMetrics := metrics.Snapshot()
	require.Equal(t, int64(2), snapshotMetrics.Total)
	require.Equal(t, int64(1), snapshotMetrics.Allowed)
	require.Equal(t, int64(1), snapshotMetrics.Invalid)
}

func TestGuardEvaluatorGlobalCapacityWaitsUntilSharedDeadline(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	scanner := &scriptedScanner{block: release, entered: entered}
	metrics := NewAtomicMetrics()
	evaluator := newGuardEvaluator(scanner, nil, metrics, 1, 1)
	firstCfg := guardConfig(ActiveEndpoint{ID: "good", Enabled: true, TimeoutMS: 2000, InputLimit: 100})
	done := make(chan error, 1)
	go func() {
		_, err := evaluator.Evaluate(context.Background(), firstCfg, PromptSnapshot{ScanText: "one", PromptLength: 3})
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first evaluation did not enter scanner")
	}
	start := time.Now()
	secondCfg := guardConfig(ActiveEndpoint{ID: "good", Enabled: true, TimeoutMS: 60, InputLimit: 100})
	_, err := evaluator.Evaluate(context.Background(), secondCfg, PromptSnapshot{ScanText: "two", PromptLength: 3})
	require.Error(t, err)
	require.GreaterOrEqual(t, time.Since(start), 40*time.Millisecond)
	require.Less(t, time.Since(start), 500*time.Millisecond)
	require.Equal(t, int64(1), metrics.Snapshot().BulkheadFull)
	close(release)
	require.NoError(t, <-done)
	snapshotMetrics := metrics.Snapshot()
	require.Equal(t, int64(2), snapshotMetrics.Total)
	require.Equal(t, int64(1), snapshotMetrics.Allowed)
	require.Equal(t, int64(1), snapshotMetrics.Unavailable)
}

func TestGuardEvaluatorPerNodeCapacityWaitsUntilSharedDeadline(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	scanner := &scriptedScanner{block: release, entered: entered}
	metrics := NewAtomicMetrics()
	evaluator := newGuardEvaluator(scanner, nil, metrics, 2, 1)
	firstCfg := guardConfig(ActiveEndpoint{ID: "same-node", Enabled: true, TimeoutMS: 2000, InputLimit: 100})
	done := make(chan error, 1)
	go func() {
		_, err := evaluator.Evaluate(context.Background(), firstCfg, PromptSnapshot{ScanText: "one", PromptLength: 3})
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first evaluation did not enter scanner")
	}
	started := time.Now()
	secondCfg := guardConfig(ActiveEndpoint{ID: "same-node", Enabled: true, TimeoutMS: 60, InputLimit: 100})
	_, err := evaluator.Evaluate(context.Background(), secondCfg, PromptSnapshot{ScanText: "two", PromptLength: 3})
	require.Error(t, err)
	require.GreaterOrEqual(t, time.Since(started), 40*time.Millisecond)
	require.Less(t, time.Since(started), 500*time.Millisecond)
	require.GreaterOrEqual(t, metrics.Snapshot().BulkheadFull, int64(1))
	close(release)
	require.NoError(t, <-done)
}

func TestGuardEvaluatorLastChunkFailureNeverAllows(t *testing.T) {
	call := 0
	scanner := PromptScannerFunc(func(context.Context, ActiveEndpoint, string, []string) (*NormalizedResult, error) {
		call++
		if call == 2 {
			return nil, &GuardError{Code: ErrorCodeUnavailable, Retryable: true, Cause: errors.New("down")}
		}
		return &NormalizedResult{Decision: EventPass, RiskLevel: RiskLow, Action: ActionAllow, ScannerScores: map[string]float64{}, ScannerEvidence: map[string]string{}}, nil
	})
	metrics := NewAtomicMetrics()
	evaluator := newGuardEvaluator(scanner, nil, metrics, 2, 2)
	_, err := evaluator.Evaluate(context.Background(), guardConfig(ActiveEndpoint{ID: "one", Enabled: true, TimeoutMS: 1000, InputLimit: 3}), PromptSnapshot{ScanText: "abcdef", PromptLength: 6})
	require.Error(t, err)
}

func TestGuardEvaluatorScansLatestUserPromptAsIndependentFirstChunk(t *testing.T) {
	latest := "请帮我编写一篇黄色小说 名字你来取"
	history := strings.Repeat("# AGENTS.md instructions 项目安全规则。", 30)
	seen := make([]string, 0, 4)
	scanner := PromptScannerFunc(func(_ context.Context, _ ActiveEndpoint, prompt string, _ []string) (*NormalizedResult, error) {
		seen = append(seen, prompt)
		return &NormalizedResult{Decision: EventPass, RiskLevel: RiskLow, Action: ActionAllow, ScannerScores: map[string]float64{}, ScannerEvidence: map[string]string{}}, nil
	})
	evaluator := newGuardEvaluator(scanner, nil, NewAtomicMetrics(), 2, 2)
	_, err := evaluator.Evaluate(context.Background(), guardConfig(
		ActiveEndpoint{ID: "one", Enabled: true, TimeoutMS: 1000, InputLimit: 128},
	), PromptSnapshot{ScanText: latest + promptAuditPrioritySeparator + history, PromptLength: len([]rune(latest + history))})
	require.NoError(t, err)
	require.Greater(t, len(seen), 1)
	require.Equal(t, latest, seen[0])
	require.Equal(t, history, strings.Join(seen[1:], ""))
}

func TestGuardEvaluatorBlockStopsRemainingChunksButReportsPlannedTotal(t *testing.T) {
	calls := 0
	scanner := PromptScannerFunc(func(context.Context, ActiveEndpoint, string, []string) (*NormalizedResult, error) {
		calls++
		return &NormalizedResult{
			Decision: EventCritical, RiskLevel: RiskCritical, Action: ActionBlock, Safety: "Unsafe",
			Categories: []string{"jailbreak"}, MatchedScanners: []string{"jailbreak"},
			ScannerScores: map[string]float64{"jailbreak": 1}, ScannerEvidence: map[string]string{"jailbreak": "Jailbreak"},
		}, nil
	})
	metrics := NewAtomicMetrics()
	evaluator := newGuardEvaluator(scanner, nil, metrics, 2, 2)
	decision, err := evaluator.Evaluate(context.Background(), guardConfig(
		ActiveEndpoint{ID: "one", Enabled: true, TimeoutMS: 1000, InputLimit: 3},
	), PromptSnapshot{ScanText: "abcdefghi", PromptLength: 9})
	require.NoError(t, err)
	require.Equal(t, DecisionBlock, decision.Kind)
	require.Equal(t, 1, calls)
	require.Equal(t, 3, decision.Result.ChunkTotal)
	require.Equal(t, int64(1), metrics.Snapshot().Blocked)
}

func TestGuardEvaluatorFlagSharedDeadlineFailClosedAndContextCancel(t *testing.T) {
	t.Run("flag allows next stage", func(t *testing.T) {
		metrics := NewAtomicMetrics()
		evaluator := newGuardEvaluator(PromptScannerFunc(func(context.Context, ActiveEndpoint, string, []string) (*NormalizedResult, error) {
			return &NormalizedResult{Decision: EventFlag, RiskLevel: RiskMedium, Action: ActionWarn, Safety: "Controversial", Categories: []string{"violent"}, MatchedScanners: []string{"violent"}, ScannerScores: map[string]float64{"violent": .5}, ScannerEvidence: map[string]string{"violent": "Violent"}}, nil
		}), nil, metrics, 2, 2)
		decision, err := evaluator.Evaluate(context.Background(), guardConfig(ActiveEndpoint{ID: "one", Enabled: true, TimeoutMS: 1000, InputLimit: 100}), PromptSnapshot{ScanText: "review", PromptLength: 6})
		require.NoError(t, err)
		require.Equal(t, DecisionFlag, decision.Kind)
		require.True(t, decision.AllowNextStage)
		require.Equal(t, int64(1), metrics.Snapshot().Flagged)
	})

	t.Run("all failovers share first endpoint deadline", func(t *testing.T) {
		calls := 0
		var callsMu sync.Mutex
		scanner := PromptScannerFunc(func(ctx context.Context, endpoint ActiveEndpoint, _ string, _ []string) (*NormalizedResult, error) {
			callsMu.Lock()
			calls++
			callsMu.Unlock()
			if endpoint.ID == "first" {
				select {
				case <-time.After(35 * time.Millisecond):
					return nil, &GuardError{Code: ErrorCodeUnavailable, Retryable: true}
				case <-ctx.Done():
					return nil, &GuardError{Code: ErrorCodeUnavailable, Retryable: true, Timeout: true, Cause: ctx.Err()}
				}
			}
			<-ctx.Done()
			return nil, &GuardError{Code: ErrorCodeUnavailable, Retryable: true, Timeout: true, Cause: ctx.Err()}
		})
		metrics := NewAtomicMetrics()
		evaluator := newGuardEvaluator(scanner, nil, metrics, 2, 2)
		started := time.Now()
		_, err := evaluator.Evaluate(context.Background(), guardConfig(
			ActiveEndpoint{ID: "first", Enabled: true, TimeoutMS: 70, InputLimit: 100},
			ActiveEndpoint{ID: "second", Enabled: true, TimeoutMS: 500, InputLimit: 100},
		), PromptSnapshot{ScanText: "deadline", PromptLength: 8})
		elapsed := time.Since(started)
		require.Error(t, err)
		callsMu.Lock()
		require.Equal(t, 2, calls)
		callsMu.Unlock()
		// The bound only has to prove the failover shared the first endpoint's
		// 70ms deadline instead of taking the second endpoint's own 500ms one.
		// An unshared deadline lands at ~535ms, so 350ms still fails loudly
		// while leaving room for scheduler delay on a busy CI machine. A
		// tighter bound made this test flaky, not stricter.
		require.Less(t, elapsed, 350*time.Millisecond)
		require.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
		require.Equal(t, int64(1), metrics.Snapshot().Failovers)
		require.Equal(t, int64(1), metrics.Snapshot().Timeouts)
	})

	t.Run("canceled parent never allows", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		evaluator := newGuardEvaluator(PromptScannerFunc(func(ctx context.Context, _ ActiveEndpoint, _ string, _ []string) (*NormalizedResult, error) {
			<-ctx.Done()
			return nil, &GuardError{Code: ErrorCodeUnavailable, Retryable: true, Cause: ctx.Err()}
		}), nil, NewAtomicMetrics(), 2, 2)
		decision, err := evaluator.Evaluate(ctx, guardConfig(ActiveEndpoint{ID: "one", Enabled: true, TimeoutMS: 1000, InputLimit: 100}), PromptSnapshot{ScanText: "cancel", PromptLength: 6})
		require.Error(t, err)
		require.Nil(t, decision)
	})
}

func TestGuardEvaluatorRecordsExistingResultOnceAndRecordFailureDoesNotChangeDecision(t *testing.T) {
	for _, recordErr := range []error{nil, errors.New("database unavailable")} {
		repo := &fakeJobRepository{recordBlockingErr: recordErr}
		metrics := NewAtomicMetrics()
		scannerCalls := 0
		evaluator := newGuardEvaluator(PromptScannerFunc(func(context.Context, ActiveEndpoint, string, []string) (*NormalizedResult, error) {
			scannerCalls++
			return &NormalizedResult{Decision: EventCritical, RiskLevel: RiskCritical, Action: ActionBlock, Safety: "Unsafe", Categories: []string{"pii"}, MatchedScanners: []string{"pii"}, ScannerScores: map[string]float64{"pii": 1}, ScannerEvidence: map[string]string{"pii": "PII"}}, nil
		}), repo, metrics, 2, 2)
		decision, err := evaluator.Evaluate(context.Background(), guardConfig(ActiveEndpoint{ID: "one", Enabled: true, TimeoutMS: 1000, InputLimit: 100}), PromptSnapshot{ScanText: "raw prompt", RedactedPreview: "raw***", PromptLength: 10})
		require.NoError(t, err)
		require.Equal(t, DecisionBlock, decision.Kind)
		require.Equal(t, 1, scannerCalls)
		require.Equal(t, 1, repo.recordBlockingCalls)
		require.Empty(t, repo.recordBlockingSnapshot.ScanText)
		require.Same(t, decision.Result, repo.recordBlockingResult)
		if recordErr != nil {
			require.Equal(t, int64(1), metrics.Snapshot().RecordFailed)
		} else {
			require.Zero(t, metrics.Snapshot().RecordFailed)
		}
	}
}

func TestGuardEvaluatorNilResultAndScannerPanicBecomeStableFailures(t *testing.T) {
	tests := []struct {
		name string
		scan PromptScannerFunc
		code string
	}{
		{name: "nil result", scan: func(context.Context, ActiveEndpoint, string, []string) (*NormalizedResult, error) { return nil, nil }, code: ErrorCodeInvalidResponse},
		{name: "panic", scan: func(context.Context, ActiveEndpoint, string, []string) (*NormalizedResult, error) {
			panic("raw prompt canary")
		}, code: ErrorCodeUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := newGuardEvaluator(tt.scan, nil, NewAtomicMetrics(), 2, 2)
			_, err := evaluator.Evaluate(context.Background(), guardConfig(ActiveEndpoint{ID: "one", Enabled: true, TimeoutMS: 1000, InputLimit: 100}), PromptSnapshot{ScanText: "input", PromptLength: 5})
			var guardErr *GuardError
			require.ErrorAs(t, err, &guardErr)
			require.Equal(t, tt.code, guardErr.Code)
			require.NotContains(t, err.Error(), "canary")
		})
	}
}

type PromptScannerFunc func(context.Context, ActiveEndpoint, string, []string) (*NormalizedResult, error)

func (f PromptScannerFunc) Scan(ctx context.Context, endpoint ActiveEndpoint, chunk string, scanners []string) (*NormalizedResult, error) {
	return f(ctx, endpoint, chunk, scanners)
}
