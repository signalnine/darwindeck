# Judge-novelty selection term: working prototype (static verdicts)

Date: 2026-06-15

First step toward in-loop novelty judging. The structural novelty signal
(knn + seed-distance + CID) anchors on the 11 classic SEEDS, so a game far from
those seeds scores high even when it is a rediscovery of a published game NOT in
the seed set (Scopa-scoring, draw poker, Hearts) -- the gap that lets variants
sweep the leaderboard. This adds a JUDGE term to computeNovelty, keyed by genome
COMPOSITION (skeleton + borrow-mechanic set), so one out-of-loop verdict steers a
whole lineage: certified-novel compositions gain novelty, certified-rediscovery
compositions lose it. `-judge-verdicts <file.json>` loads the table; absent, the
run is byte-identical.

## A/B (seed 42, pop 200, gen 60, cross-skeleton + novelty-select)

Verdict table (`judge-verdicts.json`) built from this session's blind judgments:
novel compositions (shedding knock-alone, shedding meld+run_play, casino meld,
climbing draw_penalty+knock) positive; rediscoveries (vying meld/avoidance, plain
poker, trick+avoidance Hearts, shedding knock+run_play, climbing knock) negative.

| top-20 | suppressed (variant/known) | boosted (novel) | unjudged |
|--------|:--:|:--:|:--:|
| baseline | 8 | 11 | 1 |
| judged   | 4 | 11 | 5 |

The judge term halved the variant presence (8 -> 4): shedding knock+run_play
(-0.6) dropped 6 -> 3, the vying-scoring variants stayed out, casino+meld (+0.8)
rose 1 -> 5 and took ranks 1-6. The judged run also reached higher fitness (0.881
vs 0.814) -- the bias steered toward casino+meld, novel-leaning AND strong.

## What it shows

The composition-keyed judge term shifts the population away from judged-variant
compositions and toward judged-novel ones -- the semantic novelty signal the
structural metrics cannot supply. Two limits the experiment exposes:

1. The shift is modest at JudgeWeight 1.5; a stronger weight pushes harder.
2. Evolution invents NEW compositions the static table cannot classify (vying +
   avoidance emerged unjudged, 5x in the judged top-20). A static table suppresses
   only KNOWN variants. This is the case for IN-LOOP judging (v2): judge the new
   top compositions every N generations and grow the verdict table during the
   run, so the search cannot escape into un-classified rediscovery territory.
