package admin

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SubPoolHandler manages the internal isolation pools under a group.
type SubPoolHandler struct {
	subPoolService *service.SubPoolService
	graduation     *service.SubPoolGraduationService
	settingService *service.SettingService
}

func NewSubPoolHandler(
	subPoolService *service.SubPoolService,
	graduation *service.SubPoolGraduationService,
	settingService *service.SettingService,
) *SubPoolHandler {
	return &SubPoolHandler{
		subPoolService: subPoolService,
		graduation:     graduation,
		settingService: settingService,
	}
}

// --- Request/Response DTOs ---

type CreateSubPoolRequest struct {
	Name         string  `json:"name" binding:"required,min=1,max=100"`
	Description  *string `json:"description"`
	Kind         string  `json:"kind"`
	Status       string  `json:"status"`
	KeySoftLimit *int    `json:"key_soft_limit"`
	SortOrder    int     `json:"sort_order"`
	AccountIDs   []int64 `json:"account_ids"`
}

type UpdateSubPoolRequest struct {
	Name          string     `json:"name" binding:"required,min=1,max=100"`
	Description   *string    `json:"description"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	KeySoftLimit  *int       `json:"key_soft_limit"`
	SortOrder     int        `json:"sort_order"`
	CoolingUntil  *time.Time `json:"cooling_until"`
	CoolingReason *string    `json:"cooling_reason"`
}

type SetSubPoolAccountsRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

type BindSubPoolKeyRequest struct {
	APIKeyID int64   `json:"api_key_id" binding:"required"`
	Note     *string `json:"note"`
}

type MigrateSubPoolKeysRequest struct {
	// SuspectKeyIDs stay in the burned pool; everyone else is moved out.
	SuspectKeyIDs []int64 `json:"suspect_key_ids"`
	Reason        string  `json:"reason" binding:"required,min=1,max=500"`
}

type SubPoolResponse struct {
	ID            int64      `json:"id"`
	GroupID       int64      `json:"group_id"`
	Name          string     `json:"name"`
	Description   *string    `json:"description"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	KeySoftLimit  int        `json:"key_soft_limit"`
	CoolingUntil  *time.Time `json:"cooling_until"`
	CoolingReason *string    `json:"cooling_reason"`
	SortOrder     int        `json:"sort_order"`
	AccountIDs    []int64    `json:"account_ids"`
	BoundKeys     int        `json:"bound_keys"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type SubPoolAccountKeyUsageResponse struct {
	APIKeyID    int64      `json:"api_key_id"`
	UserID      int64      `json:"user_id"`
	Calls       int64      `json:"calls"`
	FirstCallAt *time.Time `json:"first_call_at"`
	LastCallAt  *time.Time `json:"last_call_at"`
}

func subPoolToResponse(pool *service.SubPool) *SubPoolResponse {
	if pool == nil {
		return nil
	}
	accountIDs := pool.AccountIDs
	if accountIDs == nil {
		accountIDs = []int64{}
	}
	return &SubPoolResponse{
		ID:            pool.ID,
		GroupID:       pool.GroupID,
		Name:          pool.Name,
		Description:   pool.Description,
		Kind:          pool.Kind,
		Status:        pool.Status,
		KeySoftLimit:  pool.KeySoftLimit,
		CoolingUntil:  pool.CoolingUntil,
		CoolingReason: pool.CoolingReason,
		SortOrder:     pool.SortOrder,
		AccountIDs:    accountIDs,
		BoundKeys:     pool.BoundKeys,
		CreatedAt:     pool.CreatedAt,
		UpdatedAt:     pool.UpdatedAt,
	}
}

// --- Handlers ---

// List returns the pools of a group.
// GET /admin/groups/:id/sub-pools
func (h *SubPoolHandler) List(c *gin.Context) {
	groupID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	pools, err := h.subPoolService.ListByGroup(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]*SubPoolResponse, 0, len(pools))
	for i := range pools {
		out = append(out, subPoolToResponse(&pools[i]))
	}
	response.Success(c, out)
}

// Create adds a pool to a group.
// POST /admin/groups/:id/sub-pools
func (h *SubPoolHandler) Create(c *gin.Context) {
	groupID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req CreateSubPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	pool := &service.SubPool{
		GroupID:      groupID,
		Name:         req.Name,
		Description:  req.Description,
		Kind:         req.Kind,
		Status:       req.Status,
		KeySoftLimit: defaultKeySoftLimit(req.KeySoftLimit),
		SortOrder:    req.SortOrder,
	}
	ctx := c.Request.Context()
	if err := h.subPoolService.Create(ctx, pool); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if len(req.AccountIDs) > 0 {
		if err := h.subPoolService.SetAccounts(ctx, pool.ID, req.AccountIDs); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	created, err := h.subPoolService.GetByID(ctx, pool.ID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subPoolToResponse(created))
}

// Update edits pool metadata. Membership is managed through SetAccounts.
// PUT /admin/sub-pools/:pool_id
func (h *SubPoolHandler) Update(c *gin.Context) {
	poolID, ok := parseIDParam(c, "pool_id")
	if !ok {
		return
	}
	var req UpdateSubPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	pool := &service.SubPool{
		ID:            poolID,
		Name:          req.Name,
		Description:   req.Description,
		Kind:          req.Kind,
		Status:        req.Status,
		KeySoftLimit:  defaultKeySoftLimit(req.KeySoftLimit),
		SortOrder:     req.SortOrder,
		CoolingUntil:  req.CoolingUntil,
		CoolingReason: req.CoolingReason,
	}
	if err := h.subPoolService.Update(c.Request.Context(), pool); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	updated, err := h.subPoolService.GetByID(c.Request.Context(), poolID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subPoolToResponse(updated))
}

// Delete removes a pool; its keys fall back to whole-group scheduling.
// DELETE /admin/sub-pools/:pool_id
func (h *SubPoolHandler) Delete(c *gin.Context) {
	poolID, ok := parseIDParam(c, "pool_id")
	if !ok {
		return
	}
	if err := h.subPoolService.Delete(c.Request.Context(), poolID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// SetAccounts replaces the pool's upstream accounts.
// PUT /admin/sub-pools/:pool_id/accounts
func (h *SubPoolHandler) SetAccounts(c *gin.Context) {
	poolID, ok := parseIDParam(c, "pool_id")
	if !ok {
		return
	}
	var req SetSubPoolAccountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.subPoolService.SetAccounts(c.Request.Context(), poolID, req.AccountIDs); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	updated, err := h.subPoolService.GetByID(c.Request.Context(), poolID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subPoolToResponse(updated))
}

// BindKey moves one API key into this pool.
// POST /admin/sub-pools/:pool_id/keys
func (h *SubPoolHandler) BindKey(c *gin.Context) {
	poolID, ok := parseIDParam(c, "pool_id")
	if !ok {
		return
	}
	var req BindSubPoolKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	operator := subPoolAdminOperator(c)
	if err := h.subPoolService.BindKey(c.Request.Context(), req.APIKeyID, poolID, operator, req.Note); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"bound": true})
}

// MigrateCleanKeys drains a burned pool, leaving the suspected keys behind.
// POST /admin/sub-pools/:pool_id/migrate
func (h *SubPoolHandler) MigrateCleanKeys(c *gin.Context) {
	poolID, ok := parseIDParam(c, "pool_id")
	if !ok {
		return
	}
	var req MigrateSubPoolKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	operator := subPoolAdminOperator(c)
	moved, err := h.subPoolService.MigrateCleanKeys(c.Request.Context(), poolID, req.SuspectKeyIDs, operator, req.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"moved": moved})
}

// TopKeysByAccount attributes an account's traffic to the keys behind it, which
// is the report an admin opens right after an upstream account gets flagged.
// GET /admin/accounts/:id/top-keys?hours=24&limit=20
func (h *SubPoolHandler) TopKeysByAccount(c *gin.Context) {
	accountID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	hours := 24
	if raw := c.Query("hours"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 24*30 {
			response.BadRequest(c, "hours must be between 1 and 720")
			return
		}
		hours = parsed
	}
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 200 {
			response.BadRequest(c, "limit must be between 1 and 200")
			return
		}
		limit = parsed
	}

	end := timezone.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)
	items, err := h.subPoolService.TopKeysByAccount(c.Request.Context(), accountID, start, end, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]SubPoolAccountKeyUsageResponse, 0, len(items))
	for _, item := range items {
		out = append(out, SubPoolAccountKeyUsageResponse{
			APIKeyID:    item.APIKeyID,
			UserID:      item.UserID,
			Calls:       item.Calls,
			FirstCallAt: item.FirstCallAt,
			LastCallAt:  item.LastCallAt,
		})
	}
	response.Success(c, gin.H{
		"account_id": accountID,
		"start":      start,
		"end":        end,
		"items":      out,
	})
}

func defaultKeySoftLimit(v *int) int {
	if v == nil {
		return domain.SubPoolDefaultKeySoftLimit
	}
	return *v
}

// subPoolAdminOperator records who moved a key. An empty subject falls back to
// "system" so the history never loses a row for want of an actor.
func subPoolAdminOperator(c *gin.Context) string {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		return domain.SubPoolBindOperatorSystem
	}
	return service.SubPoolAdminOperator(subject.UserID)
}

// --- Probation / graduation ---

type subPoolGraduationPolicyRequest struct {
	Enabled       bool `json:"enabled"`
	ProbationDays int  `json:"probation_days" binding:"min=0,max=365"`
	MaxDailyCalls int  `json:"max_daily_calls" binding:"min=0"`
}

type subPoolGraduationPolicyResponse struct {
	Enabled       bool `json:"enabled"`
	ProbationDays int  `json:"probation_days"`
	MaxDailyCalls int  `json:"max_daily_calls"`
}

// GetGraduationPolicy returns the probation rules for probe pools.
func (h *SubPoolHandler) GetGraduationPolicy(c *gin.Context) {
	policy := h.settingService.GetSubPoolGraduationPolicy(c.Request.Context())
	response.Success(c, subPoolGraduationPolicyResponse{
		Enabled:       policy.Enabled,
		ProbationDays: policy.ProbationDays,
		MaxDailyCalls: policy.MaxDailyCalls,
	})
}

// UpdateGraduationPolicy persists the probation rules.
func (h *SubPoolHandler) UpdateGraduationPolicy(c *gin.Context) {
	var req subPoolGraduationPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	policy := service.SubPoolGraduationPolicy{
		Enabled:       req.Enabled,
		ProbationDays: req.ProbationDays,
		MaxDailyCalls: req.MaxDailyCalls,
	}
	if err := h.settingService.SetSubPoolGraduationPolicy(c.Request.Context(), policy); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subPoolGraduationPolicyResponse{
		Enabled:       policy.Enabled,
		ProbationDays: policy.ProbationDays,
		MaxDailyCalls: policy.MaxDailyCalls,
	})
}

// RunGraduation triggers one sweep immediately instead of waiting for the
// ticker, so an admin can see the effect of a settings change right away.
func (h *SubPoolHandler) RunGraduation(c *gin.Context) {
	graduated := h.graduation.RunOnce(c.Request.Context())
	response.Success(c, gin.H{"graduated": graduated})
}
