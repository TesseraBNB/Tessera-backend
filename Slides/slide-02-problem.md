# Slide 02 — Problem

---

## Caption (teks di slide — maks 35%)

```
THE PROBLEM
Public-goods funding decides with narrative, not evidence.

96.9% of Octant epoch-10   decisions rest on      an analyst must check
funding came from the      write-ups, vibes,      7 sources by hand —
top 10% of donors          and social proof       and still guesses

→ fraud and waste hide in the gap between story and data
```

---

## Visual Instructions (65%)

**Canvas & background**
- Void/obsidian base; tess-field DI-FULL (opacity naik ke 6%) — masalah hadir di dalam struktur jaringan yang padat
- Dua warna status: `--color-warn #e8b23a` glow kiri (fragmentasi data), `--color-bad #ef5d5d` glow kanan bawah (risiko fraud) — keduanya redup 15%
- Grain ON

**Layout (split 55/45)**
- **Kiri (55%) — hero visual "evidence gap":**
  - Ilustrasi dua kolom yang tidak pernah bertemu:
    - Kolom "NARRATIVE": kartu raised `--color-raised` berisi ikon `Megaphone` 28px bone-dim + label mono "claims · write-ups · social proof" — kartu ini SEMU, blur ringan 1px
    - Kolom "DATA": kartu berisi ikon `Database` 28px signal-cyan + label mono "on-chain · grants · github" — kartu ini tajam
  - Di antara keduanya: **gap retak** — zigzag line `--color-bad` 2px dengan glow merah, label mono kecil "THE GAP" merah — visual metaphor slide ini
- **Kanan (45%) — stack 3 kartu masalah** (bg surface `--color-surface #0f1417`, border line, radius 14px):
  - Ikon Lucide 22px (`Banknote` `FileQuestion` `LayoutGrid`) warna bone-dim; judul Schibsted 600 16px bone; subteks mono 12px bone-dim max 1 baris
  - Kartu 1 menampilkan datanya, bukan hanya menyebut: bar horizontal 100% dengan segmen 96.9% (top 10% donor) vs 3.1% (90% sisanya) — angka dari `/api/detect-anomalies?epoch=10` (`examples/agent-trace-epoch10.md`)
  - Kartu 3: "7 sources" = Octant, Gitcoin, OSO, GitHub, forum, RetroPGF, 11 EVM chains — sumber yang dibaca tool Tessera
  - Kartu ke-2 ("vibes") diberi border `--color-warn` tipis — masalah paling memalukan diprioritaskan
- **Footer strip:** satu kalimat takeaway dalam Instrument Serif italic 19px bone: "fraud and waste hide in the gap" — tanpa kartu, hanging quote di ruang kosong

**Aksen discipline**
- Merah/warn hanya di gap-line + border kartu 2; SEMUA teks body tetap bone (kontras AA di obsidian)
- Signal-cyan TIDAK dipakai di slide masalah — cyan = solusi/live data

**Motion**
- Gap zigzag draw-in 500ms dengan glow pulse `--color-bad` sekali; kartu fade-in stagger 140ms setelahnya

**Speaker notes:** "Di epoch 10 Octant, 96,9% dana datang dari 10% donor teratas — dan keputusan masih berbasis narasi." Sebut angka, lalu diam.
