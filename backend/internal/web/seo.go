//go:build embed

package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	htmlpkg "html"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const (
	robotsIndex   = "index,follow,max-image-preview:large"
	robotsNoIndex = "noindex, follow"
)

type seoLink struct {
	Label string
	Href  string
}

type seoPage struct {
	Key         string
	Title       string
	Description string
	Heading     string
	Intro       string
	Canonical   string
	Robots      string
	SchemaType  string
	Status      int
	Links       []seoLink
}

type seoSiteSettings struct {
	SiteName string `json:"site_name"`
	SiteLogo string `json:"site_logo"`
	DocURL   string `json:"doc_url"`
}

var noIndexExactPaths = map[string]struct{}{
	"/setup": {}, "/login": {}, "/register": {}, "/email-verify": {},
	"/forgot-password": {}, "/reset-password": {}, "/dashboard": {},
	"/keys": {}, "/batch-image": {}, "/docs/batch-image": {}, "/creation": {},
	"/usage": {}, "/redeem": {}, "/affiliate": {}, "/available-channels": {},
	"/profile": {}, "/subscriptions": {}, "/purchase": {}, "/orders": {},
	"/monitor": {}, "/admin": {},
}

func legacyFrontendRedirect(requestPath string) (string, bool) {
	switch requestPath {
	case "/index.html", "/home", "/home-v4", "/home-classic":
		return "/", true
	default:
		return "", false
	}
}

func resolveSEOPage(requestPath string, settingsJSON []byte, frontendURL string) seoPage {
	settings := parseSEOSiteSettings(settingsJSON)
	canonicalBase := canonicalBaseURL(frontendURL)
	docBase := safeHTTPURL(settings.DocURL)

	switch requestPath {
	case "/":
		return seoPage{
			Key:         "home",
			Title:       settings.SiteName + " - AI中转站｜多模型 API 聚合平台",
			Description: settings.SiteName + " 是面向开发者的 AI中转站，通过一个 API Key 统一接入当前可用的 Claude、GPT、Grok 等模型，并提供模型价格、用量查询和开发文档。",
			Heading:     settings.SiteName + " AI中转站",
			Intro:       "通过一个 API Key 统一接入当前可用的主流模型，集中查看模型价格、调用用量与开发文档。",
			Canonical:   canonicalURL(canonicalBase, "/"),
			Robots:      robotsIndex,
			SchemaType:  "home",
			Status:      http.StatusOK,
			Links: compactSEOLinks([]seoLink{
				{Label: "查看 AI 模型与 API 价格", Href: "/model-plaza"},
				{Label: "阅读 API 接入文档", Href: docBase},
				{Label: "查询 API Key 用量", Href: "/key-usage"},
			}),
		}
	case "/model-plaza":
		return seoPage{
			Key:         "model-plaza",
			Title:       "AI 模型 API 价格与模型列表｜" + settings.SiteName + " 模型广场",
			Description: "查看 " + settings.SiteName + " 当前可用模型及 API 价格，比较输入、输出 Token 计费、倍率、模型能力和可用分组；价格与可用性以页面实时数据为准。",
			Heading:     "AI 模型 API 价格与模型列表",
			Intro:       "按供应商、分组和倍率查看当前可用模型。动态模型列表和价格以页面加载后的实时数据为准。",
			Canonical:   canonicalURL(canonicalBase, "/model-plaza"),
			Robots:      robotsIndex,
			SchemaType:  "collection",
			Status:      http.StatusOK,
			Links: compactSEOLinks([]seoLink{
				{Label: "API 快速开始", Href: joinExternalURL(docBase, "/quickstart/")},
				{Label: "查询可用模型 API", Href: joinExternalURL(docBase, "/api/models/")},
				{Label: "查看 API Key 用量", Href: "/key-usage"},
			}),
		}
	case "/key-usage":
		return seoPage{
			Key:         "key-usage",
			Title:       settings.SiteName + " API Key 用量查询 - 额度、消费与请求记录",
			Description: "在浏览器本地使用 API Key 查询 " + settings.SiteName + " 额度、消费和请求记录；Key 不会被页面存储。",
			Heading:     "API Key 用量查询",
			Intro:       "查询当前 API Key 的额度、消费、Token 与请求记录。Key 仅用于本次浏览器请求，不会被页面存储。",
			Canonical:   canonicalURL(canonicalBase, "/key-usage"),
			Robots:      robotsIndex,
			SchemaType:  "tool",
			Status:      http.StatusOK,
			Links: compactSEOLinks([]seoLink{
				{Label: "API Key 安全指南", Href: joinExternalURL(docBase, "/security/api-keys/")},
				{Label: "用量与计费说明", Href: joinExternalURL(docBase, "/account/billing/")},
				{Label: "API 错误排查", Href: joinExternalURL(docBase, "/troubleshooting/errors/")},
			}),
		}
	}

	if isKnownNoIndexPath(requestPath) {
		return seoPage{Key: "private", Robots: robotsNoIndex, Status: http.StatusOK}
	}

	return seoPage{
		Key:         "not-found",
		Title:       "页面未找到｜" + settings.SiteName,
		Description: "你访问的页面不存在或已经调整。",
		Heading:     "页面未找到",
		Intro:       "该地址不存在或已经调整，请返回首页、模型广场或开发文档继续浏览。",
		Robots:      robotsNoIndex,
		Status:      http.StatusNotFound,
		Links: compactSEOLinks([]seoLink{
			{Label: "返回首页", Href: "/"},
			{Label: "查看模型广场", Href: "/model-plaza"},
			{Label: "阅读开发文档", Href: docBase},
		}),
	}
}

