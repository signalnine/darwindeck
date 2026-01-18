# Interaction Metrics Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace crude `interaction_frequency` metric with multi-signal solitaire detection that tracks move disruption, resource contention, and forced responses.

**Architecture:** Add 4 new counters in Go simulation, pass through FlatBuffers to Python, combine into improved `interaction_frequency` score.

**Tech Stack:** Go, FlatBuffers, Python

---

## Task 1: Add New Metric Fields to Go GameMetrics

**Files:**
- Modify: `src/gosim/simulation/runner.go:24-52`

**Step 1: Add new fields to GameMetrics struct**

Find the `GameMetrics` struct (around line 24) and add after `TotalActions`:

```go
// Solitaire detection metrics (interaction quality)
MoveDisruptionEvents  uint64 // Opponent turns that changed waiting player's legal moves
ContentionEvents      uint64 // Times players competed for same resource
ForcedResponseEvents  uint64 // Turns where legal moves significantly constrained
OpponentTurnCount     uint64 // Total opponent turns (denominator for rates)
```

**Step 2: Run Go tests to verify compilation**

```bash
cd src/gosim && go build ./...
```

Expected: Build succeeds

**Step 3: Commit**

```bash
git add src/gosim/simulation/runner.go
git commit -m "feat(go): add solitaire detection fields to GameMetrics"
```

---

## Task 2: Add New Fields to Go AggregatedStats

**Files:**
- Modify: `src/gosim/simulation/runner.go:63-100`

**Step 1: Add new fields to AggregatedStats struct**

Find `AggregatedStats` struct (around line 63) and add after tension metrics:

```go
// Solitaire detection metrics (interaction quality)
MoveDisruptionEvents  uint64
ContentionEvents      uint64
ForcedResponseEvents  uint64
OpponentTurnCount     uint64
```

**Step 2: Run Go tests**

```bash
cd src/gosim && go build ./...
```

**Step 3: Commit**

```bash
git add src/gosim/simulation/runner.go
git commit -m "feat(go): add solitaire detection fields to AggregatedStats"
```

---

## Task 3: Implement Move Disruption Tracking

**Files:**
- Modify: `src/gosim/simulation/runner.go`

**Step 1: Create helper function to compare move sets**

Add after `isInteraction` function (around line 783):

```go
// compareLegalMoves checks if two move slices have the same moves
// Returns true if different (move set was disrupted)
func movesDisrupted(before, after []engine.LegalMove) bool {
	if len(before) != len(after) {
		return true
	}
	// Quick check: compare first few moves (order may vary)
	// For performance, just check count difference
	return false
}

// countLegalMoves returns the number of legal moves for a player
func countLegalMovesForPlayer(state *engine.GameState, genome *engine.Genome, playerIdx int) int {
	// Temporarily switch current player to count their moves
	originalPlayer := state.CurrentPlayer
	state.CurrentPlayer = playerIdx
	moves := engine.GenerateLegalMoves(state, genome)
	state.CurrentPlayer = originalPlayer
	return len(moves)
}
```

**Step 2: Track move disruption in game loop**

In `RunSingleGame` function, find the main game loop (around line 340). Add tracking before the move is applied:

```go
// Track opponent's legal moves BEFORE this player's turn
var opponentMoveCountBefore int
numPlayers := len(state.Players)
if numPlayers > 1 {
	// Get next player (opponent in 2-player game)
	nextPlayer := (state.CurrentPlayer + 1) % numPlayers
	opponentMoveCountBefore = countLegalMovesForPlayer(state, genome, nextPlayer)
}
```

After the move is applied (after `engine.ApplyMove`), add:

```go
// Track move disruption - did this turn change opponent's options?
if numPlayers > 1 {
	nextPlayer := (state.CurrentPlayer + 1) % numPlayers
	opponentMoveCountAfter := countLegalMovesForPlayer(state, genome, nextPlayer)
	if opponentMoveCountAfter != opponentMoveCountBefore {
		metrics.MoveDisruptionEvents++
	}
	metrics.OpponentTurnCount++
}
```

