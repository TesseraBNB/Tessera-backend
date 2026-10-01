package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yeheskieltame/tessera/internal/config"
)

func testClient(url string) *Client {
	return New(&config.Config{
		HermesBaseURL:      url,
		Model:              "claude-opus-4-8",
		AgentMaxIterations: 5,
	})
}

func TestCompleteParsesText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"model":"claude-opus-4-8","role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"hello world"}]}`)
	}))
	defer srv.Close()

	resp, err := testClient(srv.URL).Complete(context.Background(), "hi", "")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "hello world" {
		t.Errorf("text = %q, want %q", resp.Text, "hello world")
	}
	if resp.Provider != "hermes" {
		t.Errorf("provider = %q, want hermes", resp.Provider)
	}
}

func TestRunExecutesToolLoop(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch atomic.AddInt32(&calls, 1) {
		case 1:
			// First turn: the model asks to call a tool.
			_, _ = io.WriteString(w, `{"stop_reason":"tool_use","role":"assistant","content":[{"type":"text","text":"checking"},{"type":"tool_use","id":"t1","name":"ping","input":{}}]}`)
		default:
			// Second turn must carry the tool_result back.
			s := string(body)
			if !strings.Contains(s, "tool_result") || !strings.Contains(s, "pong") {
				t.Errorf("second request missing tool_result/pong: %s", s)
			}
			_, _ = io.WriteString(w, `{"stop_reason":"end_turn","role":"assistant","content":[{"type":"text","text":"final: pong observed"}]}`)
		}
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	// Override the registry with a deterministic, network-free tool.
	c.reg = map[string]toolDef{
		"ping": {
			tool: Tool{Name: "ping", Description: "ping", InputSchema: schemaEmpty},
			exec: func(_ context.Context, _ json.RawMessage) (string, error) { return "pong", nil },
		},
	}

	var events []Event
	md, err := c.Run(context.Background(), "system", "do it", func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if md != "final: pong observed" {
		t.Errorf("final = %q", md)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("expected 2 model calls, got %d", calls)
	}

	var sawCall, sawResult, sawDone bool
	for _, e := range events {
		switch e.Type {
		case "tool_call":
			if e.Tool == "ping" {
				sawCall = true
			}
		case "tool_result":
			if strings.Contains(e.Result, "pong") {
				sawResult = true
			}
		case "done":
			sawDone = true
			if e.Provider != "hermes" || e.Model != "claude-opus-4-8" {
				t.Errorf("done event provider/model = %q/%q, want hermes/claude-opus-4-8", e.Provider, e.Model)
			}
		}
	}
	if !sawCall || !sawResult || !sawDone {
		t.Errorf("events incomplete: call=%v result=%v done=%v", sawCall, sawResult, sawDone)
	}
}

func TestFallbackProviderUsesOwnModel(t *testing.T) {
	var primaryModel, fallbackModel string
	modelOf := func(r *http.Request) string {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		return body.Model
	}
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryModel = modelOf(r)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "fb-key" {
			t.Errorf("fallback x-api-key = %q", r.Header.Get("x-api-key"))
		}
		fallbackModel = modelOf(r)
		_, _ = io.WriteString(w, `{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"ok"}]}`)
	}))
	defer fallback.Close()

	c := New(&config.Config{
		AnthropicBaseURL: primary.URL,
		AnthropicAPIKey:  "primary-key",
		Model:            "primary-model",
		FallbackBaseURL:  fallback.URL,
		FallbackAPIKey:   "fb-key",
		FallbackModel:    "fallback-model",
	})
	resp, err := c.Complete(context.Background(), "hi", "")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if primaryModel != "primary-model" || fallbackModel != "fallback-model" {
		t.Errorf("models sent: primary=%q fallback=%q", primaryModel, fallbackModel)
	}
	if want := strings.TrimPrefix(fallback.URL, "http://"); resp.Provider != want {
		t.Errorf("provider = %q, want fallback host %q", resp.Provider, want)
	}
}

func TestRunSendsThinkingBlocksBack(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if atomic.AddInt32(&calls, 1) == 1 {
			_, _ = io.WriteString(w, `{"stop_reason":"tool_use","role":"assistant","content":[{"type":"thinking","thinking":"need the epoch","signature":"sig-123"},{"type":"tool_use","id":"t1","name":"ping","input":{}}]}`)
			return
		}
		if s := string(body); !strings.Contains(s, `"thinking":"need the epoch"`) || !strings.Contains(s, `"signature":"sig-123"`) {
			t.Errorf("second request dropped the thinking block: %s", s)
		}
		_, _ = io.WriteString(w, `{"stop_reason":"end_turn","role":"assistant","content":[{"type":"text","text":"done"}]}`)
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	c.reg = map[string]toolDef{
		"ping": {
			tool: Tool{Name: "ping", Description: "ping", InputSchema: schemaEmpty},
			exec: func(_ context.Context, _ json.RawMessage) (string, error) { return "pong", nil },
		},
	}
	if _, err := c.Run(context.Background(), "system", "do it", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestTrimToReport(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"drops narration", "Let me check the tools first.\nWait, no name.\n\n# Evaluation\nHold", "# Evaluation\nHold"},
		{"subheading first", "Planning...\n## Summary verdict\nFund", "## Summary verdict\nFund"},
		{"already clean", "# Report\nbody", "# Report\nbody"},
		{"no heading kept", "2 + 2 = 4.", "2 + 2 = 4."},
		{"hashtag is not a heading", "#octant rocks\nplain text", "#octant rocks\nplain text"},
	}
	for _, tc := range cases {
		if got := trimToReport(tc.in); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestAnalyzeProjectReturnsReportOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "Your final message must contain only the report") {
			t.Errorf("task prompt lacks the report-only instruction")
		}
		_, _ = io.WriteString(w, `{"stop_reason":"end_turn","role":"assistant","content":[{"type":"text","text":"I have gathered the data.\n\n# Evaluation\nHold"}]}`)
	}))
	defer srv.Close()

	md, err := testClient(srv.URL).AnalyzeProject(context.Background(), "0xabc", nil)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	if md != "# Evaluation\nHold" {
		t.Errorf("report = %q", md)
	}
}

func TestNoBackendErrors(t *testing.T) {
	c := New(&config.Config{Model: "x", AgentMaxIterations: 3})
	if c.HasBackend() {
		t.Fatal("expected no backend")
	}
	if _, err := c.Complete(context.Background(), "hi", ""); err == nil {
		t.Error("expected error when no backend configured")
	}
}
