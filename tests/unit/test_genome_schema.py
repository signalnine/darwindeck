"""Tests for genome schema types."""

import pytest
from darwindeck.genome.schema import Rank, Suit, GameGenome, SetupRules, TurnStructure


def test_rank_enum_has_all_ranks() -> None:
    """Test that Rank enum contains all standard playing card ranks."""
    assert Rank.ACE.value == "A"
    assert Rank.TWO.value == "2"
    assert Rank.KING.value == "K"
    assert len(Rank) == 13


def test_suit_enum_has_four_suits() -> None:
    """Test that Suit enum contains four standard suits."""
    assert Suit.HEARTS.value == "H"
    assert Suit.DIAMONDS.value == "D"
    assert Suit.CLUBS.value == "C"
    assert Suit.SPADES.value == "S"
    assert len(Suit) == 4


# Team play tests


def test_genome_has_team_mode_field() -> None:
    """GameGenome should have team_mode field defaulting to False."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test-1",
        generation=0,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[],
        scoring_rules=[],
    )
    assert hasattr(genome, "team_mode")
    assert genome.team_mode is False


def test_genome_has_teams_field() -> None:
    """GameGenome should have teams field defaulting to empty tuple."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test-2",
        generation=0,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[],
        scoring_rules=[],
    )
    assert hasattr(genome, "teams")
    assert genome.teams == ()


def test_genome_team_mode_with_teams() -> None:
    """GameGenome should accept team configuration."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="team-game-1",
        generation=0,
        setup=SetupRules(cards_per_player=13),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[],
        scoring_rules=[],
        player_count=4,
        team_mode=True,
        teams=((0, 2), (1, 3)),  # Players 0&2 vs 1&3
    )
    assert genome.team_mode is True
    assert genome.teams == ((0, 2), (1, 3))
