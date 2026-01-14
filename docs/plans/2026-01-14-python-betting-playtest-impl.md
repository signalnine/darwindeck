# Python Betting Playtest Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add betting support to Python playtest so poker/blackjack games can be human-tested.

**Architecture:** Port Go's betting.go logic to Python's immutable state pattern. Add betting fields to state.py, move generation/application to movegen.py, and round handling to session.py.

**Tech Stack:** Python 3.13, pytest, dataclasses

---

## Task 1: Add Betting Fields to PlayerState

**Files:**
- Modify: `src/darwindeck/simulation/state.py`
- Test: `tests/unit/test_betting_moves.py` (new file)

**Step 1: Write the failing test**

Create `tests/unit/test_betting_moves.py`:

```python
"""Tests for betting move generation and application."""

import pytest
from darwindeck.simulation.state import PlayerState, Card
from darwindeck.genome.schema import Rank, Suit


class TestPlayerStateBetting:
    """Test PlayerState betting fields."""

    def test_player_state_has_chips(self):
        """PlayerState should have chips field."""
        player = PlayerState(
            player_id=0,
            hand=(),
            score=0,
            chips=500,
        )
        assert player.chips == 500

    def test_player_state_has_betting_flags(self):
        """PlayerState should have current_bet, has_folded, is_all_in."""
        player = PlayerState(
            player_id=0,
            hand=(),
            score=0,
            chips=500,
            current_bet=50,
            has_folded=False,
            is_all_in=False,
        )
        assert player.current_bet == 50
        assert player.has_folded is False
        assert player.is_all_in is False

    def test_player_state_betting_fields_default_to_zero(self):
        """Betting fields should default to 0/False for non-betting games."""
        player = PlayerState(player_id=0, hand=(), score=0)
        assert player.chips == 0
        assert player.current_bet == 0
        assert player.has_folded is False
        assert player.is_all_in is False
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestPlayerStateBetting -v`
Expected: FAIL with "unexpected keyword argument 'chips'"

**Step 3: Write minimal implementation**

Modify `src/darwindeck/simulation/state.py` PlayerState class (around line 20):

```python
@dataclass(frozen=True)
class PlayerState:
    """Immutable player state."""

    player_id: int
    hand: tuple[Card, ...]
    score: int
    # Betting fields (default to 0/False for non-betting games)
    chips: int = 0
    current_bet: int = 0
    has_folded: bool = False
    is_all_in: bool = False

    def copy_with(self, **changes) -> "PlayerState":
        """Create new PlayerState with changes."""
        current = {
            "player_id": self.player_id,
            "hand": self.hand,
            "score": self.score,
            "chips": self.chips,
            "current_bet": self.current_bet,
            "has_folded": self.has_folded,
            "is_all_in": self.is_all_in,
        }
        current.update(changes)
        return PlayerState(**current)
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestPlayerStateBetting -v`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/state.py tests/unit/test_betting_moves.py
git commit -m "feat(state): add betting fields to PlayerState"
```

---

## Task 2: Add Betting Fields to GameState

**Files:**
- Modify: `src/darwindeck/simulation/state.py`
- Test: `tests/unit/test_betting_moves.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_betting_moves.py`:

```python
from darwindeck.simulation.state import GameState


class TestGameStateBetting:
    """Test GameState betting fields."""

    def test_game_state_has_pot(self):
        """GameState should have pot field."""
        player = PlayerState(player_id=0, hand=(), score=0)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            pot=150,
        )
        assert state.pot == 150

    def test_game_state_has_betting_fields(self):
        """GameState should have current_bet and raise_count."""
        player = PlayerState(player_id=0, hand=(), score=0)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            pot=150,
            current_bet=50,
            raise_count=1,
        )
        assert state.current_bet == 50
        assert state.raise_count == 1

    def test_game_state_betting_fields_default_to_zero(self):
        """Betting fields should default to 0 for non-betting games."""
        player = PlayerState(player_id=0, hand=(), score=0)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
        )
        assert state.pot == 0
        assert state.current_bet == 0
        assert state.raise_count == 0
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestGameStateBetting -v`
Expected: FAIL with "unexpected keyword argument 'pot'"

**Step 3: Write minimal implementation**

Modify `src/darwindeck/simulation/state.py` GameState class (around line 38):

```python
@dataclass(frozen=True)
class GameState:
    """Immutable game state (hybrid design from consensus).

    Uses typed fields for common zones plus typed extensions.
    All nested structures are tuples for true immutability.
    """

    # Core state
    players: tuple[PlayerState, ...]
    deck: tuple[Card, ...]
    discard: tuple[Card, ...]
    turn: int
    active_player: int

    # Game-family specific (typed extensions, not Dict[str, Any])
    tableau: Optional[tuple[tuple[Card, ...], ...]] = None  # For solitaire-style
    community: Optional[tuple[Card, ...]] = None  # For poker-style

    # Betting fields (default to 0 for non-betting games)
    pot: int = 0
    current_bet: int = 0
    raise_count: int = 0

    def copy_with(self, **changes) -> "GameState":
        """Create a new state with specified changes."""
        current = {
            "players": self.players,
            "deck": self.deck,
            "discard": self.discard,
            "turn": self.turn,
            "active_player": self.active_player,
            "tableau": self.tableau,
            "community": self.community,
            "pot": self.pot,
            "current_bet": self.current_bet,
            "raise_count": self.raise_count,
        }
        current.update(changes)
        return GameState(**current)
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestGameStateBetting -v`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/state.py tests/unit/test_betting_moves.py
git commit -m "feat(state): add betting fields to GameState"
```

