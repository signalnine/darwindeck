"""Cognitive complexity estimation for game rules.

This module calculates how hard a game is to explain/learn, not just
how big the genome is. Key insights:

1. Genome size ≠ explanation complexity
2. Conditional nesting/conjunction matters more than count
3. Memory/state tracking is invisible but costly
4. Familiar mechanics (trick-taking, draw-play) come "free"
5. Custom printed decks (like Uno) reduce special effects complexity
   since the rules are printed directly on the cards
"""

from dataclasses import dataclass
from typing import Optional

from darwindeck.genome.schema import (
    GameGenome, TurnStructure, PlayPhase, DrawPhase, BettingPhase,
    TrickPhase, ClaimPhase, SpecialEffect, WinCondition, Location
)
from darwindeck.genome.conditions import (
    Condition, CompoundCondition, ConditionOrCompound, ConditionType
)


@dataclass
class ComplexityBreakdown:
    """Detailed breakdown of cognitive complexity sources."""

    # Core mechanics
    phase_explanation_cost: float  # Cost of explaining each phase type
    condition_complexity: float    # Nesting depth, conjunctions
    special_effects_cost: float    # Unique rules to memorize

    # State tracking (invisible but costly)
    memory_requirements: float     # Cards to track, hidden info
    state_tracking_cost: float     # Trump suit, who passed, etc.

    # Familiarity discounts
    familiar_pattern_discount: float  # Trick-taking, draw-play, etc.
    custom_deck_discount: float       # Reduction for custom printed decks (Uno-style)

    # Final score
    total_complexity: float        # 0.0 = trivial, 1.0 = very complex
    explanation_sentences: int     # Estimated sentences to explain

    def inverted_score(self) -> float:
        """Return 1.0 - complexity for fitness (simpler = better)."""
        return max(0.0, 1.0 - self.total_complexity)


def calculate_complexity(genome: GameGenome) -> ComplexityBreakdown:
    """Calculate cognitive complexity of a game's rules.

    Returns a breakdown showing where complexity comes from.
    """

    # 1. Phase explanation cost
    phase_cost = _calculate_phase_cost(genome.turn_structure)

    # 2. Condition complexity (nesting, conjunctions)
    condition_cost = _calculate_condition_complexity(genome)

    # 3. Special effects cost (unique rules)
    effects_cost = _calculate_effects_cost(genome.special_effects)

    # 4. Memory requirements
    memory_cost = _calculate_memory_cost(genome)

    # 5. State tracking cost
    state_cost = _calculate_state_tracking_cost(genome)

    # 6. Implicit complexity from game type
    # Some complexity isn't in the genome but is inherent to the game type
    implicit_cost = _calculate_implicit_complexity(genome)

    # 7. Familiar pattern discounts
    discount = _calculate_familiarity_discount(genome)

    # 8. Custom deck discount (Uno-style printed cards)
    # Custom printed decks reduce special effects complexity by 80%
    # since players don't need to memorize which cards do what
    custom_deck_discount = 0.0
    if genome.setup.custom_printed_deck and genome.special_effects:
        # 80% reduction to effects cost
        custom_deck_discount = effects_cost * 0.80
        effects_cost = effects_cost * 0.20

    # Normalize compressed components to use full 0-1 range
    # Based on observed maximums across seed genomes
    # Without this, condition/effects/state only contribute ~35% of their weight
    condition_cost_norm = min(1.0, condition_cost / 0.40)  # max ~0.40 → stretch
    effects_cost_norm = min(1.0, effects_cost / 0.15)       # max ~0.15 → stretch
    state_cost_norm = min(1.0, state_cost / 0.40)           # max ~0.40 → stretch

    # Combine with weights (all components now on 0-1 scale)
    raw_complexity = (
        phase_cost * 0.22 +
        condition_cost_norm * 0.20 +
        effects_cost_norm * 0.15 +
        memory_cost * 0.18 +
        state_cost_norm * 0.10 +
        implicit_cost * 0.15
    )

    # Apply familiarity discount as MULTIPLICATIVE reduction (not subtractive)
    # This prevents familiar games from going to zero complexity
    # Cap discount effect at 40% reduction to preserve meaningful differences
    discount_factor = min(0.40, discount * 0.50)
    total = raw_complexity * (1.0 - discount_factor)

    # Apply power transform to spread out scores more evenly
    # Without this, scores cluster in the 0.05-0.45 range
    # Power of 0.6 stretches that to approximately 0.15-0.65
    # This gives better differentiation between games
    total = pow(total, 0.6)

    # Normalize to 0-1 range (cap at 1.0)
    total = min(1.0, total)

    # Estimate explanation sentences
    sentences = _estimate_explanation_sentences(genome)

    return ComplexityBreakdown(
        phase_explanation_cost=phase_cost,
        condition_complexity=condition_cost,
        special_effects_cost=effects_cost,
        memory_requirements=memory_cost,
        state_tracking_cost=state_cost,
        familiar_pattern_discount=discount,
        custom_deck_discount=custom_deck_discount,
        total_complexity=total,
        explanation_sentences=sentences
    )


