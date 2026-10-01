# Slide 04 — How It Works

---

## Caption (teks di slide — maks 35%)

```
HOW IT WORKS
One question in. A traced verdict out.

ask ──▶ agent plans ──▶ tools execute ──▶ evidence streams ──▶ verdict
"should Octant fund X?"   model picks      in-process, live   every figure
                          the tools         over SSE           cites its call
```

---

## Visual Instructions (65%)

**Canvas & background**
- Void base; dot-field (constellation 24px) aktif penuh menggantikan tess-field — slide ini tentang alur data hidup, bukan struktur
- Signal-cyan glow garis tipis horizontal melintang di tengah slide — "data highway"
- Grain ON

**Layout (hero: pipeline horizontal 5 stage)**
- **5 node sejajar di garis highway cyan (gap 20px, tiap node kartu `bg-raised border-line rounded-lg` 150px lebar):**
  1. `ASK` — ikon `MessageSquare` 22px bone; subteks mono 10px "plain question"
  2. `PLAN` — ikon `BrainCircuit` 22px EMBER (satu-satunya node ember — keputusan AI di sini); subteks "model picks tools"
  3. `EXECUTE` — ikon `Terminal` 22px signal-cyan; subteks "in-process, no endpoints"
  4. `STREAM` — ikon `Radio` 22px signal-cyan + dot pulse; subteks "SSE tool→result"
  5. `VERDICT` — ikon `FileCheck` 22px `--color-good #5fd1a0`; subteks "cites every figure"
- **Koneksi antar node:** garis cyan solid 2px dengan partikel dot kecil bergerak kiri→kanan (animasi 1.5s loop, motion-reduce off) — data mengalir
- **Di bawah pipeline:** live trace strip — terminal card glass tinggi 120px berisi 4 baris log mono 11px dengan nomor langkah (rekaman asli `examples/agent-trace-epoch10.md`; rekaman tidak menyimpan timestamp, jangan mengarang jam):
  ```
  1 ▸ tool_call  get_project_history(0xe2F7…4AD1)
  1 ▸ result    epochs 8 → 10 · donors 80 → 52
  5 ▸ tool_call  simulate_mechanisms(epoch 10)
  ✓ ▸ text      "hold / investigate — −70% under 1-person-1-vote"
  ```
  - Warna log: `tool_call` cyan, `result` bone, `text` ember — konsisten dengan identitas
- **Node EXECUTE diberi badge pill kecil** "no tool endpoints exposed" mono 9px border-line — security statement visual

**Type discipline**
- Semua label pipeline: IBM Plex Mono; judul stage uppercase 12px tracking 0.22em
- Headline Instrument Serif 44px bone — satu serif besar, sisanya mono/sans

**Motion**
- Partikel pipeline mengalir kontinu; log strip types-in line-per-line 400ms/row
- Node menyala (border cyan) berurutan mengikuti log

**Speaker notes:** Jalan pipeline sekali dengan mata. Tekankan: "tools execute in-process — tidak ada tool endpoint yang terekspos network." Itu klausa keamanan utama.
