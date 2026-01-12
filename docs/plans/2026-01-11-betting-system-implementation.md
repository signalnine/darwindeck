# Betting/Wagering System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add betting/wagering mechanics to enable poker-style games through evolution.

**Architecture:** BettingPhase as new phase type, player-level chip tracking, simplified betting with ALL_IN support for short stacks.

**Tech Stack:** Python (schema, serialization, mutations), Go (game state, move generation, resolution), FlatBuffers (bridge)

---

## Task 1: Add BettingPhase and BettingAction to Python Schema

**Files:**
- Modify: `src/darwindeck/genome/schema.py`

**Step 1: Add BettingAction enum**

```python
class BettingAction(Enum):
    """Actions available during a betting phase."""
    CHECK = "check"    # Pass without betting (only if no current bet)
    BET = "bet"        # Place initial bet (min_bet amount)
    CALL = "call"      # Match current bet
    RAISE = "raise"    # Increase bet by min_bet
    ALL_IN = "all_in"  # Bet all remaining chips
    FOLD = "fold"      # Surrender hand, forfeit pot
```

**Step 2: Add BettingPhase dataclass**

```python
@dataclass(frozen=True)
class BettingPhase:
    """A betting round within the turn structure."""
    min_bet: int = 10          # Minimum bet/raise amount
    max_raises: int = 3        # Maximum raises per round (prevents infinite loops)
```

**Step 3: Update SetupRules**

Add `starting_chips: int = 0` field. Value of 0 means no betting enabled.

**Step 4: Run tests**

Run: `uv run pytest tests/unit/test_schema.py -v`
Expected: PASS (existing tests should still pass)

---

## Task 2: Add BettingPhase Serialization

**Files:**
- Modify: `src/darwindeck/genome/serialization.py`

**Step 1: Import BettingPhase**

Add to imports: `BettingPhase, BettingAction`

**Step 2: Add BettingPhase to _phase_to_dict**

```python
elif isinstance(phase, BettingPhase):
    return {
        "type": "BettingPhase",
        "min_bet": phase.min_bet,
        "max_raises": phase.max_raises,
    }
```

**Step 3: Add BettingPhase to _phase_from_dict**

```python
elif phase_type == "BettingPhase":
    return BettingPhase(
        min_bet=data.get("min_bet", 10),
        max_raises=data.get("max_raises", 3),
    )
```

**Step 4: Update _setup_to_dict and _setup_from_dict**

Add `starting_chips` field handling.

**Step 5: Write test**

```python
def test_betting_phase_serialization():
    phase = BettingPhase(min_bet=20, max_raises=2)
    d = _phase_to_dict(phase)
    restored = _phase_from_dict(d)
    assert restored == phase
```

**Step 6: Run tests**

Run: `uv run pytest tests/unit/test_serialization.py -v`
Expected: PASS

**Step 7: Commit**

```bash
git add src/darwindeck/genome/
git commit -m "feat: add BettingPhase and BettingAction to schema and serialization"
```

---

## Task 3: Update Go GameState for Betting

**Files:**
- Modify: `src/gosim/engine/types.go`

**Step 1: Add betting fields to PlayerState**

```go
type PlayerState struct {
    Hand       []Card
    Score      int
    HasFolded  bool // Track fold status for current hand
    IsAllIn    bool // Track all-in status (can't act but still in hand)
    Chips      int  // Current chip count
    CurrentBet int  // Amount bet this round
}
```

**Step 2: Add betting fields to GameState**

```go
type GameState struct {
    // ... existing fields ...
    Pot                int // Accumulated bets
    CurrentBet         int // Highest bet this round
    RaiseCount         int // Raises this round
    BettingStartPlayer int // Rotates each hand for position fairness
}
```

**Step 3: Update NewGameState**

Initialize new fields to zero/false.

**Step 4: Update Clone method**

Copy all betting fields.

**Step 5: Add InitializeChips helper**

