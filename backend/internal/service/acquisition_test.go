package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveAcquisitionDefaultsToOfficial(t *testing.T) {
	result := ResolveAcquisition(AcquisitionTouch{}, AcquisitionResolveOptions{})
	require.Equal(t, AcquisitionClassOfficial, result.Class)
	require.Equal(t, AcquisitionChannelOfficial, result.ChannelCode)
	require.Equal(t, "default_official", result.Reason)
	require.Equal(t, AcquisitionLevelOfficial, result.Level)
}

func TestResolveAcquisitionOwnSiteReferrerIsOfficial(t *testing.T) {
	touch := AcquisitionTouch{ReferrerHost: "www.anytoken.work", LandingPath: "/pricing"}
	result := ResolveAcquisition(touch, AcquisitionResolveOptions{SiteHosts: []string{"anytoken.work:443"}})
	require.Equal(t, AcquisitionClassOfficial, result.Class)
}

func TestResolveAcquisitionInvalidSourceFallsThrough(t *testing.T) {
	// 填错渠道码：不丢用户，继续往后判；没有其他证据就是官网。
	result := ResolveAcquisition(AcquisitionTouch{Source: "NOPE"}, AcquisitionResolveOptions{SourceChannel: nil})
	require.Equal(t, AcquisitionClassOfficial, result.Class)
	require.Equal(t, "invalid_source_default_official", result.Reason)

	// 填错渠道码但 referrer 是百度：SEO。
	result = ResolveAcquisition(AcquisitionTouch{Source: "NOPE", ReferrerHost: "www.baidu.com"}, AcquisitionResolveOptions{})
	require.Equal(t, AcquisitionClassSEO, result.Class)
	require.Equal(t, "baidu", result.Engine)
}

func TestResolveAcquisitionDisabledSourceIgnored(t *testing.T) {
	channel := &PromotionChannel{ID: 9, Code: "TG1", ChannelType: AcquisitionClassOther, Enabled: false}
	result := ResolveAcquisition(AcquisitionTouch{Source: "TG1", AffCode: "ABC"}, AcquisitionResolveOptions{SourceChannel: channel})
	require.Equal(t, AcquisitionClassInvite, result.Class, "停用渠道当作无效，落到 aff")
}

func TestResolveAcquisitionSourceBeatsAffAndSearch(t *testing.T) {
	channel := &PromotionChannel{ID: 9, Code: "TG1", ChannelType: AcquisitionClassOther, Enabled: true}
	touch := AcquisitionTouch{Source: "TG1", AffCode: "ABC", ReferrerHost: "www.google.com"}
	result := ResolveAcquisition(touch, AcquisitionResolveOptions{SourceChannel: channel})
	require.Equal(t, AcquisitionClassOther, result.Class)
	require.Equal(t, "TG1", result.ChannelCode)
	require.Equal(t, AcquisitionLevelSource, result.Level)

	// 人工渠道类型是 seo（比如买的软文）：分类跟渠道走。
	channel.ChannelType = AcquisitionClassSEO
	result = ResolveAcquisition(touch, AcquisitionResolveOptions{SourceChannel: channel})
	require.Equal(t, AcquisitionClassSEO, result.Class)
}

func TestResolveAcquisitionAffIsInvite(t *testing.T) {
	result := ResolveAcquisition(AcquisitionTouch{AffCode: "ABC", ReferrerHost: "www.baidu.com"}, AcquisitionResolveOptions{})
	require.Equal(t, AcquisitionClassInvite, result.Class)
	require.Equal(t, AcquisitionChannelInvite, result.ChannelCode)
}

func TestResolveAcquisitionSearchEngines(t *testing.T) {
	for host, engine := range map[string]string{
		"www.baidu.com":      "baidu",
		"m.baidu.com":        "baidu",
		"www.google.com.hk":  "google",
		"google.co.jp":       "google",
		"cn.bing.com":        "bing",
		"www.sogou.com":      "sogou",
		"www.so.com":         "360",
		"m.sm.cn":            "shenma",
		"chatgpt.com":        "chatgpt",
		"www.perplexity.ai":  "perplexity",
		"kimi.moonshot.cn":   "kimi",
		"www.doubao.com":     "doubao",
		"metaso.cn":          "metaso",
		"duckduckgo.com":     "duckduckgo",
		"yandex.ru":          "yandex",
		"search.yahoo.co.jp": "yahoo",
	} {
		result := ResolveAcquisition(AcquisitionTouch{ReferrerHost: host}, AcquisitionResolveOptions{})
		require.Equal(t, AcquisitionClassSEO, result.Class, host)
		require.Equal(t, engine, result.Engine, host)
	}
	// 不是搜索引擎的外站
	result := ResolveAcquisition(AcquisitionTouch{ReferrerHost: "t.me"}, AcquisitionResolveOptions{})
	require.Equal(t, AcquisitionClassOther, result.Class)
	require.Equal(t, AcquisitionChannelExternal, result.ChannelCode)
	require.Equal(t, "external_referrer", result.Reason)
	// google 出现在路径里不算
	_, ok := DetectSearchEngine("notgoogle.com")
	require.False(t, ok)
}

