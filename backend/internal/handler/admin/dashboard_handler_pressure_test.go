package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type pressureRepoCapture struct {
	service.UsageLogRepository
	snap *usagestats.UsagePressureSnapshot
}

func (r *pressureRepoCapture) GetUsagePressureSnapshot(_ context.Context, _, _ time.Time, _ string) (*usagestats.UsagePressureSnapshot, error) {
	if r.snap != nil {
		return r.snap, nil
	}
	return &usagestats.UsagePressureSnapshot{
		Windows: usagestats.UsagePressureWindows{
			M15: usagestats.UsagePressureWindow{Users: 6, Billed: 4.21, Requests: 10, Accounts: 1},
		},
		PeakHour: &usagestats.UsagePressurePeakHour{Billed: 50},
		Users: []usagestats.UsagePressureActor{
			{ID: 163, Name: "ericlliao@test.com", Requests: 5, Billed: 2.1},
		},
		Accounts: []usagestats.UsagePressureActor{
			{ID: 66274, Name: "linda", Requests: 8, Billed: 4.0},
		},
	}, nil
}

func TestGetUsagePressure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewDashboardService(&pressureRepoCapture{}, nil, nil, nil)
	h := NewDashboardHandler(svc, nil)
	router := gin.New()
	router.GET("/admin/dashboard/pressure", h.GetUsagePressure)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/pressure", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Code int                              `json:"code"`
		Data usagestats.UsagePressureSnapshot `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, usagestats.PressureLevelWarm, body.Data.Level)
	require.Equal(t, int64(6), body.Data.Windows.M15.Users)
	require.Equal(t, "linda", body.Data.Accounts[0].Name)
}

func TestGetUsagePressureUnsupported(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewDashboardService(&userBreakdownRepoCapture{}, nil, nil, nil)
	h := NewDashboardHandler(svc, nil)
	router := gin.New()
	router.GET("/admin/dashboard/pressure", h.GetUsagePressure)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/pressure", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}
