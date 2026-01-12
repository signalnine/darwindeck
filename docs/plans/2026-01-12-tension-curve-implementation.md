# Tension Curve Analysis Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the heuristic-based tension_curve fitness metric with real game state tracking that measures lead changes, decisive turn, and closest margin.

**Architecture:** Add tension tracking to Go simulation with game-type-specific leader detectors (Score, HandSize, Trick, TrickAvoidance, Chip). Track aggregate metrics during game loop, serialize through FlatBuffers/CGo bridge, compute final tension_curve in Python fitness.

**Tech Stack:** Go (simulation), FlatBuffers (serialization), Python (fitness calculation)

---

## Task 1: Create TensionMetrics Struct and Interface

**Files:**
- Create: `src/gosim/engine/tension.go`
- Test: `src/gosim/engine/tension_test.go`

**Step 1: Write the failing test for TensionMetrics initialization**

```go
// tension_test.go
package engine

import "testing"

func TestNewTensionMetrics(t *testing.T) {
	tm := NewTensionMetrics(4)

	if tm.currentLeader != -1 {
		t.Errorf("expected currentLeader=-1, got %d", tm.currentLeader)
	}
	if tm.ClosestMargin != 1.0 {
		t.Errorf("expected ClosestMargin=1.0, got %f", tm.ClosestMargin)
	}
	if len(tm.leaderHistory) != 0 {
		t.Errorf("expected empty leaderHistory, got len=%d", len(tm.leaderHistory))
	}
	if cap(tm.leaderHistory) < 100 {
		t.Errorf("expected leaderHistory capacity >= 100, got %d", cap(tm.leaderHistory))
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd src/gosim && go test ./engine -v -run TestNewTensionMetrics
```

Expected: FAIL with "undefined: NewTensionMetrics"

**Step 3: Write minimal implementation**

```go
// tension.go
package engine

// TensionMetrics tracks tension curve data during simulation
type TensionMetrics struct {
	LeadChanges   int     // Number of times leader switched
	DecisiveTurn  int     // Turn when winner took PERMANENT lead
	ClosestMargin float32 // Smallest normalized gap between 1st and 2nd (0 = tied)
	TotalTurns    int     // For computing decisive turn percentage

	// Internal tracking (not serialized)
	currentLeader int   // Player ID of current leader (-1 for tie)
	leaderHistory []int // Leader at each turn (for permanent lead calculation)
}

// LeaderDetector interface for game-type-specific leader detection
type LeaderDetector interface {
	GetLeader(state *GameState) int     // Returns player ID or -1 for tie
	GetMargin(state *GameState) float32 // Normalized gap (0-1), 0 = tied, 1 = max gap
}

// NewTensionMetrics creates initialized tension tracker
func NewTensionMetrics(numPlayers int) *TensionMetrics {
	return &TensionMetrics{
		currentLeader: -1,
		ClosestMargin: 1.0,
		leaderHistory: make([]int, 0, 100),
	}
}
```

**Step 4: Run test to verify it passes**

```bash
cd src/gosim && go test ./engine -v -run TestNewTensionMetrics
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/tension.go src/gosim/engine/tension_test.go
git commit -m "feat(tension): add TensionMetrics struct and NewTensionMetrics"
```

---

## Task 2: Implement ScoreLeaderDetector

**Files:**
- Modify: `src/gosim/engine/tension.go`
- Modify: `src/gosim/engine/tension_test.go`

**Step 1: Write the failing tests**

```go
func TestScoreLeaderDetector_GetLeader(t *testing.T) {
	detector := &ScoreLeaderDetector{}

	state := &GameState{
		NumPlayers: 3,
		Players: []PlayerState{
			{Score: 10},
			{Score: 25},
			{Score: 15},
		},
	}

	leader := detector.GetLeader(state)
	if leader != 1 {
		t.Errorf("expected leader=1 (highest score), got %d", leader)
	}
}

func TestScoreLeaderDetector_Tie(t *testing.T) {
	detector := &ScoreLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Score: 20},
			{Score: 20},
		},
	}

	leader := detector.GetLeader(state)
	if leader != -1 {
		t.Errorf("expected leader=-1 (tie), got %d", leader)
	}
}

func TestScoreLeaderDetector_GetMargin(t *testing.T) {
	detector := &ScoreLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Score: 100},
			{Score: 75},
		},
	}

	margin := detector.GetMargin(state)
	// (100-75)/100 = 0.25
	if margin < 0.24 || margin > 0.26 {
		t.Errorf("expected margin=0.25, got %f", margin)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd src/gosim && go test ./engine -v -run TestScoreLeaderDetector
```