func parseSEOSiteSettings(settingsJSON []byte) seoSiteSettings {
	settings := seoSiteSettings{SiteName: "Sub2API"}
	if len(settingsJSON) > 0 {
		_ = json.Unmarshal(settingsJSON, &settings)
	}
	settings.SiteName = strings.TrimSpace(settings.SiteName)
	if settings.SiteName == "" {
		settings.SiteName = "Sub2API"
	}
	return settings
}

func isKnownNoIndexPath(requestPath string) bool {
	if _, ok := noIndexExactPaths[requestPath]; ok {
		return true
	}
	for _, prefix := range []string{"/admin/", "/auth/", "/payment/", "/legal/", "/custom/"} {
		if strings.HasPrefix(requestPath, prefix) {
			return true
		}
	}
	return false
}

func canonicalBaseURL(value string) string {
	trimmed := strings.TrimSpace(value)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawPath = strings.TrimSuffix(parsed.RawPath, "/")
	return strings.TrimSuffix(parsed.String(), "/")
}

func canonicalURL(base, requestPath string) string {
	if base == "" {
		return ""
	}
	if requestPath == "/" {
		return base + "/"
	}
	return base + "/" + strings.TrimPrefix(requestPath, "/")
}

func safeHTTPURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return parsed.String()
}

