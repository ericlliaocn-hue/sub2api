package service

import (
	"strings"
	"time"
)

const (
	// ExtraFleetRoleKey lets operators pin a car as last-resort 20x or as a
	// normal primary. When unset, ChatGPT plan_type=pro is treated as reserve.
	ExtraFleetRoleKey = "fleet_role"

	FleetRolePrimary = "primary"
	FleetRoleReserve = "reserve"
)

// AccountFleetRole reports whether this car is normal traffic or 20x backup.
func AccountFleetRole(account *Account) string {
	if account == nil {
		return FleetRolePrimary
	}
	switch strings.ToLower(strings.TrimSpace(account.GetExtraString(ExtraFleetRoleKey))) {
	case FleetRolePrimary:
		return FleetRolePrimary
	case FleetRoleReserve:
		return FleetRoleReserve
	}
	plan := strings.ToLower(strings.TrimSpace(account.GetCredential("plan_type")))
	if plan == "" {
		plan = strings.ToLower(strings.TrimSpace(account.GetExtraString("chatgpt_plan_type")))
	}
	if plan == "pro" {
		return FleetRoleReserve
	}
	return FleetRolePrimary
}

func IsReserveFleetAccount(account *Account) bool {
	return AccountFleetRole(account) == FleetRoleReserve
}

func partitionReserveFleetAccounts(accounts []*Account) (primary, reserve []*Account) {
	primary = make([]*Account, 0, len(accounts))
	reserve = make([]*Account, 0, len(accounts))
	for _, account := range accounts {
		if account == nil {
			continue
		}
		if IsReserveFleetAccount(account) {
			reserve = append(reserve, account)
			continue
		}
		primary = append(primary, account)
	}
	return primary, reserve
}

func preferPrimaryFleetAccounts(accounts []*Account) []*Account {
	primary, reserve := partitionReserveFleetAccounts(accounts)
	if len(primary) > 0 {
		return primary
	}
	return reserve
}

// openAICodexQuotaExhausted is the hard floor: a fresh 5h/7d window at 100%
// is not schedulable, even when the admin auto-pause threshold is disabled.
func openAICodexQuotaExhausted(account *Account, now time.Time) bool {
	if account == nil || !account.IsOpenAIOAuth() {
		return false
	}
	for _, candidate := range openAIThresholdCandidates(account, now) {
		if candidate != nil && candidate.usedPercent >= 100 {
			return true
		}
	}
	return false
}

func reserveAuthDead(account *Account) bool {
	if account == nil {
		return true
	}
	msg := strings.ToLower(account.ErrorMessage)
	return strings.Contains(msg, "revoked") || strings.Contains(msg, "invalidated oauth")
}
