package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"

	"github.com/tidwall/gjson"
)

func operationsAccountTarget(a *Account, model string) (string, string, error) {
	if a == nil || (a.Type != AccountTypeAPIKey && a.Type != AccountTypeUpstream) || !a.IsActive() || strings.TrimSpace(a.GetCredential("api_key")) == "" || (a.ExpiresAt != nil && !a.ExpiresAt.After(time.Now())) {
		return "", "", errors.New("account must be active with a Base URL and API key")
	}
	if a.ProxyID != nil && (a.Proxy == nil || !a.Proxy.IsActive() || a.Proxy.IsExpired(time.Now())) {
		return "", "", errors.New("configured proxy unavailable")
	}
	if !a.IsOpenAI() && !a.IsAnthropic() && !a.IsMultiProtocolAPIKey() {
		return "", "", errors.New("account protocol is not supported")
	}
	base := strings.TrimSpace(a.GetCredential("base_url"))
	hasExplicit := base != ""
	if all, ok := a.Credentials["api_base_urls"].(map[string]any); ok {
		for _, v := range all {
			if str, ok := v.(string); ok && strings.TrimSpace(str) != "" {
				hasExplicit = true
			}
		}
	}
	if !hasExplicit {
		return "", "", errors.New("explicit Base URL required")
	}
	protocol := APIProtocolAnthropic
	if !a.IsAnthropic() {
		// Upstream passthrough accounts share Base URL/key protocol settings;
		// the gateway helper otherwise limits OpenAI capability checks to apikey.
		routing := *a
		routing.Type = AccountTypeAPIKey
		protocol = resolveUpstreamProtocol(&routing, "", a.GetMappedModel(model), nil)
	}
	if a.IsMultiProtocolAPIKey() {
		if protocol == APIProtocolAnthropic {
			base = a.GetAnthropicProtocolBaseURL()
		} else if a.IsAdaptiveAPIProtocol() {
			base = a.GetCNProtocolBaseURL(protocol)
		} else {
			base = a.GetOpenAIBaseURL()
		}
	}
	if base == "" {
		return "", "", errors.New("configured protocol has no Base URL")
	}
	return protocol, base, nil
}

