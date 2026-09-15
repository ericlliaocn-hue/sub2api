package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	SettingKeyWatchedTrafficConfig = "watched_traffic_config"

	watchedTrafficQueueSize     = 256
	watchedTrafficInsertTimeout = 5 * time.Second
	watchedTrafficMaxUsers      = 32
	WatchedTrafficTextLimit     = 96 * 1024
)

type WatchedTrafficSettingStore interface {
	GetValue(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

type WatchedTrafficRepository interface {
	Create(ctx context.Context, rec *WatchedTrafficRecord) error
	List(ctx context.Context, filter WatchedTrafficFilter) ([]WatchedTrafficRecord, *pagination.PaginationResult, error)
}

type WatchedTrafficConfig struct {
	Enabled bool    `json:"enabled"`
	UserIDs []int64 `json:"user_ids"`
}

type UpdateWatchedTrafficConfigInput struct {
	Enabled *bool    `json:"enabled"`
	UserIDs *[]int64 `json:"user_ids"`
}

type WatchedTrafficRecord struct {
	ID           int64     `json:"id"`
	RequestID    string    `json:"request_id"`
	UserID       int64     `json:"user_id"`
	UserEmail    string    `json:"user_email"`
	APIKeyID     *int64    `json:"api_key_id,omitempty"`
	APIKeyName   string    `json:"api_key_name"`
	GroupID      *int64    `json:"group_id,omitempty"`
	GroupName    string    `json:"group_name"`
	AccountID    *int64    `json:"account_id,omitempty"`
	Model        string    `json:"model"`
	Endpoint     string    `json:"endpoint"`
	StatusCode   int       `json:"status_code"`
	PromptText   string    `json:"prompt_text"`
	ResponseText string    `json:"response_text"`
	ErrorText    string    `json:"error_text"`
	CreatedAt    time.Time `json:"created_at"`
}

type WatchedTrafficFilter struct {
	UserID     *int64
	Pagination pagination.PaginationParams
}

type watchedTrafficSnapshot struct {
	Enabled bool
	Users   map[int64]struct{}
	Config  WatchedTrafficConfig
}

type WatchedTrafficService struct {
	settings WatchedTrafficSettingStore
	repo     WatchedTrafficRepository
	snapshot atomic.Value // *watchedTrafficSnapshot
	queue    chan WatchedTrafficRecord
	dropped  atomic.Uint64
}

func NewWatchedTrafficService(settings WatchedTrafficSettingStore, repo WatchedTrafficRepository) *WatchedTrafficService {
	s := &WatchedTrafficService{
		settings: settings,
		repo:     repo,
		queue:    make(chan WatchedTrafficRecord, watchedTrafficQueueSize),
	}
	s.replaceSnapshot(defaultWatchedTrafficConfig())
	if settings != nil {
		if cfg, err := s.loadConfig(context.Background()); err == nil {
			s.replaceSnapshot(cfg)
		}
	}
	if repo != nil {
		go s.worker()
	}
	return s
}

func defaultWatchedTrafficConfig() WatchedTrafficConfig {
	return WatchedTrafficConfig{Enabled: false, UserIDs: []int64{}}
}

func (s *WatchedTrafficService) currentSnapshot() *watchedTrafficSnapshot {
	if s == nil {
		return nil
	}
	snap, _ := s.snapshot.Load().(*watchedTrafficSnapshot)
	return snap
}

// Enabled is the gateway fast-path check. When false, middleware must not
// wrap the request or read the body.
func (s *WatchedTrafficService) Enabled() bool {
	snap := s.currentSnapshot()
	return snap != nil && snap.Enabled
}

// ShouldWatch is a memory-only lookup. It must not touch the database or
// any external API.
func (s *WatchedTrafficService) ShouldWatch(userID int64) bool {
	snap := s.currentSnapshot()
	if snap == nil || !snap.Enabled || userID <= 0 {
		return false
	}
	_, ok := snap.Users[userID]
	return ok
}

func (s *WatchedTrafficService) GetConfig(ctx context.Context) (WatchedTrafficConfig, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return WatchedTrafficConfig{}, err
	}
	s.replaceSnapshot(cfg)
	return cloneWatchedTrafficConfig(cfg), nil
}

func (s *WatchedTrafficService) UpdateConfig(ctx context.Context, input UpdateWatchedTrafficConfigInput) (WatchedTrafficConfig, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return WatchedTrafficConfig{}, err
	}
	if input.Enabled != nil {
		cfg.Enabled = *input.Enabled
	}
	if input.UserIDs != nil {
		cfg.UserIDs = normalizeWatchedUserIDs(*input.UserIDs)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return WatchedTrafficConfig{}, err
	}
	if s.settings == nil {
		return WatchedTrafficConfig{}, errors.New("watched traffic settings store is not configured")
	}
	if err := s.settings.Set(ctx, SettingKeyWatchedTrafficConfig, string(raw)); err != nil {
		return WatchedTrafficConfig{}, err
	}
	s.replaceSnapshot(cfg)
	return cloneWatchedTrafficConfig(cfg), nil
}