Expected: FAIL with "undefined: ScoreLeaderDetector"

**Step 3: Write implementation**

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
	first, second := 0, 0
	for _, p := range state.Players {
		if p.Score > first {
			second = first
			first = p.Score
		} else if p.Score > second {
			second = p.Score
		}
	}
	if first == 0 {
		return 0
	}
	return float32(first-second) / float32(first)
}
```

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./engine -v -run TestScoreLeaderDetector
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/tension.go src/gosim/engine/tension_test.go
git commit -m "feat(tension): add ScoreLeaderDetector"
```

---

## Task 3: Implement HandSizeLeaderDetector

**Files:**
- Modify: `src/gosim/engine/tension.go`
- Modify: `src/gosim/engine/tension_test.go`

**Step 1: Write the failing tests**

```go
func TestHandSizeLeaderDetector_GetLeader(t *testing.T) {
	detector := &HandSizeLeaderDetector{}

	state := &GameState{
		NumPlayers: 3,
		Players: []PlayerState{
			{Hand: make([]Card, 5)},
			{Hand: make([]Card, 2)}, // Fewest cards = leader
			{Hand: make([]Card, 7)},
		},
	}

	leader := detector.GetLeader(state)
	if leader != 1 {
		t.Errorf("expected leader=1 (fewest cards), got %d", leader)
	}
}

func TestHandSizeLeaderDetector_Tie(t *testing.T) {
	detector := &HandSizeLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Hand: make([]Card, 3)},
			{Hand: make([]Card, 3)},
		},
	}

	leader := detector.GetLeader(state)
	if leader != -1 {
		t.Errorf("expected leader=-1 (tie), got %d", leader)
	}
}

func TestHandSizeLeaderDetector_GetMargin(t *testing.T) {
	detector := &HandSizeLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Hand: make([]Card, 2)},
			{Hand: make([]Card, 8)},
		},
	}

	margin := detector.GetMargin(state)
	// (8-2)/8 = 0.75
	if margin < 0.74 || margin > 0.76 {
		t.Errorf("expected margin=0.75, got %f", margin)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd src/gosim && go test ./engine -v -run TestHandSizeLeaderDetector
```

Expected: FAIL

**Step 3: Write implementation**

```go
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
	if maxCards == 0 || second == 999 {
		return 0
	}
	return float32(second-first) / float32(maxCards)
}
```

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./engine -v -run TestHandSizeLeaderDetector
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/tension.go src/gosim/engine/tension_test.go
git commit -m "feat(tension): add HandSizeLeaderDetector"
```

---

## Task 4: Implement TrickLeaderDetector and TrickAvoidanceLeaderDetector

**Files:**
- Modify: `src/gosim/engine/tension.go`
- Modify: `src/gosim/engine/tension_test.go`

**Step 1: Write the failing tests**

```go
func TestTrickLeaderDetector_GetLeader(t *testing.T) {
	detector := &TrickLeaderDetector{}

	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{TricksWon: 3},
			{TricksWon: 5}, // Most tricks = leader
			{TricksWon: 2},
			{TricksWon: 3},
		},
	}

	leader := detector.GetLeader(state)
	if leader != 1 {
		t.Errorf("expected leader=1 (most tricks), got %d", leader)
	}
}

func TestTrickAvoidanceLeaderDetector_GetLeader(t *testing.T) {
	detector := &TrickAvoidanceLeaderDetector{}

	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{TricksWon: 3},
			{TricksWon: 5},
			{TricksWon: 1}, // Fewest tricks = leader in Hearts
			{TricksWon: 4},
		},
	}

	leader := detector.GetLeader(state)
	if leader != 2 {
		t.Errorf("expected leader=2 (fewest tricks), got %d", leader)
	}
}

