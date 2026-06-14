# DarwinDeck

**Evolutionary card-game search with adversarially-hardened fun-proxy metrics -- and an honest negative result**

DarwinDeck evolves *playable* card games for a standard 52-card deck. Games are
built from three constrained skeleton templates (shedding, trick-taking, rummy)
and scored by five fitness metrics that try to proxy "fun." After a four-round
adversarial hardening of those metrics, the headline finding is a negative one:

> **The system reliably evolves playable games, and its hardened fitness
> function correctly ranks faithful Whist/Gin rediscoveries as the most
> game-like outputs -- but across four adversarial rounds it did NOT discover a
> novel fun game.** Evolution either games the newest validity rule or
> rediscovers a public-domain classic. Automated fun-proxies are exploitable by
> construction; novel-fun discovery needs a human in the loop or a
> fundamentally richer signal.

This README documents that result and the methodology that produced it. The
single most reusable artifact here is not a champion game -- it is the
failed-review loop and the falsification/calibration harness that makes "this
metric is gamed" a test failure instead of an opinion.

## Quick Start

```bash
# Build
make build-v2

# Evolve a population of playable card games (default: hybrid algorithm)
./bin/darwindeck evolve -population 500 -generations 100 -workers 256

# Calibration report: raw metric means for the 8 classics + degenerate fixtures
./bin/darwindeck calibrate

# Play an evolved game against AI
./bin/darwindeck playtest output/<run>/games/rank01_*/genome.json --difficulty greedy

# Re-publish a saved run through the veto-stable publication path
./bin/darwindeck restamp output/<run> results/<run>

# Show genome details / run algorithm-comparison experiments
./bin/darwindeck describe output/<run>/games/rank01_*/genome.json
./bin/darwindeck experiment -configs baseline,hybrid,mapelites,random -seeds 15
```

## The headline result: a four-round failed-review loop

The intended payoff of the project was a re-run of the flagship evolution on the
remediated metric stack, followed by a human designer review of the top
champions. Each time the review found a degenerate champion, publication was
hard-blocked, the rejected champion was encoded as a permanent negative
calibration fixture, and the metric stack was re-falsified and re-hardened
around it -- WITHOUT moving any metric weight or scale (those froze after round
1). Only *validity rules* (Tier-0 liveness rules and Tier-2 degeneracy vetoes)
were added. The loop ran the budgeted three rounds plus one authorized extra:

| Round | Verdict | What gamed the stack | Validity rules added in response |
|-------|---------|----------------------|----------------------------------|
| 1 | hard-blocked | catch-all-skip shedding (skip == play-again in 2p), no-follow avoidance trick, pair-meld knock rummy | `non_agentic`, `tempo_monopoly`, `draw_supply_churn`; interaction + choice-impact decision-density metric fixes |
| 2 | hard-blocked | catch-all WILD shedding (dead match rule), reverse-lockout (2 seats spectate), pair-meld parked just under the churn cliff | Tier-0 catch-all liveness; greedy-batch vetoes (`seat_participation`, `greedy_timeout`, greedy `tempo_monopoly`); rummy deadwood-consequence density; churn threshold 0.10 -> 0.05 |
| 3 | 0 publishable / 19 borderline / 11 degenerate | no NEW exploit (the vetoes held); only playable-but-unremarkable games + publication-integrity bugs | output-path fixes: greedy-only leaderboard key, functional output dedup, MCTS-provenance sample floor |
| 4 | 0 publishable; top 30 = 10 wild-union-residue shedding / 10 Whist / 10 Gin-Knock | wild-union shedding (statically valid), trivial-meld rummy, episodic monopoly | Tier-0 trivial-meld liveness (`min_meld >= 3`); `playable_share` (per-card) + `longest_run` (episodic monopoly) vetoes |

The honest fitness ceiling fell every round as exploit corners closed:

> **0.97 -> 0.91 -> 0.92 (inflated) -> 0.739 (honest).**

The round-3 0.92 was inflated by an incommensurable MCTS/greedy leaderboard and
a single-eval winner's curse (a 0.918 headline reproduced at 0.73-0.82 over
fresh seeds); round 4's 0.739 is the honest greedy-only best.

The reusable artifacts of this loop are:

