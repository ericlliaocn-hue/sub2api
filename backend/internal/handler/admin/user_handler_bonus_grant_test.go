package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type bonusGrantAdminServiceStub struct {
	*stubAdminService
	calls []service.AdminBonusGrantInput
}

func (s *bonusGrantAdminServiceStub) GrantExpiringBonus(_ context.Context, input service.AdminBonusGrantInput) (*service.AdminBonusGrantResult, error) {
	s.calls = append(s.calls, input)
	return &service.AdminBonusGrantResult{Affected: len(input.UserIDs), OperationID: input.OperationID}, nil
}

func setupBonusGrantRouter(adminService service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})
		c.Next()
	})
	handler := NewUserHandler(adminService, nil, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/users/bonus-grants", handler.GrantExpiringBonus)
	return router
}

func TestUserHandlerGrantExpiringBonusMapsBatchAndIdempotencyKey(t *testing.T) {
	service.SetDefaultIdempotencyCoordinator(nil)
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(nil) })
	stub := &bonusGrantAdminServiceStub{stubAdminService: newStubAdminService()}
	router := setupBonusGrantRouter(stub)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/bonus-grants", bytes.NewBufferString(`{
		"user_ids":[9,3],"amount":5,"expires_at":"2026-08-30T15:59:00Z",
		"campaign_id":"weekend","notes":"周末活动"
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "operation-1")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, stub.calls, 1)
	require.Equal(t, []int64{9, 3}, stub.calls[0].UserIDs)
	require.Equal(t, 5.0, stub.calls[0].Amount)
	require.Equal(t, "weekend", stub.calls[0].CampaignID)
	require.Equal(t, "operation-1", stub.calls[0].OperationID)
	require.Equal(t, int64(77), stub.calls[0].GrantedBy)
	require.Equal(t, time.Date(2026, 8, 30, 15, 59, 0, 0, time.UTC), stub.calls[0].ExpiresAt)
}

func TestUserHandlerGrantExpiringBonusRequiresIdempotencyKey(t *testing.T) {
	service.SetDefaultIdempotencyCoordinator(nil)
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(nil) })
	stub := &bonusGrantAdminServiceStub{stubAdminService: newStubAdminService()}
	router := setupBonusGrantRouter(stub)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/bonus-grants", bytes.NewBufferString(`{
		"user_ids":[9],"amount":5,"expires_at":"2099-08-30T15:59:00Z","campaign_id":"weekend"
	}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, stub.calls)
}
