"""Render the Tessera deck from the Slides/*.md specs into deck.html and a PDF.

    python collect_data.py   # refresh data.json from Octant + Tessera's tools
    python build.py          # deck.html + ../Tessera_Deck.pdf (Slides/) (needs Chrome or Edge)
    python build.py --html   # deck.html only

Every figure is read from data.json (see collect_data.py); nothing is typed in.
Visual system: Slides/design.md ("Obsidian Observatory", dark only).
"""

import base64
import json
import math
import pathlib
import random
import re
import shutil
import struct
import subprocess
import sys
import zlib

HERE = pathlib.Path(__file__).resolve().parent
D = json.loads((HERE / "data.json").read_text(encoding="utf-8"))
ICONS = json.loads((HERE / "icons.json").read_text(encoding="utf-8"))["icons"]
W, H = 1376, 768
TOTAL = 12

# Colors (frontend/src/app/globals.css @theme)
VOID, BG, SURFACE, RAISED = "#07090a", "#0a0c0e", "#0f1417", "#151c21"
LINE, LINE_BRIGHT = "#222b31", "#33404a"
BONE, BONE_DIM, BONE_FAINT = "#ece7da", "#9ba39f", "#626c6a"
EMBER, EMBER_BRIGHT, EMBER_DEEP = "#e8633a", "#ff7d52", "#a83c1c"
SIGNAL, SIGNAL_BRIGHT = "#46d6d0", "#74f2ec"
GOOD, WARN, BAD = "#5fd1a0", "#e8b23a", "#ef5d5d"

FOCUS = "0xe2F7cF9C2b12c0BfcdAB571F9E50418fC08F4AD1"  # checksummed form of D["focus"]
LONG_RUN = "0x9531C059098e3d194fF87FebB587aB07B30B1306"


def short(addr):
    return f"{addr[:6]}…{addr[-4:]}"


def num(x, d=2):
    return f"{x:.{d}f}"


# ─── derived figures (all from data.json) ───────────────────────────────────
TRUST = D["trust"]
DIVERSITY = TRUST["DonorDiversity"]
WHALE = TRUST["WhaleDepRatio"]
JACCARD = TRUST["CoordinationRisk"]
ANOM = D["anomalies"]
MECH = {m["name"]: m for m in D["mechanisms"]}
STD = MECH["Standard Quadratic Funding"]
EQ = MECH["Equal Weight (1-person-1-vote)"]
FH = {h["Epoch"]: h for h in D["focus_history"]}
E8, E10 = FH[8], FH[10]
LONG = {h["Epoch"]: h for h in D["long_run_history"]["epochs"]}
SCAN = D["scan"]
RANK = D["rank"]
N_PROJECTS = len(RANK)
SCAN_ACTIVE = [c for c in SCAN["chains"] if c["txCount"] or c["balance"]]


# ─── primitives ─────────────────────────────────────────────────────────────
def icon(name, size=22, color="currentColor", sw=1.8):
    parts = []
    for tag, attrs in ICONS[name]:
        a = " ".join(f'{k}="{v}"' for k, v in attrs.items())
        parts.append(f"<{tag} {a}/>")
    return (f'<svg width="{size}" height="{size}" viewBox="0 0 24 24" fill="none" stroke="{color}" '
            f'stroke-width="{sw}" stroke-linecap="round" stroke-linejoin="round">{"".join(parts)}</svg>')


def mark(size=40, tile=True):
    """The Tessera mosaic mark (frontend/src/app/icon.svg)."""
    t = (f'<rect width="64" height="64" rx="14" fill="{BG}"/>'
         f'<rect x="0.5" y="0.5" width="63" height="63" rx="13.5" stroke="{LINE}"/>') if tile else ""
    return (f'<svg width="{size}" height="{size}" viewBox="0 0 64 64" fill="none">{t}'
            f'<polygon points="32,9 43,20 32,31 21,20" fill="{EMBER}"/>'
            f'<polygon points="44,21 55,32 44,43 33,32" fill="{SIGNAL}"/>'
            f'<polygon points="32,33 43,44 32,55 21,44" fill="{BONE}"/>'
            f'<polygon points="20,21 31,32 20,43 9,32" fill="{BONE_DIM}"/></svg>')


def header(kicker, headline, top=60):
    return (f'<header class="hdr" style="top:{top}px"><div class="kicker">{kicker}</div>'
            f'<h2 class="headline">{headline}</h2></header>')


def foot(n):
    return (f'<footer class="foot"><span class="foot-l">{mark(16, tile=False)}<span>TESSERA</span></span>'
            f'<span class="foot-r">{n:02d} / {TOTAL}</span></footer>')


_glow_png = {}


def _png(rgba, n):
    """Minimal RGBA PNG encoder (no Pillow needed)."""
    raw = b"".join(b"\x00" + rgba[y * n * 4:(y + 1) * n * 4] for y in range(n))

    def chunk(t, d):
        return struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d) & 0xFFFFFFFF)

    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", n, n, 8, 6, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b""))


def grain_tile():
    if "grain" not in _glow_png:
        n, rnd = 96, random.Random(42)
        px = bytearray()
        for _ in range(n * n):
            v = rnd.randrange(256)
            px += bytes((v, v, v, 22))
        _glow_png["grain"] = base64.b64encode(_png(bytes(px), n)).decode()
    return _glow_png["grain"]


def glow(x, y, size, color, alpha):
    """Soft light as a small radial-alpha bitmap scaled up. Vector radial shadings
    print with a hairline seam from the page edge to their centre in some viewers."""
    if color not in _glow_png:
        n = 128
        r, g, b = (int(color[i:i + 2], 16) for i in (1, 3, 5))
        px = bytearray()
        for j in range(n):
            for i in range(n):
                t = min(math.hypot(i + 0.5 - n / 2, j + 0.5 - n / 2) / (n / 2), 1.0)
                px += bytes((r, g, b, round(255 * (1 - t) ** 1.6)))
        _glow_png[color] = base64.b64encode(_png(bytes(px), n)).decode()
    return (f'<img class="glow" alt="" src="data:image/png;base64,{_glow_png[color]}" '
            f'style="left:{x - size // 2}px;top:{y - size // 2}px;width:{size}px;height:{size}px;opacity:{alpha:.3f}">')


_ids = iter(range(1, 1000))


def layers(tess=0.0, dots=0.0, grain=0.5, tess_mask=None, dots_mask=None):
    """Background motifs. The tessellation is an SVG pattern: Chrome's PDF output
    coarsens dense repeating-linear-gradients into a few stray lines."""
    out = []
    if tess:
        m = f"-webkit-mask-image:{tess_mask};mask-image:{tess_mask};" if tess_mask else ""
        pid = f"tess{next(_ids)}"
        out.append(f'<svg class="layer" style="opacity:{min(tess, 1.0)};{m}" width="{W}" height="{H}">'
                   f'<defs><pattern id="{pid}" width="30" height="52" patternUnits="userSpaceOnUse">'
                   f'<path d="M0,0 L30,52 M30,0 L0,52 M0,0 H30" stroke="{BONE}" stroke-opacity="{0.045 * tess:.3f}" stroke-width="1"/>'
                   f'</pattern></defs><rect width="{W}" height="{H}" fill="url(#{pid})"/></svg>')
    if dots:
        m = f"-webkit-mask-image:{dots_mask};mask-image:{dots_mask};" if dots_mask else ""
        out.append(f'<div class="layer dots" style="opacity:{dots};{m}"></div>')
    if grain:
        out.append(f'<div class="layer" style="opacity:{grain};background-image:url(data:image/png;base64,{grain_tile()});'
                   f'background-size:96px 96px"></div>')
    return "".join(out)


