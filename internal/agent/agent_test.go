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
		}
	}
	if !sawCall || !sawResult || !sawDone {
		t.Errorf("events incomplete: call=%v result=%v done=%v", sawCall, sawResult, sawDone)
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