---

## Task 3: Add BettingAction Enum and BettingMove Dataclass

**Files:**
- Modify: `src/darwindeck/simulation/movegen.py`
- Test: `tests/unit/test_betting_moves.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_betting_moves.py`:

```python
from darwindeck.simulation.movegen import BettingAction, BettingMove


class TestBettingTypes:
    """Test BettingAction and BettingMove types."""

    def test_betting_action_enum_values(self):
        """BettingAction should have all poker actions."""
        assert BettingAction.CHECK.value == "check"
        assert BettingAction.BET.value == "bet"
        assert BettingAction.CALL.value == "call"
        assert BettingAction.RAISE.value == "raise"
        assert BettingAction.ALL_IN.value == "all_in"
        assert BettingAction.FOLD.value == "fold"

    def test_betting_move_dataclass(self):
        """BettingMove should hold action and phase_index."""
        move = BettingMove(action=BettingAction.BET, phase_index=0)
        assert move.action == BettingAction.BET
        assert move.phase_index == 0
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestBettingTypes -v`
Expected: FAIL with "cannot import name 'BettingAction'"

**Step 3: Write minimal implementation**

Add to `src/darwindeck/simulation/movegen.py` after imports (around line 6):

```python
from enum import Enum


class BettingAction(Enum):
    """Betting action types."""
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

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestBettingTypes -v`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/movegen.py tests/unit/test_betting_moves.py
git commit -m "feat(movegen): add BettingAction enum and BettingMove dataclass"
```

---

## Task 4: Generate Betting Moves - CHECK and BET

**Files:**
- Modify: `src/darwindeck/simulation/movegen.py`
- Test: `tests/unit/test_betting_moves.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_betting_moves.py`:

```python
from darwindeck.simulation.movegen import generate_betting_moves
from darwindeck.genome.schema import BettingPhase


class TestGenerateBettingMoves:
    """Test betting move generation."""

    def _make_player(self, chips: int, current_bet: int = 0, has_folded: bool = False, is_all_in: bool = False) -> PlayerState:
        return PlayerState(
            player_id=0,
            hand=(),
            score=0,
            chips=chips,
            current_bet=current_bet,
            has_folded=has_folded,
            is_all_in=is_all_in,
        )

    def _make_state(self, player: PlayerState, current_bet: int = 0, pot: int = 0, raise_count: int = 0) -> GameState:
        return GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            pot=pot,
            current_bet=current_bet,
            raise_count=raise_count,
        )

    def test_check_available_when_no_bet(self):
        """CHECK should be available when there's no current bet."""
        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=0)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.CHECK in actions

    def test_bet_available_when_can_afford(self):
        """BET should be available when player can afford min_bet."""
        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=0)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.BET in actions

    def test_bet_not_available_when_cannot_afford(self):
        """BET should not be available when player can't afford min_bet."""
        player = self._make_player(chips=5, current_bet=0)  # Less than min_bet
        state = self._make_state(player, current_bet=0)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.BET not in actions
        assert BettingAction.ALL_IN in actions  # Can still go all-in
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestGenerateBettingMoves -v -k "check or bet"`
Expected: FAIL with "cannot import name 'generate_betting_moves'"

**Step 3: Write minimal implementation**

Add to `src/darwindeck/simulation/movegen.py`:

```python
from darwindeck.genome.schema import GameGenome, PlayPhase, Location, BettingPhase


