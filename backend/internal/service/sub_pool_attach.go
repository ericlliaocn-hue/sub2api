package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	// AttachSubPoolsBoth hangs a newly created account on the group's formal
	// pool and its observation/probe pool. This is the import-time shortcut
	// operators use when a channel should serve both stacks.
	AttachSubPoolsBoth = "both"
	// AttachSubPoolsFormal hangs the account only on the formal pool.
	AttachSubPoolsFormal = "formal"
)

// NormalizeAttachSubPools accepts the create-account field. Empty / "none"
// means do nothing; anything else must be both or formal.
func NormalizeAttachSubPools(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "none":
		return "", nil
	case AttachSubPoolsBoth, AttachSubPoolsFormal:
		return strings.ToLower(strings.TrimSpace(mode)), nil
	default:
		return "", infraerrors.BadRequest("ATTACH_SUB_POOLS_INVALID", "attach_sub_pools must be both or formal")
	}
}

func isObservationPool(pool SubPool) bool {
	if strings.EqualFold(pool.Kind, domain.SubPoolKindProbe) {
		return true
	}
	return strings.Contains(pool.Name, "观察")
}

func isClosedPool(pool SubPool) bool {
	return pool.Status == domain.SubPoolStatusClosed
}

func preferFormalName(name string) bool {
	return strings.Contains(name, "正池") || strings.Contains(name, "正式池")
}

func betterPool(current *SubPool, candidate SubPool) bool {
	if current == nil {
		return true
	}
	if candidate.SortOrder != current.SortOrder {
		return candidate.SortOrder < current.SortOrder
	}
	return candidate.ID < current.ID
}

// PickFormalPool returns the group's live formal pool. Isolation leftovers
// (观察 / 专车 / 分流 / probe) never receive automatic attaches even when
// kind is wrongly stored as formal.
func PickFormalPool(pools []SubPool) *SubPool {
	var named *SubPool
	var fallback *SubPool
	for i := range pools {
		pool := pools[i]
		if isClosedPool(pool) || isSidePool(&pool) {
			continue
		}
		if pool.Kind != "" && pool.Kind != domain.SubPoolKindFormal {
			continue
		}
		if preferFormalName(pool.Name) && betterPool(named, pool) {
			named = &pools[i]
			continue
		}
		if betterPool(fallback, pool) {
			fallback = &pools[i]
		}
	}
	if named != nil {
		return named
	}
	return fallback
}

// PickObservationPool returns the group's live observation/probe pool.
func PickObservationPool(pools []SubPool) *SubPool {
	var best *SubPool
	for i := range pools {
		pool := pools[i]
		if !isObservationPool(pool) || isClosedPool(pool) {
			continue
		}
		if betterPool(best, pool) {
			best = &pools[i]
		}
	}
	return best
}

// AttachAccountOnCreate appends the new account to the selected stacks of each
// group that already has sub-pool scheduling on. Groups without pools are
// skipped so ordinary imports stay unchanged.
func (s *SubPoolService) AttachAccountOnCreate(ctx context.Context, accountID int64, groupIDs []int64, mode string) error {
	if s == nil || accountID <= 0 {
		return nil
	}
	normalized, err := NormalizeAttachSubPools(mode)
	if err != nil {
		return err
	}
	if normalized == "" {
		return nil
	}
	seen := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupID <= 0 {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		if err := s.attachAccountToGroup(ctx, accountID, groupID, normalized); err != nil {
			return err
		}
	}
	return nil
}

func (s *SubPoolService) attachAccountToGroup(ctx context.Context, accountID, groupID int64, mode string) error {
	group, err := s.groupRepo.GetByIDLite(ctx, groupID)
	if err != nil {
		return err
	}
	if group == nil || !group.SubPoolEnabled {
		return nil
	}
	pools, err := s.repo.ListByGroup(ctx, groupID)
	if err != nil {
		return err
	}
	targets := make([]*SubPool, 0, 2)
	if formal := PickFormalPool(pools); formal != nil {
		targets = append(targets, formal)
	}
	if mode == AttachSubPoolsBoth {
		if observation := PickObservationPool(pools); observation != nil {
			targets = append(targets, observation)
		}
	}
	if len(targets) == 0 {
		return infraerrors.BadRequest(
			"SUB_POOL_ATTACH_TARGET_MISSING",
			fmt.Sprintf("group %d has sub-pools enabled but no formal/observation pool to attach", groupID),
		)
	}
	for _, pool := range targets {
		if containsInt64(pool.AccountIDs, accountID) {
			continue
		}
		next := append(append([]int64(nil), pool.AccountIDs...), accountID)
		if err := s.SetAccounts(ctx, pool.ID, next); err != nil {
			return err
		}
	}
	return nil
}
