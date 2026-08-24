package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SettingPaymentEnabled            = "payment_enabled"
	SettingMinRechargeAmount         = "MIN_RECHARGE_AMOUNT"
	SettingMaxRechargeAmount         = "MAX_RECHARGE_AMOUNT"
	SettingDailyRechargeLimit        = "DAILY_RECHARGE_LIMIT"
	SettingOrderTimeoutMinutes       = "ORDER_TIMEOUT_MINUTES"
	SettingMaxPendingOrders          = "MAX_PENDING_ORDERS"
	SettingEnabledPaymentTypes       = "ENABLED_PAYMENT_TYPES"
	SettingLoadBalanceStrategy       = "LOAD_BALANCE_STRATEGY"
	SettingBalancePayDisabled        = "BALANCE_PAYMENT_DISABLED"
	SettingBalanceRechargeMult       = "BALANCE_RECHARGE_MULTIPLIER"
	SettingRechargeBonusEnabled      = "RECHARGE_BONUS_ENABLED"
	SettingRechargeBonusTiers        = "RECHARGE_BONUS_TIERS"
	SettingRechargeBonusExpiryMode   = "RECHARGE_BONUS_EXPIRY_MODE"
	SettingRechargeBonusEndsAt       = "RECHARGE_BONUS_ENDS_AT"
	SettingRechargeBonusDurationDays = "RECHARGE_BONUS_DURATION_DAYS"
	SettingRechargeBonusStartedAt    = "RECHARGE_BONUS_STARTED_AT"
	// SettingSubscriptionUSDToCNYRate 是订阅 CNY 换算汇率（1 USD = X CNY）。
	// 0/未配置 = 关闭换算（订阅按 price 数值直付），显式配置后 CNY 通道订阅按 price × rate 收款。
	SettingSubscriptionUSDToCNYRate      = "SUBSCRIPTION_USD_TO_CNY_RATE"
	SettingRechargeFeeRate               = "RECHARGE_FEE_RATE"
	SettingProductNamePrefix             = "PRODUCT_NAME_PREFIX"
	SettingProductNameSuffix             = "PRODUCT_NAME_SUFFIX"
	SettingHelpImageURL                  = "PAYMENT_HELP_IMAGE_URL"
	SettingHelpText                      = "PAYMENT_HELP_TEXT"
	SettingCancelRateLimitOn             = "CANCEL_RATE_LIMIT_ENABLED"
	SettingCancelRateLimitMax            = "CANCEL_RATE_LIMIT_MAX"
	SettingCancelWindowSize              = "CANCEL_RATE_LIMIT_WINDOW"
	SettingCancelWindowUnit              = "CANCEL_RATE_LIMIT_UNIT"
	SettingCancelWindowMode              = "CANCEL_RATE_LIMIT_WINDOW_MODE"
	SettingAlipayForceQRCode             = "ALIPAY_FORCE_QRCODE"
	SettingAlipayMobilePrecreateDeepLink = "ALIPAY_MOBILE_PRECREATE_DEEP_LINK"
)

// Default values for payment configuration settings.
const (
	defaultOrderTimeoutMin  = 30
	defaultMaxPendingOrders = 3
)

