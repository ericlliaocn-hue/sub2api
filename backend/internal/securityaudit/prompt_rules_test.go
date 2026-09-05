package securityaudit

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptGuardRuleSetUsesPriorityAndSkipsDisabledRules(t *testing.T) {
	set, err := NewPromptGuardRuleSet([]PromptGuardRule{
		{ID: 2, RuleKey: "later", Name: "later", Category: "jailbreak", Severity: RiskHigh, PatternType: PromptGuardRulePatternLiteral, Pattern: "danger", Action: PromptGuardRuleActionBlock, Priority: 20, Enabled: true},
		{ID: 1, RuleKey: "disabled", Name: "disabled", Category: "pii", Severity: RiskCritical, PatternType: PromptGuardRulePatternLiteral, Pattern: "danger", Action: PromptGuardRuleActionBlock, Priority: 1, Enabled: false},
		{ID: 3, RuleKey: "first", Name: "first", Category: "violent", Severity: RiskCritical, PatternType: PromptGuardRulePatternRegex, Pattern: `(?i)danger`, Action: PromptGuardRuleActionFlag, Priority: 10, Enabled: true},
	})
	require.NoError(t, err)
	match, ok := set.Match("DANGER")
	require.True(t, ok)
	require.Equal(t, "first", match.Rule.RuleKey)
	require.Equal(t, PromptGuardRuleActionFlag, match.Rule.Action)
	_, ok = set.Match("ordinary safety discussion")
	require.False(t, ok)
}

func TestPromptGuardRuleSetRejectsInvalidRegex(t *testing.T) {
	_, err := NewPromptGuardRuleSet([]PromptGuardRule{{
		ID: 1, RuleKey: "invalid", Name: "invalid", Category: "jailbreak", Severity: RiskHigh,
		PatternType: PromptGuardRulePatternRegex, Pattern: "(", Action: PromptGuardRuleActionBlock,
		Priority: 1, Enabled: true,
	}})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPromptGuardRuleInvalid)
}

type promptRuleTestRepo struct {
	mu    sync.Mutex
	rules []PromptGuardRule
	err   error
}

func (r *promptRuleTestRepo) List(context.Context, bool) ([]PromptGuardRule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	return append([]PromptGuardRule(nil), r.rules...), nil
}
func (*promptRuleTestRepo) Create(context.Context, UpsertPromptGuardRuleRequest, int64) (*PromptGuardRule, error) {
	return nil, errors.New("not implemented")
}
func (*promptRuleTestRepo) Update(context.Context, UpsertPromptGuardRuleRequest, int64) (*PromptGuardRule, error) {
	return nil, errors.New("not implemented")
}
func (*promptRuleTestRepo) Delete(context.Context, int64) error { return errors.New("not implemented") }

func TestPromptGuardRuleStoreReloadsAfterExplicitUpdate(t *testing.T) {
	repo := &promptRuleTestRepo{rules: []PromptGuardRule{{
		ID: 1, RuleKey: "old", Name: "old", Category: "jailbreak", Severity: RiskHigh,
		PatternType: PromptGuardRulePatternLiteral, Pattern: "old", Action: PromptGuardRuleActionBlock, Priority: 1, Enabled: true,
	}}}
	store := NewPromptGuardRuleStore(repo)
	match, ok, err := store.Match(context.Background(), "old value")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "old", match.Rule.RuleKey)
	repo.mu.Lock()
	repo.rules = []PromptGuardRule{{
		ID: 2, RuleKey: "new", Name: "new", Category: "pii", Severity: RiskCritical,
		PatternType: PromptGuardRulePatternLiteral, Pattern: "new", Action: PromptGuardRuleActionBlock, Priority: 1, Enabled: true,
	}}
	repo.mu.Unlock()
	require.NoError(t, store.Reload(context.Background()))
	_, ok, err = store.Match(context.Background(), "old value")
	require.NoError(t, err)
	require.False(t, ok)
	match, ok, err = store.Match(context.Background(), "new value")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "new", match.Rule.RuleKey)
}

func TestPromptGuardDefaultRulePatternsCoverHighConfidenceCases(t *testing.T) {
	rules := []PromptGuardRule{
		{ID: 1, RuleKey: "sexual", Name: "sexual", Category: "sexual_content_or_sexual_acts", Severity: RiskCritical, PatternType: PromptGuardRulePatternRegex, Pattern: `(?is)(写|创作|生成).{0,40}(色情|情色|黄文)`, Action: PromptGuardRuleActionBlock, Priority: 1, Enabled: true},
		{ID: 2, RuleKey: "scrape", Name: "scrape", Category: "non_violent_illegal_acts", Severity: RiskHigh, PatternType: PromptGuardRulePatternRegex, Pattern: `(?is)(批量|自动化).{0,30}(爬取|抓取).{0,50}(网站|数据)`, Action: PromptGuardRuleActionBlock, Priority: 2, Enabled: true},
	}
	set, err := NewPromptGuardRuleSet(rules)
	require.NoError(t, err)
	for _, sample := range []string{"请写一篇色情小说", "自动化批量爬取网站数据"} {
		_, ok := set.Match(sample)
		require.True(t, ok, sample)
	}
	_, ok := set.Match("请介绍色情内容审核的合规标准")
	require.False(t, ok)
}

func TestPromptGuardRuleStoreKeepsLastGoodSnapshotOnReloadFailure(t *testing.T) {
	repo := &promptRuleTestRepo{rules: []PromptGuardRule{{
		ID: 1, RuleKey: "stable", Name: "stable", Category: "jailbreak", Severity: RiskHigh,
		PatternType: PromptGuardRulePatternLiteral, Pattern: "stable", Action: PromptGuardRuleActionBlock, Priority: 1, Enabled: true,
	}}}
	store := NewPromptGuardRuleStore(repo)
	_, ok, err := store.Match(context.Background(), "stable")
	require.NoError(t, err)
	require.True(t, ok)
	store.ttl = 0
	repo.mu.Lock()
	repo.err = errors.New("database unavailable")
	repo.mu.Unlock()
	set, err := store.Snapshot(context.Background())
	require.Error(t, err)
	require.NotNil(t, set)
	match, matched := set.Match("stable")
	require.True(t, matched)
	require.Equal(t, "stable", match.Rule.RuleKey)
}
