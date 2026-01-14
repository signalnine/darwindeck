"""Immutable game state representation."""

from dataclasses import dataclass
from typing import Optional
from darwindeck.genome.schema import Rank, Suit


@dataclass(frozen=True)
class Card:
    """Immutable playing card."""

    rank: Rank
    suit: Suit

    def __str__(self) -> str:
        return f"{self.rank.value}{self.suit.value}"


@dataclass(frozen=True)
class PlayerState:
    """Immutable player state."""

    player_id: int
    hand: tuple[Card, ...]
    score: int
    # Betting fields (default to 0/False for non-betting games)
    chips: int = 0
    current_bet: int = 0
    has_folded: bool = False
    is_all_in: bool = False

    def copy_with(self, **changes) -> "PlayerState":  # type: ignore
        """Create new PlayerState with changes."""
        current = {
            "player_id": self.player_id,
            "hand": self.hand,
            "score": self.score,
            "chips": self.chips,
            "current_bet": self.current_bet,
            "has_folded": self.has_folded,
            "is_all_in": self.is_all_in,
        }
        current.update(changes)
        return PlayerState(**current)


@dataclass(frozen=True)
class GameState:
    """Immutable game state (hybrid design from consensus).

    Uses typed fields for common zones plus typed extensions.
    All nested structures are tuples for true immutability.
    """

    # Core state
    players: tuple[PlayerState, ...]
    deck: tuple[Card, ...]
    discard: tuple[Card, ...]
    turn: int
    active_player: int

    # Game-family specific (typed extensions, not Dict[str, Any])
    tableau: Optional[tuple[tuple[Card, ...], ...]] = None  # For solitaire-style
    community: Optional[tuple[Card, ...]] = None  # For poker-style

    def copy_with(self, **changes) -> "GameState":  # type: ignore
        """Create a new state with specified changes."""
        # Helper for making state transitions
        current = {
            "players": self.players,
            "deck": self.deck,
            "discard": self.discard,
            "turn": self.turn,
            "active_player": self.active_player,
            "tableau": self.tableau,
            "community": self.community,
        }
        current.update(changes)
        return GameState(**current)