// PaymentConfig holds the payment system configuration.
type PaymentConfig struct {
	Enabled                   bool                `json:"enabled"`
	MinAmount                 float64             `json:"min_amount"`
	MaxAmount                 float64             `json:"max_amount"`
	DailyLimit                float64             `json:"daily_limit"`
	OrderTimeoutMin           int                 `json:"order_timeout_minutes"`
	MaxPendingOrders          int                 `json:"max_pending_orders"`
	EnabledTypes              []string            `json:"enabled_payment_types"`
	BalanceDisabled           bool                `json:"balance_disabled"`
	BalanceRechargeMultiplier float64             `json:"balance_recharge_multiplier"`
	RechargeBonusEnabled      bool                `json:"recharge_bonus_enabled"`
	RechargeBonusTiers        []RechargeBonusTier `json:"recharge_bonus_tiers"`
	RechargeBonusExpiryMode   string              `json:"recharge_bonus_expiry_mode"`
	RechargeBonusEndsAt       string              `json:"recharge_bonus_ends_at"`
	RechargeBonusDurationDays int                 `json:"recharge_bonus_duration_days"`
	RechargeBonusStartedAt    string              `json:"recharge_bonus_started_at"`
	// SubscriptionUSDToCNYRate 为 0 时订阅换算关闭（兼容存量行为）。
	SubscriptionUSDToCNYRate float64 `json:"subscription_usd_to_cny_rate"`
	RechargeFeeRate          float64 `json:"recharge_fee_rate"`
	LoadBalanceStrategy      string  `json:"load_balance_strategy"`
	ProductNamePrefix        string  `json:"product_name_prefix"`
	ProductNameSuffix        string  `json:"product_name_suffix"`
	HelpImageURL             string  `json:"help_image_url"`
	HelpText                 string  `json:"help_text"`
	StripePublishableKey     string  `json:"stripe_publishable_key,omitempty"`

	// Cancel rate limit settings
	CancelRateLimitEnabled bool   `json:"cancel_rate_limit_enabled"`
	CancelRateLimitMax     int    `json:"cancel_rate_limit_max"`
	CancelRateLimitWindow  int    `json:"cancel_rate_limit_window"`
	CancelRateLimitUnit    string `json:"cancel_rate_limit_unit"`
	CancelRateLimitMode    string `json:"cancel_rate_limit_window_mode"`

	// Force Alipay mobile users to use QR code instead of mobile redirect
	AlipayForceQRCode bool `json:"alipay_force_qrcode"`
	// Use Alipay face-to-face precreate and an app deep link on mobile clients.
	AlipayMobilePrecreateDeepLink bool `json:"alipay_mobile_precreate_deep_link"`
}

// UpdatePaymentConfigRequest contains fields to update payment configuration.
type UpdatePaymentConfigRequest struct {
	Enabled                   *bool                `json:"enabled"`
	MinAmount                 *float64             `json:"min_amount"`
	MaxAmount                 *float64             `json:"max_amount"`
	DailyLimit                *float64             `json:"daily_limit"`
	OrderTimeoutMin           *int                 `json:"order_timeout_minutes"`
	MaxPendingOrders          *int                 `json:"max_pending_orders"`
	EnabledTypes              []string             `json:"enabled_payment_types"`
	BalanceDisabled           *bool                `json:"balance_disabled"`
	BalanceRechargeMultiplier *float64             `json:"balance_recharge_multiplier"`
	RechargeBonusEnabled      *bool                `json:"recharge_bonus_enabled"`
	RechargeBonusTiers        *[]RechargeBonusTier `json:"recharge_bonus_tiers"`
	RechargeBonusExpiryMode   *string              `json:"recharge_bonus_expiry_mode"`
	RechargeBonusEndsAt       *string              `json:"recharge_bonus_ends_at"`
	RechargeBonusDurationDays *int                 `json:"recharge_bonus_duration_days"`
	SubscriptionUSDToCNYRate  *float64             `json:"subscription_usd_to_cny_rate"`
	RechargeFeeRate           *float64             `json:"recharge_fee_rate"`
	LoadBalanceStrategy       *string              `json:"load_balance_strategy"`
	ProductNamePrefix         *string              `json:"product_name_prefix"`
	ProductNameSuffix         *string              `json:"product_name_suffix"`
	HelpImageURL              *string              `json:"help_image_url"`
	HelpText                  *string              `json:"help_text"`

	// Cancel rate limit settings
	CancelRateLimitEnabled *bool   `json:"cancel_rate_limit_enabled"`
	CancelRateLimitMax     *int    `json:"cancel_rate_limit_max"`
	CancelRateLimitWindow  *int    `json:"cancel_rate_limit_window"`
	CancelRateLimitUnit    *string `json:"cancel_rate_limit_unit"`
	CancelRateLimitMode    *string `json:"cancel_rate_limit_window_mode"`

	// Force Alipay mobile users to use QR code instead of mobile redirect
	AlipayForceQRCode *bool `json:"alipay_force_qrcode"`
	// Use Alipay face-to-face precreate and an app deep link on mobile clients.
	AlipayMobilePrecreateDeepLink *bool `json:"alipay_mobile_precreate_deep_link"`

	VisibleMethodAlipaySource  *string `json:"payment_visible_method_alipay_source"`
	VisibleMethodWxpaySource   *string `json:"payment_visible_method_wxpay_source"`
	VisibleMethodAlipayEnabled *bool   `json:"payment_visible_method_alipay_enabled"`
	VisibleMethodWxpayEnabled  *bool   `json:"payment_visible_method_wxpay_enabled"`
}