func joinExternalURL(base, route string) string {
	if base == "" {
		return ""
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return ""
	}
	parsed.Path = path.Join(parsed.Path, route)
	if strings.HasSuffix(route, "/") {
		parsed.Path += "/"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func compactSEOLinks(links []seoLink) []seoLink {
	result := make([]seoLink, 0, len(links))
	for _, link := range links {
		if strings.TrimSpace(link.Label) != "" && strings.TrimSpace(link.Href) != "" {
			result = append(result, link)
		}
	}
	return result
}

func injectRouteSEO(baseHTML []byte, page seoPage, settingsJSON []byte, frontendURL string) []byte {
	settings := parseSEOSiteSettings(settingsJSON)
	result := baseHTML
	if page.Title != "" {
		result = replaceHTMLTitle(result, page.Title)
	}

	var head strings.Builder
	head.WriteString("\n    <!-- route-seo:start -->\n")
	if page.Description != "" {
		head.WriteString(`    <meta name="description" content="` + htmlpkg.EscapeString(page.Description) + `" />` + "\n")
	}
	if page.Robots != "" {
		head.WriteString(`    <meta name="robots" content="` + htmlpkg.EscapeString(page.Robots) + `" />` + "\n")
	}
	if page.Canonical != "" {
		head.WriteString(`    <link rel="canonical" href="` + htmlpkg.EscapeString(page.Canonical) + `" />` + "\n")
	}
	if page.Title != "" && page.Description != "" {
		head.WriteString("    <meta property=\"og:type\" content=\"" + map[bool]string{true: "website", false: "article"}[page.Key == "home"] + "\" />\n")
		head.WriteString(`    <meta property="og:site_name" content="` + htmlpkg.EscapeString(settings.SiteName) + `" />` + "\n")
		head.WriteString(`    <meta property="og:title" content="` + htmlpkg.EscapeString(page.Title) + `" />` + "\n")
		head.WriteString(`    <meta property="og:description" content="` + htmlpkg.EscapeString(page.Description) + `" />` + "\n")
		if page.Canonical != "" {
			head.WriteString(`    <meta property="og:url" content="` + htmlpkg.EscapeString(page.Canonical) + `" />` + "\n")
			imageURL := canonicalURL(canonicalBaseURL(frontendURL), "/anytoken-logo.png")
			if imageURL != "" {
				head.WriteString(`    <meta property="og:image" content="` + htmlpkg.EscapeString(imageURL) + `" />` + "\n")
			}
		}
		head.WriteString("    <meta name=\"twitter:card\" content=\"summary\" />\n")
	}
	if structuredData := seoStructuredData(page, settings, frontendURL); structuredData != "" {
		head.WriteString(`    <script id="route-structured-data" type="application/ld+json" nonce="` + NonceHTMLPlaceholder + `">` + structuredData + `</script>` + "\n")
	}
	head.WriteString("    <!-- route-seo:end -->\n")
	result = bytes.Replace(result, []byte("</head>"), []byte(head.String()+"  </head>"), 1)

	if page.Heading != "" {
		fallback := renderSEOFallback(page)
		result = bytes.Replace(result, []byte(`<div id="app"></div>`), []byte(`<div id="app">`+fallback+`</div>`), 1)
	}
	return result
}

func replaceHTMLTitle(html []byte, title string) []byte {
	start := bytes.Index(html, []byte("<title>"))
	if start == -1 {
		return html
	}
	endOffset := bytes.Index(html[start:], []byte("</title>"))
	if endOffset == -1 {
		return html
	}
	end := start + endOffset + len("</title>")
	replacement := []byte("<title>" + htmlpkg.EscapeString(title) + "</title>")
	return append(append(append([]byte(nil), html[:start]...), replacement...), html[end:]...)
}

func renderSEOFallback(page seoPage) string {
	var html strings.Builder
	html.WriteString(`<main class="seo-fallback" data-seo-page="` + htmlpkg.EscapeString(page.Key) + `">`)
	html.WriteString(`<div class="seo-fallback__content"><h1>` + htmlpkg.EscapeString(page.Heading) + `</h1>`)
	html.WriteString(`<p>` + htmlpkg.EscapeString(page.Intro) + `</p>`)
	if len(page.Links) > 0 {
		html.WriteString(`<nav aria-label="相关页面">`)
		for _, link := range page.Links {
			html.WriteString(`<a href="` + htmlpkg.EscapeString(link.Href) + `">` + htmlpkg.EscapeString(link.Label) + `</a>`)
		}
		html.WriteString(`</nav>`)
	}
	html.WriteString(`</div></main>`)
	return html.String()
}

func seoStructuredData(page seoPage, settings seoSiteSettings, frontendURL string) string {
	if page.Canonical == "" || page.SchemaType == "" {
		return ""
	}
	var payload any
	switch page.SchemaType {
	case "home":
		organization := map[string]any{
			"@type": "Organization", "@id": page.Canonical + "#organization", "name": settings.SiteName, "url": page.Canonical,
		}
		if imageURL := canonicalURL(canonicalBaseURL(frontendURL), "/anytoken-logo.png"); imageURL != "" {
			organization["logo"] = imageURL
		}
		payload = map[string]any{
			"@context": "https://schema.org",
			"@graph": []any{
				organization,
				map[string]any{"@type": "WebSite", "@id": page.Canonical + "#website", "name": settings.SiteName, "url": page.Canonical, "publisher": map[string]string{"@id": page.Canonical + "#organization"}},
			},
		}
	case "collection":
		payload = map[string]any{"@context": "https://schema.org", "@type": "CollectionPage", "name": page.Heading, "description": page.Description, "url": page.Canonical}
	case "tool":
		payload = map[string]any{"@context": "https://schema.org", "@type": "WebApplication", "name": page.Heading, "description": page.Description, "url": page.Canonical, "applicationCategory": "DeveloperApplication", "operatingSystem": "Web"}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func routeETag(baseETag, requestPath string, status int) string {
	sum := sha256.Sum256([]byte(baseETag + "\x00" + requestPath + "\x00" + http.StatusText(status)))
	return `"` + hex.EncodeToString(sum[:12]) + `"`
}