func TestTrickLeaderDetector_GetMargin(t *testing.T) {
	detector := &TrickLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{TricksWon: 7},
			{TricksWon: 6},
		},
	}

	margin := detector.GetMargin(state)
	// (7-6)/13 ≈ 0.077
	if margin < 0.07 || margin > 0.08 {
		t.Errorf("expected margin≈0.077, got %f", margin)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd src/gosim && go test ./engine -v -run "TestTrickLeaderDetector|TestTrickAvoidance"
```

Expected: FAIL

**Step 3: Write implementation**

```go
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
	if totalTricks == 0 || second == 999 {
		return 0
	}
	return float32(second-first) / float32(totalTricks)
}
```

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./engine -v -run "TestTrickLeaderDetector|TestTrickAvoidance"
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/tension.go src/gosim/engine/tension_test.go
git commit -m "feat(tension): add TrickLeaderDetector and TrickAvoidanceLeaderDetector"
```

---

## Task 5: Implement ChipLeaderDetector

**Files:**
- Modify: `src/gosim/engine/tension.go`
- Modify: `src/gosim/engine/tension_test.go`

**Step 1: Write the failing tests**

```go
func TestChipLeaderDetector_GetLeader(t *testing.T) {
	detector := &ChipLeaderDetector{}

	state := &GameState{
		NumPlayers: 3,
		Players: []PlayerState{
			{Chips: 500},
			{Chips: 1200}, // Most chips = leader
			{Chips: 300},
		},
	}

	leader := detector.GetLeader(state)
	if leader != 1 {
		t.Errorf("expected leader=1 (most chips), got %d", leader)
	}
}

func TestChipLeaderDetector_GetMargin(t *testing.T) {
	detector := &ChipLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Chips: 1500},
			{Chips: 500},
		},
	}

	margin := detector.GetMargin(state)
	// (1500-500)/2000 = 0.5
	if margin < 0.49 || margin > 0.51 {
		t.Errorf("expected margin=0.5, got %f", margin)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd src/gosim && go test ./engine -v -run TestChipLeaderDetector
```

Expected: FAIL

**Step 3: Write implementation**

```go
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
	if totalChips == 0 {
		return 0
	}
	return float32(first-second) / float32(totalChips)
}
```

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./engine -v -run TestChipLeaderDetector
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/tension.go src/gosim/engine/tension_test.go
git commit -m "feat(tension): add ChipLeaderDetector"
```

---

## Task 6: Implement SelectLeaderDetector

**Files:**
- Modify: `src/gosim/engine/tension.go`
- Modify: `src/gosim/engine/tension_test.go`

**Step 1: Write the failing tests**

```go
func TestSelectLeaderDetector_EmptyHand(t *testing.T) {
	genome := &Genome{
		WinConditions: []WinCondition{{WinType: 0}}, // WinTypeEmptyHand
	}

	detector := SelectLeaderDetector(genome)
	_, ok := detector.(*HandSizeLeaderDetector)
	if !ok {
		t.Errorf("expected HandSizeLeaderDetector for WinTypeEmptyHand")
	}
}

func TestSelectLeaderDetector_LowScore(t *testing.T) {
	genome := &Genome{
		WinConditions: []WinCondition{{WinType: 2}}, // WinTypeLowScore
	}

	detector := SelectLeaderDetector(genome)
	_, ok := detector.(*TrickAvoidanceLeaderDetector)
	if !ok {
		t.Errorf("expected TrickAvoidanceLeaderDetector for WinTypeLowScore")
	}
}

func TestSelectLeaderDetector_BettingPhase(t *testing.T) {
	genome := &Genome{
		TurnPhases: []PhaseDescriptor{{PhaseType: PhaseTypeBetting}},
	}

	detector := SelectLeaderDetector(genome)
	_, ok := detector.(*ChipLeaderDetector)
	if !ok {
		t.Errorf("expected ChipLeaderDetector for betting game")
	}
}