// MethodLimits holds per-payment-type limits.
type MethodLimits struct {
	PaymentType string  `json:"payment_type"`
	DisplayName string  `json:"display_name,omitempty"`
	Currency    string  `json:"currency"`
	FeeRate     float64 `json:"fee_rate"`
	DailyLimit  float64 `json:"daily_limit"`
	SingleMin   float64 `json:"single_min"`
	SingleMax   float64 `json:"single_max"`
}

// MethodLimitsResponse is the full response for the user-facing /limits API.
// It includes per-method limits and the global widest range (union of all methods).
type MethodLimitsResponse struct {
	Methods   map[string]MethodLimits `json:"methods"`
	GlobalMin float64                 `json:"global_min"` // 0 = no minimum
	GlobalMax float64                 `json:"global_max"` // 0 = no maximum
}

type CreateProviderInstanceRequest struct {
	ProviderKey     string            `json:"provider_key"`
	Name            string            `json:"name"`
	Config          map[string]string `json:"config"`
	SupportedTypes  []string          `json:"supported_types"`
	Enabled         bool              `json:"enabled"`
	PaymentMode     string            `json:"payment_mode"`
	SortOrder       int               `json:"sort_order"`
	Limits          string            `json:"limits"`
	RefundEnabled   bool              `json:"refund_enabled"`
	AllowUserRefund bool              `json:"allow_user_refund"`
}