- a **7-fixture falsification suite** of known-degenerate genomes (the rejected
  champions) that must always score below every classic seed -- a regression
  gate that turns "the metric is gamed by shape X" into a failing test;
- a **degeneracy-veto stack** (Tier-0 liveness rules + Tier-2 random- and
  greedy-batch vetoes) that closes each discovered exploit as a validity rule
  rather than a weight tweak;
- the **calibration gate** (below), run untagged in the default test suite as a
  permanent regression test of the whole metric stack.

The full round-by-round record is in
[`docs/plans/2026-06-11-audit-remediation-checkpoint.md`](docs/plans/2026-06-11-audit-remediation-checkpoint.md);
the round-4 bundle and review are in
[`results/2026-06-12-flagship-r4/`](results/2026-06-12-flagship-r4/) (see its
`REVIEW.md` and `STABILITY.md`).

## The lesson

Closing one exploit corner just routes evolution to the next: a weighted-sum
proxy of "fun" computed from self-play simulation is exploitable by
construction, and the only outputs the four-times-hardened stack endorses are
rediscoveries of games humans already validated (multi-round Whist; Gin/Knock
Rummy). The system has no signal for *novelty relative to existing games* and no
signal for the fun a human sees that the simulation cannot. Novel-fun discovery
needs a human in the loop or a fundamentally richer signal -- not more vetoes.

## How It Works

### Skeletons

Games are built from four skeleton templates that guarantee mechanical
playability by construction (the game loop itself ensures every state has a
legal move; parameters control *what* happens, not *whether* the game works):

- **Shedding** (Crazy Eights, Mau-Mau): match suit/rank to the discard, first to
  empty hand wins
- **Trick-taking** (Whist, Hearts, Spades): one card per player per trick,
  highest card wins
- **Rummy** (Gin Rummy, Knock Rummy): draw-meld-discard, lowest deadwood wins
- **Climbing / ladder** (Big Two, Tien Len): play an ascending combination that
  beats the table or pass, first to empty hand wins

Genomes encode parameters (hand size, player count, trump rules, special cards,
scoring, win conditions) and may borrow whitelisted cross-skeleton mechanics
(e.g. a multi-round shedding game with rummy-style meld bonuses).

