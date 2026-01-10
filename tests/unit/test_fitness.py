"""Tests for fitness evaluation metrics."""

import pytest
from darwindeck.evolution.fitness import CheapFitnessMetrics, calculate_cheap_metrics
from darwindeck.simulation.engine import GameEngine, GameResult
from darwindeck.simulation.players import RandomPlayer
from darwindeck.genome.examples import create_war_genome


def test_calculate_game_length() -> None:
    """Test game length metric."""
    genome = create_war_genome()
    engine = GameEngine()
    players = [RandomPlayer(seed=0), RandomPlayer(seed=1)]

    result = engine.simulate_game(genome, players, seed=42)
    metrics = calculate_cheap_metrics([result])

    assert metrics.avg_game_length > 0
    assert metrics.avg_game_length == result.turn_count


def test_calculate_termination_type() -> None:
    """Test completion rate metric."""
    genome = create_war_genome()
    engine = GameEngine()
    players = [RandomPlayer(seed=0), RandomPlayer(seed=1)]

    result = engine.simulate_game(genome, players, seed=42)
    metrics = calculate_cheap_metrics([result])

    # Completion rate should be calculated
    assert 0.0 <= metrics.completion_rate <= 1.0


def test_war_has_zero_decision_density() -> None:
    """Test War game has near-zero decision density (sanity check)."""
    genome = create_war_genome()
    engine = GameEngine()
    players = [RandomPlayer(seed=0), RandomPlayer(seed=1)]

    results = [engine.simulate_game(genome, players, seed=i) for i in range(10)]
    metrics = calculate_cheap_metrics(results)

    # War has no decisions - should be 0.0
    assert metrics.decision_branch_factor == 0.0
