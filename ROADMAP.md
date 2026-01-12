# DarwinDeck Roadmap

This document tracks planned features, known limitations, and future work.

## Current Status

**Core System: Complete**
- Genome schema with 16 seed games
- Go simulation engine (39x speedup over Python)
- Genetic algorithm with mutation, crossover, selection
- Two-tier skill evaluation (Greedy + MCTS)
- Parallel execution (360x speedup on 256 cores)
- LLM-powered game description generation
- Multiplayer support (2-4 players)
- Card-triggered special effects

---

## Planned Features

### Schema Extensions

**Betting/Wagering System**
- [ ] `ResourceRules` - Chip/resource tracking
- [ ] `BettingPhase` - Betting round mechanics
- [ ] Conditions: `CHIP_COUNT`, `POT_SIZE`, `CAN_AFFORD`
- [ ] Actions: `BET`, `CALL`, `RAISE`, `FOLD`, `CHECK`

**Use cases:** Poker, Blackjack with betting

**Team Play**
- [ ] Partnership tracking
- [ ] Shared scoring between teammates
- [ ] Team-based win conditions

**Use cases:** Spades (partnership), Bridge, Euchre

**Bidding/Contracts**
- [ ] `BiddingPhase` - Contract declarations
- [ ] Bid tracking and validation
- [ ] Contract-based scoring

**Use cases:** Bridge, Spades with bidding, Pinochle

### Fitness Improvements

**Tension Curve Analysis**
- [ ] Track uncertainty over game progression
- [ ] Measure dramatic moments (lead changes)
- [ ] Penalize anticlimactic endings

**Interaction Metrics**
- [ ] Measure player-to-player effects
- [ ] Track blocking, stealing, attacking moves
- [ ] Reward interactive over solitaire-like games

### Human Playtesting

**Interactive Playtesting Tool**
- [ ] CLI for playing evolved games manually
- [ ] Human vs AI mode
- [ ] Feedback collection for fitness refinement

**Rule Booklet Generation**
- [ ] PDF export of game rules
- [ ] Include setup diagrams
- [ ] Strategy hints from MCTS analysis

---

## Known Limitations

### Schema Limitations

| Feature | Status | Workaround |
|---------|--------|------------|
| Real-time actions | Not supported | Turn-based approximation |
| Hidden tableau (Concentration) | Not supported | None |
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
