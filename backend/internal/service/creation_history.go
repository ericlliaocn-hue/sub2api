package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	creationHistoryPageSize    = 20
	creationHistoryMaxPageSize = 100
	creationAssetMaxBytes      = 512 << 20
)

var (
	ErrCreationTaskNotFound  = infraerrors.NotFound("CREATION_TASK_NOT_FOUND", "creation task not found")
	ErrCreationAssetNotFound = infraerrors.NotFound("CREATION_ASSET_NOT_FOUND", "creation asset not found")
	ErrCreationAssetURL      = infraerrors.BadRequest("CREATION_ASSET_URL_INVALID", "asset URL is invalid or not allowed")
	ErrCreationAssetTooLarge = infraerrors.BadRequest("CREATION_ASSET_TOO_LARGE", "asset is too large")
)

// CreationHistoryTask is the user-facing projection of creation.generation_task.
type CreationHistoryTask struct {
	ID             int64                  `json:"id"`
	APIKeyID       int64                  `json:"api_key_id"`
	Kind           string                 `json:"kind"`
	Model          string                 `json:"model"`
	Prompt         string                 `json:"prompt"`
	Request        map[string]any         `json:"request,omitempty"`
	Status         string                 `json:"status"`
	ProviderTaskID string                 `json:"provider_task_id,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	FinishedAt     *time.Time             `json:"finished_at,omitempty"`
	Assets         []CreationHistoryAsset `json:"assets"`
}

type CreationAdminHistoryTask struct {
	CreationHistoryTask
	UserID     int64  `json:"user_id"`
	UserEmail  string `json:"user_email"`
	APIKeyName string `json:"api_key_name,omitempty"`
	GroupID    int64  `json:"group_id,omitempty"`
	GroupName  string `json:"group_name,omitempty"`
}

type CreationAdminHistoryFilters struct {
	Search string
	Kind   string
	Status string
	Model  string
}

type CreationHistoryAsset struct {
	ID         int64     `json:"id"`
	Kind       string    `json:"kind"`
	MimeType   string    `json:"mime_type,omitempty"`
	ContentURL string    `json:"content_url"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateCreationTaskInput struct {
	UserID         int64
	APIKeyID       int64
	Kind           string
	Model          string
	Prompt         string
	Request        map[string]any
	IdempotencyKey string
}

type UpdateCreationTaskInput struct {
	Status         string
	ProviderTaskID string
	ErrorMessage   string
}

type CreationHistoryService struct {
	db        *sql.DB
	mediaRoot string
	http      *http.Client
}

func NewCreationHistoryService(db *sql.DB, cfg *config.Config) *CreationHistoryService {
	root := "data"
	if cfg != nil && strings.TrimSpace(cfg.Pricing.DataDir) != "" {
		root = cfg.Pricing.DataDir
	}
	root = filepath.Join(root, "creation-media")
	return &CreationHistoryService{
		db:        db,
		mediaRoot: root,
		http:      &http.Client{Timeout: 90 * time.Second},
	}
}

func (s *CreationHistoryService) CreateTask(ctx context.Context, input CreateCreationTaskInput) (*CreationHistoryTask, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("creation history database is not configured")
	}
	if input.UserID <= 0 || input.APIKeyID <= 0 || strings.TrimSpace(input.Kind) == "" || strings.TrimSpace(input.Model) == "" {
		return nil, infraerrors.BadRequest("CREATION_TASK_INVALID", "invalid creation task")
	}
	requestBody, err := json.Marshal(input.Request)
	if err != nil {
		return nil, fmt.Errorf("marshal creation request: %w", err)
	}

	if key := strings.TrimSpace(input.IdempotencyKey); key != "" {
		existing, lookupErr := s.getTaskByIdempotencyKey(ctx, input.UserID, input.APIKeyID, key)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing != nil {
			return existing, nil
		}
	}

	var task CreationHistoryTask
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO creation.generation_task
			(user_id, api_key_id, kind, status, prompt, request, idempotency_key)
		VALUES ($1, $2, $3, 'queued', $4, $5::jsonb, NULLIF($6, ''))
		RETURNING id, kind, status, prompt, created_at`,
		input.UserID, input.APIKeyID, strings.TrimSpace(input.Kind), input.Prompt, requestBody, strings.TrimSpace(input.IdempotencyKey),
	).Scan(&task.ID, &task.Kind, &task.Status, &task.Prompt, &task.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "generation_task_user_id_api_key_id_idempotency_key_key") {
			return s.getTaskByIdempotencyKey(ctx, input.UserID, input.APIKeyID, input.IdempotencyKey)
		}
		return nil, fmt.Errorf("create creation task: %w", err)
	}
	task.Model = input.Model
	task.APIKeyID = input.APIKeyID
	_ = s.appendEvent(ctx, task.ID, "task_created", task.Status, nil)
	return &task, nil
}

// BindVideoRequestAccount persists the provider account that owns a video
// request. The Redis sticky-session binding remains the hot path, while this
// durable binding lets status/content lookups recover after a process restart.
// A missing creation task is intentionally a no-op because the same gateway
// endpoint is also available to ordinary API clients outside the studio.
func (s *CreationHistoryService) BindVideoRequestAccount(ctx context.Context, userID, apiKeyID, taskID int64, providerTaskID string, providerAccountID int64) error {
	if s == nil || s.db == nil {
		return errors.New("creation history database is not configured")
	}
	providerTaskID = strings.TrimSpace(providerTaskID)
	if userID <= 0 || apiKeyID <= 0 || providerTaskID == "" || providerAccountID <= 0 {
		return errors.New("invalid video request binding")
	}

	var query string
	var args []any
	if taskID > 0 {
		query = `
			WITH task AS (
				SELECT id FROM creation.generation_task
				WHERE id = $1 AND user_id = $2 AND api_key_id = $3 AND kind = 'video'
				  AND (provider_task_id IS NULL OR provider_task_id = $5)
			)
			INSERT INTO creation.generation_attempt
				(task_id, attempt_no, sub2api_account_id, status, external_task_id, updated_at)
			SELECT id, 1, $4, 'submitted', $5, NOW() FROM task
			ON CONFLICT (task_id, attempt_no) DO UPDATE SET
				sub2api_account_id = EXCLUDED.sub2api_account_id,
				status = EXCLUDED.status,
				external_task_id = EXCLUDED.external_task_id,
				updated_at = NOW()`
		args = []any{taskID, userID, apiKeyID, providerAccountID, providerTaskID}
	} else {
		query = `
			WITH task AS (
				SELECT id FROM creation.generation_task
				WHERE user_id = $1 AND api_key_id = $2 AND kind = 'video'
				  AND provider_task_id = $3
				ORDER BY id DESC LIMIT 1
			)
			INSERT INTO creation.generation_attempt
				(task_id, attempt_no, sub2api_account_id, status, external_task_id, updated_at)
			SELECT id, 1, $4, 'submitted', $3, NOW() FROM task
			ON CONFLICT (task_id, attempt_no) DO UPDATE SET
				sub2api_account_id = EXCLUDED.sub2api_account_id,
				status = EXCLUDED.status,
				external_task_id = EXCLUDED.external_task_id,
				updated_at = NOW()`
		args = []any{userID, apiKeyID, providerTaskID, providerAccountID}
	}
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("persist video request binding: %w", err)
	}
	return nil
}

// ResolveVideoRequestAccount resolves only a task owned by the authenticated
// user and API key. It is used as a recovery path when the Redis binding is
// gone; it never falls back to account scheduling or another user's task.
func (s *CreationHistoryService) ResolveVideoRequestAccount(ctx context.Context, userID, apiKeyID int64, providerTaskID string) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("creation history database is not configured")
	}
	providerTaskID = strings.TrimSpace(providerTaskID)
	if userID <= 0 || apiKeyID <= 0 || providerTaskID == "" {
		return 0, errors.New("invalid video request lookup")
	}
	var accountID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT attempt.sub2api_account_id
		FROM creation.generation_attempt AS attempt
		JOIN creation.generation_task AS task ON task.id = attempt.task_id
		WHERE task.user_id = $1 AND task.api_key_id = $2
		  AND (task.provider_task_id = $3 OR attempt.external_task_id = $3)
		  AND attempt.sub2api_account_id IS NOT NULL
		ORDER BY attempt.updated_at DESC, attempt.id DESC
		LIMIT 1`, userID, apiKeyID, providerTaskID).Scan(&accountID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("resolve video request binding: %w", err)
	}
	return accountID, nil
}

