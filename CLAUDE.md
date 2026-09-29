# CLAUDE.md — Tessera

## What this is

Tessera is a production tool: an **autonomous agent for Ethereum public-goods funding
intelligence** (Octant quadratic funding, Gitcoin, Optimism RetroPGF). It is a real
tool-calling agent (Claude Opus 4.8) with a **Go backend (Railway)** and a **Next.js 16
frontend (Vercel)**. It began life as a Synthesis Hackathon winner; the pre-rebuild snapshot
lives at the git tag `v1.0-hackathon`. All hackathon-specific code (submission API, Moltbook,
the `tessera-bridge`, multi-provider chain) has been removed.

See `README.md` (usage/deploy) and `ARCHITECTURE.md` (design) for detail.

## Commands

```bash
# Backend (Go ≥ 1.25)
go build ./... && go vet ./... && go test ./...
go run ./cmd/tessera serve            # API on :8080 (needs ANTHROPIC_API_KEY or HERMES_*)

# Frontend (Node ≥ 20, pnpm)
cd frontend && pnpm install && pnpm dev   # :3000 (needs NEXT_PUBLIC_API_URL)
cd frontend && pnpm lint && pnpm build
```

## Architecture (summary)

- **`internal/agent`** — the agent owns an in-process tool-calling loop over the Anthropic
  Messages API. Transport tries **Hermes** (relay, `HERMES_BASE_URL`/`HERMES_TOKEN`) then the
  **Anthropic API** (`ANTHROPIC_API_KEY`) as fallback. Model `claude-opus-4-8`. Tools execute
  in-process (`exec.go`) against `internal/data` + `internal/analysis`.
- **`internal/server`** — `App` struct DI (no package globals), `http.Server` with timeouts +
  graceful shutdown, middleware (CORS allowlist, rate-limit, recovery, request-id, slog), fast
  JSON endpoints (`handlers_data.go`) and SSE agent endpoints (`handlers_agent.go`).
- **`internal/data`** — upstream clients; shared `getJSON`/`postJSON` with retry/backoff
  (`httpx.go`); Octant responses cached (`cache.go`); concurrent epoch history.
- **`internal/analysis`** — deterministic analytics (scoring, k-means, trust graph, mechanism
  simulation) + legacy one-shot LLM evaluators behind the `analysis.LLM` interface.
- **`internal/config`** — typed config loaded once from env.
- **`internal/mcp`** — exposes the agent's tools over the Model Context Protocol (stdio via
  `tessera mcp`, and HTTP at `POST /mcp`), so an external agent (e.g. Claude Code via Hermes)
  can call them. No AI backend needed — reasoning happens client-side.
- **`internal/ethunit`** — shared wei→ETH (leaf package).
- **`internal/report`** — Markdown + branded PDF (logo via `go:embed`).

## Conventions

- Read env **only** through `internal/config.Config`; do not call `os.Getenv` elsewhere.
- No package-global mutable state in the server — everything hangs off `App`.
- AI is **Claude-only**: Hermes + Anthropic fallback. Do not reintroduce Gemini/OpenAI/CLI.
- Add an agent tool by registering it in `agent.buildRegistry` (`exec.go`); keep executors
  in-process and never expose a tool over HTTP.
- Convert wei→ETH only via `ethunit.ToETH` (or `analysis.WeiToEth`, which delegates to it).
- `analysis` must **not** import `agent` (would cycle); use the `analysis.LLM` interface +
  `agent.Client.AsLLM()`.
- New data clients should route HTTP through `getJSON`/`postJSON` for retry behaviour.

## Deploy

- Backend → **Railway** via `Dockerfile` + `railway.toml` (healthcheck `/api/health`).
- Frontend → **Vercel** (`frontend/`, set `NEXT_PUBLIC_API_URL` to the Railway URL).
- Env templates: `.env.example` (backend), `frontend/.env.example` (frontend).
- CI: `.github/workflows/ci.yml` (Go build/vet/test/golangci-lint + frontend build + docker build).

## Structure

```
cmd/tessera/main.go    CLI + `serve`
internal/{agent,analysis,config,data,ethunit,report,server}/
frontend/              Next.js 16 app (Vercel)
skills/public-goods-analyst/  OpenClaw distribution skill (build: go build -o tessera ./cmd/tessera/)
Dockerfile · railway.toml · .golangci.yml
```