func TestResolveAcquisitionPaidIsNotSEO(t *testing.T) {
	// Google Ads 自动标记 gclid：付费，不算 SEO
	result := ResolveAcquisition(AcquisitionTouch{ReferrerHost: "www.google.com", PaidClick: true}, AcquisitionResolveOptions{})
	require.Equal(t, AcquisitionClassOther, result.Class)
	require.True(t, result.Paid)
	require.Equal(t, "paid_click", result.Reason)

	result = ResolveAcquisition(AcquisitionTouch{ReferrerHost: "www.bing.com", UTMMedium: "cpc"}, AcquisitionResolveOptions{})
	require.Equal(t, AcquisitionClassOther, result.Class)
	require.True(t, result.Paid)

	// 普通 utm（比如 utm_medium=social）不算付费，按 referrer 判
	result = ResolveAcquisition(AcquisitionTouch{ReferrerHost: "www.bing.com", UTMMedium: "social"}, AcquisitionResolveOptions{})
	require.Equal(t, AcquisitionClassSEO, result.Class)
}

func TestResolveAcquisitionAdminCreatedIsManual(t *testing.T) {
	touch := AcquisitionTouch{Source: "TG1", AffCode: "ABC", ReferrerHost: "www.baidu.com"}
	result := ResolveAcquisition(touch, AcquisitionResolveOptions{AdminCreated: true, ActorUserID: 1, SourceChannel: &PromotionChannel{Code: "TG1", Enabled: true}})
	require.Equal(t, AcquisitionClassOther, result.Class)
	require.Equal(t, AcquisitionChannelManual, result.ChannelCode)
}

func TestTouchLevelOrdering(t *testing.T) {
	require.Equal(t, AcquisitionLevelOfficial, TouchLevel(AcquisitionTouch{}))
	require.Equal(t, AcquisitionLevelOfficial, TouchLevel(AcquisitionTouch{ReferrerHost: "anytoken.work"}, "anytoken.work"))
	require.Equal(t, AcquisitionLevelExternal, TouchLevel(AcquisitionTouch{ReferrerHost: "t.me"}))
	require.Equal(t, AcquisitionLevelSearch, TouchLevel(AcquisitionTouch{ReferrerHost: "www.baidu.com"}))
	require.Equal(t, AcquisitionLevelPaid, TouchLevel(AcquisitionTouch{ReferrerHost: "www.baidu.com", PaidClick: true}))
	require.Equal(t, AcquisitionLevelAff, TouchLevel(AcquisitionTouch{AffCode: "A", PaidClick: true}))
	require.Equal(t, AcquisitionLevelSource, TouchLevel(AcquisitionTouch{Source: "TG1", AffCode: "A"}))
}

func TestTouchCookieRoundTrip(t *testing.T) {
	at := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	touch := AcquisitionTouch{Source: "tg1", AffCode: "ABC", LandingPath: "/pricing", ReferrerHost: "https://www.baidu.com/s?wd=x", UTMSource: "Baidu", UTMMedium: "CPC", UTMCampaign: "autumn", PaidClick: true, FirstTouchAt: at}
	encoded := EncodeAcquisitionTouchCookie(touch, 5)
	parsed, ok := ParseAcquisitionTouchCookie(encoded)
	require.True(t, ok)
	require.Equal(t, "TG1", parsed.Source, "渠道码统一大写")
	require.Equal(t, "ABC", parsed.AffCode)
	require.Equal(t, "/pricing", parsed.LandingPath)
	require.Equal(t, "baidu.com", parsed.ReferrerHost, "只留 host，去 www")
	require.Equal(t, "baidu", parsed.UTMSource)
	require.Equal(t, "cpc", parsed.UTMMedium)
	require.True(t, parsed.PaidClick)
	require.True(t, parsed.FirstTouchAt.Equal(at))
}

func TestTouchCookieRejectsGarbage(t *testing.T) {
	for _, raw := range []string{"", "not-base64!!", "eyJ2IjoyfQ", string(make([]byte, 5000))} {
		_, ok := ParseAcquisitionTouchCookie(raw)
		require.False(t, ok, raw)
	}
}

