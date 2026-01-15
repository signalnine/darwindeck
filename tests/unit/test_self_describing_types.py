# tests/unit/test_self_describing_types.py
"""Tests for self-describing genome types."""

import pytest
from darwindeck.genome.schema import ScoringTrigger, Suit, Rank


class TestScoringTrigger:
    def test_scoring_trigger_enum_values(self):
        """ScoringTrigger enum has expected values."""
        assert ScoringTrigger.TRICK_WIN.value == "trick_win"
        assert ScoringTrigger.CAPTURE.value == "capture"
        assert ScoringTrigger.PLAY.value == "play"
        assert ScoringTrigger.HAND_END.value == "hand_end"
        assert ScoringTrigger.SET_COMPLETE.value == "set_complete"


class TestCardCondition:
    def test_card_condition_suit_only(self):
        """CardCondition can match by suit."""
        from darwindeck.genome.schema import CardCondition
        cond = CardCondition(suit=Suit.HEARTS)
        assert cond.suit == Suit.HEARTS
        assert cond.rank is None

    def test_card_condition_rank_only(self):
        """CardCondition can match by rank."""
        from darwindeck.genome.schema import CardCondition
        cond = CardCondition(rank=Rank.QUEEN)
        assert cond.rank == Rank.QUEEN
        assert cond.suit is None

    def test_card_condition_both(self):
        """CardCondition can match by suit and rank."""
        from darwindeck.genome.schema import CardCondition
        cond = CardCondition(suit=Suit.SPADES, rank=Rank.QUEEN)
        assert cond.suit == Suit.SPADES
        assert cond.rank == Rank.QUEEN

    def test_card_condition_frozen(self):
        """CardCondition is immutable."""
        from darwindeck.genome.schema import CardCondition
        cond = CardCondition(suit=Suit.HEARTS)
        with pytest.raises(AttributeError):
            cond.suit = Suit.CLUBS
