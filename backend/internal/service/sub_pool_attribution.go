package service

import (
	"context"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	// subPoolAttributionWindow is how far back the "who burned this pool" report
	// looks by default.
	subPoolAttributionWindow = 24 * time.Hour

	// subPoolSuspectShareThreshold marks a key as suspect when it drove this much
	// of the pool's traffic. One key out of a pool of eight owning more than half
	// the calls is not a coincidence.
	subPoolSuspectShareThreshold = 0.5
	// subPoolSuspectMinCalls stops the share rule from firing on a quiet pool,
	// where 3 calls out of 4 is 75% and means nothing.
	subPoolSuspectMinCalls = 50
)

// SubPoolKeyAttribution is one row of the incident report: what a key did in the
// window and whether that makes it a suspect.
type SubPoolKeyAttribution struct {
	APIKeyID    int64      `json:"api_key_id"`
	UserID      int64      `json:"user_id"`
	Calls       int64      `json:"calls"`
	Share       float64    `json:"share"`
	Violations  int64      `json:"violations"`
	Suspect     bool       `json:"suspect"`
	SuspectWhy  string     `json:"suspect_why,omitempty"`
	FirstCallAt *time.Time `json:"first_call_at"`
	LastCallAt  *time.Time `json:"last_call_at"`
	BoundAt     *time.Time `json:"bound_at"`
}

// SubPoolAttributionReport answers "who burned this pool" for one time window.
type SubPoolAttributionReport struct {
	SubPoolID  int64                   `json:"sub_pool_id"`
	Status     string                  `json:"status"`
	Start      time.Time               `json:"start"`
	End        time.Time               `json:"end"`
	TotalCalls int64                   `json:"total_calls"`
	Keys       []SubPoolKeyAttribution `json:"keys"`
}

// SuspectKeyIDs returns the keys that should stay behind when the pool drains.
func (r *SubPoolAttributionReport) SuspectKeyIDs() []int64 {
	if r == nil {
		return nil
	}
	out := make([]int64, 0, len(r.Keys))
	for _, key := range r.Keys {
		if key.Suspect {
			out = append(out, key.APIKeyID)
		}
	}
	return out
}

// Attribution builds the incident report for a pool over the given window.
//
// The report is the evidence half of the product decision that catching the
// culprit matters more than avoiding collateral damage: it names a small set of
// suspects so the rest of the pool can be moved and compensated.
func (s *SubPoolService) Attribution(ctx context.Context, subPoolID int64, window time.Duration) (*SubPoolAttributionReport, error) {
	if window <= 0 {
		window = subPoolAttributionWindow
	}
	pool, err := s.repo.GetByID(ctx, subPoolID)
	if err != nil {
		return nil, err
	}
	keyIDs, err := s.repo.ListKeyIDs(ctx, subPoolID)
	if err != nil {
		return nil, err
	}

	end := timezone.Now()
	start := end.Add(-window)
	report := &SubPoolAttributionReport{
		SubPoolID: subPoolID,
		Status:    pool.Status,
		Start:     start,
		End:       end,
		Keys:      make([]SubPoolKeyAttribution, 0, len(keyIDs)),
	}
	if len(keyIDs) == 0 {
		return report, nil
	}

	usage, err := s.usageRepo.KeyUsageInWindow(ctx, keyIDs, start, end)
	if err != nil {
		return nil, err
	}
	for _, stat := range usage {
		report.TotalCalls += stat.Calls
	}

	for _, keyID := range keyIDs {
		stat := usage[keyID]
		row := SubPoolKeyAttribution{
			APIKeyID:    keyID,
			UserID:      stat.UserID,
			Calls:       stat.Calls,
			FirstCallAt: stat.FirstCallAt,
			LastCallAt:  stat.LastCallAt,
		}
		if report.TotalCalls > 0 {
			row.Share = float64(stat.Calls) / float64(report.TotalCalls)
		}

		// Violations are counted from the key's own binding start, not the report
		// window: a hit from three days ago still belongs to this key's tenure.
		since := start
		if binding, err := s.repo.GetOpenBinding(ctx, keyID); err == nil && binding != nil {
			row.BoundAt = &binding.BoundAt
			if binding.BoundAt.Before(since) {
				since = binding.BoundAt
			}
		}
		if violations, err := s.usageRepo.CountViolationsSince(ctx, keyID, since); err == nil {
			row.Violations = violations
		}

		row.Suspect, row.SuspectWhy = classifySuspect(row, report.TotalCalls)
		report.Keys = append(report.Keys, row)
	}

	sort.SliceStable(report.Keys, func(i, j int) bool {
		if report.Keys[i].Suspect != report.Keys[j].Suspect {
			return report.Keys[i].Suspect
		}
		return report.Keys[i].Calls > report.Keys[j].Calls
	})
	return report, nil
}

// classifySuspect encodes the two signals we are willing to act on. Both are
// deliberately blunt: this decides who gets held back for review, not who gets
// banned, and a human still makes the second call.
func classifySuspect(row SubPoolKeyAttribution, totalCalls int64) (bool, string) {
	if row.Violations > 0 {
		return true, "violations"
	}
	if totalCalls >= subPoolSuspectMinCalls && row.Share >= subPoolSuspectShareThreshold {
		return true, "traffic_share"
	}
	return false, ""
}