def generate_betting_moves(state: GameState, phase: BettingPhase, player_id: int) -> list[BettingMove]:
    """Generate all legal betting moves for a player.

    Mirrors Go's GenerateBettingMoves in betting.go.
    """
    player = state.players[player_id]
    moves: list[BettingMove] = []

    # Can't act if folded, all-in, or no chips
    if player.has_folded or player.is_all_in or player.chips <= 0:
        return moves

    to_call = state.current_bet - player.current_bet
    phase_index = 0  # Will be set by caller if needed

    if to_call == 0:
        # No bet to match
        moves.append(BettingMove(action=BettingAction.CHECK, phase_index=phase_index))
        if player.chips >= phase.min_bet:
            moves.append(BettingMove(action=BettingAction.BET, phase_index=phase_index))
        elif player.chips > 0:
            # Can't afford min bet, but can go all-in
            moves.append(BettingMove(action=BettingAction.ALL_IN, phase_index=phase_index))
    else:
        # Must match, raise, all-in, or fold (Task 5)
        pass

    return moves
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestGenerateBettingMoves -v -k "check or bet"`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/movegen.py tests/unit/test_betting_moves.py
git commit -m "feat(movegen): add generate_betting_moves for CHECK and BET"
```

---

## Task 5: Generate Betting Moves - CALL, RAISE, FOLD

**Files:**
- Modify: `src/darwindeck/simulation/movegen.py`
- Test: `tests/unit/test_betting_moves.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_betting_moves.py` TestGenerateBettingMoves class:

```python
    def test_call_available_when_facing_bet(self):
        """CALL should be available when there's a bet to match."""
        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.CALL in actions

    def test_raise_available_when_can_afford(self):
        """RAISE should be available when player can afford call + min_bet."""
        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=50, raise_count=0)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.RAISE in actions

    def test_raise_not_available_at_max_raises(self):
        """RAISE should not be available when max_raises reached."""
        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=50, raise_count=3)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.RAISE not in actions

    def test_fold_available_when_facing_bet(self):
        """FOLD should be available when there's a bet to match."""
        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.FOLD in actions

    def test_all_in_when_short_stacked(self):
        """ALL_IN should be available when can't afford call."""
        player = self._make_player(chips=30, current_bet=0)  # Can't afford 50 call
        state = self._make_state(player, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.ALL_IN in actions
        assert BettingAction.CALL not in actions  # Can't afford
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestGenerateBettingMoves -v -k "call or raise or fold or short"`
Expected: FAIL (CALL, RAISE, FOLD not in moves)

**Step 3: Write minimal implementation**

Update `generate_betting_moves` in `src/darwindeck/simulation/movegen.py`, replace the `else: pass` block:

```python
    else:
        # Must match, raise, all-in, or fold
        if player.chips >= to_call:
            moves.append(BettingMove(action=BettingAction.CALL, phase_index=phase_index))
            if player.chips >= to_call + phase.min_bet and state.raise_count < phase.max_raises:
                moves.append(BettingMove(action=BettingAction.RAISE, phase_index=phase_index))
        if player.chips > 0 and player.chips < to_call:
            # Can't afford call, but can go all-in
            moves.append(BettingMove(action=BettingAction.ALL_IN, phase_index=phase_index))
        moves.append(BettingMove(action=BettingAction.FOLD, phase_index=phase_index))
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestGenerateBettingMoves -v`
Expected: PASS (8 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/movegen.py tests/unit/test_betting_moves.py
git commit -m "feat(movegen): add CALL, RAISE, FOLD to betting move generation"
```

---

## Task 6: Apply Betting Move - CHECK and BET

**Files:**
- Modify: `src/darwindeck/simulation/movegen.py`
- Test: `tests/unit/test_betting_moves.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_betting_moves.py`:

```python
from darwindeck.simulation.movegen import apply_betting_move


class TestApplyBettingMove:
    """Test betting move application."""

    def _make_player(self, chips: int, current_bet: int = 0) -> PlayerState:
        return PlayerState(
            player_id=0,
            hand=(),
            score=0,
            chips=chips,
            current_bet=current_bet,
        )

    def _make_state(self, player: PlayerState, current_bet: int = 0, pot: int = 0) -> GameState:
        return GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            pot=pot,
            current_bet=current_bet,
        )

    def test_apply_check_no_change(self):
        """CHECK should not change state."""
        player = self._make_player(chips=500)
        state = self._make_state(player, pot=100)
        phase = BettingPhase(min_bet=10, max_raises=3)
        move = BettingMove(action=BettingAction.CHECK, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].chips == 500
        assert new_state.pot == 100

    def test_apply_bet_updates_chips_and_pot(self):
        """BET should decrease chips, increase pot, set current_bet."""
        player = self._make_player(chips=500)
        state = self._make_state(player, pot=0, current_bet=0)
        phase = BettingPhase(min_bet=50, max_raises=3)
        move = BettingMove(action=BettingAction.BET, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].chips == 450  # 500 - 50
        assert new_state.players[0].current_bet == 50
        assert new_state.pot == 50
        assert new_state.current_bet == 50
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestApplyBettingMove -v -k "check or bet"`
Expected: FAIL with "cannot import name 'apply_betting_move'"

**Step 3: Write minimal implementation**

Add to `src/darwindeck/simulation/movegen.py`:

```python
def apply_betting_move(state: GameState, move: BettingMove, phase: BettingPhase) -> GameState:
    """Apply a betting move to the state, returning new state.

    Mirrors Go's ApplyBettingAction in betting.go.
    """
    player = state.players[state.active_player]

    if move.action == BettingAction.CHECK:
        return state  # No change

    elif move.action == BettingAction.BET:
        new_player = player.copy_with(
            chips=player.chips - phase.min_bet,
            current_bet=phase.min_bet,
        )
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        return state.copy_with(
            players=new_players,
            pot=state.pot + phase.min_bet,
            current_bet=phase.min_bet,
        )

    # Other actions in Task 7
    return state


