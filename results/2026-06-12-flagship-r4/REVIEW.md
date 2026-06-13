# Designer Review: round 4, the veto-stable fix, and the honest exit

This bundle preserves the round-4 flagship run (`output/2026-06-12-flagship-r4`:
pop 2000, gen 200, seed 42, -mcts-decile 0.02, commit ccf6df5-dirty -- see
`meta.json`), **re-published through the veto-stable publication path** (Wave M).
The genomes are the round-4 run's saved top 30; the fitness, rank order, and
`veto_stable`/`stable_evals` stamps here are from a fresh re-evaluation, not the
original single-eval publication (see `STABILITY.md` and `meta.json`'s restamp
annotation). Round 4 was an AUTHORIZED EXTRA swing past the budgeted three
rounds of the Task 28 failed-review loop. Verdicts are sourced from
`docs/plans/2026-06-11-audit-remediation-checkpoint.md`.

## The four-round arc

The failed-review loop ran four rounds. Each rejected champion became a
permanent calibration fixture and the metric stack was re-falsified and
re-hardened around it -- with the constraint, from round 2 on, that metric
WEIGHTS and weighted-metric SCALES stayed frozen (0.25/0.25/0.20/0.20/0.10);
the only permitted additions were validity rules (Tier-0 liveness rules and
Tier-2 degeneracy vetoes), the plan's exit-condition-(a) instrument.

| Round | Run | Verdict | What gamed the stack | Response (validity rules added) |
|-------|-----|---------|----------------------|----------------------------------|
| 1 | flagship-postfix | HARD-BLOCKED | catch-all-skip shedding (skip == play-again in 2p), no-follow avoidance trick, pair-meld knock rummy | non_agentic, tempo_monopoly, draw_supply_churn vetoes; interaction + choice-impact-decision metric fixes |
| 2 | flagship-r2 | HARD-BLOCKED | catch-all WILD shedding (dead match_rule), reverse-lockout (2 seats spectate), pair-meld parked at churn 0.088 just under the 0.10 cliff | Tier-0 catch-all liveness; greedy-batch vetoes (seat_participation, greedy_timeout, tempo on greedy); rummy deadwood-consequence density; churn 0.10 -> 0.05 |
| 3 | flagship-r3 | 0 publishable / 19 borderline / 11 degenerate | no NEW exploit (vetoes held); the panel found only playable-but-unremarkable games + publication-integrity bugs (incommensurable MCTS/greedy leaderboard, functional duplicates, winner's-curse 0.918 headline) | Wave K output-path fixes: greedy-only leaderboard key, functional output dedup, MCTS-provenance n-floor |
| 4 | flagship-r4 (THIS BUNDLE) | 0 publishable / 10 wild-union-residue shedding / 10 Whist / 10 Gin-Knock | wild-union shedding (3-suit wild, statically valid), trivial-meld rummy (min_meld < 3), episodic monopoly | Tier-0 trivial-meld liveness (min_meld >= 3); playable_share (per-card) + longest_run (episodic monopoly) vetoes |

The fitness ceiling fell every round as exploit corners closed:
**0.97 -> 0.91 -> 0.92 (inflated) -> 0.739 (honest)**. The round-3 0.92 was
inflated by the incommensurable MCTS/greedy leaderboard and a single-eval
winner's curse; round 4's 0.739 is the honest greedy-only best.

## Round 4 composition (this run's top 30)

The top 30 is exactly **10 shedding / 10 trick-taking / 10 rummy** (verified
from the genomes):

- **10 shedding** -- wild-union residue parked just under the round-4 vetoes.
  These are the hardest-to-kill remnant: statically valid (they survive the
  Tier-0 catch-all and trivial-meld rules) and they do not trip the per-card
  playable_share or episodic longest_run vetoes on the production eval. They
  are not good games; they are the shape evolution finds when every cruder
  exploit is closed.
- **10 trick-taking** -- genuine multi-round Whist. A real, playable game --
  but a public-domain REDISCOVERY, not a novel design. The metric stack
  correctly ranks faithful Whist as game-like; that is the system working, and
  also the ceiling of what it discovers.
- **10 rummy** -- genuine Gin / Knock Rummy after the min_meld >= 3 Tier-0 fix
  closed the trivial-meld exploit. Also rediscoveries.

## The motivating bug for the veto-stable fix: rank02

The round-4 run's published **rank02** (genome `gen200_55718`, a hand-11
shedding game) **fails its own `greedy_longest_run` veto on 1/10 seeds**. It
was published as rank 2 with a healthy 0.739 fitness only because production
does a SINGLE evaluation per genome: the published eval happened to land on a
passing seed, so the failing seed was invisible.

