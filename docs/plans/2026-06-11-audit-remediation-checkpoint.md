# Audit Remediation -- Checkpoint Log

Tracking doc required by Task 13.5. Baselines first, per-metric calibration tables appended as phases land.

## Throughput baseline (pre-Task-7, commit 8c36b21-dirty, 2026-06-11)

Command: `./bin/darwindeck evolve -population 50 -generations 10 -workers 8`

- Wall: 1.92s, CPU: 12.1s user + 0.9s sys
- ~500 genome evaluations (Tier 1 + Tier 2 ~255 games each where passed) => roughly 65k games / 12s CPU = ~5,400 games/sec/core-equivalent
- Regression gate per plan: >3x drop at any later checkpoint triggers a profiling sub-task

Top-of-run metric fingerprint (pre-remediation, for before/after comparison):
`gen4_47426 (0.916) decisions=0.79 arc=1.00 interact=1.00 skill=0.84 length=1.00`
(arc and interact pinned at 1.00 -- the audit's structural-constant finding, expected to disappear after Phase 2)

## Calibration baseline (Task 2 commit a9ecaf3, pre-metric-fix, means over CalibrationSeeds, n = evals passing Tier 1)

Classics: crazy-eights 0.725 (9), mau-mau 0.722 (10), whist 0.636 (10), hearts 0.692 (10), spades 0.660 (10), oh-hell 0.667 (9), gin-rummy 0.813 (8), knock-rummy 0.823 (9).
Degenerates: instant-knock-rummy 0.826 (7), forced-shedding 0.716 (1; Tier 1-killed 9/10).

Falsification confirmed numerically: the instant-knock degenerate outscores every classic; worst classic (whist) trails the best degenerate by 0.190; all four trick-taking classics sit below FitnessFloor 0.70.

## Wave log

- Wave A (Tasks 1, 4, 5-safe, 6, 23, 25): done 2026-06-11 -- commits 8eda14b, afa283e, 4127df5, 1ce1b79, 631e198, 4f4854b. Epub history rewrite still gated on user approval (0 forks, no open PRs).
- Wave B (Tasks 2, 3): done 2026-06-11 -- commits a9ecaf3, bec2e3e. Code review: APPROVE. Bonus finding fixed: rummy CheckEnd double-banked deadwood (winner-flipping); banking moved to Upkeep. Hazard comment pinned in f119093. NOTE for Task 19 (MCTS): rummy/tricktaking Upkeep is NOT idempotent -- tree search must preserve the once-per-iteration contract on cloned states.
- Wave C (Tasks 7, 8): done 2026-06-11 -- commits fe3635e, 78ece7d. Rummy OptionDelta implemented analytically (full-union probe was 4.15x; cancellation argument documented in batch.go with invalidation condition: lay-off-style mechanics break it). Rummy hot path optimized (fixed-array buckets + value-only deadwood DP) after Progress pushed it to 2.96x baseline; now 1.006x.
- Wave D (Tasks 9-13): done 2026-06-11 -- commits 47b1e03, 4aff683, 3fa2ecd, ac77034, 5989c2f, d2c1dcc, f882a67. Metrics un-pinned: whist density 0.775 (was 1.0), arc varies 0.418-0.864 across seeds (was ~1.0), hearts-4p interaction off the old 0.657 pin. Code review: FIX-REQUIRED (narrow) -> Wave D.1 fixes in flight (phantom winners in arc, short-track resolution freebie, TurnRecord.Attack union semantics, TT lead-constraint OptionDelta -- plan table amended in a7fe81c).
- CARRIED FINDINGS: (a) Task 18 must also remove the winner's-curse skip in novelty.go:109 and MAP-Elites best-ever cells (engine.go got the fix in f882a67; the other two engines did not). (b) Phase 7 docs: Individual.Fitness components are last-eval values while TotalFitness is a running mean -- champion metric breakdowns will not reconcile with weights; document when publishing. (c) Hook effects (e.g. DrawPenalty borrow) are invisible to OptionDelta/leader sampling (recorded pre-hook) -- relevant to Task 22's evolvability sub-check.

## Task 13.5 per-metric table (post Wave D.1)

Command: `./bin/darwindeck calibrate` (commit 22872f4 + calibrate subcommand; single-threaded; raw metrics, no weighting; tier1 = evals reaching Tier 2 over the 10 pinned CalibrationSeeds).

```
genome                 skeleton      tier1  decisions       arc             interact        skill           length
---------------------------------------------------------------------------------------------------------------------------
crazy-eights           shedding      10/10  0.300 sd 0.003  0.866 sd 0.015  0.385 sd 0.003  0.033 sd 0.033  1.000 sd 0.000
mau-mau                shedding      9/10   0.288 sd 0.002  0.798 sd 0.020  0.564 sd 0.005  0.030 sd 0.026  0.728 sd 0.021
whist                  trick_taking  10/10  0.776 sd 0.000  0.609 sd 0.007  0.844 sd 0.002  0.091 sd 0.020  0.800 sd 0.000
hearts                 trick_taking  10/10  0.777 sd 0.001  0.359 sd 0.009  0.844 sd 0.001  0.484 sd 0.017  0.800 sd 0.000
spades                 trick_taking  10/10  0.769 sd 0.001  0.631 sd 0.021  0.835 sd 0.001  0.176 sd 0.022  0.800 sd 0.000
oh-hell                trick_taking  9/10   0.634 sd 0.001  0.634 sd 0.033  0.783 sd 0.003  0.125 sd 0.041  0.200 sd 0.000
gin-rummy              rummy         8/10   0.690 sd 0.000  0.858 sd 0.010  0.022 sd 0.000  0.905 sd 0.011  0.000 sd 0.000
knock-rummy            rummy         9/10   0.687 sd 0.000  0.814 sd 0.009  0.021 sd 0.000  0.764 sd 0.019  0.239 sd 0.020
instant-knock-rummy    rummy         7/10   0.355 sd 0.002  0.915 sd 0.014  0.000 sd 0.000  0.056 sd 0.035  1.000 sd 0.000
forced-shedding        shedding      1/10   0.182 sd 0.000  0.712 sd 0.000  0.231 sd 0.000  0.289 sd 0.000  1.000 sd 0.000
```

Tier 1 kills: 17 total -- forced-shedding 9/10 (timeouts, the Task 16 false-reject item), instant-knock 3/10, gin 2/10, mau-mau/oh-hell/knock-rummy 1/10 each. Kill shape matches the Task 2 baseline.

Per-column sanity verdicts (does the metric VARY within every skeleton?):

- **decisions: PASS.** Varies within all three skeletons (shedding 0.182-0.300, trick-taking 0.634-0.777, rummy 0.355-0.690). The audit's structural pins (1.0 for all trick-taking) are gone; whist/hearts/spades cluster within 0.008 of each other (same follow-suit structure) but oh-hell separates, so it is not a constant. Watch item "trick-taking decision density" cleared.
- **arc: PASS with a direction flag.** Varies within all skeletons (shedding 0.712-0.866, trick-taking 0.359-0.634, rummy 0.814-0.915). Gin vs instant-knock DO differ (0.858 vs 0.915) -- but instant-knock is the table's HIGHEST arc: 3-card coin-flip games produce cheap lead changes + clean resolution. Separation of that pair must come from decisions/interact/skill/length (all of which point the right way); a Task 14 scale/weight concern, not a Task 10 constant.
- **interact: PASS for shedding/trick-taking; FLAG rummy as an effective skeleton constant.** Shedding varies 0.231-0.564. Trick-taking is off the old 2/N closed form and varies 0.783-0.844, though compressed and sitting at ratio ~0.39-0.42 against the 0.5-ratio clamp denominator -- near-saturated, not clamped; Task 14's denominator recalibration should widen it. Rummy: gin 0.022, knock 0.021 (classic spread 0.001, sd 0.000), instant-knock 0.000 -- the discard-OptionDelta signal barely fires; column is floor-pinned for rummy and gives Task 14 almost no within-skeleton gradient. Revisit under Task 11/14 if rummy separation needs it.
- **skill: PASS.** Strong variation within every skeleton (shedding 0.030-0.289, trick-taking 0.091-0.484, rummy 0.056-0.905). Gin 0.905 vs instant-knock 0.056 is the single strongest classic-vs-degenerate signal in the table.
- **length: PASS with two flags.** Varies within all skeletons (shedding 0.728-1.000, trick-taking 0.200-0.800, rummy 0.000-1.000). Flags: (1) whist/hearts/spades are deterministically 0.800 (fixed 13 decisions/player by deal -- structural, expected, not a bug); (2) gin-rummy hard-zeros (decisions/player > 100 cap) while instant-knock banks a free 1.000 -- the 15-40 band recalibration is squarely Task 14's first item.

Throughput: 33,700 games in 4.245s = **7,939 games/sec single-threaded**, vs the pre-Task-7 baseline ~5,400 games/sec/core-equivalent. No regression (gate: >3x drop, i.e. <1,800/sec); the baseline figure included evolution-engine overhead so the comparison is conservative in both directions, but the instrumentation hot path is clearly fine.

Verdict: no column is a within-skeleton constant for the metrics' own tasks; the two flagged compressions (rummy interaction floor, trick-taking interaction near-saturation) and the two ordering inversions (instant-knock arc + length) are Task 14 scale/weight work, not Phase 2 rework. Proceed to Task 14.

## Phase 3 complete: THE CALIBRATION GATE PASSES (2026-06-11)

Wave D.1 review fixes (68a9ccf, f0d3e93, 941f5e5, 22872f4) + Task 16 reordered first (bf2a513: Tier 1 10 games, completed-games avg-turns basis at cutoff 5 -- classics 29-30/30, instant-knock killed 23/30) + Task 14 (233174c: session band [4,10,60,170] from measured classic DPP, weights unchanged, two-view gate semantics per exit condition (b)) + Task 15 (db83815: floor 0.70 -> 0.42 derived).

Final survivor-conditioned means (n=10/10 for all classics): crazy-eights 0.475, mau-mau 0.490, whist 0.633, hearts 0.650, spades 0.652, oh-hell 0.549, gin 0.584, knock 0.609; degenerates instant-knock 0.434 (n=1/10, pipeline-effective 0.043), forced-shedding 0.428 (n=4/10, effective 0.171). Every classic beats every degenerate on both views; gin beats instant-knock by 0.150. The audit's falsification finding is inverted; the suite now runs untagged as a permanent regression gate.

Remaining for Task 18 besides archive semantics: drop FitnessFloor from MAP-Elites archive admission; remove winner's-curse Valid skip in novelty.go (carried finding a).