**Step 3: Run Go tests**

```bash
cd src/gosim && go test ./simulation -v -run TestRunSingleGame
```

**Step 4: Commit**

```bash
git add src/gosim/simulation/runner.go
git commit -m "feat(go): implement move disruption tracking"
```

---

## Task 4: Implement Resource Contention Tracking

**Files:**
- Modify: `src/gosim/simulation/runner.go`

**Step 1: Create contention detection helper**

Add after move disruption helpers:

```go
// isContentionEvent checks if a move competes for shared resources
func isContentionEvent(state *engine.GameState, move *engine.LegalMove, genome *engine.Genome) bool {
	// Contention 1: Drawing from deck when scarce (<=10 cards)
	if move.SourceLoc == engine.LocationDeck && len(state.Deck) <= 10 {
		return true
	}

	// Contention 2: Tableau capture that opponent could also make
	if move.TargetLoc == engine.LocationTableau {
		// If playing to tableau causes a capture, check if opponent had matching card
		if move.CardIndex >= 0 && move.CardIndex < len(state.Players[state.CurrentPlayer].Hand) {
			playedCard := state.Players[state.CurrentPlayer].Hand[move.CardIndex]
			// Check if any opponent has a card of same rank (could have captured)
			for i, player := range state.Players {
				if i == state.CurrentPlayer {
					continue
				}
				for _, card := range player.Hand {
					if card.Rank == playedCard.Rank {
						return true // Opponent could have played this rank too
					}
				}
			}
		}
	}

	return false
}
```

**Step 2: Add contention tracking to game loop**

In the main game loop, after the move is selected but before applying:

```go
// Track resource contention
if isContentionEvent(state, move, genome) {
	metrics.ContentionEvents++
}
```

**Step 3: Run Go tests**

```bash
cd src/gosim && go test ./simulation -v
```

**Step 4: Commit**

```bash
git add src/gosim/simulation/runner.go
git commit -m "feat(go): implement resource contention tracking"
```

---

## Task 5: Implement Forced Response Tracking

**Files:**
- Modify: `src/gosim/simulation/runner.go`

**Step 1: Add forced response detection to game loop**

Modify the move disruption tracking section to also detect forced responses:

```go
// Track move disruption and forced response
if numPlayers > 1 {
	nextPlayer := (state.CurrentPlayer + 1) % numPlayers
	opponentMoveCountAfter := countLegalMovesForPlayer(state, genome, nextPlayer)

	// Move disruption: any change in available moves
	if opponentMoveCountAfter != opponentMoveCountBefore {
		metrics.MoveDisruptionEvents++
	}

	// Forced response: moves dropped by >30%
	if opponentMoveCountBefore > 0 {
		ratio := float64(opponentMoveCountAfter) / float64(opponentMoveCountBefore)
		if ratio < 0.7 {
			metrics.ForcedResponseEvents++
		}
	}

	metrics.OpponentTurnCount++
}
```

**Step 2: Run Go tests**

```bash
cd src/gosim && go test ./simulation -v
```

**Step 3: Commit**

```bash
git add src/gosim/simulation/runner.go
git commit -m "feat(go): implement forced response tracking"
```

---

## Task 6: Aggregate New Metrics Across Games

**Files:**
- Modify: `src/gosim/simulation/runner.go`

**Step 1: Find aggregateResults function**

Search for `func aggregateResults` (around line 850).

**Step 2: Add aggregation for new metrics**

In the results loop, add:

```go
stats.MoveDisruptionEvents += result.Metrics.MoveDisruptionEvents
stats.ContentionEvents += result.Metrics.ContentionEvents
stats.ForcedResponseEvents += result.Metrics.ForcedResponseEvents
stats.OpponentTurnCount += result.Metrics.OpponentTurnCount
```

