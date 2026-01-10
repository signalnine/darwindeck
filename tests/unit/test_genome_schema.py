"""Tests for genome schema types."""

import pytest
from cards_evolve.genome.schema import Rank, Suit, GameGenome


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
