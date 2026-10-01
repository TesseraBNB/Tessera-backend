# Slide 07 — Live Agent Trace

---

## Caption (teks di slide — maks 35%)

```
LIVE TRACE
The agent shows its work — in real time.

▸ tool_call  get_trust_profile(epoch 10, 0xe2F7…4AD1)   4
▸ result     diversity 0.30 · whale 0.46 · jaccard 0.46   4
▸ tool_call  scan_chain(0xe2F7…4AD1)                    6
▸ result     11 chains · active on 2 · 12 txs · EOA       6
▸ text       "Hold / investigate — donors 80 → 52."
```

---

## Visual Instructions (65%)

**Canvas & background**
- Void base paling gelap (naikkan ke `--color-void` penuh, band body dihilangkan) — slide ini adalah "malam observatorium"
- Signal-cyan glow tipis di belakang terminal, 15%, blur besar — monitor menyala dalam gelap
- Grain ON 60%; tess-field off

**Layout (hero: satu terminal besar, full-bleed 80% lebar, center)**
- **Terminal card:** `bg #0f1417`, border `--color-line-bright #33404a` 1px, `rounded-xl`, `shadow` dalam, tinggi 65% slide
  - Header bar 36px: 3 dot traffic (bone-faint) + judul mono 11px "tessera · agent trace — live" + dot indikator cyan pulse 2s di kanan
  - Body: 8–10 baris log stream mono 13px/22px, padding 24px:
    - `▸ tool_call` = signal-cyan
    - `▸ result` = bone `#ece7da`
    - `▸ text` = ember `#e8633a`
    - nomor langkah tool call = bone-faint kanan-align (rekaman asli `examples/agent-trace-epoch10.md` tidak menyimpan timestamp — jangan mengarang jam)
  - Baris VERDICT terakhir (jika masuk): highlight bar `bg color-mix(signal 8%)` + border-left cyan 2px — momen kesimpulan
- **Sisi kanan terminal (kolom 25%):** 3 mini-stat vertikal dari log yang sedang stream: `diversity 0.30` `whale 0.46` `chains 11` — angka Schibsted 600 20px bone + label mono 10px bone-faint; angka whale diberi warna `--color-warn`
- **Bawah:** caption kecil Instrument Serif italic 16px bone-dim: "watch the reasoning happen — nothing arrives pre-written" — satu kalimat, tanpa kartu

**Interaksi (HTML render)**
- Log types-in baris-per-baris 500ms/row dengan cursor block berkedip cyan; jika presentasi statis: tangkap frame mid-stream dengan 5 baris terlihat + cursor aktif

**Motion**
- Cursor blink 1s; stat angka counter-up sinkron saat baris result-nya muncul; dot indikator live pulse terus

**Speaker notes:** Ini momen demo statis yang terasa hidup. Kalau presentasi live: putar trace SSE sungguhan 30 detik. Sebut: "setiap angka di layar lahir dari baris tool_call yang bisa Anda lihat."