This bundle's re-publication exposes it. Re-evaluated at the fresh restamp
seed, `gen200_55718`'s single published eval lands on its FAILING seed
(`veto:greedy_longest_run`), so its honest headline fitness is **0**; the K=5
stability check reads **4/5** (one of five fresh seeds fails). The
fresh-eval-driven re-rank therefore demotes it from rank02 to **rank29**, at
the bottom of the bundle. See its `report.md` "Restamp provenance" section and
`STABILITY.md`. (A companion rummy game, `gen200_12106`, demotes the same way
on `draw_supply_churn`.)

This is exactly the publication-integrity hole the Wave M fix closes: before
`SaveResults` writes the top-N it now re-evaluates each genome K=5 times at
fresh seeds, stamps `veto_stable` + `stable_evals`, and demotes games whose
fresh published eval fails. A reviewer reading `genome.json`/`report.md` can
now see the stability verdict instead of trusting one lucky draw.

## Verdict: honest exit

The loop budget (three rounds + one authorized extra) is spent and the metric
stack is **FROZEN** -- no metric, veto, weight, or threshold moved in response
to this review, and none will without a new plan. The veto-stable fix is an
output-path publication-integrity change only; it does not touch selection,
evolution dynamics, or the frozen metric stack.

**The project claims NO novel publishable game from the remediated pipeline.**
After four adversarial rounds, evolution either games the newest veto or
rediscovers an existing game; the most game-like outputs are faithful
Whist / Gin reimplementations, which are real games but public-domain
rediscoveries, not novel designs.

## The finding

Correct, calibrated, four-times-adversarially-hardened proxy metrics still do
not discover novel fun. This is an honest negative result with a real lesson:
**automated fun-proxies are exploitable by construction** -- closing one
exploit corner just routes evolution to the next, and the only outputs the
hardened stack endorses are rediscoveries of games humans already validated.
Novel-fun discovery needs a human in the loop or a fundamentally richer signal,
not more vetoes.

## Carried-forward research directions

Recorded for a FUTURE plan; the frozen stack does not change:

- **Human-in-the-loop fitness** -- the only path that can see fun the
  simulation cannot (the rank04-class "one fix from a real game" judgment that
  no threshold can make).
- **Richer victim-acted / decision-impact signals** -- distinguish self-tempo
  chains from opponent-locking chains; a "victim acted at all" share to rescue
  the rank04-class games the longest_run veto cannot tell from degenerate
  monopolies.
- **Novelty-vs-existing-games detection** -- the system has no way to know that
  its best trick-taking output IS Whist; a rediscovery detector would convert
  these from false positives into an explicit "rediscovered a classic" label.
- The standing round-4 hazards (playable_share is shedding-only;
  draw_supply_churn has no live fixture after the trivial-meld Tier-0 rule;
  the greedy_timeout thin margin) and optional NSGA-II (plan Task 30).

## Reading this bundle

`genome.json` and `report.md` carry the Wave M `veto_stable`/`stable_evals`
stamps. Rank order is by fresh greedy-only fitness with all veto-stable games
above the two whose single fresh eval was vetoed (ranks 29-30). `summary.json`
`best_fitness` (0.721) is the greedy-only best over veto-stable games at the
restamp seed -- below the run's reported 0.739 because the restamp uses a
different fresh seed; both are honest greedy-only numbers, neither is a
publishable-champion claim. `meta.json` records the original run inputs plus
the restamp annotation.
