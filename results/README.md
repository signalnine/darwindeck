# results/

Tracked result artifacts backing published claims.

## Convention

Every published claim (README table row, flagship game, experiment comparison)
gets its backing artifact committed here. Small text artifacts only:
`summary.json`, `results.json`, top-N `rulebook.md`/`report.md` -- JSON/MD
only, no binaries, no full game logs.

Raw evolution output stays in `output/` (gitignored). When a run produces a
publishable result, copy the small artifacts into a new bundle directory here.

## meta.json is mandatory

**Every results bundle MUST contain a `meta.json`** describing exactly how to
reproduce the run:

```json
{
  "commit_sha": "<sha the run was executed at>",
  "go_version": "<go version output>",
  "platform": "<os/arch, CPU>",
  "cli_args": "<full command line>",
  "master_seed": "<master seed>",
  "calibration_seeds": "<pinned CalibrationSeeds list or reference>",
  "date": "YYYY-MM-DD"
}
```

Without a complete `meta.json`, "reproducible" is hollow. A bundle that cannot
be reproduced (e.g. preserved evidence of pre-fix results) must set
`"non_reproducible": true` and fill unknowable fields with `"unknown"`.

## Bundles

- `2026-06-12-flagship-r3/` -- the round-3 flagship (pop 2000, gen 200,
  seed 42, -mcts-decile 0.02) exactly as the round-3 designer panel judged
  it: 0 publishable / 19 borderline / 11 degenerate, the failed-review
  loop's honest exit. `REVIEW.md` inside summarizes all three review rounds;
  `meta.json` present (note `commit_dirty: true`). Preserved as evidence --
  the reports predate the Wave K leaderboard/dedup fixes on purpose.
- `pre-fix-flagship/` -- evidence of the 2026-04-12 flagship run
  (pop 2000, gen 200, best fitness 0.919) as published before the
  Apr-Jun 2026 fitness fixes. **Non-reproducible**; preserved as evidence of
  the old results, not as a reproducible artifact.
- `pre-fix-experiments/` -- diversity-experiment `results.json` files
  (baseline/MAP-Elites/novelty/hybrid comparison, `full` and `large` runs)
  predating the same fixes. **Non-reproducible.**

Post-remediation reproducible bundles are added by Phase 7 of
`docs/plans/2026-06-11-audit-remediation.md`.
