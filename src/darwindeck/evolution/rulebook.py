"""Full rulebook generation from game genomes."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from darwindeck.genome.schema import GameGenome


@dataclass
class EdgeCaseDefault:
    """A default edge case rule."""
    name: str
    rule: str


# Define all available defaults
DECK_EXHAUSTION = EdgeCaseDefault(
    name="deck_exhaustion",
    rule="**Empty deck:** Shuffle the discard pile (except top card) to form a new deck. If still empty, skip the draw."
)

NO_VALID_PLAYS = EdgeCaseDefault(
    name="no_valid_plays",
    rule="**No valid plays:** Draw up to 3 cards until you can play, then pass if still unable."
)

SIMULTANEOUS_WIN = EdgeCaseDefault(
    name="simultaneous_win",
    rule="**Tie:** If multiple players meet win conditions simultaneously, the active player wins."
)

HAND_LIMIT = EdgeCaseDefault(
    name="hand_limit",
    rule="**Hand limit:** If your hand exceeds 15 cards, discard down to 15 at end of turn."
)

BETTING_ALL_IN = EdgeCaseDefault(
    name="betting_all_in",
    rule="**All-in:** If you can't afford to call, you may go all-in with remaining chips."
)

BETTING_POT_SPLIT = EdgeCaseDefault(
    name="betting_pot_split",
    rule="**Pot split:** If the pot can't split evenly, odd chips go to the player left of dealer."
)

TURN_LIMIT = EdgeCaseDefault(
    name="turn_limit",
    rule="**Turn limit:** If max turns reached, highest score wins (or draw if no scoring)."
)


def select_applicable_defaults(genome: "GameGenome") -> list[EdgeCaseDefault]:
    """Select edge case defaults that don't conflict with genome mechanics."""
    from darwindeck.genome.schema import BettingPhase, PlayPhase

    defaults = []

    # Check win condition types
    win_types = {wc.type for wc in genome.win_conditions}

    # Deck exhaustion - skip if it's a win condition
    if not win_types & {"deck_empty", "last_card"}:
        defaults.append(DECK_EXHAUSTION)

    # No valid plays - skip if genome has optional play (min=0)
    has_optional_play = any(
        isinstance(p, PlayPhase) and p.min_cards == 0
        for p in genome.turn_structure.phases
    )
    if not has_optional_play:
        defaults.append(NO_VALID_PLAYS)

    # Simultaneous win - always applies
    defaults.append(SIMULTANEOUS_WIN)

    # Hand limit - skip for accumulation games
    if not win_types & {"capture_all", "most_cards", "most_captured"}:
        defaults.append(HAND_LIMIT)

    # Betting defaults - only if betting phases exist
    has_betting = any(
        isinstance(p, BettingPhase) for p in genome.turn_structure.phases
    )
    if has_betting:
        defaults.append(BETTING_ALL_IN)
        defaults.append(BETTING_POT_SPLIT)

    # Turn limit - always applies
    defaults.append(TURN_LIMIT)

    return defaults


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


