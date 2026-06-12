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