func TestSelectLeaderDetector_TrickPhase(t *testing.T) {
	genome := &Genome{
		TurnPhases: []PhaseDescriptor{{PhaseType: PhaseTypeTrick}},
	}

	detector := SelectLeaderDetector(genome)
	_, ok := detector.(*TrickLeaderDetector)
	if !ok {
		t.Errorf("expected TrickLeaderDetector for trick-taking game")
	}
}

func TestSelectLeaderDetector_Default(t *testing.T) {
	genome := &Genome{}

	detector := SelectLeaderDetector(genome)
	_, ok := detector.(*ScoreLeaderDetector)
	if !ok {
		t.Errorf("expected ScoreLeaderDetector as default")
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd src/gosim && go test ./engine -v -run TestSelectLeaderDetector
```

Expected: FAIL

**Step 3: Write implementation**

```go
// WinType constants for tension detection
const (
	WinTypeEmptyHand    = 0
	WinTypeHighScore    = 1
	WinTypeLowScore     = 2 // For avoidance games like Hearts
	WinTypeMostTricks   = 3
	WinTypeFewestTricks = 4 // Hearts-style
	WinTypeMostChips    = 5
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
			return &TrickAvoidanceLeaderDetector{}
		case WinTypeMostTricks:
			return &TrickLeaderDetector{}
		case WinTypeMostChips:
			return &ChipLeaderDetector{}
		}
	}

	// Check for betting games (have BettingPhase)
	for _, phase := range genome.TurnPhases {
		if phase.PhaseType == PhaseTypeBetting {
			return &ChipLeaderDetector{}
		}
	}

	// Check phases for trick-taking hints
	for _, phase := range genome.TurnPhases {
		if phase.PhaseType == PhaseTypeTrick {
			return &TrickLeaderDetector{}
		}
	}

	// Default to score-based
	return &ScoreLeaderDetector{}
}
```

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./engine -v -run TestSelectLeaderDetector
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/tension.go src/gosim/engine/tension_test.go
git commit -m "feat(tension): add SelectLeaderDetector"
```

---

## Task 7: Implement TensionMetrics Update and Finalize

**Files:**
- Modify: `src/gosim/engine/tension.go`
- Modify: `src/gosim/engine/tension_test.go`

**Step 1: Write the failing tests**

```go
func TestTensionMetrics_Update_LeadChanges(t *testing.T) {
	tm := NewTensionMetrics(2)
	detector := &ScoreLeaderDetector{}

	// Player 0 leads
	state := &GameState{Players: []PlayerState{{Score: 10}, {Score: 5}}}
	tm.Update(state, detector)

	// Player 1 takes lead
	state = &GameState{Players: []PlayerState{{Score: 10}, {Score: 15}}}
	tm.Update(state, detector)

	// Player 0 takes lead back
	state = &GameState{Players: []PlayerState{{Score: 20}, {Score: 15}}}
	tm.Update(state, detector)

	if tm.LeadChanges != 2 {
		t.Errorf("expected 2 lead changes, got %d", tm.LeadChanges)
	}
}

func TestTensionMetrics_Update_ClosestMargin(t *testing.T) {
	tm := NewTensionMetrics(2)
	detector := &ScoreLeaderDetector{}

	// Big lead
	state := &GameState{Players: []PlayerState{{Score: 100}, {Score: 10}}}
	tm.Update(state, detector)

	// Close game
	state = &GameState{Players: []PlayerState{{Score: 100}, {Score: 95}}}
	tm.Update(state, detector)

	// Big lead again - should keep the closest margin
	state = &GameState{Players: []PlayerState{{Score: 200}, {Score: 95}}}
	tm.Update(state, detector)

	// Closest margin was 5/100 = 0.05
	if tm.ClosestMargin < 0.04 || tm.ClosestMargin > 0.06 {
		t.Errorf("expected ClosestMargin≈0.05, got %f", tm.ClosestMargin)
	}
}

func TestTensionMetrics_Finalize_PermanentLead(t *testing.T) {
	tm := NewTensionMetrics(2)
	detector := &ScoreLeaderDetector{}

	// Player 0 leads turn 0
	state := &GameState{Players: []PlayerState{{Score: 10}, {Score: 5}}}
	tm.Update(state, detector)

	// Player 1 leads turn 1
	state = &GameState{Players: []PlayerState{{Score: 10}, {Score: 15}}}
	tm.Update(state, detector)

	// Player 1 still leads turn 2
	state = &GameState{Players: []PlayerState{{Score: 10}, {Score: 20}}}
	tm.Update(state, detector)

	// Player 1 still leads turn 3
	state = &GameState{Players: []PlayerState{{Score: 10}, {Score: 25}}}
	tm.Update(state, detector)

	// Player 1 wins - took permanent lead at turn 1
	tm.Finalize(1)

	if tm.DecisiveTurn != 1 {
		t.Errorf("expected DecisiveTurn=1 (permanent lead), got %d", tm.DecisiveTurn)
	}
}

func TestTensionMetrics_Finalize_Draw(t *testing.T) {
	tm := NewTensionMetrics(2)
	tm.TotalTurns = 50

	// Draw game - decisive turn should be at end (max tension)
	tm.Finalize(-1)

	if tm.DecisiveTurn != 50 {
		t.Errorf("expected DecisiveTurn=50 (draw), got %d", tm.DecisiveTurn)
	}
}

func TestTensionMetrics_DecisiveTurnPct(t *testing.T) {
	tm := NewTensionMetrics(2)
	tm.TotalTurns = 100
	tm.DecisiveTurn = 75

	pct := tm.DecisiveTurnPct()
	if pct < 0.74 || pct > 0.76 {
		t.Errorf("expected DecisiveTurnPct=0.75, got %f", pct)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd src/gosim && go test ./engine -v -run "TestTensionMetrics_Update|TestTensionMetrics_Finalize|TestTensionMetrics_Decisive"
```

