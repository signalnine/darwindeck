# DarwinDeck

Evolutionary search for *playable* card games on a standard 52-card deck, scored by five fitness metrics that try to proxy "fun."

Games are built from six skeleton templates (shedding, trick-taking, rummy, climbing, casino, vying) that guarantee playability by construction. I hardened the metrics against gaming over four adversarial review rounds. The result:

> The system reliably evolves playable games, and the hardened metrics correctly rank faithful Whist/Gin rediscoveries as the most game-like outputs. Across four rounds it never discovered a novel fun game. Evolution games the newest validity rule or rediscovers a public-domain classic. A weighted-sum fun-proxy computed from self-play is exploitable by construction.

Then I built the richer signal that result demanded: an LLM-as-judge, *deep* cross-skeleton borrows (mechanics that change the legal-move set or win condition, not just end-of-round scoring), and a counterfactual novelty pressure. There are three deep borrows now (`run_play`, `follow_suit`, `knock`) plus casino hosting the scoring borrows, and the system evolves blind-frontier-judge-certified **novel playable games**: move+win shedding fusions (4/4 in a controlled run; pure move-tweaks correctly judged 2/2 variant), a Crazy Eights race you end by declaring (knock, 3/3 novel), and a fishing game scored by melds (3/3), all against variant/rediscovery controls that the judge correctly rejects. And the judge is now in the loop: its verdicts (keyed by genome composition) steer novelty selection, grow a verdict table across chunked-checkpoint resumes so novelty pressure compounds, and rank the published leaderboard so the discovered novel games surface above higher-fitness rediscoveries. Whether those games are *fun* to a human is untested. See [the richer signal](#the-richer-signal-novel-discovery).

The most reusable thing here is the failed-review loop and the falsification harness that makes "this metric is gamed" a failing test instead of an argument.

## Quick start

```bash
# Build
make build-v2

# Evolve a population of playable card games (default: hybrid algorithm)
./bin/darwindeck evolve -population 500 -generations 100 -workers 256

# Calibration report: raw metric means for the 11 classics + degenerate fixtures
./bin/darwindeck calibrate

# Play an evolved game against AI
./bin/darwindeck playtest output/<run>/games/rank01_*/genome.json --difficulty greedy

# Re-publish a saved run through the veto-stable publication path
./bin/darwindeck restamp output/<run> results/<run>

# Show genome details / run algorithm-comparison experiments
./bin/darwindeck describe output/<run>/games/rank01_*/genome.json
./bin/darwindeck experiment -configs baseline,hybrid,mapelites,random -seeds 15
```

## The four-round failed-review loop

The intended payoff was a flagship evolution on the remediated metric stack, then a human review of the top champions. Each time the review found a degenerate champion, publication was hard-blocked, the rejected champion became a permanent negative calibration fixture, and the metric stack was re-falsified and re-hardened around it without moving any weight or scale (those froze after round 1). Only validity rules were added (Tier-0 liveness rules, Tier-2 degeneracy vetoes). The loop ran three budgeted rounds plus one authorized extra:

| Round | Verdict | What gamed the stack | Validity rules added in response |
|-------|---------|----------------------|----------------------------------|
| 1 | hard-blocked | catch-all-skip shedding (skip == play-again in 2p), no-follow avoidance trick, pair-meld knock rummy | `non_agentic`, `tempo_monopoly`, `draw_supply_churn`; interaction + choice-impact decision-density metric fixes |
| 2 | hard-blocked | catch-all WILD shedding (dead match rule), reverse-lockout (2 seats spectate), pair-meld parked just under the churn cliff | Tier-0 catch-all liveness; greedy-batch vetoes (`seat_participation`, `greedy_timeout`, greedy `tempo_monopoly`); rummy deadwood-consequence density; churn threshold 0.10 -> 0.05 |
| 3 | 0 publishable / 19 borderline / 11 degenerate | no NEW exploit (the vetoes held); only playable-but-unremarkable games + publication-integrity bugs | output-path fixes: greedy-only leaderboard key, functional output dedup, MCTS-provenance sample floor |
| 4 | 0 publishable; top 30 = 10 wild-union-residue shedding / 10 Whist / 10 Gin-Knock | wild-union shedding (statically valid), trivial-meld rummy, episodic monopoly | Tier-0 trivial-meld liveness (`min_meld >= 3`); `playable_share` (per-card) + `longest_run` (episodic monopoly) vetoes |

The fitness ceiling fell every round as exploit corners closed: 0.97 -> 0.91 -> 0.92 (inflated) -> 0.739. The 0.92 was inflated by an incommensurable MCTS/greedy leaderboard and a single-eval winner's curse (a 0.918 headline reproduced at 0.73-0.82 on fresh seeds). 0.739 is the greedy-only best.

Reusable artifacts of the loop:

- a **7-fixture falsification suite** of known-degenerate genomes (the rejected champions) that must always score below every classic seed, turning "the metric is gamed by shape X" into a failing test;
- a **degeneracy-veto stack** (Tier-0 liveness + Tier-2 random- and greedy-batch vetoes) that closes each exploit as a validity rule, not a weight tweak;
- the **calibration gate** (below), run untagged in the default test suite as a permanent regression test of the whole metric stack.

Full round-by-round record: [`docs/plans/2026-06-11-audit-remediation-checkpoint.md`](docs/plans/2026-06-11-audit-remediation-checkpoint.md). Round-4 bundle and review: [`results/2026-06-12-flagship-r4/`](results/2026-06-12-flagship-r4/) (`REVIEW.md`, `STABILITY.md`).

## The lesson

Closing one exploit corner routes evolution to the next. A weighted-sum fun-proxy from self-play is exploitable by construction, and the only outputs the four-times-hardened stack endorses are games humans already validated (multi-round Whist, Gin/Knock Rummy). The stack has no signal for novelty relative to existing games, and none for the fun a human sees that the simulation can't. More vetoes won't fix that; a richer signal will.

That richer signal is what the follow-up built. The exploitable-proxy lesson stands: the structural metrics still can't tell novel from rediscovery, and "fun" is still a proxy. What changed is that the novelty gap now has a working signal.

## The richer signal (novel discovery)

The negative result was measured with two limits I hadn't spotted. Removing them produced blind-judge-certified novel games.

**1. The novelty bug was a legibility bug.** Cross-skeleton borrows are the novelty lever, but the rulebook generator rendered every borrow as a generic parameter-free blurb (`"Earn bonus points for forming sets or runs"`), so every judge, human or LLM, scored novelty on text that didn't describe the mechanic. Fixed in `pkg/output/rulebook.go` (`borrowedDescription` now renders each hook's concrete rule). Re-judging the same games on legible dossiers moved a blind frontier judge from "all variant" to 5/7 novel.

