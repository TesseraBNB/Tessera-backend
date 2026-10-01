package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yeheskieltame/tessera/internal/config"
	"github.com/yeheskieltame/tessera/internal/data"
)

const (
	maxTokens        = 8192
	anthropicVersion = "2023-06-01"
	anthropicBaseURL = "https://api.anthropic.com"
)

var errNoBackend = errors.New("no AI backend configured (set HERMES_BASE_URL, ANTHROPIC_API_KEY, or FALLBACK_BASE_URL + FALLBACK_API_KEY)")

// Client drives the Anthropic Messages API through Hermes (primary), then the
// Anthropic API or any Messages-compatible provider, then an optional second
// compatible provider, and owns the in-process tool-calling loop.
type Client struct {
	cfg  *config.Config
	http *http.Client
	reg  map[string]toolDef

	// data clients used by tool executors
	octant     *data.OctantClient
	blockchain *data.BlockchainClient
	oso        *data.OSOClient
	github     *data.GitHubClient
	discourse  *data.DiscourseClient
	retropgf   *data.RetroPGFClient
}

// New constructs a Client from configuration.
func New(cfg *config.Config) *Client {
	c := &Client{
		cfg:        cfg,
		http:       &http.Client{Timeout: 5 * time.Minute},
		octant:     data.NewOctantClient(),
		blockchain: data.NewBlockchainClient(),
		oso:        data.NewOSOClient(),
		github:     data.NewGitHubClient(),
		discourse:  data.NewOctantDiscourseClient(),
		retropgf:   data.NewRetroPGFClient(),
	}
	c.reg = c.buildRegistry()
	return c
}

type backend struct {
	name    string
	url     string
	headers map[string]string
	model   string // overrides the request model when set
}

// backends returns the configured Messages API endpoints in fallback order:
// Hermes first (when set), then the Anthropic API or a compatible provider at
// ANTHROPIC_BASE_URL, then the optional fallback provider.
func (c *Client) backends() []backend {
	var bs []backend
	if c.cfg.HermesBaseURL != "" {
		h := map[string]string{"content-type": "application/json", "anthropic-version": anthropicVersion}
		if c.cfg.HermesToken != "" {
			h["authorization"] = "Bearer " + c.cfg.HermesToken
		}
		bs = append(bs, backend{name: "hermes", url: c.cfg.HermesBaseURL + "/v1/messages", headers: h})
	}
	if c.cfg.AnthropicAPIKey != "" {
		bs = append(bs, messagesBackend(orDefault(c.cfg.AnthropicBaseURL, anthropicBaseURL), c.cfg.AnthropicAPIKey, ""))
	}
	if c.cfg.FallbackBaseURL != "" && c.cfg.FallbackAPIKey != "" {
		bs = append(bs, messagesBackend(c.cfg.FallbackBaseURL, c.cfg.FallbackAPIKey, c.cfg.FallbackModel))
	}
	return bs
}

// messagesBackend builds an x-api-key backend for the Anthropic API or a
// Messages-compatible provider. Non-Anthropic providers are named by host so
// status and reports show which provider actually served the run.
func messagesBackend(base, apiKey, model string) backend {
	name := "anthropic"
	if base != anthropicBaseURL {
		if u, err := url.Parse(base); err == nil && u.Host != "" {
			name = u.Host
		}
	}
	return backend{name: name, url: base + "/v1/messages", model: model, headers: map[string]string{
		"content-type":      "application/json",
		"x-api-key":         apiKey,
		"anthropic-version": anthropicVersion,
	}}
}

// HasBackend reports whether at least one AI backend is configured.
func (c *Client) HasBackend() bool { return len(c.backends()) > 0 }

// Backends returns the names of configured backends in fallback order.
func (c *Client) Backends() []string {
	bs := c.backends()
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.name
	}
	return out
}

// Model returns the configured model id.
func (c *Client) Model() string { return c.cfg.Model }

// sendMessages posts a single Messages request, trying each backend in order
// and returning the first success along with the backend name used.
func (c *Client) sendMessages(ctx context.Context, req messagesRequest) (*messagesResponse, string, error) {
	bs := c.backends()
	if len(bs) == 0 {
		return nil, "", errNoBackend
	}
	var errs []string
	for _, b := range bs {
		r := req
		if b.model != "" {
			r.Model = b.model
		}
		body, err := json.Marshal(r)
		if err != nil {
			return nil, "", err
		}
		resp, err := c.postOne(ctx, b, body)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", b.name, err))
			continue
		}
		if resp.Model == "" {
			resp.Model = r.Model // not every provider echoes the model id
		}
		return resp, b.name, nil
	}
	return nil, "", fmt.Errorf("all AI backends failed: %s", strings.Join(errs, "; "))
}

func (c *Client) postOne(ctx context.Context, b backend, body []byte) (*messagesResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, b.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range b.headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 400))
	}
	var mr messagesResponse
	if err := json.Unmarshal(raw, &mr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &mr, nil
}

// Complete runs a one-shot completion with no tools.
func (c *Client) Complete(ctx context.Context, prompt, system string) (*Response, error) {
	resp, backendName, err := c.sendMessages(ctx, messagesRequest{
		Model:     c.cfg.Model,
		MaxTokens: maxTokens,
		System:    orDefault(system, AnalystSystem),
		Messages:  []Message{{Role: "user", Content: []ContentBlock{{Type: "text", Text: prompt}}}},
	})
	if err != nil {
		return nil, err
	}
	return &Response{Text: collectText(resp.Content), Model: resp.Model, Provider: backendName}, nil
}

// CompleteChat is Complete, reserved for low-latency interactive chat.
func (c *Client) CompleteChat(ctx context.Context, prompt, system string) (*Response, error) {
	return c.Complete(ctx, prompt, system)
}

// --- helpers ---

func collectText(blocks []ContentBlock) string {
	var b strings.Builder
	for _, blk := range blocks {
		if blk.Type == "text" {
			b.WriteString(blk.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func orDefault(s, d string) string {
	if strings.TrimSpace(s) != "" {
		return s
	}
	return d
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
