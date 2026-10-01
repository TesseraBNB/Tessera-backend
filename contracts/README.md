# Tessera contracts — `TesseraAttestations`

A **tamper-proof on-chain citation layer for Tessera verdicts** on BNB Smart Chain.

Tessera itself is an off-chain analytics agent (Go + Next.js) and stays exactly
as it is — this contract is an **additive, permissionless notary** beside it.

## What it does

Anyone can commit the `keccak256` hash of a Tessera verdict (or any evidence
document) plus a pointer to its full content. The chain becomes the notary:

- the verdict provably **existed at time T** (block timestamp),
- and was **not altered afterwards** (hash mismatch = tampering),
- without trusting the committer — trust rests on the hash preimage.

```solidity
function commit(bytes32 verdictHash, RiskLevel riskLevel,
                string projectId, string evidenceUri) external;
```

`RiskLevel`: 0=LOW · 1=MEDIUM · 2=HIGH · 3=UNKNOWN.

## Safety properties (by design)

- **No custody** — the contract never holds funds or tokens.
- **No admin** — no owner, no upgrade path, no pause, no allowlist. Deployed
  once, immutable forever; nothing to govern, nothing to rug.
- **Minimal on-chain data** — hashes and pointers only; verdict content lives
  off-chain (Tessera reports / HTTP archives).
- **Idempotent** — re-committing an existing hash is a no-op (first record wins).
- **Open writes** — anyone can notarize; readers verify the preimage.

## Deployment — BSC Testnet (97)

| Field | Value |
|---|---|
| Address (verified) | [`0x56e6472693982df91df33842f1d087f2e4308427`](https://testnet.bscscan.com/address/0x56e6472693982df91df33842f1d087f2e4308427#code) |
| Deploy tx | `0x9138fbd9ee54ea3ae1fffe5834d2b33e8970a5e9b1e476b3a390b28e3782f1fc` |
| Deployer | `0xAEc63F6cEbBfacdC3516992b6ec396147c9c8361` |
| Deployed / verified | 2026-09-30 |
| Solidity | 0.8.24, optimizer 200 |

Record: `deployments/bsc-testnet.json`.

## Usage

```bash
# hash a verdict: keccak256 of the file's exact bytes. $(cat) drops the trailing
# newline, so add it back ($'\n', bash/zsh) to match what was notarized.
VERDICT_HASH=$(cast keccak "$(cat sample-verdict.txt)"$'\n')

# notarize it on BSC testnet
cast send 0x56e6472693982df91df33842f1d087f2e4308427 \
  "commit(bytes32,uint8,string,string)" \
  $VERDICT_HASH 2 "octant:ep-23:project-x" "https://…" \
  --private-key $PK --rpc-url https://bsc-testnet-rpc.publicnode.com

# verify it landed
cast call 0x56e6472693982df91df33842f1d087f2e4308427 \
  "isNotarized(bytes32)(bool)" $VERDICT_HASH --rpc-url https://bsc-testnet-rpc.publicnode.com
```

A synthetic sample verdict (`sample-verdict.txt`, illustrative values, not a real analysis)
was notarized end-to-end at deploy time to prove the flow: hash `0x3fbf9dc1…cb1f3a`, tx
`0x25bcd9…e31ee2a` (see `deployments/bsc-testnet.json`). The commands above reproduce that
hash and `isNotarized` returns `true` for it.

## Development

```bash
forge install   # forge-std
forge build
forge test      # 7 tests
```

## Future integration (post-submission, optional)

The Go agent could auto-commit every emitted verdict hash (a ~20-line addition
to `internal/report` calling the RPC). Deliberately not done pre-submission —
the contract stands alone and Tessera runs untouched.