```go
func (gs *GameState) InitializeChips(startingChips int) {
    for i := range gs.Players {
        gs.Players[i].Chips = startingChips
        gs.Players[i].CurrentBet = 0
        gs.Players[i].HasFolded = false
        gs.Players[i].IsAllIn = false
    }
    gs.Pot = 0
    gs.CurrentBet = 0
    gs.RaiseCount = 0
    gs.BettingStartPlayer = 0
}
```

**Step 6: Add ResetHand helper**

```go
func (gs *GameState) ResetHand() {
    for i := range gs.Players {
        gs.Players[i].CurrentBet = 0
        gs.Players[i].HasFolded = false
        gs.Players[i].IsAllIn = false
    }
    gs.Pot = 0
    gs.CurrentBet = 0
    gs.RaiseCount = 0
    gs.BettingStartPlayer = (gs.BettingStartPlayer + 1) % len(gs.Players)
}
```

**Step 7: Run tests**

Run: `cd src/gosim && go test ./engine -v`
Expected: PASS

**Step 8: Commit**

```bash
git add src/gosim/engine/types.go
git commit -m "feat(gosim): add betting fields to PlayerState and GameState"
```

---

## Task 4: Add BettingPhase to Go Bytecode Parser

**Files:**
- Modify: `src/gosim/engine/bytecode.go`

**Step 1: Add BettingPhase constants**

```go
const (
    PhaseTypeBetting = 5 // New phase type
)

type BettingPhaseData struct {
    MinBet    int
    MaxRaises int
}
```

**Step 2: Update ParseGenome to handle BettingPhase**

Parse betting phase data from bytecode.

**Step 3: Run tests**

Run: `cd src/gosim && go test ./engine -v`
Expected: PASS

---

## Task 5: Implement Betting Move Generation (Go)

**Files:**
- Create: `src/gosim/engine/betting.go`

**Step 1: Define BettingAction type**

```go
type BettingAction int

const (
    BettingCheck BettingAction = iota
    BettingBet
    BettingCall
    BettingRaise
    BettingAllIn
    BettingFold
)
```

**Step 2: Implement GenerateBettingMoves**

```go
func GenerateBettingMoves(gs *GameState, phase *BettingPhaseData, playerID int) []BettingAction {
    player := &gs.Players[playerID]
    moves := []BettingAction{}

    // Can't act if folded, all-in, or no chips
    if player.HasFolded || player.IsAllIn || player.Chips <= 0 {
        return moves
    }

    toCall := gs.CurrentBet - player.CurrentBet

    if toCall == 0 {
        // No bet to match
        moves = append(moves, BettingCheck)
        if player.Chips >= phase.MinBet {
            moves = append(moves, BettingBet)
        } else if player.Chips > 0 {
            // Can't afford min bet, but can go all-in
            moves = append(moves, BettingAllIn)
        }
    } else {
        // Must match, raise, all-in, or fold
        if player.Chips >= toCall {
            moves = append(moves, BettingCall)
            if player.Chips >= toCall+phase.MinBet && gs.RaiseCount < phase.MaxRaises {
                moves = append(moves, BettingRaise)
            }
        }
        if player.Chips > 0 && player.Chips < toCall {
            // Can't afford call, but can go all-in
            moves = append(moves, BettingAllIn)
        }
        moves = append(moves, BettingFold)
    }

    return moves
}
```

**Step 3: Implement ApplyBettingAction**

```go
func ApplyBettingAction(gs *GameState, phase *BettingPhaseData, playerID int, action BettingAction) {
    player := &gs.Players[playerID]

    switch action {
    case BettingCheck:
        // No change
    case BettingBet:
        player.Chips -= phase.MinBet
        player.CurrentBet += phase.MinBet
        gs.Pot += phase.MinBet
        gs.CurrentBet = phase.MinBet
    case BettingCall:
        toCall := gs.CurrentBet - player.CurrentBet
        player.Chips -= toCall
        player.CurrentBet = gs.CurrentBet
        gs.Pot += toCall
    case BettingRaise:
        toCall := gs.CurrentBet - player.CurrentBet
        raiseAmount := toCall + phase.MinBet
        player.Chips -= raiseAmount
        player.CurrentBet = gs.CurrentBet + phase.MinBet
        gs.Pot += raiseAmount
        gs.CurrentBet = player.CurrentBet
        gs.RaiseCount++
    case BettingAllIn:
        amount := player.Chips
        player.Chips = 0
        player.CurrentBet += amount
        gs.Pot += amount
        player.IsAllIn = true
        if player.CurrentBet > gs.CurrentBet {
            gs.CurrentBet = player.CurrentBet
        }
    case BettingFold:
        player.HasFolded = true
    }
}
```

