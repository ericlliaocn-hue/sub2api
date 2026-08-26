//go:build embed

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSEOTestRouter(t *testing.T, provider *mockSettingsProvider) *gin.Engine {
	t.Helper()
	server, err := NewFrontendServer(provider)
	require.NoError(t, err)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.CSPNonceKey, "seo-test-nonce")
		c.Next()
	})
	router.Use(server.Middleware())
	return router
}

func TestFrontendSEO_PublicPages(t *testing.T) {
	provider := &mockSettingsProvider{
		settings: map[string]any{
			"site_name": "AnyToken",
			"doc_url":   "https://doc.anytoken.work",
		},
		frontendURL: "https://anytoken.work",
	}
	router := newSEOTestRouter(t, provider)

	tests := []struct {
		path      string
		titleText string
		canonical string
		schema    string
	}{
		{path: "/", titleText: "多模型 AI API 聚合平台", canonical: "https://anytoken.work/", schema: "WebSite"},
		{path: "/model-plaza", titleText: "AI 模型 API 价格与模型列表", canonical: "https://anytoken.work/model-plaza", schema: "CollectionPage"},
		{path: "/key-usage", titleText: "API Key 用量查询", canonical: "https://anytoken.work/key-usage", schema: "WebApplication"},
	}

	etags := make(map[string]struct{}, len(tests))
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), tt.titleText)
			assert.Contains(t, w.Body.String(), `<link rel="canonical" href="`+tt.canonical+`"`)
			assert.Contains(t, w.Body.String(), tt.schema)
			assert.Contains(t, w.Body.String(), `class="seo-fallback"`)
			assert.Contains(t, w.Body.String(), `<h1>`+tt.titleText+`</h1>`)
			assert.Contains(t, w.Body.String(), `nonce="seo-test-nonce"`)
			assert.Empty(t, w.Header().Get("X-Robots-Tag"))
			etag := w.Header().Get("ETag")
			require.NotEmpty(t, etag)
			etags[etag] = struct{}{}
		})
	}
	assert.Len(t, etags, 3, "route-level HTML must not share an ETag")
}

func TestFrontendSEO_StaticDiscoveryFiles(t *testing.T) {
	provider := &mockSettingsProvider{settings: map[string]string{"site_name": "AnyToken"}}
	router := newSEOTestRouter(t, provider)

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/robots.txt", contentType: "text/plain", contains: "Sitemap: https://anytoken.work/sitemap.xml"},
		{path: "/sitemap.xml", contentType: "xml", contains: "https://anytoken.work/model-plaza"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), tt.contentType)
			assert.Contains(t, w.Body.String(), tt.contains)
			assert.NotContains(t, w.Body.String(), "<!doctype html>")
		})
	}
}

func TestFrontendSEO_RedirectsNoIndexAndNotFound(t *testing.T) {
	provider := &mockSettingsProvider{
		settings:    map[string]string{"site_name": "AnyToken"},
		frontendURL: "https://anytoken.work",
	}
	router := newSEOTestRouter(t, provider)

	for _, oldPath := range []string{"/index.html", "/home", "/home-v4", "/home-classic"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, oldPath, nil))
		assert.Equal(t, http.StatusMovedPermanently, w.Code, oldPath)
		assert.Equal(t, "/", w.Header().Get("Location"), oldPath)
	}

	for _, privatePath := range []string{"/login", "/dashboard", "/admin/users", "/auth/callback", "/payment/result", "/legal/terms", "/custom/demo"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, privatePath, nil))
		assert.Equal(t, http.StatusOK, w.Code, privatePath)
		assert.Equal(t, robotsNoIndex, w.Header().Get("X-Robots-Tag"), privatePath)
		assert.Contains(t, w.Body.String(), `content="noindex, follow"`, privatePath)
		assert.NotContains(t, w.Body.String(), `rel="canonical"`, privatePath)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/not-a-real-page", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, robotsNoIndex, w.Header().Get("X-Robots-Tag"))
	assert.Contains(t, w.Body.String(), "页面未找到")
	assert.Contains(t, w.Body.String(), `href="/model-plaza"`)
}

func TestFrontendSEO_DoesNotTrustRequestHostForCanonical(t *testing.T) {
	provider := &mockSettingsProvider{settings: map[string]string{"site_name": "AnyToken"}}
	router := newSEOTestRouter(t, provider)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/model-plaza", nil)
	req.Host = "attacker.example"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), `rel="canonical"`)
	assert.False(t, strings.Contains(w.Body.String(), "attacker.example"))
}

func TestCanonicalBaseURL(t *testing.T) {
	assert.Equal(t, "https://anytoken.work", canonicalBaseURL("https://anytoken.work/"))
	assert.Equal(t, "https://example.com/app", canonicalBaseURL("https://example.com/app/"))
	assert.Equal(t, "https://example.com/%E6%96%87%E6%A1%A3", canonicalBaseURL("https://example.com/%E6%96%87%E6%A1%A3/"))
	assert.Empty(t, canonicalBaseURL("https://user:pass@example.com"))
	assert.Empty(t, canonicalBaseURL("//attacker.example"))
	assert.Empty(t, canonicalBaseURL("javascript:alert(1)"))
}
