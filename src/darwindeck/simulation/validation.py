"""Degenerate game detection (conservative initial approach)."""

from typing import List
from darwindeck.genome.schema import GameGenome
from darwindeck.simulation.engine import GameResult


class DegenGameDetector:
    """Detects degenerate (trivial/broken) games.

    Conservative approach from consensus:
    - Too short: games that end immediately
    - State equivalence only (no outcome-based detection yet)
    """

    def __init__(self, genome: GameGenome) -> None:
        self.genome = genome
        # Claude's formula: max(5, deck_size / (2 * player_count))
        deck_size = 52 if genome.setup.initial_deck == "standard_52" else 52
        self.min_turns = max(5, deck_size // (2 * genome.player_count))

    def is_degenerate(self, results: List[GameResult]) -> bool:
        """Detect if game is degenerate.

        Args:
            results: List of game simulation results

        Returns:
            True if game appears degenerate
        """
        if not results:
            return True

        avg_length = sum(r.turn_count for r in results) / len(results)

        # Too short (games end immediately)
        if avg_length < self.min_turns:
            return True

        # TODO: Add more detection when we have:
        # - Ending variety (requires game outcome classification)
        # - Positional balance (requires win rate by player position)

        return False
