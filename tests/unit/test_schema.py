"""Tests for genome schema types."""

import pytest


def test_effect_type_enum():
    """EffectType enum has all expected values."""
    from darwindeck.genome.schema import EffectType

    assert EffectType.SKIP_NEXT.value == "skip_next"
    assert EffectType.REVERSE_DIRECTION.value == "reverse"
    assert EffectType.DRAW_CARDS.value == "draw_cards"
    assert EffectType.EXTRA_TURN.value == "extra_turn"
    assert EffectType.FORCE_DISCARD.value == "force_discard"


def test_special_effect_creation():
    """SpecialEffect dataclass is frozen and has correct fields."""
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector

    effect = SpecialEffect(
        trigger_rank=Rank.TWO,
        effect_type=EffectType.DRAW_CARDS,
        target=TargetSelector.NEXT_PLAYER,
        value=2
    )

    assert effect.trigger_rank == Rank.TWO
    assert effect.effect_type == EffectType.DRAW_CARDS
    assert effect.target == TargetSelector.NEXT_PLAYER
    assert effect.value == 2


def test_special_effect_default_value():
    """SpecialEffect value defaults to 1."""
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector

    effect = SpecialEffect(
        trigger_rank=Rank.JACK,
        effect_type=EffectType.SKIP_NEXT,
        target=TargetSelector.NEXT_PLAYER
    )

    assert effect.value == 1


def test_tableau_mode_enum_values():
    """TableauMode enum has expected values."""
    from darwindeck.genome.schema import TableauMode

    assert TableauMode.NONE.value == "none"
    assert TableauMode.WAR.value == "war"
    assert TableauMode.MATCH_RANK.value == "match_rank"
    assert TableauMode.SEQUENCE.value == "sequence"


def test_sequence_direction_enum_values():
    """SequenceDirection enum has expected values."""
    from darwindeck.genome.schema import SequenceDirection

    assert SequenceDirection.ASCENDING.value == "ascending"
    assert SequenceDirection.DESCENDING.value == "descending"
    assert SequenceDirection.BOTH.value == "both"


def test_setup_rules_tableau_mode_default():
    """SetupRules defaults tableau_mode to NONE."""
    from darwindeck.genome.schema import SetupRules, TableauMode, SequenceDirection

    setup = SetupRules(cards_per_player=7)
    assert setup.tableau_mode == TableauMode.NONE
    assert setup.sequence_direction == SequenceDirection.BOTH


def test_setup_rules_tableau_mode_explicit():
    """SetupRules accepts explicit tableau_mode."""
    from darwindeck.genome.schema import SetupRules, TableauMode, SequenceDirection

    setup = SetupRules(
        cards_per_player=7,
        tableau_mode=TableauMode.WAR,
        sequence_direction=SequenceDirection.ASCENDING
    )
    assert setup.tableau_mode == TableauMode.WAR
    assert setup.sequence_direction == SequenceDirection.ASCENDING


def test_bidding_phase_defaults():
    """BiddingPhase has correct defaults."""
    from darwindeck.genome.schema import BiddingPhase

    phase = BiddingPhase()
    assert phase.min_bid == 1
    assert phase.max_bid == 13
    assert phase.allow_nil == True


def test_contract_scoring_defaults():
    """ContractScoring has correct defaults."""
    from darwindeck.genome.schema import ContractScoring

    scoring = ContractScoring()
    assert scoring.points_per_trick_bid == 10
    assert scoring.overtrick_points == 1
    assert scoring.failed_contract_penalty == 10
    assert scoring.nil_bonus == 100
    assert scoring.nil_penalty == 100
    assert scoring.bag_limit == 10
    assert scoring.bag_penalty == 100