def _calculate_implicit_complexity(genome: GameGenome) -> float:
    """Calculate complexity that's inherent to game type but not in genome.

    Some complexity comes from game mechanics that aren't fully specified
    in the genome structure, like:
    - Poker hand rankings (10 different ranks to learn)
    - Meld validation (sets vs runs, valid sequences)
    - Scoring systems (deadwood counting, point cards)
    """
    cost = 0.0

    # Win conditions imply knowledge requirements
    for wc in genome.win_conditions:
        if wc.type == "best_hand":
            # Must know: high card < pair < two pair < three of kind < straight
            # < flush < full house < four of kind < straight flush < royal flush
            cost += 0.50  # This is a LOT to learn
        elif wc.type == "low_score":
            # Point counting systems vary but require understanding values
            cost += 0.20
        elif wc.type == "most_captured":
            # Capture rules need explanation
            cost += 0.15

    # Games with tableau and flexible play suggest meld/set formation
    # which requires understanding valid combinations
    from darwindeck.genome.schema import DiscardPhase
    has_flexible_play = False
    for phase in genome.turn_structure.phases:
        if isinstance(phase, PlayPhase):
            if phase.target == Location.TABLEAU and phase.max_cards > 1:
                has_flexible_play = True
                break

    if has_flexible_play:
        # Suggests meld/run formation - need to explain valid combos
        cost += 0.25

    # Games that use scoring_rules have additional complexity
    if genome.scoring_rules:
        cost += len(genome.scoring_rules) * 0.10

    return min(1.0, cost)


def _calculate_phase_cost(turn: TurnStructure) -> float:
    """Calculate cost of explaining each phase type.

    Some phases are simple (draw a card), others are paragraphs
    (trick resolution, betting rounds, claim/challenge).

    Key insight: Phase type determines INHERENT complexity regardless
    of genome size. A ClaimPhase always requires explaining:
    - What you can claim
    - That you can lie
    - That others can challenge
    - Resolution of challenges (who takes pile)
    """
    cost = 0.0
    distinct_phase_types = set()

    # Cost per phase type (in "explanation units")
    # These reflect the MINIMUM explanation needed for each mechanic
    PHASE_COSTS = {
        DrawPhase: 0.08,      # "Draw a card" - but source matters
        PlayPhase: 0.15,      # May have conditions
        TrickPhase: 0.45,     # Lead, follow suit, trump, highest wins, scoring
        BettingPhase: 0.50,   # Check, bet, call, raise, fold, all-in, pot resolution
        ClaimPhase: 0.55,     # Claim, lie option, challenge option, truth check, pile penalty
    }

    # Additional complexity for phase parameters
    from darwindeck.genome.schema import DiscardPhase

    for phase in turn.phases:
        phase_type = type(phase)
        distinct_phase_types.add(phase_type)
        base_cost = PHASE_COSTS.get(phase_type, 0.10)

        # DrawPhase: source matters a lot
        if isinstance(phase, DrawPhase):
            if phase.source == Location.OPPONENT_HAND:
                base_cost += 0.15  # "Draw from opponent's hand" is a distinct mechanic
            if not phase.mandatory:
                base_cost += 0.05  # Optional draw = decision to explain

        # DiscardPhase: matching conditions add complexity
        if isinstance(phase, DiscardPhase):
            if hasattr(phase, 'matching_condition') and phase.matching_condition:
                base_cost += 0.20  # "Discard pairs" or other matching rules
            if phase.count > 1:
                base_cost += 0.10  # Multi-card discard = more to explain

        # Add cost for phase conditions
        if hasattr(phase, 'condition') and phase.condition:
            base_cost += _condition_depth(phase.condition) * 0.12

        # Add cost for valid_play_condition
        if hasattr(phase, 'valid_play_condition') and phase.valid_play_condition:
            base_cost += _condition_depth(phase.valid_play_condition) * 0.15

        cost += base_cost

    # Count of duplicate phases (same type appearing multiple times)
    # Duplicates add less complexity than distinct types
    num_phases = len(turn.phases)
    num_distinct = len(distinct_phase_types)
    num_duplicates = num_phases - num_distinct

    # Discount duplicate phases - they're modeling artifacts, not real complexity
    # If you have 2 PlayPhases, you don't explain PlayPhase twice
    if num_duplicates > 0:
        # Reduce cost for duplicates (they only add ~20% of their base cost)
        duplicate_discount = num_duplicates * 0.10
        cost = max(0.1, cost - duplicate_discount)

    # Bonus for having many DISTINCT phase types (more mechanics to learn)
    distinct_bonus = num_distinct * 0.06
    cost += distinct_bonus

    # Normalize: cap at 1.0 but don't over-compress
    return min(1.0, cost)


