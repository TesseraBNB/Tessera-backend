# Slide 10 — Security & Reliability

---

## Caption (teks di slide — maks 35%)

```
SECURITY & RELIABILITY
Boring where it counts.

tools run in-process     provider fallback     retry · backoff ·
zero tool endpoints      xKiro → QwenCloud     cache with TTL
on the network           no single point       sources self-heal

go vet · golangci-lint · CI on every push · healthcheck /api/health
```

---

## Visual Instructions (65%)

**Canvas & background**
- Void base paling tenang: tess-field 20% hanya di pojok kiri-atas, grain 40% — keamanan = keheningan struktural
- Glow: satu warna `--color-good #5fd1a0` sangat redup 8% di seluruh frame — status hijau ambient, hampir tak terlihat
- Tidak ada warna bad/warn di slide ini

**Layout (split 60/40)**
- **Kiri (60%) — "no exposure" diagram:**
  - Ilustrasi proses: rounded rect besar bone-faint dashed label mono `GO PROCESS` berisi 4 kotak tool kecil (ikon 18px) — SEMUA di dalam satu boundary
  - Di luar boundary: ikon `Globe` dengan garis putus menuju boundary dan **tanda silang `X` merah-samar `--color-bad` 40%** di ujung — "network cannot reach tools"; hanya dua garis hijau good yang BOLEH masuk: `HTTPS in` dan `SSE out`
  - Label mono 11px bone-dim di bawah: "tools execute in-process · zero tool endpoints"
- **Kanan (40%) — stack 3 kartu jaminan** (`bg-surface border-line rounded-lg`):
  1. `Provider resilience` — ikon `Waypoints` 20px; mini diagram 2 jalur: `xkiro` (garis ember solid) → `qwencloud` (garis bone dashed); label "fallback, no single point"
  2. `Self-healing sources` — ikon `RefreshCw` 20px; 3 dot status good + label mono "retry · backoff · cache TTL"
  3. `CI hygiene` — ikon `CheckCheck` 20px; chip mono 10px: `go vet` `golangci-lint` `tests` `docker build` — empat chip sejajar
- **Footer:** healthcheck strip — mono 12px: `GET localhost:8080/api/health → 200` dengan dot hijau pulse — endpoint nyata di mesin juri

**Aksen**
- Hijau good satu-satunya warna (diagram boleh-lewat + dot status + healthcheck) — slide keamanan yang "hijau" secara visual adalah pesan itu sendiri
- Ember hanya di satu garis jalur xkiro — identitas brand jalan terus

**Motion**
- Minimal: boundary dashed draw-in 600ms; tanda X fade-in terakhir; dot healthcheck pulse 2s terus. Tidak ada animasi lain — boring = trust

**Speaker notes:** "Boring where it counts" — dibaca sambil senyum. Tools in-process, provider model punya fallback, sumber self-healing. Satu kalimat per kartu, lalu maju.
