# Slide 09 — BNB Chain Integration

---

## Caption (teks di slide — maks 35%)

```
BNB CHAIN FIRST-CLASS
The scanner's home network — and the verdict notary.

[ BSC 56 ]  [ opBNB 204 ]  [ BSC TESTNET 97 ]
USDT · USDC · FDUSD (18 dec) — native BNB + stablecoin balances via RPC
11 chains scanned · read-only · signs nothing
TesseraAttestations · BSC testnet 0x56e6…8427 · verified · no owner, no upgrade
```

---

## Visual Instructions (65%)

**Canvas & background**
- Void base; dot-field penuh — slide ini tentang jangkauan jaringan scanner
- Signal-cyan glow utama 20% center + amber `#e8b23a` BNB-echo glow kecil kanan — BNB = fokus, tapi tetap dalam bahasa visual Tessera (bukan branding BNB dominan)
- Grain ON

**Layout (hero: peta rantai + panel token)**
- **Atas — 3 chain chip besar (grid 3 kolom gap 24px):**
  - Tiap chip kartu `bg-raised rounded-xl border-line` tinggi 110px:
    - Nama chain Schibsted 600 20px bone + nomor chainid mono 12px cyan besar di kanan atas (`56` `204` `97`)
    - Bar status kecil: BSC 56 & opBNB 204 = bar cyan penuh + label mono 10px "live"; BSC testnet = bar cyan 60% + "staging"
  - Chip BSC 56 diberi border `--color-signal` 1.5px + `shadow` cyan lembut — chain utama menonjol dari 3-nya
- **Tengah — token row:** 3 pill `border-line bg-surface` mono 12px: `USDT 0x55d3…` `USDC 0x8AC7…` `FDUSD 0xc5f0…` + chip `18 decimals` ember — alamat truncated, full via link
- **Bawah — 11-chain strip:** baris 11 dot kecil (8px) bone-faint mewakili seluruh chain yang discan; 3 dot cyan lebih besar + label (BSC/opBNB/tBNB), 8 sisanya dot redup dengan label mono 9px muncul saat hover — visual "BNB di depan, ekosistem di belakang"
- **Baris notary (di atas footer):** kartu tipis `bg-surface border-line rounded-lg` lebar 100%: ikon `Stamp` 16px ember + mono 12px `TesseraAttestations · 0x56e6…8427 · BSC testnet 97` + chip `verified` good + teks mono 10px bone-dim "commit(keccak256(verdict)) — proves it existed and was never altered"
- **Footer strip 2 jaminan:** dua chip pill: `read-only scanner — signs nothing` (ikon `Eye` 14px) dan `notary: immutable, no admin, no custody` (ikon `ShieldCheck` 14px) — keduanya bone-dim, statement arsitektur, bukan klaim pemasaran

**Type discipline**
- Chainid angka = IBM Plex Mono cyan; nama chain Schibsted; alamat mono truncated
- Headline Instrument Serif 44px

**Motion**
- 3 chain chip rise stagger 150ms (56 dulu); dot 11-chain fade-in gelombang; border BSC 56 pulse cyan sekali

**Speaker notes:** "Scanner sekarang BNB-first — BSC, opBNB, dan testnet di baris depan, plus token 18-desimal USDT/USDC/FDUSD, read-only. Dan BNB Chain jadi notaris verdict: kontrak TesseraAttestations di BSC testnet, tanpa owner, tanpa upgrade — hash verdict tercatat, tidak bisa diubah diam-diam." Kalimat terakhir = positioning keamanan Tessera di konteks hackathon on-chain.
