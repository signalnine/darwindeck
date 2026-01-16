"""Tests for GenomeValidator."""

import pytest
from darwindeck.genome.validator import GenomeValidator
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    BettingPhase, PlayPhase, Location, HandEvaluation,
    HandEvaluationMethod, HandPattern, CardValue, Rank,
    WinComparison, TriggerMode, ShowdownMethod, GameRules,
)
from darwindeck.genome.examples import create_war_genome


class TestGenomeValidator:
    def test_war_genome_valid(self):
        """War genome passes validation."""
        genome = create_war_genome()
        errors = GenomeValidator.validate(genome)
        assert errors == []

    def test_score_win_requires_scoring(self):
        """Score-based win without scoring fails validation."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[]),
            special_effects=[],
            win_conditions=[WinCondition(type="high_score", threshold=100)],
            scoring_rules=[],
            card_scoring=(),  # No scoring!
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) == 1
        assert "Score-based win condition requires" in errors[0]

    def test_best_hand_requires_pattern_match(self):
        """best_hand win without PATTERN_MATCH fails validation."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[]),
            special_effects=[],
            win_conditions=[WinCondition(type="best_hand")],
            scoring_rules=[],
            hand_evaluation=None,  # No hand evaluation!
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) == 1
        assert "best_hand win condition requires" in errors[0]

    def test_betting_requires_chips(self):
        """BettingPhase without starting_chips fails validation."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5, starting_chips=0),
            turn_structure=TurnStructure(phases=[BettingPhase(min_bet=10)]),
            special_effects=[],
            win_conditions=[WinCondition(type="most_chips")],
            scoring_rules=[],
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) >= 1
        assert any("starting_chips" in e for e in errors)