func TestTouchFromLanding(t *testing.T) {
	now := time.Now()
	touch := TouchFromLanding("https://anytoken.work/register?source=tg1&aff=ABC&utm_medium=CPC&gclid=xyz", "https://www.google.com/", now)
	require.Equal(t, "TG1", touch.Source)
	require.Equal(t, "ABC", touch.AffCode)
	require.Equal(t, "/register", touch.LandingPath)
	require.Equal(t, "google.com", touch.ReferrerHost)
	require.Equal(t, "cpc", touch.UTMMedium)
	require.True(t, touch.PaidClick)

	plain := TouchFromLanding("https://anytoken.work/", "", now)
	require.True(t, plain.IsZero() || plain.LandingPath == "/")
	require.Equal(t, AcquisitionLevelOfficial, TouchLevel(plain))
}

// --- 服务层：每个新用户都要有一行 ---

type fakePromotionRepo struct {
	PromotionRepository
	channels    map[string]*PromotionChannel
	attributed  []PromotionAttributionInput
	events      []PromotionAttributionEventInput
	alreadyUser map[int64]bool
}

func (f *fakePromotionRepo) GetChannelByCode(_ context.Context, code string) (*PromotionChannel, error) {
	if channel, ok := f.channels[code]; ok {
		return channel, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakePromotionRepo) AttributeUser(_ context.Context, in PromotionAttributionInput) (bool, error) {
	if f.alreadyUser[in.UserID] {
		return false, nil
	}
	f.attributed = append(f.attributed, in)
	return true, nil
}

func (f *fakePromotionRepo) RecordAttributionEvent(_ context.Context, in PromotionAttributionEventInput) error {
	f.events = append(f.events, in)
	return nil
}

func TestPromotionServiceAttributesEveryUserDefaultOfficial(t *testing.T) {
	repo := &fakePromotionRepo{channels: map[string]*PromotionChannel{}}
	svc := NewPromotionService(repo)

	require.NoError(t, svc.AttributeAcquisition(context.Background(), 42, AcquisitionTouch{}, 0))
	require.Len(t, repo.attributed, 1)
	require.Equal(t, AcquisitionChannelOfficial, repo.attributed[0].ChannelCode)
	require.Equal(t, AcquisitionClassOfficial, repo.attributed[0].Class)
	require.Len(t, repo.events, 1)
	require.Equal(t, "default_official", repo.events[0].Outcome)
}

func TestPromotionServiceInvalidSourceStillAttributes(t *testing.T) {
	repo := &fakePromotionRepo{channels: map[string]*PromotionChannel{}}
	svc := NewPromotionService(repo)

	require.NoError(t, svc.AttributeAcquisition(context.Background(), 7, AcquisitionTouch{Source: "nope", ReferrerHost: "www.baidu.com"}, 0))
	require.Len(t, repo.attributed, 1)
	require.Equal(t, AcquisitionClassSEO, repo.attributed[0].Class)
	require.Equal(t, "baidu", repo.attributed[0].Evidence["engine"])
	require.Len(t, repo.events, 2)
	require.Equal(t, "invalid_code", repo.events[0].Outcome)
	require.Equal(t, "NOPE", repo.events[0].RequestedCode)
	require.Equal(t, "resolved", repo.events[1].Outcome)
}

func TestPromotionServiceReadsTouchFromContextAndKeepsFirstTouch(t *testing.T) {
	repo := &fakePromotionRepo{channels: map[string]*PromotionChannel{"TG1": {ID: 3, Code: "TG1", ChannelType: AcquisitionClassOther, Enabled: true}}, alreadyUser: map[int64]bool{}}
	svc := NewPromotionService(repo)

	ctx := WithAcquisitionTouch(context.Background(), AcquisitionTouch{Source: "TG1", LandingPath: "/"})
	require.NoError(t, svc.AttributeAcquisition(ctx, 1, AcquisitionTouch{AffCode: "ABC"}, 0))
	require.Len(t, repo.attributed, 1)
	require.Equal(t, "TG1", repo.attributed[0].ChannelCode, "ctx 里的 source 优先于表单 aff")
	require.Equal(t, int64(3), *repo.events[0].ChannelID)

	repo.alreadyUser[1] = true
	require.NoError(t, svc.AttributeAcquisition(ctx, 1, AcquisitionTouch{}, 0))
	require.Len(t, repo.attributed, 1, "注册后不可变")
	require.Equal(t, "already_attributed", repo.events[len(repo.events)-1].Outcome)
}

func TestPromotionServiceManualCreation(t *testing.T) {
	repo := &fakePromotionRepo{channels: map[string]*PromotionChannel{}}
	svc := NewPromotionService(repo)
	ctx := WithAcquisitionTouch(context.Background(), AcquisitionTouch{Source: "TG1"})
	require.NoError(t, svc.AttributeManualCreation(ctx, 5, 1))
	require.Len(t, repo.attributed, 1)
	require.Equal(t, AcquisitionChannelManual, repo.attributed[0].ChannelCode)
	require.Equal(t, int64(1), repo.attributed[0].Evidence["actor_user_id"])
}
