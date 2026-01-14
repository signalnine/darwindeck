# Python Betting Playtest Design

**Date:** 2026-01-14
**Status:** Draft
**Goal:** Add betting support to Python playtest so poker/blackjack games can be human-tested

## Problem Statement

Python playtest uses `movegen.py` which only handles `PlayPhase`. Games with `BettingPhase` (blackjack, poker) show "No legal moves available" and get stuck. The Go simulation has full betting support in `betting.go`, but playtest uses Python.

## Solution: Port Betting Logic to Python

Mirror Go's betting implementation in Python's immutable state pattern.

## Section 1: State Extensions

Add betting fields to existing dataclasses with defaults so non-betting games work unchanged.

**PlayerState additions (`state.py`):**
```python
@dataclass(frozen=True)
class PlayerState:
    player_id: int
    hand: tuple[Card, ...]
    score: int
    # New betting fields
    chips: int = 0
    current_bet: int = 0
    has_folded: bool = False
    is_all_in: bool = False
```

**GameState additions (`state.py`):**
```python
@dataclass(frozen=True)
class GameState:
    # ... existing fields ...
    # New betting fields
    pot: int = 0
    current_bet: int = 0
    raise_count: int = 0
```

Session initializes chips from `genome.setup.starting_chips`.

## Section 2: Betting Move Generation

New types and generation logic in `movegen.py`:

```python
from enum import Enum

class BettingAction(Enum):
    CHECK = "check"
    BET = "bet"
    CALL = "call"
    RAISE = "raise"
    ALL_IN = "all_in"
    FOLD = "fold"

@dataclass(frozen=True)
class BettingMove:
    """A betting action (separate from card play moves)."""
    action: BettingAction
    phase_index: int
```

**Generation rules (mirrors Go's `GenerateBettingMoves`):**
- If `to_call == 0`: CHECK, BET (if afford min_bet), or ALL_IN
- If `to_call > 0`: CALL, RAISE (if afford and under max_raises), ALL_IN (short stack), FOLD
- Can't act if folded, all-in, or no chips

## Section 3: Applying Betting Actions

New `apply_betting_move()` using immutable state transitions:

```python
def apply_betting_move(state: GameState, move: BettingMove, genome: GameGenome) -> GameState:
    player = state.players[state.active_player]
    phase = genome.turn_structure.phases[move.phase_index]
    min_bet = phase.min_bet

    if move.action == BettingAction.CHECK:
        return state  # No change

    elif move.action == BettingAction.BET:
        new_player = player.copy_with(
            chips=player.chips - min_bet,
            current_bet=min_bet
        )
        return _update_player(state, new_player).copy_with(
            pot=state.pot + min_bet,
            current_bet=min_bet
        )

    elif move.action == BettingAction.FOLD:
        new_player = player.copy_with(has_folded=True)
        return _update_player(state, new_player)

    # CALL, RAISE, ALL_IN follow same pattern
```

## Section 4: Playtest Session Integration

**Display changes (`display.py`):**
```
Your chips: 500 | Pot: 150 | Current bet: 50
```

**Move presentation:**
```
Your betting options:
[1] Check
[2] Bet (50)
[3] All-In (500)
```

**Round flow in `session.py`:**
1. Detect if current phase is `BettingPhase`
2. Loop betting actions until round terminates
3. Advance to next phase or resolve showdown

**AI betting:** Hand-strength heuristic - strong hands raise, weak hands check/fold.

**Round termination:**
1. Only one player hasn't folded → they win pot
2. All active players matched current bet → next phase

## Section 5: Testing Strategy

**Unit tests (`tests/unit/test_betting_moves.py`):**
- `test_check_available_when_no_bet`
- `test_fold_always_available_when_facing_bet`
- `test_raise_respects_max_raises`
- `test_all_in_available_when_short_stacked`
- `test_apply_bet_updates_state`
- `test_apply_fold_sets_flag`
- `test_round_terminates_when_bets_matched`

**Integration tests (`tests/integration/test_betting_playtest.py`):**
- `test_blackjack_genome_playtestable`
- `test_betting_round_completes`
- `test_pot_awarded_on_fold`

**Regression:** Existing non-betting games continue to work.

## Files Modified

| File | Change |
|------|--------|
| `src/darwindeck/simulation/state.py` | Add betting fields to PlayerState/GameState |
| `src/darwindeck/simulation/movegen.py` | Add BettingAction, BettingMove, generation/apply logic |
| `src/darwindeck/playtest/session.py` | Handle betting rounds, AI betting |
| `src/darwindeck/playtest/display.py` | Show chips/pot |
| `tests/unit/test_betting_moves.py` | New unit tests |
| `tests/integration/test_betting_playtest.py` | New integration tests |
