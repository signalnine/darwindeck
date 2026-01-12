# Betting/Wagering System Design

## Overview

Add betting/wagering mechanics to enable poker-style and betting card games through the evolution system.

**Scope:** Simplified betting - fixed bet sizes, limited raises, no side pots. Sufficient for novel evolved games while keeping the parameter space bounded for effective evolution.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Betting complexity | Simplified | Bounded parameters for evolution, no side pots |
| Integration approach | BettingPhase as new phase type | Consistent with existing architecture |
| Resource tracking | Player-level chips | YAGNI - simple and sufficient |
| Actions | CHECK, BET, CALL, RAISE, FOLD, ALL_IN | Core strategic choices with short-stack support |
| Round resolution | Action-complete termination | Standard poker: round ends when all matched and acted |
| Fold behavior | Eliminates from current hand | Standard poker behavior |
| Pot award | Support split pots for ties | Fair distribution on tied hands |
| Position | Rotating start position per hand | Prevents permanent positional advantage |

## Architecture

```
GameGenome
├── setup
│   └── starting_chips: int (NEW)
├── turn_structure
│   └── phases: [..., BettingPhase, ...]  (NEW phase type)
└── win_conditions (unchanged - used for showdown)
```

## New Components

### BettingPhase Dataclass

```python
@dataclass(frozen=True)
class BettingPhase:
    """A betting round within the turn structure."""

    min_bet: int = 10          # Minimum bet/raise amount
    max_raises: int = 3        # Maximum raises per round (prevents infinite loops)
```

**Removed fields:**
- `max_bet` - Not needed for simplified betting
- `mandatory` - All betting phases are mandatory
- `fixed_raise` - Raises use `min_bet` as increment for simplicity

### BettingAction Enum

```python
class BettingAction(Enum):
    CHECK = "check"    # Pass without betting (only if no current bet)
    BET = "bet"        # Place initial bet (min_bet amount)
    CALL = "call"      # Match current bet
    RAISE = "raise"    # Increase bet by min_bet
    ALL_IN = "all_in"  # Bet all remaining chips (short-stack support)
    FOLD = "fold"      # Surrender hand, forfeit pot
```

### SetupRules Extension

```python
@dataclass(frozen=True)
class SetupRules:
    cards_per_player: int
    initial_deck: str = "standard_52"
    initial_discard_count: int = 0
    trump_suit: Optional[Suit] = None
    starting_chips: int = 0  # NEW - 0 means no betting enabled
```

### PlayerState Extension (Go)

```go
type PlayerState struct {
    Hand       []Card
    Score      int
    HasFolded  bool  // NEW - track fold status for current hand
    IsAllIn    bool  // NEW - track all-in status (can't act but still in hand)
    Chips      int   // NEW - current chip count
    CurrentBet int   // NEW - amount bet this round
}
```

### GameState Extension (Go)

```go
type GameState struct {
    Players            []PlayerState
    Pot                int   // NEW - accumulated bets
    CurrentBet         int   // NEW - highest bet this round
    RaiseCount         int   // NEW - raises this round
    BettingStartPlayer int   // NEW - rotates each hand for position fairness
    // ... existing fields
}
```

## Chip Initialization

During game setup:

