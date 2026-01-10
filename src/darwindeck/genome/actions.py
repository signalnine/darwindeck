"""Three-tier action model for game moves."""

from dataclasses import dataclass
from enum import Enum
from typing import Optional
from darwindeck.genome.schema import Location
from darwindeck.genome.conditions import ConditionOrCompound


class ActionType(Enum):
    """Types of actions players can take."""

    DRAW_CARDS = "draw_cards"
    PLAY_CARD = "play_card"
    DISCARD_CARD = "discard_card"
    SKIP_TURN = "skip_turn"
    REVERSE_ORDER = "reverse_order"
    CHOOSE_SUIT = "choose_suit"
    TRANSFER_CARDS = "transfer_cards"
    ADD_SCORE = "add_score"
    PASS = "pass"

    # Trick-taking actions
    LEAD_CARD = "lead_card"              # First card of trick
    FOLLOW_SUIT = "follow_suit"          # Play card matching lead suit
    PLAY_TRUMP = "play_trump"            # Play trump card
    COLLECT_TRICK = "collect_trick"      # Winner takes trick cards
    SCORE_TRICK = "score_trick"          # Score points based on trick contents


@dataclass(frozen=True)
class PrimitiveAction:
    """Abstract action definition from genome."""

    action_type: ActionType
    source: Optional[Location] = None
    target: Optional[Location] = None
    count: Optional[int] = None
    condition: Optional[ConditionOrCompound] = None


@dataclass(frozen=True)
class ConcreteAction:
    """Action bound to specific cards."""

    primitive: PrimitiveAction
    card_indices: tuple[int, ...]  # Which cards (indices into hand/location)

    def __init__(
        self,
        primitive: PrimitiveAction,
        card_indices: tuple[int, ...] = ()
    ) -> None:
        object.__setattr__(self, "primitive", primitive)
        object.__setattr__(self, "card_indices", card_indices)
