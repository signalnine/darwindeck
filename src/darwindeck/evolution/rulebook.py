"""Full rulebook generation from game genomes."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from darwindeck.genome.schema import GameGenome


@dataclass  # Intentionally mutable: sections are populated incrementally by extractor/LLM
class RulebookSections:
    """Intermediate representation of rulebook content.

    This dataclass serves as the bridge between genome extraction and
    markdown rendering. It captures all the structured information needed
    to produce a complete, human-readable rulebook.

    Attributes:
        game_name: The name of the game.
        player_count: Number of players the game supports.
        objective: How to win the game.
        overview: Brief description of the game (optional).
        components: List of required components (e.g., "Standard 52-card deck").
        setup_steps: Ordered list of setup instructions.
        phases: List of (phase_name, phase_description) tuples.
        special_rules: Any special rules or exceptions.
        edge_cases: How to handle edge cases (e.g., empty deck).
        quick_reference: Condensed summary for quick lookup.
    """

    game_name: str
    player_count: int
    objective: str

    # Optional sections (filled by extraction or LLM)
    overview: Optional[str] = None
    components: list[str] = field(default_factory=list)
    setup_steps: list[str] = field(default_factory=list)
    phases: list[tuple[str, str]] = field(default_factory=list)  # (name, description)
    special_rules: list[str] = field(default_factory=list)
    edge_cases: list[str] = field(default_factory=list)
    quick_reference: Optional[str] = None


@dataclass
class ValidationResult:
    """Result of genome or output validation."""
    valid: bool
    errors: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)


class GenomeValidator:
    """Pre-extraction validation for genome feasibility."""

    def validate(self, genome: "GameGenome") -> ValidationResult:
        """Check genome can produce a playable rulebook."""
        from darwindeck.genome.schema import BettingPhase

        errors = []
        warnings = []

        # Card count feasibility
        total_cards_needed = genome.setup.cards_per_player * genome.player_count
        total_cards_needed += genome.setup.initial_discard_count
        if total_cards_needed > 52:
            errors.append(
                f"Setup requires {total_cards_needed} cards but deck only has 52"
            )

        # Betting requires chips
        has_betting = any(
            isinstance(p, BettingPhase) for p in genome.turn_structure.phases
        )
        if has_betting and genome.setup.starting_chips == 0:
            errors.append("BettingPhase present but starting_chips is 0")

        # Must have win conditions
        if not genome.win_conditions:
            errors.append("No win conditions defined")

        # Must have phases
        if not genome.turn_structure.phases:
            errors.append("No phases defined in turn structure")

        return ValidationResult(
            valid=len(errors) == 0,
            errors=errors,
            warnings=warnings
        )