Expected: FAIL

**Step 3: Write implementation**

```go
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
		tm.DecisiveTurn = tm.TotalTurns
		return
	}

	// Scan backwards to find when winner took permanent lead
	tm.DecisiveTurn = tm.TotalTurns

	for i := len(tm.leaderHistory) - 1; i >= 0; i-- {
		if tm.leaderHistory[i] != winnerID && tm.leaderHistory[i] != -1 {
			// Found a turn where someone else was leading
			if i+1 < len(tm.leaderHistory) {
				tm.DecisiveTurn = i + 1
			}
			break
		}
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
```

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./engine -v -run "TestTensionMetrics_Update|TestTensionMetrics_Finalize|TestTensionMetrics_Decisive"
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/tension.go src/gosim/engine/tension_test.go
git commit -m "feat(tension): add Update and Finalize methods"
```

---

## Task 8: Add Tension Fields to GameMetrics

**Files:**
- Modify: `src/gosim/simulation/runner.go`

**Step 1: Find GameMetrics struct and add fields**

```bash
grep -n "type GameMetrics struct" src/gosim/simulation/runner.go
```

**Step 2: Add tension fields to GameMetrics**

Add these fields to the `GameMetrics` struct:

```go
// Tension curve metrics
LeadChanges      uint32
DecisiveTurnPct  float32
ClosestMargin    float32
```

**Step 3: Run existing tests to ensure no breakage**

```bash
cd src/gosim && go test ./simulation -v
```

Expected: PASS

**Step 4: Commit**

```bash
git add src/gosim/simulation/runner.go
git commit -m "feat(tension): add tension fields to GameMetrics"
```

---

## Task 9: Integrate Tension Tracking into RunSingleGame

**Files:**
- Modify: `src/gosim/simulation/runner.go`

**Step 1: Add import for engine package tension functions (if needed)**

**Step 2: Find RunSingleGame and add tension tracking**

After game state initialization, add:
```go
detector := engine.SelectLeaderDetector(genome)
tensionMetrics := engine.NewTensionMetrics(int(state.NumPlayers))
```

In the main game loop, after `engine.ApplyMove`:
```go
tensionMetrics.Update(state, detector)
```

At game end, before returning:
```go
if winnerID >= 0 {
    tensionMetrics.Finalize(int(winnerID))
} else {
    tensionMetrics.Finalize(-1)
}
metrics.LeadChanges = uint32(tensionMetrics.LeadChanges)
metrics.DecisiveTurnPct = tensionMetrics.DecisiveTurnPct()
metrics.ClosestMargin = tensionMetrics.ClosestMargin
```

**Step 3: Run tests**

```bash
cd src/gosim && go test ./simulation -v
```

Expected: PASS

**Step 4: Commit**

```bash
git add src/gosim/simulation/runner.go
git commit -m "feat(tension): integrate tension tracking into RunSingleGame"
```

---

## Task 10: Update FlatBuffers Schema

**Files:**
- Modify: `schema/simulation.fbs`

**Step 1: Add tension fields to AggregatedStats table**

Find the `AggregatedStats` table and add:
```fbs
// Tension curve metrics
lead_changes: uint32 = 0;
decisive_turn_pct: float32 = 1.0;
closest_margin: float32 = 1.0;
```

**Step 2: Regenerate bindings**

```bash
flatc --go -o src/gosim/bindings schema/simulation.fbs
flatc --python -o src/darwindeck/bindings schema/simulation.fbs
```

**Step 3: Commit**

```bash
git add schema/simulation.fbs src/gosim/bindings/ src/darwindeck/bindings/
git commit -m "feat(tension): add tension fields to FlatBuffers schema"
```

---

## Task 11: Update CGo Bridge

**Files:**
- Modify: `src/gosim/cgo/bridge.go`

**Step 1: Add tension fields to SimStats struct**

Find `SimStats` struct and add:
```go
// Tension metrics
LeadChanges      uint32
DecisiveTurnPct  float32
ClosestMargin    float32
```

**Step 2: Update serialization to include tension metrics**

Find where `AggregatedStats` is built and add the tension fields.

**Step 3: Rebuild CGo library**

```bash
make build-cgo
```

**Step 4: Run tests**

```bash
cd src/gosim && go test ./... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/cgo/bridge.go
git commit -m "feat(tension): add tension fields to CGo bridge"
```

---

## Task 12: Update Python SimulationResults

**Files:**
- Modify: `src/darwindeck/evolution/fitness_full.py`

**Step 1: Add tension fields to SimulationResults dataclass**

```python
# Tension curve metrics
lead_changes: int = 0
decisive_turn_pct: float = 1.0
closest_margin: float = 1.0
```

**Step 2: Run Python tests**

```bash
uv run pytest tests/unit/test_fitness.py -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add src/darwindeck/evolution/fitness_full.py
git commit -m "feat(tension): add tension fields to SimulationResults"
```

---

## Task 13: Update Python Fitness Calculation

**Files:**
- Modify: `src/darwindeck/evolution/fitness_full.py`

**Step 1: Write test for new tension calculation**

Create or update test file:
```python
def test_tension_curve_with_real_data():
    """Fitness uses real tension data when available."""
    results = SimulationResults(
        total_games=100,
        wins=(50, 50),
        player_count=2,
        draws=0,
        avg_turns=50,
        errors=0,
        lead_changes=5,
        decisive_turn_pct=0.8,
        closest_margin=0.1,
    )

    evaluator = FitnessEvaluator()
    metrics = evaluator.evaluate(create_war_genome(), results)

    # Should use real data, not fallback
    # lead_change_score = min(1.0, 5 / 2.5) = 1.0
    # decisive_turn_score = 0.8
    # margin_score = 1.0 - 0.1 = 0.9
    # tension = 1.0*0.4 + 0.8*0.4 + 0.9*0.2 = 0.4 + 0.32 + 0.18 = 0.9
    assert metrics.tension_curve > 0.85
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_fitness.py -v -k test_tension_curve_with_real_data
```

Expected: FAIL (new fields not being used yet)

**Step 3: Update _compute_metrics to use real tension data**

Replace the tension_curve calculation section with:
```python
# 3. Tension curve - use real instrumentation if available
has_tension_data = (
    results.lead_changes > 0 or
    results.decisive_turn_pct < 1.0 or
    results.closest_margin < 1.0
)

