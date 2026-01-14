"""Tests for fitness evaluation metrics."""

import pytest
from darwindeck.evolution.fitness import CheapFitnessMetrics, calculate_cheap_metrics
from darwindeck.evolution.fitness_full import FitnessEvaluator, SimulationResults, FitnessResult
from darwindeck.simulation.engine import GameEngine, GameResult
from darwindeck.simulation.players import RandomPlayer
from darwindeck.genome.examples import create_war_genome
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, DiscardPhase, WinCondition, Location
)


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


def test_tension_curve_with_real_data() -> None:
    """Fitness uses real tension data when available."""
    results = SimulationResults(
        total_games=100,
        wins=(50, 50),
        player_count=2,
        draws=0,
        avg_turns=50,
        errors=0,
        lead_changes=5,
        decisive_turn_pct=0.8,
        closest_margin=0.1,
    )

    evaluator = FitnessEvaluator()
    metrics = evaluator.evaluate(create_war_genome(), results)

    # Should use real data, not fallback
    # lead_change_score = min(1.0, 5 / 2.5) = 1.0
    # decisive_turn_score = 0.8
    # margin_score = 1.0 - 0.1 = 0.9
    # tension = 1.0*0.4 + 0.8*0.4 + 0.9*0.2 = 0.4 + 0.32 + 0.18 = 0.9
    assert metrics.tension_curve > 0.85


class TestFitnessCoherenceIntegration:
    def test_incoherent_genome_gets_zero_fitness(self):
        """Incoherent genome should have fitness=0."""
        # Genome with high_score but no scoring rules
        genome = GameGenome(
            schema_version="1.0",
            genome_id="incoherent_test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(
                phases=(DiscardPhase(target=Location.DISCARD, count=1),)
            ),
            special_effects=[],
            win_conditions=[WinCondition(type="high_score", threshold=50)],
            scoring_rules=[],
            player_count=2,
        )

        from darwindeck.evolution.fitness_full import FullFitnessEvaluator
        evaluator = FullFitnessEvaluator()
        result = evaluator.evaluate(genome)

        assert result.fitness == 0.0
        assert result.valid is False
        assert len(result.coherence_violations) > 0
        assert "high_score" in result.coherence_violations[0]
