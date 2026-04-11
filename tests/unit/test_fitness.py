"""Tests for fitness evaluation metrics."""

import pytest
from darwindeck.evolution.fitness import CheapFitnessMetrics, calculate_cheap_metrics
from darwindeck.evolution.fitness_full import FitnessEvaluator, SimulationResults
from darwindeck.simulation.engine import GameEngine, GameResult
from darwindeck.simulation.players import RandomPlayer
from darwindeck.genome.examples import create_war_genome, create_hearts_genome


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


def test_trick_based_interaction_frequency_without_instrumentation() -> None:
    """Trick-based games should not get a free high interaction score from heuristic alone."""
    genome = create_hearts_genome()

    # Simulate results with no instrumentation data (total_actions=0)
    results = SimulationResults(
        total_games=100,
        wins=(25, 25, 25, 25),
        player_count=4,
        draws=0,
        avg_turns=52,
        errors=0,
    )

    evaluator = FitnessEvaluator()
    metrics = evaluator.evaluate(genome, results)

    # With the reduced trick bonus (0.15 instead of 0.3), the heuristic
    # interaction_frequency should be below 0.5
    assert metrics.interaction_frequency < 0.5


def test_skill_vs_luck_heuristic_can_reach_high_values() -> None:
    """A well-designed game with real decision data should reach skill_vs_luck >= 0.75."""
    evaluator = FitnessEvaluator(style='balanced')
    results = SimulationResults(
        total_games=100,
        wins=(50, 50),
        player_count=2,
        draws=0,
        avg_turns=60,
        errors=0,
        total_decisions=300,
        total_valid_moves=900,
        forced_decisions=30,
        total_hand_size=1200,
        total_interactions=150,
        total_actions=600,
    )
    genome = create_war_genome()
    metrics = evaluator.evaluate(genome, results)
    assert metrics.skill_vs_luck >= 0.75


def test_rules_complexity_not_inflated_by_trick_phase_effects():
    """Trick-based games should not get inflated rules_complexity from special effects."""
    evaluator = FitnessEvaluator(style='balanced')
    results = SimulationResults(
        total_games=100, wins=(50, 50), player_count=2, draws=0,
        avg_turns=40, errors=0,
        total_decisions=200, total_valid_moves=400, forced_decisions=50,
        total_hand_size=800, total_interactions=100, total_actions=400,
    )
    from darwindeck.genome.examples import create_hearts_genome
    from dataclasses import replace as dc_replace
    from darwindeck.genome.schema import SpecialEffect, EffectType, TargetSelector, Rank

    base_genome = create_hearts_genome()
    no_effects = dc_replace(base_genome, special_effects=())
    no_effects_m = evaluator.evaluate(no_effects, results)

    effects = (
        SpecialEffect(Rank.ACE, EffectType.SKIP_NEXT, TargetSelector.NEXT_PLAYER),
        SpecialEffect(Rank.KING, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        SpecialEffect(Rank.QUEEN, EffectType.REVERSE_DIRECTION, TargetSelector.ALL_OPPONENTS),
    )
    with_effects = dc_replace(base_genome, special_effects=effects)
    with_effects_m = evaluator.evaluate(with_effects, results)

    diff = with_effects_m.rules_complexity - no_effects_m.rules_complexity
    assert diff < 0.1, f"Inert effects inflated rules_complexity by {diff:.3f}"
def test_fitness_penalizes_positional_imbalance():
    """4-player games with first-player advantage get penalized."""
    evaluator = FitnessEvaluator(style='party')
    from darwindeck.genome.examples import create_hearts_genome
    genome = create_hearts_genome()
    balanced = SimulationResults(
        total_games=100, wins=(25, 25, 25, 25), player_count=4,
        draws=0, avg_turns=20, errors=0,
        total_decisions=200, total_valid_moves=400, forced_decisions=20,
        total_hand_size=800, total_interactions=50, total_actions=200,
    )
    imbalanced = SimulationResults(
        total_games=100, wins=(55, 25, 10, 10), player_count=4,
        draws=0, avg_turns=20, errors=0,
        total_decisions=200, total_valid_moves=400, forced_decisions=20,
        total_hand_size=800, total_interactions=50, total_actions=200,
    )
    b = evaluator.evaluate(genome, balanced)
    i = evaluator.evaluate(genome, imbalanced)
    assert i.total_fitness < b.total_fitness


def test_fitness_penalizes_2player_imbalance():
    """2-player games with positional imbalance get penalized."""
    evaluator = FitnessEvaluator(style='balanced')
    genome = create_war_genome()
    balanced = SimulationResults(
        total_games=100, wins=(50, 50), player_count=2,
        draws=0, avg_turns=40, errors=0,
        total_decisions=200, total_valid_moves=400, forced_decisions=20,
        total_hand_size=800, total_interactions=50, total_actions=200,
    )
    imbalanced = SimulationResults(
        total_games=100, wins=(70, 30), player_count=2,
        draws=0, avg_turns=40, errors=0,
        total_decisions=200, total_valid_moves=400, forced_decisions=20,
        total_hand_size=800, total_interactions=50, total_actions=200,
    )
    b = evaluator.evaluate(genome, balanced)
    i = evaluator.evaluate(genome, imbalanced)
    assert i.total_fitness < b.total_fitness
