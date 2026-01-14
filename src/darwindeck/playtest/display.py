"""Terminal display for game state and moves."""

from __future__ import annotations

from darwindeck.simulation.state import GameState, Card
from darwindeck.genome.schema import GameGenome, Location, PlayPhase


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

        # Chips (if betting game)
        if genome.setup.starting_chips > 0:
            player = state.players[player_idx]
            # PlayerState may not have chips attr in base version
            chips = getattr(player, "chips", genome.setup.starting_chips)
            lines.append(f"Your chips: {chips}")

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