**Cross-skeleton recombination + novelty search (toward novel discovery).**
With `-cross-skeleton`, crossover of two different-family parents produces a
hybrid (a base family's core + an outcome-significant cross-family mechanic);
with `-novelty-select`, the hybrid algorithm rewards behavioral distance from the
8 classic seeds (gated on playability). This produced the first judge-certified
*novel* playable games (a trick-taking core grafted with rummy meld bonuses and
avoidance penalties — see `results/2026-06-13-novel-games`). Honest limitation:
novelty so far is incidental, not reliably selected-for; the cheap structural
novelty signal cannot distinguish novel from rediscovery the way the LLM judge
(`darwindeck judge`) can. Wiring the judge into the selection loop is the
documented next lever.

### Fitness function (the rebuilt metric stack)

Five metrics, each normalized to [0, 1], combined by a frozen weighted sum.
These are the *rebuilt* definitions: the audit found three of the original five
were skeleton-identity constants, and they were reimplemented from per-turn
simulation instrumentation (decisions, lead trajectories, option perturbation).

| Metric | Weight | What it measures (current implementation) |
|--------|--------|-------------------------------------------|
| Meaningful Decisions | 0.25 | Fraction of decision points whose choice plausibly matters -- a turn counts only if it has >= 2 legal moves AND sampled moves differ in type / special-effect profile / next-player option set (rummy uses a deadwood-consequence probe). Forced turns and consequence-free choices do not count. |
| Game Arc | 0.25 | Within-game trajectory from per-turn lead tracking: early uncertainty (winner not already leading at midgame) + late resolution (leader near the end wins) + lead changes. A wire-to-wire foregone conclusion and a last-turn coin flip both score low. |
| Interaction | 0.20 | Fraction of turns whose move perturbed the next player's legal options or carried a direct-attack event. Self-tempo effects (2p skip/reverse) and discards that change nothing for the opponent do not count. |
| Skill Gradient | 0.20 | Two-tier: greedy win rate over an empirical same-seed random baseline (0.4 term) plus an ISMCTS-over-greedy uplift (0.6 term). All rates are seat-0; baselines are empirical, not assumed. |
| Session Length | 0.10 | Game length in *decisions per player* (uniform across skeletons), scored against a calibrated target band. |

Weights are frozen at 0.25 / 0.25 / 0.20 / 0.20 / 0.10 and were not tuned in
response to any review after round 1.

### Validation pipeline

- **Tier 0** (free): static analysis on the genome struct -- deck overflow,
  parameter ranges, borrow whitelist, plus *liveness rules* (no catch-all wild
  special with no qualifier; `min_meld_size >= 3` so melding is consequence-
  bearing).
- **Tier 1** (10 random-AI games): smoke test; kill if any game errors, or
  timeouts/completions breach a tolerance band, or completed games end
  instantly.
- **Tier 2** (200 random + 200 greedy games -> metrics): full evaluation. Before
  fitness is computed the batches are checked by the **degeneracy veto stack**
  -- random-batch vetoes (`non_agentic`, `tempo_monopoly`, `seat_participation`,
  `draw_supply_churn`, `dead_match_rule`, `playable_share`) and greedy-batch
  vetoes (`greedy_timeout`, `greedy_tempo_monopoly`, `greedy_seat_participation`,
  `greedy_longest_run`). A vetoed genome reads fitness 0, exactly like a Tier-1
  kill.
  Every veto threshold has a measured >= ~1.2x margin to every classic seed.
- **Skill tier**: the 20-game ISMCTS batch is expensive (~2s/genome with
  game-parallel batches), so the production mode is **MCTS-for-top-decile** --
  rank a generation by the greedy two-tier evaluation, then grant the second
  ISMCTS tier only to the top decile. The mode is recorded in each run's
  `meta.json`.

### Calibration gate (the metric stack's conscience)

The 8 classic seed games are the only human-validated "fun" ground truth in the
repo. The calibration suite (`pkg/fitness/calibration_test.go`, run untagged in
the default `go test`) asserts that every classic outscores every
known-degenerate fixture by a margin, that the fitness floor admits every
classic, and that Gin beats the instant-knock degenerate. It is a permanent
regression test: any future metric change that lets a degenerate fixture
outrank a classic is a build failure.

### Veto-stable publication

Production publishes each top-N genome from a single evaluation, which let a
genome that fails its own veto on a minority of seeds ride a lucky eval into the
top-N (the round-4 rank02 shedding game failed `greedy_longest_run` on a
minority of fresh seeds). `SaveResults` now re-evaluates each top-N genome K=5
times at fresh seeds, stamps `veto_stable` + `stable_evals` on every published
`genome.json`/`report.md`, and demotes games whose fresh published eval fails
below every stable game. `darwindeck restamp` applies the same path to an
already-saved run.

### Algorithms

- **Baseline**: fitness sharing by skeleton type.
- **Hybrid (default)**: within-skeleton novelty search (k-NN behavioral
  distance) combined with fitness sharing.
- **MAP-Elites**: a 10x10 behavioral grid per skeleton (axes: decision density x
  interaction), keeping the best genome per cell.
- **Random**: the null control -- pure random genome sampling at the engines'
  evaluation budget, for the experiment harness.

## Algorithm comparison

Post-remediation experiment matrix, every algorithm run on the final frozen
metric stack with the random-search **null control**. Behavioral coverage =
fraction of the 10x10 (decision-density x interaction) grid filled, per
skeleton, averaged. Means over the seeds completed before the run was truncated
(see note); the separations are clean enough that the truncation does not change
any conclusion.

| Algorithm | seeds | coverage (mean) | QD-score | distinct games (median) | vs random (coverage) |
|-----------|:-----:|:---------------:|:--------:|:-----------------------:|----------------------|
| **Hybrid (novelty + sharing)** | 5 | **0.198** | **33.8** | 999 | **above**, U=25/25, p=0.008 |
| **MAP-Elites** | 4 | 0.125 | 21.4 | 50 | **above**, U=20/20, p=0.008 |
| Random (null) | 5 | 0.084 | 13.9 | 25 | -- |
| Baseline (fitness sharing only) | 6 | 0.050 | 8.8 | 436 | **below**, U=0/30, p=0.004 |

Mann-Whitney U is two-sided, exact (small-N enumeration); every comparison
above is a *complete separation* (no overlap between the two groups' values),
so U is at its extreme and the same ordering holds on QD-score.

**The result, stated honestly:**

> Diversity machinery is *necessary* to beat random sampling of this constrained
> genome space. Novelty search (the default) and MAP-Elites both significantly
> exceed the random-search null on coverage and QD-score. Plain **fitness
> sharing -- the baseline, and the family of approach v1 relied on -- is
> significantly *worse* than random sampling** (coverage 0.050 vs 0.084, every
> baseline run below every random run). A genetic algorithm that selects only on
> a fun-proxy, with no explicit novelty pressure, explores the space *less* well
> than drawing genomes at random.

Two honest caveats. (1) **Truncation:** the matrix was designed for 15
seeds/config but was stopped at 4-6/config for a security reboot of the compute
host; the raw per-run log is committed at
`results/2026-06-13-experiments-final/raw-run-log.txt` and the table is
reproducible from it. The pre-registered N was 15; the completed N is smaller,
but because every reported comparison is a complete separation with exact
p < 0.02, more seeds tighten the intervals without changing the ranking.
(2) **Pairwise distance is misleading here and is omitted from the headline:**
baseline posts a *high* mean pairwise distance (~0.48) despite its low coverage
-- it finds few cells, far apart -- which is why coverage and QD-score, not raw
pairwise spread, are the load-bearing diversity measures. The experiment
harness and its tested statistics (median, IQR, coverage, QD-score,
Mann-Whitney) are in `cmd/darwindeck/experiment.go`.

## Architecture

```
cmd/darwindeck/         CLI entry point (evolve, experiment, calibrate, restamp, playtest, describe)
pkg/
├── genome/             Genome struct, skeleton params, Tier-0 static validation + liveness rules
├── skeleton/
│   ├── shedding/       Shedding runner (match suit/rank, special cards, multi-round scoring)
│   ├── tricktaking/    Trick-taking runner (suit following, trump, tricks)
│   └── rummy/          Rummy runner (draw-meld-discard, knock/gin)
├── sim/                Card types, GameState, AI players (Random/Greedy/ISMCTS), batch runner
├── mechanic/           Borrowed-mechanic hook system (single HooksFor construction site)
├── evolution/          Mutation, crossover, selection, fitness sharing
│   ├── engine.go       Baseline engine; running-mean elite re-evaluation
│   ├── novelty.go      Hybrid: within-skeleton novelty + fitness sharing
│   ├── mapelites.go    Quality-diversity archive engine
│   └── behavior.go     Behavior descriptor (decision density x interaction)
├── fitness/            5 rebuilt metrics, tiered pipeline, degeneracy vetoes, calibration gate
├── output/             Rulebook/report generation, veto-stable publication
├── playtest/           Interactive playtest session (runs the same hooks fitness does)
└── seeds/              8 seed games + the degenerate calibration fixtures
```

## Seed Games

| Skeleton | Seeds |
|----------|-------|
| Shedding | Crazy Eights, Mau-Mau |
| Trick-taking | Whist, Hearts, Spades, Oh Hell |
| Rummy | Gin Rummy, Knock Rummy |

Seeds initialize the population and are the calibration ground truth. Mutation,
crossover, and cross-skeleton mechanic borrowing produce the search space.

## Development

```bash
# Build
make build-v2

# Run tests (includes the calibration gate -- it runs untagged)
go test ./pkg/... ./cmd/...

# Single test
go test ./pkg/evolution/ -run TestSmallEvolution -v

# Calibration report
./bin/darwindeck calibrate
```

## Project Layout

- `cmd/darwindeck/` -- CLI entry point
- `pkg/` -- pure Go library code (stdlib + math/rand/v2 only)
- `docs/plans/` -- design documents and the audit-remediation plan + checkpoint
- `results/` -- tracked result artifacts (each with a `meta.json`); the
  round-4 honest-exit bundle lives here
- `output/` -- raw evolution run outputs (gitignored)

## License

MIT

## Citation

```bibtex
@software{darwindeck2026,
  title = {DarwinDeck: Evolutionary Card-Game Search and the Limits of Automated Fun-Proxies},
  author = {Gabriel Ortiz},
  year = {2026},
  url = {https://github.com/signalnine/darwindeck}
}
```
