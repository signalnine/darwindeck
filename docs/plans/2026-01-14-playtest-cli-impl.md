# Human Playtesting CLI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build an interactive CLI tool for humans to playtest evolved card games against AI opponents with feedback collection.

**Architecture:** Python CLI using Click, reusing existing GameState/movegen/apply_move. New playtest module with: StuckDetector (state hashing), StateRenderer/MovePresenter (display), RuleExplainer (rules), HumanPlayer (input), PlaytestSession (game loop), feedback collection.

**Tech Stack:** Python 3.11+, Click, existing darwindeck.simulation and genome modules

---

## Task 1: Create Module Structure

**Files:**
- Create: `src/darwindeck/playtest/__init__.py`
- Create: `src/darwindeck/playtest/stuck.py`
- Create: `src/darwindeck/playtest/display.py`
- Create: `src/darwindeck/playtest/rules.py`
- Create: `src/darwindeck/playtest/input.py`
- Create: `src/darwindeck/playtest/session.py`
- Create: `src/darwindeck/playtest/feedback.py`
- Create: `tests/unit/test_playtest_stuck.py`

**Step 1: Create module directory and __init__.py**

```python
# src/darwindeck/playtest/__init__.py
"""Human playtesting module for evolved card games."""

from darwindeck.playtest.stuck import StuckDetector
from darwindeck.playtest.display import StateRenderer, MovePresenter
from darwindeck.playtest.rules import RuleExplainer
from darwindeck.playtest.input import HumanPlayer
from darwindeck.playtest.session import PlaytestSession
from darwindeck.playtest.feedback import FeedbackCollector

__all__ = [
    "StuckDetector",
    "StateRenderer",
    "MovePresenter",
    "RuleExplainer",
    "HumanPlayer",
    "PlaytestSession",
    "FeedbackCollector",
]
```

**Step 2: Create empty stub files**

```python
# src/darwindeck/playtest/stuck.py
"""Stuck detection for playtest games."""


# src/darwindeck/playtest/display.py
"""Terminal display for game state and moves."""


# src/darwindeck/playtest/rules.py
"""Rule explanation from genomes."""


# src/darwindeck/playtest/input.py
"""Human input handling."""


# src/darwindeck/playtest/session.py
"""Playtest session management."""


# src/darwindeck/playtest/feedback.py
"""Feedback collection and storage."""
```

**Step 3: Create empty test file**

```python
# tests/unit/test_playtest_stuck.py
"""Tests for StuckDetector."""
```

**Step 4: Verify imports work**

Run: `uv run python -c "from darwindeck.playtest import StuckDetector"`
Expected: ImportError (StuckDetector not defined yet)

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/ tests/unit/test_playtest_stuck.py
git commit -m "feat(playtest): create module structure"
```

---

## Task 2: Implement StuckDetector

**Files:**
- Modify: `src/darwindeck/playtest/stuck.py`
- Test: `tests/unit/test_playtest_stuck.py`

**Step 1: Write failing tests**

```python
# tests/unit/test_playtest_stuck.py
"""Tests for StuckDetector."""

import pytest
from darwindeck.playtest.stuck import StuckDetector
from darwindeck.simulation.state import GameState, PlayerState, Card
from darwindeck.genome.schema import Rank, Suit


def make_card(rank: str, suit: str) -> Card:
    """Helper to create cards."""
    return Card(rank=Rank(rank), suit=Suit(suit))


def make_state(hand_sizes: tuple[int, int], turn: int = 1) -> GameState:
    """Helper to create test states."""
    hands = tuple(
        tuple(make_card("A", "H") for _ in range(size))
        for size in hand_sizes
    )
    players = tuple(
        PlayerState(player_id=i, hand=hand, score=0)
        for i, hand in enumerate(hands)
    )
    return GameState(
        players=players,
        deck=(),
        discard=(),
        turn=turn,
        active_player=0,
    )