def _update_player_tuple(players: tuple[PlayerState, ...], idx: int, new_player: PlayerState) -> tuple[PlayerState, ...]:
    """Return new players tuple with updated player at idx."""
    return tuple(
        new_player if i == idx else p
        for i, p in enumerate(players)
    )
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestApplyBettingMove -v -k "check or bet"`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/movegen.py tests/unit/test_betting_moves.py
git commit -m "feat(movegen): add apply_betting_move for CHECK and BET"
```

---

## Task 7: Apply Betting Move - CALL, RAISE, ALL_IN, FOLD

**Files:**
- Modify: `src/darwindeck/simulation/movegen.py`
- Test: `tests/unit/test_betting_moves.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_betting_moves.py` TestApplyBettingMove class:

```python
    def test_apply_call_matches_bet(self):
        """CALL should match the current bet."""
        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, pot=50, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)
        move = BettingMove(action=BettingAction.CALL, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].chips == 450  # 500 - 50
        assert new_state.players[0].current_bet == 50
        assert new_state.pot == 100  # 50 + 50

    def test_apply_raise_increases_bet(self):
        """RAISE should call and add min_bet."""
        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, pot=50, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)
        move = BettingMove(action=BettingAction.RAISE, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].chips == 440  # 500 - 50 - 10
        assert new_state.players[0].current_bet == 60  # 50 + 10
        assert new_state.pot == 110  # 50 + 60
        assert new_state.current_bet == 60
        assert new_state.raise_count == 1

    def test_apply_all_in_bets_all_chips(self):
        """ALL_IN should bet all remaining chips."""
        player = self._make_player(chips=30, current_bet=0)
        state = self._make_state(player, pot=50, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)
        move = BettingMove(action=BettingAction.ALL_IN, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].chips == 0
        assert new_state.players[0].current_bet == 30
        assert new_state.players[0].is_all_in is True
        assert new_state.pot == 80  # 50 + 30

    def test_apply_fold_sets_flag(self):
        """FOLD should set has_folded flag."""
        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, pot=50, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)
        move = BettingMove(action=BettingAction.FOLD, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].has_folded is True
        assert new_state.players[0].chips == 500  # Unchanged
        assert new_state.pot == 50  # Unchanged
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestApplyBettingMove -v -k "call or raise or all_in or fold"`
Expected: FAIL (state unchanged for these actions)

**Step 3: Write minimal implementation**

Update `apply_betting_move` in `src/darwindeck/simulation/movegen.py`, add cases after BET:

