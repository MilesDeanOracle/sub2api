package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

func operationsEvents(events ...string) string {
	return "data: " + strings.Join(events, "\n\ndata: ") + "\n\n"
}

type operationsHTTPTestUpstream struct {
	HTTPUpstream
	request func(*http.Request) (*http.Response, error)
}

func (u operationsHTTPTestUpstream) DoWithTLS(r *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.request(r)
}

type operationsAccountRepo struct {
	AccountRepository
	account *Account
}

func (r operationsAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}

func TestOperationsLiveModelsCacheAndInvalidation(t *testing.T) {
	a := &Account{ID: 12, Type: AccountTypeUpstream, Platform: PlatformAnthropic, Status: StatusActive, Credentials: map[string]any{"api_key": "first-key", "base_url": "https://upstream.example/v1"}}
	calls := 0
	upstream := operationsHTTPTestUpstream{request: func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, a.GetCredential("api_key"), r.Header.Get("x-api-key"))
		require.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"live-model"}]}`))}, nil
	}}
	s := NewOperationsPelicanService(nil, operationsAccountRepo{account: a}, &AccountTestService{cfg: &config.Config{}, httpUpstream: upstream}, nil, nil, nil, nil)
	ctx := context.Background()
	first, e := s.Models(ctx, a.ID, false)
	require.NoError(t, e)
	require.Equal(t, []string{"live-model"}, first.Models)
	cached, e := s.Models(ctx, a.ID, false)
	require.NoError(t, e)
	require.True(t, cached.Cached)
	require.Equal(t, 1, calls)
	_, e = s.Models(ctx, a.ID, true)
	require.NoError(t, e)
	require.Equal(t, 2, calls)
	a.Credentials["api_key"] = "rotated-key"
	_, e = s.Models(ctx, a.ID, false)
	require.NoError(t, e)
	require.Equal(t, 3, calls)
	// Losing storage settings must produce durable work units that fail before
	// sending a generation request, instead of silently skipping every schedule.
	items, e := s.prepare(ctx, &OperationsTask{StorageSource: "backup_s3", AccountConfigs: []OperationsAccountConfig{{AccountID: a.ID, ModelID: "live-model"}}}, false)
	require.NoError(t, e)
	require.Len(t, items, 1)
	_, e = s.boundStore(ctx, items[0])
	require.Error(t, e)
	require.Equal(t, 3, calls)
}

func TestOperationsAnthropicInitialUsageIsNotFinal(t *testing.T) {
	stream := operationsEvents(`{"type":"message_start","message":{"usage":{"output_tokens":1}}}`, `{"type":"content_block_delta","delta":{"type":"text_delta","text":"<html>ok</html>"}}`, `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`, `{"type":"message_stop"}`)
	r := &OperationsResult{}
	_, e := operationsReadStream(strings.NewReader(stream), APIProtocolAnthropic, r, time.Now())
	require.NoError(t, e)
	require.Equal(t, false, r.UsageMetadata["complete"])
	require.Nil(t, r.TPS)
}

func TestOperationsProtocolRequests(t *testing.T) {
	for _, tc := range []struct{ protocol, platform, path, stream string }{
		{APIProtocolChatCompletions, PlatformOpenAI, "/v1/chat/completions", operationsEvents(`{"choices":[{"delta":{"content":"<html>ok</html>"},"finish_reason":"stop"}]}`, `[DONE]`)},
		{APIProtocolResponses, PlatformOpenAI, "/v1/responses", operationsEvents(`{"type":"response.output_text.delta","delta":"<html>ok</html>"}`, `{"type":"response.completed","response":{"status":"completed"}}`)},
		{APIProtocolAnthropic, PlatformAnthropic, "/v1/messages", operationsEvents(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"<html>ok</html>"}}`, `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`, `{"type":"message_stop"}`)},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			a := &Account{ID: 8, Type: AccountTypeUpstream, Platform: tc.platform, Status: StatusActive, Credentials: map[string]any{"base_url": "https://upstream.example/v1", "api_key": "key"}, Extra: map[string]any{"openai_responses_supported": tc.protocol == APIProtocolResponses}}
			upstream := operationsHTTPTestUpstream{request: func(req *http.Request) (*http.Response, error) {
				require.Equal(t, tc.path, req.URL.Path)
				require.Equal(t, http.MethodPost, req.Method)
				if tc.protocol == APIProtocolAnthropic {
					require.Equal(t, "key", req.Header.Get("x-api-key"))
				} else {
					require.Equal(t, "Bearer key", req.Header.Get("Authorization"))
				}
				var body map[string]any
				require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
				require.Equal(t, "upstream-model", body["model"])
				require.Equal(t, true, body["stream"])
				require.NotContains(t, body, "api_key")
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.stream))}, nil
			}}
			s := &OperationsPelicanService{tests: &AccountTestService{cfg: &config.Config{}, httpUpstream: upstream}}
			r := &OperationsResult{ModelID: "upstream-model", UpstreamModelID: "upstream-model", ConfigSnapshot: OperationsTask{Prompt: "HTML"}}
			html, e := s.generate(context.Background(), a, r)
			require.NoError(t, e)
			require.Equal(t, "<html>ok</html>", html)
			require.NotNil(t, r.RequestStartedAt)
			require.NotNil(t, r.TTFTMs)
		})
	}
}
func TestOperationsStreams(t *testing.T) {
	for _, tc := range []struct {
		name, protocol, stream string
		reasoning              bool
	}{
		{"chat reasoning", APIProtocolChatCompletions, operationsEvents(`{"choices":[{"delta":{"role":"assistant"}}]}`, `{"choices":[{"delta":{"reasoning_content":"think"}}]}`, `{"choices":[{"delta":{"content":"<!doctype html><html>"}}]}`, `{"choices":[{"delta":{"content":"ok</html>"},"finish_reason":"stop"}],"usage":{"completion_tokens":20}}`, `[DONE]`), true},
		{"chat text fallback", APIProtocolChatCompletions, operationsEvents(`{"choices":[{"delta":{"content":"<html>ok</html>"},"finish_reason":"stop"}]}`, `[DONE]`), false},
		{"responses", APIProtocolResponses, operationsEvents(`{"type":"response.reasoning_summary_text.delta","delta":"thinking"}`, `{"type":"response.output_text.delta","delta":"<html>ok</html>"}`, `{"type":"response.completed","response":{"status":"completed","usage":{"output_tokens":12,"output_tokens_details":{"reasoning_tokens":2}}}}`), true},
		{"anthropic", APIProtocolAnthropic, operationsEvents(`{"type":"message_start","message":{"usage":{"input_tokens":7,"output_tokens":1}}}`, `{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"think"}}`, `{"type":"content_block_delta","delta":{"type":"text_delta","text":"<html>ok</html>"}}`, `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}`, `{"type":"message_stop"}`), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &OperationsResult{}
			html, e := operationsReadStream(strings.NewReader(tc.stream), tc.protocol, r, time.Now().Add(-time.Second))
			require.NoError(t, e)
			require.Contains(t, html, "</html>")
			require.NotNil(t, r.TTFTMs)
			require.NotNil(t, r.FirstTextMs)
			if tc.reasoning {
				require.Equal(t, r.FirstReasoningMs, r.TTFTMs)
			} else {
				require.Nil(t, r.FirstReasoningMs)
				require.Equal(t, r.FirstTextMs, r.TTFTMs)
				require.Nil(t, r.TPS)
			}
		})
	}
}
func TestOperationsRejectsIncompleteStreams(t *testing.T) {
	for _, stream := range []string{
		operationsEvents(`{"choices":[{"delta":{"content":"<html>ok</html>"}}]}`),
		operationsEvents(`{"choices":[{"delta":{"content":"<html>ok</html>"},"finish_reason":"length"}]}`, `[DONE]`),
		operationsEvents(`{"choices":[{"delta":{"content":"<html>truncated"},"finish_reason":"stop"}]}`, `[DONE]`),
		operationsEvents(`{"error":{"message":"do not expose key sk-secret"}}`),
		operationsEvents(`{"choices":[{"finish_reason":"sk-secret"}]}`, `[DONE]`),
	} {
		r := &OperationsResult{}
		_, e := operationsReadStream(strings.NewReader(stream), APIProtocolChatCompletions, r, time.Now())
		require.Error(t, e)
		require.NotContains(t, e.Error(), "sk-secret")
		require.Nil(t, r.TPS)
	}
}
func TestOperationsTPSUsesTextDuration(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	go func() {
		defer writer.Close()
		fmt.Fprint(writer, operationsEvents(`{"choices":[{"delta":{"content":"<html>"}}]}`))
		time.Sleep(20 * time.Millisecond)
		fmt.Fprint(writer, operationsEvents(`{"choices":[{"delta":{"content":"done</html>"},"finish_reason":"stop"}],"usage":{"completion_tokens":40}}`, `[DONE]`))
	}()
	r := &OperationsResult{}
	_, e := operationsReadStream(reader, APIProtocolChatCompletions, r, time.Now())
	require.NoError(t, e)
	require.NotNil(t, r.TPS)
	require.Equal(t, float64(40000)/float64(*r.LastTextMs-*r.FirstTextMs), *r.TPS)
}
func TestOperationsReasoningMapping(t *testing.T) {
	a := &Account{}
	out, e := operationsReasoning(a, APIProtocolAnthropic, "claude-sonnet-4-6", "high")
	require.NoError(t, e)
	require.Equal(t, map[string]any{"type": "adaptive"}, out["thinking"])
	_, e = operationsReasoning(a, APIProtocolAnthropic, "claude-sonnet-4-6", "xhigh")
	require.Error(t, e)
	out, e = operationsReasoning(a, APIProtocolAnthropic, "claude-sonnet-4-5", "high")
	require.NoError(t, e)
	require.Equal(t, map[string]any{"type": "enabled", "budget_tokens": 8192}, out["thinking"])
	_, e = operationsReasoning(a, APIProtocolAnthropic, "claude-sonnet-5-5", "none")
	require.Error(t, e)
}
func TestOperationsAccountEligibility(t *testing.T) {
	a := &Account{Type: AccountTypeAPIKey, Platform: PlatformAnthropic, Status: StatusActive, Credentials: map[string]any{"api_key": "secret"}}
	_, _, e := operationsAccountTarget(a, "")
	require.Error(t, e)
	a.Credentials["base_url"] = "https://example.com"
	p, base, e := operationsAccountTarget(a, "")
	require.NoError(t, e)
	require.Equal(t, APIProtocolAnthropic, p)
	require.Equal(t, "https://example.com", base)
	a.Type = AccountTypeUpstream
	_, _, e = operationsAccountTarget(a, "")
	require.NoError(t, e)
	id := int64(1)
	a.ProxyID = &id
	_, _, e = operationsAccountTarget(a, "")
	require.Error(t, e)
}
func TestOperationsResultNeverSerializesSecrets(t *testing.T) {
	r := OperationsResult{StorageBinding: "encrypted-secret", WorkerToken: "worker-secret", TriggerKey: "request-secret", VersionID: "version-secret", AccountName: "visible"}
	body, e := json.Marshal(r)
	require.NoError(t, e)
	require.NotContains(t, string(body), "secret")
	require.Contains(t, string(body), "visible")
}

