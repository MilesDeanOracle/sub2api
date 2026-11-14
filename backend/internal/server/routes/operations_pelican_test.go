package routes

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operationsRouteRepo struct {
	service.OperationsRepository
	read bool
}

func (r *operationsRouteRepo) Status(context.Context) (*service.OperationsStatus, error) {
	r.read = true
	return &service.OperationsStatus{MaxWorkers: 4}, nil
}
func TestOperationsRequiresAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/api/v1/admin")
	group.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(nil, nil, nil, nil)))
	h := admin.NewOperationsPelicanHandler(nil, nil)
	registerOperationsPelicanRoutes(group, &handler.Handlers{Admin: &handler.AdminHandlers{OperationsPelican: h}})
	for _, path := range []string{"/status", "/results/1/content", "/tasks", "/results/groups"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/operations"+path, nil))
		require.Equal(t, 401, w.Code)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/v1/admin/operations/results/groups/2", nil))
	require.Equal(t, 401, w.Code)
}
func TestOperationsRejectsRestrictedAdminKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, restricted := range []bool{true, false} {
		r := gin.New()
		group := r.Group("/api/v1/admin")
		if restricted {
			group.Use(func(c *gin.Context) {
				c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), &adminaccess.Principal{}))
				c.Next()
			})
		}
		repo := &operationsRouteRepo{}
		h := admin.NewOperationsPelicanHandler(nil, repo)
		registerOperationsPelicanRoutes(group, &handler.Handlers{Admin: &handler.AdminHandlers{OperationsPelican: h}})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/operations/status", nil))
		if restricted {
			require.Equal(t, 403, w.Code)
			require.False(t, repo.read)
			denied := httptest.NewRecorder()
			r.ServeHTTP(denied, httptest.NewRequest("DELETE", "/api/v1/admin/operations/results/groups/2", nil))
			require.Equal(t, 403, denied.Code)
		} else {
			require.Equal(t, 200, w.Code)
			require.True(t, repo.read)
		}
	}
}