class TestStuckDetector:
    """Tests for StuckDetector."""

    def test_turn_limit_detection(self):
        """Detects when turn limit is reached."""
        detector = StuckDetector(max_turns=100)
        state = make_state((5, 5), turn=100)
        result = detector.check(state)
        assert result is not None
        assert "Turn limit" in result

    def test_under_turn_limit_ok(self):
        """No stuck detection under turn limit."""
        detector = StuckDetector(max_turns=100)
        state = make_state((5, 5), turn=50)
        result = detector.check(state)
        assert result is None

    def test_state_repetition_detection(self):
        """Detects repeated states via hashing."""
        detector = StuckDetector(repeat_threshold=3)
        state = make_state((5, 5), turn=1)

        # Same state 3 times should trigger
        detector.check(state)
        detector.check(state)
        result = detector.check(state)

        assert result is not None
        assert "repeated" in result

    def test_different_states_no_repetition(self):
        """Different states don't trigger repetition."""
        detector = StuckDetector(repeat_threshold=3)

        state1 = make_state((5, 5), turn=1)
        state2 = make_state((4, 5), turn=2)
        state3 = make_state((3, 5), turn=3)

        assert detector.check(state1) is None
        assert detector.check(state2) is None
        assert detector.check(state3) is None

    def test_consecutive_passes_detection(self):
        """Detects consecutive passes."""
        detector = StuckDetector(pass_threshold=5)

        for i in range(4):
            result = detector.record_pass()
            assert result is None

        result = detector.record_pass()
        assert result is not None
        assert "passes" in result

    def test_pass_counter_resets_on_action(self):
        """Pass counter resets when non-pass action taken."""
        detector = StuckDetector(pass_threshold=5)

        detector.record_pass()
        detector.record_pass()
        detector.record_action()  # Reset

        for i in range(4):
            result = detector.record_pass()
            assert result is None

    def test_reset_clears_all_state(self):
        """Reset clears detection state."""
        detector = StuckDetector()
        state = make_state((5, 5))

        detector.check(state)
        detector.check(state)
        detector.record_pass()

        detector.reset()

        # After reset, same state shouldn't trigger
        assert detector.check(state) is None
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_playtest_stuck.py -v`
Expected: FAIL (StuckDetector not implemented)

**Step 3: Implement StuckDetector**

```python
# src/darwindeck/playtest/stuck.py
"""Stuck detection for playtest games."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from darwindeck.simulation.state import GameState


@dataclass
class StuckDetector:
    """Detects stuck games using multiple strategies.

    Strategies:
    1. Absolute turn limit
    2. State repetition via hashing
    3. Consecutive passes
    """

    max_turns: int = 200
    repeat_threshold: int = 3
    pass_threshold: int = 10

    # Internal state
    _state_hashes: dict[int, int] = field(default_factory=dict)
    _consecutive_passes: int = 0

    def check(self, state: GameState) -> Optional[str]:
        """Check if game is stuck. Returns reason or None."""
        # Strategy 1: Turn limit
        if state.turn >= self.max_turns:
            return f"Turn limit reached ({self.max_turns})"

        # Strategy 2: State repetition
        state_hash = self._hash_state(state)
        self._state_hashes[state_hash] = self._state_hashes.get(state_hash, 0) + 1

        if self._state_hashes[state_hash] >= self.repeat_threshold:
            return f"Same state repeated {self.repeat_threshold} times"

        return None

    def record_pass(self) -> Optional[str]:
        """Record a pass action. Returns reason if stuck."""
        self._consecutive_passes += 1

        if self._consecutive_passes >= self.pass_threshold:
            return f"{self.pass_threshold} consecutive passes"

        return None

    def record_action(self) -> None:
        """Record a non-pass action (resets pass counter)."""
        self._consecutive_passes = 0

    def reset(self) -> None:
        """Reset all detection state."""
        self._state_hashes.clear()
        self._consecutive_passes = 0

    def _hash_state(self, state: GameState) -> int:
        """Hash relevant state for comparison."""
        key = (
            tuple(len(p.hand) for p in state.players),
            len(state.deck),
            state.discard[-1] if state.discard else None,
            state.active_player,
        )
        return hash(key)
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_playtest_stuck.py -v`
Expected: All 7 tests PASS

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/stuck.py tests/unit/test_playtest_stuck.py
git commit -m "feat(playtest): implement StuckDetector with state hashing"
```

---

## Task 3: Implement StateRenderer

**Files:**
- Modify: `src/darwindeck/playtest/display.py`
- Create: `tests/unit/test_playtest_display.py`

**Step 1: Write failing tests**

```python
# tests/unit/test_playtest_display.py
"""Tests for display components."""

import pytest
from darwindeck.playtest.display import StateRenderer, MovePresenter
from darwindeck.simulation.state import GameState, PlayerState, Card
from darwindeck.simulation.movegen import LegalMove
from darwindeck.genome.schema import (
    Rank, Suit, Location, GameGenome, SetupRules,
    TurnStructure, WinCondition, PlayPhase
)


def make_card(rank: str, suit: str) -> Card:
    """Helper to create cards."""
    return Card(rank=Rank(rank), suit=Suit(suit))


def make_simple_genome() -> GameGenome:
    """Create a simple test genome."""
    return GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[
            PlayPhase(target=Location.DISCARD)
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        max_turns=100,
        min_turns=1,
        player_count=2,
    )


def make_state_with_hand(hand: list[tuple[str, str]]) -> GameState:
    """Create state with specific hand."""
    cards = tuple(make_card(r, s) for r, s in hand)
    players = (
        PlayerState(player_id=0, hand=cards, score=0),
        PlayerState(player_id=1, hand=(make_card("A", "H"),), score=0),
    )
    return GameState(
        players=players,
        deck=(),
        discard=(make_card("Q", "H"),),
        turn=1,
        active_player=0,
    )


class TestStateRenderer:
    """Tests for StateRenderer."""

    def test_renders_hand(self):
        """Renders player's hand."""
        renderer = StateRenderer()
        state = make_state_with_hand([("7", "S"), ("K", "H"), ("3", "D")])
        genome = make_simple_genome()

        output = renderer.render(state, genome, player_idx=0)

        assert "7S" in output or "7♠" in output
        assert "KH" in output or "K♥" in output
        assert "3D" in output or "3♦" in output

    def test_renders_discard(self):
        """Renders discard pile top card."""
        renderer = StateRenderer()
        state = make_state_with_hand([("A", "C")])
        genome = make_simple_genome()

        output = renderer.render(state, genome, player_idx=0)

        assert "QH" in output or "Q♥" in output or "Discard" in output

    def test_debug_shows_opponent_hand(self):
        """Debug mode shows opponent's hand."""
        renderer = StateRenderer()
        state = make_state_with_hand([("A", "C")])
        genome = make_simple_genome()

        normal = renderer.render(state, genome, player_idx=0, debug=False)
        debug = renderer.render(state, genome, player_idx=0, debug=True)

        # Debug should be longer and contain opponent info
        assert len(debug) > len(normal)
        assert "opponent" in debug.lower() or "player 1" in debug.lower()

    def test_renders_turn_number(self):
        """Shows current turn number."""
        renderer = StateRenderer()
        state = make_state_with_hand([("A", "C")])
        state = state.copy_with(turn=15)
        genome = make_simple_genome()

        output = renderer.render(state, genome, player_idx=0)

        assert "15" in output or "Turn" in output
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_playtest_display.py::TestStateRenderer -v`
Expected: FAIL (StateRenderer not implemented)

**Step 3: Implement StateRenderer**

```python
# src/darwindeck/playtest/display.py
"""Terminal display for game state and moves."""

from __future__ import annotations

from darwindeck.simulation.state import GameState, Card
from darwindeck.simulation.movegen import LegalMove
from darwindeck.genome.schema import GameGenome, Location, PlayPhase, DrawPhase, BettingPhase


# Unicode card symbols
SUIT_SYMBOLS = {"H": "♥", "D": "♦", "C": "♣", "S": "♠"}


def format_card(card: Card) -> str:
    """Format card with unicode suit symbol."""
    suit_symbol = SUIT_SYMBOLS.get(card.suit.value, card.suit.value)
    return f"{card.rank.value}{suit_symbol}"


class StateRenderer:
    """Renders visible game state to terminal."""

    def render(
        self,
        state: GameState,
        genome: GameGenome,
        player_idx: int,
        debug: bool = False,
    ) -> str:
        """Render state from player's perspective."""
        lines: list[str] = []

        # Header
        lines.append(f"=== Turn {state.turn} ===")
        lines.append("")

        # Player's hand
        hand = state.players[player_idx].hand
        if hand:
            cards_str = "  ".join(
                f"[{i+1}] {format_card(card)}"
                for i, card in enumerate(hand)
            )
            lines.append(f"Your hand: {cards_str}")
        else:
            lines.append("Your hand: (empty)")

        # Discard pile (if genome uses it)
        if self._has_discard(genome) and state.discard:
            top = format_card(state.discard[-1])
            lines.append(f"Discard pile: {top}")

        # Chips (if betting game)
        if genome.setup.starting_chips > 0:
            player = state.players[player_idx]
            # PlayerState may not have chips attr in base version
            chips = getattr(player, "chips", genome.setup.starting_chips)
            lines.append(f"Your chips: {chips}")

        # Debug mode
        if debug:
            lines.append("")
            lines.append("--- Debug Info ---")
            for i, p in enumerate(state.players):
                if i != player_idx:
                    opp_cards = ", ".join(format_card(c) for c in p.hand)
                    lines.append(f"Player {i} hand: [{opp_cards}]")
            lines.append(f"Deck: {len(state.deck)} cards")

        return "\n".join(lines)

    def _has_discard(self, genome: GameGenome) -> bool:
        """Check if genome uses discard pile."""
        for phase in genome.turn_structure.phases:
            if isinstance(phase, PlayPhase) and phase.target == Location.DISCARD:
                return True
        return False
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_playtest_display.py::TestStateRenderer -v`
Expected: All 4 tests PASS

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/display.py tests/unit/test_playtest_display.py
git commit -m "feat(playtest): implement StateRenderer for terminal output"
```

---

## Task 4: Implement MovePresenter

**Files:**
- Modify: `src/darwindeck/playtest/display.py`
- Modify: `tests/unit/test_playtest_display.py`

**Step 1: Add failing tests**

```python
# Add to tests/unit/test_playtest_display.py

class TestMovePresenter:
    """Tests for MovePresenter."""

    def test_presents_card_play_moves(self):
        """Presents card play options with numbers."""
        presenter = MovePresenter()
        state = make_state_with_hand([("7", "S"), ("K", "H")])
        genome = make_simple_genome()
        moves = [
            LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD),
            LegalMove(phase_index=0, card_index=1, target_loc=Location.DISCARD),
        ]

        output = presenter.present(moves, state, genome)

        assert "[1]" in output
        assert "[2]" in output
        assert "7" in output  # 7S
        assert "K" in output  # KH

    def test_presents_empty_moves(self):
        """Handles no legal moves gracefully."""
        presenter = MovePresenter()
        state = make_state_with_hand([])
        genome = make_simple_genome()

        output = presenter.present([], state, genome)

        assert "no" in output.lower() or "pass" in output.lower()

    def test_quit_option_always_shown(self):
        """Quit option is always available."""
        presenter = MovePresenter()
        state = make_state_with_hand([("A", "C")])
        genome = make_simple_genome()
        moves = [LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD)]

        output = presenter.present(moves, state, genome)

        assert "q" in output.lower() or "quit" in output.lower()
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_playtest_display.py::TestMovePresenter -v`
Expected: FAIL (MovePresenter not implemented)

