"""Full rulebook generation from game genomes."""

from __future__ import annotations

import logging
import os
from dataclasses import dataclass, field
from typing import Optional, TYPE_CHECKING

try:
    import anthropic
except ImportError:
    anthropic = None  # type: ignore

if TYPE_CHECKING:
    from darwindeck.genome.schema import GameGenome

logger = logging.getLogger(__name__)


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
    scoring_rules: list[str] = field(default_factory=list)
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
        "high_score": "Player with the highest score wins (when any player reaches {threshold} points)",
        "low_score": "Player with the lowest score wins (when any player reaches {threshold} points)",
        "capture_all": "Capture all cards to win",
        "most_tricks": "Player who wins the most tricks wins",
        "fewest_tricks": "Player who wins the fewest tricks wins",
        "most_chips": "Player with the most chips wins",
        "most_captured": "Player who captures the most cards wins",
        "first_to_score": "First player to reach {threshold} points wins",
        "all_hands_empty": "When all hands are empty, lowest score wins",
        "best_hand": "Best poker hand wins at showdown",
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
            scoring_rules=self._extract_scoring_rules(genome),
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
                text = text.replace("{threshold}", str(wc.threshold))
            else:
                text = text.replace(" (when any player reaches {threshold} points)", "")
                text = text.replace("{threshold} points", "the target")
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
            # Note: Scoring happens in _extract_scoring_rules
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

    def _extract_scoring_rules(self, genome: "GameGenome") -> list[str]:
        """Extract scoring rules, including implicit trick-taking scoring."""
        from darwindeck.genome.schema import TrickPhase

        rules = []

        # Check if this is a trick-taking game with score-based win condition
        has_trick_phase = any(
            isinstance(p, TrickPhase) for p in genome.turn_structure.phases
        )
        has_score_win = any(
            wc.type in ("low_score", "high_score", "all_hands_empty")
            for wc in genome.win_conditions
        )

        if has_trick_phase and has_score_win:
            # Implicit Hearts-style scoring in the Go simulator
            rules.append("**Trick Scoring:** When you win a trick, score 1 point for each Heart and 13 points for the Queen of Spades")

        # Explicit scoring rules from genome (if any)
        for scoring_rule in genome.scoring_rules:
            rules.append(f"**Scoring:** {scoring_rule}")

        return rules

    def _extract_special_rules(self, genome: "GameGenome") -> list[str]:
        """Extract special card effects as rules."""
        from darwindeck.genome.schema import EffectType, Rank
        from collections import defaultdict

        rules = []

        rank_names = {
            Rank.ACE: "Ace", Rank.TWO: "2", Rank.THREE: "3", Rank.FOUR: "4",
            Rank.FIVE: "5", Rank.SIX: "6", Rank.SEVEN: "7", Rank.EIGHT: "8",
            Rank.NINE: "9", Rank.TEN: "10", Rank.JACK: "Jack",
            Rank.QUEEN: "Queen", Rank.KING: "King"
        }

        effect_descriptions = {
            EffectType.SKIP_NEXT: "skips the next player's turn",
            EffectType.REVERSE_DIRECTION: "reverses the turn order",
            EffectType.DRAW_CARDS: "makes next player draw {value} cards",
            EffectType.EXTRA_TURN: "gives you an extra turn",
            EffectType.FORCE_DISCARD: "makes next player discard {value} cards",
        }

        # Group effects by trigger rank to consolidate duplicates
        effects_by_rank: dict[Rank, list[str]] = defaultdict(list)

        for effect in genome.special_effects:
            desc = effect_descriptions.get(effect.effect_type)
            if desc:
                desc = desc.replace("{value}", str(effect.value))
                effects_by_rank[effect.trigger_rank].append(desc)

        # Generate consolidated rules
        for rank, effect_list in effects_by_rank.items():
            rank_name = rank_names.get(rank, str(rank))
            if len(effect_list) == 1:
                rules.append(f"**{rank_name}:** Playing this card {effect_list[0]}")
            else:
                # Multiple effects on same rank - both trigger!
                combined = " AND ".join(effect_list)
                rules.append(f"**{rank_name}:** Playing this card {combined}")

        # Add wild card rules if any
        if genome.setup.wild_cards:
            wild_names = [rank_names.get(r, str(r)) for r in genome.setup.wild_cards]
            rules.append(f"**Wild cards ({', '.join(wild_names)}):** Can be played on any card")

        # Add tableau mode description if non-trivial
        tableau_desc = self._get_tableau_mode_description(genome)
        if tableau_desc:
            rules.append(f"**Tableau:** {tableau_desc}")

        return rules

    def _get_tableau_mode_description(self, genome: "GameGenome") -> str:
        """Get description for tableau mode.

        Returns empty string for NONE mode, otherwise returns a human-readable
        description of how cards on the tableau interact.
        """
        from darwindeck.genome.schema import TableauMode, SequenceDirection

        mode = genome.setup.tableau_mode

        if mode == TableauMode.NONE:
            return ""
        elif mode == TableauMode.WAR:
            return "When both players have played, compare ranks—the higher card wins both cards."
        elif mode == TableauMode.MATCH_RANK:
            return "If your card matches a card on the tableau by rank, capture both cards."
        elif mode == TableauMode.SEQUENCE:
            direction = genome.setup.sequence_direction
            if direction == SequenceDirection.ASCENDING:
                return "Play cards in ascending order to build on tableau piles."
            elif direction == SequenceDirection.DESCENDING:
                return "Play cards in descending order to build on tableau piles."
            else:  # BOTH
                return "Play cards in either ascending or descending order to build on tableau piles."
        return ""


