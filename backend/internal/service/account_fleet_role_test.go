package service

import (
	"testing"
	"time"
)

func TestAccountFleetRole(t *testing.T) {
	pro := &Account{Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "pro"}}
	if !IsReserveFleetAccount(pro) {
		t.Fatal("plan_type=pro should be reserve")
	}
	team := &Account{Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "team"}}
	if IsReserveFleetAccount(team) {
		t.Fatal("team should be primary")
	}
	forced := &Account{
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"plan_type": "pro"},
		Extra:       map[string]any{ExtraFleetRoleKey: FleetRolePrimary},
	}
	if IsReserveFleetAccount(forced) {
		t.Fatal("explicit primary overrides pro")
	}
	tagged := &Account{Type: AccountTypeOAuth, Extra: map[string]any{ExtraFleetRoleKey: FleetRoleReserve}}
	if !IsReserveFleetAccount(tagged) {
		t.Fatal("explicit reserve should win")
	}
}

func TestOpenAICodexQuotaExhausted(t *testing.T) {
	now := time.Now()
	full := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
		},
	}
	if !openAICodexQuotaExhausted(full, now) {
		t.Fatal("7d 100% should be exhausted")
	}
	teamKey := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: full.Extra}
	if openAICodexQuotaExhausted(teamKey, now) {
		t.Fatal("api key should not use codex exhaustion")
	}
}

func TestPreferPrimaryFleetAccounts(t *testing.T) {
	donna := &Account{ID: 1, Credentials: map[string]any{"plan_type": "team"}}
	zbj := &Account{ID: 2, Credentials: map[string]any{"plan_type": "pro"}}
	got := preferPrimaryFleetAccounts([]*Account{zbj, donna})
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("got %+v, want donna only", got)
	}
	got = preferPrimaryFleetAccounts([]*Account{zbj})
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("reserve-only = %+v", got)
	}
}
