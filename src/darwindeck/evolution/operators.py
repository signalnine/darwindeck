"""Genetic operators for game genome evolution (Phase 4)."""

from __future__ import annotations

import random
from abc import ABC, abstractmethod
from typing import List, Optional
from dataclasses import replace
from darwindeck.genome.schema import (
    GameGenome, WinCondition, SetupRules, TurnStructure,
    PlayPhase, DrawPhase, DiscardPhase, TrickPhase, ClaimPhase,
    BettingPhase, BiddingPhase, ContractScoring,
    Location, Suit, SpecialEffect, EffectType, TargetSelector, Rank,
    TableauMode, SequenceDirection, ShowdownMethod, Visibility,
    CardScoringRule, CardCondition, ScoringTrigger,
    HandEvaluation, HandPattern, CardValue,
)
from darwindeck.genome.conditions import Condition, ConditionType, Operator
from darwindeck.evolution.naming import generate_name


class MutationOperator(ABC):
    """Base class for mutation operators."""

    def __init__(self, probability: float = 0.1):
        """Initialize mutation operator.

        Args:
            probability: Mutation probability (0.0-1.0)
        """
        self.probability = probability

    @abstractmethod
    def mutate(self, genome: GameGenome) -> GameGenome:
        """Apply mutation to genome.

        Args:
            genome: Genome to mutate

        Returns:
            New mutated genome
        """
        pass

    def should_apply(self) -> bool:
        """Check if mutation should be applied based on probability."""
        return random.random() < self.probability


