"""Core genome schema types and enumerations."""

from __future__ import annotations

from enum import Enum
from typing import List, Optional, Union, TYPE_CHECKING
from dataclasses import dataclass, field

if TYPE_CHECKING:
    from darwindeck.genome.conditions import ConditionOrCompound


class Rank(Enum):
    """Playing card ranks."""

    ACE = "A"
    TWO = "2"
    THREE = "3"
    FOUR = "4"
    FIVE = "5"
    SIX = "6"
    SEVEN = "7"
    EIGHT = "8"
    NINE = "9"
    TEN = "10"
    JACK = "J"
    QUEEN = "Q"
    KING = "K"


class Suit(Enum):
    """Playing card suits."""

    HEARTS = "H"
    DIAMONDS = "D"
    CLUBS = "C"
    SPADES = "S"


class Location(Enum):
    """Card locations in game."""

    DECK = "deck"
    HAND = "hand"
    DISCARD = "discard"
    TABLEAU = "tableau"
    # Optional extensions for opponent interaction
    OPPONENT_HAND = "opponent_hand"
    OPPONENT_DISCARD = "opponent_discard"


class TargetSelector(Enum):
    """Target selection for opponent-directed actions."""

    NEXT_PLAYER = "next_player"          # Clockwise
    PREV_PLAYER = "prev_player"          # Counter-clockwise
    PLAYER_CHOICE = "player_choice"      # Active player chooses target
    RANDOM_OPPONENT = "random_opponent"  # Random selection
    ALL_OPPONENTS = "all_opponents"      # Broadcast to all
    LEFT_OPPONENT = "left_opponent"      # Physical left (3+ players)
    RIGHT_OPPONENT = "right_opponent"    # Physical right (3+ players)


class Visibility(Enum):
    """Card visibility state."""

    FACE_DOWN = "face_down"    # No one can see
    FACE_UP = "face_up"        # Everyone can see
    OWNER_ONLY = "owner_only"  # Only owning player can see
    REVEALED = "revealed"      # Temporarily shown to all


@dataclass(frozen=True)
class SetupRules:
    """Initial game configuration."""

    cards_per_player: int
    initial_deck: str = "standard_52"
    initial_discard_count: int = 0
    # NEW: Wildcard support
    wild_cards: tuple[Rank, ...] = ()
    # NEW: Visibility defaults
    hand_visibility: Visibility = Visibility.OWNER_ONLY
    deck_visibility: Visibility = Visibility.FACE_DOWN
    discard_visibility: Visibility = Visibility.FACE_UP
    # NEW: Trick-taking support
    trump_suit: Optional[Suit] = None        # Fixed trump (e.g., Spades in some variants)
    rotate_trump: bool = False               # Trump changes each hand
    random_trump: bool = False               # Trump selected randomly

    def __post_init__(self):
        """Convert lists to tuples for immutability."""
        if isinstance(self.wild_cards, list):
            object.__setattr__(self, "wild_cards", tuple(self.wild_cards))


@dataclass(frozen=True)
class PlayPhase:
    """Play cards from hand."""

    target: Location
    valid_play_condition: "ConditionOrCompound"  # type: ignore
    min_cards: int = 1
    max_cards: int = 1
    mandatory: bool = True
    pass_if_unable: bool = True


@dataclass(frozen=True)
class DrawPhase:
    """Draw cards from a location."""

    source: Location
    count: int = 1
    mandatory: bool = True
    condition: Optional["ConditionOrCompound"] = None  # type: ignore


@dataclass(frozen=True)
class DiscardPhase:
    """Discard cards to a location."""

    target: Location
    count: int = 1
    mandatory: bool = False
    matching_condition: Optional["ConditionOrCompound"] = None  # type: ignore


@dataclass(frozen=True)
class TrickPhase:
    """
    Trick-taking phase for games like Hearts, Spades, Bridge.

    A trick consists of each player playing one card in turn.
    The highest card (considering trump) wins the trick.
    """
    lead_suit_required: bool = True  # Must follow suit if able
    trump_suit: Optional[Suit] = None  # Trump overrides suit hierarchy
    high_card_wins: bool = True  # False for "lowest card wins" variants

    # Optional: special rules
    breaking_suit: Optional[Suit] = None  # Suit that cannot be played until "broken" (Hearts in Hearts)

    def __post_init__(self):
        """Convert lists to tuples for immutability."""
        # No mutable fields currently, but keep pattern for consistency
        pass


@dataclass(frozen=True)
class TurnStructure:
    """Ordered phases within a turn."""

    phases: tuple["Phase", ...]
    # NEW: Trick-taking game structure
    is_trick_based: bool = False  # True for Hearts, Spades, etc.
    tricks_per_hand: Optional[int] = None  # Number of tricks in a hand (e.g., 13 for Hearts)

    def __init__(self, phases: list, is_trick_based: bool = False, tricks_per_hand: Optional[int] = None) -> None:  # type: ignore
        object.__setattr__(self, "phases", tuple(phases))
        object.__setattr__(self, "is_trick_based", is_trick_based)
        object.__setattr__(self, "tricks_per_hand", tricks_per_hand)


@dataclass(frozen=True)
class WinCondition:
    """How to win the game."""

    type: str  # "empty_hand", "high_score", "first_to_score", "capture_all"
    threshold: Optional[int] = None


@dataclass(frozen=True)
class GameGenome:
    """Complete game specification."""

    schema_version: str
    genome_id: str
    generation: int
    setup: SetupRules
    turn_structure: TurnStructure
    special_effects: list  # type: ignore
    win_conditions: list[WinCondition]
    scoring_rules: list  # type: ignore
    max_turns: int = 100  # NEW: Termination guarantee (range: min_turns to 10000)
    player_count: int = 2
    # NEW: Validation constraints
    min_turns: int = 10  # Games ending too quickly are boring
