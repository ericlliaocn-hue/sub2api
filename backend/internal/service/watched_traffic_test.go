package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

type watchedSettingStub struct {
	mu     sync.Mutex
	values map[string]string
}

func (s *watchedSettingStub) GetValue(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		return "", ErrSettingNotFound
	}
	raw, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return raw, nil
}

func (s *watchedSettingStub) Set(_ context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func TestWatchedTrafficShouldWatchFastPath(t *testing.T) {
	svc := NewWatchedTrafficService(&watchedSettingStub{}, nil)
	if svc.Enabled() {
		t.Fatal("default config must be disabled")
	}
	if svc.ShouldWatch(295) {
		t.Fatal("disabled switch must ignore watched users")
	}

	enabled := true
	if _, err := svc.UpdateConfig(context.Background(), UpdateWatchedTrafficConfigInput{
		Enabled: &enabled,
		UserIDs: &[]int64{295},
	}); err != nil {
		t.Fatal(err)
	}
	if !svc.Enabled() {
		t.Fatal("enabled switch should flip immediately")
	}
	if !svc.ShouldWatch(295) {
		t.Fatal("user 295 should be watched")
	}
	if svc.ShouldWatch(250) {
		t.Fatal("other users must stay on the fast path")
	}

	off := false
	if _, err := svc.UpdateConfig(context.Background(), UpdateWatchedTrafficConfigInput{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	if svc.Enabled() || svc.ShouldWatch(295) {
		t.Fatal("turning the switch off must drop back to the fast path immediately")
	}
}

func TestWatchedTrafficEnqueueDropsAfterDisable(t *testing.T) {
	svc := NewWatchedTrafficService(&watchedSettingStub{}, nil)
	enabled := true
	if _, err := svc.UpdateConfig(context.Background(), UpdateWatchedTrafficConfigInput{
		Enabled: &enabled,
		UserIDs: &[]int64{295},
	}); err != nil {
		t.Fatal(err)
	}
	off := false
	if _, err := svc.UpdateConfig(context.Background(), UpdateWatchedTrafficConfigInput{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	svc.Enqueue(WatchedTrafficRecord{UserID: 295, PromptText: "should drop"})
	if len(svc.queue) != 0 {
		t.Fatalf("disabled enqueue must drop, queue=%d", len(svc.queue))
	}
}

func TestExtractWatchedPromptAndSSE(t *testing.T) {
	model, prompt := ExtractWatchedPrompt([]byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello jailbreak"}]}`))
	if model != "gpt-5.4" {
		t.Fatalf("model=%q", model)
	}
	if prompt != "user: hello jailbreak" {
		t.Fatalf("prompt=%q", prompt)
	}

	responseText, errorText := ExtractWatchedResponse([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: {\"error\":{\"message\":\"usage policy\"}}\n\n"), 200)
	if responseText != "hi" {
		t.Fatalf("response=%q", responseText)
	}
	if errorText != "usage policy" {
		t.Fatalf("error=%q", errorText)
	}

	_, errText := ExtractWatchedResponse([]byte(`{"error":{"message":"Incorrect API key provided"}}`), 401)
	if errText != "Incorrect API key provided" {
		t.Fatalf("401 error=%q", errText)
	}
}

func TestWatchedTrafficConfigPersistsJSON(t *testing.T) {
	store := &watchedSettingStub{}
	svc := NewWatchedTrafficService(store, nil)
	enabled := true
	if _, err := svc.UpdateConfig(context.Background(), UpdateWatchedTrafficConfigInput{
		Enabled: &enabled,
		UserIDs: &[]int64{295, 295, 0, 250},
	}); err != nil {
		t.Fatal(err)
	}
	var saved WatchedTrafficConfig
	if err := json.Unmarshal([]byte(store.values[SettingKeyWatchedTrafficConfig]), &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.Enabled || len(saved.UserIDs) != 2 || saved.UserIDs[0] != 295 || saved.UserIDs[1] != 250 {
		t.Fatalf("saved=%+v", saved)
	}
}