// Discovery uses only live upstream data. In particular, configured model maps
// and static connectivity-test defaults are never treated as a discovered list.
func (s *OperationsPelicanService) operationsDiscoverModels(ctx context.Context, a *Account) ([]string, error) {
	p, base, e := operationsAccountTarget(a, "")
	if e != nil {
		return nil, e
	}
	base, e = s.tests.validateUpstreamBaseURL(base)
	if e != nil {
		return nil, operationsBad("Account Base URL rejected by upstream URL policy")
	}
	target := buildV1ModelsURL(base)
	ids := map[string]bool{}
	seenPages := map[string]bool{}
	for page := 0; page < 100; page++ {
		if seenPages[target] {
			return nil, operationsBad("Upstream model pagination loop")
		}
		seenPages[target] = true
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if e != nil {
			return nil, e
		}
		operationsAuth(req, a, p, base)
		req.Header.Set("Accept", "application/json")
		resp, e := s.tests.doUpstreamModelsRequest(req, upstreamModelsProxyURL(a), a)
		if e != nil {
			return nil, operationsBad("Cannot request upstream model list")
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
		resp.Body.Close()
		if readErr != nil || len(raw) > 8<<20 {
			return nil, operationsBad("Upstream model list exceeds size limit")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, operationsBad(fmt.Sprintf("Upstream model list returned HTTP %d", resp.StatusCode))
		}
		if !gjson.ValidBytes(raw) {
			return nil, operationsBad("Invalid upstream model list")
		}
		data := gjson.GetBytes(raw, "data")
		if !data.IsArray() {
			return nil, operationsBad("Upstream did not return a model list")
		}
		for _, v := range data.Array() {
			id := strings.TrimSpace(v.Get("id").String())
			if id != "" {
				ids[id] = true
			}
		}
		if !gjson.GetBytes(raw, "has_more").Bool() {
			out := []string{}
			for id := range ids {
				out = append(out, id)
			}
			sort.Strings(out)
			if len(out) == 0 {
				return nil, operationsBad("Upstream returned no models")
			}
			return out, nil
		}
		last := gjson.GetBytes(raw, "last_id").String()
		if last == "" {
			return nil, operationsBad("Upstream model pagination cursor missing")
		}
		u, e := url.Parse(buildV1ModelsURL(base))
		if e != nil {
			return nil, e
		}
		q := u.Query()
		q.Set("after_id", last)
		q.Set("limit", "1000")
		u.RawQuery = q.Encode()
		target = u.String()
	}
	return nil, operationsBad("Upstream model pagination limit exceeded")
}
func operationsAuth(req *http.Request, a *Account, protocol, base string) {
	if protocol == APIProtocolAnthropic {
		req.Header.Set("anthropic-version", "2023-06-01")
		setAnthropicAPIKeyAuthHeader(req.Header, a, a.GetCredential("api_key"), base)
	} else {
		req.Header.Set("Authorization", "Bearer "+a.GetCredential("api_key"))
	}
	req.Header.Set("User-Agent", "sub2api-operations/1.0")
	a.ApplyHeaderOverrides(req.Header)
}

// Budget mapping is explicit and recorded, rather than forwarding OpenAI effort
// strings as Anthropic parameters. Unknown model capability is validated upstream.
func operationsReasoning(a *Account, protocol, model, effort string) (map[string]any, error) {
	out := map[string]any{}
	if effort == "" {
		return out, nil
	}
	if m, ok := a.GetUpstreamModelMetadata(model); ok {
		if m.Reasoning != nil && !*m.Reasoning && effort != "none" {
			return nil, errors.New("selected model does not support reasoning")
		}
		if len(m.SupportedReasoningLevels) > 0 {
			found := false
			for _, v := range m.SupportedReasoningLevels {
				found = found || effort == v
			}
			if !found {
				return nil, errors.New("selected effort is not supported by the model")
			}
		}
	}
	switch protocol {
	case APIProtocolChatCompletions:
		out["reasoning_effort"] = effort
	case APIProtocolResponses:
		out["reasoning"] = map[string]any{"effort": effort}
	case APIProtocolAnthropic:
		if effort == "none" {
			out["thinking"] = map[string]any{"type": "disabled"}
			raw, _ := json.Marshal(out)
			if validateClaude55Request(raw, model) != nil {
				return nil, errors.New("selected Claude model requires adaptive thinking and cannot disable reasoning")
			}
			return out, nil
		}
		if levels := claude.EffortLevelsForModel(model); len(levels) > 0 && !strings.Contains(strings.ToLower(model), "opus-4-5") {
			accepted := false
			for _, level := range levels {
				accepted = accepted || effort == level
			}
			if !accepted {
				return nil, errors.New("selected effort is not supported by this Claude model")
			}
			out["thinking"] = map[string]any{"type": "adaptive"}
			out["output_config"] = map[string]any{"effort": effort}
			return out, nil
		}
		budgets := map[string]int{"minimal": 1024, "low": 2048, "medium": 4096, "high": 8192, "xhigh": 16384, "max": 24576}
		budget, ok := budgets[effort]
		if !ok {
			return nil, errors.New("invalid reasoning effort")
		}
		out["thinking"] = map[string]any{"type": "enabled", "budget_tokens": budget}
	default:
		return nil, errors.New("unsupported upstream protocol")
	}
	return out, nil
}

func (s *OperationsPelicanService) generate(ctx context.Context, a *Account, r *OperationsResult) (string, error) {
	protocol, base, e := operationsAccountTarget(a, r.ModelID)
	if e != nil {
		return "", e
	}
	base, e = s.tests.validateUpstreamBaseURL(base)
	if e != nil {
		return "", errors.New("Account Base URL rejected by upstream URL policy")
	}
	payload := map[string]any{"model": r.UpstreamModelID, "stream": true}
	target := ""
	switch protocol {
	case APIProtocolAnthropic:
		// The shared builder accepts a root URL; normalize an explicit /v1 suffix.
		target = buildAnthropicMessagesURL(strings.TrimSuffix(base, "/v1"))
		payload["messages"] = []map[string]any{{"role": "user", "content": r.ConfigSnapshot.Prompt}}
		payload["max_tokens"] = 32768
	case APIProtocolResponses:
		target = buildOpenAIResponsesURLForPlatform(a.Platform, base)
		payload["input"] = r.ConfigSnapshot.Prompt
		payload["max_output_tokens"] = 32768
	case APIProtocolChatCompletions:
		target = buildOpenAIChatCompletionsURL(base)
		payload["messages"] = []map[string]any{{"role": "user", "content": r.ConfigSnapshot.Prompt}}
		payload["stream_options"] = map[string]any{"include_usage": true}
	default:
		return "", errors.New("unsupported upstream protocol")
	}
	for k, v := range r.EffectiveReasoning {
		payload[k] = v
	}
	raw, e := json.Marshal(payload)
	if e != nil {
		return "", e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(raw))
	if e != nil {
		return "", e
	}
	operationsAuth(req, a, protocol, base)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	now := time.Now().UTC()
	r.RequestStartedAt = &now
	resp, e := s.tests.doUpstreamModelsRequest(req, upstreamModelsProxyURL(a), a)
	if e != nil {
		if ctx.Err() != nil {
			return "", errors.New("generation timed out or was interrupted")
		}
		return "", errors.New("cannot connect to upstream")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}
	return operationsReadStream(resp.Body, protocol, r, now)
}

func operationsReadStream(reader io.Reader, protocol string, r *OperationsResult, start time.Time) (string, error) {
	var text strings.Builder
	complete := false
	validStop := false
	usageComplete := false
	streamError := ""
	var eventData strings.Builder
	r.UsageMetadata = map[string]any{"source": "upstream", "output_tokens_definition": "provider_reported_may_include_reasoning"}
	mark := func(reasoning bool, delta string) {
		if delta == "" {
			return
		}
		ms := time.Since(start).Milliseconds()
		if reasoning {
			if r.FirstReasoningMs == nil {
				v := ms
				r.FirstReasoningMs = &v
			}
			return
		}
		if r.FirstTextMs == nil {
			v := ms
			r.FirstTextMs = &v
		}
		v := ms
		r.LastTextMs = &v
		text.WriteString(delta)
	}
	usage := func(v gjson.Result, finalOutput bool) {
		if !v.Exists() {
			return
		}
		set := func(path string, dst **int64) {
			x := v.Get(path)
			if x.Exists() && x.Type == gjson.Number {
				n := x.Int()
				if n >= 0 {
					*dst = &n
				}
			}
		}
		set("prompt_tokens", &r.InputTokens)
		set("input_tokens", &r.InputTokens)
		set("completion_tokens", &r.OutputTokens)
		set("output_tokens", &r.OutputTokens)
		set("completion_tokens_details.reasoning_tokens", &r.ReasoningTokens)
		set("output_tokens_details.reasoning_tokens", &r.ReasoningTokens)
		if finalOutput && (v.Get("output_tokens").Exists() || v.Get("completion_tokens").Exists()) && r.OutputTokens != nil {
			usageComplete = true
		}
	}
	consume := func() {
		raw := strings.TrimSpace(eventData.String())
		eventData.Reset()
		if raw == "" {
			return
		}
		if raw == "[DONE]" {
			if protocol == APIProtocolChatCompletions {
				complete = true
			}
			return
		}
		if !gjson.Valid(raw) {
			streamError = "invalid upstream stream JSON"
			return
		}
		v := gjson.Parse(raw)
		if v.Get("error").Exists() || v.Get("type").String() == "error" {
			streamError = "upstream stream reported an error"
			return
		}
		switch protocol {
		case APIProtocolChatCompletions:
			c := v.Get("choices.0")
			d := c.Get("delta")
			mark(true, d.Get("reasoning_content").String())
			mark(true, d.Get("reasoning").String())
			mark(false, d.Get("content").String())
			reason := c.Get("finish_reason").String()
			if reason != "" {
				validStop = reason == "stop"
				if !validStop {
					streamError = "upstream generation did not finish normally (truncated, filtered or tool output)"
				}
			}
			usage(v.Get("usage"), true)
		case APIProtocolResponses:
			kind := v.Get("type").String()
			switch kind {
			case "response.output_text.delta":
				mark(false, v.Get("delta").String())
			case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
				mark(true, v.Get("delta").String())
				r.UsageMetadata["reasoning_observation"] = "visible_reasoning_or_summary"
			case "response.completed":
				complete = true
				validStop = v.Get("response.status").String() == "completed"
				usage(v.Get("response.usage"), true)
			case "response.failed", "response.incomplete":
				streamError = "upstream response failed or was truncated"
			}
		case APIProtocolAnthropic:
			switch v.Get("type").String() {
			case "message_start":
				usage(v.Get("message.usage"), false)
			case "content_block_start":
				b := v.Get("content_block")
				if b.Get("type").String() == "thinking" {
					mark(true, b.Get("thinking").String())
				} else if b.Get("type").String() == "text" {
					mark(false, b.Get("text").String())
				}
			case "content_block_delta":
				d := v.Get("delta")
				if d.Get("type").String() == "thinking_delta" {
					mark(true, d.Get("thinking").String())
				} else if d.Get("type").String() == "text_delta" {
					mark(false, d.Get("text").String())
				}
			case "message_delta":
				reason := v.Get("delta.stop_reason").String()
				validStop = reason == "end_turn" || reason == "stop_sequence"
				if reason != "" && !validStop {
					streamError = "upstream generation did not finish normally (truncated, filtered or tool output)"
				}
				usage(v.Get("usage"), true)
			case "message_stop":
				complete = true
			}
		}
	}
	// Bound total wire data as well as the extracted HTML; reasoning can be larger.
	scanner := bufio.NewScanner(io.LimitReader(reader, (32<<20)+1))
	scanner.Buffer(make([]byte, 64<<10), OperationsPelicanMaxHTML+1)
	wireBytes := 0
	for scanner.Scan() {
		line := scanner.Text()
		wireBytes += len(line) + 1
		if wireBytes > 32<<20 {
			return "", errors.New("upstream stream exceeds size limit")
		}
		if line == "" {
			consume()
		} else if strings.HasPrefix(line, "data:") {
			if eventData.Len() > 0 {
				eventData.WriteByte('\n')
			}
			eventData.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		if text.Len() > OperationsPelicanMaxHTML {
			return "", errors.New("generated HTML exceeds size limit")
		}
		if streamError != "" {
			break
		}
	}
	consume()
	duration := time.Since(start).Milliseconds()
	r.UpstreamDurationMs = &duration
	r.TTFTMs = r.FirstReasoningMs
	if r.TTFTMs == nil {
		r.TTFTMs = r.FirstTextMs
	}
	r.UsageMetadata["complete"] = complete && validStop && usageComplete
	if e := scanner.Err(); e != nil {
		return "", errors.New("upstream stream interrupted or exceeded event size limit")
	}
	if streamError != "" {
		return "", errors.New(streamError)
	}
	if !complete || !validStop {
		return "", errors.New("upstream stream ended without a successful completion")
	}
	if usageComplete && r.OutputTokens != nil && r.FirstTextMs != nil && r.LastTextMs != nil && *r.LastTextMs > *r.FirstTextMs {
		tps := float64(*r.OutputTokens) * 1000 / float64(*r.LastTextMs-*r.FirstTextMs)
		r.TPS = &tps
	}
	return operationsExtractHTML(text.String())
}
func operationsExtractHTML(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	lower := strings.ToLower(value)
	start := strings.Index(lower, "<!doctype")
	if start < 0 {
		start = strings.Index(lower, "<html")
	}
	end := strings.LastIndex(lower, "</html>")
	if start < 0 || end < start {
		return "", errors.New("upstream did not return a complete HTML document")
	}
	html := value[start : end+len("</html>")]
	if len(html) > OperationsPelicanMaxHTML {
		return "", errors.New("generated HTML exceeds size limit")
	}
	return html, nil
}
