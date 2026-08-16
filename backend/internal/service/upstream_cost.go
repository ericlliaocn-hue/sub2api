package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

const (
	UpstreamCostProfilesExtraKey = "upstream_cost_profiles"
	// UpstreamCostEnabledExtraKey 是账号级上游成本核算开关。默认关闭，只对
	// OpenAI 账号有意义。配置属于账号资产，账号 API 通过独立字段
	// upstream_cost_enabled / upstream_cost_profiles 读写，不要求前端直接拼 extra。
	UpstreamCostEnabledExtraKey = "upstream_cost_enabled"
)

var supportedManualUpstreamCostModels = map[string]struct{}{
	"gpt-5.6-luna":  {},
	"gpt-5.6-terra": {},
}

type UpstreamCostPrices struct {
	Input      float64 `json:"input"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	Output     float64 `json:"output"`
}

// UpstreamCostVersion is an immutable manual profile. Prices are expressed per
// million tokens, matching the values shown by upstream providers.
type UpstreamCostVersion struct {
	ID                   int64              `json:"id"`
	AccountID            int64              `json:"account_id"`
	AccountName          string             `json:"account_name"`
	Model                string             `json:"model"`
	ShortPrices          UpstreamCostPrices `json:"short_prices"`
	LongContextThreshold int                `json:"long_context_threshold"`
	LongPrices           UpstreamCostPrices `json:"long_prices"`
	DeclaredMultiplier   float64            `json:"declared_multiplier"`
	BalanceUnitCost      float64            `json:"balance_unit_cost"`
	Notes                string             `json:"notes"`
	EffectiveFrom        time.Time          `json:"effective_from"`
	CreatedBy            *int64             `json:"created_by,omitempty"`
	CreatedAt            time.Time          `json:"created_at"`
}

type UpstreamCostVersionInput struct {
	AccountID            int64
	Model                string
	ShortPrices          UpstreamCostPrices
	LongContextThreshold int
	LongPrices           UpstreamCostPrices
	DeclaredMultiplier   float64
	BalanceUnitCost      float64
	Notes                string
}

// UpstreamCostProfileInput 是账号创建/编辑 API 的独立字段
// （upstream_cost_profiles），不要求前端直接拼 extra。
type UpstreamCostProfileInput struct {
	Model                string             `json:"model"`
	ShortPrices          UpstreamCostPrices `json:"short_prices"`
	LongContextThreshold int                `json:"long_context_threshold"`
	LongPrices           UpstreamCostPrices `json:"long_prices"`
	DeclaredMultiplier   float64            `json:"declared_multiplier"`
	BalanceUnitCost      float64            `json:"balance_unit_cost"`
	Notes                string             `json:"notes"`
}

// ToUpstreamCostVersionInput converts the API shape into the repository input.
func (p UpstreamCostProfileInput) ToUpstreamCostVersionInput(accountID int64) UpstreamCostVersionInput {
	return UpstreamCostVersionInput{
		AccountID:            accountID,
		Model:                p.Model,
		ShortPrices:          p.ShortPrices,
		LongContextThreshold: p.LongContextThreshold,
		LongPrices:           p.LongPrices,
		DeclaredMultiplier:   p.DeclaredMultiplier,
		BalanceUnitCost:      p.BalanceUnitCost,
		Notes:                p.Notes,
	}
}

// ValidateUpstreamCostProfileInput validates one API profile without an
// account id (the repository fills it once the account row exists).
func ValidateUpstreamCostProfileInput(input *UpstreamCostProfileInput) error {
	if input == nil {
		return fmt.Errorf("upstream cost profile cannot be nil")
	}
	versionInput := input.ToUpstreamCostVersionInput(0)
	return validateUpstreamCostVersionContent(&versionInput)
}

func validateUpstreamCostVersionInput(input *UpstreamCostVersionInput) error {
	if input.AccountID <= 0 {
		return fmt.Errorf("invalid account id")
	}
	return validateUpstreamCostVersionContent(input)
}

// validateUpstreamCostVersionContent validates everything except the account
// id, so the account-edit API can validate profiles before the account id is
// known (the repository fills it once the account exists).
//
// 模型名不做白名单限制：预置 Luna/Terra 之外允许新增任意模型（计划 §1.1），
// 仅校验格式（非空、长度）。上游模型名会写入 usage_logs.model（MaxLen 100）。
func validateUpstreamCostVersionContent(input *UpstreamCostVersionInput) error {
	input.Model = strings.ToLower(strings.TrimSpace(input.Model))
	if input.Model == "" {
		return fmt.Errorf("upstream cost model is required")
	}
	if len(input.Model) > 100 {
		return fmt.Errorf("upstream cost model is too long (max 100)")
	}
	input.Notes = strings.TrimSpace(input.Notes)
	if input.LongContextThreshold < 0 {
		return fmt.Errorf("long context threshold cannot be negative")
	}
	prices := []float64{
		input.ShortPrices.Input, input.ShortPrices.CacheRead, input.ShortPrices.CacheWrite, input.ShortPrices.Output,
		input.LongPrices.Input, input.LongPrices.CacheRead, input.LongPrices.CacheWrite, input.LongPrices.Output,
		input.DeclaredMultiplier, input.BalanceUnitCost,
	}
	for _, value := range prices {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return fmt.Errorf("prices and multipliers must be finite non-negative numbers")
		}
	}
	if input.ShortPrices.Input == 0 || input.ShortPrices.Output == 0 {
		return fmt.Errorf("short context input and output prices must be positive")
	}
	if input.BalanceUnitCost <= 0 {
		return fmt.Errorf("balance unit cost must be positive")
	}
	if input.LongContextThreshold == 0 {
		input.LongPrices = input.ShortPrices
	} else if input.LongPrices.Input == 0 || input.LongPrices.Output == 0 {
		return fmt.Errorf("long context input and output prices must be positive when a threshold is configured")
	}
	return nil
}

func (p UpstreamCostVersion) ExtraSnapshot() map[string]any {
	return map[string]any{
		"id":                     p.ID,
		"version_id":             p.ID,
		"model":                  p.Model,
		"source":                 "manual",
		"short_prices":           p.ShortPrices,
		"long_context_threshold": p.LongContextThreshold,
		"long_prices":            p.LongPrices,
		"declared_multiplier":    p.DeclaredMultiplier,
		"balance_unit_cost":      p.BalanceUnitCost,
		"effective_from":         p.EffectiveFrom.UTC().Format(time.RFC3339Nano),
	}
}

func applyManualUpstreamCost(usageLog *UsageLog, account *Account, upstreamModel, requestedModel string, tokens UsageTokens) {
	if usageLog == nil || account == nil || len(account.Extra) == 0 {
		return
	}
	model := strings.ToLower(strings.TrimSpace(upstreamModel))
	if model == "" {
		model = strings.ToLower(strings.TrimSpace(requestedModel))
	}
	profiles, ok := account.Extra[UpstreamCostProfilesExtraKey].(map[string]any)
	if !ok {
		return
	}
	raw, ok := profiles[model]
	if !ok {
		return
	}
	profile, ok := decodeManualUpstreamCostProfile(raw)
	if !ok {
		return
	}

	prices := profile.ShortPrices
	totalInput := tokens.InputTokens + tokens.CacheCreationTokens + tokens.CacheReadTokens
	longContext := profile.LongContextThreshold > 0 && totalInput > profile.LongContextThreshold
	if longContext {
		prices = profile.LongPrices
	}
	baseCost := (float64(tokens.InputTokens)*prices.Input +
		float64(tokens.CacheReadTokens)*prices.CacheRead +
		float64(tokens.CacheCreationTokens)*prices.CacheWrite +
		float64(tokens.OutputTokens)*prices.Output) / 1_000_000
	// 倍率优先级（计划 §Phase 6）：有效倍率版本（请求开始时固定的账号倍率）
	// → 成本配置 declared_multiplier fallback → 1.0。账号倍率只乘一次，
	// 不会在 upstream_cost 之外重复乘。
	appliedMultiplier := 1.0
	// default 版本只是新账号的初始化占位，不代表已确认的上游声明倍率；
	// 它不能遮蔽成本配置里的 declared_multiplier fallback。只有 manual / probe
	// 等非 default 版本才拥有倍率口径。
	rateSourceIsDefault := usageLog.AccountRateSource != nil &&
		strings.TrimSpace(*usageLog.AccountRateSource) == string(domain.UpstreamRateSourceDefault)
	if usageLog.AccountRateVersionID != nil && usageLog.AccountRateMultiplier != nil &&
		*usageLog.AccountRateMultiplier >= 0 && !rateSourceIsDefault {
		appliedMultiplier = *usageLog.AccountRateMultiplier
	} else if profile.DeclaredMultiplier >= 0 {
		appliedMultiplier = profile.DeclaredMultiplier
	}
	upstreamCost := baseCost * appliedMultiplier * profile.BalanceUnitCost
	currentOfficialValue := usageLog.TotalCost
	normalizedMultiplier := 0.0
	if currentOfficialValue > 0 {
		normalizedMultiplier = upstreamCost / currentOfficialValue
	}
	usageLog.UpstreamCost = &upstreamCost
	snapshot := map[string]any{
		"price_version_id":       profile.ID,
		"version_id":             profile.ID,
		"model":                  model,
		"source":                 "manual",
		"short_prices":           profile.ShortPrices,
		"long_context_threshold": profile.LongContextThreshold,
		"long_prices":            profile.LongPrices,
		"long_context_applied":   longContext,
		"declared_multiplier":    profile.DeclaredMultiplier,
		"balance_unit_cost":      profile.BalanceUnitCost,
		"applied_multiplier":     appliedMultiplier,
		"upstream_base_cost":     baseCost,
		"current_official_value": currentOfficialValue,
		"official_cost":          currentOfficialValue,
		"normalized_multiplier":  normalizedMultiplier,
		"effective_from":         profile.EffectiveFrom.UTC().Format(time.RFC3339Nano),
	}
	if usageLog.AccountRateVersionID != nil {
		snapshot["rate_version_id"] = *usageLog.AccountRateVersionID
	}
	if usageLog.AccountRateSource != nil {
		snapshot["rate_source"] = *usageLog.AccountRateSource
	}
	usageLog.UpstreamCostSnapshot = snapshot
}

func decodeManualUpstreamCostProfile(raw any) (UpstreamCostVersion, bool) {
	data, err := json.Marshal(raw)
	if err != nil {
		return UpstreamCostVersion{}, false
	}
	var profile UpstreamCostVersion
	if err := json.Unmarshal(data, &profile); err != nil || profile.ID <= 0 {
		return UpstreamCostVersion{}, false
	}
	if strings.TrimSpace(profile.Model) == "" {
		return UpstreamCostVersion{}, false
	}
	return profile, true
}

// ValidateUpstreamCostProfiles validates the full profile set for an account
// create/edit. Only OpenAI accounts may carry upstream cost configuration.
func ValidateUpstreamCostProfiles(platform string, enabled *bool, profiles []UpstreamCostProfileInput) error {
	if enabled == nil && len(profiles) == 0 {
		return nil
	}
	if platform != PlatformOpenAI {
		return fmt.Errorf("upstream cost configuration requires an OpenAI account")
	}
	seenModels := make(map[string]struct{}, len(profiles))
	for i := range profiles {
		profile := profiles[i]
		if err := ValidateUpstreamCostProfileInput(&profile); err != nil {
			return err
		}
		model := strings.ToLower(strings.TrimSpace(profile.Model))
		if _, exists := seenModels[model]; exists {
			return fmt.Errorf("duplicate upstream cost profile for model %s", model)
		}
		seenModels[model] = struct{}{}
	}
	return nil
}

// AccountUpstreamCostRepository is the optional capability that atomically
// persists the manual upstream cost configuration (price version rows +
// accounts.extra) together with account-level rate versions.
type AccountUpstreamCostRepository interface {
	// CreateWithUpstreamCostConfig creates the account and, in the same
	// transaction, its initial rate version and any manual price versions.
	CreateWithUpstreamCostConfig(ctx context.Context, account *Account, costInputs []UpstreamCostVersionInput, createdBy int64) error
	// ReplaceUpstreamCostProfiles upserts manual price versions for changed
	// models and replaces the account's active profile set in one transaction.
	ReplaceUpstreamCostProfiles(ctx context.Context, accountID int64, costInputs []UpstreamCostVersionInput, enabled *bool, createdBy int64) error
}