def _calculate_condition_complexity(genome: GameGenome) -> float:
    """Calculate complexity from conditions.

    Key insight: "if card is 8: wild" = 1 sentence
    "if hand_size > 3 AND top_discard is red AND prev_passed" = paragraph

    Three factors:
    1. Presence of conditions (any conditions = some complexity)
    2. Number of clauses (more alternatives = more to explain)
    3. Nesting depth (deeply nested = harder to parse)
    """
    total_depth = 0
    total_conjunctions = 0
    total_clauses = 0  # Count individual conditions/alternatives
    condition_count = 0

    # Collect all conditions from phases
    for phase in genome.turn_structure.phases:
        if hasattr(phase, 'condition') and phase.condition:
            depth, conj, clauses = _analyze_condition_full(phase.condition)
            total_depth += depth
            total_conjunctions += conj
            total_clauses += clauses
            condition_count += 1

        if hasattr(phase, 'valid_play_condition') and phase.valid_play_condition:
            depth, conj, clauses = _analyze_condition_full(phase.valid_play_condition)
            total_depth += depth
            total_conjunctions += conj
            total_clauses += clauses
            condition_count += 1

    # Collect conditions from special effects
    # (Each effect has an implicit simple trigger)
    implicit_conditions = len(genome.special_effects)
    total_clauses += implicit_conditions

    # Score based on three factors
    if condition_count == 0 and implicit_conditions == 0:
        return 0.0

    # 1. Presence score: having conditions at all adds complexity
    #    1 condition = 0.2, 2+ conditions = 0.3-0.4
    presence_score = min(0.4, 0.15 + condition_count * 0.08)

    # 2. Clause score: more alternatives/clauses = more to explain
    #    "match suit OR rank OR 8" = 3 clauses
    clause_score = min(1.0, total_clauses / 8.0)

    # 3. Depth score: deeply nested = harder to parse
    if condition_count > 0:
        avg_depth = total_depth / condition_count
    else:
        avg_depth = 1.0
    depth_score = max(0.0, min(1.0, (avg_depth - 1) / 2.0))

    # 4. Conjunction penalty: AND logic is harder than OR
    conj_score = min(1.0, total_conjunctions / 4.0)

    # Combine: presence + clauses are most important
    return (
        presence_score * 0.35 +
        clause_score * 0.35 +
        depth_score * 0.15 +
        conj_score * 0.15
    )


def _analyze_condition_full(cond: ConditionOrCompound) -> tuple[int, int, int]:
    """Analyze condition for depth, conjunction count, and clause count.

    Returns (max_depth, conjunction_count, total_clauses).
    """
    if isinstance(cond, Condition):
        return (1, 0, 1)

    if isinstance(cond, CompoundCondition):
        max_child_depth = 0
        total_conj = 0
        total_clauses = 0

        for child in cond.conditions:
            child_depth, child_conj, child_clauses = _analyze_condition_full(child)
            max_child_depth = max(max_child_depth, child_depth)
            total_conj += child_conj
            total_clauses += child_clauses

        # Depth calculation
        if max_child_depth > 1:
            depth = max_child_depth + 1
            total_conj += 1 if cond.logic == "AND" else 0
        else:
            depth = 1
            if cond.logic == "AND" and len(cond.conditions) > 2:
                total_conj += 1

        return (depth, total_conj, total_clauses)

    return (1, 0, 1)