func (s *CreationHistoryService) UpdateTask(ctx context.Context, userID, taskID int64, input UpdateCreationTaskInput) error {
	if s == nil || s.db == nil {
		return errors.New("creation history database is not configured")
	}
	if userID <= 0 || taskID <= 0 {
		return ErrCreationTaskNotFound
	}
	status := strings.TrimSpace(input.Status)
	providerTaskID := strings.TrimSpace(input.ProviderTaskID)
	errorMessage := strings.TrimSpace(input.ErrorMessage)
	var finishedAt any
	if isCreationTerminalStatus(status) {
		finishedAt = time.Now().UTC()
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE creation.generation_task
		SET status = COALESCE(NULLIF($1, ''), status),
			provider_task_id = COALESCE(NULLIF($2, ''), provider_task_id),
			error_message = NULLIF($3, ''),
			started_at = CASE WHEN NULLIF($1, '') IS NOT NULL AND started_at IS NULL THEN NOW() ELSE started_at END,
			finished_at = COALESCE($4::timestamptz, finished_at)
		WHERE id = $5 AND user_id = $6`, status, providerTaskID, errorMessage, finishedAt, taskID, userID)
	if err != nil {
		return fmt.Errorf("update creation task: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrCreationTaskNotFound
	}
	_ = s.appendEvent(ctx, taskID, "status_changed", status, map[string]any{"provider_task_id": providerTaskID, "error_message": errorMessage})
	return nil
}

func (s *CreationHistoryService) ListTasks(ctx context.Context, userID int64, page, pageSize int) ([]CreationHistoryTask, int64, error) {
	if s == nil || s.db == nil {
		return nil, 0, errors.New("creation history database is not configured")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = creationHistoryPageSize
	}
	if pageSize > creationHistoryMaxPageSize {
		pageSize = creationHistoryMaxPageSize
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM creation.generation_task WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count creation tasks: %w", err)
	}
	offset := (page - 1) * pageSize
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(api_key_id, 0), kind, status, prompt, request, COALESCE(provider_task_id, ''), COALESCE(error_message, ''), created_at, finished_at
		FROM creation.generation_task
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`, userID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list creation tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]CreationHistoryTask, 0, pageSize)
	for rows.Next() {
		var task CreationHistoryTask
		var requestBody []byte
		if err := rows.Scan(&task.ID, &task.APIKeyID, &task.Kind, &task.Status, &task.Prompt, &requestBody, &task.ProviderTaskID, &task.ErrorMessage, &task.CreatedAt, &task.FinishedAt); err != nil {
			return nil, 0, fmt.Errorf("scan creation task: %w", err)
		}
		var request map[string]any
		if json.Unmarshal(requestBody, &request) == nil {
			task.Request = request
			task.Model, _ = request["model"].(string)
		}
		assets, assetErr := s.listAssets(ctx, userID, task.ID)
		if assetErr != nil {
			return nil, 0, assetErr
		}
		task.Assets = assets
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate creation tasks: %w", err)
	}
	return tasks, total, nil
}