# ─── data visuals ───────────────────────────────────────────────────────────
def trust_graph(w, h, pad=22):
    g = D["graph"]
    nodes = {n["address"]: n for n in g["nodes"]}
    maxf = max(n["funding_eth"] for n in nodes.values())
    xs = [n["x"] for n in nodes.values()]
    ys = [n["y"] for n in nodes.values()]
    sx = (w - 2 * pad) / (max(xs) - min(xs))
    sy = (h - 2 * pad) / (max(ys) - min(ys))

    def p(a):
        n = nodes[a]
        return pad + (n["x"] - min(xs)) * sx, pad + (n["y"] - min(ys)) * sy

    focus, partner = g["focus"], g["focus_partner"]
    out = []
    for e in sorted(g["edges"], key=lambda e: e["jaccard"]):
        if e["jaccard"] < 0.3 or {e["a"], e["b"]} == {focus, partner}:
            continue
        (x1, y1), (x2, y2) = p(e["a"]), p(e["b"])
        a = min(0.08 + (e["jaccard"] - 0.3) * 1.6, 0.42)
        out.append(f'<line x1="{x1:.1f}" y1="{y1:.1f}" x2="{x2:.1f}" y2="{y2:.1f}" stroke="{BONE}" '
                   f'stroke-opacity="{a:.2f}" stroke-width="{0.7 + (e["jaccard"] - 0.3) * 6:.2f}"/>')
    (fx, fy), (px, py) = p(focus), p(partner)
    out.append(f'<line x1="{fx:.1f}" y1="{fy:.1f}" x2="{px:.1f}" y2="{py:.1f}" stroke="{EMBER}" stroke-width="3" '
               f'filter="url(#emberglow)"/>')
    ex, ey = px - fx, py - fy
    el = math.hypot(ex, ey) or 1
    nx, ny = -ey / el, ex / el  # unit normal: overlap label sits beside the edge, off the nodes
    if nx > 0:
        nx, ny = -nx, -ny
    mx, my = (fx + px) / 2 + nx * 34, (fy + py) / 2 + ny * 34
    out.append(f'<rect x="{mx - 24:.1f}" y="{my - 12:.1f}" width="48" height="22" rx="6" fill="{VOID}" stroke="{EMBER}" stroke-opacity=".6"/>'
               f'<text x="{mx:.1f}" y="{my + 4:.1f}" text-anchor="middle" class="svg-mono" fill="{EMBER_BRIGHT}" font-size="12">{num(g["focus_max_jaccard"])}</text>')
    for a, n in nodes.items():
        x, y = p(a)
        r = 3.5 + 11 * math.sqrt(n["funding_eth"] / maxf)
        if a == focus:
            out.append(f'<circle cx="{x:.1f}" cy="{y:.1f}" r="{r + 7:.1f}" fill="{EMBER}" fill-opacity=".14"/>'
                       f'<circle cx="{x:.1f}" cy="{y:.1f}" r="{r:.1f}" fill="{EMBER}"/>')
        elif a == partner:
            out.append(f'<circle cx="{x:.1f}" cy="{y:.1f}" r="{r:.1f}" fill="{EMBER_DEEP}" stroke="{EMBER}" stroke-width="1.5"/>')
        else:
            out.append(f'<circle cx="{x:.1f}" cy="{y:.1f}" r="{r:.1f}" fill="{LINE_BRIGHT}" stroke="{BONE_FAINT}" stroke-width="1"/>')
    for a, label in ((focus, "#1"), (partner, "#2")):
        x, y = p(a)
        r = 3.5 + 11 * math.sqrt(nodes[a]["funding_eth"] / maxf)
        out.append(f'<text x="{x + r + 6:.1f}" y="{y + 4:.1f}" class="svg-mono" fill="{BONE}" font-size="12">{label}</text>')
    defs = (f'<defs><filter id="emberglow" x="-50%" y="-50%" width="200%" height="200%">'
            f'<feGaussianBlur stdDeviation="2.5" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs>')
    return f'<svg width="{w}" height="{h}" viewBox="0 0 {w} {h}">{defs}{"".join(out)}</svg>'


def sparkline(w, h):
    """Donors per epoch for the long-running project (FINDINGS.md finding 2)."""
    pad_l, pad_r, pad_t, pad_b = 34, 18, 22, 26
    ymax = 200

    def X(e):
        return pad_l + (e - 1) * (w - pad_l - pad_r) / 9

    def Y(v):
        return pad_t + (1 - v / ymax) * (h - pad_t - pad_b)

    out = []
    for v in (0, 100, 200):
        out.append(f'<line x1="{pad_l}" x2="{w - pad_r}" y1="{Y(v):.1f}" y2="{Y(v):.1f}" stroke="{LINE}" stroke-width="1"/>'
                   f'<text x="{pad_l - 8}" y="{Y(v) + 4:.1f}" text-anchor="end" class="svg-mono" fill="{BONE_FAINT}" font-size="10">{v}</text>')
    run = [e for e in range(1, 11) if e in LONG]
    contiguous = [e for e in run if e <= 6]
    pts = " ".join(f"{X(e):.1f},{Y(LONG[e]['Donors']):.1f}" for e in contiguous)
    area = f"{X(contiguous[0]):.1f},{Y(0):.1f} {pts} {X(contiguous[-1]):.1f},{Y(0):.1f}"
    out.append(f'<polygon points="{area}" fill="{BONE}" fill-opacity=".05"/>')
    out.append(f'<polyline points="{pts}" fill="none" stroke="{BONE_DIM}" stroke-width="2"/>')
    last_contig, last = contiguous[-1], run[-1]
    out.append(f'<line x1="{X(last_contig):.1f}" y1="{Y(LONG[last_contig]["Donors"]):.1f}" x2="{X(last):.1f}" '
               f'y2="{Y(LONG[last]["Donors"]):.1f}" stroke="{BONE_FAINT}" stroke-width="1.5" stroke-dasharray="3 5"/>')
    out.append(f'<text x="{(X(7) + X(9)) / 2:.1f}" y="{Y(150):.1f}" text-anchor="middle" class="svg-mono" fill="{BONE_FAINT}" font-size="10">'
               f'not funded E7–E9</text>')
    for e in run:
        v = LONG[e]["Donors"]
        hot = e in (run[0], run[-1])
        out.append(f'<circle cx="{X(e):.1f}" cy="{Y(v):.1f}" r="{4.5 if hot else 3}" fill="{EMBER if hot else BONE}"/>')
        if hot:
            first = e == run[0]
            out.append(f'<text x="{X(e) + (10 if first else 0):.1f}" y="{Y(v) + (4 if first else -10):.1f}" '
                       f'text-anchor="{"start" if first else "middle"}" class="svg-mono" fill="{EMBER_BRIGHT}" font-size="12">{v}</text>')
    for e in range(1, 11):
        out.append(f'<text x="{X(e):.1f}" y="{h - 6}" text-anchor="middle" class="svg-mono" fill="{BONE_FAINT}" font-size="10">E{e}</text>')
    return f'<svg width="{w}" height="{h}" viewBox="0 0 {w} {h}">{"".join(out)}</svg>'


def donut(size, frac):
    r = size / 2 - 12
    c = 2 * math.pi * r
    cx = cy = size / 2
    a = frac * c
    return (f'<svg width="{size}" height="{size}" viewBox="0 0 {size} {size}">'
            f'<circle cx="{cx}" cy="{cy}" r="{r:.1f}" fill="none" stroke="{GOOD}" stroke-opacity=".75" stroke-width="16"/>'
            f'<circle cx="{cx}" cy="{cy}" r="{r:.1f}" fill="none" stroke="{WARN}" stroke-width="16" '
            f'stroke-dasharray="{a:.1f} {c - a:.1f}" transform="rotate(-90 {cx} {cy})"/>'
            f'<text x="{cx}" y="{cy + 6}" text-anchor="middle" class="svg-display" fill="{BONE}" font-size="30">{num(DIVERSITY)}</text>'
            f'<text x="{cx}" y="{cy + 24}" text-anchor="middle" class="svg-mono" fill="{BONE_FAINT}" font-size="10">DIVERSITY</text></svg>')


CHAIN_SHORT = {"BNB Smart Chain": "BSC", "opBNB": "opBNB", "Ethereum": "ETH", "Base": "Base", "Optimism": "OP",
               "Arbitrum": "Arb", "Mantle": "Mantle", "Scroll": "Scroll", "Linea": "Linea", "zkSync Era": "zkSync",
               "BSC Testnet": "tBNB"}
BNB_IDS = {56, 204, 97}


def chain_bars(w, h):
    chains = SCAN["chains"]
    n = len(chains)
    col = w / n
    maxtx = max(c["txCount"] for c in chains) or 1
    base = h - 24
    out = []
    for i, c in enumerate(chains):
        x = i * col + col / 2
        bh = (h - 52) * c["txCount"] / maxtx
        bnb = c["chainId"] in BNB_IDS
        if bnb:
            out.append(f'<rect x="{i * col + 3:.1f}" y="4" width="{col - 6:.1f}" height="{h - 8}" rx="6" fill="{SIGNAL}" '
                       f'fill-opacity=".06" stroke="{SIGNAL}" stroke-opacity=".45"/>')
        if c["txCount"]:
            out.append(f'<rect x="{x - 9:.1f}" y="{base - bh:.1f}" width="18" height="{bh:.1f}" rx="3" fill="{BONE_DIM}"/>'
                       f'<text x="{x:.1f}" y="{base - bh - 6:.1f}" text-anchor="middle" class="svg-mono" fill="{BONE}" font-size="11">{c["txCount"]}</text>')
        else:
            out.append(f'<rect x="{x - 9:.1f}" y="{base - 2}" width="18" height="2" rx="1" fill="{LINE_BRIGHT}"/>')
        out.append(f'<text x="{x:.1f}" y="{h - 8}" text-anchor="middle" class="svg-mono" '
                   f'fill="{SIGNAL_BRIGHT if bnb else BONE_FAINT}" font-size="10">{CHAIN_SHORT.get(c["chain"], c["chain"])}</text>')
    return f'<svg width="{w}" height="{h}" viewBox="0 0 {w} {h}">{"".join(out)}</svg>'


GINI_ROWS = [("Standard QF", "Standard Quadratic Funding"), ("Capped QF 10%", "Capped QF (10% cap)"),
             ("Trust-weighted QF", "Trust-Weighted QF"), ("Equal weight", "Equal Weight (1-person-1-vote)")]


