package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPelicanListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.1","owned_by":"openai"},{"id":""}]}`))
	}))
	defer server.Close()

	svc := NewPelicanService(nil)
	models, err := svc.ListModels(context.Background(), server.URL, "sk-test")
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "gpt-5.1", models[0].ID)
}

func TestPelicanOpenChatStreamSendsPromptAndStreamOptions(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &payload))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	svc := NewPelicanService(nil)
	resp, err := svc.OpenChatStream(context.Background(), server.URL, "sk-test", "gpt-5.1", "low")
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	require.Equal(t, true, payload["stream"])
	require.Equal(t, "gpt-5.1", payload["model"])
	require.Equal(t, "low", payload["reasoning_effort"])
	options, ok := payload["stream_options"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, options["include_usage"])

	messages, ok := payload["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)
	message := messages[0].(map[string]any)
	require.Equal(t, "user", message["role"])
	require.Equal(t, PelicanPrompt, message["content"])
}

func TestPelicanOpenChatStreamMapsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key sk-secret"}}`))
	}))
	defer server.Close()

	svc := NewPelicanService(nil)
	_, err := svc.OpenChatStream(context.Background(), server.URL, "sk-test", "gpt-5.1", "")
	var upstream *PelicanUpstreamError
	require.ErrorAs(t, err, &upstream)
	require.Equal(t, http.StatusUnauthorized, upstream.StatusCode)
	require.Contains(t, upstream.Message, "[已隐藏]")
}
