# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**DarwinDeck** is an evolutionary computation system that uses genetic algorithms and Monte Carlo simulations to evolve novel card games playable with a standard 52-card deck. The system optimizes for configurable parameters like complexity, session length, skill/luck ratio, and player count.

## Core Architecture Concepts

### Game Representation
Games are encoded as genomes containing:
- **Setup rules**: Deal patterns, hand sizes, tableau configuration
- **Turn structure**: Draw/play/pass rules
- **Valid moves**: Card matching logic (suit, rank, sequence), special card effects
- **Win conditions**: Empty hand, point thresholds, capture goals
- **Scoring system**: Point values, bonuses, penalties

### Three-Layer System
1. **Rule DSL**: Domain-specific language for defining card game rules
2. **Simulation Engine**: Monte Carlo runner with multiple AI player types (random, greedy, MCTS)
3. **Genetic Algorithm**: Evolution engine with mutation, crossover, and selection operators

### Fitness Evaluation
Games are scored on measurable proxies for "fun":
- Decision density (meaningful choices vs forced plays)
- Comeback potential (trailing players can recover)
- Tension curve (uncertainty over time)
- Interaction frequency (actions affecting opponents)
- Rules complexity and session length
- Skill vs luck ratio (MCTS win rate vs random)

### Fitness Style Presets

The system supports 5 fitness style presets that weight metrics differently for different game types:

| Style | Description | Key Weights |
|-------|-------------|-------------|
| `balanced` | Default preset, well-rounded games | skill_vs_luck: 0.30, decision_density: 0.20 |
| `bluffing` | Hidden information, betting, player interaction | interaction_frequency: 0.30, skill_vs_luck: 0.15 |
| `strategic` | Deep thinking, skill-based play | skill_vs_luck: 0.40, decision_density: 0.25 |
| `party` | Quick, interactive, accessible games | comeback_potential: 0.25, skill_vs_luck: 0.05 |
| `trick-taking` | Trick-based mechanics | interaction_frequency: 0.25, balanced elsewhere |

**Usage via CLI:**
```bash
uv run python -m darwindeck.cli.evolve --style strategic
```

**Usage via script:**
```bash
STYLE=bluffing ./scripts/run-evolution.sh
```

**Preset definitions:** `src/darwindeck/evolution/fitness_full.py`

## Performance and Parallelization

The system implements **two-level parallelization** to maximize throughput on multi-core systems:

### Go-Level Parallelization (Phase 3)
- **Implementation:** Worker pool pattern in `src/gosim/simulation/parallel.go`
- **Performance:** 1.43x average speedup on 4-core systems (1.61x with GreedyAI)
- **Usage:** Call `RunBatchParallel()` instead of `RunBatchSimulation()`
- **Optimal batch size:** 500-1000 games per evaluation
- **Memory overhead:** < 0.5% (negligible)

### Python-Level Parallelization (Phase 4)
- **Implementation:** Process pool in `src/darwindeck/evolution/parallel_fitness.py`
- **Performance:** ~4x speedup on 4-core systems for population evaluation
- **Usage:** Use `ParallelFitnessEvaluator` for evaluating multiple genomes
- **Process-safe:** Each worker gets isolated Go simulator instance

### Combined Performance
- **Total speedup:** 3.3-4.0x end-to-end on 4-core systems
- **Throughput:** 3,000-4,000 games/second
- **Population (100 genomes):** ~25-30 seconds
- **Full evolution (100 generations):** ~42-50 minutes

### Performance Characteristics
- **Embarrassingly parallel:** Simulations are independent
- **Memory-bandwidth limited:** Speedup constrained by memory access, not computation
- **Better scaling with complex AI:** GreedyAI and MCTS benefit more than RandomAI
- **Negligible overhead:** Worthwhile even for small batches (100+ games)

### Documentation
- **Usage guide:** `docs/quickstart/parallelization-usage.md`
- **Performance results:** `docs/benchmarks/parallelization-results.md`
- **Strategy:** `docs/parallelization-strategy.md`
- **Benchmarks:** `BENCHMARK_ANALYSIS.md`, `BENCHMARK_SUMMARY.md`

### Recommended Configuration
- Use default auto-detection for worker counts
- Batch size: 1000 games for standard fitness evaluation
- Monitor throughput (should be > 1,000 games/sec)