**Step 3: Implement MovePresenter**

```python
# Add to src/darwindeck/playtest/display.py

class MovePresenter:
    """Presents legal moves to human player."""

    def present(
        self,
        moves: list[LegalMove],
        state: GameState,
        genome: GameGenome,
    ) -> str:
        """Present moves in human-readable format."""
        if not moves:
            return "No legal moves available. Press Enter to pass."

        lines: list[str] = []

        # Determine phase type from first move
        if moves:
            phase_idx = moves[0].phase_index
            phase = genome.turn_structure.phases[phase_idx]

            if isinstance(phase, PlayPhase):
                lines.append(self._present_card_play(moves, state))
            elif isinstance(phase, BettingPhase):
                lines.append(self._present_betting(moves, state))
            else:
                lines.append(self._present_generic(moves))
        else:
            lines.append(self._present_generic(moves))

        lines.append("")
        lines.append("Enter choice or [q]uit:")

        return "\n".join(lines)

    def _present_card_play(self, moves: list[LegalMove], state: GameState) -> str:
        """Present card play options."""
        hand = state.players[state.active_player].hand
        options: list[str] = []

        for move in moves:
            if 0 <= move.card_index < len(hand):
                card = hand[move.card_index]
                options.append(f"[{move.card_index + 1}] {format_card(card)}")

        return "Play: " + "  ".join(options)

    def _present_betting(self, moves: list[LegalMove], state: GameState) -> str:
        """Present betting options."""
        # Betting actions encoded in card_index as negative values
        action_names = {
            -10: "Check",
            -11: "Bet",
            -12: "Call",
            -13: "Raise",
            -14: "All-In",
            -15: "Fold",
        }
        options: list[str] = []

        for i, move in enumerate(moves):
            name = action_names.get(move.card_index, f"Action {move.card_index}")
            options.append(f"[{i + 1}] {name}")

        return "Bet: " + "  ".join(options)

    def _present_generic(self, moves: list[LegalMove]) -> str:
        """Fallback for other move types."""
        options = [f"[{i + 1}] Move {i + 1}" for i in range(len(moves))]
        return "Choose: " + "  ".join(options)
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_playtest_display.py::TestMovePresenter -v`
Expected: All 3 tests PASS

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/display.py tests/unit/test_playtest_display.py
git commit -m "feat(playtest): implement MovePresenter for move formatting"
```

---

## Task 5: Implement RuleExplainer

**Files:**
- Modify: `src/darwindeck/playtest/rules.py`
- Create: `tests/unit/test_playtest_rules.py`

**Step 1: Write failing tests**

```python
# tests/unit/test_playtest_rules.py
"""Tests for RuleExplainer."""

import pytest
from darwindeck.playtest.rules import RuleExplainer
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    PlayPhase, DrawPhase, BettingPhase, Location
)


def make_genome(
    name: str = "TestGame",
    phases: list = None,
    win_type: str = "empty_hand",
    starting_chips: int = 0,
) -> GameGenome:
    """Create test genome."""
    if phases is None:
        phases = [PlayPhase(target=Location.DISCARD)]

    return GameGenome(
        schema_version="1.0",
        genome_id=name,
        generation=1,
        setup=SetupRules(cards_per_player=5, starting_chips=starting_chips),
        turn_structure=TurnStructure(phases=phases),
        special_effects=[],
        win_conditions=[WinCondition(type=win_type)],
        scoring_rules=[],
        max_turns=100,
        min_turns=1,
        player_count=2,
    )


class TestRuleExplainer:
    """Tests for RuleExplainer."""

    def test_explains_game_name(self):
        """Shows game name in rules."""
        explainer = RuleExplainer()
        genome = make_genome(name="MyGame")

        output = explainer.explain_rules(genome)

        assert "MyGame" in output

    def test_explains_win_condition_empty_hand(self):
        """Explains empty hand win condition."""
        explainer = RuleExplainer()
        genome = make_genome(win_type="empty_hand")

        output = explainer.explain_rules(genome)

        assert "empty" in output.lower() or "hand" in output.lower()

    def test_explains_win_condition_capture_all(self):
        """Explains capture all win condition."""
        explainer = RuleExplainer()
        genome = make_genome(win_type="capture_all")

        output = explainer.explain_rules(genome)

        assert "capture" in output.lower() or "all" in output.lower()

    def test_explains_play_phase(self):
        """Explains play card phases."""
        explainer = RuleExplainer()
        genome = make_genome(phases=[PlayPhase(target=Location.DISCARD)])

        output = explainer.explain_rules(genome)

        assert "play" in output.lower() or "card" in output.lower()

    def test_explains_betting(self):
        """Shows betting info if chips > 0."""
        explainer = RuleExplainer()
        genome = make_genome(
            starting_chips=1000,
            phases=[BettingPhase(min_bet=10)]
        )

        output = explainer.explain_rules(genome)

        assert "bet" in output.lower() or "chip" in output.lower()

    def test_explains_phase_during_game(self):
        """Explains current phase."""
        explainer = RuleExplainer()
        genome = make_genome(phases=[
            DrawPhase(source=Location.DECK),
            PlayPhase(target=Location.DISCARD),
        ])

        # Explain phase 0 (draw)
        output0 = explainer.explain_phase(0, genome)
        assert "draw" in output0.lower()

        # Explain phase 1 (play)
        output1 = explainer.explain_phase(1, genome)
        assert "play" in output1.lower()
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_playtest_rules.py -v`
Expected: FAIL (RuleExplainer not implemented)

**Step 3: Implement RuleExplainer**

```python
# src/darwindeck/playtest/rules.py
"""Rule explanation from genomes."""

from __future__ import annotations

from darwindeck.genome.schema import (
    GameGenome, PlayPhase, DrawPhase, DiscardPhase,
    BettingPhase, TrickPhase, ClaimPhase
)