**Step 3: Run Go tests**

```bash
cd src/gosim && go test ./simulation -v
```

**Step 4: Commit**

```bash
git add src/gosim/simulation/runner.go
git commit -m "feat(go): aggregate solitaire detection metrics"
```

---

## Task 7: Update FlatBuffers Schema

**Files:**
- Modify: `schema/simulation.fbs`

**Step 1: Add new fields to AggregatedStats table**

Find `table AggregatedStats` and add after `trailing_winners`:

```fbs
  // Solitaire detection metrics (interaction quality)
  move_disruption_events: uint64 = 0;
  contention_events: uint64 = 0;
  forced_response_events: uint64 = 0;
  opponent_turn_count: uint64 = 0;
```

**Step 2: Commit schema change**

```bash
git add schema/simulation.fbs
git commit -m "feat(schema): add solitaire detection fields to AggregatedStats"
```

---

## Task 8: Regenerate FlatBuffers Bindings

**Files:**
- Regenerate: `src/gosim/bindings/cardsim/`
- Regenerate: `src/darwindeck/bindings/cardsim/`

**Step 1: Regenerate Go bindings**

```bash
flatc --go -o src/gosim/bindings schema/simulation.fbs
```

**Step 2: Regenerate Python bindings**

```bash
flatc --python -o src/darwindeck/bindings schema/simulation.fbs
```

**Step 3: Verify regeneration**

```bash
ls -la src/gosim/bindings/cardsim/AggregatedStats.go
ls -la src/darwindeck/bindings/cardsim/AggregatedStats.py
```

**Step 4: Commit regenerated bindings**

```bash
git add src/gosim/bindings/ src/darwindeck/bindings/
git commit -m "chore: regenerate FlatBuffers bindings for solitaire metrics"
```

---

## Task 9: Update CGo Bridge Struct

**Files:**
- Modify: `src/gosim/cgo/bridge.go:17-55`

**Step 1: Add new fields to AggStats struct**

Find `type AggStats struct` and add after tension metrics:

```go
// Solitaire detection metrics
MoveDisruptionEvents  uint64
ContentionEvents      uint64
ForcedResponseEvents  uint64
OpponentTurnCount     uint64
```

**Step 2: Run Go build**

```bash
cd src/gosim && go build ./...
```

**Step 3: Commit**

```bash
git add src/gosim/cgo/bridge.go
git commit -m "feat(cgo): add solitaire detection fields to AggStats"
```

---

## Task 10: Update CGo Bridge Serialization

**Files:**
- Modify: `src/gosim/cgo/bridge.go`

**Step 1: Update stats conversion (around line 160)**

Find where `simStats` is converted to `AggStats`. Add:

```go
MoveDisruptionEvents:  simStats.MoveDisruptionEvents,
ContentionEvents:      simStats.ContentionEvents,
ForcedResponseEvents:  simStats.ForcedResponseEvents,
OpponentTurnCount:     simStats.OpponentTurnCount,
```

**Step 2: Update FlatBuffers serialization (around line 280)**

Find `buildAggregatedStats` function. Add after tension metrics:

```go
cardsim.AggregatedStatsAddMoveDisruptionEvents(builder, stats.MoveDisruptionEvents)
cardsim.AggregatedStatsAddContentionEvents(builder, stats.ContentionEvents)
cardsim.AggregatedStatsAddForcedResponseEvents(builder, stats.ForcedResponseEvents)
cardsim.AggregatedStatsAddOpponentTurnCount(builder, stats.OpponentTurnCount)
```

**Step 3: Rebuild CGo library**

```bash
make build-cgo
```

**Step 4: Commit**

```bash
git add src/gosim/cgo/bridge.go
git commit -m "feat(cgo): serialize solitaire detection metrics"
```

---

## Task 11: Update Python SimulationResults

**Files:**
- Modify: `src/darwindeck/evolution/fitness_full.py`

**Step 1: Add new fields to SimulationResults dataclass**

