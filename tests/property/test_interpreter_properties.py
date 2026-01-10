"""Property-based tests for genome interpreter."""

import pytest
from hypothesis import given, strategies as st
from darwindeck.simulation.engine import GameEngine
from darwindeck.simulation.players import RandomPlayer
from darwindeck.genome.examples import create_war_genome


@given(seed=st.integers(min_value=0, max_value=10000))
def test_determinism_property(seed: int) -> None:
    """Property: Same seed always produces same game outcome."""
    genome = create_war_genome()
    engine = GameEngine()
    players = [RandomPlayer(seed=0), RandomPlayer(seed=1)]

    result1 = engine.simulate_game(genome, players, seed=seed)
    result2 = engine.simulate_game(genome, players, seed=seed)

    assert result1.winner == result2.winner
    assert result1.turn_count == result2.turn_count


@given(seed=st.integers(min_value=0, max_value=10000))
def test_immutability_property(seed: int) -> None:
    """Property: States in history are truly immutable."""
    genome = create_war_genome()
    engine = GameEngine()
    players = [RandomPlayer(seed=0), RandomPlayer(seed=1)]

    result = engine.simulate_game(genome, players, seed=seed)

    # Try to mutate history
    first_state = result.history[0]
    original_turn = first_state.turn

    # Frozen dataclass should prevent mutation
    with pytest.raises(AttributeError):
        first_state.turn = 999  # type: ignore

    # Verify nothing changed
    assert result.history[0].turn == original_turn


@given(seed=st.integers(min_value=0, max_value=10000))
def test_game_terminates_property(seed: int) -> None:
    """Property: All games terminate within max_turns."""
    genome = create_war_genome()
    engine = GameEngine()
    players = [RandomPlayer(seed=0), RandomPlayer(seed=1)]

    result = engine.simulate_game(genome, players, seed=seed)

    assert result.turn_count <= genome.max_turns
