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


class EffectType(Enum):
    """Types of immediate effects a card can trigger."""
    SKIP_NEXT = "skip_next"
    REVERSE_DIRECTION = "reverse"
    DRAW_CARDS = "draw_cards"
    EXTRA_TURN = "extra_turn"
    FORCE_DISCARD = "force_discard"


class BettingAction(Enum):
    """Actions available during a betting phase."""
    CHECK = "check"      # Pass without betting (only if no current bet)
    BET = "bet"          # Place initial bet (min_bet amount)
    CALL = "call"        # Match current bet
    RAISE = "raise"      # Increase bet by min_bet
    ALL_IN = "all_in"    # Bet all remaining chips
    FOLD = "fold"        # Surrender hand, forfeit pot


class ScoringTrigger(Enum):
    """When scoring happens for a card."""
    TRICK_WIN = "trick_win"       # Score when winning trick with this card
    CAPTURE = "capture"           # Score when capturing this card
    PLAY = "play"                 # Score when playing this card
    HAND_END = "hand_end"         # Score for cards in hand at end
    SET_COMPLETE = "set_complete" # Score when completing a set (Go Fish)


@dataclass(frozen=True)
class CardCondition:
    """Condition to match a card by suit and/or rank."""
    suit: Optional[Suit] = None
    rank: Optional[Rank] = None


@dataclass(frozen=True)
class CardScoringRule:
    """Score points when a card meets a condition."""
    condition: CardCondition
    points: int
    trigger: ScoringTrigger


class HandEvaluationMethod(Enum):
    """How to evaluate and compare hands."""
    NONE = "none"
    HIGH_CARD = "high_card"          # Compare highest cards
    POINT_TOTAL = "point_total"      # Sum card values (Blackjack)
    PATTERN_MATCH = "pattern_match"  # Match patterns in priority order
    CARD_COUNT = "card_count"        # Most cards wins (War)


@dataclass(frozen=True)
class CardValue:
    """Point value for a card rank."""
    rank: Rank
    value: int
    alternate_value: Optional[int] = None


@dataclass(frozen=True)
class HandPattern:
    """A pattern to match in a hand. Fully describes what to look for."""
    name: str
    rank_priority: int  # Higher = better hand (100 > 50)

    # Constraints (all must be satisfied)
    required_count: Optional[int] = None   # Exactly N cards
    same_suit_count: Optional[int] = None  # N cards must share suit
    same_rank_groups: Optional[tuple[int, ...]] = None  # (3, 2) = three + pair
    sequence_length: Optional[int] = None  # N consecutive ranks
    sequence_wrap: bool = False            # A-2-3 and Q-K-A both valid
    required_ranks: Optional[tuple[Rank, ...]] = None  # Must contain these ranks


@dataclass(frozen=True)
class HandEvaluation:
    """How to evaluate and compare hands for winning."""
    method: HandEvaluationMethod

    # For PATTERN_MATCH method
    patterns: Optional[tuple[HandPattern, ...]] = None

    # For POINT_TOTAL method (Blackjack)
    card_values: Optional[tuple[CardValue, ...]] = None
    target_value: Optional[int] = None      # Optimal score (21 in Blackjack)
    bust_threshold: Optional[int] = None    # Score that loses (22 in Blackjack)


class WinComparison(Enum):
    """How scores are compared for winning."""
    HIGHEST = "highest"  # Highest score wins (poker, most tricks)
    LOWEST = "lowest"    # Lowest score wins (Hearts, Golf)
    FIRST = "first"      # First to reach threshold wins
    NONE = "none"        # No comparison (empty_hand, capture_all)


class TriggerMode(Enum):
    """When win condition is checked."""
    IMMEDIATE = "immediate"            # Check after every action
    THRESHOLD_GATE = "threshold_gate"  # Only check when threshold reached
    ALL_HANDS_EMPTY = "all_hands_empty"  # Check when all hands empty
    DECK_EMPTY = "deck_empty"          # Check when deck is exhausted


