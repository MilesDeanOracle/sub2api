package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type operationsHTTPStub struct {
	HTTPUpstream
	request func(*http.Request, string) (*http.Response, error)
}

func (s *operationsHTTPStub) DoWithTLS(req *http.Request, proxy string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.request(req, proxy)
}

func operationsTestService(upstream HTTPUpstream) *AccountTestService {
	return &AccountTestService{httpUpstream: upstream, cfg: &config.Config{
		Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{AllowInsecureHTTP: true}},
	}}
}

func TestOperationsBaseURL(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{" https://EXAMPLE.com:443/v1/// ", "https://example.com"},
		{"http://example.com:80/tenant/v1/", "http://example.com/tenant"},
		{"https://example.com/Tenant/", "https://example.com/Tenant"},
		{"https://example.com:8443/v1", "https://example.com:8443"},
		{"https://example.com/v1?secret=hidden#fragment", "https://example.com"},
		{"https://user:secret@example.com/v1", ""}, {"not-a-url", ""}, {"file:///tmp", ""},
	} {
		t.Run(tc.input, func(t *testing.T) { require.Equal(t, tc.want, operationsBaseURL(tc.input)) })
	}
}

func TestOperationsBalancesDeduplicateAndUseRemoteCredentials(t *testing.T) {
	var mu sync.Mutex
	requests := map[string]string{}
	calls := 0
	svc := operationsTestService(&operationsHTTPStub{request: func(req *http.Request, proxy string) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		require.True(t, HTTPUpstreamRedirectsDisabled(req.Context()))
		require.Empty(t, proxy)
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "application/json", req.Header.Get("Accept"))
		requests[req.URL.String()] = req.Header.Get("Authorization")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"balance":12.34,"remaining":999,"unit":"USD","isValid":true}`))}, nil
	}})
	accounts := []Account{
		{ID: 1, Status: StatusDisabled, Credentials: map[string]any{"base_url": "https://EXAMPLE.com/v1/", "api_key": "disabled-key"}},
		{ID: 3, Status: StatusActive, Name: "selected", Credentials: map[string]any{"base_url": "https://example.com", "api_key": "real-key"}},
		{ID: 4, Status: StatusActive, Credentials: map[string]any{"base_url": "https://example.com:443/", "api_key": "different-key"}},
		{ID: 5, Status: StatusActive, Credentials: map[string]any{"base_url": "https://example.com/tenant/v1", "api_key": "tenant-key"}},
		{ID: 6, Credentials: map[string]any{"api_key": "missing-url"}},
		{ID: 7, Credentials: map[string]any{"base_url": "https://missing-key.com"}},
	}
	rows := svc.GetOperationsBalances(context.Background(), accounts)
	require.Len(t, rows, 2)
	require.Equal(t, 2, calls)
	require.Equal(t, map[string]string{
		"https://example.com/v1/usage": "Bearer real-key", "https://example.com/tenant/v1/usage": "Bearer tenant-key",
	}, requests)
	require.EqualValues(t, 3, rows[0].AccountID)
	require.Equal(t, 3, rows[0].AccountCount)
	require.Equal(t, "selected", rows[0].AccountName)
	require.Equal(t, "ok", rows[0].Status)
	require.Equal(t, "wallet", rows[0].Kind)
	require.InDelta(t, 12.34, *rows[0].Balance, 0.00001)
	require.NotNil(t, rows[0].QueriedAt)
}

func TestOperationsBalanceResponseVariants(t *testing.T) {
	for _, tc := range []struct {
		name, body, status, kind, errorCode string
		httpStatus                          int
		balance                             *float64
	}{
		{name: "zero", body: `{"balance":0}`, status: "ok", kind: "wallet", httpStatus: 200, balance: operationsFloat(0)},
		{name: "negative", body: `{"balance":-3}`, status: "ok", kind: "wallet", httpStatus: 200, balance: operationsFloat(-3)},
		{name: "quota", body: `{"remaining":7,"quota":{"limit":10}}`, status: "ok", kind: "quota", httpStatus: 200, balance: operationsFloat(7)},
		{name: "subscription", body: `{"remaining":2,"subscription":{}}`, status: "ok", kind: "subscription", httpStatus: 200, balance: operationsFloat(2)},
		{name: "no wallet field", body: `{"remaining":5}`, status: "ok", kind: "remaining", httpStatus: 200, balance: operationsFloat(5)},
		{name: "rate only", body: `{"mode":"quota_limited","rate_limits":[]}`, status: "unsupported", httpStatus: 200, errorCode: "no_balance"},
		{name: "null", body: `null`, status: "unsupported", httpStatus: 200, errorCode: "no_balance"},
		{name: "invalid json", body: `<html>secret</html>`, status: "failed", httpStatus: 200, errorCode: "invalid_response"},
		{name: "invalid number", body: `{"balance":"secret"}`, status: "failed", httpStatus: 200, errorCode: "invalid_response"},
		{name: "invalid key", body: `{"isValid":false,"balance":1}`, status: "failed", httpStatus: 200, errorCode: "invalid_api_key"},
		{name: "unauthorized", body: `secret`, status: "failed", httpStatus: 401, errorCode: "http_error"},
		{name: "redirect", body: `secret`, status: "failed", httpStatus: 302, errorCode: "http_error"},
		{name: "oversized", body: strings.Repeat("x", 1024*1024+1), status: "failed", httpStatus: 200, errorCode: "invalid_response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := operationsTestService(&operationsHTTPStub{request: func(*http.Request, string) (*http.Response, error) {
				return &http.Response{StatusCode: tc.httpStatus, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			}})
			rows := svc.GetOperationsBalances(context.Background(), []Account{{Credentials: map[string]any{"base_url": "https://example.com", "api_key": "secret"}}})
			require.Len(t, rows, 1)
			require.Equal(t, tc.status, rows[0].Status)
			require.Equal(t, tc.errorCode, rows[0].Error)
			if tc.balance == nil {
				require.Nil(t, rows[0].Balance)
			} else {
				require.Equal(t, *tc.balance, *rows[0].Balance)
				require.Equal(t, tc.kind, rows[0].Kind)
			}
		})
	}
}

func operationsFloat(v float64) *float64 { return &v }

func TestOperationsBalancesUnavailableTransport(t *testing.T) {
	var svc *AccountTestService
	rows := svc.GetOperationsBalances(context.Background(), []Account{{Credentials: map[string]any{"base_url": "https://example.com", "api_key": "secret"}}})
	require.Equal(t, "transport_unavailable", rows[0].Error)
	require.Nil(t, rows[0].Balance)
}

func TestOperationsBalancesHonorsCancellationAndURLPolicy(t *testing.T) {
	called := false
	svc := operationsTestService(&operationsHTTPStub{request: func(*http.Request, string) (*http.Response, error) {
		called = true
		return nil, nil
	}})
	accounts := []Account{{Credentials: map[string]any{"base_url": "https://example.com", "api_key": "secret"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rows := svc.GetOperationsBalances(ctx, accounts)
	require.Equal(t, "request_failed", rows[0].Error)
	require.False(t, called)
	svc.cfg.Security.URLAllowlist.Enabled = true
	svc.cfg.Security.URLAllowlist.UpstreamHosts = []string{"allowed.example"}
	rows = svc.GetOperationsBalances(context.Background(), accounts)
	require.Equal(t, "invalid_base_url", rows[0].Error)
	require.False(t, called)
}

func TestOperationsBalancesUsesConfiguredProxy(t *testing.T) {
	proxyID := int64(8)
	svc := operationsTestService(&operationsHTTPStub{request: func(req *http.Request, proxy string) (*http.Response, error) {
		require.Equal(t, "http://proxy.example:8080", proxy)
		require.True(t, HTTPUpstreamRedirectsDisabled(req.Context()))
		_, hasDeadline := req.Context().Deadline()
		require.True(t, hasDeadline)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"balance":3}`))}, nil
	}})
	account := Account{ProxyID: &proxyID, Proxy: &Proxy{ID: proxyID, Protocol: "http", Host: "proxy.example", Port: 8080}, Credentials: map[string]any{"base_url": "https://example.com", "api_key": "secret"}}
	rows := svc.GetOperationsBalances(context.Background(), []Account{account})
	require.Equal(t, "ok", rows[0].Status)
	account.Proxy = nil
	rows = svc.GetOperationsBalances(context.Background(), []Account{account})
	require.Equal(t, "proxy_unavailable", rows[0].Error)
	require.Nil(t, rows[0].Balance)
}