```python
    elif move.action == BettingAction.CALL:
        to_call = state.current_bet - player.current_bet
        new_player = player.copy_with(
            chips=player.chips - to_call,
            current_bet=state.current_bet,
        )
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        return state.copy_with(
            players=new_players,
            pot=state.pot + to_call,
        )

    elif move.action == BettingAction.RAISE:
        to_call = state.current_bet - player.current_bet
        raise_amount = to_call + phase.min_bet
        new_current_bet = state.current_bet + phase.min_bet
        new_player = player.copy_with(
            chips=player.chips - raise_amount,
            current_bet=new_current_bet,
        )
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        return state.copy_with(
            players=new_players,
            pot=state.pot + raise_amount,
            current_bet=new_current_bet,
            raise_count=state.raise_count + 1,
        )

    elif move.action == BettingAction.ALL_IN:
        amount = player.chips
        new_current_bet = player.current_bet + amount
        new_player = player.copy_with(
            chips=0,
            current_bet=new_current_bet,
            is_all_in=True,
        )
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        game_current_bet = max(state.current_bet, new_current_bet)
        return state.copy_with(
            players=new_players,
            pot=state.pot + amount,
            current_bet=game_current_bet,
        )

    elif move.action == BettingAction.FOLD:
        new_player = player.copy_with(has_folded=True)
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        return state.copy_with(players=new_players)
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestApplyBettingMove -v`
Expected: PASS (6 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/movegen.py tests/unit/test_betting_moves.py
git commit -m "feat(movegen): add CALL, RAISE, ALL_IN, FOLD to apply_betting_move"
```

---

## Task 8: Add Betting Round Helpers

**Files:**
- Modify: `src/darwindeck/simulation/movegen.py`
- Test: `tests/unit/test_betting_moves.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_betting_moves.py`:

```python
from darwindeck.simulation.movegen import count_active_players, all_bets_matched


class TestBettingHelpers:
    """Test betting round helper functions."""

    def test_count_active_players_excludes_folded(self):
        """count_active_players should not count folded players."""
        p0 = PlayerState(player_id=0, hand=(), score=0, has_folded=False)
        p1 = PlayerState(player_id=1, hand=(), score=0, has_folded=True)
        state = GameState(
            players=(p0, p1),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
        )

        assert count_active_players(state) == 1

    def test_all_bets_matched_when_equal(self):
        """all_bets_matched should return True when all players match."""
        p0 = PlayerState(player_id=0, hand=(), score=0, current_bet=50)
        p1 = PlayerState(player_id=1, hand=(), score=0, current_bet=50)
        state = GameState(
            players=(p0, p1),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            current_bet=50,
        )

        assert all_bets_matched(state) is True

    def test_all_bets_matched_ignores_folded(self):
        """all_bets_matched should ignore folded players."""
        p0 = PlayerState(player_id=0, hand=(), score=0, current_bet=50)
        p1 = PlayerState(player_id=1, hand=(), score=0, current_bet=0, has_folded=True)
        state = GameState(
            players=(p0, p1),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            current_bet=50,
        )

        assert all_bets_matched(state) is True

    def test_all_bets_matched_ignores_all_in(self):
        """all_bets_matched should ignore all-in players."""
        p0 = PlayerState(player_id=0, hand=(), score=0, current_bet=50)
        p1 = PlayerState(player_id=1, hand=(), score=0, current_bet=30, is_all_in=True)
        state = GameState(
            players=(p0, p1),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            current_bet=50,
        )

        assert all_bets_matched(state) is True
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestBettingHelpers -v`
Expected: FAIL with "cannot import name 'count_active_players'"

**Step 3: Write minimal implementation**

Add to `src/darwindeck/simulation/movegen.py`:

```python
def count_active_players(state: GameState) -> int:
    """Count players who haven't folded."""
    return sum(1 for p in state.players if not p.has_folded)


def all_bets_matched(state: GameState) -> bool:
    """Check if all active players have matched the current bet."""
    for p in state.players:
        if not p.has_folded and not p.is_all_in:
            if p.current_bet != state.current_bet:
                return False
    return True
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestBettingHelpers -v`
Expected: PASS (4 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/movegen.py tests/unit/test_betting_moves.py
git commit -m "feat(movegen): add betting round helper functions"
```

---

## Task 9: Update Session to Initialize Chips

**Files:**
- Modify: `src/darwindeck/playtest/session.py`
- Test: `tests/unit/test_betting_moves.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_betting_moves.py`:

```python
from darwindeck.playtest.session import PlaytestSession, SessionConfig
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase
)


class TestSessionBettingInit:
    """Test session initializes betting state."""

    def test_session_initializes_chips(self):
        """Session should initialize player chips from genome."""
        genome = GameGenome(
            genome_id="test_betting",
            setup=SetupRules(cards_per_player=2, starting_chips=500),
            turn_structure=TurnStructure(
                phases=(BettingPhase(min_bet=10, max_raises=3),),
            ),
            win_conditions=(WinCondition(type="high_score"),),
            scoring_rules=(),
            player_count=2,
        )
        config = SessionConfig(seed=12345)
        session = PlaytestSession(genome, config)

        # Initialize state
        state = session._initialize_state()

        assert state.players[0].chips == 500
        assert state.players[1].chips == 500
        assert state.pot == 0
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestSessionBettingInit -v`
Expected: FAIL (chips == 0, not initialized)

**Step 3: Write minimal implementation**

Modify `_initialize_state` in `src/darwindeck/playtest/session.py` (around line 88):

```python
    def _initialize_state(self) -> GameState:
        """Initialize game state from genome."""
        # Create standard 52-card deck
        deck: list[Card] = []
        for suit in Suit:
            for rank in Rank:
                deck.append(Card(rank=rank, suit=suit))

        # Shuffle with session seed
        self.rng.shuffle(deck)

        # Deal to players
        cards_per_player = self.genome.setup.cards_per_player
        hands: list[tuple[Card, ...]] = []

        for i in range(self.genome.player_count):
            hand = tuple(deck[:cards_per_player])
            deck = deck[cards_per_player:]
            hands.append(hand)

        # Get starting chips (0 for non-betting games)
        starting_chips = self.genome.setup.starting_chips

        # Create player states
        players = tuple(
            PlayerState(
                player_id=i,
                hand=hand,
                score=0,
                chips=starting_chips,
            )
            for i, hand in enumerate(hands)
        )

        return GameState(
            players=players,
            deck=tuple(deck),
            discard=(),
            turn=1,
            active_player=0,
        )
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestSessionBettingInit -v`
Expected: PASS (1 test)

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/session.py tests/unit/test_betting_moves.py
git commit -m "feat(session): initialize chips from genome setup"
```

---

## Task 10: Update Display to Show Chips and Pot

**Files:**
- Modify: `src/darwindeck/playtest/display.py`
- Test: `tests/unit/test_betting_moves.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_betting_moves.py`:

```python
from darwindeck.playtest.display import StateRenderer


class TestDisplayBetting:
    """Test display shows betting info."""

    def test_render_shows_chips_when_nonzero(self):
        """StateRenderer should show chips when player has them."""
        player = PlayerState(player_id=0, hand=(), score=0, chips=500)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            pot=150,
        )
        genome = GameGenome(
            genome_id="test",
            setup=SetupRules(cards_per_player=2, starting_chips=500),
            turn_structure=TurnStructure(phases=()),
            win_conditions=(),
            scoring_rules=(),
            player_count=1,
        )
        renderer = StateRenderer()

        output = renderer.render(state, genome, human_player_idx=0, debug=False)

        assert "chips: 500" in output.lower() or "500" in output
        assert "pot: 150" in output.lower() or "150" in output
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestDisplayBetting -v`
Expected: FAIL (chips/pot not in output)

**Step 3: Read current display.py and modify**

First read the file to understand structure:

```bash
# Read file first, then modify
```

Add chips/pot display to `render` method in `src/darwindeck/playtest/display.py`. After showing hand, add:

```python
        # Show chips and pot if betting game
        if state.players[human_player_idx].chips > 0 or state.pot > 0:
            player_chips = state.players[human_player_idx].chips
            lines.append(f"Your chips: {player_chips} | Pot: {state.pot}")
            if state.current_bet > 0:
                lines.append(f"Current bet: {state.current_bet}")
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_betting_moves.py::TestDisplayBetting -v`
Expected: PASS (1 test)

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/display.py tests/unit/test_betting_moves.py
git commit -m "feat(display): show chips and pot for betting games"
```

