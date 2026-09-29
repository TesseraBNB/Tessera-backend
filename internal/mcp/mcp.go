// Package mcp exposes Tessera's tools over the Model Context Protocol so an
// external agent (e.g. Claude Code via Hermes) can call them. The same core
// Handle drives both the stdio and HTTP transports. No AI backend is required —
// this server only provides tools; the reasoning happens on the client side.
package mcp

import (
	"context"
	"encoding/json"

	"github.com/yeheskieltame/tessera/internal/agent"
)

const defaultProtocolVersion = "2024-11-05"

// ToolProvider supplies the tools the MCP server exposes. *agent.Client satisfies it.
type ToolProvider interface {
	Tools() []agent.Tool
	CallTool(ctx context.Context, name string, args json.RawMessage) (string, error)
}

// Server is a transport-agnostic MCP server over a ToolProvider.
type Server struct {
	provider ToolProvider
	name     string
	version  string
}

func NewServer(p ToolProvider) *Server {
	return &Server{provider: p, name: "tessera", version: "1.0.0"}
}

// --- JSON-RPC 2.0 ---

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- MCP shapes ---

type mcpTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Handle processes one JSON-RPC message and returns the marshaled response, or
// nil for a notification (which receives no response).
func (s *Server) Handle(ctx context.Context, raw []byte) []byte {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return s.marshal(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
	}
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		ver := p.ProtocolVersion
		if ver == "" {
			ver = defaultProtocolVersion
		}
		return s.ok(req.ID, map[string]any{
			"protocolVersion": ver,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.name, "version": s.version},
		})

	case "ping":
		return s.ok(req.ID, map[string]any{})

	case "tools/list":
		src := s.provider.Tools()
		tools := make([]mcpTool, len(src))
		for i, t := range src {
			tools[i] = mcpTool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema}
		}
		return s.ok(req.ID, map[string]any{"tools": tools})

	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return s.err(req.ID, -32602, "invalid params")
		}
		args := p.Arguments
		if len(args) == 0 {
			args = json.RawMessage("{}")
		}
		out, err := s.provider.CallTool(ctx, p.Name, args)
		if err != nil {
			return s.ok(req.ID, toolCallResult{Content: []textContent{{Type: "text", Text: err.Error()}}, IsError: true})
		}
		return s.ok(req.ID, toolCallResult{Content: []textContent{{Type: "text", Text: out}}})

	default:
		if isNotification {
			return nil // e.g. notifications/initialized
		}
		return s.err(req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s *Server) ok(id json.RawMessage, result any) []byte {
	return s.marshal(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) err(id json.RawMessage, code int, msg string) []byte {
	return s.marshal(rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func (s *Server) marshal(r rpcResponse) []byte {
	b, _ := json.Marshal(r)
	return b
}
