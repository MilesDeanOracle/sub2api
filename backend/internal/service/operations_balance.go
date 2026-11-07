package service

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"golang.org/x/sync/errgroup"
)

// The existing upstream billing probe reads and persists rate multipliers.
// Operations needs a read-only /v1/usage query instead, reusing its transport.
// OperationsBalance contains only remote usage data, never account credentials
// or the locally tracked quota/balance. One representative key is queried per URL.
type OperationsBalance struct {
	BaseURL      string     `json:"base_url"`
	AccountID    int64      `json:"account_id"`
	AccountName  string     `json:"account_name"`
	AccountCount int        `json:"account_count"`
	Balance      *float64   `json:"balance"`
	Unit         string     `json:"unit"`
	Kind         string     `json:"kind"`
	Status       string     `json:"status"`
	Error        string     `json:"error,omitempty"`
	HTTPStatus   int        `json:"http_status,omitempty"`
	QueriedAt    *time.Time `json:"queried_at,omitempty"`
}

// Root and /v1 are equivalent gateway configurations. Keep other path prefixes
// distinct because different tenants can be hosted below the same domain.
func operationsBaseURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return ""
	}
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "https" && u.Port() == "443") || (u.Scheme == "http" && u.Port() == "80") {
		u.Host = strings.TrimSuffix(u.Host, ":"+u.Port())
	}
	u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/v1")
	u.RawPath, u.RawQuery, u.Fragment = "", "", ""
	return strings.TrimRight(u.String(), "/")
}

func (s *AccountTestService) GetOperationsBalances(ctx context.Context, accounts []Account) []OperationsBalance {
	// Prefer an active account, then the smallest ID, for stable deduplication.
	accounts = append([]Account(nil), accounts...)
	sort.SliceStable(accounts, func(i, j int) bool {
		if (accounts[i].Status == StatusActive) != (accounts[j].Status == StatusActive) {
			return accounts[i].Status == StatusActive
		}
		return accounts[i].ID < accounts[j].ID
	})
	results := make([]OperationsBalance, 0)
	representatives := make([]Account, 0)
	indices := make(map[string]int)
	for _, account := range accounts {
		base := operationsBaseURL(account.GetCredential("base_url"))
		if base == "" || strings.TrimSpace(account.GetCredential("api_key")) == "" {
			continue
		}
		if i, ok := indices[base]; ok {
			results[i].AccountCount++
			continue
		}
		indices[base] = len(results)
		results = append(results, OperationsBalance{
			BaseURL: base, AccountID: account.ID, AccountName: account.Name,
			AccountCount: 1, Status: "failed", Unit: "USD", Kind: "unknown",
		})
		representatives = append(representatives, account)
	}
	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(4)
	for i := range results {
		g.Go(func() error {
			s.queryOperationsBalance(groupCtx, representatives[i], &results[i])
			return nil
		})
	}
	_ = g.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].BaseURL < results[j].BaseURL })
	return results
}

func (s *AccountTestService) queryOperationsBalance(ctx context.Context, account Account, result *OperationsBalance) {
	defer func() { now := time.Now().UTC(); result.QueriedAt = &now }()
	if ctx.Err() != nil {
		result.Error = "request_failed"
		return
	}
	if s == nil || s.httpUpstream == nil {
		result.Error = "transport_unavailable"
		return
	}
	base, err := s.validateUpstreamBaseURL(result.BaseURL)
	if err != nil {
		result.Error = "invalid_base_url"
		return
	}
	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil || account.Proxy.ID != *account.ProxyID {
			result.Error = "proxy_unavailable"
			return
		}
		proxyURL = account.Proxy.URL()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	transportProfile := HTTPUpstreamProfileDefault
	if account.Platform == PlatformOpenAI {
		transportProfile = HTTPUpstreamProfileOpenAI
	}
	requestCtx := WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(ctx, transportProfile))
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, buildOpenAIEndpointURL(base, "/v1/usage"), nil)
	if err != nil {
		result.Error = "invalid_base_url"
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.GetCredential("api_key")))
	var profile *tlsfingerprint.Profile
	if s.tlsFPProfileService != nil {
		profile = s.tlsFPProfileService.ResolveTLSProfile(&account)
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, profile)
	if err != nil {
		result.Error = "request_failed"
		return
	}
	if resp == nil || resp.Body == nil {
		result.Error = "invalid_response"
		return
	}
	defer resp.Body.Close()
	result.HTTPStatus = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		result.Error = "http_error"
		return
	}
	const maxBody = 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		result.Error = "invalid_response"
		return
	}
	var usage struct {
		Balance      *float64        `json:"balance"`
		Remaining    *float64        `json:"remaining"`
		Unit         string          `json:"unit"`
		Quota        json.RawMessage `json:"quota"`
		Subscription json.RawMessage `json:"subscription"`
		IsValid      *bool           `json:"isValid"`
	}
	if json.Unmarshal(body, &usage) != nil {
		result.Error = "invalid_response"
		return
	}
	if usage.IsValid != nil && !*usage.IsValid {
		result.Error = "invalid_api_key"
		return
	}
	result.Balance, result.Kind = usage.Balance, "wallet"
	if result.Balance == nil {
		result.Balance, result.Kind = usage.Remaining, "remaining"
		if len(usage.Quota) > 0 && string(usage.Quota) != "null" {
			result.Kind = "quota"
		} else if len(usage.Subscription) > 0 && string(usage.Subscription) != "null" {
			result.Kind = "subscription"
		}
	}
	if result.Balance == nil || math.IsNaN(*result.Balance) || math.IsInf(*result.Balance, 0) {
		result.Balance = nil
		result.Status, result.Error = "unsupported", "no_balance"
		return
	}
	if usage.Unit != "" {
		result.Unit = usage.Unit
	}
	result.Status = "ok"
}
