# Bidding System Design

## Overview

Add Spades-style bidding to DarwinDeck, enabling contract-based trick-taking games where players declare how many tricks they expect to win.

**Scope:** Spades-style integer bidding with team contracts and Nil bids.

**Use cases:** Spades with bidding, Partnership Spades, future Bridge/Pinochle extensions.

---

## 1. BiddingPhase Schema

### Core Data Structure

```python
@dataclass(frozen=True)
class BiddingPhase:
    """Phase where players declare their contract (expected tricks)."""

    min_bid: int = 1          # Minimum bid allowed (1 = no Nil, 0 = allow Nil)
    max_bid: int = 13         # Maximum bid (validated against hand size at runtime)
    allow_nil: bool = True    # Allow bidding exactly 0 (Nil) - distinct from min_bid=0
```

**Removed (YAGNI):**
- `allow_blind_nil` - Deferred to future extension
- `bid_order` - Always clockwise, no configuration needed

### Player State Extension

```python
# Add to PlayerState (per-hand, reset each hand)
current_bid: int = -1      # -1 = hasn't bid yet, 0+ = bid amount
is_nil_bid: bool = False   # True if player bid Nil (separate from bid=0)
tricks_won: int = 0        # Tricks won this hand (reset each hand)
```

**Nil Representation (STRONG fix):**
- Nil is represented by `current_bid=0` AND `is_nil_bid=True`
- Regular zero bid (if min_bid=0, allow_nil=False) is `current_bid=0` AND `is_nil_bid=False`
- Single source of truth: the `is_nil_bid` flag determines Nil status

### Team Contract

- When `team_mode=True`, team contract = sum of partner bids (excluding Nils)
- Team must win at least their combined contract
- Individual Nil bids are scored separately (success/failure)

### Bid Actions

Single action type with validation:
- `BID(n)` - Bid n tricks where `min_bid <= n <= min(max_bid, hand_size)`
- When `allow_nil=True` and `n=0`, automatically sets `is_nil_bid=True`

---

## 2. Contract Scoring

### Scoring Configuration

```python
@dataclass(frozen=True)
class ContractScoring:
    """Scoring rules for bid contracts. Encoded in bytecode."""

    points_per_trick_bid: int = 10     # Base points per bid trick
    overtrick_points: int = 1          # Points per trick over contract (bags)
    failed_contract_penalty: int = 10  # Multiplier for failed contract

    # Nil scoring
    nil_bonus: int = 100               # Points for successful Nil
    nil_penalty: int = 100             # Penalty for failed Nil

    # Bag penalty (classic Spades rule)
    bag_limit: int = 10                # Accumulated overtricks before penalty
    bag_penalty: int = 100             # Penalty when bag limit reached
```

**Removed (YAGNI):**
- `undertrick_penalty` - Redundant with `failed_contract_penalty`
- `blind_nil_bonus/penalty` - Blind Nil not implemented

### Persistent State for Bags (STRONG fix)

```python
# Add to GameState (persists across hands)
accumulated_bags: tuple[int, ...] = ()  # Bags per team (or per player if no teams)
```

```go
// Add to GameState
AccumulatedBags []int8  // Persists across hands, indexed by team (or player)
```

### Scoring Logic

1. **Made contract:** `bid × points_per_trick_bid + overtricks × overtrick_points`
   - Overtricks added to `accumulated_bags`
2. **Failed contract:** `-bid × failed_contract_penalty`
3. **Nil success:** `+nil_bonus` (in addition to team contract)
4. **Nil failure:** `-nil_penalty` (team still needs to make their contract)
5. **Bag overflow:** When `accumulated_bags >= bag_limit`, subtract `bag_penalty` and reset bags

### Team Scoring

- Team contract = partner1_bid + partner2_bid (excluding Nils)
- Team tricks = partner1_tricks + partner2_tricks
- Nil bids scored individually, added to team total
- Bags accumulated per team

---

## 3. Go Simulation Integration

### GameState Extensions

