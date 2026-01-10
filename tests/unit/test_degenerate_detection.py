"""Tests for degenerate game detection."""

import pytest
from darwindeck.simulation.validation import DegenGameDetector
from darwindeck.simulation.engine import GameEngine, GameResult
from darwindeck.simulation.players import RandomPlayer
from darwindeck.genome.examples import create_war_genome


def test_war_is_not_too_short() -> None:
    """Test War games are not flagged as too short."""
    genome = create_war_genome()
    engine = GameEngine()
    players = [RandomPlayer(seed=0), RandomPlayer(seed=1)]

    results = [engine.simulate_game(genome, players, seed=i) for i in range(10)]
    detector = DegenGameDetector(genome)

    is_degen = detector.is_degenerate(results)

    # War games should not be degenerate (they run long enough)
    assert not is_degen


def test_short_game_detected() -> None:
    """Test games that end too quickly are detected."""
    # Create fake results with very short games
    from darwindeck.simulation.state import GameState, PlayerState

    fake_result = GameResult(
        winner=0,
        turn_count=2,  # Too short!
        history=[
            GameState(
                players=(PlayerState(0, (), 0), PlayerState(1, (), 0)),
                deck=(),
                discard=(),
                turn=0,
                active_player=0
            )
        ] * 3
    )

    genome = create_war_genome()
    detector = DegenGameDetector(genome)

    is_degen = detector.is_degenerate([fake_result])

    assert is_degen  # Should detect as too short
