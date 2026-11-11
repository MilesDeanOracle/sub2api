package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCopyPelicanSSEPassesChunksThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	require.NoError(t, copyPelicanSSE(context.Background(), strings.NewReader("data: one\n\ndata: two\n\n"), recorder, recorder))
	require.Equal(t, "data: one\n\ndata: two\n\n", recorder.Body.String())
}

func TestPelicanRequestOriginIgnoresForwardedHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "http://gateway.local/api/v1/pelican/runs", nil)
	ctx.Request.Host = "gateway.local"
	ctx.Request.Header.Set("X-Forwarded-Host", "169.254.169.254")
	ctx.Request.Header.Set("X-Forwarded-Proto", "https")

	require.Equal(t, "https://gateway.local", pelicanRequestOrigin(ctx))
}

func TestIsPelicanReasoningLevel(t *testing.T) {
	require.True(t, isPelicanReasoningLevel("low"))
	require.True(t, isPelicanReasoningLevel("xhigh"))
	require.False(t, isPelicanReasoningLevel("ultra"))
	require.False(t, isPelicanReasoningLevel(""))
}

func TestValidateManualPelicanAPIKey(t *testing.T) {
	require.NoError(t, validateManualPelicanAPIKey("sk-test-123"))
	require.Error(t, validateManualPelicanAPIKey(""))
	require.Error(t, validateManualPelicanAPIKey("sk test"))
	require.Error(t, validateManualPelicanAPIKey("sk\nkey"))
	require.Error(t, validateManualPelicanAPIKey(strings.Repeat("a", 513)))
}