```go
// Add to PlayerState (reset each hand)
CurrentBid    int8   // -1 = not bid, 0+ = bid amount
IsNilBid      bool   // True if this is a Nil bid (STRONG fix: single flag)
TricksWon     int8   // Tricks won this hand

// Add to GameState
BiddingComplete  bool     // True when all players have bid
TeamContracts    []int8   // Contract per team (sum of non-Nil bids)
AccumulatedBags  []int8   // Bags per team, persists across hands (STRONG fix)
```

### Hand Reset Logic (MEDIUM fix)

At the start of each hand:
```go
func ResetHandState(state *GameState) {
    for i := range state.Players {
        state.Players[i].CurrentBid = -1
        state.Players[i].IsNilBid = false
        state.Players[i].TricksWon = 0
    }
    state.BiddingComplete = false
    state.TeamContracts = make([]int8, len(state.TeamScores))
    // Note: AccumulatedBags is NOT reset - persists across hands
}
```

### Bytecode Encoding (STRONG fix - includes scoring)

```
BiddingPhase: [opcode=60] [min_bid] [max_bid] [flags] [scoring_data...]
  flags byte: bit 0 = allow_nil

ContractScoring encoding (12 bytes):
  [points_per_trick_bid: uint8]     // 1 byte (0-255)
  [overtrick_points: uint8]          // 1 byte
  [failed_contract_penalty: uint8]   // 1 byte
  [nil_bonus: uint16]                // 2 bytes (0-65535)
  [nil_penalty: uint16]              // 2 bytes
  [bag_limit: uint8]                 // 1 byte
  [bag_penalty: uint16]              // 2 bytes
  [reserved: uint16]                 // 2 bytes for future use
```

### Move Generation (MEDIUM fix - validation)

```go
func GenerateBidMoves(state *GameState, phase *BiddingPhase) []Move {
    moves := []Move{}
    handSize := len(state.Players[state.ActivePlayer].Hand)

    // Validate max_bid against actual hand size
    effectiveMax := min(phase.MaxBid, handSize)

    // Generate valid bid range
    for bid := phase.MinBid; bid <= effectiveMax; bid++ {
        moves = append(moves, Move{Type: BID, Value: bid})
    }

    // Add Nil option if allowed and not already covered by min_bid=0
    if phase.AllowNil && phase.MinBid > 0 {
        moves = append(moves, Move{Type: BID, Value: 0, IsNil: true})
    }

    return moves
}
```

### AI Bidding Strategy

```go
// Greedy AI: Estimate tricks based on high cards
func EstimateTricks(hand []Card, trumpSuit Suit) int {
    estimate := 0
    for _, card := range hand {
        if card.Suit == trumpSuit {
            estimate++  // Each trump is ~1 trick
        } else if card.Rank >= QUEEN {
            estimate++  // High cards in side suits
        }
    }
    return min(estimate, len(hand))
}
```

### Phase Transition

- BiddingPhase completes when all players have bid (`BiddingComplete = true`)
- Calculate TeamContracts from individual bids
- Then proceeds to TrickPhase(s)
- At hand end, call `EvaluateContracts()`
- After scoring, call `ResetHandState()` for next hand

### Contract Evaluation

```go
func EvaluateContracts(state *GameState, scoring *ContractScoring) {
    for teamIdx := range state.TeamScores {
        contract := state.TeamContracts[teamIdx]
        tricksWon := sumTeamTricks(state, teamIdx)

        // Score Nil bids first
        for _, playerIdx := range getTeamPlayers(state, teamIdx) {
            if state.Players[playerIdx].IsNilBid {
                if state.Players[playerIdx].TricksWon == 0 {
                    state.TeamScores[teamIdx] += int32(scoring.NilBonus)
                } else {
                    state.TeamScores[teamIdx] -= int32(scoring.NilPenalty)
                }
            }
        }

        // Score team contract (non-Nil bids)
        if tricksWon >= int(contract) {
            // Made contract
            state.TeamScores[teamIdx] += int32(contract) * int32(scoring.PointsPerTrickBid)
            overtricks := tricksWon - int(contract)
            state.TeamScores[teamIdx] += int32(overtricks) * int32(scoring.OvertrickPoints)

            // Accumulate bags
            state.AccumulatedBags[teamIdx] += int8(overtricks)
            if state.AccumulatedBags[teamIdx] >= int8(scoring.BagLimit) {
                state.TeamScores[teamIdx] -= int32(scoring.BagPenalty)
                state.AccumulatedBags[teamIdx] -= int8(scoring.BagLimit)
            }
        } else {
            // Failed contract
            state.TeamScores[teamIdx] -= int32(contract) * int32(scoring.FailedContractPenalty)
        }
    }
}
```

