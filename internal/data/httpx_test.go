package data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetJSONRetriesTransientThenSucceeds(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	body, err := getJSON(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("getJSON: %v", err)
	}
	if !strings.Contains(string(body), "ok") {
		t.Errorf("unexpected body: %s", body)
	}
	if got := atomic.LoadInt32(&n); got != 3 {
		t.Errorf("expected 3 attempts (2 retries), got %d", got)
	}
}

func TestGetJSONClientErrorDoesNotRetry(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := getJSON(context.Background(), srv.URL, nil); err == nil {
		t.Error("expected error on 404")
	}
	if got := atomic.LoadInt32(&n); got != 1 {
		t.Errorf("expected exactly 1 attempt for a 4xx, got %d", got)
	}
}

func TestGetJSONRespectsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // always transient → would retry
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	if _, err := getJSON(ctx, srv.URL, nil); err == nil {
		t.Error("expected error when context is cancelled")
	}
}
