# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**DarwinDeck** is an evolutionary computation system that uses genetic algorithms and Monte Carlo simulations to evolve novel card games playable with a standard 52-card deck.

## v2: Pure Go Rewrite (Active Development)

The v2 system is a pure Go rewrite that replaces the Python/Go hybrid architecture. It lives in `cmd/` and `pkg/` at the repo root.

### Quick Start

```bash
# Build
make build-v2

# Run evolution
./bin/darwindeck evolve -population 500 -generations 100

# Play a game interactively
./bin/darwindeck playtest output/.../genome.json --difficulty greedy

# Inspect a genome
./bin/darwindeck describe output/.../genome.json

# Run tests
make test-v2
```

### Architecture

Single Go binary, three layers: CLI → Evolution Engine → Simulation Core.

**Key design change from v1:** Games are built from 6 constrained skeleton templates (shedding, trick-taking, rummy, climbing/ladder a la Big Two, casino/fishing-capture a la Scopa, and vying/betting a la poker) instead of an unconstrained genome. Skeletons guarantee playability by construction — the game loop itself ensures every state has legal moves. Parameters control *what* happens, not *whether* the game works.

**Novelty search (toward novel discovery):** `evolve -cross-skeleton` enables cross-family recombination (a base skeleton's core + an outcome-significant borrowed mechanic from another family); `-novelty-select` (hybrid algorithm) rewards behavioral distance from the 11 classic seeds, gated on playability. These produced the first judge-certified *novel* playable games (`results/2026-06-13-novel-games/`).

**2026-06-14 breakthrough (supersedes the prior "novelty is incidental" limitation):** three changes turned reliable novel discovery on, validated under selection with blind frontier judges (`results/2026-06-14-evolved-novel-hybrids/`):
1. **Borrow legibility fix** — `pkg/output/rulebook.go` `borrowedDescription` rendered every borrow as a generic parameter-free blurb, so judges assessed novelty blind. Now it renders each hook's concrete rule; re-judging the same games on legible dossiers went from "all variant" to 5/7 novel.
2. **Deep borrows** — added `MechRunPlay` (climbing multi-card combos -> shedding), the first borrow that changes the *legal-move set* (consulted INSIDE the runner via `ComboPlay()`; the hook system can only do post-move scoring). Rejected `MechMeldGate` (rummy go-out gate): reads novel but does NOT terminate under the random-AI gate (~7%) — win-condition *gates* are non-viable, win-condition *points-over-rounds* are.
3. **CID novelty pressure** — `CounterfactualIntegration` (`pkg/evolution/behavior.go`): a mechanic-aware novelty term (run hooked vs borrows-removed at the same seed, measure win/length/option-flow divergence) wired into `computeNovelty` behind `-novelty-select`, so deep fusions are SELECTED-for, not incidental. A same-seed A/B shifts selection toward integration.

**The validated recipe:** a move-changing borrow (`run_play`) + a terminating multi-round meld-**points** win condition (`meld_bonus`, `rounds_per_game >= 2`) = novel + playable. Blind judges certify such combos 4/4 novel; pure move-tweaks (no win-condition change) are correctly 2/2 variant. The 2-borrow form is publishable; 3-borrow stacks are novel-but-borderline (incentive clash). Note the earlier failed `evolve -seed-dir` judge-gated restart loop (`results/2026-06-14-judge-in-loop`, 1->2->0): novelty pressure needs generation granularity (CID, in-loop) not round boundaries — which CID provides.

**Cross-skeleton novelty:** Genomes can borrow mechanics from other skeletons (e.g., a shedding game with rummy-style meld bonuses). Borrows are whitelisted and validated. SHALLOW borrows (MeldBonus/Avoidance/TrickScoring/DrawPenalty) are hook-based scoring tallies (hosted by shedding, casino, and vying among others; see `ValidBorrows()` in `pkg/genome/validate.go`); DEEP borrows (`MechRunPlay`, `MechFollowSuit`, `MechKnock` -- run_play/follow_suit/knock on shedding, knock also on climbing) live in the skeleton runner and change moves/turns/win condition. Knock ends the ROUND on a multi-round shedding host (the game on a single-round one) and uses the undercut rule: a knocker who is only tied for fewest cards does not win (`genome.KnockWinner`). `MechTrump` / `MechPlayMultiple` exist in the enum but are reserved (not whitelisted).

### Package Layout

```
cmd/
├── darwindeck/     # CLI: evolve, experiment, calibrate, restamp, playtest, serve, describe, seeds, judge (emit/rank/backfill), version
└── grammar-*/      # grammar tools: proto, fitness, evolve, illuminate, judge (see Generative Grammar below)
pkg/
├── genome/         # Genome struct, skeleton params, static validation (Tier 0)
├── skeleton/
│   ├── shedding/   # Shedding runner (Crazy Eights, Mau-Mau style)
│   ├── tricktaking/# Trick-taking runner (Whist, Hearts, Spades style)
│   ├── rummy/      # Rummy runner (Gin Rummy, Knock Rummy style)
│   ├── climbing/   # Climbing runner (Big Two / Tien Len, beat-or-pass)
│   ├── casino/     # Casino runner (fishing capture: rank-match or pip-sum)
│   └── vying/      # Vying runner (poker: hidden hands, betting rounds, showdown) + hand ranking
├── sim/            # Card types, GameState, Move, AI players (Random, Greedy + per-skeleton scorers, determinized ISMCTS), batch runner
├── mechanic/       # Borrowed mechanics hook system
├── evolution/      # Mutation, crossover, selection, population, engine
├── fitness/        # 5 metrics, Tier 1-2 validation, evaluation pipeline
├── output/         # Rulebook gen, report gen, JSON output
├── playtest/       # Interactive terminal playtest session
├── webplay/        # Browser play + human ratings (`darwindeck serve`), embedded static UI
├── judge/          # Blind novelty dossiers, LLM-judge prompt, verdict table, rank/backfill
├── grammar/        # Generative grammar: typed primitives + one generic runner (see below)
└── seeds/          # 11 seed game definitions
```

### Seed Games (11 across 6 skeletons)

`seeds.All()` returns the 11 human-validated CLASSICS below -- the single source
of truth for the evolution init pool (`cmd/darwindeck/main.go` `getAllSeeds`), the
calibration ground-truth (`calibrate.go`), and the novelty seed-distance anchors
(`pkg/evolution` `seedDescriptors`). They are real, time-tested published games
(a game still in circulation is fun by survival).

**Big Two (climbing), Casino, and SimplePoker (vying) joined the calibration set
as their skeletons landed.** A new skeleton's seed enters seeds.All() only once
the metrics can measure it: Big Two was blocked until `deltaModeClimbing`
(`pkg/sim/batch.go`) measured climbing's beat/pass constraint (before that it
scored interact=0.000 / TotalFitness ~0.401, a measurement artifact, despite
passing every veto; after, ~0.56, a little above the rummy seeds at ~0.49-0.51). Casino shipped with its
own `deltaModeCasino` (capture/trail changes the shared table) and `CasinoScorer`,
scoring ~0.77. Vying (poker) shipped with `deltaModeVying` (betting interaction is
move-type: a raise/fold is interactive, an option-count probe under-measures it)
and the `VyingScorer` (tight-aggressive bet-by-hand-strength), scoring ~0.845 (the
strongest classic). The lesson all three teach: a new skeleton needs a runner AND
an OptionDelta mode for Interaction AND a greedy scorer for the skill gradient, or
the metrics are blind to it.