## Key Constraints

All evolved games must be:
- **Playable**: No infinite loops or unreachable win states
- **Terminable**: Enforce maximum turn limits
- **Agentic**: Contain non-random decision points

## Development Approach

When implementing, follow this sequence:
1. Rule DSL design and parser
2. Simulation harness with random AI baseline
3. Game state representation and validation
4. Genetic operators (mutation, crossover, selection)
5. Advanced AI players (greedy heuristics, MCTS)
6. Fitness function implementation
7. Natural language rule generator for human playtesting

## Validation Strategy

- Random AI validates games are mechanically playable
- Greedy AI measures obvious strategy effectiveness
- MCTS approximates skilled play
- Skill gap = MCTS win rate differential vs random baseline
- Human playtesting validates proxy metrics correlate with actual enjoyment

## Performance Benchmarks

### Golang Core Validation (Phase 1)

**War Game Benchmark:**
- Python implementation: 0.07ms per game
- Golang implementation: 0.03ms per game
- Measured speedup: 2.9x

**Interface Decision:** CGo

**Rationale:** Despite modest speedup on simple War game, Golang provides measurable performance benefit. More complex simulations (MCTS, deep game trees) will show greater advantages. CGo chosen for tight integration without serialization overhead, critical for millions of evolutionary iterations.

### Golang Performance Core (Phase 3)

**Architecture:** Python→Flatbuffers→CGo→Go

**Implementation Status:** Core architecture complete, simulation logic in progress

**Components Implemented:**
1. **Bytecode Compiler** (`src/darwindeck/genome/bytecode.py`)
   - Compiles GameGenome to 36-byte header + phase data
   - OpCodes for conditions (0-19), actions (20-39), control flow (40-49), operators (50-55)
   - War genome compiles to 77 bytes
   - Deterministic compilation (same genome = same bytecode)

2. **Flatbuffers Schema** (`schema/simulation.fbs`)
   - Zero-copy binary serialization for Python↔Go
   - BatchRequest/BatchResponse for bulk simulation
   - Supports Random/Greedy/MCTS AI types (100/500/1000/2000 iterations)

3. **CGo Bridge** (`src/gosim/cgo/bridge.go`, `src/darwindeck/bindings/cgo_bridge.py`)
   - `libcardsim.so` shared library (1.8MB)
   - `SimulateBatch` entry point for batch processing
   - Memory management via `FreeCString`
   - Built with: `make build-cgo`

4. **Mutable GameState** (`src/gosim/engine/types.go`)
   - sync.Pool memory pooling for zero-allocation reuse
   - Card, PlayerState, GameState types with betting extensions
   - DrawCard, PlayCard, ShuffleDeck operations
   - Clone() for tree search
   - 3/3 unit tests passing

5. **Genome Interpreter** (`src/gosim/engine/bytecode.go`, `conditions.go`, `movegen.go`)
   - ParseGenome extracts header + phases + win conditions
   - EvaluateCondition handles hand size, location size, card rank/suit, betting
   - GenerateLegalMoves produces move list for current phase
   - ApplyMove mutates state in-place
   - CheckWinConditions returns winner ID
   - 7/7 unit tests passing

6. **MCTS Engine** (`src/gosim/mcts/node.go`, `search.go`)
   - MCTSNode with sync.Pool memory pooling
   - UCB1 selection algorithm (configurable exploration parameter)
   - Selection → Expansion → Simulation → Backpropagation
   - Returns most-visited child as best move
   - Benchmark: ~3ms per search (100 iterations)
   - 8/8 unit tests passing

7. **Batch Simulation Engine** (`src/gosim/simulation/runner.go`)
   - RunBatch: Execute N games with same genome/AI config
   - Random AI: Uniform selection from legal moves
   - Greedy AI: Heuristic-based move scoring
   - MCTS AI: Tree search with configurable iterations
   - Aggregated statistics: wins, avg/median turns, duration
   - Turn limit protection against infinite loops

8. **Golden Test Suite** (`tests/integration/test_bytecode_equivalence.py`, `src/gosim/engine/bytecode_test.go`)
   - Python bytecode compilation tests (5 passing)
   - Go bytecode parsing tests (4 passing)
   - `tests/golden/war_genome.bin` (77 bytes) validates Python↔Go equivalence
   - Determinism verification