class TweakParameterMutation(MutationOperator):
    """Mutate numeric parameters (hand size, max turns, etc.)."""

    def __init__(self, probability: float = 0.15, preserve_player_count: bool = False):
        """Initialize parameter tweaking mutation.

        Args:
            probability: Mutation probability (default: 15%)
            preserve_player_count: If True, don't mutate player_count (for filtered evolution)
        """
        super().__init__(probability)
        self.preserve_player_count = preserve_player_count

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Tweak a random numeric parameter.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with tweaked parameter
        """
        choices = [
            'cards_per_player',
            'max_turns',
            'initial_discard_count',
        ]
        # Only allow player_count mutation if not preserving it
        if not self.preserve_player_count:
            choices.append('player_count')

        choice = random.choice(choices)

        if choice == 'cards_per_player':
            # Adjust ±3 cards, keep in range [3, 26]
            delta = random.randint(-3, 3)
            new_value = max(3, min(26, genome.setup.cards_per_player + delta))
            new_setup = replace(genome.setup, cards_per_player=new_value)
            return replace(genome, setup=new_setup, generation=genome.generation + 1)

        elif choice == 'max_turns':
            # Adjust ±20%, keep in range [20, 1000]
            delta_pct = random.uniform(-0.2, 0.2)
            new_value = int(max(20, min(1000, genome.max_turns * (1 + delta_pct))))
            return replace(genome, max_turns=new_value, generation=genome.generation + 1)

        elif choice == 'initial_discard_count':
            # Toggle between 0 and 1 (most common)
            new_value = 1 - genome.setup.initial_discard_count
            new_setup = replace(genome.setup, initial_discard_count=new_value)
            return replace(genome, setup=new_setup, generation=genome.generation + 1)

        elif choice == 'player_count':
            # Change player count: 2, 3, or 4 players
            current = genome.player_count
            # Pick a different player count
            options = [p for p in [2, 3, 4] if p != current]
            new_player_count = random.choice(options)

            # Adjust cards_per_player if needed to not exceed 52 total cards
            max_cards_per_player = 52 // new_player_count
            new_cards_per_player = min(genome.setup.cards_per_player, max_cards_per_player)

            new_setup = replace(genome.setup, cards_per_player=new_cards_per_player)
            return replace(genome, setup=new_setup, player_count=new_player_count,
                          generation=genome.generation + 1)

        return genome


class SwapPhaseOrderMutation(MutationOperator):
    """Swap the order of two adjacent phases."""

    def __init__(self, probability: float = 0.1):
        """Initialize phase swapping mutation.

        Args:
            probability: Mutation probability (default: 10%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Swap two adjacent phases.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with swapped phases
        """
        phases = list(genome.turn_structure.phases)

        if len(phases) < 2:
            return genome

        # Pick random adjacent pair
        idx = random.randint(0, len(phases) - 2)
        phases[idx], phases[idx + 1] = phases[idx + 1], phases[idx]

        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(genome, turn_structure=new_turn, generation=genome.generation + 1)


class AddPhaseMutation(MutationOperator):
    """Add a new phase to turn structure."""

    def __init__(self, probability: float = 0.05):
        """Initialize phase addition mutation.

        Args:
            probability: Mutation probability (default: 5%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Add a new phase to turn structure.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with additional phase
        """
        phases = list(genome.turn_structure.phases)

        # Don't add too many phases (max 5)
        if len(phases) >= 5:
            return genome

        # Create new phase (random type)
        # Weight towards simpler phases, but allow complex ones
        phase_type = random.choices(
            ["draw", "play", "discard", "trick", "claim"],
            weights=[30, 30, 20, 10, 10],  # Trick/claim less common
            k=1
        )[0]

        if phase_type == "draw":
            new_phase = DrawPhase(
                source=random.choice([Location.DECK, Location.DISCARD]),
                count=1,
                mandatory=random.choice([True, False])
            )
        elif phase_type == "play":
            new_phase = PlayPhase(
                target=Location.DISCARD,
                valid_play_condition=Condition(
                    type=ConditionType.HAND_SIZE,
                    operator=Operator.GT,
                    value=0
                ),
                min_cards=1,
                max_cards=1,
                mandatory=True
            )
        elif phase_type == "discard":
            new_phase = DiscardPhase(
                target=Location.DISCARD,
                count=1,
                mandatory=False
            )
        elif phase_type == "trick":
            new_phase = TrickPhase(
                lead_suit_required=random.choice([True, False]),
                trump_suit=random.choice([None, Suit.SPADES, Suit.HEARTS]),
                high_card_wins=random.choice([True, False]),
                breaking_suit=random.choice([None, Suit.HEARTS])
            )
        else:  # claim (bluffing)
            new_phase = ClaimPhase(
                min_cards=1,
                max_cards=random.choice([1, 2, 3, 4]),
                sequential_rank=random.choice([True, False]),
                allow_challenge=True,
                pile_penalty=True
            )

        # Insert at random position
        insert_pos = random.randint(0, len(phases))
        phases.insert(insert_pos, new_phase)

        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(genome, turn_structure=new_turn, generation=genome.generation + 1)


class RemovePhaseMutation(MutationOperator):
    """Remove a random phase from turn structure."""

    def __init__(self, probability: float = 0.05):
        """Initialize phase removal mutation.

        Args:
            probability: Mutation probability (default: 5%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Remove a random phase.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with removed phase
        """
        phases = list(genome.turn_structure.phases)

        # Don't remove if only 1 phase left
        if len(phases) <= 1:
            return genome

        # Remove random phase
        idx = random.randint(0, len(phases) - 1)
        phases.pop(idx)

        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(genome, turn_structure=new_turn, generation=genome.generation + 1)


class ModifyConditionMutation(MutationOperator):
    """Modify condition parameters in phases."""

    def __init__(self, probability: float = 0.1):
        """Initialize condition modification mutation.

        Args:
            probability: Mutation probability (default: 10%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Modify a condition in a random phase.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with modified condition
        """
        phases = list(genome.turn_structure.phases)

        if not phases:
            return genome

        # Find phases with conditions
        phases_with_conditions = []
        for i, phase in enumerate(phases):
            if isinstance(phase, PlayPhase) and phase.valid_play_condition:
                phases_with_conditions.append(i)
            elif isinstance(phase, DrawPhase) and phase.condition:
                phases_with_conditions.append(i)

        if not phases_with_conditions:
            return genome

        # Pick random phase with condition
        phase_idx = random.choice(phases_with_conditions)
        phase = phases[phase_idx]

        # Modify the condition
        if isinstance(phase, PlayPhase):
            old_cond = phase.valid_play_condition
            new_cond = self._tweak_condition(old_cond)
            new_phase = replace(phase, valid_play_condition=new_cond)
        elif isinstance(phase, DrawPhase):
            old_cond = phase.condition
            new_cond = self._tweak_condition(old_cond)
            new_phase = replace(phase, condition=new_cond)
        else:
            return genome

        phases[phase_idx] = new_phase
        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(genome, turn_structure=new_turn, generation=genome.generation + 1)

    def _tweak_condition(self, condition) -> Optional[Condition]:
        """Tweak a condition's parameters.

        Args:
            condition: Condition or CompoundCondition to tweak

        Returns:
            Modified condition (or original if compound)
        """
        if condition is None:
            return None

        # Skip CompoundConditions - they're complex to mutate safely
        if not hasattr(condition, 'value'):
            return condition

        # Tweak value by ±2 (only for numeric values)
        if condition.value is not None and isinstance(condition.value, (int, float)):
            new_value = max(0, condition.value + random.randint(-2, 2))
            return replace(condition, value=new_value)

        # Or change operator
        if hasattr(condition, 'operator') and condition.operator is not None:
            operators = [Operator.EQ, Operator.GT, Operator.LT, Operator.GE, Operator.LE]
            new_operator = random.choice([op for op in operators if op != condition.operator])
            return replace(condition, operator=new_operator)

        return condition


class ReplacePhaseMutation(MutationOperator):
    """Replace a phase with a completely different random phase.

    More disruptive than other mutations - creates structural diversity.
    """

    def __init__(self, probability: float = 0.15):
        """Initialize phase replacement mutation.

        Args:
            probability: Mutation probability (default: 15%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Replace a random phase with a new random one.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with replaced phase
        """
        phases = list(genome.turn_structure.phases)

        if not phases:
            return genome

        # Pick random phase to replace
        idx = random.randint(0, len(phases) - 1)

        # Generate completely new random phase
        # Weight towards simpler phases, but allow complex ones
        phase_type = random.choices(
            ['draw', 'play', 'discard', 'trick', 'claim'],
            weights=[30, 30, 20, 10, 10],
            k=1
        )[0]

        if phase_type == 'draw':
            new_phase = DrawPhase(
                source=random.choice([Location.DECK, Location.DISCARD]),
                count=random.randint(1, 5),
                mandatory=random.choice([True, False]),
                condition=self._random_condition() if random.random() < 0.3 else None
            )
        elif phase_type == 'play':
            new_phase = PlayPhase(
                target=random.choice([Location.DISCARD, Location.TABLEAU]),
                valid_play_condition=self._random_condition(),
                min_cards=random.randint(0, 2),
                max_cards=random.randint(1, 10),
                mandatory=random.choice([True, False])
            )
        elif phase_type == 'discard':
            new_phase = DiscardPhase(
                target=Location.DISCARD,
                count=random.randint(1, 3),
                mandatory=random.choice([True, False])
            )
        elif phase_type == 'trick':
            new_phase = TrickPhase(
                lead_suit_required=random.choice([True, False]),
                trump_suit=random.choice([None, Suit.SPADES, Suit.HEARTS, Suit.DIAMONDS, Suit.CLUBS]),
                high_card_wins=random.choice([True, False]),
                breaking_suit=random.choice([None, Suit.HEARTS, Suit.SPADES])
            )
        else:  # claim (bluffing)
            new_phase = ClaimPhase(
                min_cards=1,
                max_cards=random.choice([1, 2, 3, 4]),
                sequential_rank=random.choice([True, False]),
                allow_challenge=True,
                pile_penalty=True
            )

        phases[idx] = new_phase
        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(genome, turn_structure=new_turn, generation=genome.generation + 1)

    def _random_condition(self) -> Condition:
        """Generate a random condition."""
        cond_type = random.choice([
            ConditionType.HAND_SIZE,
            ConditionType.LOCATION_SIZE,
        ])
        operator = random.choice([Operator.GT, Operator.GE, Operator.LT, Operator.LE, Operator.EQ])
        value = random.randint(0, 10)
        return Condition(type=cond_type, operator=operator, value=value)


class ModifyDrawCountMutation(MutationOperator):
    """Aggressively modify draw counts in DrawPhases."""

    def __init__(self, probability: float = 0.2):
        """Initialize draw count mutation.

        Args:
            probability: Mutation probability (default: 20%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Modify draw count in a random DrawPhase.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with modified draw count
        """
        phases = list(genome.turn_structure.phases)

        # Find DrawPhases
        draw_indices = [i for i, p in enumerate(phases) if isinstance(p, DrawPhase)]

        if not draw_indices:
            return genome

        idx = random.choice(draw_indices)
        phase = phases[idx]

        # Set new count (1-7, more aggressive range)
        new_count = random.randint(1, 7)
        new_phase = replace(phase, count=new_count)

        phases[idx] = new_phase
        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(genome, turn_structure=new_turn, generation=genome.generation + 1)


class ShuffleAllPhasesMutation(MutationOperator):
    """Completely shuffle the order of all phases.

    Very disruptive - creates entirely new turn structures.
    """

    def __init__(self, probability: float = 0.05):
        """Initialize phase shuffle mutation.

        Args:
            probability: Mutation probability (default: 5%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Shuffle all phases randomly.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with shuffled phases
        """
        phases = list(genome.turn_structure.phases)

        if len(phases) < 2:
            return genome

        random.shuffle(phases)
        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(genome, turn_structure=new_turn, generation=genome.generation + 1)


class ModifyWinConditionMutation(MutationOperator):
    """Modify or add win conditions.

    Supports three mutation types:
    1. Change win condition type (empty_hand, high_score, etc.)
    2. Change threshold (±20% for score-based conditions)
    3. Add new win condition (up to 3 total)
    """

    WIN_CONDITION_TYPES = [
        "empty_hand", "high_score", "first_to_score", "capture_all",
        "low_score", "all_hands_empty",  # Trick-taking games
        "most_captured", "best_hand"  # Scopa-style capture, Poker hand evaluation
    ]

    def __init__(self, probability: float = 0.1):
        """Initialize win condition mutation operator.

        Args:
            probability: Mutation probability (default: 10%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Mutate win conditions.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with mutated win conditions
        """
        if not genome.win_conditions:
            # No win conditions - add one
            return self._add_win_condition(genome)

        # Choose mutation type
        mutation_type = random.choice(["change_type", "change_threshold", "add_condition"])

        if mutation_type == "change_type":
            return self._change_win_condition_type(genome)
        elif mutation_type == "change_threshold":
            return self._change_threshold(genome)
        else:  # add_condition
            if len(genome.win_conditions) < 3:
                return self._add_win_condition(genome)
            else:
                # Already at max - change existing instead
                return self._change_win_condition_type(genome)

    def _change_win_condition_type(self, genome: GameGenome) -> GameGenome:
        """Change type of a random win condition.

        Ensures coherent mutations: when changing to a scoring-based win type,
        adds scoring rules if genome lacks them.

        Args:
            genome: Source genome

        Returns:
            New genome with modified win condition type
        """
        # Pick random win condition to modify
        idx = random.randint(0, len(genome.win_conditions) - 1)
        old_wc = genome.win_conditions[idx]

        # Choose new type (different from current)
        available_types = [t for t in self.WIN_CONDITION_TYPES if t != old_wc.type]
        new_type = random.choice(available_types)

        # Set threshold based on new type
        scoring_based_types = ["first_to_score", "high_score", "low_score"]
        if new_type in scoring_based_types:
            # Score-based: use reasonable threshold
            new_threshold = random.choice([50, 100, 200, 500])
        else:
            new_threshold = None

        # Create new win condition
        new_wc = WinCondition(type=new_type, threshold=new_threshold)

        # Build new win_conditions list
        new_win_conditions = [
            new_wc if i == idx else wc
            for i, wc in enumerate(genome.win_conditions)
        ]

        # COHERENCE: Ensure scoring rules exist for scoring-based win conditions
        new_card_scoring = genome.card_scoring
        if new_type in scoring_based_types:
            # Check if genome lacks scoring
            has_scoring = (
                len(genome.card_scoring) > 0 or
                len(genome.scoring_rules) > 0 or
                genome.turn_structure.is_trick_based  # Trick games track score via captures
            )
            if not has_scoring:
                # Add basic scoring rule: all cards = 1 point when played
                basic_scoring = CardScoringRule(
                    condition=CardCondition(),  # Match any card
                    points=1,
                    trigger=ScoringTrigger.PLAY,
                )
                new_card_scoring = (basic_scoring,)

        # Return new genome (immutable)
        return GameGenome(
            schema_version=genome.schema_version,
            genome_id=genome.genome_id,
            generation=genome.generation + 1,
            setup=genome.setup,
            turn_structure=genome.turn_structure,
            special_effects=genome.special_effects,
            win_conditions=new_win_conditions,
            scoring_rules=genome.scoring_rules,
            max_turns=genome.max_turns,
            min_turns=genome.min_turns,
            player_count=genome.player_count,
            card_scoring=new_card_scoring,
        )

    def _change_threshold(self, genome: GameGenome) -> GameGenome:
        """Change threshold of a score-based win condition by ±20%.

        Args:
            genome: Source genome

        Returns:
            New genome with modified threshold
        """
        # Find score-based conditions
        score_based_indices = [
            i for i, wc in enumerate(genome.win_conditions)
            if wc.type in ["first_to_score", "high_score", "low_score"] and wc.threshold is not None
        ]

        if not score_based_indices:
            # No score-based conditions - do nothing
            return genome

        # Pick random score-based condition
        idx = random.choice(score_based_indices)
        old_wc = genome.win_conditions[idx]

        # Change threshold by ±20%
        delta = random.uniform(-0.2, 0.2)
        new_threshold = max(10, int(old_wc.threshold * (1 + delta)))  # Min threshold = 10

        # Create new win condition
        new_wc = WinCondition(type=old_wc.type, threshold=new_threshold)

        # Build new win_conditions list
        new_win_conditions = [
            new_wc if i == idx else wc
            for i, wc in enumerate(genome.win_conditions)
        ]

        # Return new genome
        return GameGenome(
            schema_version=genome.schema_version,
            genome_id=genome.genome_id,
            generation=genome.generation + 1,
            setup=genome.setup,
            turn_structure=genome.turn_structure,
            special_effects=genome.special_effects,
            win_conditions=new_win_conditions,
            scoring_rules=genome.scoring_rules,
            max_turns=genome.max_turns,
            min_turns=genome.min_turns,
            player_count=genome.player_count,
        )

    def _add_win_condition(self, genome: GameGenome) -> GameGenome:
        """Add a new win condition (up to 3 total).

        Ensures coherent mutations: when adding a scoring-based win type,
        adds scoring rules if genome lacks them.

        Args:
            genome: Source genome

        Returns:
            New genome with additional win condition
        """
        # Choose random type
        new_type = random.choice(self.WIN_CONDITION_TYPES)

        # Set threshold if needed
        scoring_based_types = ["first_to_score", "high_score", "low_score"]
        if new_type in scoring_based_types:
            new_threshold = random.choice([50, 100, 200, 500])
        else:
            new_threshold = None

        # Create new win condition
        new_wc = WinCondition(type=new_type, threshold=new_threshold)

        # Add to existing list
        new_win_conditions = list(genome.win_conditions) + [new_wc]

        # COHERENCE: Ensure scoring rules exist for scoring-based win conditions
        new_card_scoring = genome.card_scoring
        if new_type in scoring_based_types:
            # Check if genome lacks scoring
            has_scoring = (
                len(genome.card_scoring) > 0 or
                len(genome.scoring_rules) > 0 or
                genome.turn_structure.is_trick_based  # Trick games track score via captures
            )
            if not has_scoring:
                # Add basic scoring rule: all cards = 1 point when played
                basic_scoring = CardScoringRule(
                    condition=CardCondition(),  # Match any card
                    points=1,
                    trigger=ScoringTrigger.PLAY,
                )
                new_card_scoring = (basic_scoring,)

        # Return new genome
        return GameGenome(
            schema_version=genome.schema_version,
            genome_id=genome.genome_id,
            generation=genome.generation + 1,
            setup=genome.setup,
            turn_structure=genome.turn_structure,
            special_effects=genome.special_effects,
            win_conditions=new_win_conditions,
            scoring_rules=genome.scoring_rules,
            max_turns=genome.max_turns,
            min_turns=genome.min_turns,
            player_count=genome.player_count,
            card_scoring=new_card_scoring,
        )


class AddEffectMutation(MutationOperator):
    """Add a random special effect to the genome."""

    def __init__(self, probability: float = 0.1):
        """Initialize effect addition mutation.

        Args:
            probability: Mutation probability (default: 10%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Add a new random special effect.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with additional special effect
        """
        new_effect = SpecialEffect(
            trigger_rank=random.choice(list(Rank)),
            effect_type=random.choice(list(EffectType)),
            target=random.choice([
                TargetSelector.NEXT_PLAYER,
                TargetSelector.ALL_OPPONENTS,
            ]),
            value=random.randint(1, 3),
        )
        new_effects = list(genome.special_effects) + [new_effect]
        return replace(genome, special_effects=new_effects, generation=genome.generation + 1)


class RemoveEffectMutation(MutationOperator):
    """Remove a random special effect from the genome."""

    def __init__(self, probability: float = 0.1):
        """Initialize effect removal mutation.

        Args:
            probability: Mutation probability (default: 10%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Remove a random special effect.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with removed special effect, or original if no effects
        """
        if not genome.special_effects:
            return genome
        idx = random.randrange(len(genome.special_effects))
        new_effects = [e for i, e in enumerate(genome.special_effects) if i != idx]
        return replace(genome, special_effects=new_effects, generation=genome.generation + 1)


class MutateEffectMutation(MutationOperator):
    """Mutate one field of a random special effect."""

    def __init__(self, probability: float = 0.15):
        """Initialize effect mutation operator.

        Args:
            probability: Mutation probability (default: 15%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Mutate one field of a random effect.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with mutated effect, or original if no effects
        """
        if not genome.special_effects:
            return genome

        idx = random.randrange(len(genome.special_effects))
        effect = genome.special_effects[idx]

        field = random.choice(['rank', 'type', 'target', 'value'])
        if field == 'rank':
            mutated = SpecialEffect(
                random.choice(list(Rank)),
                effect.effect_type,
                effect.target,
                effect.value,
            )
        elif field == 'type':
            mutated = SpecialEffect(
                effect.trigger_rank,
                random.choice(list(EffectType)),
                effect.target,
                effect.value,
            )
        elif field == 'target':
            mutated = SpecialEffect(
                effect.trigger_rank,
                effect.effect_type,
                random.choice([TargetSelector.NEXT_PLAYER, TargetSelector.ALL_OPPONENTS]),
                effect.value,
            )
        else:  # value
            new_value = max(1, min(4, effect.value + random.randint(-1, 1)))
            mutated = SpecialEffect(
                effect.trigger_rank,
                effect.effect_type,
                effect.target,
                new_value,
            )

        new_effects = list(genome.special_effects)
        new_effects[idx] = mutated
        return replace(genome, special_effects=new_effects, generation=genome.generation + 1)


class AddBettingPhaseMutation(MutationOperator):
    """Insert a BettingPhase at random position in turn structure."""

    def __init__(self, probability: float = 0.05):
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        phases = list(genome.turn_structure.phases)

        # Don't add too many phases (max 5)
        if len(phases) >= 5:
            return genome

        # Validate min_bet <= starting_chips
        starting_chips = genome.setup.starting_chips or 1000
        min_bet_options = [b for b in [5, 10, 20, 50] if b <= starting_chips]
        if not min_bet_options:
            min_bet_options = [max(1, starting_chips // 10)]

        new_phase = BettingPhase(
            min_bet=random.choice(min_bet_options),
            max_raises=random.choice([1, 2, 3, 4]),
        )

        insert_pos = random.randint(0, len(phases))
        phases.insert(insert_pos, new_phase)

        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(genome, turn_structure=new_turn, generation=genome.generation + 1)


class RemoveBettingPhaseMutation(MutationOperator):
    """Remove a random BettingPhase from turn structure."""

    def __init__(self, probability: float = 0.05):
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        phases = list(genome.turn_structure.phases)

        # Find BettingPhase indices
        betting_indices = [i for i, p in enumerate(phases) if isinstance(p, BettingPhase)]

        if not betting_indices:
            return genome

        # Don't remove if only 1 phase left total
        if len(phases) <= 1:
            return genome

        idx = random.choice(betting_indices)
        phases.pop(idx)

        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(genome, turn_structure=new_turn, generation=genome.generation + 1)


class MutateBettingPhaseMutation(MutationOperator):
    """Mutate parameters of a random BettingPhase."""

    def __init__(self, probability: float = 0.10):
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        phases = list(genome.turn_structure.phases)

        # Find BettingPhase indices
        betting_indices = [i for i, p in enumerate(phases) if isinstance(p, BettingPhase)]

        if not betting_indices:
            return genome

        idx = random.choice(betting_indices)
        phase = phases[idx]

        # Randomly mutate min_bet or max_raises
        starting_chips = genome.setup.starting_chips or 1000

        if random.random() < 0.5:
            # Mutate min_bet (+-50%, stay within bounds)
            delta = random.uniform(-0.5, 0.5)
            new_min_bet = max(1, min(starting_chips, int(phase.min_bet * (1 + delta))))
            new_phase = replace(phase, min_bet=new_min_bet)
        else:
            # Mutate max_raises (+-1, range 1-5)
            delta = random.choice([-1, 1])
            new_max_raises = max(1, min(5, phase.max_raises + delta))
            new_phase = replace(phase, max_raises=new_max_raises)

        phases[idx] = new_phase
        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(genome, turn_structure=new_turn, generation=genome.generation + 1)


class MutateStartingChipsMutation(MutationOperator):
    """Mutate starting_chips in setup, ensuring coherent betting.

    Ensures coherent mutations: when enabling betting (adding chips to a
    genome with 0 chips), also adds a BettingPhase if none exists.
    """

    def __init__(self, probability: float = 0.10):
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        current_chips = genome.setup.starting_chips or 0

        if current_chips == 0:
            # Enable betting by adding starting chips
            new_chips = random.choice([100, 500, 1000, 2000])
        else:
            # Mutate by +-50%
            delta = random.uniform(-0.5, 0.5)
            new_chips = max(10, int(current_chips * (1 + delta)))

        # Ensure all BettingPhases have valid min_bet
        phases = list(genome.turn_structure.phases)
        phases_modified = False

        for i, phase in enumerate(phases):
            if isinstance(phase, BettingPhase) and phase.min_bet > new_chips:
                phases[i] = replace(phase, min_bet=max(1, new_chips // 10))
                phases_modified = True

        # COHERENCE: When enabling betting (0 -> chips), ensure BettingPhase exists
        has_betting_phase = any(isinstance(p, BettingPhase) for p in phases)
        if current_chips == 0 and not has_betting_phase:
            # Add BettingPhase with min_bet = 10% of chips, reasonable max_raises
            min_bet = max(1, new_chips // 10)
            betting_phase = BettingPhase(min_bet=min_bet, max_raises=3)
            # Insert at beginning of turn (before draw/play phases)
            phases.insert(0, betting_phase)
            phases_modified = True

        new_setup = replace(genome.setup, starting_chips=new_chips)

        if phases_modified:
            new_turn = replace(genome.turn_structure, phases=tuple(phases))
            return replace(genome, setup=new_setup, turn_structure=new_turn, generation=genome.generation + 1)
        else:
            return replace(genome, setup=new_setup, generation=genome.generation + 1)


class AddBiddingPhaseMutation(MutationOperator):
    """Add BiddingPhase to trick-taking games.

    Only applies if game has TrickPhase but no BiddingPhase.
    Inserts BiddingPhase before first TrickPhase.
    Also adds default ContractScoring.
    """

    def __init__(self, probability: float = 0.05):
        super().__init__(probability)

    def can_apply(self, genome: GameGenome) -> bool:
        has_bidding = any(isinstance(p, BiddingPhase) for p in genome.turn_structure.phases)
        has_trick = any(isinstance(p, TrickPhase) for p in genome.turn_structure.phases)
        return not has_bidding and has_trick

    def mutate(self, genome: GameGenome) -> GameGenome:
        if not self.can_apply(genome):
            return genome

        phases = list(genome.turn_structure.phases)

        # Find first TrickPhase and insert BiddingPhase before it
        for i, phase in enumerate(phases):
            if isinstance(phase, TrickPhase):
                phases.insert(i, BiddingPhase())
                break

        new_turn = replace(genome.turn_structure, phases=tuple(phases))
        return replace(
            genome,
            turn_structure=new_turn,
            contract_scoring=ContractScoring(),
            generation=genome.generation + 1
        )


class RemoveBiddingPhaseMutation(MutationOperator):
    """Remove BiddingPhase from genome.

    Also removes ContractScoring.
    """

    def __init__(self, probability: float = 0.05):
        super().__init__(probability)

    def can_apply(self, genome: GameGenome) -> bool:
        return any(isinstance(p, BiddingPhase) for p in genome.turn_structure.phases)

    def mutate(self, genome: GameGenome) -> GameGenome:
        if not self.can_apply(genome):
            return genome

        phases = [p for p in genome.turn_structure.phases if not isinstance(p, BiddingPhase)]
        new_turn = replace(genome.turn_structure, phases=tuple(phases))

        return replace(
            genome,
            turn_structure=new_turn,
            contract_scoring=None,
            generation=genome.generation + 1
        )


class CleanupOrphanedResourcesMutation(MutationOperator):
    """Remove orphaned resources that lack supporting mechanics.

    This is a "repair" mutation that fixes incoherent states caused by crossover
    or other mutations. It detects and removes resources that have no use:
    - starting_chips without any BettingPhase
    - contract_scoring without any BiddingPhase
    - hand_evaluation without best_hand win condition or HAND_EVALUATION showdown

    This mutation has high probability since it only makes changes when needed.
    """

    def __init__(self, probability: float = 0.50):
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        modified = False
        new_setup = genome.setup
        new_contract_scoring = genome.contract_scoring
        new_hand_evaluation = genome.hand_evaluation

        # Check for orphaned chips
        has_betting_phase = any(
            isinstance(p, BettingPhase)
            for p in genome.turn_structure.phases
        )

        if genome.setup.starting_chips > 0 and not has_betting_phase:
            # Remove orphaned chips
            new_setup = replace(new_setup, starting_chips=0)
            modified = True

        # Check for orphaned contract_scoring
        has_bidding_phase = any(
            isinstance(p, BiddingPhase)
            for p in genome.turn_structure.phases
        )

        if genome.contract_scoring is not None and not has_bidding_phase:
            # Remove orphaned contract_scoring
            new_contract_scoring = None
            modified = True

        # Check for orphaned hand_evaluation
        # hand_evaluation is used by: best_hand win condition OR BettingPhase with HAND_EVALUATION showdown
        if genome.hand_evaluation is not None:
            has_best_hand_win = any(
                wc.type == "best_hand"
                for wc in genome.win_conditions
            )
            has_hand_eval_showdown = any(
                isinstance(p, BettingPhase) and p.showdown_method == ShowdownMethod.HAND_EVALUATION
                for p in genome.turn_structure.phases
            )
            if not has_best_hand_win and not has_hand_eval_showdown:
                # Remove orphaned hand_evaluation
                new_hand_evaluation = None
                modified = True

        if modified:
            return replace(
                genome,
                setup=new_setup,
                contract_scoring=new_contract_scoring,
                hand_evaluation=new_hand_evaluation,
                generation=genome.generation + 1
            )
        return genome


class MutateTableauModeMutation(MutationOperator):
    """Change the tableau interaction mode.

    This mutation changes how cards interact on the tableau. The tableau_mode
    determines behaviors like comparing cards (WAR), matching ranks, or building
    sequences.

    Constraint: WAR mode is only valid for 2-player games.
    """

    def __init__(self, probability: float = 0.05):
        """Initialize tableau mode mutation.

        Args:
            probability: Mutation probability (default: 5% - low weight as significant change)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Change the tableau interaction mode.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with different tableau mode
        """
        # Get valid modes for this player count
        valid_modes = [TableauMode.NONE, TableauMode.MATCH_RANK, TableauMode.SEQUENCE]
        if genome.player_count == 2:
            valid_modes.append(TableauMode.WAR)

        # Remove current mode to ensure change
        valid_modes = [m for m in valid_modes if m != genome.setup.tableau_mode]

        if not valid_modes:
            return genome

        new_mode = random.choice(valid_modes)

        # Create new setup with updated tableau_mode
        new_setup = replace(
            genome.setup,
            tableau_mode=new_mode,
        )

        return replace(genome, setup=new_setup, generation=genome.generation + 1)


class MutateSequenceDirectionMutation(MutationOperator):
    """Change the sequence direction (only when mode is SEQUENCE).

    This mutation only applies when tableau_mode is SEQUENCE. It changes
    whether sequences must be ascending, descending, or can go either way.
    """

    def __init__(self, probability: float = 0.03):
        """Initialize sequence direction mutation.

        Args:
            probability: Mutation probability (default: 3% - low weight)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Change the sequence direction.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with different sequence direction, or unchanged if not SEQUENCE mode
        """
        if genome.setup.tableau_mode != TableauMode.SEQUENCE:
            return genome  # No-op if not sequence mode

        directions = [SequenceDirection.ASCENDING, SequenceDirection.DESCENDING, SequenceDirection.BOTH]
        directions = [d for d in directions if d != genome.setup.sequence_direction]

        if not directions:
            return genome

        new_direction = random.choice(directions)

        new_setup = replace(
            genome.setup,
            sequence_direction=new_direction,
        )

        return replace(genome, setup=new_setup, generation=genome.generation + 1)


class MutateTableauVisibilityMutation(MutationOperator):
    """Change the tableau visibility setting.

    This mutation changes whether cards on the tableau are visible to all players.
    Setting tableau_visibility to FACE_DOWN enables memory/matching games like
    Concentration where players must remember card positions.

    Note: Full simulation support for hidden tableau requires card state tracking.
    This mutation prepares genomes for memory-style games.
    """

    def __init__(self, probability: float = 0.03):
        """Initialize tableau visibility mutation.

        Args:
            probability: Mutation probability (default: 3% - low weight as significant change)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Change the tableau visibility setting.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with different tableau visibility
        """
        # Valid visibility options for tableau
        valid_options = [Visibility.FACE_UP, Visibility.FACE_DOWN]

        # Remove current option to ensure change
        valid_options = [v for v in valid_options if v != genome.setup.tableau_visibility]

        if not valid_options:
            return genome

        new_visibility = random.choice(valid_options)

        new_setup = replace(
            genome.setup,
            tableau_visibility=new_visibility,
        )

        return replace(genome, setup=new_setup, generation=genome.generation + 1)


class AddCardScoringMutation(MutationOperator):
    """Add a random card scoring rule."""

    def __init__(self, probability: float = 0.05):
        """Initialize card scoring mutation.

        Args:
            probability: Mutation probability (default: 5%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Add a new card scoring rule.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with additional card scoring rule
        """
        # Pick random suit (or None for any)
        suit = random.choice([None, Suit.HEARTS, Suit.DIAMONDS, Suit.CLUBS, Suit.SPADES])

        # Pick random rank (or None for any)
        rank = random.choice([None] + list(Rank))

        # Pick points (-5 to 15)
        points = random.randint(-5, 15)

        # Pick trigger
        trigger = random.choice(list(ScoringTrigger))

        new_rule = CardScoringRule(
            condition=CardCondition(suit=suit, rank=rank),
            points=points,
            trigger=trigger,
        )

        new_scoring = genome.card_scoring + (new_rule,)
        return replace(genome, card_scoring=new_scoring, generation=genome.generation + 1)


class MutateHandPatternMutation(MutationOperator):
    """Mutate rank_priority in a hand pattern."""

    def __init__(self, probability: float = 0.05):
        """Initialize hand pattern mutation.

        Args:
            probability: Mutation probability (default: 5%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Mutate rank_priority of a random hand pattern.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with mutated hand pattern
        """
        if genome.hand_evaluation is None or not genome.hand_evaluation.patterns:
            return genome

        patterns = list(genome.hand_evaluation.patterns)
        idx = random.randrange(len(patterns))
        old = patterns[idx]

        # Mutate priority by ±5-10
        delta = random.choice([-10, -5, 5, 10])
        new_priority = max(1, min(100, old.rank_priority + delta))

        patterns[idx] = HandPattern(
            name=old.name,
            rank_priority=new_priority,
            required_count=old.required_count,
            same_suit_count=old.same_suit_count,
            same_rank_groups=old.same_rank_groups,
            sequence_length=old.sequence_length,
            sequence_wrap=old.sequence_wrap,
            required_ranks=old.required_ranks,
        )

        new_eval = HandEvaluation(
            method=genome.hand_evaluation.method,
            patterns=tuple(patterns),
            card_values=genome.hand_evaluation.card_values,
            target_value=genome.hand_evaluation.target_value,
            bust_threshold=genome.hand_evaluation.bust_threshold,
        )

        return replace(genome, hand_evaluation=new_eval, generation=genome.generation + 1)


class MutateCardValueMutation(MutationOperator):
    """Mutate point values in card_values."""

    def __init__(self, probability: float = 0.05):
        """Initialize card value mutation.

        Args:
            probability: Mutation probability (default: 5%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Mutate point value of a random card value entry.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with mutated card value
        """
        if genome.hand_evaluation is None or not genome.hand_evaluation.card_values:
            return genome

        values = list(genome.hand_evaluation.card_values)
        idx = random.randrange(len(values))
        old = values[idx]

        # Mutate value by ±1-2
        delta = random.choice([-2, -1, 1, 2])
        new_value = max(1, min(15, old.value + delta))

        values[idx] = CardValue(
            rank=old.rank,
            value=new_value,
            alternate_value=old.alternate_value,
        )

        new_eval = HandEvaluation(
            method=genome.hand_evaluation.method,
            patterns=genome.hand_evaluation.patterns,
            card_values=tuple(values),
            target_value=genome.hand_evaluation.target_value,
            bust_threshold=genome.hand_evaluation.bust_threshold,
        )

        return replace(genome, hand_evaluation=new_eval, generation=genome.generation + 1)


class MutateCardScoringMutation(MutationOperator):
    """Mutate points in an existing card scoring rule."""

    def __init__(self, probability: float = 0.1):
        """Initialize card scoring mutation operator.

        Args:
            probability: Mutation probability (default: 10%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Mutate points in a random card scoring rule.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with mutated card scoring rule, or original if no rules
        """
        if not genome.card_scoring:
            return genome

        # Pick random rule to mutate
        idx = random.randrange(len(genome.card_scoring))
        old_rule = genome.card_scoring[idx]

        # Mutate points by ±1-3
        delta = random.choice([-3, -2, -1, 1, 2, 3])
        new_points = old_rule.points + delta

        new_rule = CardScoringRule(
            condition=old_rule.condition,
            points=new_points,
            trigger=old_rule.trigger,
        )

        new_scoring = genome.card_scoring[:idx] + (new_rule,) + genome.card_scoring[idx+1:]
        return replace(genome, card_scoring=new_scoring, generation=genome.generation + 1)


class RemoveCardScoringMutation(MutationOperator):
    """Remove a random card scoring rule."""

    def __init__(self, probability: float = 0.05):
        """Initialize card scoring removal mutation operator.

        Args:
            probability: Mutation probability (default: 5%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Remove a random card scoring rule.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with removed card scoring rule, or original if no rules
        """
        if not genome.card_scoring:
            return genome

        idx = random.randrange(len(genome.card_scoring))
        new_scoring = genome.card_scoring[:idx] + genome.card_scoring[idx+1:]
        return replace(genome, card_scoring=new_scoring, generation=genome.generation + 1)


class EnableTeamModeMutation(MutationOperator):
    """Enables team mode with default 2v2 configuration.

    For 4 players: Creates 2 teams of 2 (alternating: (0,2), (1,3))
    For 6 players: Creates 2 teams of 3 (alternating)
    Only applies to even player counts >= 4.
    """

    def __init__(self, probability: float = 0.03):
        """Initialize team mode enabling mutation.

        Args:
            probability: Mutation probability (default: 3% - low weight as significant change)
        """
        super().__init__(probability)

    def can_apply(self, genome: GameGenome) -> bool:
        """Check if mutation can be applied.

        Returns False if:
        - Team mode is already enabled
        - Player count is less than 4
        - Player count is odd (can't form equal teams)
        """
        if genome.team_mode:
            return False  # Already enabled
        num_players = genome.player_count
        if num_players < 4 or num_players % 2 != 0:
            return False  # Need even number of 4+ players
        return True

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Enable team mode with alternating player assignment.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with team mode enabled
        """
        if not self.can_apply(genome):
            return genome

        num_players = genome.player_count
        # Create 2 teams with alternating player assignment
        team0 = tuple(range(0, num_players, 2))  # Even indices: 0, 2, 4...
        team1 = tuple(range(1, num_players, 2))  # Odd indices: 1, 3, 5...
        return replace(
            genome,
            team_mode=True,
            teams=(team0, team1),
            generation=genome.generation + 1,
        )


class DisableTeamModeMutation(MutationOperator):
    """Disables team mode, removing team assignments."""

    def __init__(self, probability: float = 0.03):
        """Initialize team mode disabling mutation.

        Args:
            probability: Mutation probability (default: 3% - low weight as significant change)
        """
        super().__init__(probability)

    def can_apply(self, genome: GameGenome) -> bool:
        """Check if mutation can be applied.

        Returns False if team mode is already disabled.
        """
        return genome.team_mode

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Disable team mode.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with team mode disabled
        """
        if not self.can_apply(genome):
            return genome

        return replace(
            genome,
            team_mode=False,
            teams=(),
            generation=genome.generation + 1,
        )


class MutateTeamAssignmentMutation(MutationOperator):
    """Swaps one player between teams.

    Randomly selects one player from each team and swaps them,
    maintaining valid team configuration.
    """

    def __init__(self, probability: float = 0.05):
        """Initialize team assignment mutation.

        Args:
            probability: Mutation probability (default: 5%)
        """
        super().__init__(probability)

    def can_apply(self, genome: GameGenome) -> bool:
        """Check if mutation can be applied.

        Returns False if:
        - Team mode is not enabled
        - Less than 2 teams exist
        - Either of the first two teams has no players
        """
        if not genome.team_mode:
            return False
        if len(genome.teams) < 2:
            return False
        # Need at least 1 player in each of first 2 teams
        if len(genome.teams[0]) < 1 or len(genome.teams[1]) < 1:
            return False
        return True

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Swap one player between the first two teams.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with swapped player assignments
        """
        if not self.can_apply(genome):
            return genome

        teams_list = [list(t) for t in genome.teams]

        # Pick random player from each of first two teams
        team0_idx = random.randrange(len(teams_list[0]))
        team1_idx = random.randrange(len(teams_list[1]))

        # Swap them
        teams_list[0][team0_idx], teams_list[1][team1_idx] = (
            teams_list[1][team1_idx], teams_list[0][team0_idx]
        )

        # Convert back to tuples (sorted for consistency)
        new_teams = tuple(tuple(sorted(t)) for t in teams_list)

        return replace(
            genome,
            teams=new_teams,
            generation=genome.generation + 1,
        )


class CrossoverOperator:
    """Semantic crossover operator for game genomes.

    Performs single-point crossover on turn structure phases,
    preserving semantic validity.
    """

    def __init__(self, probability: float = 0.7):
        """Initialize crossover operator.

        Args:
            probability: Crossover probability (default: 70%)
        """
        self.probability = probability

    def should_apply(self) -> bool:
        """Check if crossover should be applied based on probability."""
        return random.random() < self.probability

    def crossover(self, parent1: GameGenome, parent2: GameGenome) -> tuple[GameGenome, GameGenome]:
        """Perform crossover between two parent genomes.

        Uses single-point crossover on turn structure phases.

        Args:
            parent1: First parent genome
            parent2: Second parent genome

        Returns:
            Tuple of two offspring genomes
        """
        if not self.should_apply():
            # No crossover - return parents unchanged
            return parent1, parent2

        # Get phases from both parents
        phases1 = list(parent1.turn_structure.phases)
        phases2 = list(parent2.turn_structure.phases)

        # If either parent has no phases, return parents unchanged
        if not phases1 or not phases2:
            return parent1, parent2

        # Single-point crossover
        point1 = random.randint(0, len(phases1))
        point2 = random.randint(0, len(phases2))

        # Create offspring phase lists
        offspring1_phases = phases1[:point1] + phases2[point2:]
        offspring2_phases = phases2[:point2] + phases1[point1:]

        # Ensure at least one phase
        if not offspring1_phases:
            offspring1_phases = [phases1[0]]
        if not offspring2_phases:
            offspring2_phases = [phases2[0]]

        # Limit to max 5 phases
        offspring1_phases = offspring1_phases[:5]
        offspring2_phases = offspring2_phases[:5]

        # Create offspring genomes with new random names
        # Inherit from parent1
        offspring1 = replace(
            parent1,
            turn_structure=replace(parent1.turn_structure, phases=tuple(offspring1_phases)),
            generation=parent1.generation + 1,
            genome_id=generate_name()
        )

        # Inherit from parent2
        offspring2 = replace(
            parent2,
            turn_structure=replace(parent2.turn_structure, phases=tuple(offspring2_phases)),
            generation=parent2.generation + 1,
            genome_id=generate_name()
        )

        return offspring1, offspring2


class MutationPipeline:
    """Pipeline of mutation operators applied sequentially."""

    def __init__(self, operators: List[MutationOperator]):
        """Initialize mutation pipeline.

        Args:
            operators: List of mutation operators to apply
        """
        self.operators = operators

    def apply(self, genome: GameGenome) -> GameGenome:
        """Apply all operators in sequence.

        Each operator is applied independently based on its probability.

        Args:
            genome: Genome to mutate

        Returns:
            Mutated genome
        """
        mutated = genome
        for operator in self.operators:
            if operator.should_apply():
                mutated = operator.mutate(mutated)
        return mutated


def create_default_pipeline(
    aggressive: bool = False,
    preserve_player_count: bool = False
) -> MutationPipeline:
    """Create default mutation pipeline with standard operators.

    Args:
        aggressive: If True, use higher mutation rates for escaping local optima
        preserve_player_count: If True, don't mutate player_count (for filtered evolution)

    Returns:
        MutationPipeline with all mutation operators
    """
    # Multiplier for aggressive mode (2x rates)
    mult = 2.0 if aggressive else 1.0

    operators = [
        # Parameter tweaks (with optional player_count preservation)
        TweakParameterMutation(
            probability=min(0.30 * mult, 0.6),
            preserve_player_count=preserve_player_count
        ),  # 30% (60% aggressive)

        # Structural mutations
        SwapPhaseOrderMutation(probability=min(0.15 * mult, 0.3)),      # 15% (30% aggressive)
        AddPhaseMutation(probability=min(0.12 * mult, 0.25)),           # 12% (24% aggressive)
        RemovePhaseMutation(probability=min(0.12 * mult, 0.25)),        # 12% (24% aggressive)
        ReplacePhaseMutation(probability=min(0.15 * mult, 0.3)),        # 15% (30% aggressive) - NEW
        ShuffleAllPhasesMutation(probability=min(0.05 * mult, 0.15)),   # 5% (10% aggressive) - NEW

        # Condition/parameter mutations
        ModifyConditionMutation(probability=min(0.20 * mult, 0.4)),     # 20% (40% aggressive)
        ModifyDrawCountMutation(probability=min(0.20 * mult, 0.4)),     # 20% (40% aggressive) - NEW

        # Win condition mutations
        ModifyWinConditionMutation(probability=min(0.15 * mult, 0.3)),  # 15% (30% aggressive)

        # Special effect mutations
        AddEffectMutation(probability=min(0.10 * mult, 0.2)),           # 10% (20% aggressive)
        RemoveEffectMutation(probability=min(0.10 * mult, 0.2)),        # 10% (20% aggressive)
        MutateEffectMutation(probability=min(0.15 * mult, 0.3)),        # 15% (30% aggressive)

        # Betting mutations (poker-style chip betting)
        AddBettingPhaseMutation(probability=min(0.05 * mult, 0.15)),    # 5% (10% aggressive)
        RemoveBettingPhaseMutation(probability=min(0.05 * mult, 0.15)), # 5% (10% aggressive)
        MutateBettingPhaseMutation(probability=min(0.10 * mult, 0.2)),  # 10% (20% aggressive)
        MutateStartingChipsMutation(probability=min(0.10 * mult, 0.2)), # 10% (20% aggressive)

        # Bidding mutations (Spades-style contract bidding for trick-taking games)
        AddBiddingPhaseMutation(probability=min(0.05 * mult, 0.10)),    # 5% (10% aggressive)
        RemoveBiddingPhaseMutation(probability=min(0.05 * mult, 0.10)), # 5% (10% aggressive)

        # Tableau mode mutations (low weight - significant structural changes)
        MutateTableauModeMutation(probability=min(0.05 * mult, 0.10)),       # 5% (10% aggressive)
        MutateSequenceDirectionMutation(probability=min(0.03 * mult, 0.06)), # 3% (6% aggressive)
        MutateTableauVisibilityMutation(probability=min(0.03 * mult, 0.06)), # 3% (6% aggressive)

        # Self-describing genome mutations (card scoring, hand patterns, card values)
        AddCardScoringMutation(probability=min(0.05 * mult, 0.10)),          # 5% (10% aggressive)
        MutateCardScoringMutation(probability=min(0.10 * mult, 0.20)),       # 10% (20% aggressive)
        RemoveCardScoringMutation(probability=min(0.03 * mult, 0.06)),       # 3% (6% aggressive)
        MutateHandPatternMutation(probability=min(0.05 * mult, 0.10)),       # 5% (10% aggressive)
        MutateCardValueMutation(probability=min(0.05 * mult, 0.10)),         # 5% (10% aggressive)

        # Team play mutations (low weight - significant structural changes)
        EnableTeamModeMutation(probability=min(0.03 * mult, 0.06)),          # 3% (6% aggressive)
        DisableTeamModeMutation(probability=min(0.03 * mult, 0.06)),         # 3% (6% aggressive)
        MutateTeamAssignmentMutation(probability=min(0.05 * mult, 0.10)),    # 5% (10% aggressive)

        # Coherence repair mutations (high probability - only change when needed)
        CleanupOrphanedResourcesMutation(probability=0.50),                   # 50% (always)
    ]
    return MutationPipeline(operators)


def create_aggressive_pipeline(preserve_player_count: bool = False) -> MutationPipeline:
    """Create aggressive mutation pipeline for escaping local optima.

    Args:
        preserve_player_count: If True, don't mutate player_count (for filtered evolution)

    Returns:
        MutationPipeline with doubled mutation rates
    """
    return create_default_pipeline(aggressive=True, preserve_player_count=preserve_player_count)