**Step 4: Write tests**

```go
func TestBettingMoves_NoCurrentBet(t *testing.T) {
    gs := NewGameState(2)
    gs.Players[0].Chips = 100
    phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

    moves := GenerateBettingMoves(gs, phase, 0)

    assert.Contains(t, moves, BettingCheck)
    assert.Contains(t, moves, BettingBet)
    assert.NotContains(t, moves, BettingFold)
}

func TestBettingMoves_CantAffordCall(t *testing.T) {
    gs := NewGameState(2)
    gs.Players[0].Chips = 5
    gs.CurrentBet = 10
    phase := &BettingPhaseData{MinBet: 10, MaxRaises: 3}

    moves := GenerateBettingMoves(gs, phase, 0)

    assert.Contains(t, moves, BettingAllIn)
    assert.Contains(t, moves, BettingFold)
    assert.NotContains(t, moves, BettingCall)
}

func TestApplyBettingAction_AllIn(t *testing.T) {
    gs := NewGameState(2)
    gs.Players[0].Chips = 50
    phase := &BettingPhaseData{MinBet: 10}

    ApplyBettingAction(gs, phase, 0, BettingAllIn)

    assert.Equal(t, 0, gs.Players[0].Chips)
    assert.Equal(t, 50, gs.Pot)
    assert.True(t, gs.Players[0].IsAllIn)
}
```

**Step 5: Run tests**

Run: `cd src/gosim && go test ./engine -v`
Expected: PASS

**Step 6: Commit**

```bash
git add src/gosim/engine/betting.go
git commit -m "feat(gosim): implement betting move generation and action application"
```

---

## Task 6: Implement Betting Round Resolution (Go)

**Files:**
- Modify: `src/gosim/engine/betting.go`

**Step 1: Implement RunBettingRound with proper termination**

```go
func RunBettingRound(gs *GameState, phase *BettingPhaseData, aiPlayers []AIPlayer) {
    // Track who needs to act
    needsToAct := make([]bool, len(gs.Players))
    for i := range gs.Players {
        p := &gs.Players[i]
        needsToAct[i] = !p.HasFolded && !p.IsAllIn && p.Chips > 0
    }

    currentPlayer := gs.BettingStartPlayer

    for {
        // Check termination: only one player remains
        activeCount := CountActivePlayers(gs)
        if activeCount <= 1 {
            return
        }

        // Check termination: all remaining players are all-in
        actingCount := CountActingPlayers(gs)
        if actingCount == 0 {
            return
        }

        // Check termination: round complete (all acted and matched)
        if !anyNeedsToAct(needsToAct) && allBetsMatched(gs) {
            return
        }

        // Find next player who needs to act
        for !needsToAct[currentPlayer] {
            currentPlayer = (currentPlayer + 1) % len(gs.Players)
        }

        player := &gs.Players[currentPlayer]
        moves := GenerateBettingMoves(gs, phase, currentPlayer)

        if len(moves) == 0 {
            needsToAct[currentPlayer] = false
            currentPlayer = (currentPlayer + 1) % len(gs.Players)
            continue
        }

        action := aiPlayers[currentPlayer].SelectBettingAction(gs, phase, moves)
        oldCurrentBet := gs.CurrentBet

        ApplyBettingAction(gs, phase, currentPlayer, action)

        // If bet increased, everyone else needs to act again
        if gs.CurrentBet > oldCurrentBet {
            for i := range needsToAct {
                p := &gs.Players[i]
                if !p.HasFolded && !p.IsAllIn && p.Chips > 0 {
                    needsToAct[i] = true
                }
            }
        }

        needsToAct[currentPlayer] = false
        currentPlayer = (currentPlayer + 1) % len(gs.Players)
    }
}

func CountActivePlayers(gs *GameState) int {
    count := 0
    for _, p := range gs.Players {
        if !p.HasFolded {
            count++
        }
    }
    return count
}

func CountActingPlayers(gs *GameState) int {
    count := 0
    for _, p := range gs.Players {
        if !p.HasFolded && !p.IsAllIn && p.Chips > 0 {
            count++
        }
    }
    return count
}

func allBetsMatched(gs *GameState) bool {
    for _, p := range gs.Players {
        if !p.HasFolded && !p.IsAllIn && p.CurrentBet != gs.CurrentBet {
            return false
        }
    }
    return true
}
```

