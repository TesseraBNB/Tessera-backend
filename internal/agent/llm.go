package agent

import (
	"context"

	"github.com/yeheskieltame/tessera/internal/analysis"
)

// llmAdapter adapts a Client to analysis.LLM so the legacy one-shot qualitative
// evaluators (EvaluateProject, DeepEvaluateProject, ScanProposal, …) can run on
// top of the same Hermes/Anthropic transport without the analysis package
// importing agent.
type llmAdapter struct{ c *Client }

func (a llmAdapter) Complete(ctx context.Context, prompt, system string) (*analysis.LLMResponse, error) {
	r, err := a.c.Complete(ctx, prompt, system)
	if err != nil {
		return nil, err
	}
	return &analysis.LLMResponse{Text: r.Text, Model: r.Model, Provider: r.Provider}, nil
}

// AsLLM returns an analysis.LLM backed by this client.
func (c *Client) AsLLM() analysis.LLM { return llmAdapter{c} }
