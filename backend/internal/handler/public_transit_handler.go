package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type PublicTransitHandler struct {
	publicTransitService *service.PublicTransitService
}

func NewPublicTransitHandler(publicTransitService *service.PublicTransitService) *PublicTransitHandler {
	return &PublicTransitHandler{publicTransitService: publicTransitService}
}

func (h *PublicTransitHandler) Discovery(c *gin.Context) {
	payload, err := h.publicTransitService.Discovery(c.Request.Context(), requestBaseURL(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (h *PublicTransitHandler) Snapshot(c *gin.Context) {
	payload, err := h.publicTransitService.Snapshot(c.Request.Context(), requestBaseURL(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, payload)
}

// AdminGuard supplements the parent admin-auth middleware: the operations
// console is limited to full administrators, like the Pelican console.
func (h *PublicTransitHandler) AdminGuard(c *gin.Context) {
	if adminaccess.RestrictedFromContext(c.Request.Context()) != nil {
		response.Forbidden(c, "Operations requires full administrator access")
		c.Abort()
		return
	}
	c.Next()
}

// AdminGroupCacheList serves the operations console table: the exported groups
// matching the query conditions, with their real cumulative cache hit rate and
// the stored floor.
func (h *PublicTransitHandler) AdminGroupCacheList(c *gin.Context) {
	rows, err := h.publicTransitService.AdminGroupCache(c.Request.Context(), service.PublicTransitGroupCacheQuery{
		Search:   strings.TrimSpace(c.Query("search")),
		Platform: strings.TrimSpace(c.Query("platform")),
		Status:   strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rows)
}

func (h *PublicTransitHandler) AdminGroupCacheUpdate(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("groupId"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	// Enabled is a pointer so a body without it is rejected instead of pausing the group.
	var in struct {
		Enabled *bool    `json:"enabled"`
		Floor   *float64 `json:"floor"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid group cache setting")
		return
	}
	if in.Enabled == nil {
		response.BadRequest(c, "Enabled is required")
		return
	}
	setting, err := h.publicTransitService.UpdateGroupCacheSetting(c.Request.Context(), groupID, service.PublicTransitGroupCacheSetting{
		Enabled: *in.Enabled,
		Floor:   in.Floor,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, setting)
}

func requestBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	scheme := strings.TrimSpace(c.Request.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}
	host := strings.TrimSpace(c.Request.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}
