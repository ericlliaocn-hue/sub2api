package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type upstreamConnectionSettingRepo struct {
	mu   sync.Mutex
	data map[string]string
}

func newUpstreamConnectionSettingRepo() *upstreamConnectionSettingRepo {
	return &upstreamConnectionSettingRepo{data: make(map[string]string)}
}

func (r *upstreamConnectionSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	value, err := r.GetValue(context.Background(), key)
	if err != nil || value == "" {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}
func (r *upstreamConnectionSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.data[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (r *upstreamConnectionSettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[key] = value
	return nil
}
func (r *upstreamConnectionSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}
func (r *upstreamConnectionSettingRepo) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (r *upstreamConnectionSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}
func (r *upstreamConnectionSettingRepo) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, key)
	return nil
}

type upstreamConnectionEncryptor struct{}

func (upstreamConnectionEncryptor) Encrypt(value string) (string, error) {
	return "enc:" + base64.StdEncoding.EncodeToString([]byte(value)), nil
}
func (upstreamConnectionEncryptor) Decrypt(value string) (string, error) {
	encoded, ok := strings.CutPrefix(value, "enc:")
	if !ok {
		return "", errors.New("invalid ciphertext")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	return string(decoded), err
}

func newUpstreamConnectionTestService() (*UpstreamConnectionService, *upstreamConnectionSettingRepo) {
	repo := newUpstreamConnectionSettingRepo()
	return NewUpstreamConnectionService(repo, upstreamConnectionEncryptor{}), repo
}

func TestUpstreamConnectionCreateDefaultsToSub2APIAndMasksPassword(t *testing.T) {
	svc, repo := newUpstreamConnectionTestService()
	created, err := svc.Create(context.Background(), UpstreamConnectionInput{
		Name: "Primary", BaseURL: "https://example.com/", Username: "admin", Password: "top-secret",
	})
	require.NoError(t, err)
	require.Equal(t, UpstreamTypeSub2API, created.Type)
	require.True(t, created.HasPassword)

	raw := repo.data[upstreamConnectionsSettingKey]
	require.NotContains(t, raw, "top-secret")
	require.Contains(t, raw, `"password_cipher":"enc:`)
	encoded, err := json.Marshal(created)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "top-secret")
	require.NotContains(t, string(encoded), "password_cipher")
}

func TestUpstreamConnectionSub2APITestReadsAccountsAndGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/login":
			require.Equal(t, http.MethodPost, r.Method)
			_, _ = w.Write([]byte(`{"data":{"access_token":"test-token"}}`))
		case "/api/v1/auth/me":
			require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"id":1,"email":"admin@example.com"}`))
		case "/api/v1/admin/accounts":
			require.Equal(t, "100", r.URL.Query().Get("page_size"))
			_, _ = w.Write([]byte(`{"data":{"items":[{"id":11,"name":"Luna","platform":"openai","status":"active","balance":12.5,"rate_multiplier":1.2}]}}`))
		case "/api/v1/admin/groups/all":
			_, _ = w.Write([]byte(`{"data":[{"id":3,"name":"Pro","platform":"openai","account_count":6}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc, _ := newUpstreamConnectionTestService()
	created, err := svc.Create(context.Background(), UpstreamConnectionInput{Name: "Sub2API", Type: UpstreamTypeSub2API, BaseURL: server.URL, Username: "admin@example.com", Password: "secret"})
	require.NoError(t, err)
	snapshot, err := svc.Test(context.Background(), created.ID)
	require.NoError(t, err)
	require.Len(t, snapshot.Accounts, 1)
	require.Equal(t, "Luna", snapshot.Accounts[0].Name)
	require.Equal(t, 12.5, *snapshot.Accounts[0].Balance)
	require.Len(t, snapshot.Groups, 1)
	require.Equal(t, 6, *snapshot.Groups[0].AccountCount)
	require.Equal(t, "healthy", snapshot.Connection.Status)
}

func TestUpstreamConnectionNewAPITestUsesSessionAndUserHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/user/login" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "session-value", Path: "/"})
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":7,"username":"admin"}}`))
			return
		}
		cookie, err := r.Cookie("session")
		require.NoError(t, err)
		require.Equal(t, "session-value", cookie.Value)
		require.Equal(t, "7", r.Header.Get("New-Api-User"))
		switch r.URL.Path {
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":7}}`))
		case "/api/channel/":
			_, _ = w.Write([]byte(`{"data":[{"id":2,"name":"channel","status":1}]}`))
		case "/api/group/":
			_, _ = w.Write([]byte(`{"data":[{"id":"default","name":"default","rate_multiplier":1}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc, _ := newUpstreamConnectionTestService()
	created, err := svc.Create(context.Background(), UpstreamConnectionInput{Name: "NewAPI", Type: UpstreamTypeNewAPI, BaseURL: server.URL, Username: "admin", Password: "secret"})
	require.NoError(t, err)
	snapshot, err := svc.Test(context.Background(), created.ID)
	require.NoError(t, err)
	require.Len(t, snapshot.Accounts, 1)
	require.Len(t, snapshot.Groups, 1)
}

func TestUpstreamConnectionFailedTestMarksConnectionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	svc, _ := newUpstreamConnectionTestService()
	created, err := svc.Create(context.Background(), UpstreamConnectionInput{Name: "Broken", Type: UpstreamTypeSub2API, BaseURL: server.URL, Username: "admin", Password: "wrong"})
	require.NoError(t, err)
	_, err = svc.Test(context.Background(), created.ID)
	require.ErrorContains(t, err, "HTTP 401")
	items, listErr := svc.List(context.Background())
	require.NoError(t, listErr)
	require.Equal(t, "error", items[0].Status)
	require.Contains(t, items[0].LastError, "HTTP 401")
}

func TestUpstreamConnectionOtherTypeIsExplicitlyUnsupported(t *testing.T) {
	svc, _ := newUpstreamConnectionTestService()
	created, err := svc.Create(context.Background(), UpstreamConnectionInput{Name: "Custom", Type: UpstreamTypeOther, BaseURL: "https://example.com", Username: "admin", Password: "secret"})
	require.NoError(t, err)
	_, err = svc.Test(context.Background(), created.ID)
	require.ErrorContains(t, err, "not adapted")
}
