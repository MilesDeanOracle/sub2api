package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/gin-gonic/gin"
)

const (
	upstreamGroupProbeTimeout       = 12 * time.Second
	upstreamGroupProbeDialTimeout   = 5 * time.Second
	upstreamGroupProbeResponseLimit = 2 << 20
)

var (
	errUpstreamGroupProbeAuth = infraerrors.BadRequest(
		"UPSTREAM_GROUP_PROBE_AUTH_FAILED",
		"upstream authentication failed; check the JWT Access Token",
	)
	errUpstreamGroupProbeResponse = infraerrors.BadRequest(
		"UPSTREAM_GROUP_PROBE_RESPONSE_INVALID",
		"upstream returned an incompatible group response",
	)
)

type upstreamGroupProbeRequest struct {
	BaseURL     string `json:"base_url" binding:"required"`
	AccessToken string `json:"access_token" binding:"required"`
}

type upstreamGroupSummary struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Platform       string  `json:"platform"`
	Status         string  `json:"status"`
	RateMultiplier float64 `json:"rate_multiplier"`
	IsExclusive    bool    `json:"is_exclusive"`
	// APIKey is returned by compatible upstreams when a group exposes the
	// credential to administrators. It is optional because older upstreams do
	// not include it in /api/v1/groups/available.
	APIKey string `json:"api_key,omitempty"`
}

