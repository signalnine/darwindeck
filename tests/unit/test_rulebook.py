"""Tests for rulebook generation."""

import pytest
from darwindeck.evolution.rulebook import RulebookSections, GenomeValidator, ValidationResult
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    PlayPhase, DrawPhase, BettingPhase, Location
)


class TestRulebookSections:
    """Tests for the RulebookSections dataclass."""

    def test_rulebook_sections_creation(self):
        """RulebookSections can be created with all fields."""
        sections = RulebookSections(
            game_name="TestGame",
            player_count=2,
            overview="A test game.",
            components=["Standard 52-card deck"],
            setup_steps=["Shuffle the deck", "Deal 5 cards to each player"],
            objective="First to empty hand wins",
            phases=[("Draw", "Draw 1 card from the deck")],
            special_rules=[],
            edge_cases=["Reshuffle discard when deck empty"],
            quick_reference="Draw -> Play -> Win"
        )
        assert sections.game_name == "TestGame"
        assert len(sections.setup_steps) == 2
        assert len(sections.phases) == 1

    def test_rulebook_sections_defaults(self):
        """RulebookSections has sensible defaults."""
        sections = RulebookSections(
            game_name="Minimal",
            player_count=2,
            objective="Win the game"
        )
        assert sections.overview is None
        assert sections.components == []
        assert sections.edge_cases == []


class TestGenomeValidator:
    """Tests for pre-extraction genome validation."""

    def _make_genome(self, cards_per_player=5, player_count=2, starting_chips=0,
                     phases=None, win_conditions=None):
        """Helper to create test genomes."""
        if phases is None:
            phases = [DrawPhase(source=Location.DECK, count=1)]
        if win_conditions is None:
            win_conditions = [WinCondition(type="empty_hand")]
        return GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=cards_per_player, starting_chips=starting_chips),
            turn_structure=TurnStructure(phases=phases),
            special_effects=[],
            win_conditions=win_conditions,
            player_count=player_count,
            scoring_rules=[],
        )

    def test_valid_genome_passes(self):
        """A valid genome passes validation."""
        genome = self._make_genome()
        result = GenomeValidator().validate(genome)
        assert result.valid is True
        assert result.errors == []

    def test_too_many_cards_fails(self):
        """Dealing more cards than deck has fails."""
        genome = self._make_genome(cards_per_player=30, player_count=2)  # 60 > 52
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("cards" in e.lower() for e in result.errors)

    def test_betting_without_chips_fails(self):
        """BettingPhase with 0 starting chips fails."""
        genome = self._make_genome(
            starting_chips=0,
            phases=[BettingPhase(min_bet=10)]
        )
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("chip" in e.lower() for e in result.errors)

    def test_no_phases_fails(self):
        """Empty turn structure fails."""
        genome = self._make_genome(phases=[])
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("phase" in e.lower() for e in result.errors)

    def test_no_win_conditions_fails(self):
        """No win conditions fails."""
        genome = self._make_genome(win_conditions=[])
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("win" in e.lower() for e in result.errors)