---

## Task 11: Handle BettingPhase in Session Game Loop

**Files:**
- Modify: `src/darwindeck/playtest/session.py`
- Test: `tests/integration/test_betting_playtest.py` (new file)

**Step 1: Write the failing test**

Create `tests/integration/test_betting_playtest.py`:

```python
"""Integration tests for betting in playtest."""

import pytest
from unittest.mock import patch, MagicMock
from darwindeck.playtest.session import PlaytestSession, SessionConfig
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase, PlayPhase, Location
)


class TestBettingPlaytest:
    """Test betting games can be playtested."""

    def _make_betting_genome(self) -> GameGenome:
        """Create a simple betting game genome."""
        return GameGenome(
            genome_id="test_betting",
            setup=SetupRules(cards_per_player=2, starting_chips=500),
            turn_structure=TurnStructure(
                phases=(BettingPhase(min_bet=10, max_raises=3),),
            ),
            win_conditions=(WinCondition(type="high_score"),),
            scoring_rules=(),
            player_count=2,
        )

    def test_betting_moves_generated_for_betting_phase(self):
        """Session should generate betting moves for BettingPhase."""
        genome = self._make_betting_genome()
        config = SessionConfig(seed=12345)
        session = PlaytestSession(genome, config)
        session.state = session._initialize_state()

        # Import here to get the updated function
        from darwindeck.simulation.movegen import generate_legal_moves, BettingMove

        moves = generate_legal_moves(session.state, genome)

        # Should have betting moves, not empty
        assert len(moves) > 0
        assert all(isinstance(m, BettingMove) for m in moves)
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/integration/test_betting_playtest.py -v`
Expected: FAIL (generate_legal_moves returns empty list for BettingPhase)

