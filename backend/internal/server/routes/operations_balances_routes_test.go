package routes

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operationsRouteAdminStub struct {
	service.AdminService
	listed bool
}

func (s *operationsRouteAdminStub) ListAccounts(context.Context, int, int, string, string, string, string, int64, string, string, string) ([]service.Account, int64, error) {
	s.listed = true
	return []service.Account{}, 0, nil
}

func TestOperationsBalanceRouteDoesNotResolveAsAccountID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &operationsRouteAdminStub{}
	account := adminhandler.NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	registerAccountRoutes(r.Group("/api/v1/admin"), &handler.Handlers{Admin: &handler.AdminHandlers{Account: account}}, middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/accounts/operations-balances", nil))
	require.Equal(t, 200, w.Code, w.Body.String())
	require.True(t, svc.listed)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"items":[]}}`, w.Body.String())
}
