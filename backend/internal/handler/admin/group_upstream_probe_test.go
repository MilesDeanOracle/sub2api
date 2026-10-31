package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHTTPUpstreamGroupProberFetchesAvailableGroups(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer upstream-jwt-access-token", r.Header.Get("Authorization"))
		if r.URL.Path == "/api/v1/keys" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"items": []map[string]any{
						{"key": "anthropic-key", "group_id": 1, "status": "active"},
						{"key": "openai-key", "group": map[string]any{"name": "pool-2"}, "status": "active"},
					},
				},
			})
			return
		}
		require.Equal(t, "/api/v1/groups/available", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": []upstreamGroupSummary{
				{ID: 1, Name: "pool-1", Platform: "anthropic", Status: "active", APIKey: "anthropic-key"},
				{ID: 2, Name: "pool-2", Platform: "openai", Status: "active"},
			},
		})
	}))
	defer server.Close()

	prober := &httpUpstreamGroupProber{
		client: server.Client(),
		normalizeBaseURL: func(raw string) (string, error) {
			return raw, nil
		},
	}
	groups, err := prober.Probe(context.Background(), server.URL, "upstream-jwt-access-token")

	require.NoError(t, err)
	require.Equal(t, []upstreamGroupSummary{
		{ID: 1, Name: "pool-1", Platform: "anthropic", Status: "active", APIKey: "anthropic-key"},
		{ID: 2, Name: "pool-2", Platform: "openai", Status: "active", APIKey: "openai-key"},
	}, groups)
}

func TestHTTPUpstreamGroupProberClassifiesAuthAndIncompatibleResponses(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantReason string
	}{
		{
			name: "authentication failed",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantReason: "UPSTREAM_GROUP_PROBE_AUTH_FAILED",
		},
		{
			name: "incompatible response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"code":0,"data":[{"id":1}]}`))
			},
			wantReason: "UPSTREAM_GROUP_PROBE_RESPONSE_INVALID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()
			prober := &httpUpstreamGroupProber{
				client:           server.Client(),
				normalizeBaseURL: func(raw string) (string, error) { return raw, nil },
			}

			groups, err := prober.Probe(context.Background(), server.URL, "bad-access-token")

			require.Nil(t, groups)
			require.Error(t, err)
			require.Equal(t, test.wantReason, infraerrors.Reason(err))
		})
	}
}

func TestNormalizeUpstreamGroupBaseURLRejectsCredentialsAndPrivateTargets(t *testing.T) {
	_, err := normalizeUpstreamGroupBaseURL(nil, "https://admin:secret@example.com")
	require.ErrorContains(t, err, "user info")

	_, err = normalizeUpstreamGroupBaseURL(nil, "https://127.0.0.1:8080")
	require.ErrorContains(t, err, "not allowed")
}

type upstreamGroupProberStub struct {
	groups      []upstreamGroupSummary
	err         error
	baseURL     string
	accessToken string
}

func (s *upstreamGroupProberStub) Probe(_ context.Context, baseURL, accessToken string) ([]upstreamGroupSummary, error) {
	s.baseURL = baseURL
	s.accessToken = accessToken
	return s.groups, s.err
}

func TestGroupHandlerProbeUpstreamGroupsReturnsStableProjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &upstreamGroupProberStub{groups: []upstreamGroupSummary{{
		ID: 7, Name: "Claude Max", Platform: "anthropic", Status: "active", RateMultiplier: 1.25,
	}}}
	handler := &GroupHandler{upstreamGroupProber: stub}
	router := gin.New()
	router.POST("/probe", handler.ProbeUpstreamGroups)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/probe", stringsReader(`{"base_url":"https://upstream.example","access_token":"secret-jwt"}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "https://upstream.example", stub.baseURL)
	require.Equal(t, "secret-jwt", stub.accessToken)
	require.Contains(t, recorder.Body.String(), `"name":"Claude Max"`)
	require.NotContains(t, recorder.Body.String(), "secret-jwt")
}

func TestGroupHandlerProbeUpstreamGroupsDoesNotEchoCredentialsOnFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &upstreamGroupProberStub{err: infraerrors.BadRequest("UPSTREAM_GROUP_PROBE_REQUEST_FAILED", "unable to reach the upstream service").WithCause(errors.New("dial failed"))}
	handler := &GroupHandler{upstreamGroupProber: stub}
	router := gin.New()
	router.POST("/probe", handler.ProbeUpstreamGroups)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/probe", stringsReader(`{"base_url":"https://upstream.example","access_token":"do-not-echo"}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "do-not-echo")
}

func stringsReader(value string) *strings.Reader {
	return strings.NewReader(value)
}
