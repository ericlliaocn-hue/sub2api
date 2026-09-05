package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const upstreamConnectionsSettingKey = "admin_upstream_connections"

const (
	UpstreamTypeSub2API = "sub2api"
	UpstreamTypeNewAPI  = "newapi"
	UpstreamTypeOther   = "other"
)

type UpstreamConnection struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	BaseURL     string     `json:"base_url"`
	Username    string     `json:"username"`
	HasPassword bool       `json:"has_password"`
	Status      string     `json:"status"`
	LastTestAt  *time.Time `json:"last_test_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type UpstreamConnectionInput struct {
	Name     string
	Type     string
	BaseURL  string
	Username string
	Password string
}

type UpstreamRemoteAccount struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Platform       string   `json:"platform,omitempty"`
	Type           string   `json:"type,omitempty"`
	Status         string   `json:"status,omitempty"`
	Schedulable    *bool    `json:"schedulable,omitempty"`
	Concurrency    *int     `json:"concurrency,omitempty"`
	Priority       *int     `json:"priority,omitempty"`
	Balance        *float64 `json:"balance,omitempty"`
	RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
	LastUsedAt     string   `json:"last_used_at,omitempty"`
	ErrorMessage   string   `json:"error_message,omitempty"`
}

type UpstreamRemoteGroup struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Platform       string   `json:"platform,omitempty"`
	Status         string   `json:"status,omitempty"`
	RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
	AccountCount   *int     `json:"account_count,omitempty"`
}

type UpstreamConnectionSnapshot struct {
	Connection UpstreamConnection      `json:"connection"`
	RemoteUser map[string]any          `json:"remote_user,omitempty"`
	Accounts   []UpstreamRemoteAccount `json:"accounts,omitempty"`
	Groups     []UpstreamRemoteGroup   `json:"groups,omitempty"`
	TestedAt   time.Time               `json:"tested_at"`
}

type storedUpstreamConnection struct {
	UpstreamConnection
	PasswordCipher string `json:"password_cipher,omitempty"`
}

type UpstreamConnectionService struct {
	settings  SettingRepository
	encryptor SecretEncryptor
	mu        sync.Mutex
}

func NewUpstreamConnectionService(settings SettingRepository, encryptor SecretEncryptor) *UpstreamConnectionService {
	return &UpstreamConnectionService{settings: settings, encryptor: encryptor}
}

func (s *UpstreamConnectionService) List(ctx context.Context) ([]UpstreamConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]UpstreamConnection, 0, len(items))
	for _, item := range items {
		out = append(out, item.UpstreamConnection)
	}
	return out, nil
}

func (s *UpstreamConnectionService) Create(ctx context.Context, input UpstreamConnectionInput) (UpstreamConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateUpstreamConnectionInput(input, true); err != nil {
		return UpstreamConnection{}, err
	}
	items, err := s.load(ctx)
	if err != nil {
		return UpstreamConnection{}, err
	}
	passwordCipher, err := s.encryptPassword(input.Password)
	if err != nil {
		return UpstreamConnection{}, err
	}
	now := time.Now()
	item := storedUpstreamConnection{
		UpstreamConnection: UpstreamConnection{
			ID:          nextUpstreamConnectionID(items),
			Name:        strings.TrimSpace(input.Name),
			Type:        normalizeUpstreamType(input.Type),
			BaseURL:     normalizeUpstreamBaseURL(input.BaseURL),
			Username:    strings.TrimSpace(input.Username),
			HasPassword: true,
			Status:      "untested",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		PasswordCipher: passwordCipher,
	}
	items = append(items, item)
	if err := s.save(ctx, items); err != nil {
		return UpstreamConnection{}, err
	}
	return item.UpstreamConnection, nil
}

func (s *UpstreamConnectionService) Update(ctx context.Context, id int64, input UpstreamConnectionInput) (UpstreamConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateUpstreamConnectionInput(input, false); err != nil {
		return UpstreamConnection{}, err
	}
	items, err := s.load(ctx)
	if err != nil {
		return UpstreamConnection{}, err
	}
	for i := range items {
		if items[i].ID != id {
			continue
		}
		item := &items[i]
		if strings.TrimSpace(input.Name) != "" {
			item.Name = strings.TrimSpace(input.Name)
		}
		if strings.TrimSpace(input.Type) != "" {
			item.Type = normalizeUpstreamType(input.Type)
		}
		if strings.TrimSpace(input.BaseURL) != "" {
			item.BaseURL = normalizeUpstreamBaseURL(input.BaseURL)
		}
		if strings.TrimSpace(input.Username) != "" {
			item.Username = strings.TrimSpace(input.Username)
		}
		if strings.TrimSpace(input.Password) != "" {
			item.PasswordCipher, err = s.encryptPassword(input.Password)
			if err != nil {
				return UpstreamConnection{}, err
			}
			item.HasPassword = true
		}
		item.Status = "untested"
		item.LastError = ""
		item.LastTestAt = nil
		item.UpdatedAt = time.Now()
		if err := s.save(ctx, items); err != nil {
			return UpstreamConnection{}, err
		}
		return item.UpstreamConnection, nil
	}
	return UpstreamConnection{}, fmt.Errorf("upstream connection %d not found", id)
}

func (s *UpstreamConnectionService) Delete(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.load(ctx)
	if err != nil {
		return err
	}
	filtered := items[:0]
	removed := false
	for _, item := range items {
		if item.ID == id {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !removed {
		return fmt.Errorf("upstream connection %d not found", id)
	}
	return s.save(ctx, filtered)
}

func (s *UpstreamConnectionService) Test(ctx context.Context, id int64) (*UpstreamConnectionSnapshot, error) {
	s.mu.Lock()
	items, err := s.load(ctx)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	index := -1
	for i := range items {
		if items[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		s.mu.Unlock()
		return nil, fmt.Errorf("upstream connection %d not found", id)
	}
	item := items[index]
	password, err := s.decryptPassword(item.PasswordCipher)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}

	testedAt := time.Now()
	snapshot, testErr := testRemoteConnection(ctx, item.UpstreamConnection, item.Username, password)
	s.mu.Lock()
	defer s.mu.Unlock()
	items, loadErr := s.load(ctx)
	if loadErr == nil {
		for i := range items {
			if items[i].ID == id {
				items[i].LastTestAt = &testedAt
				items[i].UpdatedAt = testedAt
				if testErr != nil {
					items[i].Status = "error"
					items[i].LastError = testErr.Error()
				} else {
					items[i].Status = "healthy"
					items[i].LastError = ""
				}
				_ = s.save(ctx, items)
				break
			}
		}
	}
	if testErr != nil {
		return nil, testErr
	}
	for i := range items {
		if items[i].ID == id {
			snapshot.Connection = items[i].UpstreamConnection
			break
		}
	}
	snapshot.TestedAt = testedAt
	return snapshot, nil
}

func (s *UpstreamConnectionService) load(ctx context.Context) ([]storedUpstreamConnection, error) {
	if s.settings == nil {
		return nil, errors.New("settings repository is unavailable")
	}
	raw, err := s.settings.GetValue(ctx, upstreamConnectionsSettingKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return []storedUpstreamConnection{}, nil
		}
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return []storedUpstreamConnection{}, nil
	}
	var items []storedUpstreamConnection
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("decode upstream connections: %w", err)
	}
	return items, nil
}

func (s *UpstreamConnectionService) save(ctx context.Context, items []storedUpstreamConnection) error {
	raw, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("encode upstream connections: %w", err)
	}
	return s.settings.Set(ctx, upstreamConnectionsSettingKey, string(raw))
}

func (s *UpstreamConnectionService) encryptPassword(password string) (string, error) {
	if s.encryptor == nil {
		return "", errors.New("secret encryptor is unavailable")
	}
	return s.encryptor.Encrypt(password)
}

func (s *UpstreamConnectionService) decryptPassword(ciphertext string) (string, error) {
	if s.encryptor == nil {
		return "", errors.New("secret encryptor is unavailable")
	}
	return s.encryptor.Decrypt(ciphertext)
}

func validateUpstreamConnectionInput(input UpstreamConnectionInput, creating bool) error {
	if creating && strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(input.BaseURL) == "" {
		return errors.New("base_url is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(input.BaseURL))
	if err != nil || parsed.Scheme != "https" && parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("base_url must be an http(s) URL without credentials, query, or fragment")
	}
	upstreamType := strings.TrimSpace(input.Type)
	if upstreamType != "" && upstreamType != UpstreamTypeSub2API && upstreamType != UpstreamTypeNewAPI && upstreamType != UpstreamTypeOther {
		return errors.New("type must be sub2api, newapi, or other")
	}
	if strings.TrimSpace(input.Username) == "" {
		return errors.New("username is required")
	}
	if creating && strings.TrimSpace(input.Password) == "" {
		return errors.New("password is required")
	}
	return nil
}

func normalizeUpstreamType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case UpstreamTypeNewAPI:
		return UpstreamTypeNewAPI
	case UpstreamTypeOther:
		return UpstreamTypeOther
	default:
		return UpstreamTypeSub2API
	}
}

func normalizeUpstreamBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func nextUpstreamConnectionID(items []storedUpstreamConnection) int64 {
	var maxID int64
	for _, item := range items {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	return maxID + 1
}

type remoteEnvelope struct {
	Code    any             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type remoteAuth struct {
	Token  string
	UserID string
}

func testRemoteConnection(ctx context.Context, connection UpstreamConnection, username, password string) (*UpstreamConnectionSnapshot, error) {
	if connection.Type == UpstreamTypeOther {
		return nil, errors.New("other upstream type is not adapted yet")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create upstream cookie jar: %w", err)
	}
	client := &http.Client{Timeout: 15 * time.Second, Jar: jar}
	auth, err := remoteLogin(ctx, client, connection, username, password)
	if err != nil {
		return nil, err
	}
	snapshot := &UpstreamConnectionSnapshot{Connection: connection}
	if connection.Type == UpstreamTypeSub2API {
		snapshot.RemoteUser, _ = remoteJSONMap(ctx, client, connection, auth, "/api/v1/auth/me")
		if snapshot.Accounts, err = remoteAccounts(ctx, client, connection, auth); err != nil {
			return nil, err
		}
		if snapshot.Groups, err = remoteGroups(ctx, client, connection, auth); err != nil {
			return nil, err
		}
	} else {
		snapshot.RemoteUser, _ = remoteJSONMap(ctx, client, connection, auth, "/api/user/self")
		snapshot.Accounts, err = remoteAccountsFromPaths(ctx, client, connection, auth, []string{"/api/channel/", "/api/channels"})
		if err != nil {
			return nil, err
		}
		snapshot.Groups, err = remoteGroupsFromPaths(ctx, client, connection, auth, []string{"/api/group/", "/api/groups"})
		if err != nil {
			return nil, err
		}
	}
	return snapshot, nil
}

func remoteLogin(ctx context.Context, client *http.Client, connection UpstreamConnection, username, password string) (remoteAuth, error) {
	path := "/api/v1/auth/login"
	payload := map[string]string{"email": username, "password": password}
	if connection.Type == UpstreamTypeNewAPI {
		path = "/api/user/login"
		payload = map[string]string{"username": username, "password": password}
	}
	body, _ := json.Marshal(payload)
	status, response, err := remoteRequest(ctx, client, connection, http.MethodPost, path, remoteAuth{}, body)
	if err != nil {
		return remoteAuth{}, err
	}
	if status < 200 || status >= 300 {
		return remoteAuth{}, fmt.Errorf("upstream login failed: HTTP %d", status)
	}
	var envelope remoteEnvelope
	if err := json.Unmarshal(response, &envelope); err != nil {
		return remoteAuth{}, fmt.Errorf("decode upstream login response: %w", err)
	}
	var fields map[string]any
	_ = json.Unmarshal(response, &fields)
	if success, ok := fields["success"].(bool); ok && !success {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message, _ = fields["message"].(string)
		}
		if strings.TrimSpace(message) == "" {
			message = "upstream rejected the credentials"
		}
		return remoteAuth{}, errors.New(message)
	}
	auth := remoteAuth{}
	for _, candidate := range []json.RawMessage{envelope.Data, response} {
		var nested map[string]any
		if json.Unmarshal(candidate, &nested) != nil {
			continue
		}
		if auth.UserID == "" {
			auth.UserID = upstreamStringValue(nested, "id", "user_id")
		}
		for _, key := range []string{"access_token", "token", "jwt"} {
			if value, ok := nested[key].(string); ok && strings.TrimSpace(value) != "" {
				auth.Token = value
				break
			}
		}
	}
	if auth.Token != "" || connection.Type == UpstreamTypeNewAPI && hasRemoteSessionCookie(client, connection.BaseURL) {
		return auth, nil
	}
	return remoteAuth{}, errors.New("upstream login response did not contain an access token or session cookie")
}

func hasRemoteSessionCookie(client *http.Client, baseURL string) bool {
	if client == nil || client.Jar == nil {
		return false
	}
	parsed, err := url.Parse(baseURL)
	return err == nil && len(client.Jar.Cookies(parsed)) > 0
}

func remoteAccounts(ctx context.Context, client *http.Client, connection UpstreamConnection, auth remoteAuth) ([]UpstreamRemoteAccount, error) {
	return remoteAccountsFromPaths(ctx, client, connection, auth, []string{"/api/v1/admin/accounts?page=1&page_size=100"})
}

func remoteGroups(ctx context.Context, client *http.Client, connection UpstreamConnection, auth remoteAuth) ([]UpstreamRemoteGroup, error) {
	return remoteGroupsFromPaths(ctx, client, connection, auth, []string{"/api/v1/admin/groups/all"})
}

func remoteAccountsFromPaths(ctx context.Context, client *http.Client, connection UpstreamConnection, auth remoteAuth, paths []string) ([]UpstreamRemoteAccount, error) {
	var lastErr error
	for _, path := range paths {
		status, body, err := remoteRequest(ctx, client, connection, http.MethodGet, path, auth, nil)
		if err != nil {
			lastErr = err
			continue
		}
		if status < 200 || status >= 300 {
			lastErr = fmt.Errorf("upstream account query failed: HTTP %d", status)
			continue
		}
		items := extractRemoteAccounts(body)
		if items != nil {
			return items, nil
		}
		lastErr = errors.New("upstream account response shape is unsupported")
	}
	return nil, lastErr
}

func remoteGroupsFromPaths(ctx context.Context, client *http.Client, connection UpstreamConnection, auth remoteAuth, paths []string) ([]UpstreamRemoteGroup, error) {
	var lastErr error
	for _, path := range paths {
		status, body, err := remoteRequest(ctx, client, connection, http.MethodGet, path, auth, nil)
		if err != nil {
			lastErr = err
			continue
		}
		if status < 200 || status >= 300 {
			lastErr = fmt.Errorf("upstream group query failed: HTTP %d", status)
			continue
		}
		items := extractRemoteGroups(body)
		if items != nil {
			return items, nil
		}
		lastErr = errors.New("upstream group response shape is unsupported")
	}
	return nil, lastErr
}

func remoteJSONMap(ctx context.Context, client *http.Client, connection UpstreamConnection, auth remoteAuth, path string) (map[string]any, error) {
	status, body, err := remoteRequest(ctx, client, connection, http.MethodGet, path, auth, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("upstream user query failed: HTTP %d", status)
	}
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func remoteRequest(ctx context.Context, client *http.Client, connection UpstreamConnection, method, path string, auth remoteAuth, body []byte) (int, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, connection.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(auth.Token) != "" {
		request.Header.Set("Authorization", "Bearer "+auth.Token)
	}
	if connection.Type == UpstreamTypeNewAPI && strings.TrimSpace(auth.UserID) != "" {
		request.Header.Set("New-Api-User", auth.UserID)
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return response.StatusCode, nil, err
	}
	return response.StatusCode, data, nil
}

func extractRemoteAccounts(body []byte) []UpstreamRemoteAccount {
	values := extractArray(body)
	if values == nil {
		return nil
	}
	items := make([]UpstreamRemoteAccount, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			items = append(items, remoteAccountFromMap(item))
		}
	}
	return items
}

func extractRemoteGroups(body []byte) []UpstreamRemoteGroup {
	values := extractArray(body)
	if values == nil {
		return nil
	}
	items := make([]UpstreamRemoteGroup, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			items = append(items, remoteGroupFromMap(item))
		}
	}
	return items
}

func extractArray(body []byte) []any {
	var root any
	if json.Unmarshal(body, &root) != nil {
		return nil
	}
	var find func(any) []any
	find = func(value any) []any {
		switch typed := value.(type) {
		case []any:
			return typed
		case map[string]any:
			for _, key := range []string{"data", "items", "list", "results", "channels", "groups"} {
				if nested, ok := typed[key]; ok {
					if found := find(nested); found != nil {
						return found
					}
				}
			}
		}
		return nil
	}
	return find(root)
}

func remoteAccountFromMap(value map[string]any) UpstreamRemoteAccount {
	item := UpstreamRemoteAccount{ID: upstreamStringValue(value, "id", "account_id", "channel_id"), Name: upstreamStringValue(value, "name", "key", "channel_name"), Platform: upstreamStringValue(value, "platform"), Type: upstreamStringValue(value, "type"), Status: upstreamStringValue(value, "status"), LastUsedAt: upstreamStringValue(value, "last_used_at", "updated_at"), ErrorMessage: upstreamStringValue(value, "error_message", "error")}
	item.Schedulable = boolPointer(value, "schedulable", "enabled")
	item.Concurrency = intPointer(value, "concurrency", "max_concurrency")
	item.Priority = intPointer(value, "priority")
	item.Balance = floatPointer(value, "balance", "quota")
	item.RateMultiplier = floatPointer(value, "rate_multiplier", "multiplier")
	return item
}

func remoteGroupFromMap(value map[string]any) UpstreamRemoteGroup {
	return UpstreamRemoteGroup{ID: upstreamStringValue(value, "id", "group_id"), Name: upstreamStringValue(value, "name", "group_name"), Platform: upstreamStringValue(value, "platform"), Status: upstreamStringValue(value, "status"), RateMultiplier: floatPointer(value, "rate_multiplier", "multiplier"), AccountCount: intPointer(value, "account_count", "accounts_count")}
}

func upstreamStringValue(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := value[key]; ok {
			return fmt.Sprint(v)
		}
	}
	return ""
}
func floatPointer(value map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		switch v := value[key].(type) {
		case float64:
			return &v
		case json.Number:
			n, _ := strconv.ParseFloat(v.String(), 64)
			return &n
		}
	}
	return nil
}
func intPointer(value map[string]any, keys ...string) *int {
	for _, key := range keys {
		switch v := value[key].(type) {
		case float64:
			n := int(v)
			return &n
		case json.Number:
			n, _ := strconv.Atoi(v.String())
			return &n
		}
	}
	return nil
}
func boolPointer(value map[string]any, keys ...string) *bool {
	for _, key := range keys {
		if v, ok := value[key].(bool); ok {
			return &v
		}
	}
	return nil
}
