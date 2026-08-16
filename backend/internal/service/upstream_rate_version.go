package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

var (
	ErrUpstreamRateVersionInvalidAccountID    = errors.New("upstream rate version account id must be positive")
	ErrUpstreamRateVersionInvalidMultiplier   = errors.New("upstream rate version multiplier must be finite and non-negative")
	ErrUpstreamRateVersionInvalidSource       = errors.New("upstream rate version source is invalid")
	ErrUpstreamRateVersionInvalidChangeReason = errors.New("upstream rate version change reason is required")
	ErrUpstreamRateVersionChangeReasonTooLong = errors.New("upstream rate version change reason is too long")
	ErrUpstreamRateVersionEffectiveFromOrder  = errors.New("upstream rate version effective_from must be after current version")
)

// UpstreamRateVersion is the immutable account-level multiplier version used
// to explain how a request-time account rate was selected.
type UpstreamRateVersion struct {
	ID             int64
	AccountID      int64
	VersionNo      int64
	RateMultiplier float64
	Source         domain.UpstreamRateVersionSource
	EffectiveFrom  time.Time
	EffectiveTo    *time.Time
	ObservedAt     *time.Time
	Snapshot       map[string]any
	ChangeReason   domain.UpstreamRateVersionChangeReason
	CreatedBy      *int64
	CreatedAt      time.Time
}

// UpstreamRateVersionChange is the complete input to the single version
// transition transaction. A nil Snapshot/ObservedAt means that an unchanged
// version should retain its existing observation data.
type UpstreamRateVersionChange struct {
	AccountID      int64
	RateMultiplier float64
	Source         domain.UpstreamRateVersionSource
	EffectiveFrom  time.Time
	ObservedAt     *time.Time
	Snapshot       map[string]any
	ChangeReason   domain.UpstreamRateVersionChangeReason
	CreatedBy      *int64

	// OutboxPayload lets an account edit preserve its existing scheduler group
	// payload while keeping the rate transition and outbox write atomic.
	OutboxPayload any

	// SkipOutbox suppresses the scheduler outbox write. Account creation
	// enqueues its own AccountChanged event after the whole create
	// transaction commits.
	SkipOutbox bool
}

func (c UpstreamRateVersionChange) Validate() error {
	if c.AccountID <= 0 {
		return ErrUpstreamRateVersionInvalidAccountID
	}
	if math.IsNaN(c.RateMultiplier) || math.IsInf(c.RateMultiplier, 0) || c.RateMultiplier < 0 {
		return ErrUpstreamRateVersionInvalidMultiplier
	}
	if !c.Source.Valid() {
		return ErrUpstreamRateVersionInvalidSource
	}
	if c.ChangeReason == "" {
		return ErrUpstreamRateVersionInvalidChangeReason
	}
	if len(c.ChangeReason) > 64 {
		return ErrUpstreamRateVersionChangeReasonTooLong
	}
	return nil
}

// UpstreamRateVersionChangeResult reports whether the transaction created a
// new version. When Changed is false, only the current version's observation
// snapshot may have been refreshed.
type UpstreamRateVersionChangeResult struct {
	Version *UpstreamRateVersion
	Changed bool
}

// UpstreamRateVersionRepository is intentionally separate from
// AccountRepository so existing read-only account test doubles do not need to
// implement Phase 2 write behavior.
type UpstreamRateVersionRepository interface {
	ApplyUpstreamRateVersionChange(context.Context, UpstreamRateVersionChange) (*UpstreamRateVersionChangeResult, error)
}
