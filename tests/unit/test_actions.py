"""Tests for three-tier action model."""

import pytest
from cards_evolve.genome.actions import (
    ActionType,
    PrimitiveAction,
    ConcreteAction,
    Location,
)


def test_primitive_action_creation() -> None:
    """Test creating a primitive action."""
    action = PrimitiveAction(
        action_type=ActionType.DRAW_CARDS,
        source=Location.DECK,
        count=1
    )
    assert action.action_type == ActionType.DRAW_CARDS
    assert action.count == 1


def test_concrete_action_with_card_indices() -> None:
    """Test concrete action binds specific cards."""
    primitive = PrimitiveAction(
        action_type=ActionType.PLAY_CARD,
        target=Location.DISCARD
    )
    concrete = ConcreteAction(
        primitive=primitive,
        card_indices=(0,)  # Play card at index 0
    )
    assert concrete.primitive.action_type == ActionType.PLAY_CARD
    assert concrete.card_indices == (0,)


def test_action_immutability() -> None:
    """Test actions are immutable."""
    action = PrimitiveAction(
        action_type=ActionType.PASS
    )
    with pytest.raises(AttributeError):
        action.action_type = ActionType.DRAW_CARDS  # type: ignore