def _analyze_condition(cond: ConditionOrCompound) -> tuple[int, int]:
    """Analyze condition for depth and conjunction count (legacy).

    Returns (max_depth, conjunction_count).

    Key insight: "A OR B OR C" is a simple list (depth 1, easy to explain)
    but "A AND (B OR C)" requires understanding precedence (depth 2, harder)
    """
    if isinstance(cond, Condition):
        return (1, 0)

    if isinstance(cond, CompoundCondition):
        max_child_depth = 0
        total_conj = 0

        for child in cond.conditions:
            child_depth, child_conj = _analyze_condition(child)
            max_child_depth = max(max_child_depth, child_depth)
            total_conj += child_conj

        # Only count depth for nested compounds (OR of simple conditions = depth 1)
        # "A OR B OR C" with all simple children = depth 1
        # "A AND (B OR C)" = depth 2 because of nesting
        if max_child_depth > 1:
            # Has nested compound - genuinely complex
            depth = max_child_depth + 1
            total_conj += 1 if cond.logic == "AND" else 0
        else:
            # Flat list of simple conditions - not complex
            depth = 1
            # AND of multiple conditions is harder than OR
            if cond.logic == "AND" and len(cond.conditions) > 2:
                total_conj += 1

        return (depth, total_conj)

    return (1, 0)


def _condition_depth(cond: Optional[ConditionOrCompound]) -> int:
    """Get nesting depth of a condition."""
    if cond is None:
        return 0
    depth, _ = _analyze_condition(cond)
    return depth


def _calculate_effects_cost(effects: list[SpecialEffect]) -> float:
    """Calculate cost of explaining special effects.

    Key: Each unique effect type is a rule to explain.
    Multiple effects of same type are one rule with exceptions.
    """
    if not effects:
        return 0.0

    # Group by effect type
    effect_types = set()
    for effect in effects:
        effect_types.add(effect.effect_type)

    unique_types = len(effect_types)
    total_effects = len(effects)

    # Base cost per unique effect type
    type_cost = unique_types * 0.15

    # Additional cost for many triggers (more exceptions to remember)
    if total_effects > unique_types:
        exception_cost = (total_effects - unique_types) * 0.05
    else:
        exception_cost = 0.0

    return min(1.0, type_cost + exception_cost)


def _calculate_memory_cost(genome: GameGenome) -> float:
    """Calculate cognitive load from memory requirements.

    Games requiring card counting, tracking played cards, or
    remembering hidden information are harder.
    """
    cost = 0.0

    # Check win conditions for memory-heavy types
    for wc in genome.win_conditions:
        if wc.type == "most_captured":
            cost += 0.20  # Must track captured cards
        elif wc.type == "low_score":
            cost += 0.15  # Must track points throughout game
        elif wc.type == "best_hand":
            cost += 0.35  # Must understand poker hand rankings (10 ranks!)
        elif wc.type == "most_tricks":
            cost += 0.20  # Track trick counts

    # Trick-taking requires remembering what's been played
    for phase in genome.turn_structure.phases:
        if isinstance(phase, TrickPhase):
            cost += 0.30  # Card counting is valuable for strategy
            break

    # Bluffing games require tracking claims AND opponent behavior
    for phase in genome.turn_structure.phases:
        if isinstance(phase, ClaimPhase):
            cost += 0.25  # Must remember claims AND read opponents
            break

    # Betting games require tracking pot, bets, who's in
    for phase in genome.turn_structure.phases:
        if isinstance(phase, BettingPhase):
            cost += 0.15  # Pot math, position, opponent stack sizes
            break

    # Games with elimination mechanics (Old Maid style)
    # require tracking what's been discarded
    from darwindeck.genome.schema import DiscardPhase
    for phase in genome.turn_structure.phases:
        if isinstance(phase, DiscardPhase):
            if phase.count > 1:
                cost += 0.15  # Pair/set matching = track what's out
            break

    # Hidden information baseline (opponent hands, deck)
    cost += 0.08

    return min(1.0, cost)


