package analysis

import "context"

// LLMResponse is the result of a one-shot language-model completion.
type LLMResponse struct {
	Text           string
	Model          string
	Provider       string
	Fallback       bool
	FallbackReason string
}

// LLM is the minimal language-model interface the qualitative evaluators need.
// It is satisfied by *agent.Client through its AsLLM adapter, which keeps this
// package free of any dependency on the agent/transport layer (avoiding an
// import cycle, since agent imports analysis for its tools).
type LLM interface {
	Complete(ctx context.Context, prompt, system string) (*LLMResponse, error)
}
