package service

import (
	"context"
	"log/slog"
	"strings"
)

// DefaultCreateAccountProxyName is the operator-chosen default for new and
// imported accounts. Looked up by name so each site can keep its own ID.
const DefaultCreateAccountProxyName = "JP-WARP-SSH-103"

func proxyIDProvided(id *int64) bool {
	return id != nil && *id > 0
}

func (s *adminServiceImpl) applyDefaultCreateAttachSubPools(input *CreateAccountInput) {
	if input == nil || s == nil || s.subPoolAttacher == nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(input.AttachSubPools))
	if mode == AttachSubPoolsBoth {
		return
	}
	input.AttachSubPools = AttachSubPoolsFormal
}

func (s *adminServiceImpl) applyDefaultCreateProxy(ctx context.Context, input *CreateAccountInput) {
	if input == nil || proxyIDProvided(input.ProxyID) || s == nil || s.proxyRepo == nil {
		return
	}
	proxies, err := s.proxyRepo.ListActive(ctx)
	if err != nil {
		slog.Warn("default_create_proxy_list_failed", "error", err)
		return
	}
	for i := range proxies {
		if !strings.EqualFold(strings.TrimSpace(proxies[i].Name), DefaultCreateAccountProxyName) {
			continue
		}
		id := proxies[i].ID
		input.ProxyID = &id
		return
	}
}
