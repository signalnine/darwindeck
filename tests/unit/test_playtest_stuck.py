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