func (s *CreationHistoryService) ListAdminTasks(ctx context.Context, page, pageSize int, filters CreationAdminHistoryFilters) ([]CreationAdminHistoryTask, int64, error) {
	if s == nil || s.db == nil {
		return nil, 0, errors.New("creation history database is not configured")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = creationHistoryPageSize
	}
	if pageSize > creationHistoryMaxPageSize {
		pageSize = creationHistoryMaxPageSize
	}
	where, args := creationAdminTaskWhere(filters)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM creation.generation_task AS task
		LEFT JOIN users ON users.id = task.user_id
		LEFT JOIN api_keys ON api_keys.id = task.api_key_id
		LEFT JOIN groups ON groups.id = api_keys.group_id `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count admin creation tasks: %w", err)
	}
	listArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT task.id, task.user_id, COALESCE(users.email, ''), COALESCE(task.api_key_id, 0),
			COALESCE(api_keys.name, ''), COALESCE(groups.id, 0), COALESCE(groups.name, ''),
			task.kind, task.status, task.prompt, task.request,
			COALESCE(task.provider_task_id, ''), COALESCE(task.error_message, ''),
			task.created_at, task.finished_at
		FROM creation.generation_task AS task
		LEFT JOIN users ON users.id = task.user_id
		LEFT JOIN api_keys ON api_keys.id = task.api_key_id
		LEFT JOIN groups ON groups.id = api_keys.group_id
		`+where+` ORDER BY task.created_at DESC, task.id DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2), listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin creation tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]CreationAdminHistoryTask, 0, pageSize)
	for rows.Next() {
		var task CreationAdminHistoryTask
		var requestBody []byte
		if err := rows.Scan(
			&task.ID, &task.UserID, &task.UserEmail, &task.APIKeyID,
			&task.APIKeyName, &task.GroupID, &task.GroupName,
			&task.Kind, &task.Status, &task.Prompt, &requestBody,
			&task.ProviderTaskID, &task.ErrorMessage, &task.CreatedAt, &task.FinishedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan admin creation task: %w", err)
		}
		var request map[string]any
		if json.Unmarshal(requestBody, &request) == nil {
			task.Model, _ = request["model"].(string)
		}
		task.Assets, err = s.listAdminAssets(ctx, task.ID)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate admin creation tasks: %w", err)
	}
	return tasks, total, nil
}

func creationAdminTaskWhere(filters CreationAdminHistoryFilters) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, strings.Replace(clause, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	if search := strings.TrimSpace(filters.Search); search != "" {
		pattern := "%" + search + "%"
		start := len(args) + 1
		for i := 0; i < 6; i++ {
			args = append(args, pattern)
		}
		clauses = append(clauses, `(CAST(task.id AS TEXT) ILIKE $`+strconv.Itoa(start)+` OR users.email ILIKE $`+strconv.Itoa(start+1)+` OR task.prompt ILIKE $`+strconv.Itoa(start+2)+` OR COALESCE(task.request->>'model', '') ILIKE $`+strconv.Itoa(start+3)+` OR api_keys.name ILIKE $`+strconv.Itoa(start+4)+` OR groups.name ILIKE $`+strconv.Itoa(start+5)+`)`)
	}
	if kind := strings.TrimSpace(filters.Kind); kind != "" {
		add("task.kind = ?", kind)
	}
	if model := strings.TrimSpace(filters.Model); model != "" {
		add("task.request->>'model' = ?", model)
	}
	if status := strings.TrimSpace(filters.Status); status != "" {
		statuses := map[string][]string{
			"completed":  {"completed", "succeeded", "success"},
			"failed":     {"failed", "error", "cancelled"},
			"processing": {"processing", "running"},
			"queued":     {"queued", "pending"},
		}
		if values, ok := statuses[status]; ok {
			placeholders := make([]string, 0, len(values))
			for _, value := range values {
				args = append(args, value)
				placeholders = append(placeholders, "$"+strconv.Itoa(len(args)))
			}
			clauses = append(clauses, "task.status IN ("+strings.Join(placeholders, ", ")+")")
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (s *CreationHistoryService) DeleteTask(ctx context.Context, userID, taskID int64) error {
	if s == nil || s.db == nil {
		return errors.New("creation history database is not configured")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT object_key FROM creation.media_asset WHERE task_id = $1 AND user_id = $2`, taskID, userID)
	if err != nil {
		return fmt.Errorf("list creation asset files: %w", err)
	}
	var keys []string
	for rows.Next() {
		var key string
		if scanErr := rows.Scan(&key); scanErr == nil && key != "" {
			keys = append(keys, key)
		}
	}
	rows.Close()
	result, err := s.db.ExecContext(ctx, `DELETE FROM creation.generation_task WHERE id = $1 AND user_id = $2`, taskID, userID)
	if err != nil {
		return fmt.Errorf("delete creation task: %w", err)
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return ErrCreationTaskNotFound
	}
	for _, key := range keys {
		_ = os.Remove(s.safeObjectPath(key))
	}
	return nil
}

