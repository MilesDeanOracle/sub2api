package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePelicanBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "plain host", input: "https://api.example.com", want: "https://api.example.com"},
		{name: "trailing slash", input: "https://api.example.com/", want: "https://api.example.com"},
		{name: "v1 suffix", input: "https://api.example.com/v1", want: "https://api.example.com"},
		{name: "v1 suffix with path", input: "https://api.example.com/openai/v1/", want: "https://api.example.com/openai"},
		{name: "empty", input: "   ", want: ""},
		{name: "ftp", input: "ftp://api.example.com", wantErr: true},
		{name: "userinfo", input: "https://user:pass@api.example.com", wantErr: true},
		{name: "query", input: "https://api.example.com?x=1", wantErr: true},
		{name: "no host", input: "https:///v1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePelicanBaseURL(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildPelicanAPIURL(t *testing.T) {
	require.Equal(t, "https://api.example.com/v1/models", buildPelicanAPIURL("https://api.example.com", "/models"))
	require.Equal(t, "https://api.example.com/v1/models", buildPelicanAPIURL("https://api.example.com/", "/models"))
	require.Equal(t, "https://api.example.com/v1/models", buildPelicanAPIURL("https://api.example.com/v1", "/models"))
	require.Equal(
		t,
		"https://api.example.com/openai/v1/chat/completions",
		buildPelicanAPIURL("https://api.example.com/openai", "/chat/completions"),
	)
}

func TestExtractPelicanErrorMessageRedactsSecrets(t *testing.T) {
	msg := extractPelicanErrorMessage([]byte(`{"error":{"message":"bad key sk-1234567890"}}`))
	require.Contains(t, msg, "[已隐藏]")
	require.NotContains(t, msg, "sk-1234567890")

	msg = extractPelicanErrorMessage([]byte(`{"message":"rate limited"}`))
	require.Equal(t, "rate limited", msg)
}

func TestPelicanPromptIsFixed(t *testing.T) {
	svc := NewPelicanService(nil)
	test := svc.Test()
	require.Equal(t, PelicanTestID, test.ID)
	require.Equal(t, PelicanPromptVersion, test.PromptVersion)
	require.Contains(t, test.Prompt, "鹈鹕骑自行车")
	require.Contains(t, test.Prompt, "不要代码围栏")
}

func TestResolvePelicanBaseURLPriority(t *testing.T) {
	t.Setenv("PELICAN_TEST_BASE_URL", "")
	svc := NewPelicanService(nil)
	ctx := context.Background()

	base, source, err := svc.ResolveBaseURL(ctx, "https://manual.example.com/v1", "https://frontend.example.com", "https://request.example.com")
	require.NoError(t, err)
	require.Equal(t, "https://manual.example.com", base)
	require.Equal(t, "manual", source)

	base, source, err = svc.ResolveBaseURL(ctx, "", "https://frontend.example.com", "https://request.example.com")
	require.NoError(t, err)
	require.Equal(t, "https://frontend.example.com", base)
	require.Equal(t, "frontend_url", source)

	base, source, err = svc.ResolveBaseURL(ctx, "", "", "https://request.example.com/path")
	require.NoError(t, err)
	require.Equal(t, "https://request.example.com/path", base)
	require.Equal(t, "request", source)
}

func TestEnsurePublicHostRejectsLoopback(t *testing.T) {
	t.Setenv("PELICAN_ALLOW_PRIVATE_BASE_URL", "")
	svc := NewPelicanService(nil)
	require.Error(t, svc.EnsurePublicHost(context.Background(), "http://127.0.0.1:8080"))
	require.Error(t, svc.EnsurePublicHost(context.Background(), "http://169.254.169.254/latest/meta-data"))
	require.Error(t, svc.EnsurePublicHost(context.Background(), "http://10.0.0.1"))
}

func TestPelicanReasoningLevelsContainLow(t *testing.T) {
	require.Contains(t, strings.Join(PelicanReasoningLevels, ","), "low")
}