**Performance Status:** ✅ **COMPLETE - TARGET ACHIEVED**

**Fair Comparison Results (100 games, genome-based):**
- **Python:** 15.94ms per game (63 games/sec)
- **Go:** 0.40ms per game (2,472 games/sec)
- **Speedup:** **39.4x** ✅ (within 10-50x target range)

**Implementation Details:**
- Both use genome interpreter stack (move generation, phase execution, battle resolution)
- Python: `src/darwindeck/simulation/movegen.py` - immutable state with `copy_with()`
- Go: `src/gosim/engine/movegen.go` - mutable state with `sync.Pool`
- MCTS search: ~3ms per 100 iterations

**Why Initial Benchmark Was Misleading:**
- **Invalid comparison (0.2x):** Python direct War (0.07ms) vs Go genome-based (0.43ms)
- **Valid comparison (39.4x):** Python genome-based (15.94ms) vs Go genome-based (0.40ms)
- Lesson: Always compare equivalent architectures

**Performance Benefits:**
- Compiled native code vs interpreted Python
- Memory pooling with `sync.Pool` (zero-allocation reuse)
- Mutable state (in-place updates) vs immutable copies
- Batching amortizes CGo overhead

**Absolute Performance:** 2,472 games/sec sufficient for evolutionary workloads:
- 1 million games ≈ 7 minutes
- Batching of 50-100 games per CGo call
- Can scale with parallelization if needed

**Phase 3 Status:** ✅ **SUCCESS** - Target achieved, ready for Phase 4

See `PHASE3_FINAL_RESULTS.md` for comprehensive analysis and `benchmarks/compare_genome_implementations.py` for fair benchmark.

## Betting System

**Status:** ✅ **COMPLETE** - Poker-style games are now evolvable

The betting system enables poker, blackjack, and other betting card games through evolution.

### Architecture

```
GameGenome
├── setup
│   └── starting_chips: int (0 = no betting)
├── turn_structure
│   └── phases: [..., BettingPhase(min_bet, max_raises), ...]
└── win_conditions (used for showdown)
```

### BettingPhase

```python
BettingPhase(
    min_bet: int = 10,      # Minimum bet/raise amount
    max_raises: int = 3     # Max raises per round (prevents infinite loops)
)
```

### Betting Actions

| Action | Description |
|--------|-------------|
| CHECK | Pass without betting (only if no current bet) |
| BET | Place initial bet (min_bet amount) |
| CALL | Match current bet |
| RAISE | Increase bet by min_bet |
| ALL_IN | Bet all remaining chips |
| FOLD | Surrender hand, forfeit pot |

### Key Features

- **Short-stack support:** Players can go ALL_IN instead of forced fold
- **Round termination:** Ends when all bets matched or one player remains
- **Split pot support:** Ties divide pot evenly
- **Position rotation:** Starting player rotates each hand for fairness
- **AI support:** Random and Greedy AI betting strategies

### Seed Genomes with Betting

| Genome | Starting Chips | Min Bet | Max Raises |
|--------|----------------|---------|------------|
| `simple_poker` | 1000 | 10 | 3 |
| `betting_war` | 500 | 10 | 2 |
| `draw_poker` | 1000 | 20 | 3 |
| `blackjack` | 500 | 25 | 1 |

### Mutation Operators

- `AddBettingPhaseMutation` - Insert betting round
- `RemoveBettingPhaseMutation` - Remove betting round
- `MutateBettingPhaseMutation` - Modify min_bet/max_raises
- `MutateStartingChipsMutation` - Change starting chips

**Constraint:** All mutations ensure `min_bet <= starting_chips`

### Documentation

- **Design:** `docs/plans/2026-01-11-betting-system-design.md`
- **Implementation:** `docs/plans/2026-01-11-betting-system-implementation.md`

## Development Commands

### Run Tests

**Python:**
```bash
uv run pytest tests/ -v
uv run pytest tests/unit/test_specific.py -v  # Single file
```

**Golang:**
```bash
cd src/gosim
go test ./engine -v
go test ./mcts -v
go test ./simulation -v

# Benchmarks
go test ./mcts -bench=. -benchtime=3s
go test ./simulation -bench=. -benchtime=10s
```

