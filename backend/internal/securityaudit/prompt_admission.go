package securityaudit

import (
	"context"
	"strconv"
	"sync"
	"time"
)

const (
	defaultPromptAdmissionLimit      = 2
	defaultPromptAdmissionMaxWaiting = 4
	defaultPromptAdmissionWait       = 1500 * time.Millisecond
)

type promptAdmissionEntry struct {
	active  int
	waiting int
	notify  chan struct{}
}

type promptAdmissionLimiter struct {
	mu         sync.Mutex
	entries    map[string]*promptAdmissionEntry
	limit      int
	maxWaiting int
	wait       time.Duration
}

func newPromptAdmissionLimiter(limit, maxWaiting int, wait time.Duration) *promptAdmissionLimiter {
	if limit < 1 {
		limit = defaultPromptAdmissionLimit
	}
	if maxWaiting < 0 {
		maxWaiting = 0
	}
	if wait <= 0 {
		wait = defaultPromptAdmissionWait
	}
	return &promptAdmissionLimiter{
		entries: make(map[string]*promptAdmissionEntry),
		limit:   limit, maxWaiting: maxWaiting, wait: wait,
	}
}

func promptAdmissionKey(req Request) string {
	if req.UserID > 0 {
		return "user:" + strconv.FormatInt(req.UserID, 10)
	}
	if req.APIKeyID > 0 {
		return "api_key:" + strconv.FormatInt(req.APIKeyID, 10)
	}
	// Requests without an authenticated identity share a conservative bucket so
	// missing metadata cannot bypass admission control.
	return "anonymous"
}

func (l *promptAdmissionLimiter) Acquire(ctx context.Context, key string) (func(), bool) {
	if l == nil {
		return func() {}, true
	}
	if ctx == nil {
		ctx = context.Background()
	}
	waitCtx, cancel := context.WithTimeout(ctx, l.wait)
	defer cancel()
	waiting := false
	for {
		l.mu.Lock()
		entry := l.entries[key]
		if entry == nil {
			entry = &promptAdmissionEntry{notify: make(chan struct{})}
			l.entries[key] = entry
		}
		if entry.active < l.limit {
			if waiting && entry.waiting > 0 {
				entry.waiting--
			}
			entry.active++
			l.mu.Unlock()
			return l.releaseFunc(key, entry), true
		}
		if !waiting {
			if entry.waiting >= l.maxWaiting {
				l.cleanupLocked(key, entry)
				l.mu.Unlock()
				return nil, false
			}
			entry.waiting++
			waiting = true
		}
		notify := entry.notify
		l.mu.Unlock()

		select {
		case <-notify:
		case <-waitCtx.Done():
			l.mu.Lock()
			if waiting && entry.waiting > 0 {
				entry.waiting--
			}
			l.cleanupLocked(key, entry)
			l.mu.Unlock()
			return nil, false
		}
	}
}

func (l *promptAdmissionLimiter) releaseFunc(key string, entry *promptAdmissionEntry) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			if entry.active > 0 {
				entry.active--
			}
			close(entry.notify)
			entry.notify = make(chan struct{})
			l.cleanupLocked(key, entry)
			l.mu.Unlock()
		})
	}
}

func (l *promptAdmissionLimiter) cleanupLocked(key string, entry *promptAdmissionEntry) {
	if entry.active == 0 && entry.waiting == 0 && l.entries[key] == entry {
		delete(l.entries, key)
	}
}
