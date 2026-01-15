"""Terminal display for game state and moves."""

from __future__ import annotations

from typing import Union

from darwindeck.simulation.state import GameState, Card
from darwindeck.simulation.movegen import LegalMove, BettingMove, BettingAction
from darwindeck.genome.schema import GameGenome, Location, PlayPhase, BettingPhase


# Unicode card symbols
SUIT_SYMBOLS = {"H": "\u2665", "D": "\u2666", "C": "\u2663", "S": "\u2660"}


def format_card(card: Card) -> str:
    """Format card with unicode suit symbol."""
    suit_symbol = SUIT_SYMBOLS.get(card.suit.value, card.suit.value)
    return f"{card.rank.value}{suit_symbol}"


class StateRenderer:
    """Renders visible game state to terminal."""

    def render(
        self,
        state: GameState,
        genome: GameGenome,
        player_idx: int,
        debug: bool = False,
    ) -> str:
        """Render state from player's perspective."""
        lines: list[str] = []

        # Header
        lines.append(f"=== Turn {state.turn} ===")
        lines.append("")

        # Player's hand
        hand = state.players[player_idx].hand
        if hand:
            cards_str = "  ".join(
                f"[{i+1}] {format_card(card)}"
                for i, card in enumerate(hand)
            )
            lines.append(f"Your hand: {cards_str}")
        else:
            lines.append("Your hand: (empty)")

        # Discard pile (if genome uses it)
        if self._has_discard(genome) and state.discard:
            top = format_card(state.discard[-1])
            lines.append(f"Discard pile: {top}")

        # Show chips and pot if betting game
        if state.players[player_idx].chips > 0 or state.pot > 0:
            player_chips = state.players[player_idx].chips
            lines.append(f"Your chips: {player_chips} | Pot: {state.pot}")
            if state.current_bet > 0:
                lines.append(f"Current bet: {state.current_bet}")

        # Debug mode
        if debug:
            lines.append("")
            lines.append("--- Debug Info ---")
            for i, p in enumerate(state.players):
                if i != player_idx:
                    opp_cards = ", ".join(format_card(c) for c in p.hand)
                    lines.append(f"Player {i} hand: [{opp_cards}]")
            lines.append(f"Deck: {len(state.deck)} cards")

        return "\n".join(lines)

    def _has_discard(self, genome: GameGenome) -> bool:
        """Check if genome uses discard pile."""
        for phase in genome.turn_structure.phases:
            if isinstance(phase, PlayPhase) and phase.target == Location.DISCARD:
                return True
        return False


class MovePresenter:
    """Presents legal moves to human player."""

    def present(
        self,
        moves: list[Union[LegalMove, BettingMove]],
        state: GameState,
        genome: GameGenome,
    ) -> str:
        """Present moves in human-readable format."""
        if not moves:
            return "No legal moves available. Press Enter to pass."

        lines: list[str] = []

        # Separate moves by type (genome may have multiple phase types)
        card_moves = [m for m in moves if isinstance(m, LegalMove)]
        betting_moves = [m for m in moves if isinstance(m, BettingMove)]

        # Present card play options if any
        if card_moves:
            lines.append(self._present_card_play_indexed(card_moves, state, 0))

        # Present betting options if any
        if betting_moves:
            # Adjust indices to account for card moves
            lines.append(self._present_betting_indexed(betting_moves, state, len(card_moves)))

        # Fallback if neither type found
        if not card_moves and not betting_moves:
            lines.append(self._present_generic(moves))

        lines.append("")
        lines.append("Enter choice or [q]uit:")

        return "\n".join(lines)

    def _present_card_play(self, moves: list[LegalMove], state: GameState) -> str:
        """Present card play options."""
        return self._present_card_play_indexed(moves, state, 0)

    def _present_card_play_indexed(self, moves: list[LegalMove], state: GameState, offset: int) -> str:
        """Present card play options with index offset."""
        hand = state.players[state.active_player].hand
        options: list[str] = []

        for i, move in enumerate(moves):
            if 0 <= move.card_index < len(hand):
                card = hand[move.card_index]
                options.append(f"[{offset + i + 1}] {format_card(card)}")

        return "Play: " + "  ".join(options)

    def _present_betting(self, moves: list[BettingMove], state: GameState) -> str:
        """Present betting options."""
        return self._present_betting_indexed(moves, state, 0)

    def _present_betting_indexed(self, moves: list[BettingMove], state: GameState, offset: int) -> str:
        """Present betting options with index offset."""
        action_names = {
            BettingAction.CHECK: "Check",
            BettingAction.BET: "Bet",
            BettingAction.CALL: "Call",
            BettingAction.RAISE: "Raise",
            BettingAction.ALL_IN: "All-In",
            BettingAction.FOLD: "Fold",
        }
        options: list[str] = []

        for i, move in enumerate(moves):
            name = action_names.get(move.action, str(move.action))
            options.append(f"[{offset + i + 1}] {name}")

        return "Bet: " + "  ".join(options)

    def _present_generic(self, moves: list[LegalMove]) -> str:
        """Fallback for other move types."""
        options = [f"[{i + 1}] Move {i + 1}" for i in range(len(moves))]
        return "Choose: " + "  ".join(options)