### Build CGo Library

```bash
make build-cgo  # Builds libcardsim.so
```

### Benchmarks

```bash
# Phase 3: Golang performance core
uv run python benchmarks/benchmark_golang.py

# Phase 1: Python vs Go comparison
uv run python benchmarks/compare_war.py
```

### Code Quality

```bash
uv run black src/ tests/
uv run mypy src/
```

## Project Structure

```
darwindeck/
├── src/
│   ├── darwindeck/          # Python package
│   │   ├── genome/            # Game genome representation
│   │   ├── simulation/        # Game simulation engines
│   │   ├── evolution/         # Genetic algorithm
│   │   └── cli/               # Command-line interface
│   └── gosim/                 # Golang simulation core
│       └── game/              # Card game primitives
├── tests/
│   ├── unit/                  # Unit tests
│   ├── integration/           # Integration tests
│   └── property/              # Property-based tests (Hypothesis)
├── benchmarks/                # Performance comparisons
└── docs/
    ├── architecture/          # Architecture decisions
    └── plans/                 # Implementation plans
```

## Phase 2: Python Simulation Core Patterns

### Immutability Patterns

**Always use tuples for nested structures:**
```python
# ✅ Correct
@dataclass(frozen=True)
class GameState:
    deck: tuple[Card, ...]
    hands: tuple[tuple[Card, ...], ...]

# ❌ Wrong - frozen wrapper around mutable list
@dataclass(frozen=True)
class GameState:
    deck: list[Card]  # NOT immutable!
```

**Use copy_with for state transitions:**
```python
new_state = old_state.copy_with(
    turn=old_state.turn + 1,
    active_player=(old_state.active_player + 1) % 2
)
```

### Interpreter Pattern

**GenomeInterpreter converts data to logic (NOT code generation):**
- Genomes are pure dataclasses
- GameLogic instantiates based on genome fields
- Safe for pickling across processes
- No `exec()`, no security risks

### Testing Strategies

**Property-based tests with Hypothesis:**
```python
@given(seed=st.integers(min_value=0, max_value=10000))
def test_determinism_property(seed: int) -> None:
    result1 = engine.simulate_game(genome, players, seed)
    result2 = engine.simulate_game(genome, players, seed)
    assert result1.winner == result2.winner
```

**Sanity checks for metrics:**
- War game should have decision_density ≈ 0.0
- If not, metric calculation is broken

### Two-Phase Fitness Evaluation

**Phase 1 (cheap, all candidates):**
- Game length, completion rate, decision branch factor

**Phase 2 (expensive, top 10-20%):**
- Tension curve, comeback potential (deferred to Phase 3)

### Rank Comparison

**Use numeric mapping for Rank enum:**
```python
# Rank enum values are strings ("A", "2", "K")
# For comparison, use numeric mapping:
RANK_VALUES = {
    Rank.TWO: 2,
    Rank.THREE: 3,
    # ...
    Rank.ACE: 14,  # Ace high
}

def get_rank_value(card: Card) -> int:
    return RANK_VALUES[card.rank]
```

## Genome Validation and Coherence

The system has two levels of validation to ensure evolved games are playable:

### Structural Validation (`GenomeValidator`)

Located in `src/darwindeck/genome/validator.py`. Checks basic structural requirements:

```python
from darwindeck.genome.validator import GenomeValidator

errors = GenomeValidator.validate(genome)
if errors:
    print("Invalid genome:", errors)
```

**Validates:**
- At least one card play phase exists
- Card counts don't exceed deck size (52 cards)
- Score-based win conditions have scoring rules
- BettingPhase with HAND_EVALUATION showdown has hand_evaluation config

### Semantic Coherence (`SemanticCoherenceChecker`)

Located in `src/darwindeck/evolution/coherence.py`. Checks that mechanics support each other:

```python
from darwindeck.evolution.coherence import SemanticCoherenceChecker

checker = SemanticCoherenceChecker()
result = checker.check(genome)
if not result.coherent:
    print("Incoherent:", result.violations)
```

**Validates:**
- Capture win conditions (`capture_all`, `most_captured`) have tableau phases
- Scoring win conditions (`high_score`, `low_score`, `first_to_score`) have scoring rules OR is_trick_based
- `starting_chips > 0` has at least one `BettingPhase`