**Step 2: Implement showdown with split pots**

```go
func ResolveShowdown(gs *GameState, genome *ParsedGenome) []int {
    // Get players still in hand (not folded)
    activePlayers := []int{}
    for i, p := range gs.Players {
        if !p.HasFolded {
            activePlayers = append(activePlayers, i)
        }
    }

    // Single player = automatic winner
    if len(activePlayers) == 1 {
        return activePlayers
    }

    // Multiple players = use win condition to determine winner(s)
    return DetermineWinners(gs, genome, activePlayers)
}

func AwardPot(gs *GameState, winnerIDs []int) {
    if len(winnerIDs) == 0 {
        return
    }

    // Split pot evenly among winners
    share := gs.Pot / len(winnerIDs)
    remainder := gs.Pot % len(winnerIDs)

    for i, winnerID := range winnerIDs {
        gs.Players[winnerID].Chips += share
        if i == 0 {
            gs.Players[winnerID].Chips += remainder
        }
    }
    gs.Pot = 0
}
```

**Step 3: Write termination tests**

```go
func TestRoundEnds_AllMatched(t *testing.T) {
    // Setup: both players check
    // Expected: round ends
}

func TestRoundEnds_AllFoldedButOne(t *testing.T) {
    // Setup: player 1 folds
    // Expected: round ends, player 0 wins
}

func TestRoundContinues_RaiseReopens(t *testing.T) {
    // Setup: P0 bets, P1 raises
    // Expected: P0 needs to act again
}

func TestShowdown_SplitPot(t *testing.T) {
    gs := NewGameState(2)
    gs.Pot = 100
    winnerIDs := []int{0, 1}

    AwardPot(gs, winnerIDs)

    assert.Equal(t, 50, gs.Players[0].Chips)
    assert.Equal(t, 50, gs.Players[1].Chips)
}

func TestShowdown_OddRemainder(t *testing.T) {
    gs := NewGameState(2)
    gs.Pot = 101
    winnerIDs := []int{0, 1}

    AwardPot(gs, winnerIDs)

    assert.Equal(t, 51, gs.Players[0].Chips) // First gets remainder
    assert.Equal(t, 50, gs.Players[1].Chips)
}
```

**Step 4: Run tests**

Run: `cd src/gosim && go test ./engine -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/betting.go
git commit -m "feat(gosim): implement betting round resolution with split pots"
```

---

## Task 7: Add AI Betting Support (Go)

**Files:**
- Modify: `src/gosim/engine/ai.go` (or create if needed)

**Step 1: Add SelectBettingAction to AIPlayer interface**

```go
type AIPlayer interface {
    SelectMove(gs *GameState, moves []Move) Move
    SelectBettingAction(gs *GameState, phase *BettingPhaseData, moves []BettingAction) BettingAction
}
```

**Step 2: Implement for RandomAI**

```go
func (ai *RandomAI) SelectBettingAction(gs *GameState, phase *BettingPhaseData, moves []BettingAction) BettingAction {
    return moves[ai.rng.Intn(len(moves))]
}
```

**Step 3: Implement for GreedyAI**

