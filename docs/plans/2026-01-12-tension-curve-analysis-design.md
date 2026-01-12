# Tension Curve Analysis Design

**Status:** Ready for Implementation (Revised after multi-agent review)

**Goal:** Replace the heuristic-based tension_curve metric with real game state tracking to measure dramatic tension throughout games.

---

## Overview

Currently `tension_curve` is computed from game length alone:
```python
turn_score = min(1.0, results.avg_turns / 100.0)
length_bonus = min(1.0, max(0.0, (results.avg_turns - 20) / 50.0))
tension_curve = min(1.0, turn_score * 0.6 + length_bonus * 0.4)
```

This doesn't measure actual tension. We'll track who's "ahead" throughout the game and compute aggregate metrics.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Metric scope | Game-type-specific (score, hand size, tricks) | More accurate than unified metric |
| Sampling | Aggregate only (no per-turn storage) | Minimal serialization overhead |
| Tie handling | No leader during ties | Cleaner semantics |
| Trick-taking | Track per round, with avoidance support | Hearts needs inverted logic |

## Metrics

| Metric | Description | Good Value |
|--------|-------------|------------|
| `LeadChanges` | Times the leader switched | More = more tension |
| `DecisiveTurnPct` | Turn when winner took **permanent** lead (as % of game) | Later = more tension |
| `ClosestMargin` | Smallest normalized gap between 1st and 2nd | Smaller = more tension |

## Formula

```
lead_change_score = min(1.0, lead_changes / expected_changes)
decisive_turn_score = decisive_turn_pct  # 0.0-1.0
margin_score = 1.0 - closest_margin

tension_curve = (
    lead_change_score * 0.4 +
    decisive_turn_score * 0.4 +
    margin_score * 0.2
)
```

Where `expected_changes = max(1, avg_turns / turns_per_expected_change)` scales expectations by game length.
Default `turns_per_expected_change = 20`, configurable per game type.

---

## Go Implementation

### Data Structures

```go
// engine/tension.go

// TensionMetrics tracks tension curve data during simulation
type TensionMetrics struct {
    LeadChanges     int     // Number of times leader switched
    DecisiveTurn    int     // Turn when winner took PERMANENT lead
    ClosestMargin   float32 // Smallest normalized gap between 1st and 2nd (0 = tied)
    TotalTurns      int     // For computing decisive turn percentage

    // Internal tracking (not serialized)
    currentLeader   int     // Player ID of current leader (-1 for tie)
    leaderHistory   []int   // Leader at each turn (for permanent lead calculation)
}

// LeaderDetector interface for game-type-specific leader detection
type LeaderDetector interface {
    GetLeader(state *GameState) int     // Returns player ID or -1 for tie
    GetMargin(state *GameState) float32 // Normalized gap (0-1), 0 = tied, 1 = max gap
}
```

### Leader Detectors