class GenomeExtractor:
    """Deterministic extraction of rules from genome fields."""

    # Win condition type to human-readable text
    WIN_CONDITION_TEXT = {
        "empty_hand": "First player to empty their hand wins",
        "high_score": "Player with the highest score wins",
        "low_score": "Player with the lowest score wins",
        "capture_all": "Capture all cards to win",
        "most_tricks": "Player who wins the most tricks wins",
        "fewest_tricks": "Player who wins the fewest tricks wins",
        "most_chips": "Player with the most chips wins",
        "most_captured": "Player who captures the most cards wins",
        "first_to_score": "First player to reach the target score wins",
    }

    def extract(self, genome: "GameGenome") -> RulebookSections:
        """Extract rulebook sections from genome."""
        return RulebookSections(
            game_name=genome.genome_id,
            player_count=genome.player_count,
            objective=self._extract_objective(genome),
            components=self._extract_components(genome),
            setup_steps=self._extract_setup(genome),
            phases=self._extract_phases(genome),
            special_rules=self._extract_special_rules(genome),
        )

    def _extract_components(self, genome: "GameGenome") -> list[str]:
        """Extract required components."""
        components = [f"Standard 52-card deck ({genome.player_count} players)"]
        if genome.setup.starting_chips > 0:
            components.append(f"Chips or tokens ({genome.setup.starting_chips} per player)")
        if any(wc.type in ("high_score", "low_score", "first_to_score") for wc in genome.win_conditions):
            components.append("Score tracking (pen and paper)")
        return components

    def _extract_setup(self, genome: "GameGenome") -> list[str]:
        """Extract setup steps."""
        steps = ["Shuffle the deck"]

        # Deal cards
        steps.append(f"Deal {genome.setup.cards_per_player} cards to each player")

        # Initial discard
        if genome.setup.initial_discard_count > 0:
            if genome.setup.initial_discard_count == 1:
                steps.append("Place 1 card face-up to start the discard pile")
            else:
                steps.append(f"Place {genome.setup.initial_discard_count} cards face-up to start the discard pile")

        # Chips
        if genome.setup.starting_chips > 0:
            steps.append(f"Give each player {genome.setup.starting_chips} chips")

        # Remaining deck
        steps.append("Place remaining cards face-down as the draw pile")

        return steps

    def _extract_objective(self, genome: "GameGenome") -> str:
        """Extract win conditions as objective text."""
        if not genome.win_conditions:
            return "Win the game"

        objectives = []
        for wc in genome.win_conditions:
            text = self.WIN_CONDITION_TEXT.get(wc.type, f"Meet the {wc.type} condition")
            if wc.threshold:
                text = text.replace("target score", str(wc.threshold))
            objectives.append(text)

        if len(objectives) == 1:
            return objectives[0]
        else:
            return "Win by either:\n- " + "\n- ".join(objectives)

    def _extract_phases(self, genome: "GameGenome") -> list[tuple[str, str]]:
        """Extract turn phases as (name, description) tuples."""
        from darwindeck.genome.schema import (
            DrawPhase, PlayPhase, DiscardPhase, BettingPhase,
            TrickPhase, ClaimPhase, Location
        )

        phases = []
        for i, phase in enumerate(genome.turn_structure.phases, 1):
            name, desc = self._describe_phase(phase)
            phases.append((f"Phase {i}: {name}", desc))
        return phases

    def _describe_phase(self, phase) -> tuple[str, str]:
        """Convert a phase to (name, description)."""
        from darwindeck.genome.schema import (
            DrawPhase, PlayPhase, DiscardPhase, BettingPhase,
            TrickPhase, ClaimPhase, Location
        )

        if isinstance(phase, DrawPhase):
            source = "deck" if phase.source == Location.DECK else "discard pile"
            if phase.count == 1:
                desc = f"Draw 1 card from the {source}"
            else:
                desc = f"Draw {phase.count} cards from the {source}"
            if not phase.mandatory:
                desc += " (optional)"
            return ("Draw", desc)

        elif isinstance(phase, PlayPhase):
            target = "discard pile" if phase.target == Location.DISCARD else "tableau"
            if phase.min_cards == phase.max_cards:
                if phase.min_cards == 1:
                    desc = f"Play exactly 1 card to the {target}"
                else:
                    desc = f"Play exactly {phase.min_cards} cards to the {target}"
            elif phase.min_cards == 0:
                desc = f"Play up to {phase.max_cards} cards to the {target} (optional)"
            else:
                desc = f"Play {phase.min_cards}-{phase.max_cards} cards to the {target}"
            return ("Play", desc)

        elif isinstance(phase, DiscardPhase):
            if phase.count == 1:
                desc = "Discard 1 card"
            else:
                desc = f"Discard {phase.count} cards"
            if not phase.mandatory:
                desc += " (optional)"
            return ("Discard", desc)

        elif isinstance(phase, BettingPhase):
            desc = f"Betting round (minimum bet: {phase.min_bet} chips, max {phase.max_raises} raises)"
            return ("Betting", desc)

        elif isinstance(phase, TrickPhase):
            desc = "Play one card to the trick. "
            if phase.lead_suit_required:
                desc += "Must follow suit if able. "
            if phase.high_card_wins:
                desc += "Highest card wins the trick."
            else:
                desc += "Lowest card wins the trick."
            return ("Trick", desc)

        elif isinstance(phase, ClaimPhase):
            desc = f"Play {phase.min_cards}-{phase.max_cards} cards face-down and claim a rank. "
            if phase.sequential_rank:
                desc += "Claims must follow sequence (A, 2, 3, ..., K). "
            if phase.allow_challenge:
                desc += "Opponents may challenge your claim."
            return ("Claim", desc)

        else:
            return ("Unknown", "Perform the phase action")

    def _extract_special_rules(self, genome: "GameGenome") -> list[str]:
        """Extract special card effects as rules."""
        from darwindeck.genome.schema import EffectType, Rank

        rules = []

        rank_names = {
            Rank.ACE: "Ace", Rank.TWO: "2", Rank.THREE: "3", Rank.FOUR: "4",
            Rank.FIVE: "5", Rank.SIX: "6", Rank.SEVEN: "7", Rank.EIGHT: "8",
            Rank.NINE: "9", Rank.TEN: "10", Rank.JACK: "Jack",
            Rank.QUEEN: "Queen", Rank.KING: "King"
        }

        for effect in genome.special_effects:
            rank_name = rank_names.get(effect.trigger_rank, str(effect.trigger_rank))

            if effect.effect_type == EffectType.SKIP_NEXT:
                rules.append(f"**{rank_name}:** Playing this card skips the next player's turn")
            elif effect.effect_type == EffectType.REVERSE_DIRECTION:
                rules.append(f"**{rank_name}:** Playing this card reverses the turn order")
            elif effect.effect_type == EffectType.DRAW_CARDS:
                rules.append(f"**{rank_name}:** Next player must draw {effect.value} cards")
            elif effect.effect_type == EffectType.EXTRA_TURN:
                rules.append(f"**{rank_name}:** Playing this card gives you an extra turn")
            elif effect.effect_type == EffectType.FORCE_DISCARD:
                rules.append(f"**{rank_name}:** Next player must discard {effect.value} cards")

        # Add wild card rules if any
        if genome.setup.wild_cards:
            wild_names = [rank_names.get(r, str(r)) for r in genome.setup.wild_cards]
            rules.append(f"**Wild cards ({', '.join(wild_names)}):** Can be played on any card")

        return rules