### Coherent Mutation Operators

Mutation operators in `src/darwindeck/evolution/operators.py` are designed to produce coherent genomes:

- `ModifyWinConditionMutation` - When changing to scoring-based win condition, automatically adds `card_scoring` if missing
- `MutateStartingChipsMutation` - When enabling betting (0 → chips), automatically adds `BettingPhase`

**Key principle:** Mutations that add a mechanic must also add its supporting infrastructure.

## Self-Describing Genome Features

The genome schema (`src/darwindeck/genome/schema.py`) includes explicit self-describing fields:

### Card Scoring (`card_scoring`)

```python
card_scoring: tuple[CardScoringRule, ...] = ()

# Example: Hearts are worth 1 point when captured in tricks
CardScoringRule(
    condition=CardCondition(suit=Suit.HEARTS),
    points=1,
    trigger=ScoringTrigger.TRICK_WIN
)
```

**Triggers:** `TRICK_WIN`, `CAPTURE`, `PLAY`, `HAND_END`, `SET_COMPLETE`

### Hand Evaluation (`hand_evaluation`)

For poker-style games with pattern matching:

```python
hand_evaluation: Optional[HandEvaluation] = None

HandEvaluation(
    method=HandEvaluationMethod.PATTERN_MATCH,
    patterns=[
        HandPattern(name="flush", priority=5, ...),
        HandPattern(name="straight", priority=4, ...),
    ]
)
```

**Methods:** `NONE`, `HIGH_CARD`, `POINT_TOTAL`, `PATTERN_MATCH`, `CARD_COUNT`

### Card Values (`card_values`)

For blackjack-style point totals:

```python
card_values: tuple[CardValue, ...] = ()

CardValue(
    condition=CardCondition(rank=Rank.ACE),
    value=11,
    alternate_value=1  # Ace can be 1 or 11
)
```

## Evolution Pipeline

### Pipeline Flow

```
Seed Genomes → Population Init → [Mutation/Crossover → Validation → Simulation → Fitness → Selection] × N generations
```

### Validation Integration

Validation happens at multiple stages:

1. **Before Simulation** (`parallel_fitness.py`): `GenomeValidator.validate()` - structurally invalid genomes get zero fitness without running expensive simulation
2. **After Evolution** (`evolve.py`): `SemanticCoherenceChecker` - incoherent genomes are skipped when saving top results
3. **During Mutation**: Operators ensure coherent changes

### Fitness Evaluation Flow

```python
# In parallel_fitness.py _evaluate_task()
1. GenomeValidator.validate(genome)  # Structural check
   → If errors: return FitnessMetrics(valid=False, total_fitness=0.0)

2. simulator.simulate(genome, num_games=100)  # Go engine
   → Returns SimulationResults with game stats

3. evaluator.evaluate(genome, results)  # Calculate metrics
   → Returns FitnessMetrics with decision_density, comeback_potential, etc.
```

### Skill Evaluation

Top genomes undergo skill evaluation to measure strategic depth:

- **Greedy vs Random:** Does obvious strategy beat random play?
- **MCTS vs Random:** Does deep lookahead provide advantage?
- **FPA (First Player Advantage):** Penalize games where P0 always wins

```bash
# In evolution output:
HighTell: greedy=98% mcts=88% skill=0.93
```

## Playtest System

### CLI Playtest

```bash
# Interactive playtest with human player
uv run python -m darwindeck.cli.playtest path/to/genome.json --difficulty greedy --seed 42

# Options:
--difficulty random|greedy|mcts  # AI opponent strength
--seed N                          # Reproducible games
--debug                           # Show AI hand and full state
```

### Helper Script

```bash
# Quick launch with Rich TUI display
./scripts/tui.sh                           # Interactive genome picker
./scripts/tui.sh path/to/genome.json       # Specific genome
./scripts/tui.sh --latest                  # Most recent rank01 from evolution
./scripts/tui.sh --latest --difficulty mcts  # With options
```

### Rich TUI Display

When running in an interactive terminal, playtest automatically uses a Rich-based display with:
- **Colored card suits** - Red hearts/diamonds, adaptive clubs/spades
- **Boxed panels** - Game state, hand, actions, opponent info
- **Move history** - Last 5 moves shown
- **Compact mode** - Automatic layout for narrow terminals (<60 cols)