```go
// ScoreLeaderDetector - for score-based games (Gin Rummy, Scopa)
// Higher score = winning
type ScoreLeaderDetector struct{}

func (d *ScoreLeaderDetector) GetLeader(state *GameState) int {
    if len(state.Players) < 2 {
        return -1
    }
    maxScore := state.Players[0].Score
    leader := 0
    tied := false
    for i := 1; i < len(state.Players); i++ {
        if state.Players[i].Score > maxScore {
            maxScore = state.Players[i].Score
            leader = i
            tied = false
        } else if state.Players[i].Score == maxScore {
            tied = true
        }
    }
    if tied {
        return -1
    }
    return leader
}

func (d *ScoreLeaderDetector) GetMargin(state *GameState) float32 {
    if len(state.Players) < 2 {
        return 0
    }
    // Find max and second max scores
    first, second := 0, 0
    for _, p := range state.Players {
        if p.Score > first {
            second = first
            first = p.Score
        } else if p.Score > second {
            second = p.Score
        }
    }
    // Normalize by max possible gap (use first as denominator)
    if first == 0 {
        return 0
    }
    return float32(first-second) / float32(first)
}

// HandSizeLeaderDetector - for shedding games (Crazy 8s, President)
// Fewer cards = winning
type HandSizeLeaderDetector struct{}

func (d *HandSizeLeaderDetector) GetLeader(state *GameState) int {
    if len(state.Players) < 2 {
        return -1
    }
    minCards := len(state.Players[0].Hand)
    leader := 0
    tied := false
    for i := 1; i < len(state.Players); i++ {
        cards := len(state.Players[i].Hand)
        if cards < minCards {
            minCards = cards
            leader = i
            tied = false
        } else if cards == minCards {
            tied = true
        }
    }
    if tied {
        return -1
    }
    return leader
}

func (d *HandSizeLeaderDetector) GetMargin(state *GameState) float32 {
    if len(state.Players) < 2 {
        return 0
    }
    // Find min and second min hand sizes
    first, second := 999, 999
    maxCards := 0
    for _, p := range state.Players {
        cards := len(p.Hand)
        if cards > maxCards {
            maxCards = cards
        }
        if cards < first {
            second = first
            first = cards
        } else if cards < second {
            second = cards
        }
    }
    // Normalize by max hand size (gap relative to total cards)
    if maxCards == 0 || second == 999 {
        return 0
    }
    return float32(second-first) / float32(maxCards)
}

// TrickLeaderDetector - for trick-COLLECTING games (Spades, Whist)
// More tricks = winning
type TrickLeaderDetector struct{}

func (d *TrickLeaderDetector) GetLeader(state *GameState) int {
    if len(state.Players) < 2 {
        return -1
    }
    maxTricks := state.Players[0].TricksWon
    leader := 0
    tied := false
    for i := 1; i < len(state.Players); i++ {
        if state.Players[i].TricksWon > maxTricks {
            maxTricks = state.Players[i].TricksWon
            leader = i
            tied = false
        } else if state.Players[i].TricksWon == maxTricks {
            tied = true
        }
    }
    if tied {
        return -1
    }
    return leader
}

func (d *TrickLeaderDetector) GetMargin(state *GameState) float32 {
    if len(state.Players) < 2 {
        return 0
    }
    // Find max and second max tricks
    first, second := 0, 0
    totalTricks := 0
    for _, p := range state.Players {
        totalTricks += p.TricksWon
        if p.TricksWon > first {
            second = first
            first = p.TricksWon
        } else if p.TricksWon > second {
            second = p.TricksWon
        }
    }
    // Normalize by total tricks in round
    if totalTricks == 0 {
        return 0
    }
    return float32(first-second) / float32(totalTricks)
}

// TrickAvoidanceLeaderDetector - for trick-AVOIDANCE games (Hearts)
// Fewer tricks = winning
type TrickAvoidanceLeaderDetector struct{}

func (d *TrickAvoidanceLeaderDetector) GetLeader(state *GameState) int {
    if len(state.Players) < 2 {
        return -1
    }
    minTricks := state.Players[0].TricksWon
    leader := 0
    tied := false
    for i := 1; i < len(state.Players); i++ {
        if state.Players[i].TricksWon < minTricks {
            minTricks = state.Players[i].TricksWon
            leader = i
            tied = false
        } else if state.Players[i].TricksWon == minTricks {
            tied = true
        }
    }
    if tied {
        return -1
    }
    return leader
}

func (d *TrickAvoidanceLeaderDetector) GetMargin(state *GameState) float32 {
    if len(state.Players) < 2 {
        return 0
    }
    // Find min and second min tricks (leader has fewest)
    first, second := 999, 999
    totalTricks := 0
    for _, p := range state.Players {
        totalTricks += p.TricksWon
        if p.TricksWon < first {
            second = first
            first = p.TricksWon
        } else if p.TricksWon < second {
            second = p.TricksWon
        }
    }
    // Normalize by total tricks
    if totalTricks == 0 || second == 999 {
        return 0
    }
    return float32(second-first) / float32(totalTricks)
}

// ChipLeaderDetector - for betting games (Poker variants)
// More chips = winning
type ChipLeaderDetector struct{}

func (d *ChipLeaderDetector) GetLeader(state *GameState) int {
    if len(state.Players) < 2 {
        return -1
    }
    maxChips := state.Players[0].Chips
    leader := 0
    tied := false
    for i := 1; i < len(state.Players); i++ {
        if state.Players[i].Chips > maxChips {
            maxChips = state.Players[i].Chips
            leader = i
            tied = false
        } else if state.Players[i].Chips == maxChips {
            tied = true
        }
    }
    if tied {
        return -1
    }
    return leader
}

func (d *ChipLeaderDetector) GetMargin(state *GameState) float32 {
    if len(state.Players) < 2 {
        return 0
    }
    // Find max and second max chips
    first, second := 0, 0
    totalChips := 0
    for _, p := range state.Players {
        totalChips += p.Chips
        if p.Chips > first {
            second = first
            first = p.Chips
        } else if p.Chips > second {
            second = p.Chips
        }
    }
    // Normalize by total chips in play
    if totalChips == 0 {
        return 0
    }
    return float32(first-second) / float32(totalChips)
}
```

