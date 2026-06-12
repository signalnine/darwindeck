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

## Wave log

- Wave A: started 2026-06-11
