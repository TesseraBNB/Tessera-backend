package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yeheskieltame/tessera/internal/agent"
	"github.com/yeheskieltame/tessera/internal/report"
)

// agentRunTimeout bounds a single agent run (the model plus all its tool calls).
const agentRunTimeout = 8 * time.Minute

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type service struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail,omitempty"`
	}
	var services []service

	if ep, err := a.octant.GetCurrentEpoch(ctx); err != nil {
		services = append(services, service{"Octant API", "error", err.Error()})
	} else {
		services = append(services, service{"Octant API", "ok", fmt.Sprintf("epoch %d", ep.CurrentEpoch)})
	}

	if a.agent.HasBackend() {
		services = append(services, service{"AI Agent", "ok", strings.Join(a.agent.Backends(), ", ") + " · " + a.cfg.Model})
	} else {
		services = append(services, service{"AI Agent", "error", "no backend configured"})
	}

	a.writeJSON(w, http.StatusOK, map[string]any{"services": services})
}

func (a *App) handleAgentInfo(w http.ResponseWriter, r *http.Request) {
	a.writeJSON(w, http.StatusOK, map[string]any{
		"model":    a.cfg.Model,
		"backends": a.agent.Backends(),
		"ready":    a.agent.HasBackend(),
	})
}

func (a *App) handleAgentAnalyze(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	if address == "" {
		a.jsonError(w, "address query parameter is required", http.StatusBadRequest)
		return
	}
	a.runAgentStream(w, r, "Project Analysis: "+address, func(ctx context.Context, emit agent.EventFunc) (string, error) {
		return a.agent.AnalyzeProject(ctx, address, emit)
	})
}

func (a *App) handleAgentEvaluate(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	name := strings.TrimSpace(q.Get("name"))
	desc := strings.TrimSpace(q.Get("description"))
	github := strings.TrimSpace(q.Get("githubURL"))
	if name == "" || desc == "" {
		a.jsonError(w, "name and description are required", http.StatusBadRequest)
		return
	}
	a.runAgentStream(w, r, "Project Evaluation: "+name, func(ctx context.Context, emit agent.EventFunc) (string, error) {
		return a.agent.EvaluateProposal(ctx, name, desc, github, emit)
	})
}

func (a *App) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	msg := strings.TrimSpace(r.URL.Query().Get("message"))
	if msg == "" {
		a.jsonError(w, "message query parameter is required", http.StatusBadRequest)
		return
	}
	a.runAgentStream(w, r, "", func(ctx context.Context, emit agent.EventFunc) (string, error) {
		return a.agent.Run(ctx, agent.AnalystSystem, msg, emit)
	})
}

// runAgentStream guards, opens an SSE stream, runs the agent forwarding its
// events, then emits a final "result" event (with a PDF report when titled).
func (a *App) runAgentStream(w http.ResponseWriter, r *http.Request, reportTitle string, run func(ctx context.Context, emit agent.EventFunc) (string, error)) {
	if !a.agent.HasBackend() {
		a.jsonError(w, "no AI backend configured", http.StatusServiceUnavailable)
		return
	}
	if !a.budget.allow() {
		a.jsonError(w, "daily agent budget exhausted — try again tomorrow", http.StatusTooManyRequests)
		return
	}
	sse, ok := newSSE(w)
	if !ok {
		a.jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), agentRunTimeout)
	defer cancel()

	md, err := run(ctx, func(e agent.Event) { sse.send(e.Type, e) })
	if err != nil {
		sse.send("error", map[string]string{"error": err.Error()})
		return
	}

	result := map[string]any{"report": md}
	if reportTitle != "" && strings.TrimSpace(md) != "" {
		if path := a.generateReportPDF(reportTitle, md); path != "" {
			result["reportPath"] = filepath.Base(path)
		}
	}
	sse.send("result", result)
}

func (a *App) generateReportPDF(title, markdown string) string {
	rep := &report.PDFReport{
		Title:    title,
		Subtitle: "Tessera — Public Goods Intelligence",
		Model:    a.cfg.Model,
		Provider: strings.Join(a.agent.Backends(), "+"),
		Metadata: map[string]string{"Generated": time.Now().UTC().Format(time.RFC3339)},
		Sections: []report.PDFSection{{Heading: "Report", Body: markdown}},
	}
	if path, err := report.GeneratePDF(rep); err == nil {
		return path
	}
	return ""
}

func (a *App) handleListReports(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir("reports")
	if err != nil {
		a.writeJSON(w, http.StatusOK, map[string]any{"reports": []any{}})
		return
	}
	type repInfo struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		ModTime string `json:"modTime"`
	}
	var reports []repInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		reports = append(reports, repInfo{Name: e.Name(), Size: info.Size(), ModTime: info.ModTime().UTC().Format(time.RFC3339)})
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].ModTime > reports[j].ModTime })
	a.writeJSON(w, http.StatusOK, map[string]any{"reports": reports})
}

func (a *App) handleServeReport(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		a.jsonError(w, "invalid report name", http.StatusBadRequest)
		return
	}
	path := filepath.Join("reports", name)
	if _, err := os.Stat(path); err != nil {
		a.jsonError(w, "report not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, path)
}