type operationsCleanupRepo struct {
	OperationsRepository
	items           []*OperationsResult
	removed, failed bool
}

func (r *operationsCleanupRepo) PrepareGroupCleanup(context.Context, int64) (*OperationsGroupCleanupBatch, error) {
	return &OperationsGroupCleanupBatch{Items: r.items, Detached: 1}, nil
}

func (r *operationsCleanupRepo) ClaimCleanup(context.Context, int) ([]*OperationsResult, error) {
	items := r.items
	r.items = nil
	return items, nil
}
func (r *operationsCleanupRepo) CompleteCleanup(context.Context, int64) error {
	r.removed = true
	return nil
}
func (r *operationsCleanupRepo) FailCleanup(context.Context, int64, string) error {
	r.failed = true
	return nil
}

type operationsTestEncryptor struct{}

func (operationsTestEncryptor) Encrypt(s string) (string, error) { return s, nil }
func (operationsTestEncryptor) Decrypt(s string) (string, error) { return s, nil }

type operationsCleanupStore struct {
	OperationsObjectStore
	deleteError error
	deleted     bool
}

func (s *operationsCleanupStore) Delete(context.Context, string, string) error {
	s.deleted = true
	return s.deleteError
}
func TestOperationsCleanupDeletesObjectBeforeDatabase(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			repo := &operationsCleanupRepo{items: []*OperationsResult{{ID: 7, ObjectKey: OperationsPelicanPrefix + "test.html", StorageBinding: `{"bucket":"test","access_key_id":"key","secret_access_key":"secret"}`}}}
			store := &operationsCleanupStore{}
			if fail {
				store.deleteError = errors.New("denied")
			}
			svc := &OperationsPelicanService{repo: repo, encryptor: operationsTestEncryptor{}, stores: func(context.Context, *BackupS3Config) (OperationsObjectStore, error) { return store, nil }}
			svc.cleanup(context.Background())
			require.True(t, store.deleted)
			require.Equal(t, !fail, repo.removed)
			require.Equal(t, fail, repo.failed)
		})
	}
}
func TestOperationsManualGroupCleanup(t *testing.T) {
	for _, fail := range []bool{false, true} {
		repo := &operationsCleanupRepo{items: []*OperationsResult{{ID: 9, ObjectKey: OperationsPelicanPrefix + "group.html", StorageBinding: `{"bucket":"test","access_key_id":"key","secret_access_key":"secret"}`}}}
		store := &operationsCleanupStore{}
		if fail {
			store.deleteError = errors.New("denied")
		}
		svc := &OperationsPelicanService{repo: repo, encryptor: operationsTestEncryptor{}, stores: func(context.Context, *BackupS3Config) (OperationsObjectStore, error) { return store, nil }}
		out, err := svc.CleanGroup(context.Background(), 2)
		require.NoError(t, err)
		require.Equal(t, 1, out.Detached)
		require.True(t, store.deleted)
		if fail {
			require.Equal(t, 1, out.Failed)
			require.False(t, repo.removed)
			require.True(t, repo.failed)
		} else {
			require.Equal(t, 1, out.Removed)
			require.True(t, repo.removed)
		}
	}
}