To force plain text mode (for piped input or scripting):
```bash
FORCE_PLAIN_DISPLAY=1 uv run python -m darwindeck.cli.playtest genome.json
```

### Rulebook Generation

```bash
# Generate human-readable rules
uv run python -m darwindeck.cli.rulebook path/to/genome.json --output rulebook.md
```

### PlaytestSession API

```python
from darwindeck.playtest.session import PlaytestSession, SessionConfig
from darwindeck.genome.serialization import genome_from_dict

config = SessionConfig(difficulty='greedy', seed=42)
session = PlaytestSession(genome, config)
session.run()  # Interactive game loop
```

### Stuck Detection

The `StuckDetector` in `src/darwindeck/playtest/stuck.py` prevents infinite loops:
- Tracks game state hashes
- Detects repeated states (cycles)
- Enforces max turn limits

## Common Issues and Debugging

### "Score-based win condition requires scoring_rules"

**Cause:** Genome has `high_score`/`low_score`/`first_to_score` win condition but no scoring mechanism.

**Fix:** Either:
1. Add `card_scoring` rules to genome
2. Set `turn_structure.is_trick_based = True` (trick games track score via captures)
3. Add `scoring_rules` (legacy field)

### "starting_chips but no BettingPhase"

**Cause:** Genome has chips but no betting round to use them.

**Fix:** Add a `BettingPhase` to `turn_structure.phases`.

### Games Getting Stuck in Betting

**Symptoms:** Betting phase never completes, infinite loop.

**Common causes:**
1. `is_all_in` not set when chips depleted (fixed in `movegen.py`)
2. No player can act but phase doesn't advance (fixed with `_should_auto_complete_betting()`)

**Debug:** Check `count_acting_players()` - if 0, betting should auto-complete.

### Genome Loading

```python
# From JSON file
import json
from darwindeck.genome.serialization import genome_from_dict

with open('genome.json') as f:
    genome = genome_from_dict(json.load(f))

# From JSON string
from darwindeck.genome.serialization import genome_from_json
genome = genome_from_json(json_string)
```

## CLI Commands

### Evolution

```bash
# Basic evolution
uv run python -m darwindeck.cli.evolve --generations 50 --population-size 30

# With style preset
uv run python -m darwindeck.cli.evolve --style bluffing

# Skip expensive skill evaluation
uv run python -m darwindeck.cli.evolve --skip-skill-eval

# Full options
uv run python -m darwindeck.cli.evolve \
    --generations 100 \
    --population-size 50 \
    --style balanced \
    --player-count 2 \
    --output-dir output/my-run \
    --save-top-n 20 \
    --verbose
```

### Playtest

```bash
uv run python -m darwindeck.cli.playtest genome.json --difficulty greedy
```

### Rulebook

```bash
uv run python -m darwindeck.cli.rulebook genome.json --output rules.md
```

### Describe (AI-generated game description)

```bash
uv run python -m darwindeck.cli.describe genome.json
```

## Key Files Reference

| File | Purpose |
|------|---------|
| `src/darwindeck/genome/schema.py` | GameGenome dataclass, all game components |
| `src/darwindeck/genome/validator.py` | Structural validation |
| `src/darwindeck/evolution/coherence.py` | Semantic coherence checking |
| `src/darwindeck/evolution/operators.py` | Mutation and crossover operators |
| `src/darwindeck/evolution/fitness_full.py` | Fitness metrics and evaluation |
| `src/darwindeck/evolution/parallel_fitness.py` | Parallel genome evaluation |
| `src/darwindeck/simulation/movegen.py` | Move generation and application |
| `src/darwindeck/simulation/go_simulator.py` | Go engine interface |
| `src/darwindeck/playtest/session.py` | Interactive playtest session |
| `src/darwindeck/playtest/rich_display.py` | Rich TUI display rendering |
| `src/darwindeck/playtest/display_state.py` | Display state dataclasses |
| `src/gosim/engine/` | Go simulation core |
| `src/gosim/mcts/` | MCTS AI implementation |
| `scripts/tui.sh` | Helper script for playtest TUI |
