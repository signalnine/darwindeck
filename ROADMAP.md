# DarwinDeck Roadmap

This document tracks planned features, known limitations, and future work.

## Current Status

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
- [ ] Use playtest ratings to adjust fitness function
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