| Skeleton | Seeds |
|----------|-------|
| Shedding | Crazy Eights, Mau-Mau |
| Trick-taking | Whist, Hearts, Spades, Oh Hell |
| Rummy | Gin Rummy, Knock Rummy |
| Climbing | Big Two |
| Casino | Casino |
| Vying | SimplePoker |

### Fitness Function (5 metrics)

| Metric | Weight | What it measures (current implementation, post audit-remediation) |
|--------|--------|-----------------|
| Meaningful Decisions | 0.25 | Fraction of decision points whose choice plausibly MATTERED: >= 2 legal moves AND the batch runner's choice-impact sampling finds sampled moves differing in type, `Move.Amount` (bids/nominations), special-effect profile, or next-player option impact (`turnIsMeaningful`, pkg/sim/batch.go: the sampling probe runs for shedding and trick-taking, rummy judges its own turns by deadwood consequence via `ChoiceConsequenceProber`; climbing, casino, vying, and grammar runners fall back to plain >= 2 legal moves). |
| Game Arc | 0.25 | Within-game leader-track arc: 0.4*tent(comeback, 0.5) + 0.4*resolution + 0.2*lead-changes, where comeback = P(winner was NOT leading at the midgame sample) and resolution = P(the near-end leader wins). Games need >= 5 leader samples and a real winner to qualify. |
| Interaction | 0.20 | Share of turns that change the next player's options (per-skeleton OptionDelta counterfactuals -- see the Task 7 table in `docs/plans/2026-06-11-audit-remediation.md`; climbing counts legal PLAYS on both sides; vying scores by move type, raise/fold) or that carry an attack event; ratio scaled by /0.5, clamped to [0,1]. |
| Skill Gradient | 0.20 | Two-tier: 0.4*greedy-over-random + 0.6*MCTS-over-max(greedy, random), each normalized by remaining headroom, divided by skillScale=0.5. The MCTS tier's reference is the HIGHER of the greedy and random seat-0 rates, so a search that merely beats a sub-random greedy scores 0 (2026-10-02 bughunt). |
| Session Length | 0.10 | Decisions per player per game: 1.0 in the 6-60 band, linear falloff over 3-6 and 60-170, 0 outside. |

