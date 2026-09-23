package usagestats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClassifyUsagePressure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		users  int64
		billed float64
		peak   float64
		want   string
	}{
		{name: "empty is idle", want: PressureLevelIdle},
		{name: "quiet vs tall peak", users: 2, billed: 1, peak: 80, want: PressureLevelIdle},
		{name: "warm vs peak", users: 6, billed: 4.21, peak: 50, want: PressureLevelWarm},
		{name: "busy vs peak", users: 8, billed: 8, peak: 50, want: PressureLevelBusy},
		{name: "scramble above peak", users: 10, billed: 40, peak: 80, want: PressureLevelScramble},
		{name: "early morning one user", users: 1, billed: 0.5, want: PressureLevelIdle},
		{name: "early morning warm", users: 3, billed: 2, want: PressureLevelWarm},
		{name: "early morning busy", users: 7, billed: 2, want: PressureLevelBusy},
		{name: "early morning scramble by burn", users: 4, billed: 12, want: PressureLevelScramble},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, ClassifyUsagePressure(tc.users, tc.billed, tc.peak))
		})
	}
}

func TestFinalizeUsagePressure(t *testing.T) {
	t.Parallel()

	snap := &UsagePressureSnapshot{
		Windows: UsagePressureWindows{
			M15: UsagePressureWindow{Users: 6, Billed: 4.21},
		},
		PeakHour: &UsagePressurePeakHour{Billed: 50, Hour: time.Date(2026, 9, 20, 14, 0, 0, 0, time.UTC)},
	}
	FinalizeUsagePressure(snap)

	require.InDelta(t, 16.84, snap.HourlyFrom15m, 0.0001)
	require.InDelta(t, 16.84/50, snap.PeakRatio, 0.0001)
	require.Equal(t, PressureLevelWarm, snap.Level)
	require.NotNil(t, snap.Users)
	require.NotNil(t, snap.Accounts)
}

func TestRedactUsagePressureForUser(t *testing.T) {
	t.Parallel()

	snap := &UsagePressureSnapshot{
		Level: PressureLevelWarm,
		Windows: UsagePressureWindows{
			M15: UsagePressureWindow{Users: 6, Accounts: 2, Billed: 4.21},
		},
		Users:    []UsagePressureActor{{ID: 163, Name: "eric@test.com"}},
		Accounts: []UsagePressureActor{{ID: 66274, Name: "linda"}},
	}
	got := RedactUsagePressureForUser(snap)
	require.Equal(t, PressureLevelWarm, got.Level)
	require.Equal(t, int64(6), got.Windows.M15.Users)
	require.Empty(t, got.Users)
	require.Empty(t, got.Accounts)
	require.Equal(t, "eric@test.com", snap.Users[0].Name)
}