Find `class SimulationResults` (around line 72) and add after tension fields:

```python
# Solitaire detection metrics
move_disruption_events: int = 0
contention_events: int = 0
forced_response_events: int = 0
opponent_turn_count: int = 0
```

**Step 2: Run Python tests**

```bash
uv run pytest tests/unit/test_fitness.py -v
```

**Step 3: Commit**

```bash
git add src/darwindeck/evolution/fitness_full.py
git commit -m "feat(python): add solitaire detection fields to SimulationResults"
```

---

## Task 12: Update Python Go Simulator

**Files:**
- Modify: `src/darwindeck/simulation/go_simulator.py`

**Step 1: Add parsing for new fields**

Find where `SimulationResults` is constructed (around line 135). Add after tension metrics:

```python
# Solitaire detection metrics
move_disruption_events=result.MoveDisruptionEvents(),
contention_events=result.ContentionEvents(),
forced_response_events=result.ForcedResponseEvents(),
opponent_turn_count=result.OpponentTurnCount(),
```

**Step 2: Also update simulate_asymmetric method**

Find the similar construction in `simulate_asymmetric` and add the same fields.

**Step 3: Run Python tests**

```bash
uv run pytest tests/integration/test_go_simulator.py -v
```

**Step 4: Commit**

```bash
git add src/darwindeck/simulation/go_simulator.py
git commit -m "feat(python): parse solitaire detection metrics from Go"
```

---

## Task 13: Update Fitness Calculation

**Files:**
- Modify: `src/darwindeck/evolution/fitness_full.py`

**Step 1: Replace interaction_frequency calculation**

Find the interaction_frequency calculation (around line 443). Replace with:

```python
# 4. Interaction frequency - improved multi-signal approach
if results.opponent_turn_count > 0:
    # Three signals, each 0.0-1.0
    move_disruption = min(1.0, results.move_disruption_events / results.opponent_turn_count)
    forced_response = min(1.0, results.forced_response_events / results.opponent_turn_count)

    # Contention uses total_actions as denominator
    if results.total_actions > 0:
        contention = min(1.0, results.contention_events / results.total_actions)
    else:
        contention = 0.0

    # Average the three signals
    interaction_frequency = (move_disruption + contention + forced_response) / 3.0
elif hasattr(results, 'total_actions') and results.total_actions > 0:
    # Fallback to old metric if new fields not available
    interaction_ratio = results.total_interactions / results.total_actions
    interaction_frequency = min(1.0, interaction_ratio)
else:
    # Final fallback to heuristic
    special_effects_score = min(1.0, len(genome.special_effects) / 3.0)
    trick_based_score = 0.3 if genome.turn_structure.is_trick_based else 0.0
    multi_phase_score = min(0.4, len(genome.turn_structure.phases) / 10.0)
    interaction_frequency = min(1.0,
        special_effects_score * 0.4 +
        trick_based_score +
        multi_phase_score
    )
```

**Step 2: Run Python tests**

```bash
uv run pytest tests/unit/test_fitness.py -v
```

**Step 3: Commit**

```bash
git add src/darwindeck/evolution/fitness_full.py
git commit -m "feat(fitness): use multi-signal solitaire detection for interaction_frequency"
```

---

## Task 14: Test With Seed Games

**Files:**
- Create: `tests/integration/test_solitaire_detection.py`

**Step 1: Write integration test**

