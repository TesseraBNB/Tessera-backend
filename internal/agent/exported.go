package agent

import (
	"context"
	"encoding/json"
)

// Tools returns the advertised tool schemas. Exported so the MCP server can
// re-expose Tessera's tools to an external agent (e.g. Claude Code via Hermes).
func (c *Client) Tools() []Tool { return c.toolList() }

// CallTool executes a tool by name with raw JSON arguments. Exported for the
// MCP server; the executors run in-process against the data/analysis layers.
func (c *Client) CallTool(ctx context.Context, name string, input json.RawMessage) (string, error) {
	return c.execTool(ctx, name, input)
}
