# Evolved novel cross-skeleton hybrids (2026-06-14)

A full `evolve -cross-skeleton -novelty-select` run (pop 200, gen 60) that discovered
the **move-change + win-condition-change** recipe in the wild, with blind frontier
judges (3 reps each) certifying the result.

## Verdicts (3-rep majority, blind, name-scrubbed dossiers)

| rank | borrows | judge novelty | quality |
|------|---------|---------------|---------|
| 09 | run_play + meld_bonus (clean recipe) | **novel** (3/3) | **publishable** |
| 07 | run_play + meld_bonus + avoidance | **novel** (3/3) | borderline |
| 08 | run_play + meld_bonus + avoidance | **novel** (3/3) | borderline |
| 10 | run_play + meld_bonus + avoidance | **novel** (3/3) | borderline |
| 19 | run_play ONLY (contrast) | variant_of_known (3/3) | publishable |
| 20 | run_play ONLY (contrast) | variant_of_known (3/3) | borderline |

## Why it works (the mechanism, from the judges' own reasons)

- **run_play** (MechRunPlay, climbing combos -> shedding) changes the LEGAL-MOVE set
  (dump same-rank sets / same-suit runs in one turn). Alone it reads as a *variant*
  (ranks 19/20): "legal moves changed, but win/scoring unchanged."
- **meld_bonus multi-round** changes the WIN condition (bank meld points over rounds;
  emptying a hand only ends a round, you can win without going out).
- **Together** they change both moves AND win condition -> judged *novel*: a genuine
  shedding x rummy fusion with a "dump-fast vs hold-melds" decision the base lacks.

The 2-borrow recipe (run_play + meld_bonus) is the sweet spot; a 3rd borrow
(avoidance) stays novel but adds quality friction (borderline).

Requires the borrowed-mechanic rulebook-legibility fix (feat/runplay-deep-borrow
branch) so the dossier actually describes the mechanics the judge assesses.
