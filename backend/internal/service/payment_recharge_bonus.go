package service

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const maxRechargeBonusTiers = 50

// RechargeBonusTier configures an exact-match bonus for a balance recharge.
// PaymentAmount is denominated in Currency, while BonusAmount is account balance.
type RechargeBonusTier struct {
	PaymentAmount    float64 `json:"payment_amount"`
	BonusAmount      float64 `json:"bonus_amount"`
	Currency         string  `json:"currency"`
	Enabled          bool    `json:"enabled"`
	ValidityDays     int     `json:"validity_days"`
	MaxClaimsPerUser int     `json:"max_claims_per_user"`
	CampaignID       string  `json:"campaign_id"`
}

// AppliedRechargeBonus is the immutable bonus snapshot attached to an order.
type AppliedRechargeBonus struct {
	PaymentAmount      float64 `json:"payment_amount"`
	BaseCreditedAmount float64 `json:"base_credited_amount"`
	BonusAmount        float64 `json:"bonus_amount"`
	CreditedAmount     float64 `json:"credited_amount"`
	Currency           string  `json:"currency"`
	BalanceMultiplier  float64 `json:"balance_multiplier"`
	ValidityDays       int     `json:"validity_days"`
	MaxClaimsPerUser   int     `json:"max_claims_per_user"`
	CampaignID         string  `json:"campaign_id"`
}

func parseRechargeBonusTiers(raw string) []RechargeBonusTier {
	if strings.TrimSpace(raw) == "" {
		return []RechargeBonusTier{}
	}
	var tiers []RechargeBonusTier
	if err := json.Unmarshal([]byte(raw), &tiers); err != nil {
		return []RechargeBonusTier{}
	}
	normalized, err := normalizeRechargeBonusTiers(tiers)
	if err != nil {
		return []RechargeBonusTier{}
	}
	return normalized
}

func normalizeRechargeBonusTiers(tiers []RechargeBonusTier) ([]RechargeBonusTier, error) {
	if len(tiers) > maxRechargeBonusTiers {
		return nil, fmt.Errorf("at most %d recharge bonus tiers are allowed", maxRechargeBonusTiers)
	}
	normalized := make([]RechargeBonusTier, 0, len(tiers))
	seen := make(map[string]struct{}, len(tiers))
	for _, tier := range tiers {
		currency, err := payment.NormalizePaymentCurrency(tier.Currency)
		if err != nil {
			return nil, fmt.Errorf("invalid recharge bonus currency: %w", err)
		}
		if !validMoneyAmount(tier.PaymentAmount, payment.CurrencyMaxFractionDigits(currency)) {
			return nil, fmt.Errorf("recharge payment amount must be positive and use at most %d decimal places", payment.CurrencyMaxFractionDigits(currency))
		}
		if !validMoneyAmount(tier.BonusAmount, 2) {
			return nil, fmt.Errorf("recharge bonus amount must be positive and use at most 2 decimal places")
		}
		validityDays := tier.ValidityDays
		if validityDays == 0 {
			validityDays = 30
		}
		maxClaimsPerUser := tier.MaxClaimsPerUser
		if maxClaimsPerUser == 0 {
			maxClaimsPerUser = 1
		}
		if validityDays <= 0 || validityDays > 3650 {
			return nil, fmt.Errorf("recharge bonus validity days must be between 1 and 3650")
		}
		if maxClaimsPerUser <= 0 || maxClaimsPerUser > 1000 {
			return nil, fmt.Errorf("recharge bonus max claims per user must be between 1 and 1000")
		}
		campaignID := strings.TrimSpace(tier.CampaignID)
		if campaignID == "" {
			campaignID = fmt.Sprintf("legacy-%s-%s", currency, decimal.NewFromFloat(tier.PaymentAmount).Round(int32(payment.CurrencyMaxFractionDigits(currency))).String())
		}
		if len(campaignID) > 100 {
			return nil, fmt.Errorf("recharge bonus campaign id is required and must not exceed 100 characters")
		}
		paymentAmount := decimal.NewFromFloat(tier.PaymentAmount).
			Round(int32(payment.CurrencyMaxFractionDigits(currency)))
		bonusAmount := decimal.NewFromFloat(tier.BonusAmount).Round(2)
		key := currency + ":" + paymentAmount.StringFixed(int32(payment.CurrencyMaxFractionDigits(currency)))
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate recharge bonus tier for %s %s", currency, paymentAmount.String())
		}
		seen[key] = struct{}{}
		normalized = append(normalized, RechargeBonusTier{
			PaymentAmount:    paymentAmount.InexactFloat64(),
			BonusAmount:      bonusAmount.InexactFloat64(),
			Currency:         currency,
			Enabled:          tier.Enabled,
			ValidityDays:     validityDays,
			MaxClaimsPerUser: maxClaimsPerUser,
			CampaignID:       campaignID,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Currency != normalized[j].Currency {
			return normalized[i].Currency < normalized[j].Currency
		}
		return normalized[i].PaymentAmount < normalized[j].PaymentAmount
	})
	return normalized, nil
}

