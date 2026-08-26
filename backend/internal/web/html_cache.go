//go:build embed

package web

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// HTMLCache manages the cached index.html with injected settings
type HTMLCache struct {
	mu              sync.RWMutex
	cachedHTML      []byte
	settingsJSON    []byte
	frontendURL     string
	etag            string
	baseHTMLHash    string // Hash of the original index.html (immutable after build)
	settingsVersion uint64 // Incremented when settings change
}

// CachedHTML represents the cache state
type CachedHTML struct {
	Content      []byte
	SettingsJSON []byte
	FrontendURL  string
	ETag         string
}

// NewHTMLCache creates a new HTML cache instance
func NewHTMLCache() *HTMLCache {
	return &HTMLCache{}
}

// SetBaseHTML initializes the cache with the base HTML template
func (c *HTMLCache) SetBaseHTML(baseHTML []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	hash := sha256.Sum256(baseHTML)
	c.baseHTMLHash = hex.EncodeToString(hash[:8]) // First 8 bytes for brevity
}

// Invalidate marks the cache as stale
func (c *HTMLCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.settingsVersion++
	c.cachedHTML = nil
	c.settingsJSON = nil
	c.frontendURL = ""
	c.etag = ""
}

// Get returns the cached HTML or nil if cache is stale
func (c *HTMLCache) Get() *CachedHTML {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cachedHTML == nil {
		return nil
	}
	return &CachedHTML{
		Content:      append([]byte(nil), c.cachedHTML...),
		SettingsJSON: append([]byte(nil), c.settingsJSON...),
		FrontendURL:  c.frontendURL,
		ETag:         c.etag,
	}
}

// Set updates the cache with new rendered HTML
func (c *HTMLCache) Set(html []byte, settingsJSON []byte, frontendURL ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cachedHTML = append([]byte(nil), html...)
	c.settingsJSON = append([]byte(nil), settingsJSON...)
	if len(frontendURL) > 0 {
		c.frontendURL = frontendURL[0]
	} else {
		c.frontendURL = ""
	}
	c.etag = c.generateETag(settingsJSON, c.frontendURL)
}

// generateETag creates an ETag from base HTML hash + settings hash
func (c *HTMLCache) generateETag(settingsJSON []byte, frontendURL string) string {
	settingsHash := sha256.Sum256(append(append([]byte(nil), settingsJSON...), frontendURL...))
	return `"` + c.baseHTMLHash + "-" + hex.EncodeToString(settingsHash[:8]) + `"`
}
