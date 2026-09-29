package mcp

import (
	"bufio"
	"bytes"
	"context"
	"io"
)

// ServeStdio runs the MCP stdio transport: newline-delimited JSON-RPC messages
// in on r, responses out on w (one JSON object per line). Used by `tessera mcp`
// and registered with `hermes mcp add tessera -- tessera mcp`.
func (s *Server) ServeStdio(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // allow large tool payloads
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		msg := make([]byte, len(line)) // scanner reuses its buffer; copy before handling
		copy(msg, line)

		resp := s.Handle(ctx, msg)
		if resp == nil {
			continue
		}
		if _, err := w.Write(append(resp, '\n')); err != nil {
			return err
		}
	}
	return scanner.Err()
}