type UpdateProviderInstanceRequest struct {
	Name            *string           `json:"name"`
	Config          map[string]string `json:"config"`
	SupportedTypes  []string          `json:"supported_types"`
	Enabled         *bool             `json:"enabled"`
	PaymentMode     *string           `json:"payment_mode"`
	SortOrder       *int              `json:"sort_order"`
	Limits          *string           `json:"limits"`
	RefundEnabled   *bool             `json:"refund_enabled"`
	AllowUserRefund *bool             `json:"allow_user_refund"`
}
type CreatePlanRequest struct {
	GroupID       int64    `json:"group_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Price         float64  `json:"price"`
	OriginalPrice *float64 `json:"original_price"`
	Currency      string   `json:"currency"`
	ValidityDays  int      `json:"validity_days"`
	ValidityUnit  string   `json:"validity_unit"`
	Features      string   `json:"features"`
	ProductName   string   `json:"product_name"`
	ForSale       bool     `json:"for_sale"`
	SortOrder     int      `json:"sort_order"`
}

type UpdatePlanRequest struct {
	GroupID       *int64   `json:"group_id"`
	Name          *string  `json:"name"`
	Description   *string  `json:"description"`
	Price         *float64 `json:"price"`
	OriginalPrice *float64 `json:"original_price"`
	Currency      *string  `json:"currency"`
	ValidityDays  *int     `json:"validity_days"`
	ValidityUnit  *string  `json:"validity_unit"`
	Features      *string  `json:"features"`
	ProductName   *string  `json:"product_name"`
	ForSale       *bool    `json:"for_sale"`
	SortOrder     *int     `json:"sort_order"`
}

// PaymentConfigService manages payment configuration and CRUD for
// provider instances, channels, and subscription plans.
type PaymentConfigService struct {
	entClient     *dbent.Client
	settingRepo   SettingRepository
	encryptionKey []byte
}

// NewPaymentConfigService creates a new PaymentConfigService.
func NewPaymentConfigService(entClient *dbent.Client, settingRepo SettingRepository, encryptionKey []byte) *PaymentConfigService {
	return &PaymentConfigService{entClient: entClient, settingRepo: settingRepo, encryptionKey: encryptionKey}
}

// IsPaymentEnabled returns whether the payment system is enabled.
func (s *PaymentConfigService) IsPaymentEnabled(ctx context.Context) bool {
	val, err := s.settingRepo.GetValue(ctx, SettingPaymentEnabled)
	if err != nil {
		return false
	}
	return val == "true"
}

// GetPaymentConfig returns the full payment configuration.
func (s *PaymentConfigService) GetPaymentConfig(ctx context.Context) (*PaymentConfig, error) {
	keys := []string{
		SettingPaymentEnabled, SettingMinRechargeAmount, SettingMaxRechargeAmount,
		SettingDailyRechargeLimit, SettingOrderTimeoutMinutes, SettingMaxPendingOrders,
		SettingEnabledPaymentTypes, SettingBalancePayDisabled, SettingBalanceRechargeMult, SettingRechargeBonusEnabled, SettingRechargeBonusTiers,
		SettingRechargeBonusExpiryMode, SettingRechargeBonusEndsAt, SettingRechargeBonusDurationDays, SettingRechargeBonusStartedAt,
		SettingSubscriptionUSDToCNYRate, SettingRechargeFeeRate, SettingLoadBalanceStrategy,
		SettingProductNamePrefix, SettingProductNameSuffix,
		SettingHelpImageURL, SettingHelpText,
		SettingCancelRateLimitOn, SettingCancelRateLimitMax,
		SettingCancelWindowSize, SettingCancelWindowUnit, SettingCancelWindowMode,
		SettingAlipayForceQRCode, SettingAlipayMobilePrecreateDeepLink,
		SettingPaymentVisibleMethodAlipayEnabled, SettingPaymentVisibleMethodAlipaySource,
		SettingPaymentVisibleMethodWxpayEnabled, SettingPaymentVisibleMethodWxpaySource,
	}
	vals, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get payment config settings: %w", err)
	}
	cfg := s.parsePaymentConfig(vals)
	// Load Stripe publishable key from the first enabled Stripe provider instance
	cfg.StripePublishableKey = s.getStripePublishableKey(ctx)
	return cfg, nil
}

func (s *PaymentConfigService) parsePaymentConfig(vals map[string]string) *PaymentConfig {
	cfg := &PaymentConfig{
		Enabled:                   vals[SettingPaymentEnabled] == "true",
		MinAmount:                 pcParseFloat(vals[SettingMinRechargeAmount], 1),
		MaxAmount:                 pcParseFloat(vals[SettingMaxRechargeAmount], 0),
		DailyLimit:                pcParseFloat(vals[SettingDailyRechargeLimit], 0),
		OrderTimeoutMin:           pcParseInt(vals[SettingOrderTimeoutMinutes], defaultOrderTimeoutMin),
		MaxPendingOrders:          pcParseInt(vals[SettingMaxPendingOrders], defaultMaxPendingOrders),
		BalanceDisabled:           vals[SettingBalancePayDisabled] == "true",
		BalanceRechargeMultiplier: normalizeBalanceRechargeMultiplier(pcParseFloat(vals[SettingBalanceRechargeMult], defaultBalanceRechargeMultiplier)),
		RechargeBonusEnabled:      vals[SettingRechargeBonusEnabled] == "true",
		RechargeBonusTiers:        parseRechargeBonusTiers(vals[SettingRechargeBonusTiers]),
		RechargeBonusExpiryMode:   normalizeRechargeBonusExpiryMode(vals[SettingRechargeBonusExpiryMode]),
		RechargeBonusEndsAt:       strings.TrimSpace(vals[SettingRechargeBonusEndsAt]),
		RechargeBonusDurationDays: pcParseInt(vals[SettingRechargeBonusDurationDays], 0),
		RechargeBonusStartedAt:    strings.TrimSpace(vals[SettingRechargeBonusStartedAt]),
		SubscriptionUSDToCNYRate:  normalizeSubscriptionUSDToCNYRate(pcParseFloat(vals[SettingSubscriptionUSDToCNYRate], 0)),
		RechargeFeeRate:           pcParseFloat(vals[SettingRechargeFeeRate], 0),
		LoadBalanceStrategy:       vals[SettingLoadBalanceStrategy],
		ProductNamePrefix:         vals[SettingProductNamePrefix],
		ProductNameSuffix:         vals[SettingProductNameSuffix],
		HelpImageURL:              vals[SettingHelpImageURL],
		HelpText:                  vals[SettingHelpText],

		CancelRateLimitEnabled: vals[SettingCancelRateLimitOn] == "true",
		CancelRateLimitMax:     pcParseInt(vals[SettingCancelRateLimitMax], 10),
		CancelRateLimitWindow:  pcParseInt(vals[SettingCancelWindowSize], 1),
		CancelRateLimitUnit:    vals[SettingCancelWindowUnit],
		CancelRateLimitMode:    vals[SettingCancelWindowMode],

		AlipayForceQRCode:             vals[SettingAlipayForceQRCode] == "true",
		AlipayMobilePrecreateDeepLink: vals[SettingAlipayMobilePrecreateDeepLink] == "true",
	}
	cfg.AlipayMobilePrecreateDeepLink = pcEnvBoolOverride(
		SettingAlipayMobilePrecreateDeepLink,
		cfg.AlipayMobilePrecreateDeepLink,
	)
	if cfg.LoadBalanceStrategy == "" {
		cfg.LoadBalanceStrategy = payment.DefaultLoadBalanceStrategy
	}
	if raw := vals[SettingEnabledPaymentTypes]; raw != "" {
		types := make([]string, 0, len(strings.Split(raw, ",")))
		for _, t := range strings.Split(raw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				types = append(types, t)
			}
		}
		cfg.EnabledTypes = NormalizeVisibleMethods(types)
	}
	return cfg
}

func pcEnvBoolOverride(key string, fallback bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

const (
	rechargeBonusExpiryModeFixed = "fixed"
	rechargeBonusExpiryModeDays  = "days"
)

type rechargeBonusScheduleUpdate struct {
	mode         string
	endsAt       string
	durationDays int
	startedAt    string
}

func normalizeRechargeBonusExpiryMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case rechargeBonusExpiryModeFixed:
		return rechargeBonusExpiryModeFixed
	case rechargeBonusExpiryModeDays:
		return rechargeBonusExpiryModeDays
	default:
		return ""
	}
}

// IsRechargeBonusActiveAt is the authoritative promotion-window check. A blank
// schedule keeps legacy installations active until an administrator saves one.
func (c *PaymentConfig) IsRechargeBonusActiveAt(now time.Time) bool {
	if c == nil || !c.RechargeBonusEnabled {
		return false
	}
	if strings.TrimSpace(c.RechargeBonusEndsAt) == "" {
		return c.RechargeBonusExpiryMode == ""
	}
	endsAt, err := time.Parse(time.RFC3339, c.RechargeBonusEndsAt)
	if err != nil {
		return false
	}
	return now.Before(endsAt)
}

func (s *PaymentConfigService) prepareRechargeBonusSchedule(ctx context.Context, req UpdatePaymentConfigRequest) (*rechargeBonusScheduleUpdate, error) {
	scheduleProvided := req.RechargeBonusExpiryMode != nil || req.RechargeBonusEndsAt != nil || req.RechargeBonusDurationDays != nil
	if !scheduleProvided && req.RechargeBonusEnabled == nil {
		return nil, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingRechargeBonusEnabled,
		SettingRechargeBonusExpiryMode,
		SettingRechargeBonusEndsAt,
		SettingRechargeBonusDurationDays,
		SettingRechargeBonusStartedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("get recharge bonus schedule: %w", err)
	}
	current := s.parsePaymentConfig(values)
	enabledAfter := current.RechargeBonusEnabled
	if req.RechargeBonusEnabled != nil {
		enabledAfter = *req.RechargeBonusEnabled
	}
	mode := current.RechargeBonusExpiryMode
	if req.RechargeBonusExpiryMode != nil {
		mode = normalizeRechargeBonusExpiryMode(*req.RechargeBonusExpiryMode)
		if mode == "" {
			return nil, infraerrors.BadRequest("INVALID_RECHARGE_BONUS_EXPIRY_MODE", "recharge bonus expiry mode must be fixed or days")
		}
	}
	// Preserve legacy no-expiry behavior until a schedule is explicitly supplied.
	if mode == "" && !scheduleProvided {
		return nil, nil
	}
	now := time.Now().UTC().Truncate(time.Second)
	result := &rechargeBonusScheduleUpdate{
		mode:         mode,
		endsAt:       current.RechargeBonusEndsAt,
		durationDays: current.RechargeBonusDurationDays,
		startedAt:    current.RechargeBonusStartedAt,
	}
	switch mode {
	case rechargeBonusExpiryModeFixed:
		if req.RechargeBonusEndsAt != nil {
			result.endsAt = strings.TrimSpace(*req.RechargeBonusEndsAt)
		}
		if result.endsAt == "" {
			if !enabledAfter {
				result.durationDays = 0
				return result, nil
			}
			return nil, infraerrors.BadRequest("INVALID_RECHARGE_BONUS_ENDS_AT", "recharge bonus end time is required")
		}
		endsAt, parseErr := time.Parse(time.RFC3339, result.endsAt)
		if parseErr != nil {
			return nil, infraerrors.BadRequest("INVALID_RECHARGE_BONUS_ENDS_AT", "recharge bonus end time must be RFC3339")
		}
		if enabledAfter && !endsAt.After(now) {
			return nil, infraerrors.BadRequest("INVALID_RECHARGE_BONUS_ENDS_AT", "recharge bonus end time must be in the future")
		}
		result.endsAt = endsAt.UTC().Format(time.RFC3339)
		result.durationDays = 0
		if result.startedAt == "" || (!current.RechargeBonusEnabled && enabledAfter) {
			result.startedAt = now.Format(time.RFC3339)
		}
	case rechargeBonusExpiryModeDays:
		if req.RechargeBonusDurationDays != nil {
			result.durationDays = *req.RechargeBonusDurationDays
		}
		if result.durationDays < 1 || result.durationDays > 3650 {
			return nil, infraerrors.BadRequest("INVALID_RECHARGE_BONUS_DURATION_DAYS", "recharge bonus duration days must be between 1 and 3650")
		}
		restart := current.RechargeBonusExpiryMode != rechargeBonusExpiryModeDays ||
			current.RechargeBonusDurationDays != result.durationDays ||
			(!current.RechargeBonusEnabled && enabledAfter) || current.RechargeBonusStartedAt == ""
		if enabledAfter && restart {
			result.startedAt = now.Format(time.RFC3339)
			result.endsAt = now.AddDate(0, 0, result.durationDays).Format(time.RFC3339)
		}
	}
	return result, nil
}

// getStripePublishableKey finds the publishable key from the first enabled Stripe provider instance.
func (s *PaymentConfigService) getStripePublishableKey(ctx context.Context) string {
	if s.entClient == nil {
		return ""
	}
	instances, err := s.entClient.PaymentProviderInstance.Query().
		Where(
			paymentproviderinstance.EnabledEQ(true),
			paymentproviderinstance.ProviderKeyEQ(payment.TypeStripe),
		).Limit(1).All(ctx)
	if err != nil || len(instances) == 0 {
		return ""
	}
	cfg, err := s.decryptConfig(instances[0].Config)
	if err != nil || cfg == nil {
		return ""
	}
	return cfg[payment.ConfigKeyPublishableKey]
}

// UpdatePaymentConfig updates the payment configuration settings.
// NOTE: This function exceeds 30 lines because each field requires an independent
// nil-check before serialisation — this is inherent to patch-style update patterns
// and cannot be meaningfully decomposed without introducing unnecessary abstraction.
func (s *PaymentConfigService) UpdatePaymentConfig(ctx context.Context, req UpdatePaymentConfigRequest) error {
	if req.BalanceRechargeMultiplier != nil {
		if math.IsNaN(*req.BalanceRechargeMultiplier) || math.IsInf(*req.BalanceRechargeMultiplier, 0) || *req.BalanceRechargeMultiplier <= 0 {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_MULTIPLIER", "balance recharge multiplier must be greater than 0")
		}
	}
	if req.SubscriptionUSDToCNYRate != nil {
		v := *req.SubscriptionUSDToCNYRate
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return infraerrors.BadRequest("INVALID_SUBSCRIPTION_USD_TO_CNY_RATE", "subscription USD to CNY rate must be 0 (disabled) or a positive number")
		}
	}
	var normalizedBonusTiers []RechargeBonusTier
	if req.RechargeBonusTiers != nil {
		var err error
		normalizedBonusTiers, err = normalizeRechargeBonusTiers(*req.RechargeBonusTiers)
		if err != nil {
			return infraerrors.BadRequest("INVALID_RECHARGE_BONUS_TIERS", err.Error())
		}
	}
	rechargeSchedule, err := s.prepareRechargeBonusSchedule(ctx, req)
	if err != nil {
		return err
	}
	if req.RechargeFeeRate != nil {
		v := *req.RechargeFeeRate
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
			return infraerrors.BadRequest("INVALID_RECHARGE_FEE_RATE", "recharge fee rate must be between 0 and 100")
		}
		// Enforce max 2 decimal places
		if math.Round(v*100) != v*100 {
			return infraerrors.BadRequest("INVALID_RECHARGE_FEE_RATE", "recharge fee rate allows at most 2 decimal places")
		}
	}
	m := make(map[string]string)
	if req.Enabled != nil {
		m[SettingPaymentEnabled] = formatBoolOrEmpty(req.Enabled)
	}
	if req.MinAmount != nil {
		m[SettingMinRechargeAmount] = formatPositiveFloat(req.MinAmount)
	}
	if req.MaxAmount != nil {
		m[SettingMaxRechargeAmount] = formatPositiveFloat(req.MaxAmount)
	}
	if req.DailyLimit != nil {
		m[SettingDailyRechargeLimit] = formatPositiveFloat(req.DailyLimit)
	}
	if req.OrderTimeoutMin != nil {
		m[SettingOrderTimeoutMinutes] = formatPositiveInt(req.OrderTimeoutMin)
	}
	if req.MaxPendingOrders != nil {
		m[SettingMaxPendingOrders] = formatPositiveInt(req.MaxPendingOrders)
	}
	if req.EnabledTypes != nil {
		m[SettingEnabledPaymentTypes] = strings.Join(req.EnabledTypes, ",")
	}
	if req.BalanceDisabled != nil {
		m[SettingBalancePayDisabled] = formatBoolOrEmpty(req.BalanceDisabled)
	}
	if req.BalanceRechargeMultiplier != nil {
		m[SettingBalanceRechargeMult] = formatPositiveFloat(req.BalanceRechargeMultiplier)
	}
	if req.RechargeBonusEnabled != nil {
		m[SettingRechargeBonusEnabled] = formatBoolOrEmpty(req.RechargeBonusEnabled)
	}
	if req.RechargeBonusTiers != nil {
		encoded, err := json.Marshal(normalizedBonusTiers)
		if err != nil {
			return fmt.Errorf("encode recharge bonus tiers: %w", err)
		}
		m[SettingRechargeBonusTiers] = string(encoded)
	}
	if rechargeSchedule != nil {
		m[SettingRechargeBonusExpiryMode] = rechargeSchedule.mode
		m[SettingRechargeBonusEndsAt] = rechargeSchedule.endsAt
		m[SettingRechargeBonusDurationDays] = strconv.Itoa(rechargeSchedule.durationDays)
		m[SettingRechargeBonusStartedAt] = rechargeSchedule.startedAt
	}
	if req.SubscriptionUSDToCNYRate != nil {
		m[SettingSubscriptionUSDToCNYRate] = formatPositiveFloatExact(req.SubscriptionUSDToCNYRate)
	}
	if req.RechargeFeeRate != nil {
		m[SettingRechargeFeeRate] = formatNonNegativeFloat(req.RechargeFeeRate)
	}
	if req.LoadBalanceStrategy != nil {
		m[SettingLoadBalanceStrategy] = derefStr(req.LoadBalanceStrategy)
	}
	if req.ProductNamePrefix != nil {
		m[SettingProductNamePrefix] = derefStr(req.ProductNamePrefix)
	}
	if req.ProductNameSuffix != nil {
		m[SettingProductNameSuffix] = derefStr(req.ProductNameSuffix)
	}
	if req.HelpImageURL != nil {
		m[SettingHelpImageURL] = derefStr(req.HelpImageURL)
	}
	if req.HelpText != nil {
		m[SettingHelpText] = derefStr(req.HelpText)
	}
	if req.CancelRateLimitEnabled != nil {
		m[SettingCancelRateLimitOn] = formatBoolOrEmpty(req.CancelRateLimitEnabled)
	}
	if req.CancelRateLimitMax != nil {
		m[SettingCancelRateLimitMax] = formatPositiveInt(req.CancelRateLimitMax)
	}
	if req.CancelRateLimitWindow != nil {
		m[SettingCancelWindowSize] = formatPositiveInt(req.CancelRateLimitWindow)
	}
	if req.CancelRateLimitUnit != nil {
		m[SettingCancelWindowUnit] = derefStr(req.CancelRateLimitUnit)
	}
	if req.CancelRateLimitMode != nil {
		m[SettingCancelWindowMode] = derefStr(req.CancelRateLimitMode)
	}
	if req.AlipayForceQRCode != nil {
		m[SettingAlipayForceQRCode] = formatBoolOrEmpty(req.AlipayForceQRCode)
	}
	if req.AlipayMobilePrecreateDeepLink != nil {
		m[SettingAlipayMobilePrecreateDeepLink] = formatBoolOrEmpty(req.AlipayMobilePrecreateDeepLink)
	}
	if req.VisibleMethodAlipaySource != nil {
		m[SettingPaymentVisibleMethodAlipaySource] = derefStr(req.VisibleMethodAlipaySource)
	}
	if req.VisibleMethodWxpaySource != nil {
		m[SettingPaymentVisibleMethodWxpaySource] = derefStr(req.VisibleMethodWxpaySource)
	}
	if req.VisibleMethodAlipayEnabled != nil {
		m[SettingPaymentVisibleMethodAlipayEnabled] = formatBoolOrEmpty(req.VisibleMethodAlipayEnabled)
	}
	if req.VisibleMethodWxpayEnabled != nil {
		m[SettingPaymentVisibleMethodWxpayEnabled] = formatBoolOrEmpty(req.VisibleMethodWxpayEnabled)
	}
	return s.settingRepo.SetMultiple(ctx, m)
}

func formatBoolOrEmpty(v *bool) string {
	if v == nil {
		return ""
	}
	return strconv.FormatBool(*v)
}

func formatPositiveFloat(v *float64) string {
	if v == nil || *v <= 0 {
		return "" // empty → parsePaymentConfig uses default
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

// formatPositiveFloatExact 保留完整精度，用于汇率等对小数位敏感的配置。
func formatPositiveFloatExact(v *float64) string {
	if v == nil || *v <= 0 {
		return "" // empty → parsePaymentConfig 视为未配置（换算关闭）
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func formatNonNegativeFloat(v *float64) string {
	if v == nil || *v < 0 {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

func formatPositiveInt(v *int) string {
	if v == nil || *v <= 0 {
		return ""
	}
	return strconv.Itoa(*v)
}

func derefStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func splitTypes(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func joinTypes(types []string) string {
	return strings.Join(types, ",")
}

func pcParseFloat(s string, defaultVal float64) float64 {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

func pcParseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func buildVisibleMethodSourceAvailability(instances []*dbent.PaymentProviderInstance) map[string]bool {
	available := make(map[string]bool, 4)
	for _, inst := range instances {
		switch inst.ProviderKey {
		case payment.TypeAlipay:
			if inst.SupportedTypes == "" || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeAlipay) || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeAlipayDirect) {
				available[VisibleMethodSourceOfficialAlipay] = true
			}
		case payment.TypeWxpay:
			if inst.SupportedTypes == "" || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeWxpay) || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeWxpayDirect) {
				available[VisibleMethodSourceOfficialWechat] = true
			}
		case payment.TypeEasyPay:
			for _, supportedType := range splitTypes(inst.SupportedTypes) {
				switch NormalizeVisibleMethod(supportedType) {
				case payment.TypeAlipay:
					available[VisibleMethodSourceEasyPayAlipay] = true
				case payment.TypeWxpay:
					available[VisibleMethodSourceEasyPayWechat] = true
				}
			}
		}
	}
	return available
}

func applyVisibleMethodRoutingToEnabledTypes(base []string, vals map[string]string, available map[string]bool) []string {
	shouldExpose := map[string]bool{
		payment.TypeAlipay: visibleMethodShouldBeExposed(payment.TypeAlipay, vals, available),
		payment.TypeWxpay:  visibleMethodShouldBeExposed(payment.TypeWxpay, vals, available),
	}

	seen := make(map[string]struct{}, len(base)+2)
	out := make([]string, 0, len(base)+2)
	appendType := func(paymentType string) {
		paymentType = NormalizeVisibleMethod(paymentType)
		if paymentType == "" {
			return
		}
		if _, ok := seen[paymentType]; ok {
			return
		}
		seen[paymentType] = struct{}{}
		out = append(out, paymentType)
	}

	for _, paymentType := range base {
		visibleMethod := NormalizeVisibleMethod(paymentType)
		switch visibleMethod {
		case payment.TypeAlipay, payment.TypeWxpay:
			if shouldExpose[visibleMethod] {
				appendType(visibleMethod)
			}
		default:
			appendType(visibleMethod)
		}
	}

	for _, visibleMethod := range []string{payment.TypeAlipay, payment.TypeWxpay} {
		if shouldExpose[visibleMethod] {
			appendType(visibleMethod)
		}
	}
	return out
}

func visibleMethodShouldBeExposed(method string, vals map[string]string, available map[string]bool) bool {
	enabledKey := visibleMethodEnabledSettingKey(method)
	sourceKey := visibleMethodSourceSettingKey(method)
	if enabledKey == "" || sourceKey == "" || vals[enabledKey] != "true" {
		return false
	}
	source := NormalizeVisibleMethodSource(method, vals[sourceKey])
	return source != "" && available[source]
}