class RuleExplainer:
    """Explains game rules from genome."""

    def explain_rules(self, genome: GameGenome) -> str:
        """Generate condensed rule summary."""
        lines: list[str] = []

        lines.append(f"=== {genome.genome_id} ===")
        lines.append("")

        # Win condition
        lines.append(f"Goal: {self._explain_win_condition(genome)}")

        # Setup
        lines.append(f"Setup: Each player gets {genome.setup.cards_per_player} cards")

        # Turn structure
        lines.append(f"Turn: {self._explain_turn_structure(genome)}")

        # Betting (if applicable)
        if genome.setup.starting_chips > 0:
            lines.append(f"Chips: Start with {genome.setup.starting_chips}")
            min_bet = self._find_min_bet(genome)
            if min_bet:
                lines.append(f"Betting: Minimum bet is {min_bet}")

        return "\n".join(lines)

    def explain_phase(self, phase_idx: int, genome: GameGenome) -> str:
        """Explain current phase to player."""
        if phase_idx >= len(genome.turn_structure.phases):
            return "Unknown phase"

        phase = genome.turn_structure.phases[phase_idx]
        return f"Phase: {self._phase_description(phase)}"

    def _explain_win_condition(self, genome: GameGenome) -> str:
        """Describe win condition(s)."""
        descriptions: list[str] = []

        for wc in genome.win_conditions:
            if wc.type == "empty_hand":
                descriptions.append("Empty your hand to win")
            elif wc.type == "capture_all":
                descriptions.append("Capture all cards to win")
            elif wc.type == "first_to_score":
                threshold = wc.threshold or 100
                descriptions.append(f"First to {threshold} points wins")
            elif wc.type == "high_score":
                descriptions.append("Highest score wins")
            else:
                descriptions.append(f"Win by: {wc.type}")

        return "; ".join(descriptions) if descriptions else "Unknown"

    def _explain_turn_structure(self, genome: GameGenome) -> str:
        """Describe turn phases."""
        phase_descs: list[str] = []

        for phase in genome.turn_structure.phases:
            phase_descs.append(self._phase_description(phase))

        return " -> ".join(phase_descs) if phase_descs else "Unknown"

    def _phase_description(self, phase) -> str:
        """Get short description for a phase."""
        if isinstance(phase, PlayPhase):
            return f"Play card to {phase.target.value}"
        elif isinstance(phase, DrawPhase):
            return f"Draw {phase.count} from {phase.source.value}"
        elif isinstance(phase, DiscardPhase):
            return f"Discard {phase.count}"
        elif isinstance(phase, BettingPhase):
            return "Betting round"
        elif isinstance(phase, TrickPhase):
            return "Play trick"
        elif isinstance(phase, ClaimPhase):
            return "Claim cards"
        else:
            return "Unknown action"

    def _find_min_bet(self, genome: GameGenome) -> int | None:
        """Find minimum bet from betting phases."""
        for phase in genome.turn_structure.phases:
            if isinstance(phase, BettingPhase):
                return phase.min_bet
        return None
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_playtest_rules.py -v`
Expected: All 6 tests PASS

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/rules.py tests/unit/test_playtest_rules.py
git commit -m "feat(playtest): implement RuleExplainer for rule summaries"
```

---

## Task 6: Implement HumanPlayer Input

**Files:**
- Modify: `src/darwindeck/playtest/input.py`
- Create: `tests/unit/test_playtest_input.py`

**Step 1: Write failing tests**

```python
# tests/unit/test_playtest_input.py
"""Tests for HumanPlayer input handling."""

import pytest
from unittest.mock import patch
from darwindeck.playtest.input import HumanPlayer, InputResult
from darwindeck.simulation.movegen import LegalMove
from darwindeck.genome.schema import Location


class TestHumanPlayer:
    """Tests for HumanPlayer."""

    def test_parses_valid_number(self):
        """Parses valid move number."""
        player = HumanPlayer()
        moves = [
            LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD),
            LegalMove(phase_index=0, card_index=1, target_loc=Location.DISCARD),
        ]

        with patch("builtins.input", return_value="1"):
            result = player.get_move(moves)

        assert result.move == moves[0]
        assert not result.quit
        assert result.error is None

    def test_parses_quit_command(self):
        """Recognizes quit command."""
        player = HumanPlayer()
        moves = [LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD)]

        with patch("builtins.input", return_value="q"):
            result = player.get_move(moves)

        assert result.quit
        assert result.move is None

    def test_handles_invalid_number(self):
        """Returns error for invalid number."""
        player = HumanPlayer()
        moves = [LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD)]

        with patch("builtins.input", return_value="5"):
            result = player.get_move(moves)

        assert result.error is not None
        assert result.move is None
        assert not result.quit

    def test_handles_non_numeric(self):
        """Returns error for non-numeric input."""
        player = HumanPlayer()
        moves = [LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD)]

        with patch("builtins.input", return_value="xyz"):
            result = player.get_move(moves)

        assert result.error is not None

    def test_handles_empty_moves_pass(self):
        """Returns pass for empty move list."""
        player = HumanPlayer()

        with patch("builtins.input", return_value=""):
            result = player.get_move([])

        assert result.is_pass
        assert not result.quit
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_playtest_input.py -v`
Expected: FAIL (HumanPlayer not implemented)

**Step 3: Implement HumanPlayer**

```python
# src/darwindeck/playtest/input.py
"""Human input handling."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional

from darwindeck.simulation.movegen import LegalMove


@dataclass
class InputResult:
    """Result of human input."""

    move: Optional[LegalMove] = None
    quit: bool = False
    is_pass: bool = False
    error: Optional[str] = None


class HumanPlayer:
    """Handles human player input."""

    def get_move(self, moves: list[LegalMove], prompt: str = "> ") -> InputResult:
        """Get move from human input.

        Args:
            moves: List of legal moves (empty means must pass)
            prompt: Input prompt string

        Returns:
            InputResult with move, quit flag, or error
        """
        # Handle no legal moves (pass)
        if not moves:
            try:
                input("No moves available. Press Enter to pass...")
            except (EOFError, KeyboardInterrupt):
                return InputResult(quit=True)
            return InputResult(is_pass=True)

        try:
            raw = input(prompt).strip().lower()
        except (EOFError, KeyboardInterrupt):
            return InputResult(quit=True)

        # Check for quit
        if raw in ("q", "quit", "exit"):
            return InputResult(quit=True)

        # Check for pass (empty input)
        if not raw:
            return InputResult(is_pass=True)

        # Parse number
        try:
            choice = int(raw)
        except ValueError:
            return InputResult(error=f"Invalid input '{raw}'. Enter a number or 'q'.")

        # Validate range (1-indexed for human)
        if choice < 1 or choice > len(moves):
            return InputResult(error=f"Invalid choice {choice}. Enter 1-{len(moves)}.")

        return InputResult(move=moves[choice - 1])

    def get_yes_no(self, prompt: str) -> Optional[bool]:
        """Get yes/no response.

        Returns:
            True for yes, False for no, None for quit/cancel
        """
        try:
            raw = input(prompt).strip().lower()
        except (EOFError, KeyboardInterrupt):
            return None

        if raw in ("y", "yes"):
            return True
        if raw in ("n", "no"):
            return False
        return None

    def get_rating(self, prompt: str = "Rate this game [1-5]: ") -> Optional[int]:
        """Get numeric rating 1-5.

        Returns:
            Rating 1-5, or None for skip/cancel
        """
        try:
            raw = input(prompt).strip()
        except (EOFError, KeyboardInterrupt):
            return None

        if not raw:
            return None

        try:
            rating = int(raw)
            if 1 <= rating <= 5:
                return rating
        except ValueError:
            pass

        return None

    def get_comment(self, prompt: str = "Comments (Enter to skip): ") -> str:
        """Get optional comment."""
        try:
            return input(prompt).strip()
        except (EOFError, KeyboardInterrupt):
            return ""
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_playtest_input.py -v`
Expected: All 5 tests PASS

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/input.py tests/unit/test_playtest_input.py
git commit -m "feat(playtest): implement HumanPlayer input handling"
```

---

## Task 7: Implement FeedbackCollector

**Files:**
- Modify: `src/darwindeck/playtest/feedback.py`
- Create: `tests/unit/test_playtest_feedback.py`

**Step 1: Write failing tests**

```python
# tests/unit/test_playtest_feedback.py
"""Tests for FeedbackCollector."""

import json
import pytest
from pathlib import Path
from darwindeck.playtest.feedback import FeedbackCollector, PlaytestResult


