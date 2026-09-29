package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// sseWriter streams Server-Sent Events to the client.
type sseWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

// newSSE prepares an SSE stream, or returns ok=false if the ResponseWriter does
// not support flushing.
func newSSE(w http.ResponseWriter) (*sseWriter, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // disable proxy buffering (nginx/Railway)
	w.WriteHeader(http.StatusOK)
	f.Flush()
	return &sseWriter{w: w, f: f}, true
}

// send writes one named event with a JSON data payload and flushes it.
func (s *sseWriter) send(event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		b = []byte(`{"error":"failed to encode event"}`)
	}
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, b)
	s.f.Flush()
}
