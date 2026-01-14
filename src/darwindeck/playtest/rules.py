"""Rule explanation from genomes."""

from __future__ import annotations

from darwindeck.genome.schema import (
    GameGenome, PlayPhase, DrawPhase, DiscardPhase,
    BettingPhase, TrickPhase, ClaimPhase
)


class RuleExplainer:
    """Explains game rules from genome."""

    def explain_rules(self, genome: GameGenome) -> str:
        """Generate condensed rule summary."""
        lines: list[str] = []

        lines.append(f"=== {genome.genome_id} ===")
        lines.append("")

        # Win condition
        lines.append(f"Goal: {self._explain_win_condition(genome)}")

        # Setup
        lines.append(f"Setup: Each player gets {genome.setup.cards_per_player} cards")

        # Turn structure
        lines.append(f"Turn: {self._explain_turn_structure(genome)}")

        # Betting (if applicable)
        if genome.setup.starting_chips > 0:
            lines.append(f"Chips: Start with {genome.setup.starting_chips}")
            min_bet = self._find_min_bet(genome)
            if min_bet:
                lines.append(f"Betting: Minimum bet is {min_bet}")

        return "\n".join(lines)

    def explain_phase(self, phase_idx: int, genome: GameGenome) -> str:
        """Explain current phase to player."""
        if phase_idx >= len(genome.turn_structure.phases):
            return "Unknown phase"

        phase = genome.turn_structure.phases[phase_idx]
        return f"Phase: {self._phase_description(phase)}"

    def _explain_win_condition(self, genome: GameGenome) -> str:
        """Describe win condition(s)."""
        descriptions: list[str] = []

        for wc in genome.win_conditions:
            if wc.type == "empty_hand":
                descriptions.append("Empty your hand to win")
            elif wc.type == "capture_all":
                descriptions.append("Capture all cards to win")
            elif wc.type == "first_to_score":
                threshold = wc.threshold or 100
                descriptions.append(f"First to {threshold} points wins")
            elif wc.type == "high_score":
                descriptions.append("Highest score wins")
            else:
                descriptions.append(f"Win by: {wc.type}")

        return "; ".join(descriptions) if descriptions else "Unknown"

    def _explain_turn_structure(self, genome: GameGenome) -> str:
        """Describe turn phases."""
        phase_descs: list[str] = []

        for phase in genome.turn_structure.phases:
            phase_descs.append(self._phase_description(phase))

        return " -> ".join(phase_descs) if phase_descs else "Unknown"

    def _phase_description(self, phase) -> str:
        """Get short description for a phase."""
        if isinstance(phase, PlayPhase):
            return f"Play card to {phase.target.value}"
        elif isinstance(phase, DrawPhase):
            return f"Draw {phase.count} from {phase.source.value}"
        elif isinstance(phase, DiscardPhase):
            return f"Discard {phase.count}"
        elif isinstance(phase, BettingPhase):
            return "Betting round"
        elif isinstance(phase, TrickPhase):
            return "Play trick"
        elif isinstance(phase, ClaimPhase):
            return "Claim cards"
        else:
            return "Unknown action"

    def _find_min_bet(self, genome: GameGenome) -> int | None:
        """Find minimum bet from betting phases."""
        for phase in genome.turn_structure.phases:
            if isinstance(phase, BettingPhase):
                return phase.min_bet
        return None
