# Slide 05 — Architecture

---

## Caption (teks di slide — maks 35%)

```
ARCHITECTURE
A Go engine behind a Next.js lens.

[ VERCEL ]        tessera-bnb.vercel.app — UI, SSE client, observatory view
[ YOUR MACHINE ]  Go service on localhost:8080 — /api/* JSON · /api/agent/* SSE
                  internal/agent → tool loop → Anthropic Messages API
                                   xKiro └ (fallback) QwenCloud · Claude optional
[ DATA ]          octant gitcoin retroPGF · scan_chain: BSC 56 ·
                  opBNB 204 · BSC testnet 97 · +8 chains
```

---

## Visual Instructions (65%)

**Canvas & background**
- Void base; tess-field 30% kiri-atas fade — arsitektur = struktur, mosaic kembali
- Grain ON; glow tidak perlu — slide ini diagram murni, gelap dan presisi

**Layout (3 layer horizontal band, stack vertikal, gap 24px)**
- **Band 1 — Frontend (20% tinggi):** kartu `bg-surface border-line rounded-xl` tipis lebar 100%
  - Kiri: chip mono "VERCEL" bone-faint; tengah: label Schibsted 15px "tessera-bnb.vercel.app · Next.js 16 · SSE client"; kanan: ikon `Globe` 20px bone-dim
- **Band 2 — Backend engine (hero, 50% tinggi):** kartu `bg-raised rounded-xl border-line` dengan `shadow` dalam
  - Header: chip mono "YOUR MACHINE" + label "Go service · localhost:8080 · no globals · DI App struct"
  - Body kiri: dua route chip: `/api/*` (mono cyan, subteks "JSON, fast, no LLM") dan `/api/agent/*` (mono ember, subteks "SSE tool_call → result → text") — dua jalur, dua warna identitas
  - Body kanan: mini-flow vertikal `internal/agent` → tool loop → `Messages API` → **xKiro** (node ember glow) dengan cabang dashed `(fallback) QwenCloud` — cabang fallback pakai garis putus bone-faint; label mono 9px di bawah node: "any Anthropic-compatible model"
  - Ikon `Server` 24px bone-dim di pojok
- **Band 3 — Data sources (20% tinggi):** grid chip 4 kolom:
  - 3 chip sumber funding (mono 11px bone-dim): `Octant` `Gitcoin` `RetroPGF`
  - 1 chip besar highlight `scan_chain` — border signal-cyan, isi: `BSC 56 · opBNB 204 · BSC testnet 97` mono cyan 11px + subteks "+8 chains, read-only" — BNB first-class, chip ini lebih besar dari 3 tetangganya (110% width)
- **Konektor:** garis vertikal 2px bone-faint antar band dengan arrowhead; jalur SSE diberi warna cyan, jalur JSON bone

**Detail wajib**
- Label "read-only" menempel di scan_chain — Tessera tidak menandatangani transaksi, itu harus terbaca dari struktur
- Tidak ada ikon dekoratif berlebih; hingga 1 ikon per band

**Motion**
- Band fade-in top→down 200ms stagger; node xKiro pulse ember sekali; chip scan_chain border-pulse cyan 2s

**Speaker notes:** "Lensa Next.js di Vercel, mesin Go jalan di laptop Anda di localhost:8080 — dan scanner-nya BSC-first: 56, opBNB 204, testnet 97." Sebut lisan: provider model cukup diganti lewat env (xKiro, QwenCloud, atau Claude), dengan fallback otomatis.
