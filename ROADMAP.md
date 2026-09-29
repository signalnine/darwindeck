# DarwinDeck Roadmap

This document tracks planned features, known limitations, and future work.

## v2 Scope Decisions (recorded 2026-06-11, updated 2026-09-28)

The active system is the pure Go v2 rewrite (`cmd/`, `pkg/`). The "v2 shipped" and "v2 open" sections below cover it; everything from "Current Status (v1 legacy)" down describes the legacy v1 Python/Go hybrid (`src/`), which is no longer developed. Status of the capabilities v2 originally dropped or deferred:

| Capability | v2 status | Notes |
|------------|-----------|-------|
| MCTS skill evaluation | **Done** | v1's MCTS was omniscient (it cloned hidden hands) and was not ported. v2 has a determinized ISMCTS player (`pkg/sim/mcts.go`) feeding the second tier of a two-tier skill gradient. It costs ~14.5s per 20-game batch (~7x over the 2s/genome budget), so production grants it only to the top decile of each generation (`evolve -mcts-decile`, default 0.10). |
| Pareto / NSGA-II selection | **Open** | Still weighted-sum fitness. Multi-objective selection remains the day-one open question (Phase 8 of the remediation plan). |
| Betting/wagering | **Done (new design)** | The vying skeleton (`pkg/skeleton/vying`: hidden hands, fold/call/raise, max-raises cap, showdown) and the grammar's `vying` move-generator. Not a port of v1's `BettingPhase`. |
| Bidding/contracts | **Grammar only** | `ModBid` in `pkg/grammar/modifier.go` (declare a trick target, scored by making the contract). Not in the v2 genome/skeletons. |
| Team/partnership play | **Grammar only** | `ModTeams` in `pkg/grammar/modifier.go` (2v2, evens vs odds). Not in the v2 genome/skeletons. |
| Web UI | **Done (new design)** | `darwindeck serve` (`pkg/webplay`): a Go browser playtest server with a game picker and a 1-5 rating per session. The v1 FastAPI + SvelteKit UI still targets only the v1 engine. |

## v2 Shipped

- **Six skeletons** (2026-04-11 -> 2026-06-15): shedding, trick-taking, rummy (2026-04-11), climbing (2026-06-13), casino (2026-06-14), vying (2026-06-15); 11 classic seeds across them, all in the calibration gate.
- **Audit remediation and the four-round failed-review loop** (2026-06-11 -> 2026-06-13): rebuilt metrics, degeneracy-veto stack, calibration gate, veto-stable publication, `restamp`, the experiment matrix with a random-search null.
- **Cross-skeleton recombination + novelty search** (2026-06-13): `-cross-skeleton`, `-novelty-select`.
- **Deep borrows** (2026-06-14 -> 2026-06-15): `run_play` (shedding), `follow_suit` (shedding), `knock` (shedding, then climbing). Casino (2026-06-14) and vying (2026-06-15) host the scoring borrows. A rummy go-out gate was tried and rejected (does not terminate under random play).
- **CID novelty pressure** (2026-06-14): `CounterfactualIntegration` in `pkg/evolution/behavior.go`, leave-one-out, default weight 1.5.
- **LLM judge, then judge in the loop** (2026-06-13 -> 2026-06-23): blind dossiers (`judge emit`/`rank`), judge-novelty selection term, chunked whole-population checkpoint/resume so the verdict table grows mid-run, judge-aware publication ranking, `judge backfill` to complete the composition table. Procedure: `docs/judge-in-loop.md`.
- **MAP-Elites** as a v2 algorithm (`pkg/evolution/mapelites.go`) and over the grammar space (`cmd/grammar-illuminate`, 2026-06-26).
- **Browser playtest + ratings** (2026-06-16 -> 2026-06-18): `darwindeck serve`, named game lobby, `seeds export` for blind classic anchors, the served set (`results/2026-06-18-served-set/`), `scripts/ratings-report.sh`.
- **Generative grammar** (`pkg/grammar`, 2026-06-23 -> 2026-06-26): 7 move-generators, 15 modifiers, 137/137 modified families playable by construction, ~81% of in-scope surveyed games representable (`results/2026-06-26-grammar-coverage/`).
- **Bughunts** (2026-07-01, 2026-07-17, 2026-07-24): judge dossier leaks, webplay session lifecycle, grammar fidelity, simulation determinism, operator coverage gaps, deadwood width cliff, Progress contract.

## v2 Open

