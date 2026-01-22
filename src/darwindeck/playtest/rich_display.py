"""Rich-based terminal display with Live updates.

This module provides a Rich-based display renderer that takes DisplayState
and renders it using Rich's Panel, Text, and Live components.
"""

from __future__ import annotations

import time
from typing import TYPE_CHECKING

from rich.console import Console, Group
from rich.live import Live
from rich.panel import Panel
from rich.text import Text

if TYPE_CHECKING:
    from darwindeck.playtest.display_state import DisplayState

# Constants
SUIT_SYMBOLS = {"H": "\u2665", "D": "\u2666", "C": "\u2663", "S": "\u2660"}
SUIT_COLORS = {"H": "red", "D": "red", "C": "default", "S": "default"}
MIN_WIDE_WIDTH = 60
MOVE_LOG_SIZE = 5


def format_card_rich(rank: str, suit: str) -> Text:
    """Format card with colored suit symbol.

    Args:
        rank: Card rank (e.g., "A", "K", "10")
        suit: Card suit single letter (e.g., "H", "S")

    Returns:
        Rich Text with styled rank and colored suit symbol
    """
    symbol = SUIT_SYMBOLS.get(suit, suit)
    color = SUIT_COLORS.get(suit, "default")

    text = Text()
    text.append(rank, style="bold")
    text.append(symbol, style=color)
    return text


