<div align="center">

# Tessera (BNB Chain)

**An autonomous agent for public-goods funding intelligence — BNB Chain edition.**
_Evidence over narrative, notarized on BSC._

</div>

Tessera evaluates public-goods projects (Octant quadratic funding, Gitcoin, Optimism
RetroPGF) the way a skeptical analyst would. It is a **real tool-calling agent** — the
model decides which data to pull, Tessera runs the tools in-process against live sources,
and the agent reasons to a verdict. Any Anthropic Messages-compatible model works: Claude,
or free models through [xKiro](https://xkiro.com). Every figure is traced to a tool call; nothing
is invented.

This edition makes **BNB Smart Chain the first-class network** of the on-chain scanner and
adds a **tamper-proof verdict notary** — an immutable, zero-admin smart contract on BSC
testnet that stamps every verdict as a keccak256 hash with a block timestamp.

- **Live agent trace** — watch the agent call `get_project_history`, `get_trust_profile`,
  `scan_chain`, `simulate_mechanisms`, … in real time over SSE.
- **Trust-graph forensics** — Shannon-entropy donor diversity, Jaccard donor overlap, and
  whale dependency surface Sybil/coordination risk.
- **Mechanism simulation** — replays an epoch under Standard / Capped / Equal / Trust-weighted
  QF with Gini and top-share.
- **Cross-ecosystem validation** — identifies a project by its payout address in Gitcoin and
  Open Source Observer (name, repos, funding from every source), then corroborates against
  GitHub, the Octant forum, and Optimism RetroPGF.
- **Gitcoin Grants** — looks a project's payout address up in Gitcoin's history (rounds,
  unique donors, USD donated and matched) and runs the same trust-graph forensics on any
  Gitcoin round. Data comes from Open Source Observer's Gitcoin dataset, 2019 to GG23
  (May 2025); Gitcoin's own indexer went offline when Grants Stack shut down.
- **On-chain citation layer (BNB Chain)** — `TesseraAttestations`, an immutable
  zero-admin registry on BSC: every verdict can be notarized as a hash + timestamp,
  proving it existed and was never altered.

## BNB Chain integration

### Chain scanner — BNB-first

`scan_chain` (agent tool + CLI `scan-chain`) covers **11 chains, BNB first**:

| Chain | ChainId | Role |
| --- | --- | --- |
| **BNB Smart Chain** | **56** | first-class — native BNB balances, BSC stables (18 dec): USDT `0x55d3…`, USDC `0x8AC7…`, FDUSD `0xc5f0…` |
| **opBNB** | **204** | first-class — native balance + tx checks |
| **BSC Testnet** | **97** | staging — tBNB, MockERC20-friendly |
| + 8 EVM L1/L2s | — | Ethereum, Base, Arbitrum, Optimism, … |

The scanner is **read-only**: Tessera signs nothing and holds no keys to funds.
See `internal/data/blockchain.go`.

### TesseraAttestations — verdict notary (BSC Testnet 97)

An additive, permissionless smart contract (`contracts/`) that turns BNB Chain into
the notary for Tessera verdicts:

- `commit(bytes32 verdictHash, RiskLevel, projectId, evidenceUri)` — notarize the
  keccak256 of any verdict/report; idempotent, open to anyone
- `getAttestation(hash)` / `isNotarized(hash)` — prove the verdict existed at block
  time T and was not altered afterwards

**Safety by construction:** no owner, no upgrade path, no custody, no admin
surface — deployed once, immutable forever.

| Field | Value |
| --- | --- |
| Address (verified) | [`0x56e6472693982df91df33842f1d087f2e4308427`](https://testnet.bscscan.com/address/0x56e6472693982df91df33842f1d087f2e4308427#code) |
| Network | BNB Smart Chain Testnet (chainId 97) |
| Deploy tx | `0x9138fbd9ee54ea3ae1fffe5834d2b33e8970a5e9b1e476b3a390b28e3782f1fc` |
| Deployed / verified | 2026-09-30 · Solidity 0.8.24 · 7 Foundry tests |

Quick usage:

```bash
# keccak256 of the file's exact bytes; $'\n' restores the trailing newline $(cat) drops
VERDICT_HASH=$(cast keccak "$(cat contracts/sample-verdict.txt)"$'\n')
cast send 0x56e6472693982df91df33842f1d087f2e4308427 \
  "commit(bytes32,uint8,string,string)" \
  $VERDICT_HASH 2 "octant:ep-23:project-x" "https://…" \
  --private-key $PK --rpc-url https://bsc-testnet-rpc.publicnode.com
```

Full reference: [`contracts/README.md`](contracts/README.md) · record:
[`contracts/deployments/bsc-testnet.json`](contracts/deployments/bsc-testnet.json).

## Architecture

```
Browser ── https://tessera-bnb.vercel.app (Next.js 16, Vercel)
   │  calls the hosted API; falls back to http://localhost:8080 when it does not answer
   ▼
Go service on Railway (Dockerfile) — or `go run ./cmd/tessera serve` on your machine
   ├─ /api/*         JSON (fast, no LLM)
   ├─ /api/agent/*   SSE (tool_call → result → text)
   └─ internal/agent ── tool loop ──> Anthropic Messages API provider, with fallback
        executes tools in-process     (Hermes relay → ANTHROPIC_BASE_URL → FALLBACK_*)
        └─> data/ (cache + retry + concurrency) · analysis/ · report/

BSC 56 / opBNB 204 / testnet 97  ──read-only RPC──>  scan_chain tool
TesseraAttestations (BSC 97)     ──optional commit──>  verdict notarization
```

The agent runs an **in-process tool-calling loop** over the Anthropic Messages API. Backends
are tried in order: an optional **Hermes** relay, the provider at `ANTHROPIC_BASE_URL` (the
Anthropic API when unset, or any compatible provider such as xKiro), then an optional
fallback provider with its own model id. Tools execute inside the Go process, so no tool
endpoint is ever exposed to the network. See
[ARCHITECTURE.md](ARCHITECTURE.md) for detail.

## Try it

Open **https://tessera-bnb.vercel.app** — nothing to install. The UI talks to the hosted API
at `https://tessera-api-production-bef4.up.railway.app` (Railway); the model badge in the top-right shows it
is connected. In the console, paste an Octant payout address (e.g. rotki
`0x9531c059098e3d194ff87febb587ab07b30b1306`) and run the agent, or explore an epoch.

## Run it locally

To run your own backend (and, optionally, the UI) on your machine:

**1. Backend** (Go ≥ 1.25)

```bash
git clone https://github.com/TesseraBNB/Tessera-backend.git tessera && cd tessera
cp .env.example .env          # Windows: copy .env.example .env
# optional: put a free xKiro key (https://xkiro.com) in ANTHROPIC_API_KEY to enable the agent
go run ./cmd/tessera serve    # → http://localhost:8080
```

Check it: `curl http://localhost:8080/api/health` → `{"status":"ok"}`.

**2. Frontend locally** — Node ≥ 20, pnpm. The deployed site prefers the hosted API and only
falls back to your `localhost:8080` when that is unreachable, so run the UI locally to use your
own backend:

```bash
git clone https://github.com/TesseraBNB/Tessera-frontend.git && cd Tessera-frontend
pnpm install && pnpm dev      # → http://localhost:3000 (API defaults to http://localhost:8080)
```

**Without an AI key** the server still runs: Explore (epoch ranking + anomaly detection),
the `/api/*` analytics, `scan-chain` and the MCP tools all work; the agent endpoints answer
503 until a key is set. Octant's allocation rounds have data for epochs 1–10 (Tessera
defaults to the latest funded epoch). Gitcoin history and OSO signals need `OSO_API_KEY`;
without it those tools report the gap instead of failing the run.

`.env.example` is preset for xKiro with `qwen/qwen3.8-omni-flash:free`. For Claude directly,
clear `ANTHROPIC_BASE_URL`, use an Anthropic key and `TESSERA_MODEL=claude-opus-4-8`.

**Contracts** (Foundry, optional — already deployed):

```bash
cd contracts
forge install && forge test   # 7 tests
cp .env.example .env          # DEPLOYER_PRIVATE_KEY + ETHERSCAN_API_KEY
forge script script/Deploy.s.sol --rpc-url bsc_testnet --broadcast --verify
```

## Configuration

Backend (`.env`):

| Variable | Purpose | Default |
| --- | --- | --- |
| `HERMES_BASE_URL` / `HERMES_TOKEN` | Optional relay tried first (Anthropic Messages API shape) | — |
| `ANTHROPIC_API_KEY` | Key for the provider at `ANTHROPIC_BASE_URL` | — |
| `ANTHROPIC_BASE_URL` | Any Anthropic Messages-compatible provider (e.g. `https://api.xkiro.com`) | Anthropic API |
| `FALLBACK_BASE_URL` / `FALLBACK_API_KEY` / `FALLBACK_MODEL` | Second Messages-compatible provider, tried last, with its own model id | — |
| `TESSERA_MODEL` | Agent model (`.env.example`: `qwen/qwen3.8-omni-flash:free`) | `claude-opus-4-8` |
| `PORT` | HTTP port | `8080` |
| `ALLOWED_ORIGINS` | CORS allowlist (CSV) | `http://localhost:3000,https://tessera-bnb.vercel.app` |
| `OSO_API_KEY` | Open Source Observer SQL API key (free: https://www.oso.xyz → Settings → API Keys). Enables Gitcoin history and OSO project signals | — |
| `GITHUB_TOKEN` | Higher GitHub API rate limit | — |
| `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` | Per-IP rate limit | `1` / `5` |
| `AGENT_MAX_ITERATIONS` | Max tool-use rounds per run | `12` |
| `AGENT_DAILY_BUDGET` | Global agent runs/day, 0 = unlimited | `0` |
| `CACHE_TTL` | In-memory upstream cache | `10m` |

The agent needs at least one of `HERMES_BASE_URL`, `ANTHROPIC_API_KEY`, or `FALLBACK_BASE_URL` + `FALLBACK_API_KEY`.

Frontend (Tessera-frontend `.env.local`, both optional): `NEXT_PUBLIC_API_URL` (default
`http://localhost:8080`), `NEXT_PUBLIC_SITE_URL`.

Contracts (`contracts/.env`): `DEPLOYER_PRIVATE_KEY`, `BSC_TESTNET_RPC`,
`ETHERSCAN_API_KEY` — see `contracts/.env.example`.

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
./tessera scan-chain <0xaddr>         # 11-chain on-chain scan (BNB Chain first: BSC 56 · opBNB 204 · testnet 97)
./tessera gitcoin-rounds [-r 42161:865]  # Gitcoin rounds, or one round's projects (needs OSO_API_KEY)
./tessera status                      # connectivity + agent backends
```

## Use Tessera as an MCP server

Expose Tessera's thirteen tools to any MCP-aware agent (e.g. **Claude Code**) — no API
key required, since the reasoning happens on the client side and the tools run in-process.

```bash
# stdio (local): register the binary with your agent
hermes mcp add tessera --command "$(pwd)/tessera" --args mcp
# …or run it directly:  ./tessera mcp   (newline-delimited JSON-RPC on stdio)
```

The deployed server also speaks MCP over HTTP at `POST /mcp` (Streamable HTTP), so a remote
agent can call the same tools. Both transports share one core (`internal/mcp`).

## Deploy

- **Frontend → Vercel:** https://tessera-bnb.vercel.app from `TesseraBNB/Tessera-frontend`,
  deployed on every push to `main`. `NEXT_PUBLIC_API_URL` (set at build time) is the hosted
  API; the site falls back to `http://localhost:8080` when it does not answer.
- **Backend → Railway:** `https://tessera-api-production-bef4.up.railway.app`, built from `Dockerfile`
  (`railway.toml`: healthcheck `/api/health`) and deployed on every push to `main` of
  `TesseraBNB/Tessera-backend`. Its variables mirror `.env.example`; keep the frontend origin in
  `ALLOWED_ORIGINS`. `.railwayignore` keeps manual `railway up` uploads small.
- **Contracts → BSC:** `cd contracts && forge script script/Deploy.s.sol --rpc-url bsc_testnet
  --broadcast --verify` (already live + verified on BSC testnet 97).

## Development

```bash
go build ./... && go vet ./... && go test ./...   # backend
pnpm lint && pnpm build                           # frontend (Tessera-frontend repo)
cd contracts && forge build && forge test         # contracts (7 tests)
```

CI (`.github/workflows/ci.yml`) runs the backend suite plus `golangci-lint` and a Docker
build; the frontend repo runs its own lint + build.

## Project structure

```
cmd/tessera/        CLI + server entrypoint
internal/
  agent/            tool-calling loop, Messages API transport + fallback, tool registry
  analysis/         deterministic analytics (scoring, trust graph, mechanisms)
  config/           typed env configuration
  data/             upstream clients (Octant, OSO SQL + Gitcoin, GitHub, Discourse, RetroPGF, chains — BSC first) + cache/retry
  ethunit/          wei→ETH (leaf, shared)
  report/           Markdown + branded PDF generation
  server/           HTTP API: app, middleware, SSE, handlers
frontend/           Next.js 16 app (separate repo: TesseraBNB/Tessera-frontend)
contracts/          TesseraAttestations — immutable verdict notary on BSC (Foundry)
Dockerfile · railway.toml
```

## License

MIT.