class RulebookEnhancer:
    """Optional LLM enhancement for rulebook sections."""

    def enhance(self, sections: RulebookSections, genome: Optional["GameGenome"]) -> RulebookSections:
        """Enhance sections with LLM-generated content.

        Args:
            sections: Extracted rulebook sections
            genome: Original genome (for validation)

        Returns:
            Enhanced sections (or original if LLM unavailable)
        """
        api_key = os.environ.get("ANTHROPIC_API_KEY")
        if not api_key:
            logger.warning("ANTHROPIC_API_KEY not set, skipping LLM enhancement")
            return sections

        if anthropic is None:
            logger.warning("anthropic package not installed, skipping LLM enhancement")
            return sections

        try:
            client = anthropic.Anthropic(api_key=api_key)

            # Generate overview
            overview = self._generate_overview(client, sections, genome)
            if overview:
                sections = RulebookSections(
                    game_name=sections.game_name,
                    player_count=sections.player_count,
                    objective=sections.objective,
                    overview=overview,
                    components=sections.components,
                    setup_steps=sections.setup_steps,
                    phases=sections.phases,
                    scoring_rules=sections.scoring_rules,
                    special_rules=sections.special_rules,
                    edge_cases=sections.edge_cases,
                    quick_reference=sections.quick_reference,
                )

            # TODO: Add example turn generation
            # TODO: Add quick reference generation

            return sections

        except Exception as e:
            logger.warning(f"LLM enhancement failed: {e}")
            return sections

    def _generate_overview(
        self, client, sections: RulebookSections, genome: Optional["GameGenome"]
    ) -> Optional[str]:
        """Generate engaging overview."""
        phase_names = [name for name, _ in sections.phases]

        prompt = f"""Write a 1-2 sentence overview for this card game:

Game: {sections.game_name}
Players: {sections.player_count}
Phases: {', '.join(phase_names)}
Objective: {sections.objective}

Make it engaging and accessible. Do not invent mechanics not listed above.
Return ONLY the overview text, no quotes or formatting."""

        try:
            response = client.messages.create(
                model="claude-sonnet-4-20250514",
                max_tokens=100,
                messages=[{"role": "user", "content": prompt}]
            )
            return response.content[0].text.strip()
        except Exception as e:
            logger.warning(f"Overview generation failed: {e}")
            return None


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

        # LLM enhancement (optional)
        if use_llm:
            enhancer = RulebookEnhancer()
            sections = enhancer.enhance(sections, genome)

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

        # Scoring (if any)
        if sections.scoring_rules:
            lines.append("## Scoring")
            for rule in sections.scoring_rules:
                lines.append(rule)
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
