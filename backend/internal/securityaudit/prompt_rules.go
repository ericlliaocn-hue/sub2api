package securityaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	PromptGuardRulePatternLiteral = "literal"
	PromptGuardRulePatternRegex   = "regex"
	PromptGuardRuleActionBlock    = "block"
	PromptGuardRuleActionFlag     = "flag"

	defaultPromptGuardRulesTTL     = 5 * time.Second
	maxPromptGuardRuleKeyRunes     = 100
	maxPromptGuardRuleNameRunes    = 200
	maxPromptGuardRulePatternRunes = 2000
	maxPromptGuardRuleCount        = 1000
)

var (
	ErrPromptGuardRuleNotFound = errors.New("prompt guard rule not found")
	ErrPromptGuardRuleInvalid  = errors.New("prompt guard rule invalid")
)

// PromptGuardRule is the persisted, administrator-managed fast-path rule.
// Patterns are never prompt data; they are policy configuration.
type PromptGuardRule struct {
	ID          int64     `json:"id"`
	RuleKey     string    `json:"rule_key"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Severity    RiskLevel `json:"severity"`
	PatternType string    `json:"pattern_type"`
	Pattern     string    `json:"pattern"`
	Action      string    `json:"action"`
	Priority    int       `json:"priority"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   int64     `json:"updated_by"`
}

