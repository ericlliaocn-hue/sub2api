package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type pressureUsageRepoStub struct {
	service.UsageLogRepository
}

func (r *pressureUsageRepoStub) GetUsagePressureSnapshot(_ context.Context, _, _ time.Time, _ string) (*usagestats.UsagePressureSnapshot, error) {
	return &usagestats.UsagePressureSnapshot{
		Windows: usagestats.UsagePressureWindows{
			M15: usagestats.UsagePressureWindow{Users: 6, Accounts: 2, Billed: 4.21, Requests: 10},
		},
		PeakHour: &usagestats.UsagePressurePeakHour{Billed: 50},
		Users:    []usagestats.UsagePressureActor{{ID: 163, Name: "eric@test.com"}},
		Accounts: []usagestats.UsagePressureActor{{ID: 66274, Name: "linda"}},
	}, nil
}

type pressureSettingRepoStub struct {
	allowed bool
}

func (s *pressureSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get")
}
func (s *pressureSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue")
}
func (s *pressureSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set")
}
func (s *pressureSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	if s.allowed {
		out[service.SettingKeyAllowUserViewUsagePressure] = "true"
	}
	return out, nil
}
func (s *pressureSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple")
}
func (s *pressureSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll")
}
func (s *pressureSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete")
}

func newUserPressureRouter(allowed bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(&pressureUsageRepoStub{}, nil, nil, nil)
	settingSvc := service.NewSettingService(&pressureSettingRepoStub{allowed: allowed}, &config.Config{})
	h := NewUsageHandler(usageSvc, nil, nil, settingSvc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		c.Next()
	})
	router.GET("/usage/dashboard/pressure", h.DashboardPressure)
	return router
}

func TestDashboardPressureForbiddenWhenDisabled(t *testing.T) {
	router := newUserPressureRouter(false)
	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/pressure", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestDashboardPressureRedactsActorsWhenEnabled(t *testing.T) {
	router := newUserPressureRouter(true)
	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/pressure", nil)
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
	require.Empty(t, body.Data.Users)
	require.Empty(t, body.Data.Accounts)
}
