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
