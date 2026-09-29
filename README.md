<div align="center">

# Tessera

**An autonomous agent for Ethereum public-goods funding intelligence.**
_Evidence over narrative._

</div>

Tessera evaluates public-goods projects (Octant quadratic funding, Gitcoin, Optimism
RetroPGF) the way a skeptical analyst would. It is a **real tool-calling agent** — Claude
Opus 4.8 decides which data to pull, Tessera runs the tools in-process against live
sources, and the agent reasons to a verdict. Every figure is traced to a tool call; nothing
is invented.

- **Live agent trace** — watch the agent call `get_project_history`, `get_trust_profile`,
  `scan_chain`, `simulate_mechanisms`, … in real time over SSE.
- **Trust-graph forensics** — Shannon-entropy donor diversity, Jaccard donor overlap, and
  whale dependency surface Sybil/coordination risk.
- **Mechanism simulation** — replays an epoch under Standard / Capped / Equal / Trust-weighted
  QF with Gini and top-share.
- **Cross-ecosystem validation** — corroborates against OSO, GitHub, the Octant forum, and
  Optimism RetroPGF.

## Architecture

```
Browser ── Vercel (Next.js 16) ──HTTPS──> Railway (Go service)
                                             ├─ /api/*         JSON (fast, no LLM)
                                             ├─ /api/agent/*   SSE (tool_call → result → text)
                                             └─ internal/agent ── tool loop ──> Hermes ──> Opus 4.8
                                                  executes tools in-process     └─(fallback)─> Anthropic API
                                                  └─> data/ (cache + retry + concurrency) · analysis/ · report/
```

The agent runs an **in-process tool-calling loop** over the Anthropic Messages API. In
production the request is relayed by **Hermes** to a real Claude Code Opus 4.8 agent; if
Hermes is unset or failing, Tessera falls back to the Anthropic API directly. Tools execute
inside the Go process, so no tool endpoint is ever exposed to the network. See
[ARCHITECTURE.md](ARCHITECTURE.md) for detail.

## Quickstart (local)

**Backend** (Go ≥ 1.25):

```bash
cp .env.example .env          # set ANTHROPIC_API_KEY (or HERMES_BASE_URL + HERMES_TOKEN)
go run ./cmd/tessera serve    # → http://localhost:8080
```

**Frontend** (Node ≥ 20, pnpm):

```bash
cd frontend
cp .env.example .env.local    # NEXT_PUBLIC_API_URL=http://localhost:8080
pnpm install
pnpm dev                      # → http://localhost:3000
```

## Configuration

Backend (`.env`):

| Variable | Purpose | Default |
| --- | --- | --- |
| `HERMES_BASE_URL` / `HERMES_TOKEN` | Relay to a real Claude Code Opus 4.8 agent (primary) | — |
| `ANTHROPIC_API_KEY` | Direct Anthropic API (fallback / simplest local setup) | — |
| `TESSERA_MODEL` | Agent model | `claude-opus-4-8` |
| `PORT` | HTTP port | `8080` |
| `ALLOWED_ORIGINS` | CORS allowlist (CSV) | `http://localhost:3000` |
| `OSO_API_KEY`, `GITHUB_TOKEN` | Enrich cross-referencing | — |
| `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` | Per-IP rate limit | `1` / `5` |
| `AGENT_MAX_ITERATIONS` | Max tool-use rounds per run | `12` |
| `AGENT_DAILY_BUDGET` | Global agent runs/day, 0 = unlimited | `0` |
| `CACHE_TTL` | In-memory upstream cache | `10m` |

At least one of `HERMES_BASE_URL` or `ANTHROPIC_API_KEY` must be set.

Frontend (`frontend/.env.local`): `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_SITE_URL`.

## API

| Endpoint | Description |
| --- | --- |
| `GET /api/health` · `GET /api/status` · `GET /api/agent/info` | Liveness, upstream status, agent backends |
| `GET /api/epochs/current` · `GET /api/projects?epoch=` | Octant epoch + project list |
| `GET /api/analyze-epoch?epoch=` | Composite ranking (k-means + scoring) |
| `GET /api/detect-anomalies?epoch=` · `GET /api/trust-graph?epoch=` · `GET /api/simulate?epoch=` | Quant analyses |
| `GET /api/agent/analyze?address=` | **SSE** — full agent project analysis |
| `GET /api/agent/evaluate?name=&description=&githubURL=` | **SSE** — proposal evaluation |
| `GET /api/agent/chat?message=` | **SSE** — open-ended agent chat |
| `GET /api/reports` · `GET /api/reports/{name}` | List / download generated PDF reports |

## CLI

```bash
go build -o tessera ./cmd/tessera/
./tessera serve                       # HTTP API
./tessera analyze-project <0xaddr>    # agent project analysis
./tessera evaluate "Name" -d "desc"   # agent proposal evaluation
./tessera analyze-epoch -e 5          # composite ranking
./tessera trust-graph -e 5            # trust-graph metrics
./tessera simulate -e 5               # mechanism comparison
./tessera scan-chain <0xaddr>         # 11-chain on-chain scan (BNB Chain + EVM L1/L2s)
./tessera status                      # connectivity + agent backends
```

## Use Tessera as an MCP server

Expose Tessera's ten tools to any MCP-aware agent (e.g. **Claude Code via Hermes**) — no API
key required, since the reasoning happens on the client side and the tools run in-process.

```bash
# stdio (local): register the binary with your agent
hermes mcp add tessera --command "$(pwd)/tessera" --args mcp
# …or run it directly:  ./tessera mcp   (newline-delimited JSON-RPC on stdio)
```

The deployed server also speaks MCP over HTTP at `POST /mcp` (Streamable HTTP), so a remote
agent can call the same tools. Both transports share one core (`internal/mcp`).

## Deploy

- **Backend → Railway:** Docker build from the repo `Dockerfile` (see `railway.toml`); set the
  env vars above; healthcheck `/api/health`.
- **Frontend → Vercel:** import `frontend/`; set `NEXT_PUBLIC_API_URL` to the Railway URL.
- **Hermes:** point `HERMES_BASE_URL`/`HERMES_TOKEN` at your relay (Anthropic Messages API
  shape, with `tools`/`tool_use`). Without it, set `ANTHROPIC_API_KEY` and Tessera uses the
  Anthropic API directly.

## Development

```bash
go build ./... && go vet ./... && go test ./...   # backend
cd frontend && pnpm lint && pnpm build            # frontend
```

CI (`.github/workflows/ci.yml`) runs all of the above plus `golangci-lint` and a Docker build.

## Project structure

```
cmd/tessera/        CLI + server entrypoint
internal/
  agent/            tool-calling loop, Hermes/Anthropic transport, tool registry
  analysis/         deterministic analytics (scoring, trust graph, mechanisms)
  config/           typed env configuration
  data/             upstream clients (Octant, OSO, GitHub, Discourse, RetroPGF, chains) + cache/retry
  ethunit/          wei→ETH (leaf, shared)
  report/           Markdown + branded PDF generation
  server/           HTTP API: app, middleware, SSE, handlers
frontend/           Next.js 16 app (Vercel)
Dockerfile · railway.toml
```

## License

MIT.