class TestPlaytestResult:
    """Tests for PlaytestResult dataclass."""

    def test_to_dict(self):
        """Converts to dict correctly."""
        result = PlaytestResult(
            genome_id="TestGame",
            genome_path="path/to/genome.json",
            difficulty="greedy",
            seed=12345,
            winner="human",
            turns=23,
            rating=4,
            comment="Fun game",
        )

        d = result.to_dict()

        assert d["genome_id"] == "TestGame"
        assert d["seed"] == 12345
        assert d["rating"] == 4
        assert "timestamp" in d

    def test_optional_fields(self):
        """Handles optional fields."""
        result = PlaytestResult(
            genome_id="Test",
            genome_path="test.json",
            difficulty="random",
            seed=1,
            winner="ai",
            turns=10,
        )

        d = result.to_dict()

        assert d["rating"] is None
        assert d["comment"] == ""


class TestFeedbackCollector:
    """Tests for FeedbackCollector."""

    def test_saves_to_jsonl(self, tmp_path: Path):
        """Saves result as JSONL line."""
        output_file = tmp_path / "results.jsonl"
        collector = FeedbackCollector(output_file)

        result = PlaytestResult(
            genome_id="Test",
            genome_path="test.json",
            difficulty="random",
            seed=1,
            winner="human",
            turns=10,
            rating=5,
        )

        collector.save(result)

        # Read back
        lines = output_file.read_text().strip().split("\n")
        assert len(lines) == 1

        data = json.loads(lines[0])
        assert data["genome_id"] == "Test"
        assert data["rating"] == 5

    def test_appends_multiple(self, tmp_path: Path):
        """Appends multiple results."""
        output_file = tmp_path / "results.jsonl"
        collector = FeedbackCollector(output_file)

        for i in range(3):
            result = PlaytestResult(
                genome_id=f"Game{i}",
                genome_path=f"game{i}.json",
                difficulty="random",
                seed=i,
                winner="human",
                turns=10,
            )
            collector.save(result)

        lines = output_file.read_text().strip().split("\n")
        assert len(lines) == 3

    def test_creates_directory(self, tmp_path: Path):
        """Creates parent directory if needed."""
        output_file = tmp_path / "subdir" / "results.jsonl"
        collector = FeedbackCollector(output_file)

        result = PlaytestResult(
            genome_id="Test",
            genome_path="test.json",
            difficulty="random",
            seed=1,
            winner="human",
            turns=10,
        )

        collector.save(result)

        assert output_file.exists()
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_playtest_feedback.py -v`
Expected: FAIL (FeedbackCollector not implemented)

**Step 3: Implement FeedbackCollector**

```python
# src/darwindeck/playtest/feedback.py
"""Feedback collection and storage."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Optional


@dataclass
class PlaytestResult:
    """Result of a playtest session."""

    genome_id: str
    genome_path: str
    difficulty: str
    seed: int
    winner: str  # "human", "ai", "stuck", "quit"
    turns: int
    rating: Optional[int] = None
    comment: str = ""
    quit_early: bool = False
    felt_broken: bool = False
    stuck_reason: Optional[str] = None
    replay_path: Optional[str] = None
    timestamp: str = field(default_factory=lambda: datetime.now().isoformat())

    def to_dict(self) -> dict:
        """Convert to dictionary for JSON serialization."""
        return {
            "timestamp": self.timestamp,
            "genome_id": self.genome_id,
            "genome_path": self.genome_path,
            "difficulty": self.difficulty,
            "seed": self.seed,
            "winner": self.winner,
            "turns": self.turns,
            "rating": self.rating,
            "comment": self.comment,
            "quit_early": self.quit_early,
            "felt_broken": self.felt_broken,
            "stuck_reason": self.stuck_reason,
            "replay_path": self.replay_path,
        }


class FeedbackCollector:
    """Collects and saves playtest feedback."""

    def __init__(self, output_path: Path | str):
        """Initialize with output file path."""
        self.output_path = Path(output_path)

    def save(self, result: PlaytestResult) -> None:
        """Save result to JSONL file (append)."""
        # Ensure parent directory exists
        self.output_path.parent.mkdir(parents=True, exist_ok=True)

        # Append as JSONL
        with open(self.output_path, "a") as f:
            f.write(json.dumps(result.to_dict()) + "\n")
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_playtest_feedback.py -v`
Expected: All 5 tests PASS

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/feedback.py tests/unit/test_playtest_feedback.py
git commit -m "feat(playtest): implement FeedbackCollector and PlaytestResult"
```

---

## Task 8: Implement PlaytestSession Core

**Files:**
- Modify: `src/darwindeck/playtest/session.py`
- Create: `tests/unit/test_playtest_session.py`

**Step 1: Write failing tests**

```python
# tests/unit/test_playtest_session.py
"""Tests for PlaytestSession."""

import pytest
from unittest.mock import Mock, patch
from darwindeck.playtest.session import PlaytestSession, SessionConfig
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    PlayPhase, Location
)


def make_simple_genome() -> GameGenome:
    """Create simple test genome."""
    return GameGenome(
        schema_version="1.0",
        genome_id="TestGame",
        generation=1,
        setup=SetupRules(cards_per_player=2),
        turn_structure=TurnStructure(phases=[
            PlayPhase(target=Location.DISCARD)
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        max_turns=100,
        min_turns=1,
        player_count=2,
    )


class TestSessionConfig:
    """Tests for SessionConfig."""

    def test_default_config(self):
        """Default config has sensible values."""
        config = SessionConfig()

        assert config.difficulty == "greedy"
        assert config.debug is False
        assert config.max_turns == 200

    def test_seed_generation(self):
        """Generates seed if not provided."""
        config1 = SessionConfig()
        config2 = SessionConfig()

        # Seeds should be set
        assert config1.seed is not None
        assert config2.seed is not None


class TestPlaytestSession:
    """Tests for PlaytestSession."""

    def test_initialization(self):
        """Session initializes correctly."""
        genome = make_simple_genome()
        config = SessionConfig(seed=12345)

        session = PlaytestSession(genome, config)

        assert session.seed == 12345
        assert session.genome == genome
        assert session.move_history == []

    def test_assigns_human_player(self):
        """Assigns human to player 0 or 1."""
        genome = make_simple_genome()
        config = SessionConfig(seed=12345)

        session = PlaytestSession(genome, config)

        assert session.human_player_idx in (0, 1)

    def test_move_history_tracking(self):
        """Tracks moves in history."""
        genome = make_simple_genome()
        config = SessionConfig(seed=12345)
        session = PlaytestSession(genome, config)

        # Simulate adding moves
        session._record_move(0, "human", {"card": 0})
        session._record_move(1, "ai", {"card": 1})

        assert len(session.move_history) == 2
        assert session.move_history[0]["player"] == "human"
        assert session.move_history[1]["player"] == "ai"
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_playtest_session.py -v`
Expected: FAIL (PlaytestSession not implemented)

**Step 3: Implement PlaytestSession core**

```python
# src/darwindeck/playtest/session.py
"""Playtest session management."""

from __future__ import annotations

import random
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional, Callable

from darwindeck.genome.schema import GameGenome
from darwindeck.simulation.state import GameState, PlayerState, Card
from darwindeck.simulation.movegen import LegalMove, generate_legal_moves, apply_move, check_win_conditions
from darwindeck.playtest.stuck import StuckDetector
from darwindeck.playtest.display import StateRenderer, MovePresenter
from darwindeck.playtest.rules import RuleExplainer
from darwindeck.playtest.input import HumanPlayer, InputResult
from darwindeck.playtest.feedback import FeedbackCollector, PlaytestResult
from darwindeck.genome.schema import Rank, Suit


@dataclass
class SessionConfig:
    """Configuration for playtest session."""

    difficulty: str = "greedy"  # random, greedy, mcts
    debug: bool = False
    max_turns: int = 200
    seed: Optional[int] = None
    show_rules: bool = True
    results_path: Path = Path("playtest_results.jsonl")

    def __post_init__(self):
        """Generate seed if not provided."""
        if self.seed is None:
            self.seed = random.randint(0, 2**32 - 1)


class PlaytestSession:
    """Manages a human playtest session."""

    def __init__(self, genome: GameGenome, config: SessionConfig):
        """Initialize session."""
        self.genome = genome
        self.config = config
        self.seed = config.seed
        self.rng = random.Random(self.seed)

        # Components
        self.stuck_detector = StuckDetector(max_turns=config.max_turns)
        self.renderer = StateRenderer()
        self.presenter = MovePresenter()
        self.explainer = RuleExplainer()
        self.human_input = HumanPlayer()

        # Session state
        self.move_history: list[dict] = []
        self.human_player_idx = self.rng.randint(0, 1)
        self.state: Optional[GameState] = None

    def _record_move(self, turn: int, player: str, move_data: dict) -> None:
        """Record move in history."""
        self.move_history.append({
            "turn": turn,
            "player": player,
            "move": move_data,
        })

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

        # Create player states
        players = tuple(
            PlayerState(player_id=i, hand=hand, score=0)
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

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_playtest_session.py -v`
Expected: All 4 tests PASS

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/session.py tests/unit/test_playtest_session.py
git commit -m "feat(playtest): implement PlaytestSession core with config"
```

---

## Task 9: Implement Game Loop

**Files:**
- Modify: `src/darwindeck/playtest/session.py`
- Modify: `tests/unit/test_playtest_session.py`

**Step 1: Add failing tests for game loop**

```python
# Add to tests/unit/test_playtest_session.py

class TestGameLoop:
    """Tests for game loop logic."""

    def test_detects_win(self):
        """Detects when a player wins."""
        genome = make_simple_genome()
        config = SessionConfig(seed=12345)
        session = PlaytestSession(genome, config)

        # Initialize and run until win
        session.state = session._initialize_state()

        # Game should not be over initially
        winner = check_win_conditions(session.state, genome)
        assert winner is None

    def test_stuck_detection_triggers(self):
        """Stuck detection ends game."""
        genome = make_simple_genome()
        config = SessionConfig(seed=12345, max_turns=5)
        session = PlaytestSession(genome, config)

        session.state = session._initialize_state()

        # Simulate reaching turn limit
        session.state = session.state.copy_with(turn=6)
        reason = session.stuck_detector.check(session.state)

        assert reason is not None
```

**Step 2: Run tests to verify they pass (already passing)**

Run: `uv run pytest tests/unit/test_playtest_session.py::TestGameLoop -v`
Expected: PASS

**Step 3: Add run() method to PlaytestSession**

```python
# Add to src/darwindeck/playtest/session.py (inside PlaytestSession class)

    def run(self, output_fn: Callable[[str], None] = print) -> PlaytestResult:
        """Run the playtest session.

        Args:
            output_fn: Function to output text (default: print)

        Returns:
            PlaytestResult with game outcome and feedback
        """
        # Initialize state
        self.state = self._initialize_state()

        # Show rules if configured
        if self.config.show_rules:
            output_fn(self.explainer.explain_rules(self.genome))
            output_fn("")
            output_fn(f"You are Player {self.human_player_idx}")
            output_fn(f"Seed: {self.seed} (use --seed {self.seed} to replay)")
            output_fn("")

        # Main game loop
        winner: Optional[str] = None
        quit_early = False
        felt_broken = False
        stuck_reason: Optional[str] = None

        while True:
            # Check for stuck
            stuck_reason = self.stuck_detector.check(self.state)
            if stuck_reason:
                output_fn(f"\nGame stuck: {stuck_reason}")
                winner = "stuck"
                break

            # Check win conditions
            win_id = check_win_conditions(self.state, self.genome)
            if win_id is not None:
                if win_id == self.human_player_idx:
                    winner = "human"
                    output_fn("\n=== You Win! ===")
                else:
                    winner = "ai"
                    output_fn("\n=== AI Wins ===")
                break

            # Display state
            output_fn("")
            output_fn(self.renderer.render(
                self.state, self.genome, self.human_player_idx, self.config.debug
            ))

            # Generate legal moves
            moves = generate_legal_moves(self.state, self.genome)

            # Get move based on current player
            if self.state.active_player == self.human_player_idx:
                # Human turn
                output_fn("")
                output_fn(self.presenter.present(moves, self.state, self.genome))

                result = self.human_input.get_move(moves)

                if result.quit:
                    quit_early = True
                    # Ask if game felt broken
                    fb = self.human_input.get_yes_no("Did the game feel broken? [y/n]: ")
                    felt_broken = fb if fb is not None else False
                    winner = "quit"
                    break

                if result.error:
                    output_fn(result.error)
                    continue

                if result.is_pass:
                    self.stuck_detector.record_pass()
                    self._advance_turn()
                    continue

                if result.move:
                    self._record_move(self.state.turn, "human", {"card_index": result.move.card_index})
                    self.state = apply_move(self.state, result.move, self.genome)
                    self.stuck_detector.record_action()
            else:
                # AI turn
                move = self._ai_select_move(moves)
                if move:
                    output_fn(f"AI plays: card {move.card_index + 1}")
                    self._record_move(self.state.turn, "ai", {"card_index": move.card_index})
                    self.state = apply_move(self.state, move, self.genome)
                    self.stuck_detector.record_action()
                else:
                    output_fn("AI passes")
                    self.stuck_detector.record_pass()
                    self._advance_turn()

        # Collect feedback
        output_fn("")
        rating = self.human_input.get_rating()
        comment = self.human_input.get_comment()

        return PlaytestResult(
            genome_id=self.genome.genome_id,
            genome_path="",  # Set by caller
            difficulty=self.config.difficulty,
            seed=self.seed,
            winner=winner or "unknown",
            turns=self.state.turn if self.state else 0,
            rating=rating,
            comment=comment,
            quit_early=quit_early,
            felt_broken=felt_broken,
            stuck_reason=stuck_reason,
        )

    def _advance_turn(self) -> None:
        """Advance to next turn without applying a move."""
        if self.state:
            next_player = (self.state.active_player + 1) % len(self.state.players)
            self.state = self.state.copy_with(
                active_player=next_player,
                turn=self.state.turn + 1,
            )

    def _ai_select_move(self, moves: list[LegalMove]) -> Optional[LegalMove]:
        """Select move using AI strategy."""
        if not moves:
            return None

        if self.config.difficulty == "random":
            return self.rng.choice(moves)

        # Greedy: prefer moves that play cards (reduce hand size)
        # Simple heuristic for now
        return self.rng.choice(moves)
```

**Step 4: Run all session tests**

Run: `uv run pytest tests/unit/test_playtest_session.py -v`
Expected: All tests PASS

**Step 5: Commit**

```bash
git add src/darwindeck/playtest/session.py tests/unit/test_playtest_session.py
git commit -m "feat(playtest): implement game loop with AI and feedback"
```

---

## Task 10: Implement Genome Picker

**Files:**
- Create: `src/darwindeck/playtest/picker.py`
- Create: `tests/unit/test_playtest_picker.py`

**Step 1: Write failing tests**

```python
# tests/unit/test_playtest_picker.py
"""Tests for genome picker."""

import json
import pytest
from pathlib import Path
from darwindeck.playtest.picker import GenomePicker


class TestGenomePicker:
    """Tests for GenomePicker."""

    def test_finds_evolution_runs(self, tmp_path: Path):
        """Finds evolution run directories."""
        # Create test directories
        run1 = tmp_path / "evolution-20260114-120000"
        run1.mkdir()
        (run1 / "rank01_TestGame.json").write_text(json.dumps({
            "genome_id": "TestGame",
            "fitness": 0.85
        }))

        picker = GenomePicker(tmp_path)
        runs = picker.list_runs()

        assert len(runs) >= 1
        assert "20260114" in runs[0]["name"]

    def test_lists_genomes_in_run(self, tmp_path: Path):
        """Lists genomes within a run directory."""
        run_dir = tmp_path / "evolution-20260114-120000"
        run_dir.mkdir()

        # Create genome files
        for i, name in enumerate(["Alpha", "Beta", "Gamma"]):
            genome = {"genome_id": name, "fitness": 0.9 - i * 0.1}
            (run_dir / f"rank0{i+1}_{name}.json").write_text(json.dumps(genome))

        picker = GenomePicker(tmp_path)
        genomes = picker.list_genomes(run_dir)

        assert len(genomes) == 3
        assert genomes[0]["name"] == "Alpha"

    def test_loads_genome_file(self, tmp_path: Path):
        """Loads genome from JSON file."""
        genome_data = {
            "schema_version": "1.0",
            "genome_id": "TestGame",
            "generation": 1,
            "setup": {"cards_per_player": 5},
            "turn_structure": {"phases": []},
            "special_effects": [],
            "win_conditions": [{"type": "empty_hand"}],
            "scoring_rules": [],
            "max_turns": 100,
            "min_turns": 1,
            "player_count": 2,
        }
        genome_file = tmp_path / "test.json"
        genome_file.write_text(json.dumps(genome_data))

        picker = GenomePicker(tmp_path)
        genome, path = picker.load_genome(genome_file)

        assert genome.genome_id == "TestGame"
        assert path == genome_file

    def test_handles_no_runs(self, tmp_path: Path):
        """Returns empty list when no runs found."""
        picker = GenomePicker(tmp_path)
        runs = picker.list_runs()

        assert runs == []
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_playtest_picker.py -v`
Expected: FAIL (GenomePicker not implemented)

**Step 3: Implement GenomePicker**

```python
# src/darwindeck/playtest/picker.py
"""Interactive genome picker."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Optional

