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

    min_bid: int = 0          # Minimum bid allowed (0 for Nil)
    max_bid: int = 13         # Maximum bid (usually hand size)
    allow_nil: bool = True    # Allow bidding exactly 0 (Nil)
    allow_blind_nil: bool = False  # Bid nil before seeing cards (future)
    bid_order: str = "clockwise"   # "clockwise" or "dealer_last"
```

### Player State Extension

```python
# Add to PlayerState
current_bid: int = -1      # -1 = hasn't bid yet, 0 = Nil, 1-13 = trick bid
tricks_won: int = 0        # Track tricks won this hand
```

### Team Contract

- When `team_mode=True`, team contract = sum of partner bids (excluding Nils)
- Team must win at least their combined contract
- Individual Nil bids are scored separately (success/failure)

### Bid Actions

- `BID_N` - Bid N tricks (0-13)
- `BID_NIL` - Special nil bid (distinct from regular 0 for scoring purposes)

---

## 2. Contract Scoring

### Scoring Configuration

```python
@dataclass(frozen=True)
class ContractScoring:
    """Scoring rules for bid contracts."""

    points_per_trick_bid: int = 10     # Base points per bid trick
    overtrick_points: int = 1          # Points per trick over contract (bags)
    undertrick_penalty: int = 0        # Penalty per trick under
    failed_contract_penalty: int = 10  # Multiplier for failed contract

    # Nil scoring
    nil_bonus: int = 100               # Points for successful Nil
    nil_penalty: int = 100             # Penalty for failed Nil
    blind_nil_bonus: int = 200         # Points for successful Blind Nil
    blind_nil_penalty: int = 200       # Penalty for failed Blind Nil

    # Bag penalty (classic Spades rule)
    bag_limit: int = 10                # Accumulated overtricks before penalty
    bag_penalty: int = 100             # Penalty when bag limit reached
```

### Scoring Logic

1. **Made contract:** `bid × points_per_trick_bid + overtricks × overtrick_points`
2. **Failed contract:** `-bid × failed_contract_penalty`
3. **Nil success:** `+nil_bonus` (in addition to team contract)
4. **Nil failure:** `-nil_penalty` (team still needs to make their contract)

### Team Scoring

- Team contract = partner1_bid + partner2_bid (excluding Nils)
- Team tricks = partner1_tricks + partner2_tricks
- Nil bids scored individually, added to team total

---

## 3. Go Simulation Integration

### GameState Extensions

```go
// Add to PlayerState
CurrentBid    int8   // -1 = not bid, 0+ = bid amount
BidIsNil      bool   // True if this is a Nil bid
TricksWon     int8   // Tricks won this hand

// Add to GameState
BiddingComplete  bool     // True when all players have bid
TeamContracts    []int8   // Contract per team (sum of non-Nil bids)
```

### Bytecode Encoding

```
BiddingPhase: [opcode=60] [min_bid] [max_bid] [flags]
  flags byte: bit 0 = allow_nil, bit 1 = allow_blind_nil
```

### Move Generation

- During BiddingPhase, generate moves: `BID_0` through `BID_max_bid`
- If `allow_nil` and bid=0, mark as Nil bid
- AI strategies: Random bids uniformly, Greedy estimates based on high cards

### Phase Transition

- BiddingPhase completes when all players have bid
- Then proceeds to TrickPhase(s)
- At hand end, score based on contracts

### Contract Evaluation

```go
func EvaluateContracts(state *GameState) {
    // For each team:
    //   1. Sum tricks won by team members
    //   2. Compare to team contract
    //   3. Score Nil bids separately
    //   4. Apply points to TeamScores
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

class RemoveBiddingPhaseMutation(MutationOperator):
    """Remove BiddingPhase from genome."""

class MutateBiddingPhaseMutation(MutationOperator):
    """Tweak bidding parameters."""
    # Adjust min/max bid, toggle nil options

class MutateContractScoringMutation(MutationOperator):
    """Tweak contract scoring values."""
    # Adjust point values, penalties, bonuses
```

### Coherence Rules

- BiddingPhase requires at least one TrickPhase (contracts need tricks)
- ContractScoring requires BiddingPhase
- Add to `CleanupOrphanedResourcesMutation`: remove orphaned contract scoring

### Seed Game Updates

- Update `create_spades_genome()` with BiddingPhase
- Update `create_partnership_spades_genome()` with team contracts

---

## 5. Implementation Plan

### Task Order

1. Python Schema - BiddingPhase, ContractScoring dataclasses
2. Validator - Coherence rules (bidding requires tricks)
3. Bytecode - Encode/decode BiddingPhase
4. Go GameState - Add bid tracking fields
5. Go Bytecode - Parse BiddingPhase
6. Go MoveGen - Generate bid moves during BiddingPhase
7. Go Scoring - EvaluateContracts at hand end
8. FlatBuffers - Add bid fields to results
9. Mutation Operators - Add/remove/mutate bidding
10. Seed Games - Update Spades with real bidding
11. Integration Tests - Full pipeline test

### Testing Strategy

- Unit tests for each component
- Golden test: Spades genome with bidding compiles to expected bytecode
- Simulation test: Run 100 games, verify contracts are scored
- AI test: MCTS should outperform Random more with bidding (estimation skill)

### Out of Scope (YAGNI)

- Bridge-style suit bidding (future extension)
- Passing cards between partners
- Blind Nil implementation (start with regular Nil only)

---

## 6. Success Criteria

1. Spades with bidding runs end-to-end through Go simulation
2. Team contracts score correctly (made/failed)
3. Nil bids work (bonus/penalty applied)
4. Evolution can add/remove bidding from games
5. MCTS shows skill advantage over Random in bidding games