```python
"""Test solitaire detection metrics with seed games."""
import pytest
from darwindeck.genome.serialization import genome_from_dict
from darwindeck.simulation.go_simulator import GoSimulator
from darwindeck.evolution.fitness_full import FitnessEvaluator
import json
from pathlib import Path


class TestSolitaireDetection:
    """Test that solitaire detection produces expected results for known games."""

    @pytest.fixture
    def simulator(self):
        return GoSimulator(seed=42)

    @pytest.fixture
    def evaluator(self):
        return FitnessEvaluator(style='balanced')

    def test_war_has_high_contention(self, simulator):
        """War should have high contention (shared tableau captures)."""
        war_path = Path("seeds/shedding/war.json")
        with open(war_path) as f:
            genome = genome_from_dict(json.load(f))

        results = simulator.simulate(genome, num_games=50)

        # War has shared tableau - should see contention
        assert results.opponent_turn_count > 0
        contention_rate = results.contention_events / max(1, results.total_actions)
        assert contention_rate > 0.1, f"War should have contention, got {contention_rate}"

    def test_hearts_has_high_disruption(self, simulator):
        """Hearts (trick-taking) should have high move disruption."""
        hearts_path = Path("seeds/trick_taking/hearts.json")
        with open(hearts_path) as f:
            genome = genome_from_dict(json.load(f))

        results = simulator.simulate(genome, num_games=50, player_count=4)

        # Trick-taking changes what cards others can play
        if results.opponent_turn_count > 0:
            disruption_rate = results.move_disruption_events / results.opponent_turn_count
            assert disruption_rate > 0.3, f"Hearts should have disruption, got {disruption_rate}"

    def test_interaction_frequency_reasonable(self, simulator, evaluator):
        """Test that interaction_frequency produces reasonable scores."""
        war_path = Path("seeds/shedding/war.json")
        with open(war_path) as f:
            genome = genome_from_dict(json.load(f))

        results = simulator.simulate(genome, num_games=100)
        metrics = evaluator.evaluate(genome, results)

        # War is interactive (shared tableau), should score reasonably
        assert 0.1 < metrics.interaction_frequency < 0.8
```

**Step 2: Run the test**

```bash
uv run pytest tests/integration/test_solitaire_detection.py -v
```

Expected: Tests pass (may need adjustment based on actual values)

**Step 3: Commit**

```bash
git add tests/integration/test_solitaire_detection.py
git commit -m "test: add solitaire detection integration tests"
```

---

## Task 15: Run Evolution and Validate

**Step 1: Run short evolution**

```bash
uv run python -m darwindeck.cli.evolve \
    --population-size 30 \
    --generations 10 \
    --output-dir output/test-interaction-metrics \
    --style balanced
```

**Step 2: Check evolved games have reasonable interaction scores**

```bash
uv run python -c "
import json
from pathlib import Path

output_dir = Path('output/test-interaction-metrics')
latest = sorted(output_dir.iterdir())[-1]

for genome_file in sorted(latest.glob('rank*.json'))[:5]:
    with open(genome_file) as f:
        data = json.load(f)
    interaction = data.get('fitness_metrics', {}).get('interaction_frequency', 'N/A')
    print(f'{genome_file.name}: interaction_frequency={interaction}')
"
```

**Step 3: Final commit**

```bash
git add -A
git commit -m "feat: complete solitaire detection implementation

Multi-signal interaction metrics that detect parallel solitaire:
- Move disruption: opponent turns that change your options
- Resource contention: competing for same cards/positions
- Forced response: moves significantly constrained by opponent

Replaces crude interaction_frequency with average of three signals."
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add fields to GameMetrics | runner.go |
| 2 | Add fields to AggregatedStats | runner.go |
| 3 | Implement move disruption tracking | runner.go |
| 4 | Implement contention tracking | runner.go |
| 5 | Implement forced response tracking | runner.go |
| 6 | Aggregate metrics | runner.go |
| 7 | Update FlatBuffers schema | simulation.fbs |
| 8 | Regenerate bindings | bindings/ |
| 9 | Update CGo struct | bridge.go |
| 10 | Update CGo serialization | bridge.go |
| 11 | Update SimulationResults | fitness_full.py |
| 12 | Update go_simulator.py | go_simulator.py |
| 13 | Update fitness calculation | fitness_full.py |
| 14 | Integration tests | test_solitaire_detection.py |
| 15 | Evolution validation | (manual) |

**Total: 15 tasks**
