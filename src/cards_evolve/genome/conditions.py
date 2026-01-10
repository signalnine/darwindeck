"""Condition system for composable game logic predicates."""

from dataclasses import dataclass
from enum import Enum
from typing import Optional, Union, List, Literal


class ConditionType(Enum):
    """Types of conditions that can be evaluated."""

    HAND_SIZE = "hand_size"
    CARD_MATCHES_RANK = "card_matches_rank"
    CARD_MATCHES_SUIT = "card_matches_suit"
    CARD_MATCHES_COLOR = "card_matches_color"
    CARD_IS_RANK = "card_is_rank"
    PLAYER_HAS_CARD = "player_has_card"
    LOCATION_EMPTY = "location_empty"
    LOCATION_SIZE = "location_size"
    SCORE_COMPARE = "score_compare"
    SEQUENCE_ADJACENT = "sequence_adjacent"

    # NEW: Wildcard matching
    MATCHES_OR_WILD = "matches_or_wild"  # Card matches rank/suit OR is wild

    # NEW: Pattern matching (for set collection games)
    HAS_SET_OF_N = "has_set_of_n"  # N cards of same rank
    HAS_RUN_OF_N = "has_run_of_n"  # N cards in sequence
    HAS_MATCHING_PAIR = "has_matching_pair"  # Two cards with matching property


class Operator(Enum):
    """Comparison operators."""

    EQ = "=="
    NE = "!="
    LT = "<"
    GT = ">"
    LE = "<="
    GE = ">="


@dataclass(frozen=True)
class Condition:
    """Single condition predicate."""

    type: ConditionType
    operator: Optional[Operator] = None
    value: Optional[Union[int, str]] = None
    reference: Optional[str] = None  # "top_discard", "last_played", etc.


@dataclass(frozen=True)
class CompoundCondition:
    """Combine conditions with AND/OR logic."""

    logic: Literal["AND", "OR"]
    conditions: tuple["ConditionOrCompound", ...]

    def __init__(
        self,
        logic: Literal["AND", "OR"],
        conditions: List["ConditionOrCompound"]
    ) -> None:
        # Convert list to tuple for immutability
        object.__setattr__(self, "logic", logic)
        object.__setattr__(self, "conditions", tuple(conditions))


# Type alias for nested conditions
ConditionOrCompound = Union[Condition, CompoundCondition]