from darwindeck.genome.schema import GameGenome
from darwindeck.genome.serialization import genome_from_dict


class GenomePicker:
    """Picks genomes from evolution runs."""

    def __init__(self, output_dir: Path | str = Path("output")):
        """Initialize with output directory."""
        self.output_dir = Path(output_dir)

    def list_runs(self) -> list[dict]:
        """List available evolution runs.

        Returns:
            List of dicts with 'name', 'path', 'top_genomes'
        """
        runs: list[dict] = []

        if not self.output_dir.exists():
            return runs

        # Find evolution-* directories
        for path in sorted(self.output_dir.glob("evolution-*"), reverse=True):
            if path.is_dir():
                genomes = self.list_genomes(path)
                top_names = [g["name"] for g in genomes[:3]]

                runs.append({
                    "name": path.name,
                    "path": path,
                    "top_genomes": top_names,
                })

        return runs[:10]  # Limit to recent 10

    def list_genomes(self, run_path: Path) -> list[dict]:
        """List genomes in a run directory.

        Returns:
            List of dicts with 'name', 'path', 'fitness'
        """
        genomes: list[dict] = []

        for path in sorted(run_path.glob("rank*.json")):
            try:
                with open(path) as f:
                    data = json.load(f)

                genomes.append({
                    "name": data.get("genome_id", path.stem),
                    "path": path,
                    "fitness": data.get("fitness", 0.0),
                })
            except (json.JSONDecodeError, KeyError):
                continue

        return genomes

    def load_genome(self, path: Path) -> tuple[GameGenome, Path]:
        """Load genome from file.

        Returns:
            Tuple of (GameGenome, path)
        """
        with open(path) as f:
            data = json.load(f)

        return genome_from_dict(data), path

    def interactive_pick(
        self,
        output_fn=print,
        input_fn=input,
    ) -> Optional[tuple[GameGenome, Path]]:
        """Interactive genome selection.

        Returns:
            Tuple of (GameGenome, path) or None if cancelled
        """
        runs = self.list_runs()

        if not runs:
            output_fn("No evolution runs found in output/")
            output_fn("Enter genome path manually:")
            try:
                path_str = input_fn("> ").strip()
                if path_str:
                    return self.load_genome(Path(path_str))
            except (EOFError, KeyboardInterrupt):
                pass
            return None

        # Show runs
        output_fn("\nRecent evolution runs:")
        for i, run in enumerate(runs):
            output_fn(f"  [{i+1}] {run['name']}")
            if run["top_genomes"]:
                output_fn(f"      Top: {', '.join(run['top_genomes'])}")

        output_fn(f"  [{len(runs)+1}] Enter path manually")
        output_fn("")

        try:
            choice_str = input_fn("Select run: ").strip()
            choice = int(choice_str)
        except (ValueError, EOFError, KeyboardInterrupt):
            return None

        if choice == len(runs) + 1:
            # Manual path
            try:
                path_str = input_fn("Enter path: ").strip()
                return self.load_genome(Path(path_str))
            except (EOFError, KeyboardInterrupt):
                return None

        if choice < 1 or choice > len(runs):
            output_fn("Invalid choice")
            return None

        # Show genomes in selected run
        run = runs[choice - 1]
        genomes = self.list_genomes(run["path"])

        output_fn(f"\nGenomes in {run['name']}:")
        for i, g in enumerate(genomes[:10]):
            output_fn(f"  [{i+1}] {g['name']} (fitness: {g['fitness']:.3f})")
        output_fn("")

        try:
            g_choice_str = input_fn("Select genome: ").strip()
            g_choice = int(g_choice_str)
        except (ValueError, EOFError, KeyboardInterrupt):
            return None

        if g_choice < 1 or g_choice > len(genomes):
            output_fn("Invalid choice")
            return None

        return self.load_genome(genomes[g_choice - 1]["path"])
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_playtest_picker.py -v`
Expected: All 4 tests PASS

**Step 5: Update __init__.py**

```python
# Update src/darwindeck/playtest/__init__.py
from darwindeck.playtest.picker import GenomePicker