**2. Every borrow was shallow** (an end-of-round scoring tally). None touched the legal-move set, turn order, or win condition, so a hybrid was a classic plus a scoring footnote, which a judge reads as a variant. There are now three *deep* borrows, consulted inside the runner where the hook system (post-move scoring only) cannot reach: `MechRunPlay` (climbing's multi-card combinations, dump a same-rank set or same-suit run in one turn) expands the move set, `MechFollowSuit` (trick-taking's follow obligation) restricts it, and `MechKnock` (rummy's knock, declare when your hand is small and fewest cards wins) changes the win condition. Casino additionally hosts the scoring borrows, giving a fishing game scored by melds or penalty cards. `MechMeldGate` (a rummy go-out gate) marks the boundary knock stays inside: a go-out *gate* reads novel but dies under the random-AI playability gate (~7% completion), while knock changes the win condition yet can only end the game sooner, so it terminates by construction.

The first validated recipe, by hand and under selection: a move-changing borrow (`run_play`) plus a terminating multi-round meld-**points** win condition (`meld_bonus`, `rounds_per_game >= 2`, you can win without going out) = novel and playable. A full `evolve -cross-skeleton -novelty-select` run found this combination on its own. Blind frontier judges (3 reps, name-scrubbed dossiers) certified 4/4 move+win hybrids novel (the clean 2-borrow recipe publishable), the pure-`run_play` contrast 2/2 `variant_of_known`, and a plain-trick control as a Whist rediscovery. Artifacts: [`results/2026-06-14-evolved-novel-hybrids/`](results/2026-06-14-evolved-novel-hybrids/).

Knock and casino-scoring shipped next and were blind-judge-validated the same way ([`results/2026-06-14-knock-casino-scored-novel/`](results/2026-06-14-knock-casino-scored-novel/), 3 judges/game, plain-casino and seed controls). `knock` is novel on its own (3/3): a Crazy Eights race you can end by declaring, fewest cards wins, is a combination no published shedding game has, so a move-change pairing is not required. Casino scored by melds is novel (3/3); casino scored by a single penalty suit is not (2/3 variant, just a house rule). Both controls came back variant, so the verdicts discriminate. The pattern across all of it: a win-condition *structure* change drives novelty (knock; casino meld scoring), a scoring overlay alone does not.

**Novelty is selected-for now, through a pre-filter.** The old novelty signal was a 2-D behavior shadow (decision-density x interaction), blind to mechanic structure: a genuine fusion landing at Crazy Eights' coordinates scored distance 0. CID (counterfactual integration depth) replaces it for borrowed genomes. Drop each borrow singly (leave-one-out, max marginal), re-run at the same seed, and measure how much play changes (win distribution, length, option flow). A deep borrow scores high, an inert or pile-on borrow scores ~0, a borrowless genome scores 0. It's wired as an additive novelty term behind `-novelty-select` (`pkg/evolution`, `CounterfactualIntegration`). Same-seed A/Bs pull integrated hybrids up the rankings monotonically with weight: best move+win hybrid at rank 7 (weight 0), rank 5 (0.5), rank 1 (2.0); the production weight of 1.5 puts all of the top 10 on borrows.

CID rewards integration, which is a different thing from novelty. A fully integral borrow can reproduce a known game: trick-taking plus penalty avoidance is Hearts, and CID scores it high because the avoidance genuinely changes the win. A blind-judge check of the production top 4 came back 1/4 novel: only the shed + run_play + meld_bonus recipe was certified novel, the other three were Hearts-family rediscoveries CID had promoted for real-but-known integration. The reason is structural: every structural novelty signal (k-NN behavior distance, seed distance, CID) anchors on the 11 classic SEEDS, so a game far from those seeds scores high even when it rediscovers a published game NOT in the seed set (Scopa-scoring, draw poker, Hearts). Only the LLM judge carries the full space of published games as its anchor. CID's job is to make integrated games rise to where the judge can find them, not to replace it.

**The judge is now in the loop.** The verdicts feed back into evolution, keyed by genome COMPOSITION (skeleton + borrow-mechanic set) so one verdict steers a whole lineage. Three pieces (`pkg/evolution`, `cmd/darwindeck` `-judge-verdicts`/`-checkpoint`/`-chunk`):

- **Selection** (`computeNovelty`): a `JudgeWeight * verdict(composition)` term, behind the same Valid+floor gate as CID, biases the search toward certified-novel compositions and away from rediscoveries. An A/B halved variant presence in the top-20.
- **Compounding** (chunked checkpoint/resume): the run is split into chunks across processes; between chunks the judge classifies the new top compositions and the verdict table grows, so the search cannot escape into un-classified rediscovery territory. The WHOLE population is checkpointed, not just the elite. A 3-chunk run drove the suppressed-elite fraction 12 -> 4 as the search abandoned the plain classics (Casino, Big Two, Whist, Pinochle-family trick+meld) it had drifted into. The earlier elite-only `-seed-dir` restart loop went 1 -> 2 -> 0 because it lost the population; the whole-population checkpoint is what lets novelty pressure compound (`results/2026-06-15-judge-in-loop-v2/`).
- **Publication** (`-judge-verdicts` output ranking): the leaderboard ranks by `fitness + 0.2*verdict`, so a certified-novel game surfaces above a higher-fitness rediscovery while fitness stays the base. The judge-in-loop final top-12 came out all-novel, with a shedding knock-alone game lifted from fitness 0.72 into the top by its novel verdict, and the high-fitness rediscoveries (plain Casino ~0.77) demoted out.

Everything is byte-identical with no verdict table loaded (each term is gated on it), so calibration and un-judged runs are unchanged.

Scope: novelty here is LLM-judge-certified (blind, 3-rep, with a variant/rediscovery contrast), not human-validated. The structural metrics still can't judge novelty, which is why the judge exists. Quality runs borderline-to-publishable (clean 2-borrow recipe publishable, 3-borrow stacks borderline from a dump-fast-vs-hold incentive clash). Whether any of it is fun to a human is untested.

## How it works

### Skeletons

Six skeleton templates guarantee mechanical playability by construction: the game loop ensures every state has a legal move, and parameters control *what* happens, not *whether* the game works.

- **Shedding** (Crazy Eights, Mau-Mau): match suit/rank to the discard, first to empty hand wins
- **Trick-taking** (Whist, Hearts, Spades): one card per player per trick, highest card wins
- **Rummy** (Gin Rummy, Knock Rummy): draw-meld-discard, lowest deadwood wins
- **Climbing** (Big Two, Tien Len): play an ascending combination that beats the table or pass, first to empty hand wins
- **Casino** (Casino, Scopa): play a card to capture table cards by rank-match or pip-sum, else trail it; most captured cards wins
- **Vying** (poker): wager chips on hidden hands across betting rounds (fold/call/raise), best poker hand at showdown takes the pot, most chips wins. The one family whose core decision is a wager, not a card play

Genomes encode parameters (hand size, player count, trump rules, special cards, scoring, win conditions) and may borrow whitelisted cross-skeleton mechanics, like a multi-round shedding game with rummy-style meld bonuses.

**Cross-skeleton recombination and novelty search.** With `-cross-skeleton`, crossover of two different-family parents produces a hybrid (a base family's core plus an outcome-significant cross-family mechanic). With `-novelty-select`, the hybrid algorithm rewards behavioral distance from the classic seeds (gated on playability) plus the CID integration term. Borrows come in two depths: shallow scoring tallies (`meld_bonus`, `avoidance`, `trick_scoring`, `draw_penalty`) implemented as hooks, and deep mechanics (`run_play`, `follow_suit`, `knock`) implemented inside the runner that change the move set or win condition, with casino additionally hosting the scoring borrows. A deep *move* borrow also needs the host's greedy scorer taught about it or a third of the fitness function stays blind: the `run_play` games read skill 0 until the shedding scorer learned to value dumping a larger combo (skill 0.00 -> ~0.5, same lesson as "a new skeleton needs a runner and an OptionDelta mode and a greedy scorer"). The deep borrows plus CID plus the LLM judge are what produce reliably novel games (see above).

**Judge-in-the-loop: the restart loop failed, the chunked-checkpoint loop works.** The first attempt was a judge-gated RESTART loop (`evolve -seed-dir <dir>`): evolve, judge the elite, re-seed the next round from the novel survivors. Over 3 rounds (`results/2026-06-14-judge-in-loop`) it did not compound novelty (trajectory 1 -> 2 -> 0) -- re-seeding from only the elite threw away the population's diversity, so in-round fitness eroded the novel hybrids faster than between-round selection accumulated them. The working version (above) fixes both failure modes: verdicts feed novelty SELECTION continuously (not just at round boundaries), and the chunked checkpoint carries the WHOLE population across judge calls (not just the elite), so the pressure compounds. The `-seed-dir` flag remains for seeding a run from custom genomes.

### Fitness function (the rebuilt metric stack)

Five metrics, each normalized to [0, 1], combined by a frozen weighted sum. These are the *rebuilt* definitions: the audit found three of the original five were skeleton-identity constants, and they were reimplemented from per-turn simulation instrumentation (decisions, lead trajectories, option perturbation).

| Metric | Weight | What it measures |
|--------|--------|------------------|
| Meaningful Decisions | 0.25 | Fraction of decision points whose choice plausibly matters: a turn counts only with >= 2 legal moves AND sampled moves that differ in type / special-effect profile / next-player option set (rummy uses a deadwood-consequence probe). Forced turns and consequence-free choices do not count. |
| Game Arc | 0.25 | Within-game trajectory from per-turn lead tracking: early uncertainty (winner not already leading at midgame) + late resolution (leader near the end wins) + lead changes. A wire-to-wire foregone conclusion and a last-turn coin flip both score low. |
| Interaction | 0.20 | Fraction of turns whose move perturbed the next player's legal options or carried a direct-attack event. Self-tempo effects (2p skip/reverse) and discards that change nothing for the opponent do not count. Climbing's beat/pass constraint is measured via `deltaModeClimbing`. |
| Skill Gradient | 0.20 | Two-tier: greedy win rate over an empirical same-seed random baseline (0.4 term) plus an ISMCTS-over-greedy uplift (0.6 term). All rates are seat-0; baselines are empirical. |
| Session Length | 0.10 | Game length in decisions per player (uniform across skeletons), scored against a calibrated target band. |

Weights are frozen at 0.25 / 0.25 / 0.20 / 0.20 / 0.10 and were not tuned in response to any review after round 1.

### Validation pipeline

- **Tier 0** (free): static analysis on the genome struct (deck overflow, parameter ranges, borrow whitelist) plus liveness rules (no catch-all wild special with no qualifier; `min_meld_size >= 3` so melding is consequence-bearing).
- **Tier 1** (10 random-AI games): smoke test; kill if any game errors, timeouts/completions breach a tolerance band, or completed games end instantly.
- **Tier 2** (200 random + 200 greedy games -> metrics): full evaluation. Before fitness is computed the batches pass the degeneracy veto stack: random-batch vetoes (`non_agentic`, `tempo_monopoly`, `seat_participation`, `draw_supply_churn`, `dead_match_rule`, `playable_share`) and greedy-batch vetoes (`greedy_timeout`, `greedy_tempo_monopoly`, `greedy_seat_participation`, `greedy_longest_run`). A vetoed genome reads fitness 0, same as a Tier-1 kill. Every veto threshold has a measured >= ~1.2x margin to every classic seed.
- **Skill tier**: the 20-game ISMCTS batch is expensive (~2s/genome with game-parallel batches), so the production mode is MCTS-for-top-decile: rank a generation by the greedy two-tier evaluation, then grant the second ISMCTS tier only to the top decile. The mode is recorded in each run's `meta.json`.

### Calibration gate

The 11 classic seed games are the fun ground truth: real, time-tested published games (a game still in circulation is fun by survival). The calibration suite (`pkg/fitness/calibration_test.go`, run untagged in the default `go test`) asserts that every classic outscores every known-degenerate fixture by a margin, that the fitness floor admits every classic, and that Gin beats the instant-knock degenerate. Any future metric change that lets a degenerate fixture outrank a classic is a build failure.

Big Two (climbing) and Casino joined the calibration set as their skeletons gained the instrumentation to measure them. Big Two had been excluded because the Interaction metric was climbing-blind: it scored Interaction 0.000 / TotalFitness ~0.401, skimming the floor as a measurement artifact, despite passing every degeneracy veto. `deltaModeClimbing` measures its beat/pass constraint, Big Two now scores ~0.55 (on par with Gin Rummy). Casino shipped with its own `deltaModeCasino` and a greedy capture scorer and calibrates at ~0.772, the strongest classic. The lesson both teach: a new skeleton needs a runner AND an OptionDelta mode for Interaction AND a greedy scorer for the skill gradient, or the metrics are blind to it.

### Veto-stable publication

A genome that fails its own veto on a minority of seeds could once ride a lucky single eval into the top-N (the round-4 rank02 shedding game failed `greedy_longest_run` on a minority of fresh seeds). `SaveResults` now re-evaluates each top-N genome K=5 times at fresh seeds, stamps `veto_stable` + `stable_evals` on every published `genome.json`/`report.md`, and demotes games whose fresh eval fails below every stable game. `darwindeck restamp` applies the same path to an already-saved run.

### Algorithms

- **Baseline**: fitness sharing by skeleton type.
- **Hybrid (default)**: within-skeleton novelty search (k-NN behavioral distance) plus fitness sharing, plus the CID/seed-distance novelty terms under `-novelty-select`.
- **MAP-Elites**: a 10x10 behavioral grid per skeleton (axes: decision density x interaction), keeping the best genome per cell.
- **Random**: the null control, pure random genome sampling at the engines' evaluation budget, for the experiment harness.

## Algorithm comparison

Post-remediation experiment matrix, every algorithm on the final frozen metric stack against the random-search null control. Behavioral coverage = fraction of the 10x10 (decision-density x interaction) grid filled, per skeleton, averaged. Means over the seeds completed before the run was truncated (see note); the separations are clean enough that truncation changes no conclusion.

| Algorithm | seeds | coverage (mean) | QD-score | distinct games (median) | vs random (coverage) |
|-----------|:-----:|:---------------:|:--------:|:-----------------------:|----------------------|
| **Hybrid (novelty + sharing)** | 5 | **0.198** | **33.8** | 999 | **above**, U=25/25, p=0.008 |
| **MAP-Elites** | 4 | 0.125 | 21.4 | 50 | **above**, U=20/20, p=0.008 |
| Random (null) | 5 | 0.084 | 13.9 | 25 | -- |
| Baseline (fitness sharing only) | 6 | 0.050 | 8.8 | 436 | **below**, U=0/30, p=0.004 |

Mann-Whitney U is two-sided, exact (small-N enumeration); every comparison is a complete separation (no overlap between the two groups' values), so U is at its extreme and the same ordering holds on QD-score.

> Diversity machinery is necessary to beat random sampling of this constrained genome space. Novelty search (the default) and MAP-Elites both significantly exceed the random-search null on coverage and QD-score. Plain **fitness sharing**, the baseline and the family of approach v1 relied on, is significantly *worse* than random sampling (coverage 0.050 vs 0.084, every baseline run below every random run). A genetic algorithm that selects only on a fun-proxy with no explicit novelty pressure explores the space less well than drawing genomes at random.

Two caveats. **Truncation:** the matrix was designed for 15 seeds/config but stopped at 4-6/config for a security reboot of the compute host; the raw per-run log is at `results/2026-06-13-experiments-final/raw-run-log.txt` and the table reproduces from it. Every reported comparison is a complete separation with exact p < 0.02, so more seeds tighten the intervals without changing the ranking. **Pairwise distance is misleading here** and is omitted from the headline: baseline posts a high mean pairwise distance (~0.48) despite low coverage (few cells, far apart), which is why coverage and QD-score are the load-bearing diversity measures. The harness and its tested statistics (median, IQR, coverage, QD-score, Mann-Whitney) are in `cmd/darwindeck/experiment.go`.

## Architecture

```
cmd/darwindeck/         CLI entry point (evolve, experiment, calibrate, restamp, playtest, describe)
pkg/
├── genome/             Genome struct, skeleton params, Tier-0 static validation + liveness rules
├── skeleton/
│   ├── shedding/       Shedding runner (match suit/rank, special cards, multi-round scoring, run_play/follow_suit/knock deep borrows)
│   ├── tricktaking/    Trick-taking runner (suit following, trump, tricks)
│   ├── rummy/          Rummy runner (draw-meld-discard, knock/gin)
│   ├── climbing/       Climbing runner (beat-or-pass combinations, ladder)
│   ├── casino/         Casino runner (fishing capture: rank-match or pip-sum, trail; scoring-borrow host)
│   └── vying/          Vying runner (poker: hidden hands, betting rounds, showdown by hand rank)
├── sim/                Card types, GameState, AI players (Random/Greedy/ISMCTS), batch runner
├── mechanic/           Borrowed-mechanic hook system (single HooksFor construction site)
├── evolution/          Mutation, crossover, selection, novelty (k-NN + seed-distance + CID)
│   ├── engine.go       Baseline engine; running-mean elite re-evaluation
│   ├── novelty.go      Hybrid: within-skeleton novelty + fitness sharing + CID term
│   ├── mapelites.go    Quality-diversity archive engine
│   └── behavior.go     Behavior descriptor + CounterfactualIntegration (CID)
├── fitness/            5 rebuilt metrics, tiered pipeline, degeneracy vetoes, calibration gate
├── output/             Rulebook/report generation, veto-stable publication
├── playtest/           Interactive playtest session (runs the same hooks fitness does)
├── judge/              LLM-as-judge: blind dossier emitter + verdict ingest/rank
└── seeds/              10 seed games + the degenerate calibration fixtures
```

## Seed games

| Skeleton | Seeds |
|----------|-------|
| Shedding | Crazy Eights, Mau-Mau |
| Trick-taking | Whist, Hearts, Spades, Oh Hell |
| Rummy | Gin Rummy, Knock Rummy |
| Climbing | Big Two |
| Casino | Casino |
| Vying | SimplePoker |

The 11 classic seeds (`seeds.All()`) are the single source of truth: the evolution init pool, the calibration ground truth, and the novelty seed-distance anchors. Mutation, crossover, and cross-skeleton mechanic borrowing produce the rest of the search space.

## Development

```bash
# Build
make build-v2

# Run tests (includes the calibration gate, which runs untagged)
go test ./pkg/... ./cmd/...

# Single test
go test ./pkg/evolution/ -run TestSmallEvolution -v

# Calibration report
./bin/darwindeck calibrate
```

## Project layout

- `cmd/darwindeck/` -- CLI entry point
- `pkg/` -- pure Go library code (stdlib + math/rand/v2 only)
- `docs/plans/` -- design documents, the audit-remediation plan and checkpoint
- `results/` -- tracked result artifacts (each with a `meta.json`); the round-4 exit bundle lives here
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
