# Tessera: BNB Chain verification

Date: 2026-09-25. Tessera has no contracts and signs no transactions. Its only chain touchpoint is the
read-only multi-chain scanner (`internal/data/blockchain.go`), which the CLI (`scan-chain`), the agent tool,
and the MCP tool (`scan_chain`) all use. So this check ran **real read-only scans against live BNB Chain RPCs**
instead of using an anvil fork.

## Checks

| Check | Result | Notes |
|---|---|---|
| Contracts build/test | SKIPPED | No on-chain component |
| Fork deploy / core on-chain flow | SKIPPED | Nothing to deploy |
| `go build ./... && go vet ./... && go test -count=1 ./...` | PASS | agent, analysis, data tests pass |
| Rebuild binary (`go build -o tessera ./cmd/tessera/`) | PASS | The stale committed binary was replaced |
| RPC chain IDs | PASS | `bsc-dataseed.bnbchain.org` = 56, `opbnb-mainnet-rpc.bnbchain.org` = 204, `data-seed-prebsc-1-s1.bnbchain.org:8545` = 97, all serving live blocks |
| BSC stablecoin addresses and decimals (`cast`) | PASS | USDT `0x55d3…7955`, USDC `0x8AC7…580d`, FDUSD `0xc5f0…6409`: all have code, symbols match, all 18 decimals |
| Live scan, PancakeSwap V2 router `0x10ED43C718714eb63d5aA57B78B54704E256024E` | PASS | BSC (56) shows Contract; opBNB (204) and the other chains show an EOA with dust; BSC Testnet (97) shows Contract. About 3.7 s for 11 chains |
| Live scan, PancakeSwap testnet router `0xD99D1c33…50D1` | PASS | Chain 97 shows Contract (matches `cast codesize` 18089) |
| Live scan, PancakeSwap V3 SmartRouter `0x678Aa4bF…fa86` | PASS | Chains 56, 204 and 97 all show Contract (opBNB codesize 24316) |
| Live scan, Binance hot wallet `0xF977…aceC` (stablecoin decimals) | PASS | BSC USDT/USDC come out as 100,000,000 each, which matches raw `balanceOf` (1e26 at 18 decimals). FDUSD is about 27.5M |
| Backend `serve` (port 3150): `/api/health` | PASS | 200 `{"status":"ok"}` |
| MCP over HTTP: `tools/list` + `tools/call scan_chain` | PASS | Returns JSON for all 11 chains with BSC/opBNB/BSC Testnet first |
| `/api/status` | PARTIAL | Octant API returns 403 (Cloudflare challenge, external and unrelated to BNB). "AI Agent" shows no backend because no key was set on purpose |
| Explorer enrichment (`api.bscscan.com`, `api-testnet.bscscan.com`) | FAIL (external) | BscScan V1 is gone (301 / "deprecated V1 endpoint"). `contractVerified`, `recentTxCount` and `tokenTransfers` are always false/0 on every Etherscan-family chain. Failures are swallowed, so scans still succeed |
| Frontend: `pnpm install`, `tsc --noEmit`, `next build` | PASS | Next 16.2.0, all 5 routes static |
| Frontend `next start` (port 3151) | PASS | `/`, `/dashboard` and `/opengraph-image` return 200. Page copy shows "BNB Chain + 10 EVM chains". No chain-mismatch text (there is no wallet connection) |

## Bugs fixed

- `cmd/tessera/main.go` (`scan-chain` CLI): the summary still said `Total native: … ETH-equiv` even though the
  total now adds BNB and MNT. It now says `(sum of ETH/BNB/MNT units, not price-normalised)`, which matches `FormatSignals`.
- `cmd/tessera/main.go`: the table had no FDUSD column, so BSC FDUSD balances only appeared in the summary.
  Added an FDUSD column.
- Rebuilt the stale `tessera` binary.

## Remaining for production on BNB Chain

- No deploy, key, or tBNB needed (0 tBNB).
- Optional: move explorer calls to Etherscan API V2 (`https://api.etherscan.io/v2/api?chainid=56|97&apikey=…`) with an
  `ETHERSCAN_API_KEY` in `internal/config`. Without it, verification and recent-tx enrichment stay empty.
- Optional: add opBNB stablecoin addresses and an explorer for chain 204.
- Deploy the backend as before (Railway, `ANTHROPIC_API_KEY` or `HERMES_*`) and the frontend (Vercel, `NEXT_PUBLIC_API_URL`).