### Detector Selection

```go
// WinType constants (should match bytecode.go)
const (
    WinTypeEmptyHand   = 0
    WinTypeHighScore   = 1
    WinTypeLowScore    = 2  // For avoidance games like Hearts
    WinTypeMostTricks  = 3
    WinTypeFewestTricks = 4 // Hearts-style
    WinTypeMostChips   = 5
)

// SelectLeaderDetector chooses detector based on genome's win conditions
func SelectLeaderDetector(genome *Genome) LeaderDetector {
    // Check win conditions first - most reliable indicator
    for _, wc := range genome.WinConditions {
        switch wc.WinType {
        case WinTypeEmptyHand:
            return &HandSizeLeaderDetector{}
        case WinTypeHighScore:
            return &ScoreLeaderDetector{}
        case WinTypeLowScore, WinTypeFewestTricks:
            // Avoidance games - fewer is better
            return &TrickAvoidanceLeaderDetector{}
        case WinTypeMostTricks:
            return &TrickLeaderDetector{}
        case WinTypeMostChips:
            return &ChipLeaderDetector{}
        }
    }

    // Check for betting games (have starting chips)
    hasBetting := false
    for _, phase := range genome.TurnPhases {
        if phase.PhaseType == PhaseTypeBetting {
            hasBetting = true
            break
        }
    }
    if hasBetting {
        return &ChipLeaderDetector{}
    }

    // Check phases for trick-taking hints
    for _, phase := range genome.TurnPhases {
        if phase.PhaseType == PhaseTypeTrick {
            // Default to collecting (most common)
            // Hearts would have WinTypeLowScore/WinTypeFewestTricks
            return &TrickLeaderDetector{}
        }
    }

    // Default to score-based
    return &ScoreLeaderDetector{}
}
```

### Update Logic

```go
// NewTensionMetrics creates initialized tension tracker
func NewTensionMetrics(numPlayers int) *TensionMetrics {
    return &TensionMetrics{
        currentLeader: -1,
        ClosestMargin: 1.0, // Start at max gap (will track minimum)
        leaderHistory: make([]int, 0, 100), // Pre-allocate for typical game
    }
}

// Update called after each turn in the game loop
func (tm *TensionMetrics) Update(state *GameState, detector LeaderDetector) {
    newLeader := detector.GetLeader(state)
    margin := detector.GetMargin(state)

    // Track closest margin seen (smaller = more tension)
    if margin < tm.ClosestMargin {
        tm.ClosestMargin = margin
    }

    // Track lead changes (ignore ties)
    if newLeader != -1 && tm.currentLeader != -1 && newLeader != tm.currentLeader {
        tm.LeadChanges++
    }

    // Update current leader
    if newLeader != -1 {
        tm.currentLeader = newLeader
    }

    // Record leader for permanent lead calculation
    tm.leaderHistory = append(tm.leaderHistory, tm.currentLeader)
    tm.TotalTurns++
}

// Finalize computes DecisiveTurn based on winner
// DecisiveTurn = first turn where winner took lead and NEVER lost it
func (tm *TensionMetrics) Finalize(winnerID int) {
    // Handle invalid winner (draw or error)
    if winnerID < 0 {
        // No clear winner - set decisive turn to end (maximum tension)
        tm.DecisiveTurn = tm.TotalTurns
        return
    }

    // Scan backwards to find when winner took permanent lead
    // Start from end and find first turn where leader wasn't the winner
    tm.DecisiveTurn = tm.TotalTurns // Default: winner led from the end

    for i := len(tm.leaderHistory) - 1; i >= 0; i-- {
        if tm.leaderHistory[i] != winnerID && tm.leaderHistory[i] != -1 {
            // Found a turn where someone else was leading
            // Decisive turn is the next turn (when winner took back lead)
            if i+1 < len(tm.leaderHistory) {
                tm.DecisiveTurn = i + 1
            }
            break
        }
        // Winner was leading (or tied) at turn i
        tm.DecisiveTurn = i
    }
}

// DecisiveTurnPct returns decisive turn as percentage of game
func (tm *TensionMetrics) DecisiveTurnPct() float32 {
    if tm.TotalTurns == 0 {
        return 0
    }
    return float32(tm.DecisiveTurn) / float32(tm.TotalTurns)
}

// Reset clears tension metrics for reuse (e.g., between hands in multi-hand games)
func (tm *TensionMetrics) Reset(numPlayers int) {
    tm.LeadChanges = 0
    tm.DecisiveTurn = 0
    tm.ClosestMargin = 1.0
    tm.TotalTurns = 0
    tm.currentLeader = -1
    tm.leaderHistory = tm.leaderHistory[:0] // Reuse backing array
}
```

