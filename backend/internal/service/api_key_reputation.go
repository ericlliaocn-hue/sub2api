package service

import (
	"context"
	"time"
)

// Reputation sanction levels, stored on api_key_reputation.sanction.
const (
	ReputationSanctionNone     = "none"
	ReputationSanctionDemoted  = "demoted"
	ReputationSanctionDisabled = "disabled"
)

// reputationMaxScore is the score of a key with no hits in the window.
const reputationMaxScore = 100

// APIKeyReputation is the materialised standing of one key.
type APIKeyReputation struct {
	APIKeyID       int64      `json:"api_key_id"`
	Score          int        `json:"score"`
	SevereHits     int        `json:"severe_hits"`
	TotalHits      int        `json:"total_hits"`
	LastEventAt    *time.Time `json:"last_event_at"`
	ScoredAt       time.Time  `json:"scored_at"`
	Sanction       string     `json:"sanction"`
	SanctionedAt   *time.Time `json:"sanctioned_at"`
	SanctionReason *string    `json:"sanction_reason"`
}

// ReputationPenalty is one key's aggregated misconduct over the scoring window,
// as computed by the database.
type ReputationPenalty struct {
	APIKeyID    int64
	Penalty     int
	SevereHits  int
	TotalHits   int
	LastEventAt *time.Time
}

// APIKeyReputationRepository persists derived reputation.
//
// The scoring aggregate deliberately lives in SQL: the alternative is pulling
// every moderation event of every key into Go once per sweep.
type APIKeyReputationRepository interface {
	// AggregatePenalties returns the weighted, time-decayed penalty of every key
	// with at least one qualifying hit inside the window.
	AggregatePenalties(ctx context.Context, since time.Time, window time.Duration) ([]ReputationPenalty, error)
	// Upsert writes the recomputed score, leaving the sanction columns alone.
	Upsert(ctx context.Context, rep *APIKeyReputation) error
	// Get returns one key's reputation, or nil when it has never been scored.
	Get(ctx context.Context, apiKeyID int64) (*APIKeyReputation, error)
	// ListWorst returns the lowest-scoring keys for the admin console.
	ListWorst(ctx context.Context, limit int) ([]APIKeyReputation, error)
	// MarkSanctioned records that a sanction was applied, so the next sweep does
	// not apply it again.
	MarkSanctioned(ctx context.Context, apiKeyID int64, sanction, reason string) error
	// ClearSanction resets a key after an operator overturns the decision.
	ClearSanction(ctx context.Context, apiKeyID int64) error
	// ResetScoresNotIn restores keys that have aged out of the window back to a
	// clean score, so a single old incident does not follow a key forever.
	ResetScoresNotIn(ctx context.Context, keepAPIKeyIDs []int64) error
}