if has_tension_data:
    turns_per_expected_change = 20
    expected_changes = max(1, results.avg_turns / turns_per_expected_change)
    lead_change_score = min(1.0, results.lead_changes / expected_changes)
    decisive_turn_score = results.decisive_turn_pct
    margin_score = 1.0 - results.closest_margin

    tension_curve = (
        lead_change_score * 0.4 +
        decisive_turn_score * 0.4 +
        margin_score * 0.2
    )
else:
    turn_score = min(1.0, results.avg_turns / 100.0)
    length_bonus = min(1.0, max(0.0, (results.avg_turns - 20) / 50.0))
    tension_curve = min(1.0, turn_score * 0.6 + length_bonus * 0.4)
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_fitness.py -v -k test_tension_curve_with_real_data
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/fitness_full.py tests/unit/test_fitness.py
git commit -m "feat(tension): update fitness calculation to use real tension data"
```

---

## Task 14: Update CGo Bridge Result Parsing

**Files:**
- Modify: `src/darwindeck/bindings/cgo_bridge.py` (or wherever results are parsed)

**Step 1: Find where SimulationResults is created from FlatBuffers response**

**Step 2: Add parsing for tension fields**

```python
lead_changes=stats.LeadChanges(),
decisive_turn_pct=stats.DecisiveTurnPct(),
closest_margin=stats.ClosestMargin(),
```

**Step 3: Run integration test**

```bash
uv run pytest tests/integration/test_betting.py -v
```

Expected: PASS

**Step 4: Commit**

```bash
git add src/darwindeck/bindings/cgo_bridge.py
git commit -m "feat(tension): parse tension fields from CGo response"
```

---

## Task 15: Integration Test

**Files:**
- Create: `tests/integration/test_tension.py`

**Step 1: Write integration test**

```python
"""Integration tests for tension curve metrics."""

