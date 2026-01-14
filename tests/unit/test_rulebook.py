"""Tests for rulebook generation."""

import pytest
from darwindeck.evolution.rulebook import RulebookSections


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
