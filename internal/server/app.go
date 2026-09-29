// Package server exposes the Tessera HTTP API: fast quantitative JSON endpoints
// plus streaming (SSE) endpoints that drive the autonomous agent. All shared
// state lives on App (no package globals), and the server runs with explicit
// timeouts and graceful shutdown.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/yeheskieltame/tessera/internal/agent"
	"github.com/yeheskieltame/tessera/internal/config"
	"github.com/yeheskieltame/tessera/internal/data"
	"github.com/yeheskieltame/tessera/internal/mcp"
)

// App holds shared dependencies and serves the Tessera HTTP API.
type App struct {
	cfg     *config.Config
	agent   *agent.Client
	mcp     *mcp.Server
	octant  *data.OctantClient
	log     *slog.Logger
	limiter *rateLimiter
	budget  *dailyBudget
}

// New constructs an App from configuration.
func New(cfg *config.Config) *App {
	ag := agent.New(cfg)
	return &App{
		cfg:     cfg,
		agent:   ag,
		mcp:     mcp.NewServer(ag),
		octant:  data.NewOctantClient(),
		log:     slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
		limiter: newRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst),
		budget:  newDailyBudget(cfg.AgentDailyBudget),
	}
}

// Handler wires routes and wraps them with the middleware chain.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", a.handleRoot)

	// Health & info
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("GET /api/status", a.handleStatus)
	mux.HandleFunc("GET /api/agent/info", a.handleAgentInfo)

	// Quantitative JSON endpoints (no LLM)
	mux.HandleFunc("GET /api/epochs/current", a.handleCurrentEpoch)
	mux.HandleFunc("GET /api/projects", a.handleProjects)
	mux.HandleFunc("GET /api/analyze-epoch", a.handleAnalyzeEpoch)
	mux.HandleFunc("GET /api/detect-anomalies", a.handleDetectAnomalies)
	mux.HandleFunc("GET /api/trust-graph", a.handleTrustGraph)
	mux.HandleFunc("GET /api/simulate", a.handleSimulate)

	// Agent (SSE) endpoints
	mux.HandleFunc("GET /api/agent/analyze", a.handleAgentAnalyze)
	mux.HandleFunc("GET /api/agent/evaluate", a.handleAgentEvaluate)
	mux.HandleFunc("GET /api/agent/chat", a.handleAgentChat)

	// Reports
	mux.HandleFunc("GET /api/reports", a.handleListReports)
	mux.HandleFunc("GET /api/reports/{name}", a.handleServeReport)

	// MCP (Model Context Protocol) over HTTP — exposes Tessera's tools to
	// external agents (e.g. Claude Code via Hermes).
	mux.HandleFunc("/mcp", a.mcp.HTTPHandler())

	return a.middleware(mux)
}

// Run starts the server and blocks until an interrupt is received, then shuts
// down gracefully.
func (a *App) Run() error {
	srv := &http.Server{
		Addr:              ":" + a.cfg.Port,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0, // disabled: SSE responses stream for minutes
		IdleTimeout:       120 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		a.log.Info("tessera listening", "addr", srv.Addr, "model", a.cfg.Model, "backends", a.agent.Backends())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-stop:
		a.log.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	a.writeJSON(w, http.StatusOK, map[string]any{
		"service": "tessera",
		"docs":    "/api/health, /api/status, /api/agent/info, /api/agent/analyze, /api/agent/evaluate",
	})
}

// --- small helpers ---

func (a *App) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *App) jsonError(w http.ResponseWriter, msg string, status int) {
	a.writeJSON(w, status, map[string]string{"error": msg})
}

func parseEpoch(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("epoch"))
	return n
}
