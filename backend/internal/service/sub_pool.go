package service

import (
	"context"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSubPoolNotFound = infraerrors.NotFound("SUB_POOL_NOT_FOUND", "sub-pool not found")
	ErrSubPoolExists   = infraerrors.Conflict("SUB_POOL_EXISTS", "sub-pool name already exists in this group")
	// ErrSubPoolAccountTaken guards the "one sub-pool per account per group"
	// invariant: an upstream account may only back a single pool inside a group,
	// otherwise two pools would share a blast radius and attribution would blur.
	ErrSubPoolAccountTaken  = infraerrors.Conflict("SUB_POOL_ACCOUNT_TAKEN", "account already belongs to another sub-pool in this group")
	ErrSubPoolGroupMismatch = infraerrors.BadRequest("SUB_POOL_GROUP_MISMATCH", "sub-pool does not belong to the API key's group")
)

// SubPool is an internal isolation unit under a user-visible group. Users still
// see one group and one rate multiplier; the pool decides which upstream
// accounts their key may reach and who shares them.
type SubPool struct {
	ID            int64
	GroupID       int64
	Name          string
	Description   *string
	Kind          string
	Status        string
	KeySoftLimit  int
	CoolingUntil  *time.Time
	CoolingReason *string
	SortOrder     int
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Populated by list/detail queries, not stored on the row itself.
	AccountIDs []int64
	BoundKeys  int
}

// IsProbe reports whether the pool holds disposable accounts used for the
// observation period of freshly created keys.
func (p *SubPool) IsProbe() bool {
	return p != nil && p.Kind == domain.SubPoolKindProbe
}

// AcceptsNewBindings reports whether the pool may receive additional keys.
// Cooling pools are being drained and closed pools only serve existing keys.
func (p *SubPool) AcceptsNewBindings() bool {
	if p == nil || p.Status != domain.SubPoolStatusHealthy {
		return false
	}
	return !p.IsFull()
}

// IsFull reports whether the soft cap on bound keys is reached. A limit of 0
// means unlimited.
func (p *SubPool) IsFull() bool {
	if p == nil || p.KeySoftLimit <= 0 {
		return false
	}
	return p.BoundKeys >= p.KeySoftLimit
}

// SubPoolAccount is one upstream account attached to a pool.
type SubPoolAccount struct {
	SubPoolID int64
	AccountID int64
	GroupID   int64
	Role      string
	CreatedAt time.Time
}

// SubPoolBinding is one row of the append-only binding history. The open row
// (UnboundAt == nil) mirrors api_keys.sub_pool_id.
type SubPoolBinding struct {
	ID        int64
	APIKeyID  int64
	SubPoolID int64
	GroupID   int64
	BoundAt   time.Time
	UnboundAt *time.Time
	Reason    string
	Operator  string
	Note      *string
}

// SubPoolBindInput describes a binding transition. Reason and Operator are
// mandatory because the history exists for incident attribution: a row without
// a stated cause is not evidence.
type SubPoolBindInput struct {
	APIKeyID  int64
	SubPoolID int64
	GroupID   int64
	Reason    string
	Operator  string
	Note      *string
}

// SubPoolAdminOperator formats the operator column for an admin-initiated move.
func SubPoolAdminOperator(adminUserID int64) string {
	return domain.SubPoolBindOperatorAdminPrefix + strconv.FormatInt(adminUserID, 10)
}

// SubPoolRepository persists pools, their account membership and the key
// binding history.
type SubPoolRepository interface {
	Create(ctx context.Context, pool *SubPool) error
	GetByID(ctx context.Context, id int64) (*SubPool, error)
	Update(ctx context.Context, pool *SubPool) error
	Delete(ctx context.Context, id int64) error

	// ListByGroup returns the pools of a group ordered by sort_order then id,
	// with BoundKeys and AccountIDs populated.
	ListByGroup(ctx context.Context, groupID int64) ([]SubPool, error)
	ExistsByName(ctx context.Context, groupID int64, name string, excludeID int64) (bool, error)

	// SetAccounts replaces the pool's account membership atomically.
	SetAccounts(ctx context.Context, subPoolID int64, accountIDs []int64) error
	// ListAccountIDs returns the accounts backing a pool.
	ListAccountIDs(ctx context.Context, subPoolID int64) ([]int64, error)

	// BindKey moves a key into a pool: it closes the open history row, opens a
	// new one and updates api_keys.sub_pool_id in a single transaction.
	BindKey(ctx context.Context, in SubPoolBindInput) error
	// UnbindKey clears the binding, e.g. when the pool is deleted.
	UnbindKey(ctx context.Context, apiKeyID int64, reason, operator string) error
	// ListKeyIDs returns the API keys currently bound to a pool.
	ListKeyIDs(ctx context.Context, subPoolID int64) ([]int64, error)
	// ListBindingHistory returns the most recent bindings of a key, newest first.
	ListBindingHistory(ctx context.Context, apiKeyID int64, limit int) ([]SubPoolBinding, error)
	// GetOpenBinding returns the currently open binding row of a key, if any.
	GetOpenBinding(ctx context.Context, apiKeyID int64) (*SubPoolBinding, error)
}
