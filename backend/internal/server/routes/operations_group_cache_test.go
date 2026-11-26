package routes

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type groupCacheRouteSettings struct {
	service.SettingRepository
	stored map[string]string
}

// Channel monitoring stays off so the list does not reach into monitor services.
func (r *groupCacheRouteSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{service.SettingKeyChannelMonitorEnabled: "false"}, nil
}

func (r *groupCacheRouteSettings) Set(_ context.Context, key, value string) error {
	if r.stored == nil {
		r.stored = map[string]string{}
	}
	r.stored[key] = value
	return nil
}

type groupCacheRouteGroups struct{ service.GroupRepository }

func (groupCacheRouteGroups) ListActive(context.Context) ([]service.Group, error) {
	return []service.Group{{ID: 7, Name: "public", Platform: "anthropic", RateMultiplier: 1.25, Status: service.StatusActive}}, nil
}

func (groupCacheRouteGroups) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	if id == 7 {
		return &service.Group{ID: 7, Name: "public", Platform: "anthropic", Status: service.StatusActive}, nil
	}
	return nil, service.ErrGroupNotFound
}

type groupCacheRouteChannels struct{ service.ChannelRepository }

func (groupCacheRouteChannels) ListAll(context.Context) ([]service.Channel, error) { return nil, nil }

type groupCacheRouteUsage struct{ service.UsageLogRepository }

func (groupCacheRouteUsage) GetGroupCacheUsageSummary(context.Context, time.Time, time.Time) ([]usagestats.GroupCacheUsageSummary, error) {
	return []usagestats.GroupCacheUsageSummary{{
		GroupID: 7,
		Total:   usagestats.GroupCacheUsageWindow{InputTokens: 100, CacheCreationTokens: 20, CacheReadTokens: 30, CacheHitRate: 20},
	}}, nil
}

func newGroupCacheRouteService(settings service.SettingRepository) *service.PublicTransitService {
	groups := groupCacheRouteGroups{}
	return service.NewPublicTransitService(
		service.NewChannelService(groupCacheRouteChannels{}, groups, nil, nil, nil),
		nil,
		service.NewSettingService(settings, nil),
		nil,
		groups,
		groupCacheRouteUsage{},
	)
}

func TestOperationsGroupCacheRequiresAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/api/v1/admin")
	group.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(nil, nil, nil, nil)))
	registerOperationsGroupCacheRoutes(group, &handler.Handlers{
		PublicTransit: handler.NewPublicTransitHandler(newGroupCacheRouteService(&groupCacheRouteSettings{})),
	})
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/v1/admin/operations/group-cache", ""},
		{"PUT", "/api/v1/admin/operations/group-cache/7", `{"enabled":true,"floor":85}`},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		require.Equal(t, 401, w.Code, tc.method+" "+tc.path)
	}
}

func TestOperationsGroupCacheRejectsRestrictedAdminKeys(t *testing.T) {
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
		settings := &groupCacheRouteSettings{}
		registerOperationsGroupCacheRoutes(group, &handler.Handlers{
			PublicTransit: handler.NewPublicTransitHandler(newGroupCacheRouteService(settings)),
		})

		list := httptest.NewRecorder()
		r.ServeHTTP(list, httptest.NewRequest("GET", "/api/v1/admin/operations/group-cache", nil))
		update := httptest.NewRecorder()
		r.ServeHTTP(update, httptest.NewRequest("PUT", "/api/v1/admin/operations/group-cache/7", strings.NewReader(`{"enabled":true,"floor":85}`)))
		if restricted {
			require.Equal(t, 403, list.Code)
			require.Equal(t, 403, update.Code)
			require.Empty(t, settings.stored)
			continue
		}
		require.Equal(t, 200, list.Code)
		// The console lists the real rate and tokens next to the stored floor.
		require.Contains(t, list.Body.String(), `"group_id":7`)
		require.Contains(t, list.Body.String(), `"cache_hit_rate":20`)
		require.Contains(t, list.Body.String(), `"input_tokens":100`)
		require.Contains(t, list.Body.String(), `"cache_creation_tokens":20`)
		require.Contains(t, list.Body.String(), `"cache_read_tokens":30`)
		require.Contains(t, list.Body.String(), `"setting":{"enabled":false,"floor":null}`)
		require.Equal(t, 200, update.Code)
		require.Equal(t, `{"enabled":true,"floor":85}`, settings.stored["public_transit_group_cache.7"])
	}
}

func TestOperationsGroupCacheUpdateRejectsInvalidRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerOperationsGroupCacheRoutes(r.Group("/api/v1/admin"), &handler.Handlers{
		PublicTransit: handler.NewPublicTransitHandler(newGroupCacheRouteService(&groupCacheRouteSettings{})),
	})
	for _, tc := range []struct {
		path     string
		body     string
		expected int
	}{
		{path: "0", body: `{"enabled":true}`, expected: 400},
		{path: "-1", body: `{"enabled":true}`, expected: 400},
		{path: "not-an-id", body: `{"enabled":true}`, expected: 400},
		{path: "7", body: `not json`, expected: 400},
		{path: "7", body: `{"floor":85}`, expected: 400},
		{path: "7", body: `{"enabled":"yes"}`, expected: 400},
		{path: "7", body: `{"enabled":true,"floor":"85"}`, expected: 400},
		{path: "7", body: `{"enabled":true,"floor":0}`, expected: 400},
		{path: "7", body: `{"enabled":true,"floor":100}`, expected: 400},
		{path: "7", body: `{"enabled":true,"floor":85.55}`, expected: 400},
		{path: "99", body: `{"enabled":true}`, expected: 404},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/admin/operations/group-cache/"+tc.path, strings.NewReader(tc.body)))
		require.Equal(t, tc.expected, w.Code, tc.path+" "+tc.body)
	}
}

func TestOperationsGroupCacheListAppliesQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerOperationsGroupCacheRoutes(r.Group("/api/v1/admin"), &handler.Handlers{
		PublicTransit: handler.NewPublicTransitHandler(newGroupCacheRouteService(&groupCacheRouteSettings{})),
	})
	list := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/operations/group-cache"+query, nil))
		return w
	}

	// Group 7 is "public" on anthropic and has no stored setting, so it counts as paused.
	require.Contains(t, list("").Body.String(), `"group_id":7`)
	require.Contains(t, list("?search=pub&platform=anthropic&status=paused").Body.String(), `"group_id":7`)
	require.Contains(t, list("?search=PUBLIC").Body.String(), `"group_id":7`)
	require.NotContains(t, list("?search=nomatch").Body.String(), `"group_id"`)
	require.NotContains(t, list("?platform=openai").Body.String(), `"group_id"`)
	// A query without matches still returns a list.
	require.JSONEq(t, `{"code":0,"message":"success","data":{"items":[]}}`, list("?status=enabled").Body.String())
	require.Equal(t, 400, list("?status=bogus").Code)
}