type upstreamAPIKeyEntry struct {
	Key       string `json:"key"`
	APIKey    string `json:"api_key"`
	Status    string `json:"status"`
	GroupName string `json:"group_name"`
	GroupID   *int64 `json:"group_id"`
	Group     *struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"group"`
}

type upstreamGroupProber interface {
	Probe(ctx context.Context, baseURL, accessToken string) ([]upstreamGroupSummary, error)
}

type httpUpstreamGroupProber struct {
	client           *http.Client
	normalizeBaseURL func(string) (string, error)
}

type upstreamGroupEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newHTTPUpstreamGroupProber(cfg *config.Config) upstreamGroupProber {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	allowPrivate := cfg != nil && cfg.Security.URLAllowlist.AllowPrivateHosts
	if !allowPrivate {
		transport.DialContext = upstreamGroupPublicDialContext
	}
	client := &http.Client{
		Timeout:   upstreamGroupProbeTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if len(via) > 0 && !sameURLOrigin(req.URL, via[0].URL) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return &httpUpstreamGroupProber{
		client: client,
		normalizeBaseURL: func(raw string) (string, error) {
			return normalizeUpstreamGroupBaseURL(cfg, raw)
		},
	}
}

// ProbeUpstreamGroups performs a read-only, one-shot inspection of a compatible
// Sub2API upstream. The supplied credentials are never persisted or logged.
func (h *GroupHandler) ProbeUpstreamGroups(c *gin.Context) {
	var req upstreamGroupProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest(
			"UPSTREAM_GROUP_PROBE_INVALID_REQUEST",
			"base_url and access_token are required",
		))
		return
	}
	if strings.TrimSpace(req.AccessToken) == "" {
		response.ErrorFrom(c, infraerrors.BadRequest(
			"UPSTREAM_GROUP_PROBE_INVALID_REQUEST",
			"base_url and access_token are required",
		))
		return
	}
	prober := h.upstreamGroupProber
	if prober == nil {
		prober = newHTTPUpstreamGroupProber(h.cfg)
	}
	groups, err := prober.Probe(c.Request.Context(), req.BaseURL, req.AccessToken)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"groups": groups, "total": len(groups)})
}

func (p *httpUpstreamGroupProber) Probe(ctx context.Context, rawBaseURL, accessToken string) ([]upstreamGroupSummary, error) {
	baseURL, err := p.normalizeBaseURL(rawBaseURL)
	if err != nil {
		return nil, infraerrors.BadRequest(
			"UPSTREAM_GROUP_PROBE_URL_INVALID",
			"invalid or disallowed upstream Base URL",
		).WithCause(err)
	}
	accessToken = strings.TrimSpace(accessToken)
	if strings.HasPrefix(strings.ToLower(accessToken), "bearer ") {
		accessToken = strings.TrimSpace(accessToken[len("bearer "):])
	}
	if accessToken == "" {
		return nil, infraerrors.BadRequest(
			"UPSTREAM_GROUP_PROBE_INVALID_REQUEST",
			"base_url and access_token are required",
		)
	}

	return p.fetchAvailableGroups(ctx, baseURL, accessToken)
}

func (p *httpUpstreamGroupProber) fetchAvailableGroups(ctx context.Context, baseURL, accessToken string) ([]upstreamGroupSummary, error) {
	target, err := url.JoinPath(baseURL, "/api/v1/groups/available")
	if err != nil {
		return nil, errUpstreamGroupProbeResponse.WithCause(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, errUpstreamGroupProbeResponse.WithCause(err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, infraerrors.BadRequest(
			"UPSTREAM_GROUP_PROBE_REQUEST_FAILED",
			"unable to reach the upstream service",
		).WithCause(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, errUpstreamGroupProbeAuth
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, infraerrors.BadRequest(
			"UPSTREAM_GROUP_PROBE_REQUEST_FAILED",
			fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode),
		)
	}

	body, err := readUpstreamGroupResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	var envelope upstreamGroupEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Code != 0 || len(envelope.Data) == 0 {
		return nil, errUpstreamGroupProbeResponse.WithCause(err)
	}
	var groups []upstreamGroupSummary
	if err := json.Unmarshal(envelope.Data, &groups); err != nil {
		return nil, errUpstreamGroupProbeResponse.WithCause(err)
	}
	for _, group := range groups {
		if group.ID <= 0 || strings.TrimSpace(group.Name) == "" || strings.TrimSpace(group.Platform) == "" {
			return nil, errUpstreamGroupProbeResponse
		}
	}

	// /groups/available intentionally exposes only group metadata. Fetch the
	// user's API keys separately and associate them by group_id so the admin UI
	// can avoid creating accounts with a placeholder credential.
	if keys, keyErr := p.fetchAPIKeys(ctx, baseURL, accessToken); keyErr == nil {
		keysByGroup := make(map[int64]string, len(keys))
		keysByName := make(map[string]string, len(keys))
		for _, key := range keys {
			if !isUsableUpstreamAPIKey(key) {
				continue
			}
			value := upstreamAPIKeyValue(key)
			if key.GroupID != nil {
				if _, exists := keysByGroup[*key.GroupID]; !exists {
					keysByGroup[*key.GroupID] = value
				}
			}
			if name := upstreamAPIKeyGroupName(key); name != "" {
				if _, exists := keysByName[name]; !exists {
					keysByName[name] = value
				}
			}
		}
		for i := range groups {
			if value := keysByGroup[groups[i].ID]; value != "" {
				groups[i].APIKey = value
				continue
			}
			groups[i].APIKey = keysByName[normalizeUpstreamGroupName(groups[i].Name)]
		}
	}
	return groups, nil
}

func upstreamAPIKeyGroupName(key upstreamAPIKeyEntry) string {
	if key.Group != nil && strings.TrimSpace(key.Group.Name) != "" {
		return normalizeUpstreamGroupName(key.Group.Name)
	}
	return normalizeUpstreamGroupName(key.GroupName)
}

func normalizeUpstreamGroupName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func (p *httpUpstreamGroupProber) fetchAPIKeys(ctx context.Context, baseURL, accessToken string) ([]upstreamAPIKeyEntry, error) {
	target, err := url.JoinPath(baseURL, "/api/v1/keys")
	if err != nil {
		return nil, errUpstreamGroupProbeResponse.WithCause(err)
	}
	query := target + "?page=1&page_size=1000"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, query, nil)
	if err != nil {
		return nil, errUpstreamGroupProbeResponse.WithCause(err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("upstream keys returned HTTP %d", resp.StatusCode)
	}
	body, err := readUpstreamGroupResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	var envelope upstreamGroupEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Code != 0 || len(envelope.Data) == 0 {
		return nil, errUpstreamGroupProbeResponse.WithCause(err)
	}
	return parseUpstreamAPIKeys(envelope.Data)
}

func parseUpstreamAPIKeys(data json.RawMessage) ([]upstreamAPIKeyEntry, error) {
	var entries []upstreamAPIKeyEntry
	if err := json.Unmarshal(data, &entries); err == nil {
		normalizeUpstreamAPIKeyGroupIDs(entries)
		return entries, nil
	}
	var page struct {
		Items []upstreamAPIKeyEntry `json:"items"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, errUpstreamGroupProbeResponse.WithCause(err)
	}
	normalizeUpstreamAPIKeyGroupIDs(page.Items)
	return page.Items, nil
}

