# design.md — Tessera Pitch Deck Style

Rendered deck: `Slides/Tessera_Deck.pdf`, built from these specs by `Slides/deck/build.py`
(`python collect_data.py` refreshes the figures from Octant and Tessera's own tools via MCP;
`python build.py` renders HTML and prints the PDF with Chrome/Edge).

Visual system for the Tessera deck, ported 1:1 from the live frontend
(`frontend/src/app/globals.css`, `frontend/src/app/layout.tsx`). Theme
name: **"Obsidian Observatory"** — obsidian surfaces, warm bone text,
ember signature accent, signal-cyan for live data, tessellation motifs.

Feeling target: a dark astronomical instrument crossed with a forensic
lab report — evidence-bound, calm, precise. Never neon, never hype.

## Color

### Palette (from @theme tokens)
- **Surfaces:** `--color-void #07090a` (deepest) · `--color-bg #0a0c0e` (body) · `--color-surface #0f1417` (cards) · `--color-raised #151c21` (elevated) · lines `#222b31` / `#33404a`
- **Text (warm bone):** `--color-bone #ece7da` (primary) · `--color-bone-dim #9ba39f` (secondary) · `--color-bone-faint #626c6a` (metadata)
- **Ember (warm signature):** `--color-ember #e8633a` · bright `#ff7d52` · deep `#a83c1c` — the brand voice; agent, verdicts, identity moments
- **Signal (cool, live data):** `--color-signal #46d6d0` · bright `#74f2ec` · deep `#1c817d` — the data voice; tool calls, live streams, chain chips, anything real-time
- **Status:** good `#5fd1a0` · warn `#e8b23a` · bad `#ef5d5d` — never decorative, only state

### Accent discipline (hard rules)
- Ember = agent/brand/verdict. Signal-cyan = live data/tool calls/verified chains. They appear as a pair when both identities matter (slide 03, 11)
- Problem slides mute brand color: warn/bad lead, cyan absent. Solution slides restore the ember/cyan pair
- Max 2–3 accent elements per slide; all body text stays bone for AA contrast on obsidian
- Dark mode only — this system has no light variant, and neither does the deck

## Typography (3 families, by role)

- **Display — Instrument Serif** (400, normal + italic): wordmark, headlines, quotes, the closing creed. Editorial weight through serif scale, not bold.
- **UI — Schibsted Grotesk** (400–600): subheads, card titles, stat numbers, phase headers
- **Data — IBM Plex Mono** (400–600): ALL tool names, addresses (truncated), logs, chain ids, timestamps, spec chips. Rule: if it's a function name or on-chain fact, it's Plex Mono.

Scale: wordmark 84–96px · headline Instrument Serif 44px · card titles Schibsted 600 15–16px · body/mono 11–13px. Micro-label convention: `.label` style — uppercase, tracking 0.22em, 11px, bone-dim.

## Motifs (signature, per-slide rotation)

1. **`.tess-field`** — tessellated diamond weave (3× repeating-linear-gradient, bone 3–6%, 26/45px cells): the "structure" motif. High on architecture/findings slides, faded on emotional ones
2. **`.dot-field`** — 24px constellation dots: the "live network" motif. Used on flow/pipeline/chain slides
3. **`.grain`** — 40–60% film grain overlay: always on, depth without noise
- Rotation rule: tess (structure) XOR dots (flow) per slide; grain always

## Layout & Geometry

- Radii: 14px (lg) / 20px (xl) — cards `rounded-lg/xl`, pills 99px
- Slide margins ~25–30%; one hero element per slide (terminal, pipeline, graph, timeline)
- Glow usage: 15–25% opacity, 120–160px blur, max ONE main + ONE echo per slide
- Full-bleed darkness slides (01, 07, 12) drop the body layer for depth — "observatory night"

## Slide Content Rule (mandatory 65/35)

Every slide file:
1. **Caption block (≤35%)** — exact on-slide text: kicker, headline, labels, chips, truncated addresses, log excerpts. Phrases only.
2. **Visual instructions (≥65%)** — canvas spec (motif + glow + grain), layout grid, component spec (card styles, icons, type size/color per element), illustration spec (trust graph, gini bars, pipeline particles), accent discipline, motion, speaker notes.

Caption discipline: addresses truncated; lists ≤5 items; no tables; log excerpts ≤10 lines; every number must come from the repo (deploy blocks, entropy values, gini figures, chain ids) — never invented. Agent figures come from the recorded run in `examples/agent-trace-epoch10.md`; epoch-wide patterns from `FINDINGS.md`; contract facts from `contracts/deployments/bsc-testnet.json`. The run has no timestamps, so traces show step numbers, not clock times.

## Motion (if rendered as HTML)

- Entrances: fade/rise 400–600ms, stagger 120–150ms
- Diagrams: SVG draw-in (edges, timelines, converging lines) 400–600ms
- Live-feel elements: log type-in 400–500ms/row with cyan block cursor; dot indicators pulse 2s; pipeline particles flow 1.5s loop; orbits rotate 40s
- One-shot pulses for verdicts/verified badges (satisfying beat)
- `prefers-reduced-motion`: everything static and instant; no autoplay choreography beyond entrances
- Closing slide: single fade sequence, zero loops — stillness as confidence

## Per-Slide Special Cases

- **02 Problem:** brand ember muted; warn/bad carry the pain; the "gap" zigzag is the only red element
- **03 Solution:** the ONLY slide where ember and cyan share the stage symmetrically (agent × data identity pairing)
- **04 How It Works:** dot-field replaces tess-field; PLAN node is the single ember node in a cyan pipeline
- **06 Tools:** instrument cards show miniature real outputs (sparkline, donut, bars) — not icon-only cards
- **07 Trace:** darkest slide; terminal hero; type-in cursor; the static deck's "alive" moment
- **08 Analysis:** findings from FINDINGS.md only; ember limited to 3 discovery points
- **09 BNB:** BSC chips lead with cyan borders; 11-chain dot strip shows breadth without crowding
- **10 Security:** green ambient state; "boring where it counts" reads visually quiet
- **12 Closing:** void + one ember glow + two-line creed; five colors total; emptiest slide in the deck
