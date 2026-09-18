package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

// 获客分类：回答“人从哪来”。邀请返利（inviter_id）是另一个维度，两者互不覆盖。
const (
	AcquisitionClassOfficial = "official"
	AcquisitionClassSEO      = "seo"
	AcquisitionClassInvite   = "invite"
	AcquisitionClassOther    = "other"
)

// 系统渠道编码，由迁移 244 插入，不可删改。
const (
	AcquisitionChannelOfficial = "OFFICIAL"
	AcquisitionChannelSEO      = "SEO"
	AcquisitionChannelInvite   = "INVITE"
	AcquisitionChannelManual   = "MANUAL"
	AcquisitionChannelExternal = "EXTERNAL"
)

// 证据等级：只允许升级不允许降级。官网逛过再点推广链接算推广；推广进来后再逛不会被改回官网。
const (
	AcquisitionLevelOfficial = 0
	AcquisitionLevelExternal = 1
	AcquisitionLevelSearch   = 2
	AcquisitionLevelPaid     = 3
	AcquisitionLevelAff      = 4
	AcquisitionLevelSource   = 5
)

// AcquisitionTouchCookieName 是前端落地时写入、服务端注册时读取的一方 Cookie。
const AcquisitionTouchCookieName = "s2a_touch"

// AcquisitionTouchMaxAge Cookie 有效期。
const AcquisitionTouchMaxAge = 30 * 24 * time.Hour

const acquisitionFieldLimit = 512

// AcquisitionTouch 是一次落地采集到的原始证据，前端只送证据，分类由服务端裁定。
type AcquisitionTouch struct {
	Source       string    `json:"source,omitempty"`
	AffCode      string    `json:"aff_code,omitempty"`
	LandingPath  string    `json:"landing_path,omitempty"`
	ReferrerHost string    `json:"referrer_host,omitempty"`
	UTMSource    string    `json:"utm_source,omitempty"`
	UTMMedium    string    `json:"utm_medium,omitempty"`
	UTMCampaign  string    `json:"utm_campaign,omitempty"`
	PaidClick    bool      `json:"paid_click,omitempty"`
	FirstTouchAt time.Time `json:"first_touch_at,omitempty"`
}

// IsZero 表示没有任何证据。
func (t AcquisitionTouch) IsZero() bool {
	return t.Source == "" && t.AffCode == "" && t.LandingPath == "" && t.ReferrerHost == "" &&
		t.UTMSource == "" && t.UTMMedium == "" && t.UTMCampaign == "" && !t.PaidClick
}

// Merge 用 other 补齐 t 的空字段（不覆盖已有值）。
func (t AcquisitionTouch) Merge(other AcquisitionTouch) AcquisitionTouch {
	if t.Source == "" {
		t.Source = other.Source
	}
	if t.AffCode == "" {
		t.AffCode = other.AffCode
	}
	if t.LandingPath == "" {
		t.LandingPath = other.LandingPath
	}
	if t.ReferrerHost == "" {
		t.ReferrerHost = other.ReferrerHost
	}
	if t.UTMSource == "" {
		t.UTMSource = other.UTMSource
	}
	if t.UTMMedium == "" {
		t.UTMMedium = other.UTMMedium
	}
	if t.UTMCampaign == "" {
		t.UTMCampaign = other.UTMCampaign
	}
	t.PaidClick = t.PaidClick || other.PaidClick
	if t.FirstTouchAt.IsZero() {
		t.FirstTouchAt = other.FirstTouchAt
	}
	return t
}

// AcquisitionResult 是服务端裁定后的归因结论。
type AcquisitionResult struct {
	Class        string
	ChannelCode  string
	Reason       string
	Engine       string
	ReferrerHost string
	Paid         bool
	Level        int
	Touch        AcquisitionTouch
}

// AcquisitionResolveOptions 裁定时的上下文。
type AcquisitionResolveOptions struct {
	// AdminCreated 表示管理员 / API 创建，直接归 MANUAL；ActorUserID 只用于留证。
	AdminCreated bool
	ActorUserID  int64
	// SiteHosts 本站域名（小写、去 www），referrer 命中算站内。
	SiteHosts []string
	// SourceChannel 是 touch.Source 查到的渠道；nil 表示编码不存在或已停用。
	SourceChannel *PromotionChannel
}

type acquisitionTouchContextKey struct{}

