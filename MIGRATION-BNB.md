# Tessera → BNB Chain migration

Tessera is an off-chain analytics agent (Go backend + Next.js frontend). It has **no smart
contracts** and signs **no transactions** — its only chain touchpoint is the read-only
multi-chain scanner (`internal/data/blockchain.go`, agent tool `scan_chain`, CLI `scan-chain`).
The migration makes BNB Chain the first-class network in that scanner. The product's
public-goods data sources (Octant, Gitcoin, Optimism RetroPGF) are Ethereum-ecosystem
by nature and were left intact.

## What changed

- `internal/data/blockchain.go`
  - Added **BNB Smart Chain** (chainId 56, `https://bsc-dataseed.bnbchain.org`, BscScan API) and
    **opBNB** (chainId 204, `https://opbnb-mainnet-rpc.bnbchain.org`) at the top of `SupportedChains`.
  - Replaced **Monad Testnet** (10143) with **BSC Testnet** (chainId 97,
    `https://data-seed-prebsc-1-s1.bnbchain.org:8545`, `https://api-testnet.bscscan.com`, native `tBNB`).
  - Added BSC stablecoins to `chainTokens[56]`, all **18 decimals**:
    USDT `0x55d398326f99059fF775485246999027B3197955`,
    USDC `0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d`,
    FDUSD `0xc5f0f7b66764F6ec8C8Dff7BA683102295E16409`.
  - `FormatSignals` no longer calls the summed native balance "ETH-equivalent" (it now mixes BNB).
  - Scanner now covers 11 chains (was 9).
- Text updated for the new chain set: `cmd/tessera/main.go` (help/usage + balance line),
  `internal/agent/exec.go` (`scan_chain` tool description), `internal/agent/prompts.go`,
  `internal/analysis/reliability.go`, `README.md`, `skills/public-goods-analyst/SKILL.md`,
  `frontend/src/app/page.tsx`, `frontend/src/app/opengraph-image.tsx`.

No new env vars: RPC endpoints are hardcoded public endpoints, same as the existing chains.

## Verification

`go build ./... && go vet ./... && go test ./...` passes. (`gofmt -l` flags
`internal/report/pdf.go`, which was already unformatted before this change.) No live RPC calls were made.

## TODO / notes

- The committed `tessera` binary in the repo root is stale; rebuild with `go build -o tessera ./cmd/tessera/`.
- Explorer calls (`api.bscscan.com`, `api-testnet.bscscan.com`) are unauthenticated V1-style
  requests like the other chains; BscScan/Etherscan now prefer the V2 multichain API
  (`https://api.etherscan.io/v2/api?chainid=56&apikey=…`). Add an `ETHERSCAN_API_KEY` through
  `internal/config` if explorer enrichment (recent txs, token transfers, verification) is needed.
- opBNB has no explorer API or token list configured (native balance/tx/contract checks only).
  Add opBNB stablecoin addresses after checking them on opbnb.bscscan.com.
- Native balances across chains are summed in their own units (ETH + BNB + MNT); there is no price
  normalisation. Add a Chainlink/price lookup if a USD total is needed.
- No deployment required — Tessera has no on-chain component.