func validMoneyAmount(value float64, fractionDigits int) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return false
	}
	return decimal.NewFromFloat(value).Equal(decimal.NewFromFloat(value).Round(int32(fractionDigits)))
}

func calculateAppliedRechargeBonus(paymentAmount float64, currency string, multiplier float64, tiers []RechargeBonusTier) *AppliedRechargeBonus {
	normalizedCurrency, err := payment.NormalizePaymentCurrency(currency)
	if err != nil {
		return nil
	}
	paymentDecimal := decimal.NewFromFloat(paymentAmount).Round(int32(payment.CurrencyMaxFractionDigits(normalizedCurrency)))
	for _, tier := range tiers {
		if !tier.Enabled || tier.Currency != normalizedCurrency {
			continue
		}
		tierAmount := decimal.NewFromFloat(tier.PaymentAmount).Round(int32(payment.CurrencyMaxFractionDigits(normalizedCurrency)))
		if !paymentDecimal.Equal(tierAmount) {
			continue
		}
		base := decimal.NewFromFloat(calculateCreditedBalance(paymentAmount, multiplier))
		bonus := decimal.NewFromFloat(tier.BonusAmount).Round(2)
		validityDays := tier.ValidityDays
		if validityDays <= 0 {
			validityDays = 30
		}
		maxClaims := tier.MaxClaimsPerUser
		if maxClaims <= 0 {
			maxClaims = 1
		}
		campaignID := strings.TrimSpace(tier.CampaignID)
		if campaignID == "" {
			campaignID = fmt.Sprintf("legacy-%s-%.2f", normalizedCurrency, tier.PaymentAmount)
		}
		return &AppliedRechargeBonus{
			PaymentAmount:      paymentDecimal.InexactFloat64(),
			BaseCreditedAmount: base.InexactFloat64(),
			BonusAmount:        bonus.InexactFloat64(),
			CreditedAmount:     base.Add(bonus).Round(2).InexactFloat64(),
			Currency:           normalizedCurrency,
			BalanceMultiplier:  normalizeBalanceRechargeMultiplier(multiplier),
			ValidityDays:       validityDays,
			MaxClaimsPerUser:   maxClaims,
			CampaignID:         campaignID,
		}
	}
	return nil
}

func rechargeBonusFromSnapshot(snapshot map[string]any) (*AppliedRechargeBonus, bool) {
	if snapshot == nil {
		return nil, false
	}
	raw, ok := snapshot["recharge_bonus"]
	if !ok || raw == nil {
		return nil, false
	}
	if applied, ok := raw.(*AppliedRechargeBonus); ok && applied != nil {
		normalizeAppliedRechargeBonus(applied)
		return applied, applied.BonusAmount > 0
	}
	if applied, ok := raw.(AppliedRechargeBonus); ok {
		normalizeAppliedRechargeBonus(&applied)
		return &applied, applied.BonusAmount > 0
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	var applied AppliedRechargeBonus
	if err := json.Unmarshal(payload, &applied); err != nil || applied.BonusAmount <= 0 {
		return nil, false
	}
	normalizeAppliedRechargeBonus(&applied)
	return &applied, true
}

func normalizeAppliedRechargeBonus(applied *AppliedRechargeBonus) {
	if applied.ValidityDays <= 0 {
		applied.ValidityDays = 30
	}
	if applied.MaxClaimsPerUser <= 0 {
		applied.MaxClaimsPerUser = 1
	}
	if strings.TrimSpace(applied.CampaignID) == "" {
		applied.CampaignID = fmt.Sprintf("legacy-%s-%.2f", strings.ToUpper(applied.Currency), applied.PaymentAmount)
	}
}
