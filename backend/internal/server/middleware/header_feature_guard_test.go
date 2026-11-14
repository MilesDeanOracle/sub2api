package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type headerFeaturesStub struct {
	features service.HeaderFeaturesRuntime
	err      error
}

func (s headerFeaturesStub) GetHeaderFeaturesRuntime(context.Context) (service.HeaderFeaturesRuntime, error) {
	return s.features, s.err
}

func TestHeaderFeatureGuards(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		features   service.HeaderFeaturesRuntime
		pelican    bool
		err        error
		status     int
	}{
		{"all users", "user", service.HeaderFeaturesRuntime{PelicanEnabled: true, PelicanVisibility: "all"}, true, nil, 204},
		{"admin only denies user", "user", service.HeaderFeaturesRuntime{PelicanEnabled: true, PelicanVisibility: "admin"}, true, nil, 403},
		{"admin only allows admin", "admin", service.HeaderFeaturesRuntime{PelicanEnabled: true, PelicanVisibility: "admin"}, true, nil, 204},
		{"disabled denies admin", "admin", service.HeaderFeaturesRuntime{}, true, nil, 403},
		{"feedback on", "user", service.HeaderFeaturesRuntime{FeedbackEnabled: true}, false, nil, 204},
		{"feedback off", "admin", service.HeaderFeaturesRuntime{}, false, nil, 403},
		{"storage error", "admin", service.HeaderFeaturesRuntime{}, true, errors.New("offline"), 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set(string(ContextKeyUserRole), tc.role) })
			stub := headerFeaturesStub{tc.features, tc.err}
			guard := FeedbackFeatureGuard(stub)
			if tc.pelican {
				guard = PelicanFeatureGuard(stub)
			}
			router.GET("/feature", guard, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/feature", nil))
			require.Equal(t, tc.status, recorder.Code)
		})
	}
}