**Step 3: Modify generate_legal_moves to handle BettingPhase**

Update `generate_legal_moves` in `src/darwindeck/simulation/movegen.py`:

```python
def generate_legal_moves(state: GameState, genome: GameGenome) -> List[LegalMove | BettingMove]:
    """Generate all legal moves for current player."""
    moves: List[LegalMove | BettingMove] = []
    current_player = state.active_player

    for phase_idx, phase in enumerate(genome.turn_structure.phases):
        if isinstance(phase, BettingPhase):
            # Generate betting moves
            betting_moves = generate_betting_moves(state, phase, current_player)
            # Set correct phase_index
            for bm in betting_moves:
                moves.append(BettingMove(action=bm.action, phase_index=phase_idx))

        elif isinstance(phase, PlayPhase):
            # PlayPhase: play cards from hand (existing logic)
            target = phase.target
            min_cards = phase.min_cards
            max_cards = phase.max_cards

            if min_cards <= 1 and max_cards >= 1:
                for card_idx in range(len(state.players[current_player].hand)):
                    moves.append(LegalMove(
                        phase_index=phase_idx,
                        card_index=card_idx,
                        target_loc=target
                    ))

    return moves
```

Also update the import at the top of movegen.py:

```python
from typing import List, Optional, Union
```

And the return type annotation.

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/integration/test_betting_playtest.py -v`
Expected: PASS (1 test)

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/movegen.py tests/integration/test_betting_playtest.py
git commit -m "feat(movegen): handle BettingPhase in generate_legal_moves"
```

---

## Task 12: Present Betting Moves to Human Player

**Files:**
- Modify: `src/darwindeck/playtest/display.py`
- Test: `tests/integration/test_betting_playtest.py`

**Step 1: Write the failing test**

Add to `tests/integration/test_betting_playtest.py`:

