# tests/unit/test_self_describing_types.py
"""Tests for self-describing genome types."""

import pytest
from darwindeck.genome.schema import ScoringTrigger, Suit, Rank, CardValue


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


class TestCardScoringRule:
    def test_hearts_scoring_rule(self):
        """CardScoringRule can express Hearts 1-point-per-heart."""
        from darwindeck.genome.schema import CardScoringRule, CardCondition
        rule = CardScoringRule(
            condition=CardCondition(suit=Suit.HEARTS),
            points=1,
            trigger=ScoringTrigger.TRICK_WIN
        )
        assert rule.points == 1
        assert rule.trigger == ScoringTrigger.TRICK_WIN
        assert rule.condition.suit == Suit.HEARTS

    def test_queen_of_spades_scoring(self):
        """CardScoringRule can express Queen of Spades 13 points."""
        from darwindeck.genome.schema import CardScoringRule, CardCondition
        rule = CardScoringRule(
            condition=CardCondition(suit=Suit.SPADES, rank=Rank.QUEEN),
            points=13,
            trigger=ScoringTrigger.TRICK_WIN
        )
        assert rule.points == 13

    def test_negative_points(self):
        """CardScoringRule can have negative points."""
        from darwindeck.genome.schema import CardScoringRule, CardCondition
        rule = CardScoringRule(
            condition=CardCondition(rank=Rank.ACE),
            points=-10,
            trigger=ScoringTrigger.HAND_END
        )
        assert rule.points == -10

    def test_card_scoring_rule_frozen(self):
        """CardScoringRule is immutable."""
        from darwindeck.genome.schema import CardScoringRule, CardCondition
        rule = CardScoringRule(
            condition=CardCondition(suit=Suit.HEARTS),
            points=1,
            trigger=ScoringTrigger.TRICK_WIN
        )
        with pytest.raises(AttributeError):
            rule.points = 5


class TestHandEvaluationMethod:
    def test_hand_evaluation_method_values(self):
        """HandEvaluationMethod enum has expected values."""
        from darwindeck.genome.schema import HandEvaluationMethod
        assert HandEvaluationMethod.NONE.value == "none"
        assert HandEvaluationMethod.HIGH_CARD.value == "high_card"
        assert HandEvaluationMethod.POINT_TOTAL.value == "point_total"
        assert HandEvaluationMethod.PATTERN_MATCH.value == "pattern_match"
        assert HandEvaluationMethod.CARD_COUNT.value == "card_count"


class TestCardValue:
    def test_card_value_simple(self):
        """CardValue can express simple point value."""
        cv = CardValue(rank=Rank.KING, value=10)
        assert cv.rank == Rank.KING
        assert cv.value == 10
        assert cv.alternate_value is None

    def test_card_value_with_alternate(self):
        """CardValue can express alternate value (Ace in Blackjack)."""
        cv = CardValue(rank=Rank.ACE, value=11, alternate_value=1)
        assert cv.value == 11
        assert cv.alternate_value == 1

    def test_card_value_frozen(self):
        """CardValue is immutable."""
        cv = CardValue(rank=Rank.KING, value=10)
        with pytest.raises(AttributeError):
            cv.value = 20