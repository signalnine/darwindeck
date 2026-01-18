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
        assert len(errors) >= 1
        assert any("Score-based win condition requires" in e for e in errors)

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
        assert len(errors) >= 1
        assert any("best_hand win condition requires" in e for e in errors)

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


class TestTeamValidation:
    """Tests for team configuration validation."""

    def test_validate_team_mode_without_teams_fails(self):
        """team_mode=True with empty teams should fail validation."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=4,
            team_mode=True,
            teams=(),  # Empty teams with team_mode=True
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) >= 1
        assert any("team" in e.lower() for e in errors)

    def test_validate_duplicate_player_in_teams_fails(self):
        """Player appearing in multiple teams should fail validation."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=4,
            team_mode=True,
            teams=((0, 1), (1, 2)),  # Player 1 in two teams
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) >= 1
        assert any("duplicate" in e.lower() for e in errors)

    def test_validate_player_index_out_of_range_fails(self):
        """Player index >= num_players should fail validation."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=4,
            team_mode=True,
            teams=((0, 2), (1, 5)),  # Player 5 doesn't exist
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) >= 1
        assert any("index" in e.lower() or "range" in e.lower() for e in errors)

    def test_validate_missing_player_in_teams_fails(self):
        """Not all players assigned to teams should fail validation."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=4,
            team_mode=True,
            teams=((0,), (1,)),  # Two teams but players 2 and 3 not assigned
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) >= 1
        assert any("assigned" in e.lower() for e in errors)

    def test_validate_single_team_fails(self):
        """Only one team should fail validation (need at least 2)."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=4,
            team_mode=True,
            teams=((0, 1, 2, 3),),  # All players on one team
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) >= 1
        assert any("team" in e.lower() and ("2" in e or "two" in e.lower()) for e in errors)

    def test_validate_valid_team_config_passes(self):
        """Valid 2v2 team configuration should pass."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=4,
            team_mode=True,
            teams=((0, 2), (1, 3)),  # Valid 2v2
        )
        errors = GenomeValidator.validate(genome)
        # Filter out non-team related errors for this test
        team_errors = [e for e in errors if "team" in e.lower()]
        assert len(team_errors) == 0, f"Expected no team errors but got: {team_errors}"

    def test_validate_team_mode_false_with_teams_passes(self):
        """team_mode=False ignores teams field (for backward compatibility)."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=4,
            team_mode=False,
            teams=((0, 2), (1, 3)),  # Ignored when team_mode=False
        )
        errors = GenomeValidator.validate(genome)
        # Should not have team-related errors because team_mode is False
        team_errors = [e for e in errors if "team" in e.lower()]
        assert len(team_errors) == 0, f"Expected no team errors but got: {team_errors}"
