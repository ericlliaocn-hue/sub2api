package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type pressureRepoStub struct {
	UsageLogRepository
	now        time.Time
	todayStart time.Time
	tz         string
	snap       *usagestats.UsagePressureSnapshot
}

func (r *pressureRepoStub) GetUsagePressureSnapshot(_ context.Context, now, todayStart time.Time, tz string) (*usagestats.UsagePressureSnapshot, error) {
	r.now = now
	r.todayStart = todayStart
	r.tz = tz
	return r.snap, nil
}

func TestGetUsagePressureFinalizesSnapshot(t *testing.T) {
	require.NoError(t, timezone.Init("Asia/Shanghai"))

	now := time.Date(2026, 9, 20, 21, 5, 0, 0, timezone.Location())
	repo := &pressureRepoStub{
		snap: &usagestats.UsagePressureSnapshot{
			Windows: usagestats.UsagePressureWindows{
				M15: usagestats.UsagePressureWindow{Users: 6, Billed: 4.21},
			},
			PeakHour: &usagestats.UsagePressurePeakHour{Billed: 50},
		},
	}
	svc := NewDashboardService(repo, nil, nil, nil)

	snap, err := svc.GetUsagePressure(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, now, repo.now)
	require.Equal(t, timezone.StartOfDay(now), repo.todayStart)
	require.Equal(t, "Asia/Shanghai", repo.tz)
	require.Equal(t, now, snap.GeneratedAt)
	require.Equal(t, usagestats.PressureLevelWarm, snap.Level)
	require.InDelta(t, 16.84, snap.HourlyFrom15m, 0.0001)
}

func TestGetUsagePressureUnsupportedRepo(t *testing.T) {
	svc := NewDashboardService(&usageRepoStub{}, nil, nil, nil)
	_, err := svc.GetUsagePressure(context.Background(), time.Time{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not supported")
}
