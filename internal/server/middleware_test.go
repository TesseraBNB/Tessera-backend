package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeheskieltame/tessera/internal/config"
)

func TestCORSAllowsListedOriginWithPrivateNetworkPreflight(t *testing.T) {
	a := &App{cfg: &config.Config{AllowedOrigins: []string{"https://tessera-bnb.vercel.app"}}}
	h := a.cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodOptions, "/api/agent/info", nil)
	req.Header.Set("Origin", "https://tessera-bnb.vercel.app")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://tessera-bnb.vercel.app" {
		t.Errorf("Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Private-Network"); got != "true" {
		t.Errorf("Allow-Private-Network = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "ngrok-skip-browser-warning") {
		t.Errorf("Allow-Headers = %q, want the ngrok bypass header", got)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d", rec.Code)
	}
}

func TestCORSIgnoresUnlistedOrigin(t *testing.T) {
	a := &App{cfg: &config.Config{AllowedOrigins: []string{"https://tessera-bnb.vercel.app"}}}
	h := a.cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/api/agent/info", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unlisted origin got Allow-Origin %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Private-Network"); got != "" {
		t.Errorf("unlisted origin got Allow-Private-Network %q", got)
	}
}
