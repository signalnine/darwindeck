"""Tests for game simulation engine."""

import pytest
from cards_evolve.simulation.engine import GameEngine, GameResult
from cards_evolve.simulation.players import RandomPlayer
from cards_evolve.genome.examples import create_war_genome


def test_game_engine_simulates_war() -> None:
    """Test game engine can simulate War game."""
    genome = create_war_genome()
    engine = GameEngine()
    players = [RandomPlayer(), RandomPlayer()]

    result = engine.simulate_game(genome, players, seed=42)

    assert isinstance(result, GameResult)
    assert result.winner in [0, 1]
    assert result.turn_count > 0
    assert result.turn_count <= genome.max_turns


def test_game_engine_deterministic() -> None:
    """Test same seed produces same game outcome."""
    genome = create_war_genome()
    engine = GameEngine()
    players = [RandomPlayer(), RandomPlayer()]

    result1 = engine.simulate_game(genome, players, seed=42)
    result2 = engine.simulate_game(genome, players, seed=42)

    assert result1.winner == result2.winner
    assert result1.turn_count == result2.turn_count


def test_game_result_has_history() -> None:
    """Test GameResult includes state history."""
    genome = create_war_genome()
    engine = GameEngine()
    players = [RandomPlayer(), RandomPlayer()]

    result = engine.simulate_game(genome, players, seed=42)

    assert len(result.history) > 0
    assert len(result.history) == result.turn_count + 1  # Initial state + each turn
