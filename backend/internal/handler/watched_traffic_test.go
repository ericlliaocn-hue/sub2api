package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestWatchedTrafficMiddlewareDisabledDoesNotWrap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewWatchedTrafficService(&watchedSettingStore{}, nil)
	var sawBody io.ReadCloser
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 1, UserID: 295})
		c.Next()
	})
	router.Use(WatchedTrafficMiddleware(svc))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		sawBody = c.Request.Body
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-5.4"}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if _, ok := sawBody.(*limitedCaptureReader); ok {
		t.Fatal("disabled switch must not wrap the request body")
	}
}

func TestWatchedTrafficMiddlewareRecordsWatchedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &watchedSettingStore{}
	repo := &watchedRepoStub{ch: make(chan service.WatchedTrafficRecord, 1)}
	svc := service.NewWatchedTrafficService(store, repo)
	enabled := true
	if _, err := svc.UpdateConfig(context.Background(), service.UpdateWatchedTrafficConfigInput{
		Enabled: &enabled,
		UserIDs: &[]int64{295},
	}); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
			ID: 322, UserID: 295, Name: "hk",
			User:  &service.User{ID: 295, Email: "s1976910941@gmail.com"},
			Group: &service.Group{ID: 19, Name: "pro号池(严禁破限)"},
		})
		c.Next()
	})
	router.Use(WatchedTrafficMiddleware(svc))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		if !bytes.Contains(body, []byte("hello")) {
			t.Fatalf("handler must still see the body: %s", body)
		}
		c.Header("Content-Type", "text/event-stream")
		c.String(http.StatusOK, "data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if got := w.Body.String(); !bytes.Contains([]byte(got), []byte("world")) {
		t.Fatalf("client response lost: %s", got)
	}

	select {
	case rec := <-repo.ch:
		if rec.UserID != 295 || rec.PromptText != "user: hello" || rec.ResponseText != "world" || rec.Model != "gpt-5.4" {
			t.Fatalf("record=%+v", rec)
		}
		if rec.UserEmail != "s1976910941@gmail.com" || rec.APIKeyName != "hk" {
			t.Fatalf("identity=%+v", rec)
		}
	case <-time.After(time.Second):
		t.Fatal("expected an async record for the watched user")
	}
}

func TestWatchedTrafficMiddlewareSkipsOtherUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &watchedSettingStore{}
	repo := &watchedRepoStub{ch: make(chan service.WatchedTrafficRecord, 1)}
	svc := service.NewWatchedTrafficService(store, repo)
	enabled := true
	if _, err := svc.UpdateConfig(context.Background(), service.UpdateWatchedTrafficConfigInput{
		Enabled: &enabled,
		UserIDs: &[]int64{295},
	}); err != nil {
		t.Fatal(err)
	}

	var sawBody io.ReadCloser
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 1, UserID: 250})
		c.Next()
	})
	router.Use(WatchedTrafficMiddleware(svc))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		sawBody = c.Request.Body
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-5.4"}`))
	router.ServeHTTP(httptest.NewRecorder(), req)
	if _, ok := sawBody.(*limitedCaptureReader); ok {
		t.Fatal("unwatched users must stay on the fast path")
	}
	select {
	case rec := <-repo.ch:
		t.Fatalf("unexpected record: %+v", rec)
	case <-time.After(50 * time.Millisecond):
	}
}

type watchedSettingStore struct {
	values map[string]string
}

func (s *watchedSettingStore) GetValue(_ context.Context, key string) (string, error) {
	if s.values == nil {
		return "", service.ErrSettingNotFound
	}
	raw, ok := s.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return raw, nil
}

func (s *watchedSettingStore) Set(_ context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

type watchedRepoStub struct {
	ch chan service.WatchedTrafficRecord
}

func (r *watchedRepoStub) Create(_ context.Context, rec *service.WatchedTrafficRecord) error {
	if r.ch != nil && rec != nil {
		r.ch <- *rec
	}
	return nil
}

func (r *watchedRepoStub) List(context.Context, service.WatchedTrafficFilter) ([]service.WatchedTrafficRecord, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