func (s *WatchedTrafficService) ListLogs(ctx context.Context, filter WatchedTrafficFilter) ([]WatchedTrafficRecord, *pagination.PaginationResult, error) {
	if s.repo == nil {
		return nil, nil, errors.New("watched traffic repository is not configured")
	}
	return s.repo.List(ctx, filter)
}

// Enqueue stores one captured request asynchronously. If the operator has
// already flipped the switch off, the record is dropped.
func (s *WatchedTrafficService) Enqueue(rec WatchedTrafficRecord) {
	if s == nil || !s.ShouldWatch(rec.UserID) {
		return
	}
	rec.PromptText = clipWatchedText(rec.PromptText)
	rec.ResponseText = clipWatchedText(rec.ResponseText)
	rec.ErrorText = clipWatchedText(rec.ErrorText)
	select {
	case s.queue <- rec:
	default:
		s.dropped.Add(1)
		if s.dropped.Load()%32 == 1 {
			slog.Warn("watched_traffic.queue_full", "dropped", s.dropped.Load(), "user_id", rec.UserID)
		}
	}
}

func (s *WatchedTrafficService) worker() {
	for rec := range s.queue {
		ctx, cancel := context.WithTimeout(context.Background(), watchedTrafficInsertTimeout)
		if err := s.repo.Create(ctx, &rec); err != nil {
			slog.Warn("watched_traffic.insert_failed", "error", err, "user_id", rec.UserID, "request_id", rec.RequestID)
		}
		cancel()
	}
}

func (s *WatchedTrafficService) loadConfig(ctx context.Context) (WatchedTrafficConfig, error) {
	cfg := defaultWatchedTrafficConfig()
	if s == nil || s.settings == nil {
		return cfg, nil
	}
	raw, err := s.settings.GetValue(ctx, SettingKeyWatchedTrafficConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return cfg, nil
		}
		return cfg, err
	}
	if raw == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultWatchedTrafficConfig(), err
	}
	cfg.UserIDs = normalizeWatchedUserIDs(cfg.UserIDs)
	return cfg, nil
}

func (s *WatchedTrafficService) replaceSnapshot(cfg WatchedTrafficConfig) {
	users := make(map[int64]struct{}, len(cfg.UserIDs))
	for _, id := range cfg.UserIDs {
		if id > 0 {
			users[id] = struct{}{}
		}
	}
	s.snapshot.Store(&watchedTrafficSnapshot{
		Enabled: cfg.Enabled,
		Users:   users,
		Config:  cloneWatchedTrafficConfig(cfg),
	})
}

func normalizeWatchedUserIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
		if len(out) >= watchedTrafficMaxUsers {
			break
		}
	}
	return out
}

func cloneWatchedTrafficConfig(cfg WatchedTrafficConfig) WatchedTrafficConfig {
	clone := cfg
	clone.UserIDs = append([]int64(nil), cfg.UserIDs...)
	if clone.UserIDs == nil {
		clone.UserIDs = []int64{}
	}
	return clone
}

func clipWatchedText(s string) string {
	if len(s) <= WatchedTrafficTextLimit {
		return s
	}
	return s[:WatchedTrafficTextLimit]
}
