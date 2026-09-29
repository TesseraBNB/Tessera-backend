package mcp

import (
	"io"
	"net/http"
)

// HTTPHandler serves the MCP "Streamable HTTP" transport: each POST carries one
// JSON-RPC message and the response is returned as application/json. The server
// is stateless (tools are independent), so no session header is required.
func (s *Server) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		resp := s.Handle(r.Context(), body)
		if resp == nil {
			w.WriteHeader(http.StatusAccepted) // notification: no body
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	}
}
