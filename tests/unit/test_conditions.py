"""Tests for genome condition system."""

import pytest
from darwindeck.genome.conditions import (
    Condition,
    ConditionType,
    Operator,
    CompoundCondition,
)
from darwindeck.genome.schema import Rank, Suit


def test_simple_condition_creation() -> None:
    """Test creating a simple condition."""
    cond = Condition(
        type=ConditionType.CARD_MATCHES_RANK,
        reference="top_discard"
    )
    assert cond.type == ConditionType.CARD_MATCHES_RANK
    assert cond.reference == "top_discard"


def test_compound_condition_and_logic() -> None:
    """Test AND compound condition."""
    cond = CompoundCondition(
        logic="AND",
        conditions=[
            Condition(type=ConditionType.CARD_MATCHES_RANK, reference="top"),
            Condition(type=ConditionType.CARD_MATCHES_SUIT, reference="top"),
        ]
    )
    assert cond.logic == "AND"
    assert len(cond.conditions) == 2


def test_condition_with_value() -> None:
    """Test condition with comparison value."""
    cond = Condition(
        type=ConditionType.HAND_SIZE,
        operator=Operator.GT,
        value=5
    )
    assert cond.operator == Operator.GT
    assert cond.value == 5