class RulebookGenerator:
    """Generates complete rulebooks from genomes."""

    def __init__(self):
        self.validator = GenomeValidator()
        self.extractor = GenomeExtractor()

    def generate(self, genome: "GameGenome", use_llm: bool = True) -> str:
        """Generate a complete rulebook for a genome.

        Args:
            genome: The game genome
            use_llm: Whether to use LLM enhancement (default True)

        Returns:
            Complete rulebook as markdown string

        Raises:
            ValueError: If genome fails validation
        """
        # Validate genome first
        validation = self.validator.validate(genome)
        if not validation.valid:
            raise ValueError(f"Invalid genome: {'; '.join(validation.errors)}")

        # Extract sections
        sections = self.extractor.extract(genome)

        # Get applicable edge case defaults
        defaults = select_applicable_defaults(genome)
        sections.edge_cases = [d.rule for d in defaults]

        # TODO: LLM enhancement (Task 8)
        # if use_llm:
        #     sections = RulebookEnhancer().enhance(sections, genome)

        # Render to markdown
        return self._render_markdown(sections)

    def _render_markdown(self, sections: RulebookSections) -> str:
        """Render sections to markdown format."""
        lines = []

        # Title
        lines.append(f"# {sections.game_name}")
        lines.append("")

        # Overview (if present)
        if sections.overview:
            lines.append("## Overview")
            lines.append(sections.overview)
            lines.append("")

        # Components
        lines.append("## Components")
        for component in sections.components:
            lines.append(f"- {component}")
        lines.append("")

        # Setup
        lines.append("## Setup")
        for i, step in enumerate(sections.setup_steps, 1):
            lines.append(f"{i}. {step}")
        lines.append("")

        # Objective
        lines.append("## Objective")
        lines.append(sections.objective)
        lines.append("")

        # Turn Structure
        lines.append("## Turn Structure")
        lines.append(f"Each turn consists of {len(sections.phases)} phase(s):")
        lines.append("")
        for name, desc in sections.phases:
            lines.append(f"### {name}")
            lines.append(desc)
            lines.append("")

        # Special Rules (if any)
        if sections.special_rules:
            lines.append("## Special Rules")
            for rule in sections.special_rules:
                lines.append(rule)
                lines.append("")

        # Edge Cases
        lines.append("## Edge Cases")
        for edge_case in sections.edge_cases:
            lines.append(edge_case)
            lines.append("")

        # Quick Reference (if present)
        if sections.quick_reference:
            lines.append("## Quick Reference")
            lines.append(sections.quick_reference)
            lines.append("")

        return "\n".join(lines)