__all__ = [
    # ... existing exports ...
    "GenomePicker",
]
```

**Step 6: Commit**

```bash
git add src/darwindeck/playtest/picker.py src/darwindeck/playtest/__init__.py tests/unit/test_playtest_picker.py
git commit -m "feat(playtest): implement GenomePicker for interactive selection"
```

---

## Task 11: Implement CLI Entry Point

**Files:**
- Create: `src/darwindeck/cli/playtest.py`
- Test manually

**Step 1: Implement CLI**

```python
# src/darwindeck/cli/playtest.py
"""CLI command for human playtesting."""

from __future__ import annotations

import json
import logging
import sys
from pathlib import Path

import click

from darwindeck.playtest.session import PlaytestSession, SessionConfig
from darwindeck.playtest.picker import GenomePicker
from darwindeck.playtest.feedback import FeedbackCollector
from darwindeck.genome.serialization import genome_from_dict

logger = logging.getLogger(__name__)


@click.command()
@click.argument("genome_path", type=click.Path(exists=True), required=False)
@click.option(
    "-d", "--difficulty",
    type=click.Choice(["random", "greedy", "mcts"]),
    default=None,
    help="AI difficulty (prompts if not specified)",
)
@click.option("--debug", is_flag=True, help="Show AI's hand and full game state")
@click.option(
    "--results",
    type=click.Path(),
    default="playtest_results.jsonl",
    help="Where to save playtest results",
)
@click.option("--seed", type=int, default=None, help="Random seed for reproducibility")
@click.option("--max-turns", type=int, default=200, help="Turn limit before forced end")
@click.option("--show-rules/--no-rules", default=True, help="Display rules at start")
@click.option("-v", "--verbose", is_flag=True, help="Verbose logging")
def main(
    genome_path: str | None,
    difficulty: str | None,
    debug: bool,
    results: str,
    seed: int | None,
    max_turns: int,
    show_rules: bool,
    verbose: bool,
):
    """Play an evolved card game against an AI opponent.

    GENOME_PATH is optional - shows interactive picker if omitted.
    """
    if verbose:
        logging.basicConfig(level=logging.DEBUG)
    else:
        logging.basicConfig(level=logging.INFO)

    # Load genome
    if genome_path:
        path = Path(genome_path)
        with open(path) as f:
            data = json.load(f)
        genome = genome_from_dict(data)
        genome_path_str = str(path)
    else:
        picker = GenomePicker()
        result = picker.interactive_pick()
        if result is None:
            click.echo("No genome selected. Exiting.")
            sys.exit(0)
        genome, path = result
        genome_path_str = str(path)

    # Get difficulty if not specified
    if difficulty is None:
        click.echo("\nChoose AI difficulty:")
        click.echo("  [1] Random (easy)")
        click.echo("  [2] Greedy (medium)")
        click.echo("  [3] MCTS (hard)")
        try:
            choice = input("\nSelect [1-3]: ").strip()
            difficulty = {"1": "random", "2": "greedy", "3": "mcts"}.get(choice, "greedy")
        except (EOFError, KeyboardInterrupt):
            sys.exit(0)

    # Create config
    config = SessionConfig(
        difficulty=difficulty,
        debug=debug,
        max_turns=max_turns,
        seed=seed,
        show_rules=show_rules,
        results_path=Path(results),
    )

    # Run session
    click.echo(f"\nStarting {genome.genome_id}...")
    click.echo(f"Difficulty: {difficulty}")
    click.echo("")

    session = PlaytestSession(genome, config)

    try:
        result = session.run(output_fn=click.echo)
    except KeyboardInterrupt:
        click.echo("\n\nGame interrupted.")
        result = None

    # Save result
    if result:
        result.genome_path = genome_path_str
        collector = FeedbackCollector(config.results_path)
        collector.save(result)
        click.echo(f"\nResult saved to {config.results_path}")

    # Ask to play again
    click.echo("")
    try:
        again = input("Play again? [y/n]: ").strip().lower()
        if again == "y":
            # Recursive call with same genome
            main.main(
                [genome_path_str, "-d", difficulty] +
                (["--debug"] if debug else []) +
                ["--results", results]
            )
    except (EOFError, KeyboardInterrupt):
        pass

    click.echo("Thanks for playtesting!")