func normalizeUpstreamAPIKeyGroupIDs(entries []upstreamAPIKeyEntry) {
	for i := range entries {
		if entries[i].GroupID == nil && entries[i].Group != nil && entries[i].Group.ID > 0 {
			id := entries[i].Group.ID
			entries[i].GroupID = &id
		}
	}
}

func upstreamAPIKeyValue(key upstreamAPIKeyEntry) string {
	if value := strings.TrimSpace(key.Key); value != "" {
		return value
	}
	return strings.TrimSpace(key.APIKey)
}

func isUsableUpstreamAPIKey(key upstreamAPIKeyEntry) bool {
	value := upstreamAPIKeyValue(key)
	if value == "" {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(key.Status))
	return status == "" || status == "active"
}

func readUpstreamGroupResponse(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, upstreamGroupProbeResponseLimit+1))
	if err != nil {
		return nil, errUpstreamGroupProbeResponse.WithCause(err)
	}
	if len(body) > upstreamGroupProbeResponseLimit {
		return nil, infraerrors.BadRequest(
			"UPSTREAM_GROUP_PROBE_RESPONSE_INVALID",
			"upstream response is too large",
		)
	}
	return body, nil
}

func normalizeUpstreamGroupBaseURL(cfg *config.Config, raw string) (string, error) {
	allowInsecureHTTP := cfg != nil && cfg.Security.URLAllowlist.AllowInsecureHTTP
	options := urlvalidator.ValidationOptions{}
	if cfg != nil {
		options.AllowPrivate = cfg.Security.URLAllowlist.AllowPrivateHosts
		if cfg.Security.URLAllowlist.Enabled {
			options.AllowedHosts = cfg.Security.URLAllowlist.UpstreamHosts
			options.RequireAllowlist = true
		}
	}
	normalized, err := urlvalidator.ValidateHTTPURL(raw, allowInsecureHTTP, options)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("base URL must not include user info, query, or fragment")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func sameURLOrigin(a, b *url.URL) bool {
	if a == nil || b == nil || !strings.EqualFold(a.Scheme, b.Scheme) {
		return false
	}
	return strings.EqualFold(canonicalURLHost(a), canonicalURLHost(b))
}

func canonicalURLHost(value *url.URL) string {
	port := value.Port()
	if port == "" {
		if strings.EqualFold(value.Scheme, "https") {
			port = "443"
		} else if strings.EqualFold(value.Scheme, "http") {
			port = "80"
		}
	}
	return net.JoinHostPort(strings.ToLower(value.Hostname()), port)
}

func upstreamGroupPublicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if urlvalidator.IsBlockedHost(host) {
		return nil, fmt.Errorf("target host is not allowed")
	}
	resolver := net.DefaultResolver
	addresses, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, errors.New("target host has no addresses")
	}
	dialer := &net.Dialer{Timeout: upstreamGroupProbeDialTimeout, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, candidate := range addresses {
		ip := candidate.IP
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			lastErr = fmt.Errorf("resolved target address is not allowed")
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = errors.New("target host has no usable addresses")
	}
	return nil, lastErr
}
