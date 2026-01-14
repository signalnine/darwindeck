"""Semantic coherence checking for evolved genomes."""

from dataclasses import dataclass
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from darwindeck.genome.schema import GameGenome


@dataclass
class CoherenceResult:
    """Result of semantic coherence check."""
    coherent: bool
    violations: list[str]


class SemanticCoherenceChecker:
    """Validates genome has internally consistent mechanics."""

    CAPTURE_WIN_CONDITIONS = frozenset({"capture_all", "most_captured"})

    def check(self, genome: "GameGenome") -> CoherenceResult:
        """Check genome for semantic coherence."""
        violations = []
        violations.extend(self._check_win_conditions(genome))
        return CoherenceResult(
            coherent=len(violations) == 0,
            violations=violations
        )

    def _check_win_conditions(self, genome: "GameGenome") -> list[str]:
        """Check win conditions have supporting mechanics."""
        from darwindeck.genome.schema import PlayPhase, Location

        violations = []

        has_tableau_phase = any(
            isinstance(p, PlayPhase) and p.target == Location.TABLEAU
            for p in genome.turn_structure.phases
        )

        for wc in genome.win_conditions:
            if wc.type in self.CAPTURE_WIN_CONDITIONS:
                if not has_tableau_phase:
                    violations.append(
                        f"Win condition '{wc.type}' requires PlayPhase targeting TABLEAU"
                    )

        return violations