def gini_bars(w, row_h=30):
    label_w, val_w = 150, 56
    bar_w = w - label_w - val_w
    out = []
    for i, (label, key) in enumerate(GINI_ROWS):
        g = MECH[key]["gini"]
        y = i * row_h
        hot = key == EQ["name"]
        out.append(f'<text x="0" y="{y + 15}" class="svg-mono" fill="{BONE if hot else BONE_DIM}" font-size="12">{label}</text>'
                   f'<rect x="{label_w}" y="{y + 4}" width="{bar_w}" height="12" rx="6" fill="{VOID}"/>'
                   f'<rect x="{label_w}" y="{y + 4}" width="{bar_w * g / 0.42:.1f}" height="12" rx="6" fill="{EMBER if hot else BONE_FAINT}"/>'
                   f'<text x="{w}" y="{y + 15}" text-anchor="end" class="svg-mono" fill="{BONE if hot else BONE_DIM}" font-size="12">{num(g, 3)}</text>')
    return f'<svg width="{w}" height="{len(GINI_ROWS) * row_h}" viewBox="0 0 {w} {len(GINI_ROWS) * row_h}">{"".join(out)}</svg>'


# ─── slides ─────────────────────────────────────────────────────────────────
def s01():
    chips = "".join(f'<span class="chip chip-signal">[ {c} ]</span>' for c in ("BSC 56", "opBNB 204", "BSC TESTNET 97"))
    return f'''<section class="slide" style="background:{VOID}">
{layers(tess=1, dots=1, grain=.5, tess_mask="radial-gradient(60% 70% at 50% 45%, #000 20%, transparent 80%)",
        dots_mask="radial-gradient(35% 45% at 88% 88%, #000, transparent)")}
{glow(170, 120, 1100, EMBER, .26)}
<div class="center">
  <div style="margin-bottom:26px">{mark(68)}</div>
  <h1 class="wordmark">TESSERA</h1>
  <p class="tagline">An autonomous agent for public-goods funding intelligence.</p>
  <p class="motto">Evidence over narrative.</p>
  <div class="chips" style="margin-top:36px">{chips}<span class="chip-note">— first-class networks</span></div>
</div>
<div class="title-foot">github.com/TesseraBNB · tessera-bnb.vercel.app · backend runs on your machine</div>
</section>'''


def s02():
    top10 = ANOM["whaleConcentration"] * 100
    sources = ["Octant", "Gitcoin", "OSO", "GitHub", "forum", "RetroPGF", "11 chains"]
    crack = "M40,0 L18,60 L52,118 L22,182 L58,246 L26,306 L50,360"
    return f'''<section class="slide">
{layers(tess=1.5, grain=.5)}
{glow(240, 420, 760, WARN, .13)}{glow(1200, 760, 700, BAD, .12)}
{header("The problem", "Public-goods funding decides with<br>narrative, not evidence.")}
<div class="abs" style="left:88px;top:252px;width:250px;height:330px;filter:blur(1.1px);opacity:.72">
  <div class="card col-card">{icon("megaphone", 30, BONE_DIM)}<div class="kicker" style="margin-top:18px">Narrative</div>
  <div class="mono big-list">claims ·<br>write-ups ·<br>social proof</div></div></div>
<svg class="abs" style="left:350px;top:248px" width="80" height="360" viewBox="0 0 80 360">
  <defs><filter id="redglow" x="-80%" y="-10%" width="260%" height="120%"><feGaussianBlur stdDeviation="4" result="b"/>
  <feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs>
  <path d="{crack}" fill="none" stroke="{BAD}" stroke-width="2.5" filter="url(#redglow)"/></svg>
<div class="abs mono" style="left:356px;top:412px;width:70px;text-align:center;color:{BAD};font-size:13px;letter-spacing:.2em;background:{BG};padding:6px 0">THE<br>GAP</div>
<div class="abs" style="left:442px;top:252px;width:250px;height:330px">
  <div class="card col-card" style="border-color:{LINE_BRIGHT}">{icon("database", 30, BONE)}<div class="kicker" style="margin-top:18px;color:{BONE}">Data</div>
  <div class="mono big-list" style="color:{BONE}">on-chain ·<br>grants ·<br>github</div></div></div>
<div class="abs" style="left:770px;top:236px;width:518px">
  <div class="card row-card">
    <div class="row-head">{icon("banknote", 24, BONE_DIM)}<span class="row-title">{top10:.1f}% of Octant epoch-10 funding</span></div>
    <div class="mono row-sub">came from the top 10% of {ANOM["uniqueDonors"]} donors</div>
    <div class="split-bar"><span style="width:{top10:.1f}%;background:{WARN}"></span><span style="width:{100 - top10:.1f}%;background:{BONE_FAINT}"></span></div>
    <div class="split-legend mono"><span>top 10% · {top10:.1f}%</span><span>other 90% · {100 - top10:.1f}%</span></div>
  </div>
  <div class="card row-card" style="border-color:rgba(232,178,58,.55)">
    <div class="row-head">{icon("file-question-mark", 24, BONE_DIM)}<span class="row-title">Decisions rest on narrative</span></div>
    <div class="mono row-sub">write-ups, vibes and social proof</div>
  </div>
  <div class="card row-card">
    <div class="row-head">{icon("layout-grid", 24, BONE_DIM)}<span class="row-title">{len(sources)} sources, checked by hand</span></div>
    <div class="mini-chips">{"".join(f'<span class="mini-chip">{s}</span>' for s in sources)}</div>
  </div>
</div>
<p class="abs quote" style="left:88px;top:626px">Fraud and waste hide in the gap between story and data.</p>
{foot(2)}
</section>'''


def s03():
    cx, cy, r_in, r_out = 330, 470, 132, 196
    tools = [("get_trust_profile", 340), ("scan_chain", 20), ("simulate_mechanisms", 160), ("get_project_history", 200)]
    sources = [("Octant", 250), ("Gitcoin", 300), ("RetroPGF", 40), ("11 EVM chains", 140)]

    def at(r, deg):
        a = math.radians(deg)
        return cx + r * math.cos(a), cy + r * math.sin(a)

    tool_html = "".join(
        f'<div class="orbit-chip mono" style="left:{at(r_in, d)[0]:.0f}px;top:{at(r_in, d)[1]:.0f}px">{t}</div>' for t, d in tools)
    src_html = "".join(
        f'<div class="orbit-src mono" style="left:{at(r_out, d)[0]:.0f}px;top:{at(r_out, d)[1]:.0f}px">{s}</div>' for s, d in sources)
    pillars = [
        ("bot", EMBER, "Real tool-calling agent", "The model decides which tools to run; Tessera executes them in-process.", ""),
        ("crosshair", EMBER, "Evidence-bound", "Every figure traces to a tool call — nothing is invented.", ""),
        ("radio", SIGNAL, "Live trace", "Watch the agent call tools over SSE, in real time.", f"border-color:rgba(70,214,208,.55)"),
    ]
    pill_html = "".join(
        f'<div class="card pillar" style="{st}"><div class="pillar-icon">{icon(ic, 26, col)}</div>'
        f'<div><div class="pillar-title mono">{t}</div><div class="pillar-body">{b}</div></div>'
        f'{"<span class=pulse-dot></span>" if col == SIGNAL else ""}</div>' for ic, col, t, b, st in pillars)
    return f'''<section class="slide">
{layers(tess=1, grain=.5, tess_mask="radial-gradient(70% 80% at 30% 60%, #000 20%, transparent 80%)")}
{glow(cx, cy, 560, EMBER, .24)}{glow(1150, 170, 420, SIGNAL, .10)}
{header("The solution", "A skeptical analyst that never sleeps —<br>and shows its work.")}
<svg class="abs" style="left:0;top:0" width="{W}" height="{H}">
  <circle cx="{cx}" cy="{cy}" r="{r_out}" fill="none" stroke="{LINE_BRIGHT}" stroke-width="1"/>
  <circle cx="{cx}" cy="{cy}" r="{r_in}" fill="none" stroke="{SIGNAL}" stroke-opacity=".7" stroke-width="1.5" stroke-dasharray="5 7"/>
  <circle cx="{cx}" cy="{cy}" r="58" fill="{EMBER}"/>
  <circle cx="{cx}" cy="{cy}" r="74" fill="none" stroke="{EMBER}" stroke-opacity=".25" stroke-width="10"/>
</svg>
<div class="abs" style="left:{cx - 22}px;top:{cy - 22}px">{icon("brain-circuit", 44, VOID, 1.7)}</div>
{tool_html}{src_html}
<div class="abs" style="left:700px;top:236px;width:588px;display:flex;flex-direction:column;gap:16px">{pill_html}</div>
<div class="verdict-bar" style="top:682px">
  <span class="badge" style="background:rgba(232,178,58,.14);color:{WARN};border-color:rgba(232,178,58,.5)">HOLD / INVESTIGATE</span>
  <span class="mono">whale dependency {num(WHALE)} · {EQ["focus_change_pct"]:+.0f}% under one-person-one-vote</span>
  <span class="mono dim" style="margin-left:auto">Octant epoch {D["epoch"]} · {short(FOCUS)}</span>
</div>
</section>'''