class RichDisplay:
    """Rich-based terminal display with Live updates."""

    def __init__(self) -> None:
        """Initialize the display with a Rich Console."""
        self.console = Console()

    def render(self, state: "DisplayState") -> Panel:
        """Build complete display layout from DisplayState.

        Args:
            state: The DisplayState to render

        Returns:
            Rich Panel containing the complete layout
        """
        if state.terminal_width < MIN_WIDE_WIDTH:
            return self._render_compact(state)
        return self._render_wide(state)

    def show_and_prompt(self, state: "DisplayState", prompt: str = "Enter choice: ") -> str:
        """Render state, then prompt for input. Handles Live lifecycle safely.

        Uses transient=True so display clears after showing, then prompts
        for input outside the Live context.

        Args:
            state: The DisplayState to render
            prompt: The prompt string to display

        Returns:
            User input string
        """
        # Clear screen and render the state
        self.console.clear()
        panel = self.render(state)
        self.console.print(panel)

        # Prompt for input outside Live context
        return self.console.input(f"[bold green]{prompt}[/bold green]")

    def show_ai_turn(self, state: "DisplayState", duration: float = 0.3) -> None:
        """Show AI turn briefly with Live display.

        Args:
            state: The DisplayState to render
            duration: How long to show the AI turn (seconds)
        """
        self.console.clear()
        panel = self.render(state)

        with Live(panel, console=self.console, transient=True, refresh_per_second=4):
            time.sleep(duration)

    def show_message(self, message: str, style: str = "bold") -> None:
        """Show a standalone message (win/lose, errors).

        Args:
            message: The message to display
            style: Rich style string for the message
        """
        self.console.print(f"[{style}]{message}[/{style}]")

    def show_error(self, message: str) -> None:
        """Show error message in bold red.

        Args:
            message: The error message to display
        """
        self.console.print(f"[bold red]Error: {message}[/bold red]")

    def get_terminal_width(self) -> int:
        """Get current terminal width.

        Returns:
            Terminal width in characters
        """
        return self.console.width

    def _render_wide(self, state: "DisplayState") -> Panel:
        """Render full-width layout with panels.

        Args:
            state: The DisplayState to render

        Returns:
            Rich Panel with nested panels for each section
        """
        sections = []

        # Header panel
        header = self._build_header(state)
        sections.append(header)

        # Opponent panel
        opponent = self._build_opponent_panel(state)
        sections.append(opponent)

        # Hand panel
        hand = self._build_hand_panel(state)
        sections.append(hand)

        # Discard panel (if applicable)
        if state.discard_top is not None:
            discard = self._build_discard_panel(state)
            sections.append(discard)

        # Actions panel
        actions = self._build_actions_panel(state)
        sections.append(actions)

        # Move log (last N moves)
        if state.move_log:
            log = self._build_move_log(state)
            sections.append(log)

        # Combine all sections into a group
        group = Group(*sections)

        return Panel(
            group,
            title=f"[bold cyan]{state.game_name}[/bold cyan]",
            border_style="dim cyan",
        )

    def _render_compact(self, state: "DisplayState") -> Panel:
        """Render compact layout for narrow terminals.

        Args:
            state: The DisplayState to render

        Returns:
            Rich Panel with simpler text-based layout
        """
        text = Text()

        # Header line
        text.append(f"Turn {state.turn}", style="bold")
        text.append(f" | Phase: {state.phase_name}")
        text.append(f" | You: P{state.player_id}\n")

        # Chips/pot (if applicable)
        if state.player_chips > 0 or state.pot > 0:
            text.append(f"Chips: {state.player_chips} | Pot: {state.pot}")
            if state.current_bet > 0:
                text.append(f" | Bet: {state.current_bet}")
            text.append("\n")

        # Opponent
        text.append(f"Opponent: {state.opponent_card_count} cards")
        if state.opponent_chips > 0:
            text.append(f" | {state.opponent_chips} chips")
        text.append("\n\n")

        # Hand
        text.append("Hand: ", style="bold")
        for i, (rank, suit) in enumerate(state.hand_cards):
            text.append(f"[{i+1}]", style="bold green")
            text.append_text(format_card_rich(rank, suit))
            text.append("  ")
        text.append("\n")

        # Discard
        if state.discard_top is not None:
            rank, suit = state.discard_top
            text.append("Discard: ")
            text.append_text(format_card_rich(rank, suit))
            text.append("\n")

        text.append("\n")

        # Actions
        text.append("Actions: ", style="bold")
        for move in state.moves:
            text.append(f"[{move.index}]", style="bold green")
            text.append(f" {move.label}  ")
        text.append("\n")

        # Move log (condensed)
        if state.move_log:
            text.append("\n")
            recent = state.move_log[-MOVE_LOG_SIZE:]
            for direction, description in recent:
                arrow = "\u2190" if direction == "opponent" else "\u2192"
                text.append(f"{arrow} {description}  ", style="dim")

        return Panel(
            text,
            title=f"[bold cyan]{state.game_name}[/bold cyan]",
            border_style="dim cyan",
        )

    def _build_header(self, state: "DisplayState") -> Panel:
        """Build the header panel with turn, phase, and player info.

        Args:
            state: The DisplayState

        Returns:
            Panel containing header info
        """
        text = Text()
        text.append(f"Turn: {state.turn}", style="bold")
        text.append("  |  ")
        text.append(f"Phase: {state.phase_name}")
        text.append("  |  ")
        text.append(f"You: P{state.player_id}")

        # Add chips/pot info if applicable
        if state.player_chips > 0 or state.pot > 0:
            text.append("\n")
            text.append(f"Your chips: {state.player_chips}", style="bold yellow")
            text.append("  |  ")
            text.append(f"Pot: {state.pot}", style="bold green")
            if state.current_bet > 0:
                text.append("  |  ")
                text.append(f"Current bet: {state.current_bet}")
            if state.player_bet > 0:
                text.append("  |  ")
                text.append(f"Your bet: {state.player_bet}")

        return Panel(text, border_style="dim cyan")

    def _build_opponent_panel(self, state: "DisplayState") -> Panel:
        """Build the opponent info panel.

        Args:
            state: The DisplayState

        Returns:
            Panel containing opponent info
        """
        text = Text()
        text.append(f"Cards: {state.opponent_card_count}", style="bold")

        if state.opponent_chips > 0:
            text.append("  |  ")
            text.append(f"Chips: {state.opponent_chips}", style="yellow")

        if state.opponent_bet > 0:
            text.append("  |  ")
            text.append(f"Bet: {state.opponent_bet}")

        return Panel(text, title="[dim]Opponent[/dim]", border_style="dim cyan")

    def _build_hand_panel(self, state: "DisplayState") -> Panel:
        """Build the player's hand panel with numbered cards.

        Args:
            state: The DisplayState

        Returns:
            Panel containing the player's hand
        """
        if not state.hand_cards:
            text = Text("(empty hand)", style="dim italic")
        else:
            text = Text()
            for i, (rank, suit) in enumerate(state.hand_cards):
                text.append(f"[{i+1}]", style="bold green")
                text.append(" ")
                text.append_text(format_card_rich(rank, suit))
                text.append("   ")

        return Panel(text, title="[dim]Your Hand[/dim]", border_style="dim cyan")

    def _build_discard_panel(self, state: "DisplayState") -> Panel:
        """Build the discard pile panel.

        Args:
            state: The DisplayState

        Returns:
            Panel containing the top discard card
        """
        if state.discard_top is None:
            text = Text("(empty)", style="dim italic")
        else:
            rank, suit = state.discard_top
            text = Text("Top: ")
            text.append_text(format_card_rich(rank, suit))

        return Panel(text, title="[dim]Discard[/dim]", border_style="dim cyan")

    def _build_actions_panel(self, state: "DisplayState") -> Panel:
        """Build the actions panel with numbered options.

        Args:
            state: The DisplayState

        Returns:
            Panel containing available actions
        """
        if not state.moves:
            text = Text("No moves available", style="dim italic")
        else:
            text = Text()
            for move in state.moves:
                text.append(f"[{move.index}]", style="bold green")
                text.append(" ")
                # Format card labels with colors
                if move.move_type == "card" and len(move.label) >= 2:
                    # Parse rank and suit from label (e.g., "A\u2665")
                    rank = move.label[:-1]
                    suit_symbol = move.label[-1]
                    # Map symbol back to suit letter for coloring
                    suit_map = {"\u2665": "H", "\u2666": "D", "\u2663": "C", "\u2660": "S"}
                    suit = suit_map.get(suit_symbol, "")
                    if suit:
                        text.append_text(format_card_rich(rank, suit))
                    else:
                        text.append(move.label)
                else:
                    text.append(move.label, style="bold" if move.move_type == "betting" else "")
                text.append("   ")

        return Panel(text, title="[dim]Actions[/dim]", border_style="dim cyan")

    def _build_move_log(self, state: "DisplayState") -> Text:
        """Build the move log showing recent moves.

        Args:
            state: The DisplayState

        Returns:
            Text containing the move log
        """
        text = Text()

        # Show last N moves
        recent = state.move_log[-MOVE_LOG_SIZE:]

        for direction, description in recent:
            if direction == "opponent":
                text.append("\u2190 ", style="dim cyan")
                text.append(f"AI {description}", style="dim")
            else:
                text.append("\u2192 ", style="dim green")
                text.append(f"You {description}", style="dim")
            text.append("  ")

        return text


__all__ = ["RichDisplay", "format_card_rich", "SUIT_SYMBOLS", "SUIT_COLORS", "MIN_WIDE_WIDTH", "MOVE_LOG_SIZE"]