```go
func InitializeChips(gs *GameState, startingChips int) {
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

At start of each hand:

```go
func ResetHand(gs *GameState) {
    for i := range gs.Players {
        gs.Players[i].CurrentBet = 0
        gs.Players[i].HasFolded = false
        gs.Players[i].IsAllIn = false
    }
    gs.Pot = 0
    gs.CurrentBet = 0
    gs.RaiseCount = 0
    // Rotate starting position
    gs.BettingStartPlayer = (gs.BettingStartPlayer + 1) % len(gs.Players)
}
```

## Move Generation

Legal actions depend on game state:

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

## Action Effects

| Action | Chip Movement | State Changes |
|--------|---------------|---------------|
| CHECK | None | None |
| BET | `player.Chips -= min_bet` | `player.CurrentBet += min_bet`, `gs.Pot += min_bet`, `gs.CurrentBet = min_bet` |
| CALL | `player.Chips -= toCall` | `player.CurrentBet = gs.CurrentBet`, `gs.Pot += toCall` |
| RAISE | `player.Chips -= (toCall + min_bet)` | `player.CurrentBet = gs.CurrentBet + min_bet`, `gs.Pot += (toCall + min_bet)`, `gs.CurrentBet = player.CurrentBet`, `gs.RaiseCount++` |
| ALL_IN | `amount = player.Chips`, `player.Chips = 0` | `player.CurrentBet += amount`, `gs.Pot += amount`, `player.IsAllIn = true`, if `player.CurrentBet > gs.CurrentBet`: `gs.CurrentBet = player.CurrentBet` |
| FOLD | None | `player.HasFolded = true` |

## Betting Round Termination

A betting round ends when **all** of these conditions are met:

1. Every active player (not folded, not all-in) has acted at least once since the last raise
2. All active players have equal `CurrentBet` (matched the highest bet)
3. OR only one player remains (everyone else folded)
4. OR all remaining players are all-in

**State machine:**

```go
func RunBettingRound(gs *GameState, phase *BettingPhaseData, aiPlayers []AIPlayer) {
    // Track who needs to act
    needsToAct := make([]bool, len(gs.Players))
    for i := range gs.Players {
        p := &gs.Players[i]
        needsToAct[i] = !p.HasFolded && !p.IsAllIn && p.Chips > 0
    }

    // Start from rotating position
    currentPlayer := gs.BettingStartPlayer

    for {
        // Check termination conditions
        activeCount := countActive(gs)
        if activeCount <= 1 {
            return // Everyone folded or one player left
        }

        actingCount := countActing(gs) // Not folded, not all-in, has chips
        if actingCount == 0 {
            return // All remaining players are all-in
        }

        if !anyNeedsToAct(needsToAct) && allBetsMatched(gs) {
            return // Round complete
        }

        // Find next player who can act
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

func allBetsMatched(gs *GameState) bool {
    for _, p := range gs.Players {
        if !p.HasFolded && !p.IsAllIn && p.CurrentBet != gs.CurrentBet {
            return false
        }
    }
    return true
}
```

## Showdown Resolution (with Split Pots)

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
    // DetermineWinners returns slice to support ties
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
        // First winner gets remainder (arbitrary but consistent)
        if i == 0 {
            gs.Players[winnerID].Chips += remainder
        }
    }
    gs.Pot = 0
}
```

## AI Support

### Random AI
Uniform selection from legal betting actions.

### Greedy AI
Heuristic based on hand strength:
- Strong hand (>0.7): RAISE > BET > ALL_IN
- Medium hand (>0.3): CALL > CHECK
- Weak hand: CHECK > FOLD
- Short stack: ALL_IN if strong, FOLD otherwise

### MCTS AI
Betting actions become part of action space. Tree search handles betting naturally.

## Mutation Operators

| Mutation | Effect |
|----------|--------|
| AddBettingPhaseMutation | Insert BettingPhase at random position |
| RemoveBettingPhaseMutation | Remove a BettingPhase |
| MutateBettingPhaseMutation | Adjust min_bet, max_raises |
| MutateStartingChipsMutation | Adjust starting chip count |

**Parameter validation:** Mutations ensure `min_bet <= starting_chips` and `min_bet > 0`.

## Serialization

BettingPhase serializes to JSON:

```json
{
    "type": "BettingPhase",
    "min_bet": 10,
    "max_raises": 3
}
```

Backward compatible - genomes without betting fields load with defaults.

## Files to Modify

| File | Changes |
|------|---------|
| `src/darwindeck/genome/schema.py` | Add BettingPhase, BettingAction, update SetupRules |
| `src/darwindeck/genome/serialization.py` | Add BettingPhase serialization |
| `src/gosim/engine/types.go` | Add betting fields to PlayerState/GameState |
| `src/gosim/engine/betting.go` | New file for move gen, action application, round resolution |
| `src/darwindeck/evolution/operators.py` | Add betting mutations |
| `schema/simulation.fbs` | Add betting fields |

## Testing Strategy

### Unit Tests (Go)

**Move Generation Tests:**
```go
func TestBettingMoves_NoCurrentBet(t *testing.T)     // CHECK, BET available
func TestBettingMoves_WithCurrentBet(t *testing.T)   // CALL, RAISE, FOLD available
func TestBettingMoves_CantAffordCall(t *testing.T)   // ALL_IN, FOLD only
func TestBettingMoves_CantAffordMinBet(t *testing.T) // ALL_IN only (no bet)
func TestBettingMoves_FoldedPlayer(t *testing.T)     // Empty moves
func TestBettingMoves_AllInPlayer(t *testing.T)      // Empty moves
func TestBettingMoves_MaxRaisesReached(t *testing.T) // No RAISE option
```

**Action Application Tests:**
```go
func TestApplyBet(t *testing.T)      // Chips decrease, pot increases, CurrentBet set
func TestApplyCall(t *testing.T)     // Chips decrease by toCall amount
func TestApplyRaise(t *testing.T)    // RaiseCount incremented, CurrentBet updated
func TestApplyAllIn(t *testing.T)    // Chips = 0, IsAllIn = true
func TestApplyFold(t *testing.T)     // HasFolded = true
```

**Round Termination Tests:**
```go
func TestRoundEnds_AllMatched(t *testing.T)      // Everyone called
func TestRoundEnds_AllFoldedButOne(t *testing.T) // Single player remains
func TestRoundEnds_AllAllIn(t *testing.T)        // No one can act
func TestRoundContinues_UnmatchedBet(t *testing.T) // Someone hasn't matched
func TestRoundContinues_RaiseReopens(t *testing.T) // Raise resets needsToAct
```

**Showdown Tests:**
```go
func TestShowdown_SingleWinner(t *testing.T)   // Full pot to winner
func TestShowdown_TiedHands(t *testing.T)      // Split pot evenly
func TestShowdown_EveryoneFolded(t *testing.T) // Last player wins
func TestShowdown_OddChipRemainder(t *testing.T) // First winner gets extra
```

### Integration Tests (Python)

```python
def test_betting_genome_roundtrip():
    """BettingPhase serializes and deserializes correctly."""

def test_betting_simulation_completes():
    """Game with BettingPhase runs without infinite loop."""

def test_betting_chips_conserved():
    """Total chips in system equals starting_chips * player_count."""

def test_betting_mutations():
    """AddBettingPhaseMutation adds valid BettingPhase."""

def test_betting_parameter_validation():
    """Mutations don't produce invalid configs (min_bet > chips)."""
```

### Edge Cases to Test

| Scenario | Expected Behavior |
|----------|-------------------|
| Player starts with fewer chips than min_bet | Can only ALL_IN or CHECK |
| All players go all-in immediately | Round ends, showdown determines winner |
| Player raises to exactly max_raises | RAISE no longer available |
| Two players tie with odd pot (e.g., 101) | First winner gets 51, second gets 50 |
| All but one player folds before showdown | Remaining player wins without showing |

## Out of Scope

- Side pots (all-in players can only win up to their contribution)
- Variable bet amounts (fixed increments only)
- Blind/ante structure (can add later as BettingPhase variant)
- Tournament mode (single session focus)

## Success Criteria

1. BettingPhase can be added to any genome's turn structure
2. Evolution can mutate betting parameters
3. AI players make legal betting decisions
4. Short-stack players can go all-in instead of forced fold
5. Betting rounds terminate correctly (no infinite loops)
6. Tied hands split the pot fairly
7. Position rotates each hand for fairness
8. Genomes with betting save/load correctly

## Multi-Agent Review Feedback (Addressed)

### High Priority (Fixed)

| Issue | Resolution |
|-------|------------|
| All-In handling missing | Added ALL_IN action and IsAllIn state |
| Betting round termination undefined | Added explicit state machine with termination conditions |
| Split pot handling absent | Changed to return []int winners, divide pot evenly |

### Medium Priority (Fixed)

| Issue | Resolution |
|-------|------------|
| max_bet unused | Removed from design |
| mandatory unused | Removed from design |
| Position rotation missing | Added BettingStartPlayer with rotation |
| Chip initialization missing | Added explicit InitializeChips and ResetHand |
| Testing strategy absent | Added comprehensive Testing Strategy section |

### Weak (Noted for Future)

| Issue | Status |
|-------|--------|
| Blinds/Antes missing | Out of scope - can add as BettingPhase variant later. Note: Without forced bets, rational agents may exploit CHECK-forever strategy. Evolution fitness doesn't require optimal play, so acceptable for now. |
| Parameter validation | Added mutation validation note. Mutations ensure `min_bet <= starting_chips`. |
