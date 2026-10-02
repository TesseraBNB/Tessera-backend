package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// finalTurnNudge goes back with the last round's tool results, so a model that
// would keep calling tools writes its answer from the evidence it already has.
const finalTurnNudge = "You have used every tool round available. Do not call any more tools: write your final answer now from the evidence gathered so far, and name any evidence you could not collect."

// Run drives the tool-calling loop: the model is given the tool set and a task,
// it decides which tools to call, Tessera executes them in-process and feeds the
// results back, repeating until the model produces a final answer (stop_reason
// is no longer "tool_use"). When AgentMaxIterations rounds are used up, one more
// turn asks for the answer with tool use switched off, so a run always ends in
// a report rather than an error. Streaming events are delivered via emit, which
// may be nil. The returned string is the model's final answer text.
func (c *Client) Run(ctx context.Context, system, task string, emit EventFunc) (string, error) {
	if emit == nil {
		emit = func(Event) {}
	}
	if !c.HasBackend() {
		return "", errNoBackend
	}

	tools := c.toolList()
	msgs := []Message{{Role: "user", Content: []ContentBlock{{Type: "text", Text: task}}}}

	for i := 0; i <= c.cfg.AgentMaxIterations; i++ {
		final := i == c.cfg.AgentMaxIterations
		req := messagesRequest{
			Model:     c.cfg.Model,
			MaxTokens: maxTokens,
			System:    orDefault(system, AnalystSystem),
			Messages:  msgs,
			Tools:     tools,
		}
		if final {
			req.ToolChoice = &ToolChoice{Type: "none"}
		}
		resp, backendName, err := c.sendMessages(ctx, req)
		if err != nil && final {
			// A provider without tool_choice support: rely on the nudge alone.
			req.ToolChoice = nil
			resp, backendName, err = c.sendMessages(ctx, req)
		}
		if err != nil {
			emit(Event{Type: "error", Text: err.Error()})
			return "", err
		}
		msgs = append(msgs, Message{Role: "assistant", Content: resp.Content})

		var toolResults []ContentBlock
		var turnText strings.Builder
		for _, blk := range resp.Content {
			switch blk.Type {
			case "text":
				if blk.Text != "" {
					emit(Event{Type: "text", Text: blk.Text})
					turnText.WriteString(blk.Text)
				}
			case "tool_use":
				if final {
					continue // out of rounds: never run more tools
				}
				emit(Event{Type: "tool_call", Tool: blk.Name, Input: blk.Input})
				out, terr := c.execTool(ctx, blk.Name, blk.Input)
				isErr := terr != nil
				if isErr {
					out = terr.Error()
				}
				emit(Event{Type: "tool_result", Tool: blk.Name, Result: out, IsError: isErr})
				toolResults = append(toolResults, ContentBlock{
					Type:      "tool_result",
					ToolUseID: blk.ID,
					Content:   out,
					IsError:   isErr,
				})
			}
		}

		// Final turn: no more tool calls requested.
		if resp.StopReason != "tool_use" {
			emit(Event{Type: "done", Provider: backendName, Model: resp.Model})
			return strings.TrimSpace(turnText.String()), nil
		}

		if final {
			break
		}
		// Feed tool results back for the next turn; after the last tool round,
		// ask for the answer.
		if i == c.cfg.AgentMaxIterations-1 {
			toolResults = append(toolResults, ContentBlock{Type: "text", Text: finalTurnNudge})
		}
		msgs = append(msgs, Message{Role: "user", Content: toolResults})
	}

	return "", fmt.Errorf("agent reached the max of %d tool-use rounds without finishing", c.cfg.AgentMaxIterations)
}

// AnalyzeProject runs the agent to produce a full intelligence report for an
// Octant project by address.
func (c *Client) AnalyzeProject(ctx context.Context, address string, emit EventFunc) (string, error) {
	task := fmt.Sprintf(`Produce a rigorous public-goods funding evaluation of the Octant project at address %s.

Investigate using your tools: pull its cross-epoch funding history, rank it against peers in its latest epoch, analyze its trust graph (donor diversity, whale dependency, coordination/Sybil risk), simulate how alternative funding mechanisms would change its allocation, and scan its on-chain activity. Cross-reference external signals (OSO, GitHub, forum, RetroPGF) when an identifier is available.

Then write the report in Markdown with these sections: Summary verdict (fund / hold / investigate, with a confidence level), Funding trajectory, Trust & Sybil assessment, Mechanism sensitivity, On-chain & ecosystem signals, Risks & red flags, and Evidence gaps. Ground every claim in tool data; never invent numbers.

`+reportOnly, address)
	md, err := c.Run(ctx, AnalystSystem, task, emit)
	return trimToReport(md), err
}

// reportOnly asks for a final message that is the report alone. Some models
// narrate their planning as plain text in the final turn.
const reportOnly = "Your final message must contain only the report: begin it with a top-level Markdown heading, with no planning, commentary, or tool narration before it."

var mdHeading = regexp.MustCompile(`(?m)^#{1,6} `)

// trimToReport drops any narration before the report's first Markdown heading
// (backstop for reportOnly). Text without a heading is returned unchanged.
func trimToReport(md string) string {
	if loc := mdHeading.FindStringIndex(md); loc != nil {
		return md[loc[0]:]
	}
	return md
}

// EvaluateProposal runs the agent to evaluate a project proposal across eight
// dimensions, grounding the assessment in any available external signals.
func (c *Client) EvaluateProposal(ctx context.Context, name, description, githubURL string, emit EventFunc) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, `Evaluate this public-goods project proposal across eight dimensions: Impact, Team, Innovation, Sustainability, Ecosystem fit, Transparency, Community, and Risk.

Project: %s
Description: %s
`, name, description)
	if githubURL != "" {
		fmt.Fprintf(&b, "GitHub: %s\n\n", githubURL)
		b.WriteString("Use get_github_signals (parse the owner and repo from the URL) to ground the Team, Innovation, and Transparency dimensions in real repository data. ")
	} else {
		b.WriteString("\n")
	}
	b.WriteString("Use get_oso_metrics, get_forum_sentiment, and find_in_retropgf where a project name is available. Score each dimension 0-10 with a one-line justification tied to evidence, then give an overall funding recommendation. Do not fabricate metrics; if a signal is unavailable, say so explicitly.\n\n")
	b.WriteString(reportOnly)
	md, err := c.Run(ctx, AnalystSystem, b.String(), emit)
	return trimToReport(md), err
}
