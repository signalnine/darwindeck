# Judge-in-loop: the two operating modes (exploit vs discover)

Date: 2026-06-16

Two full judge-in-loop runs demonstrating the complete v1+v2+v3 system end to end
(`-judge-verdicts` selection term + chunked-checkpoint table growth + judge-aware
publication ranking). The two runs differ only in the starting verdict table.

## Steady state (rich table) -- exploitation

pop 250, gen 60, 3 chunks of 20, seed 7, started from the accumulated
17-composition table (`steady-state-table.json`).

The search pre-steered straight onto the known-novel compositions: 18/20 novel at
the first chunk boundary and unchanged through the run; NO new compositions
surfaced (the table already covered the top). Final v3 leaderboard top-20:
18 novel, 0 rediscovery (the two rediscoveries clinging on by raw fitness -- plain
vying, plain trick -- were demoted out by the publication ranking). casino+meld
ranks 1-6, climb+knock+draw_penalty 7-8/10-11, and shedding knock-alone surfaced
into ranks 9/12 from fitness 0.72 on its +1.0 novel verdict, above climbing games
at 0.83. The system converges on what it has learned is novel and surfaces it.

## Discovery (empty table) -- learning from scratch

pop 200, gen 45, 3 chunks of 15, seed 11, started from an EMPTY table
(`discovery-final-table.json` is the grown result).

| stage | rediscovery | novel | new/unjudged |
|-------|:--:|:--:|:--:|
| gen 15 (empty table, pure structural novelty) | 13 | 7 | 0 |
| gen 30 (6-comp table, pressure on) | 4 | 12 | 4 |
| gen 45 (9-comp table, final) | 5 | 12 | 3 |

Chunk 1 surfaced 6 compositions on pure structural novelty; the judge classified
them mostly rediscovery (plain poker known, trick+meld Pinochle, casino+meld+
avoidance variant; casino+meld and climb+knock+draw_penalty split 1/1). With
those in the table, chunk 2 dropped rediscoveries 13 -> 4 and EXPLORED AWAY into
new territory (it found vying+meld+avoidance, vying+meld, trick+meld+avoidance --
not yet in the table). Judged (variants), the table grew to 9, chunk 3 converged
on the genuinely-novel few. Final top-6 casino+meld, 7-8 climb+knock+draw_penalty.

## What the two modes show

The system both EXPLOITS learned novelty (steady state) and LEARNS it from scratch
(discovery), with the same machinery -- the only difference is the starting table.
The discovery loop compounds: the judge classifies what the search surfaces, the
search explores away from the rediscoveries, and the new frontier gets judged in
turn. Discovery is never "done" -- a vying+avoidance composition slipped in
unjudged at gen 45; there is always a frontier.

Honest caveats. Judge variance is real: casino+meld came back 3/3 novel in earlier
sessions but split 1/1 here under a tougher panel, which is why a 2-3 vote panel
matters and why borderline (+0.3) compositions should not be over-trusted. And
the structural signal still does the heavy lifting between judge calls (CID +
behavioral distance); the judge supplies the semantic novelty/rediscovery
distinction the structural metrics anchored on 11 seeds cannot. Whether any of
these games is fun to a human remains untested.
