package middleware

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type headerFeatureSettings interface {
	GetHeaderFeaturesRuntime(context.Context) (service.HeaderFeaturesRuntime, error)
}

func PelicanFeatureGuard(settings headerFeatureSettings) gin.HandlerFunc {
	return headerFeatureGuard(settings, true)
}

func FeedbackFeatureGuard(settings headerFeatureSettings) gin.HandlerFunc {
	return headerFeatureGuard(settings, false)
}

func headerFeatureGuard(settings headerFeatureSettings, pelican bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		features, err := settings.GetHeaderFeaturesRuntime(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusServiceUnavailable, "Feature settings unavailable")
			c.Abort()
			return
		}
		role, _ := GetUserRoleFromContext(c)
		allowed := features.FeedbackEnabled
		if pelican {
			allowed = features.PelicanEnabled && (features.PelicanVisibility == "all" || role == "admin")
		}
		if !allowed {
			response.Forbidden(c, "This feature is disabled or unavailable for your account")
			c.Abort()
			return
		}
		c.Next()
	}
}
