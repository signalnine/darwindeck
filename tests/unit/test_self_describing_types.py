# tests/unit/test_self_describing_types.py
"""Tests for self-describing genome types."""

import pytest
from darwindeck.genome.schema import ScoringTrigger, Suit, Rank, CardValue, HandPattern


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


class TestHandPattern:
    def test_flush_pattern(self):
        """HandPattern can express a flush (5 same suit)."""
        pattern = HandPattern(
            name="Flush",
            rank_priority=60,
            required_count=5,
            same_suit_count=5,
        )
        assert pattern.name == "Flush"
        assert pattern.rank_priority == 60
        assert pattern.same_suit_count == 5

    def test_full_house_pattern(self):
        """HandPattern can express full house (3+2)."""
        pattern = HandPattern(
            name="Full House",
            rank_priority=70,
            required_count=5,
            same_rank_groups=(3, 2),
        )
        assert pattern.same_rank_groups == (3, 2)

    def test_straight_pattern(self):
        """HandPattern can express straight (5 consecutive)."""
        pattern = HandPattern(
            name="Straight",
            rank_priority=50,
            required_count=5,
            sequence_length=5,
            sequence_wrap=True,
        )
        assert pattern.sequence_length == 5
        assert pattern.sequence_wrap is True

    def test_royal_flush_pattern(self):
        """HandPattern can express royal flush with required ranks."""
        pattern = HandPattern(
            name="Royal Flush",
            rank_priority=100,
            required_count=5,
            same_suit_count=5,
            sequence_length=5,
            required_ranks=(Rank.TEN, Rank.JACK, Rank.QUEEN, Rank.KING, Rank.ACE),
        )
        assert pattern.required_ranks == (Rank.TEN, Rank.JACK, Rank.QUEEN, Rank.KING, Rank.ACE)

    def test_hand_pattern_frozen(self):
        """HandPattern is immutable."""
        pattern = HandPattern(name="Test", rank_priority=10)
        with pytest.raises(AttributeError):
            pattern.name = "Changed"


class TestHandEvaluation:
    def test_poker_hand_evaluation(self):
        """HandEvaluation can express poker with patterns."""
        from darwindeck.genome.schema import HandEvaluation, HandEvaluationMethod
        eval = HandEvaluation(
            method=HandEvaluationMethod.PATTERN_MATCH,
            patterns=(
                HandPattern(name="Flush", rank_priority=60, same_suit_count=5),
                HandPattern(name="Pair", rank_priority=20, same_rank_groups=(2,)),
            ),
        )
        assert eval.method == HandEvaluationMethod.PATTERN_MATCH
        assert len(eval.patterns) == 2

    def test_blackjack_hand_evaluation(self):
        """HandEvaluation can express blackjack with card values."""
        from darwindeck.genome.schema import HandEvaluation, HandEvaluationMethod
        eval = HandEvaluation(
            method=HandEvaluationMethod.POINT_TOTAL,
            card_values=(
                CardValue(rank=Rank.ACE, value=11, alternate_value=1),
                CardValue(rank=Rank.KING, value=10),
            ),
            target_value=21,
            bust_threshold=22,
        )
        assert eval.target_value == 21
        assert eval.bust_threshold == 22

    def test_hand_evaluation_frozen(self):
        """HandEvaluation is immutable."""
        from darwindeck.genome.schema import HandEvaluation, HandEvaluationMethod
        eval = HandEvaluation(method=HandEvaluationMethod.HIGH_CARD)
        with pytest.raises(AttributeError):
            eval.method = HandEvaluationMethod.PATTERN_MATCH


class TestWinConditionEnums:
    def test_win_comparison_values(self):
        """WinComparison enum has expected values."""
        from darwindeck.genome.schema import WinComparison
        assert WinComparison.HIGHEST.value == "highest"
        assert WinComparison.LOWEST.value == "lowest"
        assert WinComparison.FIRST.value == "first"
        assert WinComparison.NONE.value == "none"

    def test_trigger_mode_values(self):
        """TriggerMode enum has expected values."""
        from darwindeck.genome.schema import TriggerMode
        assert TriggerMode.IMMEDIATE.value == "immediate"
        assert TriggerMode.THRESHOLD_GATE.value == "threshold_gate"
        assert TriggerMode.ALL_HANDS_EMPTY.value == "all_hands_empty"
        assert TriggerMode.DECK_EMPTY.value == "deck_empty"


