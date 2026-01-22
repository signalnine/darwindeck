"""Intermediate representation for display rendering.

This module provides dataclasses that decouple the rendering logic from game
internals. DisplayState acts as a ViewModel that:
1. Session builds from GameState
2. RichDisplay renders to terminal
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass
class MoveOption:
    """Simplified move for display.

    Represents a legal move in a format suitable for rendering,
    without exposing internal game state details.
    """

    index: int
    label: str  # "A\u2665", "Pass", "Fold", etc.
    move_type: str  # "card", "betting", "pass"


@dataclass
class DisplayState:
    """Intermediate representation for display rendering.

    Contains all information needed to render the game state to the terminal,
    decoupled from the internal GameState representation.
    """

    game_name: str
    turn: int
    phase_name: str
    player_id: int

    # Hand info
    hand_cards: list[tuple[str, str]]  # [(rank, suit), ...]

    # Opponent info
    opponent_card_count: int
    opponent_chips: int
    opponent_bet: int

    # Player resources
    player_chips: int
    player_bet: int
    pot: int
    current_bet: int

    # Table state
    discard_top: tuple[str, str] | None  # (rank, suit) or None

    # Available moves
    moves: list[MoveOption]  # Simplified move representation

    # History
    move_log: list[tuple[str, str]]  # [(direction, description), ...]

    # Terminal info
    terminal_width: int


__all__ = ["DisplayState", "MoveOption"]