```go
func (ai *GreedyAI) SelectBettingAction(gs *GameState, phase *BettingPhaseData, moves []BettingAction) BettingAction {
    playerID := gs.ActivePlayer
    handStrength := ai.EvaluateHandStrength(gs.Players[playerID].Hand)

    // Strong hand
    if handStrength > 0.7 {
        if contains(moves, BettingRaise) { return BettingRaise }
        if contains(moves, BettingBet) { return BettingBet }
        if contains(moves, BettingAllIn) { return BettingAllIn }
    }

    // Medium hand
    if handStrength > 0.3 {
        if contains(moves, BettingCall) { return BettingCall }
        if contains(moves, BettingCheck) { return BettingCheck }
    }

    // Weak hand
    if contains(moves, BettingCheck) { return BettingCheck }
    return BettingFold
}
```

**Step 4: Implement for MCTS** (betting as part of action space)

**Step 5: Run tests**

Run: `cd src/gosim && go test ./... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add src/gosim/
git commit -m "feat(gosim): add AI betting action selection"
```

---

## Task 8: Update FlatBuffers Schema

**Files:**
- Modify: `schema/simulation.fbs`

**Step 1: Add betting fields to request**

```flatbuffers
table BatchRequest {
    // ... existing fields ...
    starting_chips: int = 0;
}
```

**Step 2: Add betting stats to response**

```flatbuffers
table BatchResponse {
    // ... existing fields ...
    avg_final_chips: [float];  // Average final chips per player
    fold_rate: float;          // Percentage of hands ending in fold
}
```

**Step 3: Regenerate FlatBuffers code**

Run: `flatc --go --python -o src/ schema/simulation.fbs`

**Step 4: Commit**

```bash
git add schema/ src/
git commit -m "feat: add betting fields to FlatBuffers schema"
```

---

## Task 9: Update Python Bytecode Compiler

**Files:**
- Modify: `src/darwindeck/genome/bytecode.py`

**Step 1: Add BettingPhase compilation**

```python
def _compile_phase(self, phase) -> bytes:
    if isinstance(phase, BettingPhase):
        return struct.pack(
            ">BII",
            5,  # PhaseTypeBetting
            phase.min_bet,
            phase.max_raises,
        )
    # ... existing phase types
```

**Step 2: Update header to include starting_chips**

**Step 3: Run tests**

Run: `uv run pytest tests/unit/test_bytecode.py -v`
Expected: PASS

**Step 4: Commit**

```bash
git add src/darwindeck/genome/bytecode.py
git commit -m "feat: add BettingPhase to bytecode compiler"
```

---

## Task 10: Add Betting Mutation Operators

**Files:**
- Modify: `src/darwindeck/evolution/operators.py`

**Step 1: Implement AddBettingPhaseMutation**

```python
class AddBettingPhaseMutation(Mutation):
    """Insert a BettingPhase at random position."""

    def mutate(self, genome: GameGenome, rng: random.Random) -> GameGenome:
        # Validate min_bet <= starting_chips
        starting_chips = genome.setup.starting_chips or 1000
        min_bet_options = [b for b in [5, 10, 20, 50] if b <= starting_chips]
        if not min_bet_options:
            min_bet_options = [starting_chips // 10 or 1]

        new_phase = BettingPhase(
            min_bet=rng.choice(min_bet_options),
            max_raises=rng.choice([1, 2, 3, 4]),
        )
        phases = list(genome.turn_structure.phases)
        insert_pos = rng.randint(0, len(phases))
        phases.insert(insert_pos, new_phase)
        # ... return updated genome
```

**Step 2: Implement RemoveBettingPhaseMutation**

**Step 3: Implement MutateBettingPhaseMutation**

**Step 4: Implement MutateStartingChipsMutation**

Ensure `min_bet <= starting_chips` constraint is maintained.

**Step 5: Add to create_default_pipeline**

```python
# Betting mutations
AddBettingPhaseMutation(probability=0.05),
RemoveBettingPhaseMutation(probability=0.05),
MutateBettingPhaseMutation(probability=0.10),
MutateStartingChipsMutation(probability=0.10),
```

**Step 6: Run tests**

Run: `uv run pytest tests/unit/test_operators.py -v`
Expected: PASS

