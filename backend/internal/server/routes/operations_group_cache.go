package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"

	"github.com/gin-gonic/gin"
)

// registerOperationsGroupCacheRoutes mounts the operations console API for the
// per-group public snapshot cache hit rate floor.
func registerOperationsGroupCacheRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	if h.PublicTransit == nil {
		return
	}
	groupCache := admin.Group("/operations/group-cache")
	groupCache.Use(h.PublicTransit.AdminGuard)
	groupCache.GET("", h.PublicTransit.AdminGroupCacheList)
	groupCache.PUT("/:groupId", h.PublicTransit.AdminGroupCacheUpdate)
}
