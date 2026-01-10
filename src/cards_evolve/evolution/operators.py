"""Genetic operators for game genome evolution (Phase 4)."""

from __future__ import annotations

import random
from abc import ABC, abstractmethod
from typing import List, Optional
from dataclasses import replace
from cards_evolve.genome.schema import (
    GameGenome, WinCondition, SetupRules, TurnStructure,
    PlayPhase, DrawPhase, DiscardPhase, Location
)
from cards_evolve.genome.conditions import Condition, ConditionType, Operator


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

    def __init__(self, probability: float = 0.15):
        """Initialize parameter tweaking mutation.

        Args:
            probability: Mutation probability (default: 15%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Tweak a random numeric parameter.

        Args:
            genome: Genome to mutate

        Returns:
            New genome with tweaked parameter
        """
        choice = random.choice([
            'cards_per_player',
            'max_turns',
            'initial_discard_count'
        ])

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
        new_phase = random.choice([
            DrawPhase(
                source=random.choice([Location.DECK, Location.DISCARD]),
                count=1,
                mandatory=random.choice([True, False])
            ),
            PlayPhase(
                target=Location.DISCARD,
                valid_play_condition=Condition(
                    type=ConditionType.HAND_SIZE,
                    operator=Operator.GT,
                    value=0
                ),
                min_cards=1,
                max_cards=1,
                mandatory=True
            ),
            DiscardPhase(
                target=Location.DISCARD,
                count=1,
                mandatory=False
            )
        ])

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

    def _tweak_condition(self, condition: Optional[Condition]) -> Optional[Condition]:
        """Tweak a condition's parameters.

        Args:
            condition: Condition to tweak

        Returns:
            Modified condition
        """
        if condition is None:
            return None

        # Tweak value by ±2
        if condition.value is not None:
            new_value = max(0, condition.value + random.randint(-2, 2))
            return replace(condition, value=new_value)

        # Or change operator
        operators = [Operator.EQ, Operator.GT, Operator.LT, Operator.GE, Operator.LE]
        new_operator = random.choice([op for op in operators if op != condition.operator])
        return replace(condition, operator=new_operator)


class AddSpecialEffectMutation(MutationOperator):
    """Add a special effect to the genome.

    NOTE: Currently a no-op placeholder since SpecialEffect schema
    is not yet fully implemented. Will be activated when special effects
    are properly defined in the schema.
    """

    def __init__(self, probability: float = 0.05):
        """Initialize special effect addition mutation.

        Args:
            probability: Mutation probability (default: 5%)
        """
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Add a new special effect (currently no-op).

        Args:
            genome: Genome to mutate

        Returns:
            Unchanged genome (special effects not yet implemented)
        """
        # TODO: Implement when SpecialEffect schema is complete
        # For now, return genome unchanged
        return genome


class ModifyWinConditionMutation(MutationOperator):
    """Modify or add win conditions.

    Supports three mutation types:
    1. Change win condition type (empty_hand, high_score, etc.)
    2. Change threshold (±20% for score-based conditions)
    3. Add new win condition (up to 3 total)
    """

    WIN_CONDITION_TYPES = ["empty_hand", "high_score", "first_to_score", "capture_all"]

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
        if new_type in ["first_to_score", "high_score"]:
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
            if wc.type in ["first_to_score", "high_score"] and wc.threshold is not None
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

        Args:
            genome: Source genome

        Returns:
            New genome with additional win condition
        """
        # Choose random type
        new_type = random.choice(self.WIN_CONDITION_TYPES)

        # Set threshold if needed
        if new_type in ["first_to_score", "high_score"]:
            new_threshold = random.choice([50, 100, 200, 500])
        else:
            new_threshold = None

        # Create new win condition
        new_wc = WinCondition(type=new_type, threshold=new_threshold)

        # Add to existing list
        new_win_conditions = list(genome.win_conditions) + [new_wc]

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

        # Create offspring genomes
        # Inherit from parent1
        offspring1 = replace(
            parent1,
            turn_structure=replace(parent1.turn_structure, phases=tuple(offspring1_phases)),
            generation=parent1.generation + 1,
            genome_id=f"{parent1.genome_id}-x-{parent2.genome_id}"
        )

        # Inherit from parent2
        offspring2 = replace(
            parent2,
            turn_structure=replace(parent2.turn_structure, phases=tuple(offspring2_phases)),
            generation=parent2.generation + 1,
            genome_id=f"{parent2.genome_id}-x-{parent1.genome_id}"
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


def create_default_pipeline() -> MutationPipeline:
    """Create default mutation pipeline with standard operators.

    Returns:
        MutationPipeline with all 7 mutation operators
    """
    operators = [
        TweakParameterMutation(probability=0.15),      # 15% - common tweaks
        SwapPhaseOrderMutation(probability=0.10),      # 10% - reorder phases
        AddPhaseMutation(probability=0.05),            # 5% - add phases
        RemovePhaseMutation(probability=0.05),         # 5% - remove phases
        ModifyConditionMutation(probability=0.10),     # 10% - tweak conditions
        AddSpecialEffectMutation(probability=0.05),    # 5% - add effects
        ModifyWinConditionMutation(probability=0.10),  # 10% - change win conditions
    ]
    return MutationPipeline(operators)
