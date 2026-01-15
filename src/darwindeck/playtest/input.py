"""Human input handling."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional, Union

from darwindeck.simulation.movegen import LegalMove, BettingMove


@dataclass
class InputResult:
    """Result of human input."""

    move: Optional[Union[LegalMove, BettingMove]] = None
    quit: bool = False
    is_pass: bool = False
    error: Optional[str] = None


class HumanPlayer:
    """Handles human player input."""

    def get_move(self, moves: list[Union[LegalMove, BettingMove]], prompt: str = "> ") -> InputResult:
        """Get move from human input.

        Args:
            moves: List of legal moves (empty means must pass)
            prompt: Input prompt string

        Returns:
            InputResult with move, quit flag, or error
        """
        # Handle no legal moves (pass)
        if not moves:
            try:
                input("No moves available. Press Enter to pass...")
            except (EOFError, KeyboardInterrupt):
                return InputResult(quit=True)
            return InputResult(is_pass=True)

        try:
            raw = input(prompt).strip().lower()
        except (EOFError, KeyboardInterrupt):
            return InputResult(quit=True)

        # Check for quit
        if raw in ("q", "quit", "exit"):
            return InputResult(quit=True)

        # Check for pass (empty input)
        if not raw:
            return InputResult(is_pass=True)

        # Parse number
        try:
            choice = int(raw)
        except ValueError:
            return InputResult(error=f"Invalid input '{raw}'. Enter a number or 'q'.")

        # Validate range (1-indexed for human)
        if choice < 1 or choice > len(moves):
            return InputResult(error=f"Invalid choice {choice}. Enter 1-{len(moves)}.")

        return InputResult(move=moves[choice - 1])

    def get_yes_no(self, prompt: str) -> Optional[bool]:
        """Get yes/no response.

        Returns:
            True for yes, False for no, None for quit/cancel
        """
        try:
            raw = input(prompt).strip().lower()
        except (EOFError, KeyboardInterrupt):
            return None

        if raw in ("y", "yes"):
            return True
        if raw in ("n", "no"):
            return False
        return None

    def get_rating(self, prompt: str = "Rate this game [1-5]: ") -> Optional[int]:
        """Get numeric rating 1-5.

        Returns:
            Rating 1-5, or None for skip/cancel
        """
        try:
            raw = input(prompt).strip()
        except (EOFError, KeyboardInterrupt):
            return None

        if not raw:
            return None

        try:
            rating = int(raw)
            if 1 <= rating <= 5:
                return rating
        except ValueError:
            pass

        return None

    def get_comment(self, prompt: str = "Comments (Enter to skip): ") -> str:
        """Get optional comment."""
        try:
            return input(prompt).strip()
        except (EOFError, KeyboardInterrupt):
            return ""