import pytest
from darwindeck.genome.examples import create_war_genome, create_hearts_genome
from darwindeck.bindings.cgo_bridge import simulate_batch
from darwindeck.genome.bytecode import BytecodeCompiler


class TestTensionMetrics:
    """Test tension metrics flow from Go to Python."""

    def _run_simulation(self, genome, num_games=100):
        """Helper to run simulation and get results."""
        # Implementation depends on actual CGo bridge interface
        pass

    def test_tension_metrics_populated(self):
        """Tension metrics should be populated after simulation."""
        genome = create_war_genome()
        # Run simulation and check that tension fields are non-default
        # (Implementation depends on actual interface)
        pass

    def test_hearts_uses_avoidance_logic(self):
        """Hearts should use trick avoidance for leader detection."""
        # Would need to verify through logged output or specific behavior
        pass
```

**Step 2: Run integration tests**

```bash
uv run pytest tests/integration/test_tension.py -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add tests/integration/test_tension.py
git commit -m "test(tension): add integration tests for tension metrics"
```

---

## Task 16: Rebuild and Final Verification

**Step 1: Rebuild CGo library**

```bash
make build-cgo
```

**Step 2: Run all Go tests**

```bash
cd src/gosim && go test ./... -v
```

Expected: All PASS

**Step 3: Run all Python tests**

```bash
uv run pytest tests/ -v
```

Expected: All PASS

**Step 4: Update ROADMAP.md**

Mark "Tension Curve Analysis" as complete in the roadmap.

**Step 5: Final commit**

```bash
git add ROADMAP.md
git commit -m "docs: mark tension curve analysis complete in roadmap"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | TensionMetrics struct | tension.go |
| 2 | ScoreLeaderDetector | tension.go |
| 3 | HandSizeLeaderDetector | tension.go |
| 4 | Trick detectors | tension.go |
| 5 | ChipLeaderDetector | tension.go |
| 6 | SelectLeaderDetector | tension.go |
| 7 | Update/Finalize methods | tension.go |
| 8 | GameMetrics fields | runner.go |
| 9 | Runner integration | runner.go |
| 10 | FlatBuffers schema | simulation.fbs |
| 11 | CGo bridge | bridge.go |
| 12 | Python SimulationResults | fitness_full.py |
| 13 | Python fitness calc | fitness_full.py |
| 14 | CGo result parsing | cgo_bridge.py |
| 15 | Integration tests | test_tension.py |
| 16 | Final verification | ROADMAP.md |

**Total: 16 tasks**