def s04():
    nodes = [("ASK", "message-square", BONE, "plain question", ""),
             ("PLAN", "brain-circuit", EMBER, "model picks the tools", "plan"),
             ("EXECUTE", "terminal", SIGNAL, "in-process, live", ""),
             ("STREAM", "radio", SIGNAL, "SSE tool → result", "stream"),
             ("VERDICT", "file-check", GOOD, "cites every figure", "verdict")]
    nw, gap, x0, ny = 196, 48, 102, 268
    node_html = ""
    for i, (t, ic, col, sub, kind) in enumerate(nodes):
        x = x0 + i * (nw + gap)
        border = {"plan": "rgba(232,99,58,.7)", "verdict": "rgba(95,209,160,.55)"}.get(kind, LINE_BRIGHT)
        dot = '<span class="pulse-dot" style="top:12px;right:12px"></span>' if kind == "stream" else ""
        node_html += (f'<div class="pnode" style="left:{x}px;top:{ny}px;width:{nw}px;border-color:{border}">{dot}'
                      f'{icon(ic, 28, col)}<div class="pnode-t mono" style="color:{col if kind == "plan" else BONE}">{t}</div>'
                      f'<div class="pnode-s mono">{sub}</div></div>')
    arrows = "".join(
        f'<line x1="{x0 + i * (nw + gap) + nw + 6}" y1="{ny + 64}" x2="{x0 + (i + 1) * (nw + gap) - 10}" y2="{ny + 64}" '
        f'stroke="{SIGNAL}" stroke-width="2"/><circle cx="{x0 + i * (nw + gap) + nw + gap / 2 + 3}" cy="{ny + 64}" r="3.5" fill="{SIGNAL_BRIGHT}"/>'
        f'<path d="M{x0 + (i + 1) * (nw + gap) - 16},{ny + 58} L{x0 + (i + 1) * (nw + gap) - 8},{ny + 64} L{x0 + (i + 1) * (nw + gap) - 16},{ny + 70}" '
        f'fill="none" stroke="{SIGNAL}" stroke-width="2"/>' for i in range(4))
    lines = [("1", "tool_call", f"get_project_history({short(FOCUS)})", SIGNAL),
             ("1", "result", f"epoch 8 → 10 · donors {E8['Donors']} → {E10['Donors']} · "
                             f"{num(E8['Allocated'] + E8['Matched'])} → {num(E10['Allocated'] + E10['Matched'])} ETH", BONE),
             ("5", "tool_call", f"simulate_mechanisms(epoch {D['epoch']})", SIGNAL),
             ("✓", "text", f'"hold / investigate — {EQ["focus_change_pct"]:+.0f}% under one-person-one-vote"', EMBER)]
    log = "".join(f'<div class="log-line"><span class="log-n">{n}</span><span class="log-k" style="color:{c}">▸ {k}</span>'
                  f'<span class="log-v" style="color:{BONE if k != "text" else EMBER}">{v}</span></div>' for n, k, v, c in lines)
    return f'''<section class="slide">
{layers(dots=1, grain=.5)}
<div class="abs" style="left:0;top:{ny + 63}px;width:{W}px;height:2px;background:linear-gradient(90deg, transparent, rgba(70,214,208,.55), transparent);box-shadow:0 0 24px 4px rgba(70,214,208,.25)"></div>
{header("How it works", "One question in. A traced verdict out.")}
<svg class="abs" style="left:0;top:0" width="{W}" height="{H}">{arrows}</svg>
{node_html}
<div class="abs serif-i" style="left:{x0}px;top:{ny + 140}px;width:{nw}px;text-align:center;font-size:18px;color:{BONE_DIM}">“should Octant fund X?”</div>
<div class="abs mono no-ep" style="left:{x0 + 2 * (nw + gap)}px;top:{ny + 140}px;width:{nw}px">{icon("shield-check", 15, GOOD)} no tool endpoints exposed</div>
<div class="terminal" style="left:88px;top:500px;width:1200px">
  <div class="term-bar"><span></span><span></span><span></span><b class="mono">tessera · agent trace</b><i class="mono">examples/agent-trace-epoch10.md</i></div>
  <div class="term-body">{log}</div>
</div>
{foot(4)}
</section>'''


def s05():
    data_chips = "".join(f'<span class="chip">{s}</span>' for s in ("Octant", "Gitcoin", "OSO", "GitHub", "forum", "RetroPGF"))
    return f'''<section class="slide">
{layers(tess=1, grain=.5, tess_mask="radial-gradient(50% 60% at 15% 20%, #000, transparent 75%)")}
{header("Architecture", "A Go engine behind a Next.js lens.")}
<div class="band" style="top:200px;height:76px">
  <span class="band-chip mono">VERCEL</span>
  <span class="band-t">tessera-bnb.vercel.app <span class="dim">· Next.js 16 · SSE client · observatory view</span></span>
  <span style="margin-left:auto">{icon("globe", 24, BONE_DIM)}</span>
</div>
<svg class="abs" style="left:0;top:0" width="{W}" height="{H}">
  <line x1="420" y1="278" x2="420" y2="312" stroke="{BONE_FAINT}" stroke-width="2"/><path d="M413,304 L420,314 L427,304" fill="none" stroke="{BONE_FAINT}" stroke-width="2"/>
  <line x1="470" y1="312" x2="470" y2="278" stroke="{SIGNAL}" stroke-width="2"/><path d="M463,288 L470,278 L477,288" fill="none" stroke="{SIGNAL}" stroke-width="2"/>
  <line x1="688" y1="580" x2="688" y2="608" stroke="{BONE_FAINT}" stroke-width="2" stroke-dasharray="3 4"/>
  <line x1="1090" y1="580" x2="1090" y2="608" stroke="{SIGNAL}" stroke-width="2" stroke-dasharray="3 4"/>
</svg>
<div class="abs mono dim" style="left:494px;top:289px;font-size:13px">browser → http://localhost:8080 · JSON + <span style="color:{SIGNAL}">SSE</span></div>
<div class="band band-hero" style="top:316px;height:262px">
  <div class="band-row"><span class="band-chip mono" style="color:{BONE};border-color:{LINE_BRIGHT}">YOUR MACHINE</span>
    <span class="band-t">Go service · localhost:8080 <span class="dim">· no globals · DI App struct</span></span>
    <span style="margin-left:auto">{icon("server", 24, BONE_DIM)}</span></div>
  <div class="abs" style="left:28px;top:84px;width:330px">
    <div class="route"><span class="mono" style="color:{SIGNAL}">/api/*</span><span class="dim">JSON · fast · no LLM</span></div>
    <div class="route"><span class="mono" style="color:{EMBER}">/api/agent/*</span><span class="dim">SSE tool_call → result → text</span></div>
  </div>
  <div class="flow abs" style="left:400px;top:96px">
    <span class="flow-n mono">internal/agent</span><span class="flow-a">→</span>
    <span class="flow-n mono">tool loop</span><span class="flow-a">→</span>
    <span class="flow-n mono">Messages API</span><span class="flow-a">→</span>
    <span class="flow-n mono flow-hot">xKiro</span>
  </div>
  <svg class="abs" style="left:930px;top:146px" width="90" height="70"><path d="M30,6 C30,40 50,52 72,52" fill="none" stroke="{BONE_FAINT}" stroke-width="1.5" stroke-dasharray="4 5"/></svg>
  <div class="abs mono" style="left:1008px;top:188px;color:{BONE_DIM};font-size:14px;white-space:nowrap">QwenCloud <span class="dim" style="font-size:12px">fallback</span></div>
  <div class="abs mono dim" style="left:400px;top:196px;font-size:12px">any Anthropic-compatible model · Claude and a Hermes relay also supported</div>
</div>
<div class="band" style="top:610px;height:80px;gap:10px">
  <span class="band-chip mono">DATA</span>{data_chips}
  <span class="chip chip-scan" style="margin-left:auto"><b>scan_chain</b> BSC 56 · opBNB 204 · BSC testnet 97 <span class="dim">+8 chains · read-only</span></span>
</div>
{foot(5)}
</section>'''


