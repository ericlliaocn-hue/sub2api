package service

import "testing"

func TestSettingKeyAllowUserViewErrorRequests_Constant(t *testing.T) {
	if SettingKeyAllowUserViewErrorRequests != "allow_user_view_error_requests" {
		t.Fatalf("unexpected key: %s", SettingKeyAllowUserViewErrorRequests)
	}
}

func TestSettingKeyAllowUserViewUsagePressure_Constant(t *testing.T) {
	if SettingKeyAllowUserViewUsagePressure != "allow_user_view_usage_pressure" {
		t.Fatalf("unexpected key: %s", SettingKeyAllowUserViewUsagePressure)
	}
}