// WithAcquisitionTouch 把落地证据放进 ctx，注册成功后由 AuthService 读取。
func WithAcquisitionTouch(ctx context.Context, touch AcquisitionTouch) context.Context {
	return context.WithValue(ctx, acquisitionTouchContextKey{}, touch)
}

// AcquisitionTouchFromContext 读取 ctx 中的落地证据；没有时返回零值。
func AcquisitionTouchFromContext(ctx context.Context) AcquisitionTouch {
	if ctx == nil {
		return AcquisitionTouch{}
	}
	if value, ok := ctx.Value(acquisitionTouchContextKey{}).(AcquisitionTouch); ok {
		return value
	}
	return AcquisitionTouch{}
}

type acquisitionSiteHostContextKey struct{}

// WithAcquisitionSiteHost 记录本次请求的站点 host，用于判断 referrer 是否站内。
func WithAcquisitionSiteHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, acquisitionSiteHostContextKey{}, normalizeAcquisitionHost(host))
}

func acquisitionSiteHostFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(acquisitionSiteHostContextKey{}).(string); ok {
		return value
	}
	return ""
}

// WithPromotionSource 兼容旧调用：只带渠道编码。
func WithPromotionSource(ctx context.Context, source string) context.Context {
	touch := AcquisitionTouchFromContext(ctx)
	touch.Source = normalizePromotionCode(source)
	return WithAcquisitionTouch(ctx, touch)
}

// acquisitionTouchWire 是 Cookie 里的压缩字段名，前端 acquisitionTouch.ts 必须保持一致。
type acquisitionTouchWire struct {
	V int    `json:"v"`
	T int64  `json:"t"`
	P string `json:"p"`
	R string `json:"r"`
	S string `json:"s"`
	A string `json:"a"`
	U struct {
		S string `json:"s"`
		M string `json:"m"`
		C string `json:"c"`
	} `json:"u"`
	C bool `json:"c"`
	K int  `json:"k"`
}

// ParseAcquisitionTouchCookie 解析 s2a_touch。损坏、超长、版本不识别一律当作没有证据。
func ParseAcquisitionTouchCookie(raw string) (AcquisitionTouch, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 4096 {
		return AcquisitionTouch{}, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		if decoded, err = base64.StdEncoding.DecodeString(raw); err != nil {
			unescaped, unescapeErr := url.QueryUnescape(raw)
			if unescapeErr != nil {
				return AcquisitionTouch{}, false
			}
			decoded = []byte(unescaped)
		}
	}
	var wire acquisitionTouchWire
	if err := json.Unmarshal(decoded, &wire); err != nil {
		return AcquisitionTouch{}, false
	}
	if wire.V != 1 {
		return AcquisitionTouch{}, false
	}
	touch := AcquisitionTouch{
		Source:       normalizePromotionCode(clipAcquisitionField(wire.S, 64)),
		AffCode:      strings.TrimSpace(clipAcquisitionField(wire.A, 64)),
		LandingPath:  clipAcquisitionField(wire.P, acquisitionFieldLimit),
		ReferrerHost: normalizeAcquisitionHost(clipAcquisitionField(wire.R, 255)),
		UTMSource:    strings.ToLower(clipAcquisitionField(wire.U.S, 128)),
		UTMMedium:    strings.ToLower(clipAcquisitionField(wire.U.M, 128)),
		UTMCampaign:  clipAcquisitionField(wire.U.C, 128),
		PaidClick:    wire.C,
	}
	if wire.T > 0 {
		touch.FirstTouchAt = time.UnixMilli(wire.T).UTC()
	}
	return touch, true
}

// EncodeAcquisitionTouchCookie 生成服务端 Set-Cookie 用的值，字段与前端一致。
func EncodeAcquisitionTouchCookie(touch AcquisitionTouch, level int) string {
	var wire acquisitionTouchWire
	wire.V = 1
	if !touch.FirstTouchAt.IsZero() {
		wire.T = touch.FirstTouchAt.UnixMilli()
	} else {
		wire.T = time.Now().UnixMilli()
	}
	wire.P = touch.LandingPath
	wire.R = touch.ReferrerHost
	wire.S = touch.Source
	wire.A = touch.AffCode
	wire.U.S = touch.UTMSource
	wire.U.M = touch.UTMMedium
	wire.U.C = touch.UTMCampaign
	wire.C = touch.PaidClick
	wire.K = level
	body, _ := json.Marshal(wire)
	return base64.RawURLEncoding.EncodeToString(body)
}

