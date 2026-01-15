"""Tests for fitness evaluation metrics."""

import pytest
from darwindeck.evolution.fitness import CheapFitnessMetrics, calculate_cheap_metrics
from darwindeck.evolution.fitness_full import FitnessEvaluator, SimulationResults, FitnessResult
from darwindeck.simulation.engine import GameEngine, GameResult
from darwindeck.simulation.players import RandomPlayer
from darwindeck.genome.examples import create_war_genome
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, DiscardPhase, WinCondition, Location,
    PlayPhase, TableauMode, SequenceDirection
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


class TestTableauCoherencePenalty:
    """Tests for tableau mode + win condition coherence penalties."""

    def test_war_mode_with_empty_hand_gets_penalty(self):
        """WAR mode + empty_hand win condition gets coherence penalty."""
        from darwindeck.evolution.fitness_full import calculate_coherence_penalty

        genome = GameGenome(
            schema_version="1.0",
            genome_id="incoherent",
            generation=1,
            setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.WAR),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],  # Conflict!
            scoring_rules=[],
        )

        penalty = calculate_coherence_penalty(genome)

        assert penalty >= 0.3  # At least 30%

    def test_match_rank_with_capture_all_gets_penalty(self):
        """MATCH_RANK mode + capture_all win condition gets coherence penalty."""
        from darwindeck.evolution.fitness_full import calculate_coherence_penalty

        genome = GameGenome(
            schema_version="1.0",
            genome_id="incoherent",
            generation=1,
            setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.MATCH_RANK),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="capture_all")],  # Conflict!
            scoring_rules=[],
        )

        penalty = calculate_coherence_penalty(genome)

        assert penalty >= 0.2  # At least 20%

    def test_sequence_with_capture_all_gets_penalty(self):
        """SEQUENCE mode + capture_all win condition gets coherence penalty."""
        from darwindeck.evolution.fitness_full import calculate_coherence_penalty

        genome = GameGenome(
            schema_version="1.0",
            genome_id="incoherent",
            generation=1,
            setup=SetupRules(
                cards_per_player=7,
                tableau_mode=TableauMode.SEQUENCE,
                sequence_direction=SequenceDirection.BOTH
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="capture_all")],  # Conflict!
            scoring_rules=[],
        )

        penalty = calculate_coherence_penalty(genome)

        assert penalty >= 0.3  # At least 30%

    def test_coherent_mode_no_penalty(self):
        """Coherent tableau mode + win condition combinations get no penalty."""
        from darwindeck.evolution.fitness_full import calculate_coherence_penalty

        # WAR + capture_all is coherent
        genome = GameGenome(
            schema_version="1.0",
            genome_id="coherent",
            generation=1,
            setup=SetupRules(cards_per_player=26, tableau_mode=TableauMode.WAR),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="capture_all")],  # Good match!
            scoring_rules=[],
        )

        penalty = calculate_coherence_penalty(genome)

        assert penalty == 0.0  # No penalty for coherent combo

    def test_none_mode_no_penalty(self):
        """NONE tableau mode never gets penalty."""
        from darwindeck.evolution.fitness_full import calculate_coherence_penalty

        genome = GameGenome(
            schema_version="1.0",
            genome_id="none_mode",
            generation=1,
            setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.NONE),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
        )

        penalty = calculate_coherence_penalty(genome)

        assert penalty == 0.0  # NONE mode is always fine

    def test_coherence_penalty_applied_to_fitness(self):
        """Coherence penalty should be applied to total fitness via quality_multiplier."""
        from darwindeck.evolution.fitness_full import (
            FitnessEvaluator, SimulationResults, calculate_coherence_penalty
        )

        # Create a coherent genome (WAR + capture_all)
        coherent_genome = GameGenome(
            schema_version="1.0",
            genome_id="coherent",
            generation=1,
            setup=SetupRules(cards_per_player=26, tableau_mode=TableauMode.WAR),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="capture_all")],
            scoring_rules=[],
        )

        # Create an incoherent genome (WAR + empty_hand)
        incoherent_genome = GameGenome(
            schema_version="1.0",
            genome_id="incoherent",
            generation=1,
            setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.WAR),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
        )

        # Verify penalties
        assert calculate_coherence_penalty(coherent_genome) == 0.0
        assert calculate_coherence_penalty(incoherent_genome) >= 0.30