func (s *CreationHistoryService) SaveRemoteAsset(ctx context.Context, userID, taskID int64, kind, rawURL string) (*CreationHistoryAsset, error) {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		decoded, mimeType, err := decodeDataURL(rawURL)
		if err != nil {
			return nil, ErrCreationAssetURL
		}
		return s.saveAssetStream(ctx, userID, taskID, kind, mimeType, bytes.NewReader(decoded))
	}
	u, err := validateCreationAssetURL(rawURL)
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Do(newCreationAssetRequest(ctx, u.String()))
	if err != nil {
		return nil, fmt.Errorf("download creation asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download creation asset: upstream returned %s", resp.Status)
	}
	mimeType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if parsed, _, parseErr := mime.ParseMediaType(mimeType); parseErr == nil {
		mimeType = parsed
	}
	return s.saveAssetStream(ctx, userID, taskID, kind, mimeType, resp.Body)
}

func (s *CreationHistoryService) SaveUploadedAsset(ctx context.Context, userID, taskID int64, kind, mimeType string, reader io.Reader) (*CreationHistoryAsset, error) {
	return s.saveAssetStream(ctx, userID, taskID, kind, mimeType, reader)
}

func (s *CreationHistoryService) AssetPath(ctx context.Context, userID, assetID int64) (string, string, error) {
	if s == nil || s.db == nil {
		return "", "", errors.New("creation history database is not configured")
	}
	var objectKey, mimeType string
	err := s.db.QueryRowContext(ctx, `
		SELECT object_key, COALESCE(mime_type, '')
		FROM creation.media_asset
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, assetID, userID).Scan(&objectKey, &mimeType)
	if err == sql.ErrNoRows {
		return "", "", ErrCreationAssetNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("get creation asset: %w", err)
	}
	path := s.safeObjectPath(objectKey)
	if path == "" {
		return "", "", ErrCreationAssetNotFound
	}
	if _, err := os.Stat(path); err != nil {
		return "", "", ErrCreationAssetNotFound
	}
	return path, mimeType, nil
}

func (s *CreationHistoryService) AdminAssetPath(ctx context.Context, assetID int64) (string, string, error) {
	if s == nil || s.db == nil {
		return "", "", errors.New("creation history database is not configured")
	}
	var objectKey, mimeType string
	err := s.db.QueryRowContext(ctx, `
		SELECT object_key, COALESCE(mime_type, '')
		FROM creation.media_asset
		WHERE id = $1 AND deleted_at IS NULL`, assetID).Scan(&objectKey, &mimeType)
	if err == sql.ErrNoRows {
		return "", "", ErrCreationAssetNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("get admin creation asset: %w", err)
	}
	path := s.safeObjectPath(objectKey)
	if path == "" {
		return "", "", ErrCreationAssetNotFound
	}
	if _, err := os.Stat(path); err != nil {
		return "", "", ErrCreationAssetNotFound
	}
	return path, mimeType, nil
}

func (s *CreationHistoryService) getTaskByIdempotencyKey(ctx context.Context, userID, apiKeyID int64, key string) (*CreationHistoryTask, error) {
	var task CreationHistoryTask
	var requestBody []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(api_key_id, 0), kind, status, prompt, request, COALESCE(provider_task_id, ''), COALESCE(error_message, ''), created_at, finished_at
		FROM creation.generation_task
		WHERE user_id = $1 AND api_key_id = $2 AND idempotency_key = $3`, userID, apiKeyID, key).
		Scan(&task.ID, &task.APIKeyID, &task.Kind, &task.Status, &task.Prompt, &requestBody, &task.ProviderTaskID, &task.ErrorMessage, &task.CreatedAt, &task.FinishedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get creation task by idempotency key: %w", err)
	}
	var request map[string]any
	if json.Unmarshal(requestBody, &request) == nil {
		task.Model, _ = request["model"].(string)
	}
	task.Assets, err = s.listAssets(ctx, userID, task.ID)
	return &task, err
}

