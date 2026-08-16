package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type upstreamCostConfigAdminRepo struct {
	*upstreamBillingProbeAccountRepo
	createdAccount     *Account
	createdCostInputs  []UpstreamCostVersionInput
	createdBy          int64
	replacedCostInputs []UpstreamCostVersionInput
	replacedEnabled    *bool
}

func (r *upstreamCostConfigAdminRepo) CreateWithUpstreamCostConfig(_ context.Context, account *Account, costInputs []UpstreamCostVersionInput, createdBy int64) error {
	r.createdAccount = account
	r.createdCostInputs = costInputs
	r.createdBy = createdBy
	return r.Create(context.Background(), account)
}

func (r *upstreamCostConfigAdminRepo) ReplaceUpstreamCostProfiles(_ context.Context, accountID int64, costInputs []UpstreamCostVersionInput, enabled *bool, createdBy int64) error {
	r.replacedCostInputs = costInputs
	r.replacedEnabled = enabled
	return nil
}

func lunaProfile() UpstreamCostProfileInput {
	return UpstreamCostProfileInput{
		Model:           "gpt-5.6-luna",
		ShortPrices:     UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		LongPrices:      UpstreamCostPrices{Input: 1, CacheRead: 0.1, CacheWrite: 1, Output: 6},
		BalanceUnitCost: 1,
	}
}

func TestCreateAccountWithUpstreamCostConfig(t *testing.T) {
	enabled := true
	repo := &upstreamCostConfigAdminRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{}}
	svc := &adminServiceImpl{accountRepo: repo}

	created, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "openai-cost",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test"},
		SkipDefaultGroupBind: true,
		UpstreamCostEnabled:  &enabled,
		UpstreamCostProfiles: []UpstreamCostProfileInput{
			lunaProfile(),
			{Model: "gpt-5.6-terra", ShortPrices: UpstreamCostPrices{Input: 2, Output: 12}, LongPrices: UpstreamCostPrices{Input: 2, Output: 12}, BalanceUnitCost: 1},
		},
		CreatedBy: 42,
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotNil(t, repo.createdAccount)
	require.Equal(t, true, repo.createdAccount.Extra[UpstreamCostEnabledExtraKey])
	require.Len(t, repo.createdCostInputs, 2)
	require.Equal(t, "gpt-5.6-luna", repo.createdCostInputs[0].Model)
	require.Equal(t, int64(42), repo.createdBy)
}

func TestCreateAccountWithUpstreamCostConfigRejectsNonOpenAI(t *testing.T) {
	repo := &upstreamCostConfigAdminRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{}}
	svc := &adminServiceImpl{accountRepo: repo}

	_, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "gemini-cost",
		Platform:             PlatformGemini,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test"},
		SkipDefaultGroupBind: true,
		UpstreamCostProfiles: []UpstreamCostProfileInput{lunaProfile()},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "requires an OpenAI account")
	require.Nil(t, repo.createdAccount)
}

func TestCreateAccountWithUpstreamCostConfigRejectsDuplicateModel(t *testing.T) {
	repo := &upstreamCostConfigAdminRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{}}
	svc := &adminServiceImpl{accountRepo: repo}

	_, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "dup-cost",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test"},
		SkipDefaultGroupBind: true,
		UpstreamCostProfiles: []UpstreamCostProfileInput{lunaProfile(), lunaProfile()},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate upstream cost profile")
}

func TestUpdateAccountWithUpstreamCostProfiles(t *testing.T) {
	enabled := false
	repo := &upstreamCostConfigAdminRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{
		77: {ID: 77, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Extra: map[string]any{}},
	}}}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 77, &UpdateAccountInput{
		Name:                 "edited",
		UpstreamCostEnabled:  &enabled,
		UpstreamCostProfiles: []UpstreamCostProfileInput{lunaProfile()},
		CreatedBy:            7,
	})

	require.NoError(t, err)
	require.Equal(t, "edited", updated.Name)
	require.Len(t, repo.replacedCostInputs, 1)
	require.Equal(t, "gpt-5.6-luna", repo.replacedCostInputs[0].Model)
	require.NotNil(t, repo.replacedEnabled)
	require.Equal(t, false, *repo.replacedEnabled)
}

func TestUpdateAccountWithUpstreamCostProfilesRejectsNonOpenAI(t *testing.T) {
	repo := &upstreamCostConfigAdminRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{
		78: {ID: 78, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive, Extra: map[string]any{}},
	}}}
	svc := &adminServiceImpl{accountRepo: repo}

	_, err := svc.UpdateAccount(context.Background(), 78, &UpdateAccountInput{
		UpstreamCostProfiles: []UpstreamCostProfileInput{lunaProfile()},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "requires an OpenAI account")
	require.Nil(t, repo.replacedCostInputs)
}

func TestValidateUpstreamCostProfiles(t *testing.T) {
	require.NoError(t, ValidateUpstreamCostProfiles(PlatformOpenAI, nil, nil))

	enabled := true
	require.NoError(t, ValidateUpstreamCostProfiles(PlatformOpenAI, &enabled, []UpstreamCostProfileInput{lunaProfile()}))
	require.Error(t, ValidateUpstreamCostProfiles(PlatformGemini, &enabled, nil))
	require.Error(t, ValidateUpstreamCostProfiles(PlatformOpenAI, &enabled, []UpstreamCostProfileInput{{Model: "gpt-5.5"}}))

	badPrices := lunaProfile()
	badPrices.ShortPrices = UpstreamCostPrices{Input: -1, Output: 6}
	require.Error(t, ValidateUpstreamCostProfiles(PlatformOpenAI, &enabled, []UpstreamCostProfileInput{badPrices}))
}

// 修复：编辑时只切换开关（不提供 profiles）不得清空价格集合，
// 也不得报错——costInputs 应为 nil（未提供），enabled 单独传递。
func TestUpdateAccountToggleOnlyKeepsProfiles(t *testing.T) {
	enabled := false
	repo := &upstreamCostConfigAdminRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{
		79: {ID: 79, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Extra: map[string]any{}},
	}}}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 79, &UpdateAccountInput{
		UpstreamCostEnabled: &enabled,
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	// costInputs 必须为 nil（未提供价格集合），只传 enabled。
	require.Nil(t, repo.replacedCostInputs)
	require.NotNil(t, repo.replacedEnabled)
	require.Equal(t, false, *repo.replacedEnabled)
}