---

## 4. Evolution & Mutation

### New Mutation Operators

```python
class AddBiddingPhaseMutation(MutationOperator):
    """Add BiddingPhase to trick-taking games."""
    # Only applies if game has TrickPhase but no BiddingPhase
    # Inserts BiddingPhase before first TrickPhase
    # Also adds default ContractScoring

class RemoveBiddingPhaseMutation(MutationOperator):
    """Remove BiddingPhase from genome."""
    # Also removes ContractScoring

class MutateBiddingPhaseMutation(MutationOperator):
    """Tweak bidding parameters."""
    # Adjust min_bid (0-3), max_bid (hand_size ± 2), toggle allow_nil

class MutateContractScoringMutation(MutationOperator):
    """Tweak contract scoring values."""
    # Adjust point values within reasonable ranges
```

### Coherence Rules

- BiddingPhase requires at least one TrickPhase (contracts need tricks)
- ContractScoring requires BiddingPhase
- Add to `CleanupOrphanedResourcesMutation`: remove orphaned ContractScoring

### Seed Game Updates

- Update `create_spades_genome()` with BiddingPhase and ContractScoring
- Update `create_partnership_spades_genome()` with team contracts

---

## 5. Implementation Plan

### Task Order

1. Python Schema - BiddingPhase, ContractScoring dataclasses
2. Validator - Coherence rules (bidding requires tricks)
3. Bytecode - Encode/decode BiddingPhase with ContractScoring
4. Go GameState - Add bid tracking fields + AccumulatedBags
5. Go Bytecode - Parse BiddingPhase and ContractScoring
6. Go MoveGen - Generate bid moves with validation
7. Go Scoring - EvaluateContracts with bag tracking
8. Go HandReset - ResetHandState between hands
9. FlatBuffers - Add bid fields to results
10. Mutation Operators - Add/remove/mutate bidding
11. Seed Games - Update Spades with real bidding
12. Integration Tests - Full pipeline test

### Testing Strategy

- Unit tests for each component
- Golden test: Spades genome with bidding compiles to expected bytecode
- Simulation test: Run 100 games, verify contracts are scored
- Bag accumulation test: Verify bags persist across hands and trigger penalty
- Nil bid test: Verify Nil success/failure scoring
- AI test: MCTS should outperform Random more with bidding (estimation skill)
- Invalid state tests: Bid > hand size rejected, min_bid enforced

### Out of Scope (YAGNI)

- Bridge-style suit bidding (future extension)
- Passing cards between partners
- Blind Nil implementation (future extension)
- Dealer-last bid order (always clockwise)

---

## 6. Success Criteria

1. Spades with bidding runs end-to-end through Go simulation
2. Team contracts score correctly (made/failed)
3. Nil bids work (bonus/penalty applied)
4. Bags accumulate across hands and trigger penalty at limit
5. Bid validation enforces min_bid and max_bid <= hand_size
6. Evolution can add/remove bidding from games
7. MCTS shows skill advantage over Random in bidding games

---

## 7. Design Review Changes

Based on multi-agent consensus review (Claude, Gemini, Codex):

### STRONG Issues Fixed
1. **Bag tracking** - Added `AccumulatedBags` to GameState, persists across hands
2. **Nil representation** - Single `IsNilBid` flag, no separate BID_NIL action
3. **Scoring pipeline** - ContractScoring encoded in bytecode, decoded in Go

### MEDIUM Issues Fixed
4. **Bid validation** - `max_bid` validated against `hand_size` at runtime
5. **bid_order removed** - YAGNI, always clockwise
6. **blind_nil removed** - YAGNI, deferred to future
7. **Hand reset documented** - `ResetHandState()` function specified