func (s *CreationHistoryService) listAssets(ctx context.Context, userID, taskID int64) ([]CreationHistoryAsset, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, COALESCE(mime_type, ''), created_at
		FROM creation.media_asset
		WHERE task_id = $1 AND user_id = $2 AND deleted_at IS NULL
		ORDER BY id ASC`, taskID, userID)
	if err != nil {
		return nil, fmt.Errorf("list creation assets: %w", err)
	}
	defer rows.Close()
	assets := make([]CreationHistoryAsset, 0, 1)
	for rows.Next() {
		var asset CreationHistoryAsset
		if err := rows.Scan(&asset.ID, &asset.Kind, &asset.MimeType, &asset.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan creation asset: %w", err)
		}
		// The frontend loads this URL through apiClient, whose base URL already
		// contains /api/v1. Return an API-relative path to avoid duplicating the
		// version prefix after a page refresh.
		asset.ContentURL = "/creation/assets/" + strconv.FormatInt(asset.ID, 10) + "/content"
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (s *CreationHistoryService) listAdminAssets(ctx context.Context, taskID int64) ([]CreationHistoryAsset, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, COALESCE(mime_type, ''), created_at
		FROM creation.media_asset
		WHERE task_id = $1 AND deleted_at IS NULL
		ORDER BY id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list admin creation assets: %w", err)
	}
	defer rows.Close()
	assets := make([]CreationHistoryAsset, 0, 1)
	for rows.Next() {
		var asset CreationHistoryAsset
		if err := rows.Scan(&asset.ID, &asset.Kind, &asset.MimeType, &asset.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan admin creation asset: %w", err)
		}
		asset.ContentURL = "/admin/creation/assets/" + strconv.FormatInt(asset.ID, 10) + "/content"
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (s *CreationHistoryService) saveAssetStream(ctx context.Context, userID, taskID int64, kind, mimeType string, reader io.Reader) (*CreationHistoryAsset, error) {
	if s == nil || s.db == nil || userID <= 0 || taskID <= 0 {
		return nil, ErrCreationTaskNotFound
	}
	var taskUserID int64
	if err := s.db.QueryRowContext(ctx, `SELECT user_id FROM creation.generation_task WHERE id = $1`, taskID).Scan(&taskUserID); err == sql.ErrNoRows || taskUserID != userID {
		return nil, ErrCreationTaskNotFound
	} else if err != nil {
		return nil, fmt.Errorf("check creation task ownership: %w", err)
	}
	if strings.TrimSpace(kind) != "image" && strings.TrimSpace(kind) != "video" {
		return nil, infraerrors.BadRequest("CREATION_ASSET_KIND_INVALID", "asset kind must be image or video")
	}
	if err := os.MkdirAll(filepath.Join(s.mediaRoot, strconv.FormatInt(userID, 10)), 0o750); err != nil {
		return nil, fmt.Errorf("create creation media directory: %w", err)
	}
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("create creation asset id: %w", err)
	}
	key := filepath.Join(strconv.FormatInt(userID, 10), fmt.Sprintf("%x.%s", token, extensionForMime(mimeType, kind)))
	path := s.safeObjectPath(key)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".creation-asset-*")
	if err != nil {
		return nil, fmt.Errorf("create creation asset file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	written, copyErr := copyWithLimit(tmp, reader, creationAssetMaxBytes)
	closeErr := tmp.Close()
	if copyErr != nil {
		if errors.Is(copyErr, ErrCreationAssetTooLarge) {
			return nil, copyErr
		}
		return nil, fmt.Errorf("write creation asset: %w", copyErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close creation asset: %w", closeErr)
	}
	if written == 0 {
		return nil, infraerrors.BadRequest("CREATION_ASSET_EMPTY", "asset is empty")
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return nil, fmt.Errorf("finalize creation asset: %w", err)
	}
	var asset CreationHistoryAsset
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO creation.media_asset (task_id, user_id, kind, mime_type, object_key)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)
		RETURNING id, kind, COALESCE(mime_type, ''), created_at`, taskID, userID, kind, mimeType, key).
		Scan(&asset.ID, &asset.Kind, &asset.MimeType, &asset.CreatedAt)
	if err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("save creation asset metadata: %w", err)
	}
	asset.ContentURL = "/creation/assets/" + strconv.FormatInt(asset.ID, 10) + "/content"
	return &asset, nil
}

