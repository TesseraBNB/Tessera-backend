# Slide 08 — The Analysis (Findings)

---

## Caption (teks di slide — maks 35%)

```
THE ANALYSIS
What the evidence reveals.

TRUST GRAPH               MECHANISM               EVIDENCE GAPS
max donor overlap 0.46    1-person-1-vote cuts    OSO · RetroPGF had
whale 0.46 · 38 of 52     the #1 project −70%;    no record — named
donors repeat             gini 0.40 → 0.17        as gaps, not filled
```

---

## Visual Instructions (65%)

**Canvas & background**
- Void base; tess-field 50% (tema: struktur yang terungkap); grain ON
- Glow warna netral bone 8% center — temuan dilihat dalam cahaya obyektif, bukan drama merah

**Layout (hero: 3 panel temuan sejajar, grid 3 kolom gap 20px, tiap 32%)**
- **Panel 1 — Trust Graph:**
  - Kartu `bg-surface border-line rounded-xl`; header: label mono 11px cyan `TRUST GRAPH` + ikon `Network` 20px
  - Ilustrasi: **trust graph sungguhan** — 9 node dot bone 8px dengan edge garis 1px; SATU cluster 4-node diberi edge tebal 2px `--color-warn` + node ember — "coordination ring" terdeteksi; node lain edge bone-faint
  - Caption mini mono 10px: "max jaccard 0.46 · whale 0.46 · epoch 10"
- **Panel 2 — Mechanism:**
  - Kartu sama; header `MECHANISM` cyan + ikon `FlaskConical`
  - Ilustrasi: **before/after bar ganda** — 2 grup bar horizontal: "standard QF" (gini 0.396, bar bad `#ef5d5d` panjang) vs "equal weight" (gini 0.167, bar good `#5fd1a0` lebih pendek); panah ember melengkung antar keduanya + label mono "#1 project −70%"
  - Caption: "epoch 10 · 24 projects · 4 mechanisms simulated"
- **Panel 3 — Evidence gaps:**
  - Kartu sama; header `EVIDENCE GAPS` cyan + ikon `GitCompare`
  - Ilustrasi: 4 baris sumber (chip mono) dengan status: `Octant` ✓ `on-chain` ✓ (12 txs) `OSO` ✗ `RetroPGF` ✗ — garis dari yang ✓ menuju node verdict ember; yang ✗ berhenti dengan label "gap"
  - Caption: "missing signals are named, never invented"
- **Footer strip:** verdict bar nyata dari `examples/agent-trace-epoch10.md` — strip glass `bg-raised` mono 12px: `VERDICT: HOLD / INVESTIGATE · evidence: 8 tool calls · epoch 10` dengan badge warna `--color-warn` — jangan hijau (temuan = perhatian)

**Aksen discipline ketat**
- Ember hanya: cluster ring + node verdict + panah — tiga titik temuan
- Good/bad hanya dalam bar gini — data bicara
- Panel header semua cyan (seragam instrumen); TIDAK ada warna keempat

**Motion**
- Graph edges draw-in 400ms lalu cluster ring glow-pulse warn sekali; bar gini grow-out kiri→kanan; garis konvergen draw-in terakhir

**Speaker notes:** Satu temuan nyata per panel (overlap donor, sensitivitas mekanisme, celah bukti yang diakui) — semua dari run nyata di `examples/agent-trace-epoch10.md`, bukan ilustrasi kosong. Sebut: "ini analisis, bukan vibe — dan kalau data tidak ada, agent bilang tidak ada."