class TestGameRulesEnums:
    def test_pass_action_values(self):
        """PassAction enum has expected values."""
        from darwindeck.genome.schema import PassAction
        assert PassAction.NONE.value == "none"
        assert PassAction.CLEAR_TABLEAU.value == "clear_tableau"
        assert PassAction.END_ROUND.value == "end_round"
        assert PassAction.SKIP_PLAYER.value == "skip_player"

    def test_deck_empty_action_values(self):
        """DeckEmptyAction enum has expected values."""
        from darwindeck.genome.schema import DeckEmptyAction
        assert DeckEmptyAction.RESHUFFLE_DISCARD.value == "reshuffle_discard"
        assert DeckEmptyAction.GAME_ENDS.value == "game_ends"
        assert DeckEmptyAction.SKIP_DRAW.value == "skip_draw"

    def test_tie_breaker_values(self):
        """TieBreaker enum has expected values."""
        from darwindeck.genome.schema import TieBreaker
        assert TieBreaker.ACTIVE_PLAYER.value == "active_player"
        assert TieBreaker.ALTERNATING.value == "alternating"
        assert TieBreaker.SPLIT.value == "split"
        assert TieBreaker.BATTLE.value == "battle"


class TestGameRules:
    def test_game_rules_defaults(self):
        """GameRules has sensible defaults."""
        from darwindeck.genome.schema import GameRules, PassAction, DeckEmptyAction, TieBreaker
        rules = GameRules()
        assert rules.consecutive_pass_action == PassAction.NONE
        assert rules.deck_empty_action == DeckEmptyAction.RESHUFFLE_DISCARD
        assert rules.tie_breaker == TieBreaker.ACTIVE_PLAYER

    def test_game_rules_custom(self):
        """GameRules can be customized."""
        from darwindeck.genome.schema import GameRules, PassAction, DeckEmptyAction
        rules = GameRules(
            consecutive_pass_action=PassAction.CLEAR_TABLEAU,
            passes_to_trigger=3,
            deck_empty_action=DeckEmptyAction.GAME_ENDS,
        )
        assert rules.consecutive_pass_action == PassAction.CLEAR_TABLEAU
        assert rules.passes_to_trigger == 3

    def test_game_rules_frozen(self):
        """GameRules is immutable."""
        from darwindeck.genome.schema import GameRules, TieBreaker
        rules = GameRules()
        with pytest.raises(AttributeError):
            rules.tie_breaker = TieBreaker.SPLIT


class TestPhaseEnums:
    def test_claim_rank_mode_values(self):
        """ClaimRankMode enum has expected values."""
        from darwindeck.genome.schema import ClaimRankMode
        assert ClaimRankMode.SEQUENTIAL.value == "sequential"
        assert ClaimRankMode.PLAYER_CHOICE.value == "player_choice"
        assert ClaimRankMode.FIXED.value == "fixed"

    def test_breaking_rule_values(self):
        """BreakingRule enum has expected values."""
        from darwindeck.genome.schema import BreakingRule
        assert BreakingRule.NONE.value == "none"
        assert BreakingRule.CANNOT_LEAD_UNTIL_BROKEN.value == "cannot_lead_until_broken"
        assert BreakingRule.CANNOT_PLAY_UNTIL_BROKEN.value == "cannot_play_until_broken"


class TestShowdownMethod:
    def test_showdown_method_values(self):
        """ShowdownMethod enum has expected values."""
        from darwindeck.genome.schema import ShowdownMethod
        assert ShowdownMethod.HAND_EVALUATION.value == "hand_evaluation"
        assert ShowdownMethod.HIGHEST_CARD.value == "highest_card"
        assert ShowdownMethod.FOLD_ONLY.value == "fold_only"


class TestWinConditionExtended:
    def test_win_condition_new_fields(self):
        """WinCondition has new explicit fields."""
        from darwindeck.genome.schema import WinCondition, WinComparison, TriggerMode
        wc = WinCondition(
            type="low_score",
            threshold=100,
            comparison=WinComparison.LOWEST,
            trigger_mode=TriggerMode.THRESHOLD_GATE,
        )
        assert wc.comparison == WinComparison.LOWEST
        assert wc.trigger_mode == TriggerMode.THRESHOLD_GATE

    def test_win_condition_defaults(self):
        """WinCondition new fields have sensible defaults."""
        from darwindeck.genome.schema import WinCondition, WinComparison, TriggerMode
        wc = WinCondition(type="empty_hand")
        assert wc.comparison == WinComparison.NONE
        assert wc.trigger_mode == TriggerMode.IMMEDIATE
        assert wc.required_hand_size is None

    def test_win_condition_best_hand(self):
        """WinCondition for best_hand with required_hand_size."""
        from darwindeck.genome.schema import WinCondition, WinComparison
        wc = WinCondition(
            type="best_hand",
            comparison=WinComparison.HIGHEST,
            required_hand_size=5,
        )
        assert wc.required_hand_size == 5