### Runner Integration

```go
// In RunSingleGame, after game setup:
detector := SelectLeaderDetector(genome)
tensionMetrics := NewTensionMetrics(int(state.NumPlayers))

// In the main game loop, after each move is applied:
tensionMetrics.Update(state, detector)

// At game end (handle draws):
if winnerID >= 0 {
    tensionMetrics.Finalize(winnerID)
} else {
    // Draw - set decisive turn to end (game was contested until the end)
    tensionMetrics.Finalize(-1)
}

// Add to GameMetrics (returned to Python):
metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
metrics.ClosestMargin = tensionMetrics.ClosestMargin
```

---

## FlatBuffers Schema

Add to `schema/simulation.fbs`:

```fbs
table AggregatedStats {
    // ... existing fields ...

    // Tension curve metrics
    lead_changes: uint32 = 0;
    decisive_turn_pct: float32 = 1.0;  // Default to 1.0 (decided at end) for backward compat
    closest_margin: float32 = 1.0;     // Default to 1.0 (max gap) for backward compat
}
```

Regenerate with: `flatc --go --python schema/simulation.fbs`

---

## CGo Bridge

Update `SimStats` struct in `bridge.go`:

```go
type SimStats struct {
    // ... existing fields ...

    // Tension metrics
    LeadChanges      uint32
    DecisiveTurnPct  float32
    ClosestMargin    float32
}
```

---

## Python Fitness

Update `SimulationResults` in `fitness_full.py`:

```python
@dataclass(frozen=True)
class SimulationResults:
    # ... existing fields ...

    # Tension curve metrics
    lead_changes: int = 0
    decisive_turn_pct: float = 1.0   # Default: decided at end
    closest_margin: float = 1.0      # Default: max gap (no tension data)
```

Update `_compute_metrics()`:

```python
# 3. Tension curve - use real instrumentation if available
# Check if we have real tension data (not just defaults)
has_tension_data = (
    results.lead_changes > 0 or
    results.decisive_turn_pct < 1.0 or
    results.closest_margin < 1.0
)

if has_tension_data:
    # Real tension data from Go simulation
    # Scale expected lead changes by game length
    turns_per_expected_change = 20  # Could be configurable per game type
    expected_changes = max(1, results.avg_turns / turns_per_expected_change)
    lead_change_score = min(1.0, results.lead_changes / expected_changes)

    # Decisive turn: later is better (more suspense)
    decisive_turn_score = results.decisive_turn_pct

    # Margin: tighter is better (0 = tied, 1 = max gap)
    margin_score = 1.0 - results.closest_margin

    tension_curve = (
        lead_change_score * 0.4 +
        decisive_turn_score * 0.4 +
        margin_score * 0.2
    )
else:
    # Fallback to heuristic (backward compatibility)
    turn_score = min(1.0, results.avg_turns / 100.0)
    length_bonus = min(1.0, max(0.0, (results.avg_turns - 20) / 50.0))
    tension_curve = min(1.0, turn_score * 0.6 + length_bonus * 0.4)
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `src/gosim/engine/tension.go` | New file: structs, 5 detectors, update logic |
| `src/gosim/engine/bytecode.go` | Add WinType constants if not present |
| `src/gosim/simulation/runner.go` | Integrate tension tracking in game loop |
| `schema/simulation.fbs` | Add 3 tension fields with defaults |
| `src/gosim/cgo/bridge.go` | Add tension fields to SimStats |
| `src/darwindeck/evolution/fitness_full.py` | Update SimulationResults and calculation |
| `src/darwindeck/bindings/` | Regenerate FlatBuffers bindings |

---

## Testing Strategy

### Go Unit Tests

```go
// Leader detection tests
func TestScoreLeaderDetector(t *testing.T)           // Higher score = leader
func TestScoreLeaderDetector_Tie(t *testing.T)       // Returns -1 on tie
func TestHandSizeLeaderDetector(t *testing.T)        // Fewer cards = leader
func TestTrickLeaderDetector(t *testing.T)           // More tricks = leader
func TestTrickAvoidanceLeaderDetector(t *testing.T)  // Fewer tricks = leader (Hearts)
func TestChipLeaderDetector(t *testing.T)            // More chips = leader