### Validation Pipeline

- **Tier 0 (free):** Static analysis on genome struct — deck overflow, param ranges, borrow whitelist
- **Tier 1 (10 games):** Quick sim with random AI -- fail on >= 3 timeouts or < 7 completions (tolerance band, `pkg/fitness/tier1.go`), or if completed games average < 5 turns, or one player wins every completed game
- **Tier 2 (400 games):** Full evaluation -- 200 random + 200 greedy games -> fitness metrics; the top decile (`-mcts-decile`, default 0.10) additionally gets a 20-game ISMCTS batch for the skill gradient

### v2 Development Commands

```bash
# Build
make build-v2                        # Build bin/darwindeck
go build -o bin/darwindeck ./cmd/darwindeck/  # Direct

# Test
go test ./pkg/... -v                 # All tests
go test ./pkg/genome/ -v             # Single package
go test ./pkg/evolution/ -run TestSmallEvolution -v  # Single test

# Evolution
./bin/darwindeck evolve -population 30 -generations 5 -verbose  # Quick test
./bin/darwindeck evolve -population 500 -generations 100 -workers 256  # Full run on EPYC

# Browser play + human ratings (pkg/webplay)
./bin/darwindeck serve output/.../genome.json -port 8080
./bin/darwindeck serve -dir results/2026-06-18-served-set/   # game picker

# Blind novelty judging (see docs/judge-in-loop.md)
./bin/darwindeck judge emit <in> --out <dir>        # ids are a pseudo-random permutation; the private key goes to <dir>/../<dir-name>.answer-key.json
./bin/darwindeck judge rank|backfill ...
```

evolve/experiment/calibrate reject stray positional arguments (a stray token used to silently drop every flag after it). Dossier simulations apply the genome's borrow hooks (`mechanic.HooksFor`), same as fitness -- before 2026-10-02 they did not, so verdicts on hook-borrow genomes judged from older dossiers are suspect (see `results/2026-06-14-evolved-novel-hybrids/README.md`).

### Design Doc

Full v2 design: `docs/plans/2026-04-11-v2-rewrite-design.md`

Genome JSON format (fields, ranges, enums, borrow whitelist, examples): `docs/genome-schema-v2.md`

---

## Generative Grammar (`pkg/grammar`) -- the synthesis, covers most known card games

The grammar is the synthesis of v1 (generative but mostly garbage) and v2
(playable-by-construction but only ~6 hand-coded skeletons): a game is a
composition of TYPED PRIMITIVES (move-generator x end-condition x scoring + typed
modifiers) run by ONE generic interpreter (`runner.go`), and EVERY well-typed
composition is **playable-by-construction** -- `LegalMoves` is never empty (safety)
and the game always terminates under random play (liveness). So the reachable
family count is the combinatorial product of the primitives, not the 6 skeletons.

As of 2026-06-26 the grammar represents **~81% (21/26) of the in-scope games in a 60-game surveyed corpus** (7
move-generators, 15 modifiers, **137 modified families, all 137/137 playable** --
0 stuck, 0 non-terminating). Coverage trajectory 14% -> 67% -> 81% across three
blind expert-agent surveys (`results/2026-06-26-grammar-coverage/`).

**Move-generators (7):** play_match (Crazy Eights), beat_or_pass (climbing/Big
Two), accumulate (Blackjack/31), capture (Casino/Scopa, rank-match + pip-sum),
trick (follow-suit), rummy (draw-discard, fewest-deadwood), vying (poker betting,
max-raises-capped, best-hand showdown).

