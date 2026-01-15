"""Terminal display for game state and moves."""

from __future__ import annotations

from darwindeck.simulation.state import GameState, Card
from darwindeck.simulation.movegen import LegalMove
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
        moves: list[LegalMove],
        state: GameState,
        genome: GameGenome,
    ) -> str:
        """Present moves in human-readable format."""
        if not moves:
            return "No legal moves available. Press Enter to pass."

        lines: list[str] = []

        # Determine phase type from first move
        if moves:
            phase_idx = moves[0].phase_index
            phase = genome.turn_structure.phases[phase_idx]

            if isinstance(phase, PlayPhase):
                lines.append(self._present_card_play(moves, state))
            elif isinstance(phase, BettingPhase):
                lines.append(self._present_betting(moves, state))
            else:
                lines.append(self._present_generic(moves))
        else:
            lines.append(self._present_generic(moves))

        lines.append("")
        lines.append("Enter choice or [q]uit:")

        return "\n".join(lines)

    def _present_card_play(self, moves: list[LegalMove], state: GameState) -> str:
        """Present card play options."""
        hand = state.players[state.active_player].hand
        options: list[str] = []

        for move in moves:
            if 0 <= move.card_index < len(hand):
                card = hand[move.card_index]
                options.append(f"[{move.card_index + 1}] {format_card(card)}")

        return "Play: " + "  ".join(options)

    def _present_betting(self, moves: list[LegalMove], state: GameState) -> str:
        """Present betting options."""
        # Betting actions encoded in card_index as negative values
        action_names = {
            -10: "Check",
            -11: "Bet",
            -12: "Call",
            -13: "Raise",
            -14: "All-In",
            -15: "Fold",
        }
        options: list[str] = []

        for i, move in enumerate(moves):
            name = action_names.get(move.card_index, f"Action {move.card_index}")
            options.append(f"[{i + 1}] {name}")

        return "Bet: " + "  ".join(options)

    def _present_generic(self, moves: list[LegalMove]) -> str:
        """Fallback for other move types."""
        options = [f"[{i + 1}] Move {i + 1}" for i in range(len(moves))]
        return "Choose: " + "  ".join(options)