func (s *CreationHistoryService) appendEvent(ctx context.Context, taskID int64, eventType, status string, payload map[string]any) error {
	body, _ := json.Marshal(payload)
	_, err := s.db.ExecContext(ctx, `INSERT INTO creation.task_event (task_id, event_type, status, payload) VALUES ($1, $2, NULLIF($3, ''), $4::jsonb)`, taskID, eventType, status, body)
	return err
}

func (s *CreationHistoryService) safeObjectPath(key string) string {
	if strings.TrimSpace(key) == "" {
		return ""
	}
	root, err := filepath.Abs(s.mediaRoot)
	if err != nil {
		return ""
	}
	path, err := filepath.Abs(filepath.Join(s.mediaRoot, key))
	if err != nil || (path != root && !strings.HasPrefix(path, root+string(os.PathSeparator))) {
		return ""
	}
	return path
}

func copyWithLimit(dst io.Writer, src io.Reader, max int64) (int64, error) {
	limited := io.LimitReader(src, max+1)
	n, err := io.Copy(dst, limited)
	if n > max {
		return n, ErrCreationAssetTooLarge
	}
	return n, err
}

func validateCreationAssetURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, ErrCreationAssetURL
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateCreationIP(ip) {
			return nil, ErrCreationAssetURL
		}
		return u, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil, ErrCreationAssetURL
	}
	for _, ip := range ips {
		if isPrivateCreationIP(ip) {
			return nil, ErrCreationAssetURL
		}
	}
	return u, nil
}

func isPrivateCreationIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()
}

func newCreationAssetRequest(ctx context.Context, rawURL string) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	req.Header.Set("User-Agent", "sub2api-creation-media/1.0")
	return req
}

func decodeDataURL(raw string) ([]byte, string, error) {
	parts := strings.SplitN(raw, ",", 2)
	if len(parts) != 2 || !strings.HasPrefix(strings.ToLower(parts[0]), "data:") {
		return nil, "", ErrCreationAssetURL
	}
	meta := strings.TrimPrefix(strings.ToLower(parts[0]), "data:")
	mimeType := "application/octet-stream"
	if semi := strings.IndexByte(meta, ';'); semi >= 0 {
		if meta[:semi] != "" {
			mimeType = meta[:semi]
		}
		meta = meta[semi+1:]
	} else if meta != "" {
		mimeType = meta
	}
	if meta == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		return decoded, mimeType, err
	}
	return []byte(parts[1]), mimeType, nil
}

func extensionForMime(mimeType, kind string) string {
	if strings.HasPrefix(mimeType, "video/") || kind == "video" {
		if strings.Contains(mimeType, "webm") {
			return "webm"
		}
		return "mp4"
	}
	if strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg") {
		return "jpg"
	}
	if strings.Contains(mimeType, "webp") {
		return "webp"
	}
	return "png"
}

func isCreationTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "succeeded", "success", "failed", "cancelled", "error":
		return true
	default:
		return false
	}
}
