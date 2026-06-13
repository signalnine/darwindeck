# Designer Review: the three-round failed-review loop and the honest exit

This bundle preserves the round-3 flagship run (`output/2026-06-12-flagship-r3`:
pop 2000, gen 200, seed 42, -mcts-decile 0.02, commit 1bd5e5d-dirty -- see
`meta.json`) exactly as the round-3 designer panel judged it. It is the final
run of the Task 28 failed-review loop, whose budget was three rounds. The
loop ended without a publishable champion; this document is the record of
why, round by round. Verdicts are sourced from
`docs/plans/2026-06-11-audit-remediation-checkpoint.md`.

## Round 1 -- flagship-postfix: HARD-BLOCKED (3 gamed archetypes)

The entire top 30 collapsed to three archetypes that gamed the metric stack:

- **A1, catch-all-skip shedding** (ranks 1-10): a catch-all skip special
  matching every card plus 3 wild suits; in 2p, skip == play-again -- the
  designer's note: "13 consecutive plays, opponent acted 0 times."
- **A2, no-follow avoidance trick-taking** (ranks 11-20): no-follow + flat
  avoidance + winner-leads; off-suit never wins, the follower always ducks --
  zero meaningful decisions.
- **A3, pair-meld knock rummy** (ranks 21-29): min_meld_size 2 knock race
  over a ~1-card stock.

Response (round-2 commits): all three encoded as permanent calibration
fixtures; interaction metric fixed (2p skip/reverse are self-tempo, not
attacks); choice-impact decision density; Tier 2 degeneracy vetoes
(non_agentic, tempo_monopoly, draw_supply_churn) as validity rules -- the
weight vector stayed frozen.

## Round 2 -- flagship-r2: HARD-BLOCKED (veto-dodging cousins)

The next flagship's champions were cousins of the round-1 archetypes, parked
just outside the new detectors:

- Shedding ranks 1-10 rode a **catch-all WILD** ({type:4}, ByRank=0/BySuit=0)
  that deletes match_rule/draw_penalty as dead genes; greedy skill 0.00;
  rank01 cycled to the 390-turn cap under greedy play -- invisible to the
  random-batch Tier 1.
- rank03 locked 2 of 4 seats out via **adjacent-pair reverse ping-pong**
  (same-player runs ~1, invisible to tempo_monopoly).
- Rummy ranks 21-30 were the PREDICTED pair-meld count-density archetype,
  with rank22's draw churn **parked at 0.088, just under the 0.10 veto
  cliff**.
- A trick-taking champion was, on inspection, a **whist rediscovery** --
  novelty-credited mechanics that reduce to the classic; noted as a caution
  for novelty claims rather than a block by itself.
- Publication bugs: report-vs-genome fitness divergence, no meta.json,
  silent MCTS-mean inflation over component sums, dead rulebook text.

Response (round-3 commits 76945ad..abbaaee): Tier-0 catch-all liveness,
greedy-batch vetoes (tempo_monopoly + seat_participation + greedy_timeout on
the greedy batch), rummy deadwood-consequence density, churn veto tightened
0.10 -> 0.05 (pre-sanctioned by the round-2 hazard note), publication
reconciliation (meta.json, report==genome fitness, explicit MCTS uplift),
rulebook truth (no dead-rule text).

## Round 3 -- flagship-r3 (THIS BUNDLE): 0 publishable / 19 borderline / 11 degenerate

No champion gamed the stack in the round-1/round-2 sense -- the vetoes held.
The panel still found no publishable game:

- **19 borderline**: playable but unremarkable -- no champion a designer
  would publish under the project's own bar.
- **11 degenerate**: weak-but-not-vetoed failure shapes.
- **Incommensurable leaderboard**: ranks 1-10 are exactly the MCTS-decile
  grantees; their published means carry +0.085..+0.145 uplift over their
  greedy-only means, so the grant boundary IS the top-10 boundary. Every one
  of the ten published MCTS-mode means rests on a SINGLE two-tier eval
  (`n=1` in each report's provenance section).
- **Winner's curse on the headline**: the 0.918 headline (rank01) was
  reproduced by a reviewer at 0.73-0.82 over fresh seeds.
- **Functional duplicates**: ranks 1/2/3 are the same game differing only in
  DEAD card_points genes (no consumer in borrow-less single-round shedding);
  all three render identical rulebooks.

### Verdict: honest exit

The three-round loop budget is spent and the metric stack is frozen. Rather
than tune metrics toward these specimens or re-run for a luckier draw, the
project declares the honest exit: **no publishable champion is claimed from
this run.** The artifacts stand as evidence of what the evolved-game pipeline
produces after the full remediation, judged by its own review procedure.

## What was fixed after this review (output/reporting only; metrics frozen)

Wave K commits (this repo, post-1bd5e5d) -- they change FUTURE runs, not the
artifacts preserved here:

- Commensurable leaderboard: all published ranking by the greedy-only
  running mean (`Individual.OutputRank`); `summary.json` `best_fitness` is
  the greedy-only best with an explicit `mcts_best` alongside; report.md /
  genome.json headline is the greedy-only mean.
- MCTS provenance: report.md always prints the MCTS sample count and
  suppresses the uplift line below n=3 ("insufficient samples (n=N)").
- Functional output dedup: the output-ranking dedup hash ignores genes that
  are dead under the rulebook's liveness rules, so the ranks-1/2/3 shape
  collapses to one slot.

## Carried round-4 detector candidates (not run; loop budget spent)

Recorded for any future round, NOT applied to the frozen stack:

- Per-turn playable-share statistic (how much of the hand is playable each
  turn -- the dead_match_rule generalization).
- Chain-length / decisive-swing statistic on the greedy batch.
- Gameplay-level dedup (behavioral, beyond gene liveness) -- partially
  landed via the liveness-aware output dedup above.

## Reading this bundle

The reports here were written by the pre-Wave-K binary: headlines on ranks
1-10 are MCTS-mode means and ranks 1/2/3 occupy three slots. That is
intentional -- this bundle preserves what the panel reviewed. `meta.json`
records the run inputs; note `commit_dirty: true` (the working tree at run
time carried uncommitted changes), so bit-exact reproduction is not
guaranteed from the commit SHA alone.
