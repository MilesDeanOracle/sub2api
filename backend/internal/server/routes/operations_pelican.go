package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerOperationsPelicanRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	if h.Admin.OperationsPelican == nil {
		return
	}
	handler := h.Admin.OperationsPelican
	ops := admin.Group("/operations")
	ops.Use(handler.Guard)
	ops.GET("/accounts", handler.Accounts)
	ops.GET("/accounts/:id/models", handler.Models)
	ops.GET("/storage-sources", handler.StorageSources)
	ops.POST("/storage-sources/:source/test", handler.TestStorage)
	ops.GET("/tasks", handler.Tasks)
	ops.POST("/tasks", handler.Save)
	ops.PUT("/tasks/:id", handler.Save)
	ops.PUT("/tasks/:id/enabled", handler.Enable)
	ops.DELETE("/tasks/:id", handler.Delete)
	ops.POST("/tasks/:id/runs", handler.Trigger)
	ops.GET("/runs/:runId", handler.Run)
	ops.GET("/status", handler.Status)
	ops.GET("/results", handler.Results)
	ops.GET("/results/filters", handler.Filters)
	ops.GET("/results/groups", handler.Groups)
	ops.DELETE("/results/groups/:groupId", handler.CleanGroup)
	ops.GET("/results/:id", handler.Result)
	ops.GET("/results/:id/content", handler.Content)
}
