package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

func grokBaseURLValidator(account *Account, cfg *config.Config) (xai.BaseURLValidator, error) {
	if account == nil || !account.IsGrok() {
		return nil, fmt.Errorf("grok account is required")
	}
	switch account.Type {
	case AccountTypeOAuth:
		// Official gateway hosts are always trusted and always usable, even when
		// the operator enables a restrictive URL allowlist. A custom forwarding
		// host is vetted by the same operator policy as API-key accounts.
		//
		// The official-vs-custom decision is made on the host, not via
		// ValidateTrustedBaseURL: that validator relaxes to accept-any under the
		// XAI_ALLOW_UNSAFE_URL_OVERRIDES debug switch, which must never let an
		// OAuth bearer token reach an arbitrary custom host.
		policyValidator := grokOperatorPolicyValidator(cfg)
		return redactedGrokBaseURLValidator(func(raw string) (string, error) {
			if xai.IsOfficialBaseURL(raw) {
				return xai.ValidateTrustedBaseURL(raw)
			}
			return policyValidator(raw)
		}), nil
	case AccountTypeAPIKey:
		return redactedGrokBaseURLValidator(grokOperatorPolicyValidator(cfg)), nil
	default:
		return nil, fmt.Errorf("unsupported grok account type: %s", account.Type)
	}
}

// grokOperatorPolicyValidator 按全局出站 URL 安全策略校验自定义 base_url：
// 白名单开启时强制 UpstreamHosts；关闭时仅做格式校验（HTTP 允许与否跟随配置）。
func grokOperatorPolicyValidator(cfg *config.Config) xai.BaseURLValidator {
	if cfg == nil {
		return xai.ValidateBaseURL
	}
	if !cfg.Security.URLAllowlist.Enabled {
		return func(raw string) (string, error) {
			return urlvalidator.ValidateURLFormat(raw, cfg.Security.URLAllowlist.AllowInsecureHTTP)
		}
	}
	return func(raw string) (string, error) {
		return urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{
			AllowedHosts:     cfg.Security.URLAllowlist.UpstreamHosts,
			RequireAllowlist: true,
			AllowPrivate:     cfg.Security.URLAllowlist.AllowPrivateHosts,
		})
	}
}

func redactedGrokBaseURLValidator(validator xai.BaseURLValidator) xai.BaseURLValidator {
	return func(raw string) (string, error) {
		validated, err := validator(raw)
		if err != nil {
			return "", errors.New("base URL rejected by URL security policy")
		}
		return validated, nil
	}
}

func buildGrokResponsesURL(account *Account, cfg *config.Config) (string, error) {
	validator, err := grokBaseURLValidator(account, cfg)
	if err != nil {
		return "", err
	}
	return xai.BuildResponsesURLWithValidator(account.GetGrokBaseURL(), validator)
}

func buildGrokChatCompletionsURL(account *Account, cfg *config.Config) (string, error) {
	validator, err := grokBaseURLValidator(account, cfg)
	if err != nil {
		return "", err
	}
	return xai.BuildChatCompletionsURLWithValidator(account.GetGrokBaseURL(), validator)
}

// buildGrokBillingURL 解析 billing 探测端点：跟随账号的转发 base_url，
// 未定制的账号仍指向官方 CLI 网关。
func buildGrokBillingURL(account *Account, cfg *config.Config, weekly bool) (string, error) {
	validator, err := grokBaseURLValidator(account, cfg)
	if err != nil {
		return "", err
	}
	return xai.BuildBillingURLWithValidator(account.GetGrokBaseURL(), weekly, validator)
}

func buildGrokMediaURL(account *Account, cfg *config.Config, endpoint GrokMediaEndpoint, requestID string) (string, error) {
	if account != nil && account.IsSeedance() {
		return buildSeedanceMediaURL(account, cfg, endpoint, requestID)
	}
	validator, err := grokBaseURLValidator(account, cfg)
	if err != nil {
		return "", err
	}
	baseURL := account.GetGrokMediaBaseURL()
	switch endpoint {
	case GrokMediaEndpointImagesGenerations:
		return xai.BuildImagesGenerationsURLWithValidator(baseURL, validator)
	case GrokMediaEndpointImagesEdits:
		return xai.BuildImagesEditsURLWithValidator(baseURL, validator)
	case GrokMediaEndpointVideosGenerations:
		return xai.BuildVideosGenerationsURLWithValidator(baseURL, validator)
	case GrokMediaEndpointVideosEdits:
		return xai.BuildVideosEditsURLWithValidator(baseURL, validator)
	case GrokMediaEndpointVideosExtensions:
		return xai.BuildVideosExtensionsURLWithValidator(baseURL, validator)
	case GrokMediaEndpointVideoStatus:
		return xai.BuildVideoURLWithValidator(baseURL, requestID, validator)
	case GrokMediaEndpointVideoContent:
		videoURL, err := xai.BuildVideoURLWithValidator(baseURL, requestID, validator)
		if err != nil {
			return "", err
		}
		return videoURL + "/content", nil
	default:
		return "", fmt.Errorf("unsupported grok media endpoint: %s", endpoint)
	}
}

func buildSeedanceMediaURL(account *Account, cfg *config.Config, endpoint GrokMediaEndpoint, requestID string) (string, error) {
	if account == nil || !account.IsSeedance() {
		return "", fmt.Errorf("seedance account is required")
	}
	base := strings.TrimRight(account.GetSeedanceBaseURL(), "/")
	if base == "" {
		return "", fmt.Errorf("seedance base URL is required")
	}
	validator := grokOperatorPolicyValidator(cfg)
	validated, err := validator(base)
	if err != nil {
		return "", errors.New("base URL rejected by URL security policy")
	}
	parsed, err := url.Parse(strings.TrimRight(validated, "/"))
	if err != nil || parsed.Hostname() == "" {
		return "", errors.New("seedance base URL is invalid")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, "/v1") {
		path += "/v1"
	}
	suffix := "/video/generations"
	rawSuffix := suffix
	switch endpoint {
	case GrokMediaEndpointVideoStatus:
		requestID = strings.TrimSpace(requestID)
		if requestID == "" {
			return "", errors.New("seedance request ID is required")
		}
		suffix += "/" + requestID
		rawSuffix += "/" + url.PathEscape(requestID)
	case GrokMediaEndpointVideoContent:
		requestID = strings.TrimSpace(requestID)
		if requestID == "" {
			return "", errors.New("seedance request ID is required")
		}
		// Qingfeng's Seedance-compatible relay keeps the BytePlus status
		// endpoint at /v1/video/generations/:id, but exposes the authenticated
		// binary proxy at /v1/videos/:id/content.  These are deliberately
		// different routes; using the status route here returns JSON instead of
		// the generated media bytes.
		suffix = "/videos/" + requestID + "/content"
		rawSuffix = "/videos/" + url.PathEscape(requestID) + "/content"
	default:
		if endpoint != GrokMediaEndpointVideosGenerations {
			return "", fmt.Errorf("unsupported seedance media endpoint: %s", endpoint)
		}
	}
	parsed.Path = strings.TrimRight(path, "/") + suffix
	parsed.RawPath = strings.TrimRight(path, "/") + rawSuffix
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}
