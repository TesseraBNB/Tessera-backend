# Slide 11 — Roadmap

---

## Caption (teks di slide — maks 35%)

```
ROADMAP
From analyst to ambient intelligence.

SHIPPED ✓   BNB-first scanner · verdict notary on BSC testnet ·
            live trace UI · trust + mechanism tools · MCP surface + PDF reports
NEXT ▶      auto-notarize every verdict · USD-normalized balances ·
            scheduled epoch watch (agent runs itself when epochs close)
LATER ◼     verdict API for treasuries · multi-agent second opinions
            · funding-simulation playground
```

---

## Visual Instructions (65%)

**Canvas & background**
- Void base; tess-field 35% + dot-field overlay tipis — dua motif bertemu di horizon waktu
- Glow ember horizon kanan (masa depan agent) + cyan kecil kiri (data hari ini) — gradien arah kiri→kanan: cyan memudar, ember menguat

**Layout (hero: timeline horizontal 3 fase)**
- **Garis timeline** bone-faint 2px melintang tengah dengan arrowhead kanan; 3 marker:
  - **Marker 1 "Shipped" (kiri 22%):** dot solid signal-cyan 14px + overlay check `CheckCircle2` 18px good; fase card `bg-raised border-line rounded-xl` berisi 5 chip item mono 11px (`BNB scanner` `verdict notary` `live trace` `trust tools` `MCP + PDF`) — tiap chip punya dot check good kecil
  - **Marker 2 "Next" (tengah 52%):** dot outline cyan berdenyut (pulse ring 2.2s) — card berisi 3 chip item ikon `ArrowRight` bone-dim; item pertama `auto-notarize` diberi border cyan + glow — hari ini notarisasi verdict dilakukan manual lewat `cast send`, item ini membuat setiap verdict tercatat otomatis di BSC
  - **Marker 3 "Later" (kanan 80%):** dot bone-faint 10px; card redup `bg-surface` berisi 3 item mono 11px bone-faint TANPA ikon — vision quiet
- **Bawah timeline:** baseline gradient strip tipis cyan→ember→transparent
- **Badge khusus di marker 2:** chip pill `epoch watch` mono 10px ember-border — "agent runs itself" adalah fitur paling agent-native di roadmap, beri sorotan

**Hierarki pembacaan**
- Mata: marker 2 (pulse) → kiri (bukti cyan solid) → kanan (visi redup) — urutan argumen: sekarang hidup, dekat konkret, jauh imajiner

**Type**
- Fase header Schibsted 600 14px uppercase tracking 0.1em; semua item mono 11px; headline Instrument Serif 44px

**Motion**
- Timeline draw-in 600ms; marker 2 pulse mulai setelah garis tiba; chip epoch watch pop-in scale terakhir dengan glow ember sekali

**Speaker notes:** "Hari ini: scanner BNB-first, notaris verdict di BSC testnet, trace hidup. Berikutnya: setiap verdict otomatis dinotarisasi, USD-normalisasi, dan epoch watch — agent yang bangun sendiri tiap epoch tutup. Jauh: verdict API untuk treasury." Sebut MCP surface sebagai bonus dev-relations.