def s06():
    whale_pct = WHALE * 100
    cards = [
        ("get_project_history", ["funding record", "across epochs"], sparkline(536, 112),
         f"{short(LONG_RUN)} · donors per epoch · {LONG[1]['Donors']} → {LONG[10]['Donors']}", ""),
        ("get_trust_profile", ["shannon entropy", "whale dependency", "jaccard overlap"],
         f'<div style="display:flex;align-items:center;gap:26px">{donut(132, WHALE)}<div class="legend mono">'
         f'<div><i style="background:{WARN}"></i>top donor · {whale_pct:.1f}% of direct allocations</div>'
         f'<div><i style="background:{GOOD}"></i>{TRUST["UniqueDonors"] - 1} other donors · {100 - whale_pct:.1f}%</div>'
         f'<div class="dim">repeat donors {TRUST["RepeatDonors"]} / {TRUST["UniqueDonors"]} · max overlap {num(JACCARD)}</div></div></div>',
         f"{short(FOCUS)} · epoch {D['epoch']}", ""),
        ("scan_chain", ["11 chains", "BNB first-class"], chain_bars(536, 112),
         f"{short(FOCUS)} · 11 chains in {SCAN['scanDurationMs'] / 1000:.1f} s · active on {len(SCAN_ACTIVE)} · {SCAN['totalTxCount']} txs",
         "scan"),
        ("simulate_mechanisms", ["4 QF modes", "gini · top-share"], f'<div style="padding-top:8px">{gini_bars(536)}</div>',
         f"epoch {D['epoch']} · {N_PROJECTS} projects · gini {num(STD['gini'])} → {num(EQ['gini'])}", ""),
    ]
    html = ""
    for i, (name, specs, viz, cap, kind) in enumerate(cards):
        x = 88 + (i % 2) * 612
        y = 198 + (i // 2) * 250
        style = "border-color:rgba(70,214,208,.6);box-shadow:0 0 30px -10px rgba(70,214,208,.45)" if kind == "scan" else ""
        corner = '<span class="corner-chip mono">BNB FIRST-CLASS</span>' if kind == "scan" else ""
        html += (f'<div class="card instr" style="left:{x}px;top:{y}px;{style}">{corner}'
                 f'<div class="instr-name mono">{name}</div>'
                 f'<div class="instr-specs">{"".join(f"<span>{s}</span>" for s in specs)}</div>'
                 f'<div class="instr-viz">{viz}</div><div class="instr-cap mono">{cap}</div></div>')
    return f'''<section class="slide">
{layers(dots=.8, grain=.5, dots_mask="radial-gradient(45% 60% at 95% 50%, #000, transparent)")}
{header("The tools", "Forensic instruments, not dashboards.")}
{html}
<div class="abs mono dim" style="left:88px;top:700px;font-size:12px;letter-spacing:.06em">tools execute in-process · no endpoints · every call traced</div>
{foot(6)}
</section>'''


def s07():
    lines = [
        ("tool_call", f"get_project_history({short(FOCUS)})", 1),
        ("result", f"epoch 8: {num(E8['Allocated'] + E8['Matched'])} ETH · {E8['Donors']} donors → epoch 10: "
                   f"{num(E10['Allocated'] + E10['Matched'])} ETH · {E10['Donors']} donors", 1),
        ("tool_call", f"rank_projects(epoch {D['epoch']})", 3),
        ("result", f"#1 of {N_PROJECTS} · composite score {RANK[0]['score']:.0f}", 3),
        ("tool_call", f"get_trust_profile(epoch {D['epoch']}, {short(FOCUS)})", 4),
        ("result", f"diversity {num(DIVERSITY)} · whale {num(WHALE)} · jaccard {num(JACCARD)}", 4),
        ("tool_call", f"scan_chain({short(FOCUS)})", 6),
        ("result", f"11 chains · active on {len(SCAN_ACTIVE)} · {SCAN['totalTxCount']} txs · EOA", 6),
        ("text", '"verdict: hold / investigate"', None),
    ]
    color = {"tool_call": SIGNAL, "result": BONE, "text": EMBER}
    body = ""
    for k, v, step in lines:
        body += (f'<div class="t-line"><span class="t-k" style="color:{color[k]}">▸ {k}</span>'
                 f'<span class="t-v" style="color:{EMBER if k == "text" else BONE}">{v}'
                 f'{"<span class=cursor></span>" if k == "text" else ""}</span>'
                 f'<span class="t-step">{step or ""}</span></div>')
    stats = [("diversity", num(DIVERSITY), BONE), ("whale", num(WHALE), WARN), ("chains", "11", BONE)]
    stat_html = "".join(f'<div class="stat"><div class="stat-v" style="color:{c}">{v}</div><div class="stat-k mono">{k}</div></div>'
                        for k, v, c in stats)
    return f'''<section class="slide" style="background:{VOID}">
{layers(grain=.6)}
{glow(560, 420, 900, SIGNAL, .10)}
{header("Live trace", "The agent shows its work — in real time.")}
<div class="terminal big" style="left:88px;top:196px;width:930px">
  <div class="term-bar"><span></span><span></span><span></span><b class="mono">tessera · agent trace — live</b><span class="pulse-dot" style="position:static;margin-left:auto"></span></div>
  <div class="term-body">{body}</div>
</div>
<div class="abs" style="left:1062px;top:214px;width:226px;display:flex;flex-direction:column;gap:30px">{stat_html}</div>
<p class="abs serif-i" style="left:88px;top:672px;width:1200px;text-align:center;font-size:22px;color:{BONE_DIM}">watch the reasoning happen — nothing arrives pre-written</p>
{foot(7)}
</section>'''


def s08():
    g = D["graph"]
    rows = [("Octant", "history · rank · trust graph", True),
            ("On-chain", f"{SCAN['totalTxCount']} txs on {len(SCAN_ACTIVE)} of 11 chains", True),
            ("OSO", "no record for this address", False),
            ("RetroPGF", "not found", False)]
    ev = ""
    for i, (src, what, ok) in enumerate(rows):
        y = i * 62
        col = GOOD if ok else BAD
        ev += (f'<div class="ev-row" style="top:{y}px"><span class="ev-mark" style="color:{col};border-color:{col}">'
               f'{icon("check" if ok else "x", 14, col, 2.4)}</span><div><div class="mono" style="color:{BONE};font-size:14px">{src}</div>'
               f'<div class="mono dim" style="font-size:11.5px">{what}</div></div>'
               f'{"" if ok else f"<span class=gap-tag>gap</span>"}</div>')
    conv = "".join(
        f'<path d="M{256},{24 + i * 62} C{290},{24 + i * 62} {300},{122} {322},{122}" fill="none" stroke="{GOOD}" stroke-opacity=".6" stroke-width="1.5"/>'
        for i in (0, 1))
    eq_drop = EQ["focus_change_pct"]
    std_w, eq_w = 250 * STD["gini"] / 0.42, 250 * EQ["gini"] / 0.42
    return f'''<section class="slide">
{layers(tess=1.2, grain=.5)}
{glow(688, 420, 900, BONE, .05)}
{header("The analysis", "What the evidence reveals.")}
<div class="card panel3" style="left:88px">
  <div class="p3-head mono">{icon("network", 18, SIGNAL)} TRUST GRAPH</div>
  <div style="margin:4px -6px 0">{trust_graph(356, 300)}</div>
  <div class="p3-cap">#1 and #2 share {g["focus_max_jaccard"] * 100:.0f}% of their donors</div>
  <div class="p3-sub mono">epoch {D["epoch"]} · {len(g["nodes"])} projects · overlap ≥ 0.30 drawn</div>
</div>
<div class="card panel3" style="left:496px">
  <div class="p3-head mono">{icon("flask-conical", 18, SIGNAL)} MECHANISM</div>
  <div style="position:relative;height:300px;margin-top:4px">
    <div class="mech-bar" style="top:46px;width:{std_w:.0f}px;background:{BAD}"></div>
    <div class="mono mech-l" style="top:72px">standard QF</div>
    <div class="mono mech-v" style="top:72px;right:16px">gini {num(STD['gini'], 3)}</div>
    <div class="mech-bar" style="top:196px;width:{eq_w:.0f}px;background:{GOOD}"></div>
    <div class="mono mech-l" style="top:222px">equal weight · 1 person, 1 vote</div>
    <div class="mono mech-v" style="top:248px;right:16px">gini {num(EQ['gini'], 3)}</div>
    <svg class="abs" style="left:0;top:0" width="340" height="300">
      <path d="M{std_w - 16:.0f},98 C{std_w - 10:.0f},150 {eq_w + 70:.0f},150 {eq_w + 14:.0f},192" fill="none" stroke="{EMBER}" stroke-width="2"/>
      <path d="M{eq_w + 9:.0f},178 L{eq_w + 13:.0f},193 L{eq_w + 27:.0f},187" fill="none" stroke="{EMBER}" stroke-width="2"/></svg>
    <div class="mono" style="position:absolute;left:0;top:132px;color:{EMBER_BRIGHT};font-size:14px;white-space:nowrap">#1 project {eq_drop:+.0f}%</div>
  </div>
  <div class="p3-cap">One person, one vote cuts the #1 project {abs(eq_drop):.0f}%</div>
  <div class="p3-sub mono">epoch {D["epoch"]} · {N_PROJECTS} projects · 4 mechanisms simulated</div>
</div>
<div class="card panel3" style="left:904px">
  <div class="p3-head mono">{icon("git-compare", 18, SIGNAL)} EVIDENCE GAPS</div>
  <div style="position:relative;height:300px;margin-top:22px">{ev}
    <svg class="abs" style="left:0;top:0" width="360" height="260">{conv}<circle cx="334" cy="122" r="12" fill="{EMBER}"/></svg>
  </div>
  <div class="p3-cap">Missing signals are named, never invented</div>
  <div class="p3-sub mono">OSO and RetroPGF had nothing for this address</div>
</div>
<div class="verdict-bar" style="top:690px">
  <span class="badge" style="background:rgba(232,178,58,.14);color:{WARN};border-color:rgba(232,178,58,.5)">VERDICT: HOLD / INVESTIGATE</span>
  <span class="mono">evidence: 8 tool calls · epoch {D["epoch"]} · {short(FOCUS)}</span>
  <span class="mono dim" style="margin-left:auto">examples/agent-trace-epoch10.md</span>
</div>
</section>'''


def s09():
    dep = json.loads((HERE.parent.parent / "contracts" / "deployments" / "bsc-testnet.json").read_text(encoding="utf-8"))
    chains = [("BSC", 56, "live", 1.0), ("opBNB", 204, "live", 1.0), ("BSC Testnet", 97, "staging", 0.6)]
    cards = ""
    for i, (n, cid, state, fill) in enumerate(chains):
        x = 88 + i * 408
        hot = cid == 56
        st = "border-color:rgba(70,214,208,.75);box-shadow:0 0 34px -8px rgba(70,214,208,.5)" if hot else ""
        cards += (f'<div class="card chain-card" style="left:{x}px;{st}"><div class="cc-name">{n}</div>'
                  f'<div class="cc-id mono">{cid}</div><div class="cc-bar"><span style="width:{fill * 100:.0f}%"></span></div>'
                  f'<div class="cc-state mono">{state}</div></div>')
    tokens = [("USDT", "0x55d398326f99059fF775485246999027B3197955"), ("USDC", "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"),
              ("FDUSD", "0xc5f0f7b66764F6ec8C8Dff7BA683102295E16409")]
    tok = "".join(f'<span class="chip"><b>{s}</b> <span class="dim">{short(a)}</span></span>' for s, a in tokens)
    strip = ""
    for i, c in enumerate(SCAN["chains"]):
        bnb = c["chainId"] in BNB_IDS
        strip += (f'<div class="dot-col"><span class="dot" style="{"width:16px;height:16px;background:" + SIGNAL + ";box-shadow:0 0 14px " + SIGNAL if bnb else ""}"></span>'
                  f'<span class="mono" style="color:{SIGNAL_BRIGHT if bnb else BONE_FAINT}">{CHAIN_SHORT.get(c["chain"])}</span></div>')
    return f'''<section class="slide">
{layers(dots=1, grain=.5)}
{glow(688, 330, 900, SIGNAL, .12)}{glow(1240, 260, 380, WARN, .08)}
{header("BNB Chain first-class", "The scanner’s home network — and the verdict notary.")}
{cards}
<div class="abs" style="left:88px;top:352px;display:flex;gap:12px;align-items:center">{tok}
  <span class="chip" style="color:{EMBER_BRIGHT};border-color:rgba(232,99,58,.6)">18 decimals</span>
  <span class="mono dim" style="font-size:13px;margin-left:8px">native BNB + stablecoins via RPC</span></div>
<div class="abs" style="left:88px;top:420px;width:1200px">
  <div class="mono dim" style="font-size:13px;margin-bottom:12px">11 chains scanned per address · read-only</div>
  <div class="dot-strip">{strip}</div>
</div>
<div class="card notary" style="top:536px">
  <div>{icon("stamp", 28, EMBER)}</div>
  <div style="flex:1">
    <div class="mono" style="font-size:16px;color:{BONE}">{dep["contract"]} <span class="dim">· {short(dep["address"])} · BSC testnet {dep["chainId"]}</span>
      <span class="badge" style="margin-left:10px;color:{GOOD};border-color:rgba(95,209,160,.5);background:rgba(95,209,160,.1)">verified</span></div>
    <div class="mono dim" style="font-size:13px;margin-top:8px">commit(keccak256(verdict)) — proves a verdict existed and was never altered ·
      sample notarized at deploy, tx {short(dep["sampleAttestation"]["tx"])}</div>
  </div>
</div>
<div class="abs" style="left:88px;top:668px;display:flex;gap:14px">
  <span class="chip">{icon("eye", 15, BONE_DIM)} read-only scanner — signs nothing</span>
  <span class="chip">{icon("shield-check", 15, BONE_DIM)} notary: immutable · no admin · no custody</span>
</div>
{foot(9)}
</section>'''


def s10():
    boxes = "".join(f'<div class="tool-box">{icon(n, 24, GOOD)}</div>' for n in ("history", "network", "scan-search", "flask-conical"))
    return f'''<section class="slide">
{layers(tess=.6, grain=.4, tess_mask="radial-gradient(35% 45% at 8% 10%, #000, transparent)")}
{glow(688, 420, 1300, GOOD, .07)}
{header("Security & reliability", "Boring where it counts.")}
<div class="boundary" style="left:250px;top:226px;width:520px;height:330px">
  <span class="boundary-label mono">GO PROCESS</span>
  <div class="tool-row">{boxes}</div>
  <div class="mono dim" style="position:absolute;left:0;right:0;bottom:26px;text-align:center;font-size:13px">tools execute in-process · zero tool endpoints</div>
</div>
<svg class="abs" style="left:0;top:0" width="{W}" height="{H}">
  <line x1="150" y1="391" x2="244" y2="391" stroke="{BONE_FAINT}" stroke-width="2" stroke-dasharray="5 6"/>
  <g transform="translate(218,391)"><circle r="15" fill="{BG}" stroke="{BAD}" stroke-opacity=".55"/>
  <path d="M-6,-6 L6,6 M6,-6 L-6,6" stroke="{BAD}" stroke-opacity=".8" stroke-width="2.4"/></g>
  <path d="M150,300 C200,300 210,262 250,262" fill="none" stroke="{GOOD}" stroke-width="2"/>
  <path d="M250,520 C210,520 200,482 150,482" fill="none" stroke="{GOOD}" stroke-width="2"/>
</svg>
<div class="abs" style="left:96px;top:369px">{icon("globe", 44, BONE_DIM, 1.5)}</div>
<div class="abs mono" style="left:96px;top:276px;color:{GOOD};font-size:12px">HTTPS in</div>
<div class="abs mono" style="left:96px;top:494px;color:{GOOD};font-size:12px">SSE out</div>
<div class="abs" style="left:820px;top:200px;width:468px;display:flex;flex-direction:column;gap:16px">
  <div class="card sec-card"><div class="sec-t">{icon("waypoints", 20, BONE_DIM)} Provider resilience</div>
    <svg width="420" height="44"><line x1="6" y1="14" x2="190" y2="14" stroke="{EMBER}" stroke-width="2.5"/>
    <text x="198" y="18" class="svg-mono" fill="{BONE}" font-size="12">xkiro</text>
    <path d="M60,14 C90,14 100,34 130,34 L190,34" fill="none" stroke="{BONE_FAINT}" stroke-width="2" stroke-dasharray="5 5"/>
    <text x="198" y="38" class="svg-mono" fill="{BONE_DIM}" font-size="12">qwencloud (fallback)</text></svg>
    <div class="mono dim sec-s">automatic fallback · no single point</div></div>
  <div class="card sec-card"><div class="sec-t">{icon("refresh-cw", 20, BONE_DIM)} Self-healing sources</div>
    <div style="display:flex;gap:10px;margin:10px 0 8px"><i class="gdot"></i><i class="gdot"></i><i class="gdot"></i></div>
    <div class="mono dim sec-s">retry · backoff · cache TTL</div></div>
  <div class="card sec-card"><div class="sec-t">{icon("check-check", 20, BONE_DIM)} CI hygiene</div>
    <div style="display:flex;gap:8px;margin-top:12px">{"".join(f'<span class="mini-chip">{c}</span>' for c in ("go vet", "golangci-lint v2", "tests", "docker build"))}</div>
    <div class="mono dim sec-s" style="margin-top:10px">on every push</div></div>
</div>
<div class="health mono" style="top:630px"><i class="gdot"></i>GET localhost:8080/api/health → <span style="color:{GOOD}">200</span>
  <span class="dim" style="margin-left:auto">real endpoint, on the judge’s machine</span></div>
{foot(10)}
</section>'''


def s11():
    shipped = ["BNB-first scanner", "verdict notary on BSC testnet", "live trace UI", "trust + mechanism tools", "MCP surface + PDF reports"]
    nxt = ["auto-notarize every verdict", "USD-normalized balances", "scheduled epoch watch"]
    later = ["verdict API for treasuries", "multi-agent second opinions", "funding-simulation playground"]
    ship = "".join(f'<div class="rm-item">{icon("check", 15, GOOD, 2.4)}<span>{s}</span></div>' for s in shipped)
    nx = "".join(
        f'<div class="rm-item" style="{"border-color:rgba(70,214,208,.7);box-shadow:0 0 22px -6px rgba(70,214,208,.6)" if i == 0 else ""}'
        f'{";border-color:rgba(232,99,58,.7);color:" + EMBER_BRIGHT if i == 2 else ""}">'
        f'{icon("arrow-right", 15, BONE_DIM, 2)}<span>{s}</span></div>' for i, s in enumerate(nxt))
    lt = "".join(f'<div class="mono" style="color:{BONE_FAINT};font-size:15px;margin-bottom:16px">{s}</div>' for s in later)
    xs = [190, 640, 1080]
    return f'''<section class="slide">
{layers(tess=.8, dots=.4, grain=.5)}
{glow(120, 300, 600, SIGNAL, .16)}{glow(1300, 300, 700, EMBER, .18)}
{header("Roadmap", "From analyst to ambient intelligence.")}
<svg class="abs" style="left:0;top:0" width="{W}" height="{H}">
  <defs><linearGradient id="tl" gradientUnits="userSpaceOnUse" x1="88" y1="0" x2="1280" y2="0"><stop offset="0" stop-color="{SIGNAL}"/><stop offset=".55" stop-color="{BONE_FAINT}"/><stop offset="1" stop-color="{EMBER}"/></linearGradient></defs>
  <line x1="88" y1="262" x2="1280" y2="262" stroke="url(#tl)" stroke-width="2"/><path d="M1270,254 L1284,262 L1270,270" fill="none" stroke="{EMBER}" stroke-width="2"/>
  <circle cx="{xs[0]}" cy="262" r="9" fill="{SIGNAL}"/>
  <circle cx="{xs[1]}" cy="262" r="9" fill="{BG}" stroke="{SIGNAL}" stroke-width="2.5"/><circle cx="{xs[1]}" cy="262" r="17" fill="none" stroke="{SIGNAL}" stroke-opacity=".35" stroke-width="2"/>
  <circle cx="{xs[2]}" cy="262" r="6" fill="{BONE_FAINT}"/>
</svg>
<div class="abs rm-phase mono" style="left:88px;top:296px;color:{SIGNAL}">SHIPPED ✓</div>
<div class="card rm-card" style="left:88px;top:336px;width:420px">{ship}</div>
<div class="abs rm-phase mono" style="left:538px;top:296px;color:{BONE}">NEXT ▶</div>
<div class="card rm-card" style="left:538px;top:336px;width:420px;border-color:rgba(70,214,208,.45)">{nx}
  <div class="mono" style="color:{EMBER_BRIGHT};font-size:12px;margin-top:6px">epoch watch: the agent runs itself when an epoch closes</div></div>
<div class="abs rm-phase mono" style="left:988px;top:296px;color:{BONE_FAINT}">LATER ■</div>
<div class="abs" style="left:988px;top:346px;width:300px">{lt}</div>
{foot(11)}
</section>'''


def s12():
    return f'''<section class="slide" style="background:{VOID}">
{layers(tess=.3, grain=.5)}
{glow(688, 330, 760, EMBER, .17)}
<div class="center">
  <h1 class="wordmark" style="font-size:118px;letter-spacing:0">Tessera</h1>
  <p class="motto" style="font-size:30px;margin-top:6px">Evidence over narrative.</p>
  <p class="creed">Every verdict <span style="color:{SIGNAL}">cites</span> its source.<br>Every source <span style="color:{SIGNAL}">traces</span> to a call.</p>
  <p class="mono" style="margin-top:52px;color:{BONE_FAINT};font-size:15px;letter-spacing:.04em">github.com/TesseraBNB · tessera-bnb.vercel.app</p>
</div>
</section>'''


CSS = """
@page { size: 1376px 768px; margin: 0; }
* { box-sizing: border-box; margin: 0; padding: 0; }
html, body { background: #07090a; }
body { -webkit-print-color-adjust: exact; print-color-adjust: exact; color: #ece7da;
  font-family: "Schibsted Grotesk", system-ui, sans-serif; -webkit-font-smoothing: antialiased; letter-spacing: -0.005em; }
.slide { position: relative; width: 1376px; height: 768px; overflow: hidden; background: #0a0c0e; break-after: page; }
.slide:last-child { break-after: auto; }
.layer { position: absolute; inset: 0; pointer-events: none; }
.dots { background-image: radial-gradient(rgba(236,231,218,.10) 1px, transparent 1px); background-size: 24px 24px; }
.abs { position: absolute; }
.mono { font-family: "IBM Plex Mono", ui-monospace, monospace; letter-spacing: 0; }
.dim { color: #9ba39f; }
.serif-i { font-family: "Instrument Serif", serif; font-style: italic; }
.svg-mono { font-family: "IBM Plex Mono", monospace; }
.svg-display { font-family: "Instrument Serif", serif; }
.hdr { position: absolute; left: 88px; }
.kicker { font-family: "IBM Plex Mono", monospace; font-size: 13px; font-weight: 500; letter-spacing: .24em; text-transform: uppercase; color: #9ba39f; }
.headline { font-family: "Instrument Serif", serif; font-weight: 400; font-size: 54px; line-height: 1.04; margin-top: 12px; color: #ece7da; letter-spacing: -0.01em; }
.foot { position: absolute; left: 88px; right: 88px; bottom: 30px; display: flex; justify-content: space-between; align-items: center;
  font-family: "IBM Plex Mono", monospace; font-size: 12px; letter-spacing: .22em; color: #626c6a; }
.foot-l { display: inline-flex; align-items: center; gap: 10px; }
.card { position: absolute; background: linear-gradient(180deg, #0f1417, #0e1215); border: 1px solid #222b31; border-radius: 14px; }
.chip { display: inline-flex; align-items: center; gap: 8px; font-family: "IBM Plex Mono", monospace; font-size: 13px;
  padding: 8px 15px; border: 1px solid #222b31; border-radius: 99px; background: #151c21; color: #ece7da; white-space: nowrap; }
.chip b { font-weight: 600; }
.chip-signal { color: #46d6d0; letter-spacing: .2em; font-size: 13px; padding: 9px 16px; }
.chips { display: flex; gap: 12px; align-items: center; justify-content: center; }
.chip-note { font-family: "IBM Plex Mono", monospace; font-size: 13px; color: #626c6a; margin-left: 4px; }
.mini-chips { display: flex; flex-wrap: wrap; gap: 7px; margin-top: 12px; }
.mini-chip { font-family: "IBM Plex Mono", monospace; font-size: 12px; padding: 4px 10px; border: 1px solid #33404a; border-radius: 99px; color: #9ba39f; }
.badge { font-family: "IBM Plex Mono", monospace; font-size: 12.5px; font-weight: 600; letter-spacing: .08em; padding: 6px 12px; border: 1px solid; border-radius: 8px; }
.glow { position: absolute; pointer-events: none; }
/* 01 */
.center { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; }
.wordmark { font-family: "Instrument Serif", serif; font-weight: 400; font-size: 132px; line-height: .95; letter-spacing: .03em; color: #ece7da; }
.tagline { font-size: 25px; color: #9ba39f; margin-top: 18px; }
.motto { font-family: "Instrument Serif", serif; font-style: italic; font-size: 34px; color: #e8633a; margin-top: 14px; }
.title-foot { position: absolute; left: 0; right: 0; bottom: 38px; text-align: center; font-family: "IBM Plex Mono", monospace; font-size: 14px; color: #626c6a; letter-spacing: .03em; }
/* 02 */
.col-card { position: relative; height: 100%; padding: 30px 26px; background: #151c21; }
.big-list { margin-top: 26px; font-size: 21px; line-height: 1.75; color: #9ba39f; }
.row-card { position: relative; padding: 18px 22px; margin-bottom: 14px; }
.row-head { display: flex; align-items: center; gap: 14px; }
.row-title { font-size: 19px; font-weight: 600; color: #ece7da; }
.row-sub { font-size: 13px; color: #9ba39f; margin: 6px 0 0 38px; }
.split-bar { display: flex; height: 12px; border-radius: 6px; overflow: hidden; margin: 14px 0 6px 38px; gap: 2px; }
.split-legend { display: flex; justify-content: space-between; font-size: 11.5px; color: #626c6a; margin-left: 38px; }
.quote { font-family: "Instrument Serif", serif; font-style: italic; font-size: 30px; color: #ece7da; }
/* 03 */
.orbit-chip { position: absolute; transform: translate(-50%, -50%); font-size: 13px; padding: 6px 12px; border-radius: 99px;
  background: #0b1416; border: 1px solid rgba(70,214,208,.55); color: #74f2ec; white-space: nowrap; }
.orbit-src { position: absolute; transform: translate(-50%, -50%); font-size: 13px; color: #9ba39f; background: #0a0c0e; padding: 2px 8px; white-space: nowrap; }
.pillar { position: relative; display: flex; gap: 18px; align-items: flex-start; padding: 22px 26px; }
.pillar-icon { margin-top: 2px; }
.pillar-title { font-size: 15px; font-weight: 600; letter-spacing: .14em; text-transform: uppercase; color: #ece7da; }
.pillar-body { font-size: 18px; color: #9ba39f; margin-top: 7px; line-height: 1.4; }
.pulse-dot { position: absolute; top: 20px; right: 20px; width: 9px; height: 9px; border-radius: 50%; background: #46d6d0; box-shadow: 0 0 12px #46d6d0; }
.verdict-bar { position: absolute; left: 88px; width: 1200px; height: 54px; display: flex; align-items: center; gap: 18px; padding: 0 20px;
  background: rgba(21,28,33,.85); border: 1px solid #33404a; border-radius: 12px; font-size: 14px; }
.verdict-bar .mono { font-size: 14px; }
/* 04 */
.pnode { position: absolute; height: 128px; background: #151c21; border: 1px solid #33404a; border-radius: 14px; padding: 18px 16px;
  display: flex; flex-direction: column; align-items: center; text-align: center; }
.pnode-t { font-size: 15px; font-weight: 600; letter-spacing: .22em; margin-top: 10px; }
.pnode-s { font-size: 12.5px; color: #9ba39f; margin-top: 6px; }
.no-ep { display: flex; align-items: center; justify-content: center; gap: 7px; font-size: 12.5px; color: #9ba39f; }
.terminal { position: absolute; background: #0f1417; border: 1px solid #33404a; border-radius: 16px; overflow: hidden; }
.term-bar { display: flex; align-items: center; gap: 8px; height: 40px; padding: 0 18px; border-bottom: 1px solid #222b31; background: #0c1013; }
.term-bar > span { width: 10px; height: 10px; border-radius: 50%; background: #33404a; }
.term-bar b { font-weight: 500; font-size: 13px; color: #9ba39f; margin-left: 12px; }
.term-bar i { font-style: normal; font-size: 12px; color: #626c6a; margin-left: auto; }
.term-body { padding: 18px 24px; }
.log-line { display: flex; gap: 18px; font-family: "IBM Plex Mono", monospace; font-size: 16px; line-height: 1.95; }
.log-n { width: 16px; color: #626c6a; text-align: right; }
.log-k { width: 120px; }
/* 05 */
.band { position: absolute; left: 88px; width: 1200px; display: flex; align-items: center; gap: 18px; padding: 0 26px;
  background: #0f1417; border: 1px solid #222b31; border-radius: 16px; }
.band-hero { display: block; padding: 0; background: #151c21; border-color: #33404a; box-shadow: inset 0 0 60px rgba(0,0,0,.35); }
.band-row { display: flex; align-items: center; gap: 18px; padding: 22px 26px 0; }
.band-chip { font-size: 12.5px; letter-spacing: .2em; padding: 6px 12px; border: 1px solid #33404a; border-radius: 8px; color: #9ba39f; }
.band-t { font-size: 19px; color: #ece7da; }
.route { display: flex; flex-direction: column; gap: 4px; padding: 14px 18px; margin-bottom: 12px; border: 1px solid #222b31; border-radius: 12px; background: #0f1417; font-size: 14px; }
.route .mono { font-size: 17px; }
.flow { display: flex; align-items: center; gap: 12px; }
.flow-n { font-size: 15px; padding: 10px 14px; border: 1px solid #33404a; border-radius: 10px; background: #0f1417; }
.flow-a { color: #626c6a; font-size: 18px; }
.flow-hot { color: #0a0c0e; background: #e8633a; border-color: #e8633a; box-shadow: 0 0 28px -2px rgba(232,99,58,.75); font-weight: 600; }
.chip-scan { border-color: rgba(70,214,208,.65); color: #74f2ec; background: rgba(70,214,208,.07); }
.chip-scan b { color: #74f2ec; }
/* 06 */
.instr { width: 588px; height: 236px; padding: 18px 24px; }
.instr-name { font-size: 19px; color: #46d6d0; font-weight: 500; }
.instr-specs { display: flex; gap: 6px; margin-top: 8px; }
.instr-specs span { font-family: "IBM Plex Mono", monospace; font-size: 11px; color: #9ba39f; padding: 3px 9px; border: 1px solid #33404a; border-radius: 99px; }
.instr-viz { position: absolute; left: 24px; right: 24px; top: 76px; }
.instr-cap { position: absolute; left: 24px; bottom: 12px; font-size: 12px; color: #626c6a; }
.corner-chip { position: absolute; right: 18px; top: 18px; font-size: 10.5px; letter-spacing: .14em; color: #74f2ec; padding: 4px 9px; border: 1px solid rgba(70,214,208,.6); border-radius: 6px; }
.legend { font-size: 13px; color: #ece7da; display: flex; flex-direction: column; gap: 10px; }
.legend i { display: inline-block; width: 10px; height: 10px; border-radius: 3px; margin-right: 9px; }
/* 07 */
.terminal.big .term-body { padding: 24px 30px; }
.t-line { display: flex; gap: 18px; font-family: "IBM Plex Mono", monospace; font-size: 16.5px; line-height: 2.05; }
.t-k { width: 118px; flex: none; }
.t-v { flex: 1; }
.t-step { width: 22px; text-align: right; color: #626c6a; }
.cursor { display: inline-block; width: 10px; height: 19px; background: #46d6d0; margin-left: 6px; vertical-align: -3px; }
.stat-v { font-family: "Schibsted Grotesk", sans-serif; font-weight: 600; font-size: 44px; line-height: 1; }
.stat-k { font-size: 12px; letter-spacing: .2em; text-transform: uppercase; color: #626c6a; margin-top: 8px; }
/* 08 */
.panel3 { top: 196px; width: 384px; height: 476px; padding: 20px 14px 0 20px; }
.p3-head { display: flex; align-items: center; gap: 9px; font-size: 13px; letter-spacing: .2em; color: #46d6d0; }
.p3-cap { position: absolute; left: 20px; right: 16px; bottom: 44px; font-size: 18px; font-weight: 600; color: #ece7da; line-height: 1.25; }
.p3-sub { position: absolute; left: 20px; right: 16px; bottom: 20px; font-size: 11.5px; color: #626c6a; }
.mech-l { position: absolute; left: 0; font-size: 13px; color: #9ba39f; white-space: nowrap; }
.mech-bar { position: absolute; left: 0; height: 18px; border-radius: 9px; }
.mech-v { position: absolute; font-size: 13px; color: #ece7da; white-space: nowrap; }
.ev-row { position: absolute; left: 0; width: 250px; display: flex; gap: 12px; align-items: flex-start; }
.ev-mark { flex: none; width: 24px; height: 24px; border: 1.5px solid; border-radius: 7px; display: flex; align-items: center; justify-content: center; margin-top: 2px; }
.gap-tag { position: absolute; left: 262px; top: 4px; font-family: "IBM Plex Mono", monospace; font-size: 11px; color: #ef5d5d; border: 1px dashed rgba(239,93,93,.6); border-radius: 6px; padding: 2px 8px; }
/* 09 */
.chain-card { top: 196px; width: 384px; height: 132px; padding: 20px 24px; }
.cc-name { font-size: 24px; font-weight: 600; color: #ece7da; }
.cc-id { position: absolute; right: 24px; top: 14px; font-size: 40px; color: #46d6d0; }
.cc-bar { position: absolute; left: 24px; right: 24px; top: 84px; height: 6px; border-radius: 3px; background: #222b31; overflow: hidden; }
.cc-bar span { display: block; height: 100%; background: #46d6d0; }
.cc-state { position: absolute; left: 24px; top: 100px; font-size: 12px; color: #9ba39f; letter-spacing: .1em; }
.dot-strip { display: flex; justify-content: space-between; align-items: flex-end; padding: 0 6px; }
.dot-col { display: flex; flex-direction: column; align-items: center; gap: 10px; width: 90px; }
.dot-col .dot { width: 9px; height: 9px; border-radius: 50%; background: #33404a; }
.dot-col .mono { font-size: 12px; }
.notary { left: 88px; width: 1200px; height: 104px; display: flex; gap: 20px; align-items: center; padding: 0 26px;
  border-color: rgba(232,99,58,.45); box-shadow: 0 0 40px -16px rgba(232,99,58,.55); }
/* 10 */
.boundary { position: absolute; border: 1.5px dashed #626c6a; border-radius: 22px; }
.boundary-label { position: absolute; left: 50%; top: -14px; transform: translateX(-50%); background: #0a0c0e; padding: 4px 14px;
  border: 1px solid #33404a; border-radius: 8px; font-size: 13px; letter-spacing: .2em; color: #9ba39f; }
.tool-row { position: absolute; left: 0; right: 0; top: 118px; display: flex; justify-content: center; gap: 26px; }
.tool-box { width: 82px; height: 82px; border: 1.5px solid rgba(95,209,160,.55); border-radius: 14px; display: flex; align-items: center; justify-content: center; background: rgba(95,209,160,.05); }
.sec-card { position: relative; padding: 16px 22px; }
.sec-t { display: flex; align-items: center; gap: 10px; font-size: 17px; font-weight: 600; color: #ece7da; }
.sec-s { font-size: 12.5px; }
.gdot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; background: #5fd1a0; box-shadow: 0 0 10px #5fd1a0; }
.health { position: absolute; left: 88px; width: 1200px; height: 56px; display: flex; align-items: center; gap: 14px; padding: 0 22px;
  font-size: 15px; background: rgba(15,20,23,.9); border: 1px solid #222b31; border-radius: 12px; }
/* 11 */
.rm-phase { font-size: 15px; font-weight: 600; letter-spacing: .2em; }
.rm-card { padding: 20px; display: flex; flex-direction: column; gap: 10px; }
.rm-item { display: flex; align-items: center; gap: 10px; font-family: "IBM Plex Mono", monospace; font-size: 15px; color: #ece7da;
  padding: 11px 14px; border: 1px solid #33404a; border-radius: 10px; background: #151c21; }
/* 12 */
.creed { font-size: 25px; line-height: 1.6; color: #9ba39f; margin-top: 44px; }
"""


def fonts_css():
    css = (HERE / "fonts.css").read_text(encoding="utf-8")

    def embed(m):
        data = base64.b64encode((HERE / m.group(1)).read_bytes()).decode()
        return f"url(data:font/woff2;base64,{data}) format('woff2')"

    return re.sub(r"url\((fonts/[^)]+\.woff2)\)( format\('woff2'\))?", embed, css)


def build_html():
    slides = [s01(), s02(), s03(), s04(), s05(), s06(), s07(), s08(), s09(), s10(), s11(), s12()]
    doc = (f'<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Tessera — Pitch Deck</title>'
           f'<meta name="viewport" content="width=1376"><style>{fonts_css()}{CSS}</style></head><body>'
           f'{"".join(slides)}</body></html>')
    out = HERE / "deck.html"
    out.write_text(doc, encoding="utf-8")
    return out


def find_browser():
    for c in (r"C:\Program Files\Google\Chrome\Application\chrome.exe",
              r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
              "google-chrome", "chromium", "chromium-browser", "microsoft-edge"):
        if pathlib.Path(c).exists() or shutil.which(c):
            return c
    sys.exit("Chrome/Edge not found; open deck.html in a browser and print to PDF instead.")


def build_pdf(html):
    pdf = HERE.parent / "Tessera_Deck.pdf"
    subprocess.run([find_browser(), "--headless=new", "--disable-gpu", "--no-pdf-header-footer",
                    "--virtual-time-budget=4000", f"--print-to-pdf={pdf}", html.as_uri()],
                   check=True, capture_output=True)
    return pdf


if __name__ == "__main__":
    html = build_html()
    print("wrote", html)
    if "--html" not in sys.argv:
        print("wrote", build_pdf(html))
