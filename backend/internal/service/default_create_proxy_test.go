package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type defaultProxyRepoStub struct {
	ProxyRepository
	proxies []Proxy
	err     error
}

func (s *defaultProxyRepoStub) ListActive(context.Context) ([]Proxy, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.proxies, nil
}

func TestApplyDefaultCreateProxyFillsMissingProxy(t *testing.T) {
	svc := &adminServiceImpl{proxyRepo: &defaultProxyRepoStub{proxies: []Proxy{
		{ID: 2, Name: "SG-OpenAI-01"},
		{ID: 5, Name: "JP-WARP-SSH-103"},
	}}}
	input := &CreateAccountInput{}
	svc.applyDefaultCreateProxy(context.Background(), input)
	require.NotNil(t, input.ProxyID)
	require.Equal(t, int64(5), *input.ProxyID)
}

func TestApplyDefaultCreateProxyKeepsExplicitProxy(t *testing.T) {
	chosen := int64(2)
	svc := &adminServiceImpl{proxyRepo: &defaultProxyRepoStub{proxies: []Proxy{
		{ID: 5, Name: "JP-WARP-SSH-103"},
	}}}
	input := &CreateAccountInput{ProxyID: &chosen}
	svc.applyDefaultCreateProxy(context.Background(), input)
	require.Equal(t, int64(2), *input.ProxyID)
}

func TestApplyDefaultCreateAttachSubPools(t *testing.T) {
	svc := &adminServiceImpl{subPoolAttacher: &SubPoolService{}}
	input := &CreateAccountInput{}
	svc.applyDefaultCreateAttachSubPools(input)
	require.Equal(t, AttachSubPoolsFormal, input.AttachSubPools)

	input = &CreateAccountInput{AttachSubPools: "none"}
	svc.applyDefaultCreateAttachSubPools(input)
	require.Equal(t, "none", input.AttachSubPools)

	svc = &adminServiceImpl{}
	input = &CreateAccountInput{}
	svc.applyDefaultCreateAttachSubPools(input)
	require.Empty(t, input.AttachSubPools)
}

func TestCreateAccountUsesDefaultProxyWhenUnset(t *testing.T) {
	repo := &upstreamBillingProbeAccountRepo{}
	svc := &adminServiceImpl{
		accountRepo: repo,
		proxyRepo: &defaultProxyRepoStub{proxies: []Proxy{
			{ID: 5, Name: "JP-WARP-SSH-103"},
		}},
	}
	created, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "donna",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test"},
		SkipDefaultGroupBind: true,
	})
	require.NoError(t, err)
	require.NotNil(t, created.ProxyID)
	require.Equal(t, int64(5), *created.ProxyID)
}
