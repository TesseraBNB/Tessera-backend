# Slide 03 — Solution

---

## Caption (teks di slide — maks 35%)

```
THE SOLUTION
A skeptical analyst that never sleeps — and shows its work.

REAL TOOL-CALLING AGENT       EVIDENCE-BOUND            LIVE TRACE
the model decides             every figure traces       watch the agent
which tools to run;           to a tool call —          call tools over SSE
tools execute in-process      nothing is invented       in real time
```

---

## Visual Instructions (65%)

**Canvas & background**
- Void base; tess-field kembali fade 40% — solusi merapikan kekacauan slide 02
- Ember glow utama center 25% + signal-cyan glow kecil kanan-atas 12% — dua identitas (agent = ember, data-live = cyan) bertemu
- Grain ON

**Layout (hero: agent core + 3 pilar)**
- **Pusat (40% lebar):** "agent core" — lingkaran konsentris:
  - Inti: disc ember `#e8633a` 80px dengan ikon `BrainCircuit` bone 36px
  - Ring orbit 1: garis putus signal-cyan 1.5px berisi 4 chip tool mini mengorbit (ikon `History` `Network` `ScanSearch` `FlaskConical` 16px, label mono 9px): `get_project_history` `get_trust_profile` `scan_chain` `simulate_mechanisms` — nama tool ASLI dari repo
  - Ring orbit 2: garis bone-faint solid dengan 3 chip sumber (`Octant` `Gitcoin` `RetroPGF`)
  - Orbit berputar pelan 40s (motion-reduce: static)
- **Kanan (55%):** 3 pilar kartu vertikal stack, gap 16px:
  - Tiap kartu: `bg-surface border-line rounded-lg`, ikon 20px (`Bot` `Crosshair` `Radio`) ember, judul Schibsted 600 15px bone, body mono 12px bone-dim 1 baris
  - Kartu "LIVE TRACE": border signal-cyan + dot indikator cyan berdenyut 2s = hidup
- **Bawah:** verdict bar — strip glass `bg-raised` berisi mono 12px: `VERDICT: HOLD / INVESTIGATE · whale dependency 0.46 · −70% under 1-person-1-vote` dengan badge `--color-warn` — output nyata dari `examples/agent-trace-epoch10.md` (epoch 10, proyek `0xe2F7…4AD1`), bukan mock

**Aksen**
- Ember = agent/brand; cyan = live/data; keduanya muncul SIMETRIS di slide ini (perkenalan pasangan identitas)
- Bone untuk semua body text

**Motion**
- Inti ember pulse halus 3s; orbit tools rotate 40s linear; verdict bar types-in 800ms

**Speaker notes:** "Tool-calling agent sungguhan — bukan chatbot wrap. Tools jalan in-process, setiap angka bisa dilacak ke tool call." Dua kalimat, selesai.
