# Architecture

Tessera is a Go API service (run on the user's machine, or any container host) plus a
Next.js app (Vercel). The Go service embeds an autonomous agent that drives an
**in-process tool-calling loop** over the Anthropic Messages API.

## Topology

```
                         Vercel                   localhost:8080 (or a host)
┌───────────┐      ┌──────────────────┐       ┌──────────────────────────────┐
│  Browser  │─────▶│  Next.js 16 app  │──SSE─▶│  Go service (internal/server) │
└───────────┘      │  NEXT_PUBLIC_API │ JSON  │  http.Server + timeouts +     │
                   └──────────────────┘       │  graceful shutdown            │
                                              │   middleware: CORS allowlist, │
                                              │   rate-limit, recovery,       │
                                              │   request-id, slog            │
                                              │                               │
                                              │  internal/agent (tool loop)   │
                                              │     │                         │
                                              │     ├─ tools exec in-process ─┼─▶ internal/data
                                              │     │  (Octant/OSO/GitHub/…)  │   (cache+retry+concurrency)
                                              │     │                         │   internal/analysis
                                              │     ▼                         │
                                              │   Messages API providers:     │
                                              │   Hermes → BASE_URL → FALLBACK│
                                              └──────────────────────────────┘
```

## The agent loop (`internal/agent`)

Option A: **Tessera owns the loop.** Providers are only transports that speak the
Anthropic Messages API (including `tools`/`tool_use`). The loop (`loop.go`):

1. Send `messages + tools + system` to the model via `sendMessages`, trying each
   configured backend in order: Hermes relay, the provider at `ANTHROPIC_BASE_URL`
   (Anthropic API by default, or e.g. xKiro), then `FALLBACK_*` with its own model id.
2. If the response contains `tool_use` blocks, execute each tool **in-process**
   (`exec.go` → `internal/data` + `internal/analysis`), append `tool_result`, and loop.
3. When `stop_reason != "tool_use"`, the turn's text is the final answer.

Bounded by `AGENT_MAX_ITERATIONS` and a per-run context deadline. Every step is emitted
as an `Event` (`tool_call`, `tool_result`, `text`, `done`, `error`) so the HTTP layer can
stream it over SSE.

Tools run inside the process, so **no tool endpoint is exposed** — there is no inbound
attack surface or SSRF risk from the agent. The tool registry (`exec.go`) wraps existing
deterministic analytics and data clients; the model never sees raw credentials.

### Avoiding an import cycle

`agent` imports `analysis` (its tools call deterministic analytics). The legacy one-shot
LLM evaluators live in `analysis` and used to depend on the provider layer. To keep
`analysis` free of any transport dependency, it defines an `analysis.LLM` interface;
`agent.Client.AsLLM()` adapts the client to it. So the dependency only ever points
`agent → analysis`, never back.

## Data pipeline (`internal/data`)

- `httpx.go` — one shared `*http.Client`; `getJSON`/`postJSON` with exponential backoff +
  jitter, retrying network errors / 429 / 5xx and surfacing 4xx immediately.
- `cache.go` — small TTL cache; the Octant client caches per-endpoint responses
  (`CACHE_TTL`), since closed-epoch data is immutable.
- `oso.go` — Open Source Observer over its async SQL API (`api.oso.xyz/v1/async-sql`,
  Trino): submit, poll, download the JSON-lines result, cache it for an hour. Every value
  that reaches a query is validated first (addresses, names, `chainId:roundId` refs).
- `gitcoin.go` — Gitcoin Grants history from OSO's `int_events__gitcoin_funding` (every
  donation and matching payout, 2019 to May 2025), plus address → OSO project resolution
  (Gitcoin's recipient mapping first, OSO's artifact registry as the slow fallback).
- Concurrency — `GetProjectHistory` fans out across epochs with bounded goroutines instead
  of a sequential loop.
- `ethunit` — single wei→ETH implementation shared by `data` and `analysis` (a leaf package,
  so no cycle).

## HTTP layer (`internal/server`)

`App` holds all dependencies (config, agent, Octant client, logger, limiter, budget) — no
package-level mutable state, so there is no cross-request data race. Files:

- `app.go` — `App`, route table (method-aware `http.ServeMux`), `http.Server` with
  `ReadHeaderTimeout`/`ReadTimeout`/`IdleTimeout` (WriteTimeout disabled for SSE) and
  graceful shutdown on SIGTERM.
- `middleware.go` — recover → request-id → slog → CORS allowlist → per-IP token-bucket
  rate-limit; plus a global daily agent budget (cost guard).
- `sse.go` — Server-Sent Events writer.
- `handlers_data.go` — fast quantitative JSON endpoints (no LLM).
- `handlers_agent.go` — SSE agent runs (analyze / evaluate / chat), report listing/serving,
  PDF generation.

## Security model

- **CORS allowlist** (not `*`); the browser calls the Go service directly, only allowed origins
  get the ACAO header.
- **Rate limiting** per IP + a **daily budget** on agent runs to bound LLM cost.
- **No secrets in the client.** API keys live only in the backend's environment (`.env`).
- **No inbound tool surface** — tools are in-process Go functions.
- Report file serving is path-traversal guarded.

## Configuration

All runtime config is read once into `config.Config` (`internal/config`) and injected;
the rest of the code never calls `os.Getenv`. See the table in [README](README.md).