// TouchFromLanding 从落地 URL 和 referrer 提取证据（信标端点使用）。
func TouchFromLanding(landingURL, referrer string, now time.Time) AcquisitionTouch {
	touch := AcquisitionTouch{FirstTouchAt: now.UTC()}
	if parsed, err := url.Parse(strings.TrimSpace(landingURL)); err == nil {
		touch.LandingPath = clipAcquisitionField(parsed.Path, acquisitionFieldLimit)
		query := parsed.Query()
		touch.Source = normalizePromotionCode(clipAcquisitionField(query.Get("source"), 64))
		touch.AffCode = strings.TrimSpace(clipAcquisitionField(firstNonEmpty(query.Get("aff"), query.Get("aff_code")), 64))
		touch.UTMSource = strings.ToLower(clipAcquisitionField(query.Get("utm_source"), 128))
		touch.UTMMedium = strings.ToLower(clipAcquisitionField(query.Get("utm_medium"), 128))
		touch.UTMCampaign = clipAcquisitionField(query.Get("utm_campaign"), 128)
		for _, key := range paidClickParams {
			if query.Get(key) != "" {
				touch.PaidClick = true
				break
			}
		}
	}
	if referrer = strings.TrimSpace(referrer); referrer != "" {
		if parsed, err := url.Parse(referrer); err == nil {
			touch.ReferrerHost = normalizeAcquisitionHost(parsed.Host)
		}
	}
	return touch
}

var paidClickParams = []string{"gclid", "gbraid", "wbraid", "fbclid", "msclkid", "ttclid", "dclid", "yclid"}

var paidUTMMediums = map[string]bool{"cpc": true, "ppc": true, "paid": true, "paidsocial": true, "paid_social": true, "cpm": true, "display": true}

// 搜索引擎与 AI 搜索：按 host 后缀匹配。做 API 中转，从 ChatGPT / Kimi 搜过来的越来越多，也算 SEO，但记下引擎名。
var acquisitionSearchEngines = []struct {
	suffix string
	name   string
}{
	{"google.", "google"},
	{"bing.com", "bing"},
	{"baidu.com", "baidu"},
	{"sogou.com", "sogou"},
	{"so.com", "360"},
	{"sm.cn", "shenma"},
	{"yandex.", "yandex"},
	{"duckduckgo.com", "duckduckgo"},
	{"naver.com", "naver"},
	{"yahoo.", "yahoo"},
	{"ecosia.org", "ecosia"},
	{"brave.com", "brave"},
	{"chatgpt.com", "chatgpt"},
	{"openai.com", "chatgpt"},
	{"perplexity.ai", "perplexity"},
	{"claude.ai", "claude"},
	{"gemini.google.com", "gemini"},
	{"kimi.moonshot.cn", "kimi"},
	{"kimi.com", "kimi"},
	{"doubao.com", "doubao"},
	{"metaso.cn", "metaso"},
	{"yiyan.baidu.com", "yiyan"},
	{"tongyi.aliyun.com", "tongyi"},
	{"deepseek.com", "deepseek"},
	{"you.com", "you"},
}

// DetectSearchEngine 判断 host 是否搜索引擎或 AI 搜索，返回引擎名。
func DetectSearchEngine(host string) (string, bool) {
	host = normalizeAcquisitionHost(host)
	if host == "" {
		return "", false
	}
	for _, engine := range acquisitionSearchEngines {
		if strings.HasSuffix(engine.suffix, ".") {
			// "google." 匹配 google.com / google.co.jp / www.google.com.hk
			if strings.HasPrefix(host, engine.suffix) || strings.Contains(host, "."+engine.suffix) {
				return engine.name, true
			}
			continue
		}
		if host == engine.suffix || strings.HasSuffix(host, "."+engine.suffix) {
			return engine.name, true
		}
	}
	return "", false
}

// IsPaidTouch 带付费点击参数或付费 UTM medium。
func IsPaidTouch(touch AcquisitionTouch) bool {
	return touch.PaidClick || paidUTMMediums[strings.ToLower(strings.TrimSpace(touch.UTMMedium))]
}