```python
from darwindeck.playtest.display import MovePresenter
from darwindeck.simulation.movegen import BettingMove, BettingAction


class TestBettingMovePresentation:
    """Test betting moves are presented correctly."""

    def test_present_betting_moves(self):
        """MovePresenter should format betting options."""
        moves = [
            BettingMove(action=BettingAction.CHECK, phase_index=0),
            BettingMove(action=BettingAction.BET, phase_index=0),
        ]
        player = PlayerState(player_id=0, hand=(), score=0, chips=500)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
        )
        genome = GameGenome(
            genome_id="test",
            setup=SetupRules(cards_per_player=2, starting_chips=500),
            turn_structure=TurnStructure(phases=(BettingPhase(min_bet=10, max_raises=3),)),
            win_conditions=(),
            scoring_rules=(),
            player_count=1,
        )
        presenter = MovePresenter()

        output = presenter.present(moves, state, genome)

        assert "check" in output.lower()
        assert "bet" in output.lower()
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/integration/test_betting_playtest.py::TestBettingMovePresentation -v`
Expected: FAIL (presenter doesn't handle BettingMove)

**Step 3: Modify MovePresenter in display.py**

Read `src/darwindeck/playtest/display.py` first, then add handling for BettingMove in the `present` method.

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/integration/test_betting_playtest.py::TestBettingMovePresentation -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/display.py tests/integration/test_betting_playtest.py
git commit -m "feat(display): present betting moves to human player"
```

---

## Task 13: Add AI Betting Strategy

**Files:**
- Modify: `src/darwindeck/playtest/session.py`
- Test: `tests/integration/test_betting_playtest.py`

**Step 1: Write the failing test**

Add to `tests/integration/test_betting_playtest.py`:

```python
class TestAIBetting:
    """Test AI betting strategy."""

    def test_ai_selects_betting_move(self):
        """AI should select a valid betting move."""
        from darwindeck.simulation.movegen import BettingMove, BettingAction

        genome = GameGenome(
            genome_id="test_betting",
            setup=SetupRules(cards_per_player=2, starting_chips=500),
            turn_structure=TurnStructure(
                phases=(BettingPhase(min_bet=10, max_raises=3),),
            ),
            win_conditions=(WinCondition(type="high_score"),),
            scoring_rules=(),
            player_count=2,
        )
        config = SessionConfig(seed=12345)
        session = PlaytestSession(genome, config)
        session.state = session._initialize_state()

        moves = [
            BettingMove(action=BettingAction.CHECK, phase_index=0),
            BettingMove(action=BettingAction.BET, phase_index=0),
        ]

        selected = session._ai_select_betting_move(moves)

        assert selected is not None
        assert isinstance(selected, BettingMove)
        assert selected in moves
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/integration/test_betting_playtest.py::TestAIBetting -v`
Expected: FAIL with "'PlaytestSession' object has no attribute '_ai_select_betting_move'"

**Step 3: Add _ai_select_betting_move to session.py**

Add method to PlaytestSession class:

```python
def _ai_select_betting_move(self, moves: list) -> Optional[Any]:
    """Select betting move using AI strategy."""
    from darwindeck.simulation.movegen import BettingMove, BettingAction

    if not moves:
        return None

    if self.config.difficulty == "random":
        return self.rng.choice(moves)

    # Greedy: simple hand-strength heuristic
    # Strong hand: prefer BET/RAISE
    # Weak hand: prefer CHECK/FOLD
    ai_player_idx = 1 - self.human_player_idx
    hand = self.state.players[ai_player_idx].hand

    # Simple strength: count high cards (10, J, Q, K, A)
    high_card_count = sum(
        1 for card in hand
        if card.rank.value in ('10', 'J', 'Q', 'K', 'A')
    )
    hand_strength = high_card_count / max(len(hand), 1)

    # Map strength to action preference
    action_priority = []
    if hand_strength > 0.5:
        action_priority = [BettingAction.RAISE, BettingAction.BET, BettingAction.CALL, BettingAction.CHECK]
    else:
        action_priority = [BettingAction.CHECK, BettingAction.CALL, BettingAction.FOLD]

    for preferred in action_priority:
        for move in moves:
            if move.action == preferred:
                return move

    # Fallback to first available
    return moves[0] if moves else None
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/integration/test_betting_playtest.py::TestAIBetting -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/session.py tests/integration/test_betting_playtest.py
git commit -m "feat(session): add AI betting strategy"
```

---

## Task 14: Run All Tests and Verify

**Files:**
- All modified files

**Step 1: Run full test suite**

```bash
uv run pytest tests/unit/test_betting_moves.py tests/integration/test_betting_playtest.py -v
```

Expected: All tests pass

**Step 2: Run existing playtest tests to check for regressions**

```bash
uv run pytest tests/ -k playtest -v
```

Expected: All tests pass

**Step 3: Manual smoke test**

```bash
./scripts/playtest.sh
# Select a betting game like blackjack
# Verify betting options appear
# Verify chips display
```

**Step 4: Final commit if any fixes needed**

```bash
git status
# If clean, no commit needed
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add betting fields to PlayerState | state.py, test_betting_moves.py |
| 2 | Add betting fields to GameState | state.py, test_betting_moves.py |
| 3 | Add BettingAction/BettingMove types | movegen.py, test_betting_moves.py |
| 4 | Generate CHECK and BET moves | movegen.py, test_betting_moves.py |
| 5 | Generate CALL, RAISE, FOLD moves | movegen.py, test_betting_moves.py |
| 6 | Apply CHECK and BET moves | movegen.py, test_betting_moves.py |
| 7 | Apply CALL, RAISE, ALL_IN, FOLD | movegen.py, test_betting_moves.py |
| 8 | Add betting round helpers | movegen.py, test_betting_moves.py |
| 9 | Initialize chips in session | session.py, test_betting_moves.py |
| 10 | Display chips and pot | display.py, test_betting_moves.py |
| 11 | Handle BettingPhase in movegen | movegen.py, test_betting_playtest.py |
| 12 | Present betting moves | display.py, test_betting_playtest.py |
| 13 | Add AI betting strategy | session.py, test_betting_playtest.py |
| 14 | Run all tests | - |

**Total: 14 tasks**
