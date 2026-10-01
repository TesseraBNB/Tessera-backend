"""Collect the real figures the deck draws, into data.json.

Every number on the slides comes from here: Tessera's own tools (called over
MCP on a running backend, `go run ./cmd/tessera serve`) and the public Octant
API. Re-run to refresh:  python collect_data.py [http://localhost:8080]
"""

import json
import math
import random
import sys
import urllib.request
from itertools import combinations

BACKEND = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8080"
OCTANT = "https://backend.mainnet.octant.app"
UA = "Tessera/1.0 (+https://github.com/TesseraBNB)"  # Octant's Cloudflare rejects library UAs
EPOCH = 10
FOCUS = "0xe2f7cf9c2b12c0bfcdab571f9e50418fc08f4ad1"  # rank #1 in epoch 10 (examples/agent-trace-epoch10.md)
LONG_RUN = "0x9531c059098e3d194ff87febb587ab07b30b1306"  # FINDINGS.md finding 2: funded since epoch 1


def get(url):
    req = urllib.request.Request(url, headers={"User-Agent": UA, "Accept": "application/json"})
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.loads(r.read())


def tool(name, args):
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "tools/call",
                       "params": {"name": name, "arguments": args}}).encode()
    req = urllib.request.Request(BACKEND + "/mcp", data=body, headers={"content-type": "application/json"})
    with urllib.request.urlopen(req, timeout=120) as r:
        res = json.loads(r.read())["result"]
    if res.get("isError"):
        raise RuntimeError(f"{name}: {res['content'][0]['text']}")
    return json.loads(res["content"][0]["text"])


def wei(s):
    return int(s) / 1e18


def layout(nodes, weight, seed=7, iters=600):
    """Deterministic Fruchterman-Reingold: donor overlap pulls projects together."""
    rnd = random.Random(seed)
    pos = {n: [rnd.uniform(-1, 1), rnd.uniform(-1, 1)] for n in nodes}
    k = 1.0 / math.sqrt(len(nodes))
    temp = 0.12
    for _ in range(iters):
        disp = {n: [0.0, 0.0] for n in nodes}
        for a, b in combinations(nodes, 2):
            dx, dy = pos[a][0] - pos[b][0], pos[a][1] - pos[b][1]
            d = max(math.hypot(dx, dy), 1e-3)
            rep = k * k / d
            att = (d * d / k) * weight.get((a, b), 0.0) * 4.0
            f = rep - att
            disp[a][0] += dx / d * f
            disp[a][1] += dy / d * f
            disp[b][0] -= dx / d * f
            disp[b][1] -= dy / d * f
        for n in nodes:  # gentle gravity keeps isolated projects on canvas
            disp[n][0] -= pos[n][0] * 0.05
            disp[n][1] -= pos[n][1] * 0.05
            dl = max(math.hypot(*disp[n]), 1e-9)
            step = min(dl, temp)
            pos[n][0] += disp[n][0] / dl * step
            pos[n][1] += disp[n][1] / dl * step
        temp *= 0.992
    xs, ys = [p[0] for p in pos.values()], [p[1] for p in pos.values()]
    span = max(max(xs) - min(xs), max(ys) - min(ys))
    return {n: [(p[0] - min(xs)) / span, (p[1] - min(ys)) / span] for n, p in pos.items()}


def main():
    allocs = get(f"{OCTANT}/allocations/epoch/{EPOCH}")["allocations"]
    rewards = get(f"{OCTANT}/rewards/projects/epoch/{EPOCH}")["rewards"]

    donors = {}
    for a in allocs:
        donors.setdefault(a["project"].lower(), set()).add(a["donor"].lower())
    funding = {r["address"].lower(): wei(r["allocated"]) + wei(r["matched"]) for r in rewards}
    nodes = sorted(set(donors) & set(funding))

    jac = {}
    for a, b in combinations(nodes, 2):
        u = len(donors[a] | donors[b])
        jac[(a, b)] = len(donors[a] & donors[b]) / u if u else 0.0
    focus_pair = max(((p, j) for p, j in jac.items() if FOCUS in p), key=lambda x: x[1])

    pos = layout(nodes, jac)
    graph = {
        "epoch": EPOCH,
        "nodes": [{"address": n, "funding_eth": round(funding[n], 4), "donors": len(donors[n]),
                   "x": round(pos[n][0], 4), "y": round(pos[n][1], 4)} for n in nodes],
        "edges": [{"a": a, "b": b, "jaccard": round(j, 4)} for (a, b), j in jac.items() if j >= 0.2],
        "focus": FOCUS,
        "focus_partner": [x for x in focus_pair[0] if x != FOCUS][0],
        "focus_max_jaccard": round(focus_pair[1], 4),
    }

    data = {
        "epoch": EPOCH,
        "focus": FOCUS,
        "graph": graph,
        "anomalies": get(f"{BACKEND}/api/detect-anomalies?epoch={EPOCH}")["report"],
        "current_epoch": tool("get_current_epoch", {}),
        "rank": [{"address": p["Address"].lower(), "score": p["CompositeScore"],
                  "funding_eth": round(p["TotalFunding"], 4)} for p in tool("rank_projects", {"epoch": EPOCH})["projects"]],
        "trust": tool("get_trust_profile", {"epoch": EPOCH, "address": FOCUS}),
        "mechanisms": [{"name": m["Name"], "gini": round(m["GiniCoeff"], 4), "top_share": round(m["TopShare"], 4),
                        "focus_change_pct": round(next(p["Change"] for p in m["Projects"] if p["Address"].lower() == FOCUS), 1)}
                       for m in tool("simulate_mechanisms", {"epoch": EPOCH})["mechanisms"]],
        "focus_history": tool("get_project_history", {"address": FOCUS}),
        "long_run_history": {"address": LONG_RUN, "epochs": tool("get_project_history", {"address": LONG_RUN})},
        "scan": tool("scan_chain", {"address": FOCUS}),
    }
    with open("data.json", "w", encoding="utf-8") as f:
        json.dump(data, f, indent=1)
    print(f"nodes={len(nodes)} edges(j>=0.2)={len(graph['edges'])} focus partner={graph['focus_partner']} "
          f"j={graph['focus_max_jaccard']}")


if __name__ == "__main__":
    main()