// Margin calculation tests
func TestScoreMargin_Normalized(t *testing.T)        // Verify 0-1 range
func TestHandSizeMargin_Normalized(t *testing.T)     // Verify 0-1 range
func TestMargin_ZeroWhenTied(t *testing.T)           // Margin = 0 on tie

// Tension metrics tests
func TestTensionMetrics_LeadChanges(t *testing.T)    // Count switches correctly
func TestTensionMetrics_IgnoresTies(t *testing.T)    // Tie doesn't count as change
func TestDecisiveTurn_PermanentLead(t *testing.T)    // Finds PERMANENT lead, not last
func TestDecisiveTurn_DrawGame(t *testing.T)         // Handles winnerID = -1
func TestDecisiveTurn_NeverLed(t *testing.T)         // Winner never led until end

// Detector selection tests
func TestSelectLeaderDetector_EmptyHand(t *testing.T)      // Returns HandSizeLeaderDetector
func TestSelectLeaderDetector_LowScore(t *testing.T)       // Returns TrickAvoidanceLeaderDetector
func TestSelectLeaderDetector_BettingGame(t *testing.T)    // Returns ChipLeaderDetector
func TestSelectLeaderDetector_TrickPhase(t *testing.T)     // Returns TrickLeaderDetector
```

### Integration Tests

```python
def test_tension_metrics_in_simulation():
    """Tension metrics flow from Go to Python."""

def test_tension_curve_uses_real_data():
    """Fitness uses instrumented data when available."""

def test_tension_curve_fallback():
    """Falls back to heuristic when no tension data."""

def test_hearts_uses_avoidance_detector():
    """Hearts genome gets TrickAvoidanceLeaderDetector."""

def test_poker_uses_chip_detector():
    """Poker genome gets ChipLeaderDetector."""
```

---

## Issues Fixed (from Multi-Agent Review)

| Issue | Severity | Fix |
|-------|----------|-----|
| Hearts/avoidance semantics wrong | STRONG | Added `TrickAvoidanceLeaderDetector` and `WinTypeFewestTricks`/`WinTypeLowScore` |
| DecisiveTurn tracks last lead, not permanent | STRONG | Scan backwards through `leaderHistory` to find permanent lead |
| No handling for draws | STRONG | `Finalize(-1)` sets DecisiveTurn to TotalTurns (maximum tension) |
| Inconsistent margin normalization | MODERATE | All detectors normalize to 0-1 using total (chips/tricks) or max (score/cards) |
| Hardcoded magic number 20 | MODERATE | Made `turns_per_expected_change` a named constant, noted as configurable |
| FlatBuffers defaults | WEAK | Added explicit defaults matching "no tension data" state |

---

## Success Criteria

1. Tension metrics tracked correctly for all game types (including avoidance)
2. DecisiveTurn correctly identifies when winner took **permanent** lead
3. Draws handled gracefully (winnerID = -1)
4. Data flows through CGo bridge without errors
5. Fitness calculation uses real data when available
6. Backward compatible with simulations without tension data
7. No measurable performance regression (aggregate-only approach)