class PassAction(Enum):
    """What happens when players pass consecutively."""
    NONE = "none"                    # No special action
    CLEAR_TABLEAU = "clear_tableau"  # Clear cards from tableau
    END_ROUND = "end_round"          # End the current round
    SKIP_PLAYER = "skip_player"      # Skip to next player


class DeckEmptyAction(Enum):
    """What happens when deck is empty."""
    RESHUFFLE_DISCARD = "reshuffle_discard"  # Shuffle discard to form new deck
    GAME_ENDS = "game_ends"                  # Game ends immediately
    SKIP_DRAW = "skip_draw"                  # Skip draw phase


class TieBreaker(Enum):
    """How ties are resolved."""
    ACTIVE_PLAYER = "active_player"  # Active player wins ties
    ALTERNATING = "alternating"      # Alternate who wins ties
    SPLIT = "split"                  # Split the winnings
    BATTLE = "battle"                # Play tiebreaker round (War)


@dataclass(frozen=True)
class GameRules:
    """Explicit rules for edge cases."""
    consecutive_pass_action: PassAction = PassAction.NONE
    passes_to_trigger: Optional[int] = None  # None = num_players - 1
    deck_empty_action: DeckEmptyAction = DeckEmptyAction.RESHUFFLE_DISCARD
    keep_top_discard: bool = True
    tie_breaker: TieBreaker = TieBreaker.ACTIVE_PLAYER
    same_player_on_win: bool = False


class TableauMode(Enum):
    """How cards on the tableau interact."""
    NONE = "none"              # Cards accumulate, no interaction
    WAR = "war"                # Compare cards, winner takes all (2-player only)
    MATCH_RANK = "match_rank"  # Matching rank captures
    SEQUENCE = "sequence"      # Build ascending/descending piles


class SequenceDirection(Enum):
    """Direction for SEQUENCE tableau mode."""
    ASCENDING = "ascending"
    DESCENDING = "descending"
    BOTH = "both"


@dataclass(frozen=True)
class SpecialEffect:
    """A card-triggered immediate effect."""
    trigger_rank: Rank
    effect_type: EffectType
    target: TargetSelector
    value: int = 1


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
    # NEW: Betting support
    starting_chips: int = 0                  # 0 means no betting enabled
    # NEW: Custom deck support (reduces special effects complexity)
    custom_printed_deck: bool = False        # True for Uno-style decks with effects printed on cards
    # NEW: Tableau interaction mode
    tableau_mode: TableauMode = TableauMode.NONE
    sequence_direction: SequenceDirection = SequenceDirection.BOTH

    def __post_init__(self):
        """Convert lists to tuples for immutability."""
        if isinstance(self.wild_cards, list):
            object.__setattr__(self, "wild_cards", tuple(self.wild_cards))


@dataclass(frozen=True)
class PlayPhase:
    """Play cards from hand."""

    target: Location
    valid_play_condition: Optional["ConditionOrCompound"] = None  # type: ignore
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
class BettingPhase:
    """A betting round within the turn structure."""
    min_bet: int = 10       # Minimum bet/raise amount
    max_raises: int = 3     # Maximum raises per round (prevents infinite loops)


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
class ClaimPhase:
    """
    Bluffing/claiming phase for games like Cheat/BS/I Doubt It.

    Players play cards face-down and claim what they are.
    Other players can challenge the claim. If challenged:
    - If claim was TRUE: challenger takes the discard pile
    - If claim was FALSE: claimer takes the discard pile

    The claimed rank typically follows a sequence (A, 2, 3, ... K, A, 2, ...).
    """
    min_cards: int = 1  # Minimum cards to play per claim
    max_cards: int = 4  # Maximum cards to play per claim
    sequential_rank: bool = True  # Must claim in order (A, 2, 3, ..., K, A, ...)
    allow_challenge: bool = True  # Opponents can challenge claims
    pile_penalty: bool = True  # Loser takes discard pile (vs just revealing)


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
