#!/usr/bin/env bash
# Pull the web-playtest ratings from the cards LXC and summarize them.
#
# The load-bearing question for the whole project: do the EVOLVED games rate
# anywhere near the CLASSIC anchors a human already knows are fun? This pulls
# /opt/darwindeck/playtest_results.jsonl off CT 106 and reports per-game means
# plus the evolved-vs-classic comparison, classifying each game with the served
# set's catalog (results/2026-06-18-served-set/catalog.json).
#
# Usage: scripts/ratings-report.sh [ssh-host] [ctid]
#   defaults: anarres 106
set -euo pipefail

HOST=${1:-anarres}
CTID=${2:-106}
HERE="$(cd "$(dirname "$0")" && pwd)"
CATALOG="$HERE/../results/2026-06-18-served-set/catalog.json"

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

# Pull the append-only ratings log out of the container (read-only).
ssh "$HOST" "sudo pct exec $CTID -- cat /opt/darwindeck/playtest_results.jsonl" > "$TMP" 2>/dev/null || true

python3 - "$TMP" "$CATALOG" <<'PY'
import json, sys, statistics

recs_path, cat_path = sys.argv[1], sys.argv[2]
catalog = json.load(open(cat_path))
recs = []
for line in open(recs_path):
    line = line.strip()
    if line:
        try:
            recs.append(json.loads(line))
        except json.JSONDecodeError:
            pass

if not recs:
    print("No ratings yet. Once people play, this fills in.")
    sys.exit(0)

def kind(gid):
    return catalog.get(gid, {}).get("kind", "?")  # evolved | classic | ?

# Per-game aggregation.
games = {}
for r in recs:
    gid = r.get("genome_id", "?")
    g = games.setdefault(gid, {"n": 0, "ratings": [], "wins": 0, "decisive": 0, "completed": 0})
    g["n"] += 1
    rt = r.get("rating")
    if isinstance(rt, int):
        g["ratings"].append(rt)
    w = r.get("winner")
    if w in ("human", "ai"):
        g["decisive"] += 1
        g["completed"] += 1
        if w == "human":
            g["wins"] += 1
    elif w == "none":
        g["completed"] += 1  # ran to the turn cap; not broken

def mean(xs):
    return statistics.mean(xs) if xs else None

print(f"{len(recs)} sessions across {len(games)} games\n")
hdr = f"{'game':14} {'kind':8} {'plays':>5} {'rated':>5} {'mean':>5} {'win%':>5} {'fin%':>5}"
print(hdr); print("-" * len(hdr))
def row(gid, g):
    m = mean(g["ratings"])
    win = 100*g["wins"]/g["decisive"] if g["decisive"] else None
    fin = 100*g["completed"]/g["n"] if g["n"] else None
    print(f"{gid:14} {kind(gid):8} {g['n']:5d} {len(g['ratings']):5d} "
          f"{('%.2f'%m) if m is not None else '  - ':>5} "
          f"{('%.0f'%win) if win is not None else '  - ':>5} "
          f"{('%.0f'%fin) if fin is not None else '  - ':>5}")

# Sort: evolved first then classic, each by mean rating desc.
order = sorted(games.items(), key=lambda kv: (kind(kv[0]) != "evolved", -(mean(kv[1]["ratings"]) or -1)))
for gid, g in order:
    row(gid, g)

# Headline comparison: evolved vs classic mean rating.
ev = [rt for gid, g in games.items() if kind(gid) == "evolved" for rt in g["ratings"]]
cl = [rt for gid, g in games.items() if kind(gid) == "classic" for rt in g["ratings"]]
print()
print(f"evolved  : n={len(ev):3d}  mean={('%.2f'%mean(ev)) if ev else '   -'}")
print(f"classic  : n={len(cl):3d}  mean={('%.2f'%mean(cl)) if cl else '   -'}")
if ev and cl:
    delta = mean(ev) - mean(cl)
    print(f"\nevolved - classic = {delta:+.2f}  "
          f"({'evolved games rate on par or better' if delta >= -0.25 else 'classics rate higher'})")
if len(ev) + len(cl) < 30:
    print(f"\n[!] only {len(ev)+len(cl)} rated sessions -- treat as anecdote, not signal. Need ~30+ per arm.")
PY
