// Package agent implements Tessera's autonomous public-goods analyst: an
// in-process tool-calling loop over the Anthropic Messages API. The model
// decides which tools to call; Tessera executes them locally against its data
// and analysis layers and feeds the results back until a final answer is
// produced. Transport goes through Hermes (a relay) when configured, falling
// back to the Anthropic API directly.
package agent

import "encoding/json"

// Message is one turn in an Anthropic Messages API conversation.
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock is a single block within a message. The active fields depend on
// Type: "text", "tool_use", or "tool_result".
type ContentBlock struct {
	Type string `json:"type"`

	// Type == "text"
	Text string `json:"text,omitempty"`

	// Type == "tool_use"
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// Type == "tool_result"
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   any    `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// Tool is a tool definition advertised to the model.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Response is a one-shot completion result (no tools).
type Response struct {
	Text     string `json:"text"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

// Event is emitted during an agent run so callers (e.g. an SSE handler) can
// stream the agent's progress in real time.
type Event struct {
	Type    string          `json:"type"` // "text" | "tool_call" | "tool_result" | "done" | "error"
	Text    string          `json:"text,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
	Result  string          `json:"result,omitempty"`
	IsError bool            `json:"isError,omitempty"`
}

// EventFunc receives streaming events during an agent run. It is called
// synchronously from the run loop and must not block for long.
type EventFunc func(Event)

// --- internal Messages API wire shapes ---

type messagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
}

type messagesResponse struct {
	ID         string         `json:"id"`
	Model      string         `json:"model"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
}