// TouchLevel 计算一次触达的证据等级（不做渠道校验）。
func TouchLevel(touch AcquisitionTouch, siteHosts ...string) int {
	switch {
	case touch.Source != "":
		return AcquisitionLevelSource
	case touch.AffCode != "":
		return AcquisitionLevelAff
	case IsPaidTouch(touch):
		return AcquisitionLevelPaid
	}
	if _, ok := DetectSearchEngine(touch.ReferrerHost); ok {
		return AcquisitionLevelSearch
	}
	if touch.ReferrerHost != "" && !isAcquisitionSiteHost(touch.ReferrerHost, siteHosts) {
		return AcquisitionLevelExternal
	}
	return AcquisitionLevelOfficial
}

// ResolveAcquisition 是唯一的分类裁定入口。
// 顺序：管理员创建 > 有效 source > aff > 付费 > 搜索 > 外站 > 官网。
// 有效 source 需要调用方先查渠道并放进 opts.SourceChannel；这里不碰数据库。
func ResolveAcquisition(touch AcquisitionTouch, opts AcquisitionResolveOptions) AcquisitionResult {
	result := AcquisitionResult{Touch: touch, ReferrerHost: touch.ReferrerHost}
	if opts.AdminCreated || opts.ActorUserID > 0 {
		result.Class = AcquisitionClassOther
		result.ChannelCode = AcquisitionChannelManual
		result.Reason = "admin_created"
		result.Level = AcquisitionLevelSource
		return result
	}
	if touch.Source != "" && opts.SourceChannel != nil && opts.SourceChannel.Enabled {
		result.Class = normalizeAcquisitionClass(opts.SourceChannel.ChannelType)
		result.ChannelCode = opts.SourceChannel.Code
		result.Reason = "source_code"
		result.Level = AcquisitionLevelSource
		result.Paid = IsPaidTouch(touch)
		return result
	}
	if touch.AffCode != "" {
		result.Class = AcquisitionClassInvite
		result.ChannelCode = AcquisitionChannelInvite
		result.Reason = "aff_link"
		result.Level = AcquisitionLevelAff
		return result
	}
	if IsPaidTouch(touch) {
		result.Class = AcquisitionClassOther
		result.ChannelCode = AcquisitionChannelExternal
		result.Reason = "paid_click"
		result.Paid = true
		result.Level = AcquisitionLevelPaid
		return result
	}
	if engine, ok := DetectSearchEngine(touch.ReferrerHost); ok {
		result.Class = AcquisitionClassSEO
		result.ChannelCode = AcquisitionChannelSEO
		result.Reason = "search_referrer"
		result.Engine = engine
		result.Level = AcquisitionLevelSearch
		return result
	}
	if touch.ReferrerHost != "" && !isAcquisitionSiteHost(touch.ReferrerHost, opts.SiteHosts) {
		result.Class = AcquisitionClassOther
		result.ChannelCode = AcquisitionChannelExternal
		result.Reason = "external_referrer"
		result.Level = AcquisitionLevelExternal
		return result
	}
	result.Class = AcquisitionClassOfficial
	result.ChannelCode = AcquisitionChannelOfficial
	result.Level = AcquisitionLevelOfficial
	if touch.Source != "" {
		result.Reason = "invalid_source_default_official"
	} else {
		result.Reason = "default_official"
	}
	return result
}

func normalizeAcquisitionClass(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AcquisitionClassOfficial:
		return AcquisitionClassOfficial
	case AcquisitionClassSEO:
		return AcquisitionClassSEO
	case AcquisitionClassInvite:
		return AcquisitionClassInvite
	default:
		return AcquisitionClassOther
	}
}

func isValidAcquisitionClass(value string) bool {
	switch value {
	case AcquisitionClassOfficial, AcquisitionClassSEO, AcquisitionClassInvite, AcquisitionClassOther:
		return true
	}
	return false
}

func isAcquisitionSiteHost(host string, siteHosts []string) bool {
	host = normalizeAcquisitionHost(host)
	if host == "" {
		return true
	}
	for _, site := range siteHosts {
		if site = normalizeAcquisitionHost(site); site != "" && site == host {
			return true
		}
	}
	return false
}

func normalizeAcquisitionHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if strings.Contains(host, "://") {
		if parsed, err := url.Parse(host); err == nil {
			host = parsed.Host
		}
	}
	if idx := strings.IndexByte(host, '/'); idx >= 0 {
		host = host[:idx]
	}
	if idx := strings.IndexByte(host, ':'); idx >= 0 {
		host = host[:idx]
	}
	host = strings.TrimPrefix(host, "www.")
	return host
}

func clipAcquisitionField(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
