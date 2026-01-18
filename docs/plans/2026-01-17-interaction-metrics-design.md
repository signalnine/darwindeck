# Interaction Metrics Design: Solitaire Detection

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the crude `interaction_frequency` metric with a multi-signal approach that detects "parallel solitaire" patterns where players don't meaningfully affect each other.

**Architecture:** Track three signals during Go simulation (move disruption, shared resource contention, forced response), aggregate them, pass through FlatBuffers to Python, and average them to produce an improved `interaction_frequency` score.

**Tech Stack:** Go (simulation), FlatBuffers (serialization), Python (fitness calculation)

---

## Problem Statement

Current `interaction_frequency` is too crude - it counts ANY interaction (playing to tableau, drawing from opponent) without distinguishing meaningful interactions from incidental ones. This allows "parallel solitaire" games to score well even though players aren't really affecting each other.

**Solitaire patterns to detect:**
1. **Low state disruption** - Player A's actions rarely change what Player B can do
2. **Independent progression** - Each player's progress unaffected by opponent
3. **No reactive play** - Optimal strategy doesn't depend on opponent actions

---

## Data Flow

```
Go Simulator → GameMetrics (+ 3 new counters) → AggregatedStats → FlatBuffers → Python
                                                                                   ↓
                                                              improved interaction_frequency
                                                              (average of 3 signals)
```

**New fields tracked in Go:**

| Field | Type | Description |
|-------|------|-------------|
| `MoveDisruptionEvents` | uint32 | Turns where opponent changed your legal moves |
| `ContentionEvents` | uint32 | Times players competed for same resource |
| `ForcedResponseEvents` | uint32 | Turns where legal moves significantly constrained |
| `OpponentTurnCount` | uint32 | Total opponent turns (shared denominator) |

---

## Detection Logic

### 1. Move Disruption Detection

Tracks when opponent's turn changes the waiting player's available moves.

```go
// Before opponent's turn ends, snapshot waiting player's legal moves
beforeMoves := engine.GenerateLegalMoves(state, genome, waitingPlayer)

// After opponent's turn
afterMoves := engine.GenerateLegalMoves(state, genome, waitingPlayer)

// If move count or move set changed, count as disruption
if !sameMoveSet(beforeMoves, afterMoves) {
    metrics.MoveDisruptionEvents++
}
```

### 2. Shared Resource Contention

Detects when players compete for the same resources.

| Situation | Detection |
|-----------|-----------|
| Tableau capture race | Player captures card that opponent could also capture |
| Deck scarcity | Draw from deck when ≤10 cards remain |
| Blocking play | Play to position that blocks opponent's valid target |

```go
// On tableau capture: check if opponent also had a matching card
if move.Target == TABLEAU && isCapture {
    if opponentCouldCapture(state, capturedCard, opponent) {
        metrics.ContentionEvents++
    }
}

// On deck draw when scarce
if move.Source == DECK && len(state.Deck) <= 10 {
    metrics.ContentionEvents++
}
```

### 3. Forced Response Detection

Tracks when opponent's action significantly constrains your options.

```go
// Compare legal move counts before/after opponent turn
beforeCount := len(beforeMoves)
afterCount := len(afterMoves)

// Significant constraint: moves dropped by >30%
if afterCount < beforeCount && float64(afterCount)/float64(beforeCount) < 0.7 {
    metrics.ForcedResponseEvents++
}
```

---

## Schema Changes

### FlatBuffers (`schema/simulation.fbs`)

```fbs
table AggregatedStats {
    // ... existing fields ...

    // New interaction metrics (solitaire detection)
    move_disruption_events: uint32;
    contention_events: uint32;
    forced_response_events: uint32;
    opponent_turn_count: uint32;
}
```

### Python (`fitness_full.py`)

```python
@dataclass(frozen=True)
class SimulationResults:
    # ... existing fields ...

    # Solitaire detection signals
    move_disruption_events: int = 0
    contention_events: int = 0
    forced_response_events: int = 0
    opponent_turn_count: int = 0
```

---

## Fitness Calculation

```python
def calculate_interaction_frequency(results: SimulationResults) -> float:
    # Avoid division by zero
    if results.opponent_turn_count == 0:
        return 0.0

    # Three signals, each 0.0-1.0
    move_disruption = min(1.0, results.move_disruption_events / results.opponent_turn_count)
    forced_response = min(1.0, results.forced_response_events / results.opponent_turn_count)

    # Contention uses total_actions as denominator
    if results.total_actions > 0:
        contention = min(1.0, results.contention_events / results.total_actions)
    else:
        contention = 0.0

    # Average the three signals
    return (move_disruption + contention + forced_response) / 3.0
```

**Interpretation:**
- < 0.15: Parallel solitaire (players don't affect each other)
- 0.15-0.35: Low interaction (some shared resources)
- 0.35-0.55: Moderate interaction (typical card game)
- > 0.55: High interaction (heavy player-vs-player)

---

## Edge Cases

| Case | Handling |
|------|----------|
| Single-player games | Return 0.0 (no opponent) |
| Games ending turn 1 | Fallback if `opponent_turn_count == 0` |
| No legal moves (stuck) | Skip tracking that turn |
| Infinite move sets | Cap move comparison at first 50 moves |

**Backward Compatibility:**
- New fields default to 0 in FlatBuffers
- If new fields are 0 but `total_actions > 0`, fall back to current heuristic
- Existing genomes continue working with better scoring

---

## Testing Strategy

### Expected Results by Game Type

| Game | Move Disruption | Contention | Forced Response | Expected Score |
|------|-----------------|------------|-----------------|----------------|
| War | Low | High | Low | ~0.3-0.4 |
| Parallel solitaire | Low | Low | Low | < 0.15 |
| Hearts | High | Medium | High | > 0.5 |
| Uno | High | Medium | Medium | > 0.4 |

### Validation Steps

1. Run seed games through new metrics
2. Verify known-interactive games (Hearts, Uno) score high
3. Verify known-parallel games score low
4. Run evolution and check games become more interactive over generations

---

## Implementation Tasks

1. **Go: Add metric fields** - Add new fields to `GameMetrics` struct
2. **Go: Implement move disruption tracking** - Snapshot and compare legal moves
3. **Go: Implement contention tracking** - Detect shared resource competition
4. **Go: Implement forced response tracking** - Detect significant move constraints
5. **Go: Aggregate metrics** - Sum across games in `AggregatedStats`
6. **FlatBuffers: Update schema** - Add new fields to `AggregatedStats` table
7. **FlatBuffers: Regenerate bindings** - Run flatc for Go and Python
8. **Python: Update SimulationResults** - Add new fields
9. **Python: Update CGo bridge** - Parse new fields from FlatBuffers
10. **Python: Update fitness calculation** - Use new signals for `interaction_frequency`
11. **Testing: Validate seed games** - Check expected scores
12. **Testing: Run evolution** - Verify improvement over generations

---

## Files to Modify

| File | Changes |
|------|---------|
| `src/gosim/simulation/runner.go` | Add tracking logic, new metric fields |
| `src/gosim/engine/movegen.go` | May need helper for move set comparison |
| `schema/simulation.fbs` | Add 4 new fields to AggregatedStats |
| `src/gosim/bindings/cardsim/` | Regenerated from flatc |
| `src/darwindeck/bindings/` | Regenerated from flatc |
| `src/gosim/cgo/bridge.go` | Pass new fields through CGo |
| `src/darwindeck/evolution/fitness_full.py` | Update SimulationResults, calculation |
| `src/darwindeck/simulation/go_simulator.py` | Parse new fields |