type UpsertPromptGuardRuleRequest struct {
	ID          int64     `json:"id"`
	RuleKey     string    `json:"rule_key"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Severity    RiskLevel `json:"severity"`
	PatternType string    `json:"pattern_type"`
	Pattern     string    `json:"pattern"`
	Action      string    `json:"action"`
	Priority    int       `json:"priority"`
	Enabled     bool      `json:"enabled"`
}

type PromptGuardRuleRepository interface {
	List(ctx context.Context, includeDisabled bool) ([]PromptGuardRule, error)
	Create(ctx context.Context, rule UpsertPromptGuardRuleRequest, actorID int64) (*PromptGuardRule, error)
	Update(ctx context.Context, rule UpsertPromptGuardRuleRequest, actorID int64) (*PromptGuardRule, error)
	Delete(ctx context.Context, id int64) error
}

type compiledPromptGuardRule struct {
	rule        PromptGuardRule
	literalFold string
	regex       *regexp.Regexp
}

type PromptGuardRuleSet struct {
	rules  []compiledPromptGuardRule
	digest string
}

type PromptGuardRuleMatch struct {
	Rule PromptGuardRule `json:"rule"`
}

func ValidatePromptGuardRule(input UpsertPromptGuardRuleRequest) error {
	input = normalizePromptGuardRuleRequest(input)
	if input.RuleKey == "" || len([]rune(input.RuleKey)) > maxPromptGuardRuleKeyRunes {
		return fmt.Errorf("%w: rule_key", ErrPromptGuardRuleInvalid)
	}
	if input.Name == "" || len([]rune(input.Name)) > maxPromptGuardRuleNameRunes {
		return fmt.Errorf("%w: name", ErrPromptGuardRuleInvalid)
	}
	if input.Category == "" || len([]rune(input.Category)) > maxPromptGuardRuleKeyRunes {
		return fmt.Errorf("%w: category", ErrPromptGuardRuleInvalid)
	}
	if input.Severity != RiskMedium && input.Severity != RiskHigh && input.Severity != RiskCritical {
		return fmt.Errorf("%w: severity", ErrPromptGuardRuleInvalid)
	}
	if input.PatternType != PromptGuardRulePatternLiteral && input.PatternType != PromptGuardRulePatternRegex {
		return fmt.Errorf("%w: pattern_type", ErrPromptGuardRuleInvalid)
	}
	if input.Pattern == "" || len([]rune(input.Pattern)) > maxPromptGuardRulePatternRunes {
		return fmt.Errorf("%w: pattern", ErrPromptGuardRuleInvalid)
	}
	if input.Action != PromptGuardRuleActionBlock && input.Action != PromptGuardRuleActionFlag {
		return fmt.Errorf("%w: action", ErrPromptGuardRuleInvalid)
	}
	if input.Priority < 1 || input.Priority > 100000 {
		return fmt.Errorf("%w: priority", ErrPromptGuardRuleInvalid)
	}
	if input.PatternType == PromptGuardRulePatternRegex {
		if _, err := regexp.Compile(input.Pattern); err != nil {
			return fmt.Errorf("%w: pattern regex: %v", ErrPromptGuardRuleInvalid, err)
		}
	}
	return nil
}

func compilePromptGuardRule(rule PromptGuardRule) (compiledPromptGuardRule, error) {
	request := UpsertPromptGuardRuleRequest{
		ID: rule.ID, RuleKey: rule.RuleKey, Name: rule.Name, Category: rule.Category,
		Severity: rule.Severity, PatternType: rule.PatternType, Pattern: rule.Pattern,
		Action: rule.Action, Priority: rule.Priority, Enabled: rule.Enabled,
	}
	if request.Priority == 0 {
		request.Priority = 100
	}
	if err := ValidatePromptGuardRule(request); err != nil {
		return compiledPromptGuardRule{}, err
	}
	rule.RuleKey = request.RuleKey
	rule.Name = request.Name
	rule.Category = request.Category
	rule.PatternType = request.PatternType
	rule.Pattern = request.Pattern
	rule.Action = request.Action
	rule.Priority = request.Priority
	rule.Severity = request.Severity
	compiled := compiledPromptGuardRule{rule: rule}
	if rule.PatternType == PromptGuardRulePatternRegex {
		compiled.regex = regexp.MustCompile(rule.Pattern)
	} else {
		compiled.literalFold = strings.ToLower(rule.Pattern)
	}
	return compiled, nil
}

func NewPromptGuardRuleSet(rules []PromptGuardRule) (*PromptGuardRuleSet, error) {
	if len(rules) > maxPromptGuardRuleCount {
		return nil, fmt.Errorf("%w: too many rules", ErrPromptGuardRuleInvalid)
	}
	compiled := make([]compiledPromptGuardRule, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		item, err := compilePromptGuardRule(rule)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, item)
	}
	sort.SliceStable(compiled, func(i, j int) bool {
		if compiled[i].rule.Priority != compiled[j].rule.Priority {
			return compiled[i].rule.Priority < compiled[j].rule.Priority
		}
		return compiled[i].rule.ID < compiled[j].rule.ID
	})
	canonical := make([]PromptGuardRule, len(compiled))
	for i := range compiled {
		canonical[i] = compiled[i].rule
	}
	raw, _ := json.Marshal(canonical)
	digest := sha256.Sum256(raw)
	return &PromptGuardRuleSet{rules: compiled, digest: hex.EncodeToString(digest[:])}, nil
}

func (s *PromptGuardRuleSet) Digest() string {
	if s == nil {
		return "none"
	}
	return s.digest
}

func (s *PromptGuardRuleSet) Match(text string) (PromptGuardRuleMatch, bool) {
	if s == nil || strings.TrimSpace(text) == "" {
		return PromptGuardRuleMatch{}, false
	}
	for _, rule := range s.rules {
		matched := false
		if rule.regex != nil {
			matched = rule.regex.MatchString(text)
		} else {
			matched = strings.Contains(strings.ToLower(text), rule.literalFold)
		}
		if matched {
			return PromptGuardRuleMatch{Rule: rule.rule}, true
		}
	}
	return PromptGuardRuleMatch{}, false
}

type promptGuardRuleSnapshot struct {
	set      *PromptGuardRuleSet
	loadedAt time.Time
}

type PromptGuardRuleStore struct {
	repo      PromptGuardRuleRepository
	ttl       time.Duration
	snapshot  atomic.Pointer[promptGuardRuleSnapshot]
	refreshMu sync.Mutex
}

func NewPromptGuardRuleStore(repo PromptGuardRuleRepository) *PromptGuardRuleStore {
	return &PromptGuardRuleStore{repo: repo, ttl: defaultPromptGuardRulesTTL}
}

func (s *PromptGuardRuleStore) Snapshot(ctx context.Context) (*PromptGuardRuleSet, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	now := time.Now()
	if current := s.snapshot.Load(); current != nil && now.Sub(current.loadedAt) < s.ttl {
		return current.set, nil
	}
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	if current := s.snapshot.Load(); current != nil && now.Sub(current.loadedAt) < s.ttl {
		return current.set, nil
	}
	rules, err := s.repo.List(ctx, false)
	if err != nil {
		if current := s.snapshot.Load(); current != nil {
			return current.set, err
		}
		return nil, err
	}
	set, err := NewPromptGuardRuleSet(rules)
	if err != nil {
		if current := s.snapshot.Load(); current != nil {
			return current.set, err
		}
		return nil, err
	}
	s.snapshot.Store(&promptGuardRuleSnapshot{set: set, loadedAt: time.Now()})
	return set, nil
}

func (s *PromptGuardRuleStore) Reload(ctx context.Context) error {
	if s == nil || s.repo == nil {
		return nil
	}
	rules, err := s.repo.List(ctx, false)
	if err != nil {
		return err
	}
	set, err := NewPromptGuardRuleSet(rules)
	if err != nil {
		return err
	}
	s.snapshot.Store(&promptGuardRuleSnapshot{set: set, loadedAt: time.Now()})
	return nil
}

func (s *PromptGuardRuleStore) Match(ctx context.Context, text string) (PromptGuardRuleMatch, bool, error) {
	set, err := s.Snapshot(ctx)
	if err != nil {
		return PromptGuardRuleMatch{}, false, err
	}
	match, ok := set.Match(text)
	return match, ok, nil
}

func (s *PromptGuardRuleStore) Digest(ctx context.Context) string {
	set, err := s.Snapshot(ctx)
	if err != nil || set == nil {
		return "none"
	}
	return set.Digest()
}