**Step 7: Commit**

```bash
git add src/darwindeck/evolution/operators.py
git commit -m "feat: add betting mutation operators to evolution pipeline"
```

---

## Task 11: Create Simple Poker Seed Genome

**Files:**
- Modify: `src/darwindeck/genome/examples.py`

**Step 1: Create simple_poker_genome**

```python
def create_simple_poker_genome() -> GameGenome:
    """Simple poker - deal 5 cards, bet, best hand wins."""
    return GameGenome(
        schema_version="1.0",
        genome_id="simple_poker",
        generation=0,
        setup=SetupRules(
            cards_per_player=5,
            starting_chips=1000,
        ),
        turn_structure=TurnStructure(
            phases=[
                BettingPhase(min_bet=10, max_raises=3),
            ],
        ),
        win_conditions=[
            WinCondition(type="best_poker_hand"),
        ],
        max_turns=1,
        player_count=2,
    )
```

**Step 2: Add to SEED_GENOMES list**

**Step 3: Run tests**

Run: `uv run pytest tests/ -v -k poker`
Expected: PASS

**Step 4: Commit**

```bash
git add src/darwindeck/genome/examples.py
git commit -m "feat: add simple poker seed genome with betting"
```

---

## Task 12: Integration Tests

**Files:**
- Create: `tests/integration/test_betting.py`

**Step 1: Test betting genome serialization round-trip**

```python
def test_betting_genome_roundtrip():
    genome = create_simple_poker_genome()
    json_str = genome_to_json(genome)
    restored = genome_from_json(json_str)
    assert restored.setup.starting_chips == 1000
    assert isinstance(restored.turn_structure.phases[0], BettingPhase)
```

**Step 2: Test betting simulation completes**

```python
def test_betting_simulation_completes():
    """Game with BettingPhase runs without infinite loop."""
    genome = create_simple_poker_genome()
    results = run_simulation(genome, num_games=100)
    assert results.games_played == 100
```

**Step 3: Test chips conservation**

```python
def test_betting_chips_conserved():
    """Total chips in system equals starting_chips * player_count."""
    genome = create_simple_poker_genome()
    # Run single game and verify total chips unchanged
```

**Step 4: Test betting mutations**

```python
def test_betting_mutations():
    genome = create_war_genome()  # No betting
    mutation = AddBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome, random.Random(42))
    betting_phases = [p for p in mutated.turn_structure.phases if isinstance(p, BettingPhase)]
    assert len(betting_phases) == 1
```

**Step 5: Test parameter validation**

```python
def test_betting_parameter_validation():
    """Mutations don't produce invalid configs (min_bet > chips)."""
    genome = GameGenome(
        setup=SetupRules(starting_chips=20),
        # ...
    )
    mutation = MutateBettingPhaseMutation(probability=1.0)
    for _ in range(100):
        mutated = mutation.mutate(genome, random.Random())
        for phase in mutated.turn_structure.phases:
            if isinstance(phase, BettingPhase):
                assert phase.min_bet <= mutated.setup.starting_chips
```

**Step 6: Run all tests**

Run: `uv run pytest tests/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add tests/
git commit -m "test: add betting system integration tests"
```

---

## Summary

**Total Tasks:** 12

**Key Deliverables:**
1. BettingPhase and BettingAction (with ALL_IN) in Python schema
2. Serialization support for betting
3. Go GameState betting fields (including IsAllIn, BettingStartPlayer)
4. Go betting move generation with short-stack support
5. Betting round resolution with proper termination logic
6. Split pot support for tied hands
7. AI betting support
8. FlatBuffers schema updates
9. Mutation operators with parameter validation
10. Simple poker seed genome
11. Comprehensive integration tests

**Backward Compatibility:**
- `starting_chips=0` means no betting (existing genomes unchanged)
- All existing tests should continue to pass

**Multi-Agent Review Issues Addressed:**
- ALL_IN action for short stacks (no forced fold)
- Explicit betting round termination state machine
- Split pot handling for ties
- Position rotation for fairness
- Testing strategy included