def _calculate_state_tracking_cost(genome: GameGenome) -> float:
    """Calculate cost of tracking game state.

    Trump suit, direction of play, who passed, betting state, etc.
    """
    cost = 0.0

    # Check for state-heavy mechanics
    for phase in genome.turn_structure.phases:
        if isinstance(phase, TrickPhase):
            cost += 0.15  # Trump suit, lead suit
        if isinstance(phase, BettingPhase):
            cost += 0.20  # Pot, current bet, who's in

    # Special effects that change game state
    for effect in genome.special_effects:
        if effect.effect_type.value == "reverse":
            cost += 0.10  # Must track direction
        elif effect.effect_type.value == "skip_next":
            cost += 0.05  # Simpler state

    # Multi-player increases state tracking
    if genome.player_count > 2:
        cost += 0.10 * (genome.player_count - 2)

    return min(1.0, cost)


def _calculate_familiarity_discount(genome: GameGenome) -> float:
    """Calculate discount for familiar game patterns.

    Trick-taking, draw-and-play, etc. come "free" cognitively
    because most players already know them.

    NOTE: Discounts are SMALLER than before to preserve differentiation.
    Familiarity helps but doesn't eliminate learning curve.
    """
    discount = 0.0

    # Trick-taking is familiar (Hearts, Spades, Bridge)
    # But you still need to learn trump rules, scoring, etc.
    for phase in genome.turn_structure.phases:
        if isinstance(phase, TrickPhase):
            discount += 0.15  # Reduced from 0.30
            break

    # Simple draw-play pattern (Crazy Eights, Uno)
    # Pattern is familiar but matching rules still need explanation
    has_draw = any(isinstance(p, DrawPhase) for p in genome.turn_structure.phases)
    has_play = any(isinstance(p, PlayPhase) for p in genome.turn_structure.phases)
    if has_draw and has_play and len(genome.turn_structure.phases) <= 3:
        discount += 0.10  # Reduced from 0.20

    # Betting is familiar (Poker)
    # But check/bet/call/raise/fold still needs explanation
    for phase in genome.turn_structure.phases:
        if isinstance(phase, BettingPhase):
            discount += 0.08  # Reduced from 0.15
            break

    # War is trivial - truly minimal explanation needed
    if (len(genome.turn_structure.phases) == 1 and
        isinstance(genome.turn_structure.phases[0], PlayPhase)):
        discount += 0.25  # Reduced from 0.40

    return min(1.0, discount)


def _estimate_explanation_sentences(genome: GameGenome) -> int:
    """Estimate sentences needed to explain the game."""
    sentences = 0

    # Setup: 1-2 sentences
    sentences += 2

    # Each phase type
    for phase in genome.turn_structure.phases:
        if isinstance(phase, DrawPhase):
            sentences += 1
        elif isinstance(phase, PlayPhase):
            sentences += 2
            if hasattr(phase, 'valid_play_condition') and phase.valid_play_condition:
                sentences += _condition_depth(phase.valid_play_condition)
        elif isinstance(phase, TrickPhase):
            sentences += 5  # Lead, follow, trump, resolution, scoring
        elif isinstance(phase, BettingPhase):
            sentences += 4  # Check, bet, raise, fold
        elif isinstance(phase, ClaimPhase):
            sentences += 3  # Claim, challenge, resolution

    # Special effects
    # For custom printed decks, only need 1 sentence ("follow card instructions")
    # For standard decks, need 2 sentences per effect type explaining trigger + effect
    if genome.special_effects:
        unique_effects = len(set(e.effect_type for e in genome.special_effects))
        if genome.setup.custom_printed_deck:
            sentences += 1  # "Follow the instructions printed on special cards"
        else:
            sentences += unique_effects * 2

    # Win conditions
    sentences += len(genome.win_conditions)

    return sentences


def get_rules_complexity_score(genome: GameGenome) -> float:
    """Get the inverted complexity score for fitness calculation.

    Returns 0.0-1.0 where 1.0 = simplest, 0.0 = most complex.
    """
    breakdown = calculate_complexity(genome)
    return breakdown.inverted_score()
