# Slide 06 — The Tools

---

## Caption (teks di slide — maks 35%)

```
THE TOOLS
Forensic instruments, not dashboards.

get_project_history   get_trust_profile    scan_chain          simulate_mechanisms
funding record        shannon entropy      11 chains, BSC      4 QF modes · gini
across epochs         jaccard overlap      opBNB first-class   top-share replay
```

---

## Visual Instructions (65%)

**Canvas & background**
- Void base; dot-field kanan; grain ON; tanpa glow — instrumen dilihat di cahaya netral

**Layout (hero: 4 kartu instrumen, grid 2×2, gap 20px, tiap kartu besar 46% lebar)**
- **Semua kartu sebangun** (`bg-raised border-line rounded-xl`, padding 22px) — disiplin "instrumen lab":
  - Header kartu: nama tool dalam IBM Plex Mono 14px `--color-signal` (nama fungsi = data) + ikon 22px bone-dim di kanan
  - Baris spec: 2–3 chip mini mono 10px bone-dim (`shannon` `jaccard` `whale dep` dll)
  - Ilustrasi output mini per kartu (INILAH pembeda slide — tiap kartu menunjukkan bentuk datanya):
    1. `get_project_history`: sparkline 12 titik bone dengan 3 titik ember (epoch penting)
    2. `get_trust_profile`: donut 3 segmen (warn `#e8b23a` / bad `#ef5d5d` / good `#5fd1a0`) proporsi 50/30/20 + label "diversity 0.30" (Shannon ternormalisasi 0–1; nilai asli epoch 10 dari `examples/agent-trace-epoch10.md`)
    3. `scan_chain`: 3 bar horizontal (BSC panjang cyan, opBNB sedang, testnet pendek) + label "11 chains"
    4. `simulate_mechanisms`: 4 bar mini grup warna bone dengan satu grup overlay ember (mode terpilih) + label "gini 0.40 → 0.17" (Standard QF → Equal weight, epoch 10)
  - Kartu 3 (`scan_chain`) diberi border signal-cyan 1px + chip pojok "BNB FIRST-CLASS" mono 9px cyan — satu-satunya aksen border di grid
- **Footer:** strip mono 11px bone-faint: "tools execute in-process · no endpoints · every call traced" — tiga jaminan dalam satu baris

**Type discipline**
- Nama tool mono cyan 14px; spec chip mono 10px; TIDAK ada judul Schibsted besar di dalam kartu — instrumen berbicara lewat nama fungsinya
- Headline slide: Instrument Serif 44px

**Aksen**
- Ember hanya di: 3 titik sparkline, grup bar terpilih, dan tak lain — cyan punya peran data-signal
- Good/warn/bad hanya muncuk dalam donut trust — warna status tidak menghias

**Motion**
- Kartu rise-in stagger 120ms; ilustrasi mini draw-in setelah kartunya (sparkline animate, donut sweep, bar grow) 400ms each

**Speaker notes:** "Empat instrumen forensik — setiap angka yang muncul di verdict bisa dilacak ke salah satu dari empat ini." Sebut shannon entropy & whale dependency sebagai contoh bahasa analis skeptis.