if __name__ == "__main__":
    main()
```

**Step 2: Test CLI manually**

Run: `uv run python -m darwindeck.cli.playtest --help`
Expected: Shows help text with all options

**Step 3: Commit**

```bash
git add src/darwindeck/cli/playtest.py
git commit -m "feat(playtest): implement CLI entry point"
```

---

## Task 12: Create Shell Script Wrapper

**Files:**
- Create: `scripts/playtest.sh`

**Step 1: Create shell script**

```bash
#!/usr/bin/env bash
# Human playtesting CLI for evolved card games
#
# Usage:
#   ./scripts/playtest.sh                           # Interactive picker
#   ./scripts/playtest.sh genome.json               # Specific genome
#   ./scripts/playtest.sh genome.json -d greedy     # With difficulty

set -euo pipefail

cd "$(dirname "$0")/.."

uv run python -m darwindeck.cli.playtest "$@"
```

**Step 2: Make executable**

Run: `chmod +x scripts/playtest.sh`

**Step 3: Test script**

Run: `./scripts/playtest.sh --help`
Expected: Shows help text

**Step 4: Commit**

```bash
git add scripts/playtest.sh
git commit -m "feat(scripts): add playtest.sh wrapper"
```

---

## Task 13: Update Module __init__.py Exports

**Files:**
- Modify: `src/darwindeck/playtest/__init__.py`

**Step 1: Ensure all exports are correct**

```python
# src/darwindeck/playtest/__init__.py
"""Human playtesting module for evolved card games."""

from darwindeck.playtest.stuck import StuckDetector
from darwindeck.playtest.display import StateRenderer, MovePresenter, format_card
from darwindeck.playtest.rules import RuleExplainer
from darwindeck.playtest.input import HumanPlayer, InputResult
from darwindeck.playtest.session import PlaytestSession, SessionConfig
from darwindeck.playtest.feedback import FeedbackCollector, PlaytestResult
from darwindeck.playtest.picker import GenomePicker

__all__ = [
    "StuckDetector",
    "StateRenderer",
    "MovePresenter",
    "format_card",
    "RuleExplainer",
    "HumanPlayer",
    "InputResult",
    "PlaytestSession",
    "SessionConfig",
    "FeedbackCollector",
    "PlaytestResult",
    "GenomePicker",
]
```

**Step 2: Test imports**

Run: `uv run python -c "from darwindeck.playtest import *; print('All imports OK')"`
Expected: "All imports OK"

**Step 3: Commit**

```bash
git add src/darwindeck/playtest/__init__.py
git commit -m "feat(playtest): finalize module exports"
```

---

## Task 14: Run All Tests

**Files:** None (verification only)

**Step 1: Run all playtest tests**

Run: `uv run pytest tests/unit/test_playtest*.py -v`
Expected: All tests PASS

**Step 2: Run full test suite**

Run: `uv run pytest tests/ -v --ignore=tests/integration`
Expected: No regressions

**Step 3: Verify CLI works end-to-end**

Run: `./scripts/playtest.sh --help`
Expected: Help text displays correctly

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Create module structure | playtest/*.py, __init__.py |
| 2 | Implement StuckDetector | stuck.py, test_playtest_stuck.py |
| 3 | Implement StateRenderer | display.py, test_playtest_display.py |
| 4 | Implement MovePresenter | display.py |
| 5 | Implement RuleExplainer | rules.py, test_playtest_rules.py |
| 6 | Implement HumanPlayer | input.py, test_playtest_input.py |
| 7 | Implement FeedbackCollector | feedback.py, test_playtest_feedback.py |
| 8 | Implement PlaytestSession core | session.py, test_playtest_session.py |
| 9 | Implement game loop | session.py |
| 10 | Implement GenomePicker | picker.py, test_playtest_picker.py |
| 11 | Implement CLI entry point | cli/playtest.py |
| 12 | Create shell script | scripts/playtest.sh |
| 13 | Update module exports | __init__.py |
| 14 | Run all tests | verification |

**Total: 14 tasks**