- NSGA-II / Pareto selection (see above).
- Feed human ratings into fitness (below).
- Wire the grammar into `darwindeck evolve` (it has its own GA/MAP-Elites drivers today).
- Grammar coverage gaps: bid-named trump + bowers (Euchre/Bridge), community cards (Hold'em/Stud).
- A casino choice-consequence prober: Meaningful Decisions over-counts casino (~0.87 of trail-vs-trail non-choices).
- ISMCTS cost: rummy move generation dominates the ~14.5s/batch.

> **Historical-accuracy note (2026-06-11 audit):** the v1 checklists below are preserved as history, but two classes of claims no longer match the v1 code as it stands: (1) the "Python-level process pool (~4x speedup)" and "360x combined" parallelization items -- `ParallelFitnessEvaluator` runs serially on current code (`num_workers` is ignored; Python 3.13 multiprocessing hangs with CGo); (2) the Python-Go golden equivalence tests (`tests/integration/test_bytecode_equivalence.py`) are `@pytest.mark.skip` in the current suite, so cross-language equivalence is not verified.

## Current Status (v1 legacy -- historical)

**Core System: Complete**
- Genome schema with 19 seed games (including 4 betting games)
- Go simulation engine (39x speedup over Python)
- Genetic algorithm with mutation, crossover, selection
- Two-tier skill evaluation (Greedy + MCTS)
- Parallel execution (360x speedup on 256 cores)
- LLM-powered game description generation
- Multiplayer support (2-4 players)
- Card-triggered special effects
- Betting/wagering system for poker-style games
- Team/partnership play with shared scoring

---

## Planned Features

### Schema Extensions

*No pending schema extensions - all planned features complete*

### Fitness Improvements

**Tension Curve Analysis** ✅ Complete
- [x] Track uncertainty over game progression
- [x] Measure dramatic moments (lead changes)
- [x] Penalize anticlimactic endings

**Interaction Metrics** ✅ Complete
- [x] Measure player-to-player effects
- [x] Track blocking, stealing, attacking moves
- [x] Reward interactive over solitaire-like games

### Human Playtesting

**Feedback-Driven Evolution**
- [x] Collect human ratings: v2 `darwindeck serve` appends a 1-5 rating per session to `playtest_results.jsonl`; `scripts/ratings-report.sh` reports per-game means and evolved-vs-classic against blind classic anchors (2026-06-18, commit 9a3a0fe)
- [ ] Use playtest ratings to adjust fitness function (no ratings analysis in the repo yet)
- [ ] Track which evolved mechanics humans find fun
- [ ] A/B testing of rule variations

---

## Known Limitations

### Schema Limitations

| Feature | Status | Workaround |
|---------|--------|------------|
| Real-time actions | Not supported | Turn-based approximation |
| Hidden tableau (Concentration) | Schema ready | `tableau_visibility=FACE_DOWN` - simulation pending |
| Simultaneous play | Not supported | Sequential turns |
| Player choice prompts | Not supported | AI makes all decisions |
| Complex scoring formulas | Limited | Basic threshold scoring only |

### Performance Limitations

| Scenario | Current | Potential Improvement |
|----------|---------|----------------------|
| MCTS depth | 100-2000 iterations | GPU acceleration |
| Very long games | 10,000 turn limit | Early termination heuristics |
| Large populations | 1000 genomes | Distributed evolution |

### Accuracy Limitations

| Metric | Confidence | Issue |
|--------|------------|-------|
| Decision density | High | Well-validated |
| Skill vs luck | Medium | MCTS approximates human skill |
| "Fun" proxy | Low | No human validation yet |

---

## Completed

### Phase 1: Foundation (Complete)
- [x] Genome schema design
- [x] Python simulation prototype
- [x] Go vs Python performance comparison

### Phase 2: Python Core (Complete)
- [x] Immutable game state
- [x] Genome interpreter
- [x] Move generation and validation
- [x] Property-based testing

### Phase 3: Go Performance Core (Complete)
- [x] Bytecode compiler
- [x] FlatBuffers serialization
- [x] CGo bridge
- [x] MCTS implementation
- [x] 39x speedup achieved

### Phase 3.5: Critical Gaps (Complete)
- [x] TrickPhase for trick-taking games
- [x] ClaimPhase for bluffing games
- [x] ConditionType extensions
- [x] 16 seed game encodings

### Phase 4: Genetic Algorithm (Complete)
- [x] Mutation operators
- [x] Crossover operators
- [x] Tournament selection
- [x] Fitness evaluation with skill penalties
- [x] Parallel population evaluation

### Parallelization (Complete)
- [x] Go-level: Worker pool (1.43x speedup)
- [x] Python-level: Process pool (~4x speedup)
- [x] Combined: 360x on 256 cores

### Skill Evaluation (Complete)
- [x] Two-tier evaluation (Greedy + MCTS)
- [x] First-player advantage detection
- [x] In-evolution penalties
- [x] Style-aware skill handling

### Multiplayer Simulation (Complete)
- [x] Go GameState with `[]PlayerState` slice and `NumPlayers`
- [x] FlatBuffers schema with dynamic `wins: [uint32]` and `ai_types: [ubyte]`
- [x] CGo bridge for N-player serialization
- [x] Python fitness metrics for N-player games
- [x] 4-player seed games (Hearts, Spades, President, Knock-Out Whist)

### Special Effects System (Complete)
- [x] EffectType enum and SpecialEffect dataclass
- [x] Go execution (ApplyEffect, AdvanceTurn)
- [x] Bytecode encoding and parsing
- [x] Evolution mutation operators
- [x] Uno-style seed game

### Betting/Wagering System (Complete)
- [x] `BettingPhase` with min_bet and max_raises
- [x] `BettingAction` enum (CHECK, BET, CALL, RAISE, ALL_IN, FOLD)
- [x] Go simulation with betting round loop
- [x] Player chip tracking and pot management
- [x] Showdown resolution with split pot support
- [x] AI betting strategies (Random, Greedy)
- [x] Mutation operators for betting evolution
- [x] 4 betting seed games (simple_poker, draw_poker, betting_war, blackjack)

### Tension Curve Analysis (Complete)
- [x] Game-type-specific leader detectors (Score, HandSize, Trick, TrickAvoidance, Chip)
- [x] TensionMetrics tracking (lead changes, decisive turn, closest margin)
- [x] Go simulation integration with per-turn tracking
- [x] FlatBuffers serialization through CGo bridge
- [x] Python fitness calculation using real tension data
- [x] Integration tests for complete pipeline

### Interactive Playtesting Tool (Complete)
- [x] CLI for playing evolved games manually (`darwindeck.cli.playtest`)
- [x] Human vs AI mode (random, greedy, MCTS difficulties)
- [x] Playtest result recording to JSONL
- [x] Stuck state detection (repeated game states)
- [x] Seed control for reproducible sessions

### Rule Booklet Generation (Complete)
- [x] LLM-powered rulebook generation (`darwindeck.cli.rulebook`)
- [x] Markdown export of game rules
- [x] Human-readable phase descriptions
- [x] Special rules and edge case documentation
- [x] Batch generation for evolution outputs

### Self-Describing Genomes (Complete)
- [x] `CardScoringRule` dataclass for explicit card point values
- [x] `HandEvaluationMethod` enum for poker hands, blackjack, etc.
- [x] `CardValue` dataclass for point totals
- [x] `CardCondition` for matching cards by rank/suit
- [x] `ScoringTrigger` enum for when scoring applies
- [x] Genomes now contain all information needed for rulebook generation

### Semantic Coherence (Complete)
- [x] `SemanticCoherenceChecker` validates mechanics support each other
- [x] Detects orphaned scoring rules (no triggers)
- [x] Detects win conditions without supporting mechanics
- [x] Detects betting phases without chips
- [x] Coherent mutation operators add supporting infrastructure
- [x] Integration with evolution pipeline (pre-simulation validation)

### Tableau Modes (Complete)
- [x] `TableauMode` enum (NONE, SHARED, PER_PLAYER)
- [x] SHARED: Single tableau all players interact with
- [x] PER_PLAYER: Each player has their own tableau
- [x] NONE: No tableau in game
- [x] Go simulation support for all modes

### Interaction Metrics / Solitaire Detection (Complete)
- [x] Multi-signal approach replacing crude interaction_frequency
- [x] Move disruption tracking (opponent turns that change your options)
- [x] Resource contention tracking (competing for same cards/positions)
- [x] Forced response tracking (moves significantly constrained by opponent)
- [x] Go simulation with proper player indexing (capture BEFORE ApplyMove)
- [x] FlatBuffers serialization through CGo bridge
- [x] Python fitness calculation using average of three signals
- [x] Comparison script for validating metrics across game types

### Team Play (Complete)
- [x] `team_mode` and `teams` fields in GameGenome
- [x] Flexible team configurations (2v2, 3v1, etc.)
- [x] Dual scoring (individual + team) tracked simultaneously
- [x] Team-based win condition evaluation
- [x] Precomputed `PlayerToTeam` lookup for O(1) team identification
- [x] Bytecode encoding with team data section
- [x] FlatBuffers schema with `TeamAssignment` table
- [x] Go simulation with `TeamScores`, `PlayerToTeam`, `WinningTeam`
- [x] Team mutation operators (EnableTeamMode, DisableTeamMode, MutateTeamAssignment)
- [x] Partnership Spades seed game
- [x] Full integration tests for team game simulation

### Bidding/Contracts (Complete)
- [x] `BiddingPhase` for contract declarations (min_bid, max_bid, allow_nil)
- [x] `ContractScoring` with points_per_trick_bid, overtrick_points, penalties
- [x] Nil bid support with bonus/penalty
- [x] Bag accumulation and bag penalty system
- [x] Go simulation with bidding round and contract evaluation
- [x] Bytecode encoding for BiddingPhase (PhaseTypeBidding = 7)
- [x] Semantic coherence checks (BiddingPhase requires TrickPhase, contract_scoring requires BiddingPhase)
- [x] Cleanup mutation for orphaned contract_scoring
- [x] Spades seed game with full bidding support

---

## Contributing

To work on a roadmap item:
1. Check if there's an existing plan in `docs/plans/`
2. If not, create a design document first
3. Update this roadmap when work begins
4. Mark items complete when merged

---

## Version History

| Date | Version | Milestone |
|------|---------|-----------|
| 2026-01-10 | 0.1.0 | Initial release with full evolution pipeline |
| 2026-01-11 | 0.1.1 | Diversity-based seeding, documentation updates |
| 2026-01-11 | 0.1.2 | Confirmed multiplayer (2-4 players) fully functional |
| 2026-01-12 | 0.2.0 | Betting/wagering system complete - poker-style games now evolvable |
| 2026-01-12 | 0.2.1 | Tension curve analysis complete - real game tension tracking in fitness |
| 2026-01-17 | 0.3.0 | Interactive playtest CLI and LLM rulebook generation |
| 2026-01-17 | 0.3.1 | Self-describing genomes with CardScoringRule, HandEvaluationMethod |
| 2026-01-17 | 0.3.2 | Semantic coherence checking and coherent mutation operators |
| 2026-01-17 | 0.3.3 | Interaction metrics with multi-signal solitaire detection |
| 2026-01-17 | 0.4.0 | Team play support - partnership games now evolvable |
| 2026-01-21 | 0.5.0 | Bidding/contracts complete - Spades-style games now evolvable |

v2 has no release versions: `darwindeck version` prints `git describe` (no tags exist, so it's a commit hash). v2 milestones by date:

| Date | Milestone |
|------|-----------|
| 2026-04-11 | v2 pure Go rewrite: shedding, trick-taking, rummy skeletons, greedy AI, fitness, evolution engine |
| 2026-06-11 | Audit remediation: rebuilt metrics, calibration gate, determinized ISMCTS + two-tier skill gradient |
| 2026-06-12 | Degeneracy vetoes; failed-review rounds 1-4 |
| 2026-06-13 | Veto-stable publication + `restamp`; experiment matrix; LLM-as-judge; cross-skeleton + novelty-select; climbing skeleton |
| 2026-06-14 | Deep borrows (`run_play`, `follow_suit`, `knock`); CID novelty; casino skeleton; first judge-certified novel games |
| 2026-06-15 | Vying skeleton (6th family); vying scoring host; judge-in-loop selection, chunked checkpoints, judge-aware ranking |
| 2026-06-16 | `darwindeck serve` browser playtest |
| 2026-06-18 | Game lobby, classic anchors, ratings report |
| 2026-06-23 | Complete verdict table + `judge backfill`; grammar prototype |
| 2026-06-26 | Grammar: all six families + vying move-gen, 15 modifiers, ~81% coverage |
| 2026-07-01 | Bughunt: judge leaks, webplay lifecycle, grammar fidelity, evolution hardening |
| 2026-07-15 | Simulation determinism + parallel execution fix |
| 2026-07-17 | Bughunt (all high/medium findings, then close-out) |
| 2026-07-24 | Bughunt: operator coverage gaps, deadwood width cliff, Progress contract; repo-wide gofmt |