**Modifiers (15), typed via `Modifier.CompatibleWith(spec)` -- the lift of v2's
hand-maintained per-host borrow whitelist into a small total function:** run_play
(combos on shedding + climbing), follow_suit, draw_penalty, knock (fewest-cards;
rummy Gin go-out), meld_bonus, avoidance (Hearts), trump (Spades), bid (trick
contract), teams (2v2), skip, force_draw, reverse (the Uno set; ill-typed at 2
players, where it is inert), nominate (Crazy Eights), wild (rummy melds),
sum_capture (Scopa building).

**Ties are never settled by absolute seat index** (2026-10-02 bughunt: seat-0
tie-breaks produced up to 2:1 seat skews). Each family breaks ties by a stated
rule -- a secondary score, then the single highest/lowest card, or turn order
from whoever ended the game -- and `rulebook.go` states it. Rummy ends only at a
turn boundary (after the discard); accumulate requires taking a card before
sticking; `Progress` (adapter.go) is computed from the same effective score that
decides the winner. `TestNoFixedSeatAdvantage` and `TestProgressLeaderIsWinner`
pin these.

**Key types:** `GameSpec` (the composition); `GameSpec.WellTyped()` (the coherence
type -- makes inert/degenerate compositions unrepresentable, e.g. an inert score
rule or an unreachable end); `GameSpec.Family()` / `Composition()` (structural
identity / judge-table key); `Runner` (the interpreter over `sim.GameState`);
`Adapter` (makes a GameSpec satisfy `sim.GenericRunner` so it runs in the REAL
`sim.RunBatch` engine); `SpecGenome` (a carrier `*genome.Genome` for the fitness
layer's per-skeleton delta-mode + greedy scorer).

**Integration with the existing engine:** `fitness.EvaluateWithRunner(g, runner,
greedy, seed)` runs a spec through the SAME 5-metric pipeline (the injected-runner
plug-in point); `judge.BuildGrammarDossier` / `EmitGrammar` write blind dossiers
keyed on `Composition` for the existing judge-in-loop verdict table.

**Commands:**
```bash
go run ./cmd/grammar-proto/       # expressiveness + family count + playable-by-construction + per-family typing diagnostic
go run ./cmd/grammar-fitness/     # each canonical spec's 5 metrics vs its hand-coded seed
go run ./cmd/grammar-evolve/ [-verdicts <file>]   # novelty + niche-sharing GA over GameSpecs
go run ./cmd/grammar-illuminate/  # MAP-Elites: fill the behavior grid (illuminate, don't converge)
go run ./cmd/grammar-judge/ emit -out <dir>      # blind grammar dossiers keyed on Composition (reuses pkg/judge)
```

**THE INVARIANT (do not break):** every move-generator carries an unconditional
fallback move (never-empty `LegalMoves`), and termination is a property of the
RUNNER -- the PlayMatch all-pass deadlock, the capture can't-redeal end, the rummy
deck-drain, the vying max-raises cap -- NOT a harness-level stalemate net (that
lesson cost a real bug: liveness in the harness silently passed while the real
engine timed out). Adding a primitive touches `spec.go` (enum + `WellTyped`),
`runner.go` (Setup/LegalMoves/Apply/CheckEnd/score), `enumerate.go`
(domains + `Canonical`), `adapter.go` (Progress/specSkeleton/events), `modifier.go`
(`CompatibleWith`), `rulebook.go` (the natural-language descriptions), and the cmd
name arrays. The grammar tests (`pkg/grammar/*_test.go`) pin never-stuck +
always-terminate across the WHOLE space; keep them green.

**Findings:** `results/2026-06-23-grammar-prototype/` (the de-risk + steps 1-7:
typed spec -> coherence type -> modifiers -> real-engine integration -> fitness
parity -> evolution -> blind judge) and `results/2026-06-26-grammar-coverage/`
(corpus coverage survey + the remaining-gap roadmap).

---

## v1: Python/Go Hybrid (Legacy)

The v1 system lives in `src/` (Python `src/darwindeck`, CGo `src/gosim`) plus the v1 web UI in `web/` (SvelteKit -> FastAPI) and `scripts/tui.sh` / `scripts/run-evolution.sh`. It is no longer under active development and nothing in v2 depends on it. Its full reference (genome schema, betting/bidding/teams, CGo bridge, benchmarks, CLI) is archived at `docs/archive/v1-CLAUDE.md`. Docs under `docs/` that carry a "Legacy v1" banner describe v1, not v2.
