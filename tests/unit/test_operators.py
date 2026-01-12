"""Test special effect and betting mutation operators."""

import pytest
import random
from dataclasses import replace


def test_add_effect_mutation():
    """AddEffectMutation adds an effect to the genome."""
    from darwindeck.evolution.operators import AddEffectMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()
    original_count = len(genome.special_effects)

    mutation = AddEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    assert len(mutated.special_effects) == original_count + 1


def test_remove_effect_mutation():
    """RemoveEffectMutation removes an effect."""
    from darwindeck.evolution.operators import RemoveEffectMutation
    from darwindeck.genome.schema import (
        SpecialEffect, EffectType, Rank, TargetSelector,
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    # Create genome with effects
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[
            SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        ],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = RemoveEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    assert len(mutated.special_effects) == 0


def test_remove_effect_mutation_no_effects():
    """RemoveEffectMutation returns genome unchanged if no effects exist."""
    from darwindeck.evolution.operators import RemoveEffectMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()  # War has no special effects
    assert len(genome.special_effects) == 0

    mutation = RemoveEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert len(mutated.special_effects) == 0
    assert mutated.generation == genome.generation  # No mutation occurred


def test_mutate_effect_mutation():
    """MutateEffectMutation changes one field of an effect."""
    from darwindeck.evolution.operators import MutateEffectMutation
    from darwindeck.genome.schema import (
        SpecialEffect, EffectType, Rank, TargetSelector,
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    random.seed(42)  # For reproducibility

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[
            SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        ],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = MutateEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should still have one effect
    assert len(mutated.special_effects) == 1
    # Something should have changed
    original = genome.special_effects[0]
    changed = mutated.special_effects[0]
    assert (original.trigger_rank != changed.trigger_rank or
            original.effect_type != changed.effect_type or
            original.target != changed.target or
            original.value != changed.value)


def test_mutate_effect_mutation_no_effects():
    """MutateEffectMutation returns genome unchanged if no effects exist."""
    from darwindeck.evolution.operators import MutateEffectMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()  # War has no special effects
    assert len(genome.special_effects) == 0

    mutation = MutateEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert len(mutated.special_effects) == 0
    assert mutated.generation == genome.generation  # No mutation occurred


def test_add_effect_creates_valid_effect():
    """AddEffectMutation creates a valid SpecialEffect with correct types."""
    from darwindeck.evolution.operators import AddEffectMutation
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector
    from darwindeck.genome.examples import create_war_genome

    random.seed(123)
    genome = create_war_genome()

    mutation = AddEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    new_effect = mutated.special_effects[0]
    assert isinstance(new_effect, SpecialEffect)
    assert isinstance(new_effect.trigger_rank, Rank)
    assert isinstance(new_effect.effect_type, EffectType)
    assert isinstance(new_effect.target, TargetSelector)
    assert isinstance(new_effect.value, int)
    assert 1 <= new_effect.value <= 3


# =====================================================================
# Betting Mutation Tests
# =====================================================================


def test_add_betting_phase_mutation():
    """AddBettingPhaseMutation adds a BettingPhase to the genome."""
    from darwindeck.evolution.operators import AddBettingPhaseMutation
    from darwindeck.genome.schema import BettingPhase
    from darwindeck.genome.examples import create_war_genome

    random.seed(42)
    genome = create_war_genome()
    original_phase_count = len(genome.turn_structure.phases)

    mutation = AddBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should have one more phase
    assert len(mutated.turn_structure.phases) == original_phase_count + 1
    # One of them should be a BettingPhase
    betting_phases = [p for p in mutated.turn_structure.phases if isinstance(p, BettingPhase)]
    assert len(betting_phases) == 1


def test_add_betting_phase_respects_max_phases():
    """AddBettingPhaseMutation respects max 5 phases limit."""
    from darwindeck.evolution.operators import AddBettingPhaseMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, DrawPhase, Location
    )

    # Create genome with 5 phases
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[
            DrawPhase(source=Location.DECK) for _ in range(5)
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = AddBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should not add another phase
    assert len(mutated.turn_structure.phases) == 5
    assert mutated.generation == 0  # No mutation occurred


def test_remove_betting_phase_mutation():
    """RemoveBettingPhaseMutation removes a BettingPhase."""
    from darwindeck.evolution.operators import RemoveBettingPhaseMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition,
        BettingPhase, DrawPhase, Location
    )

    # Create genome with a BettingPhase
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7, starting_chips=1000),
        turn_structure=TurnStructure(phases=[
            DrawPhase(source=Location.DECK),
            BettingPhase(min_bet=10, max_raises=3),
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = RemoveBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should have one less phase
    assert len(mutated.turn_structure.phases) == 1
    # No BettingPhase should remain
    betting_phases = [p for p in mutated.turn_structure.phases if isinstance(p, BettingPhase)]
    assert len(betting_phases) == 0


def test_remove_betting_phase_mutation_no_betting():
    """RemoveBettingPhaseMutation returns unchanged if no BettingPhases."""
    from darwindeck.evolution.operators import RemoveBettingPhaseMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()  # War has no betting phases

    mutation = RemoveBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert mutated.generation == genome.generation


def test_mutate_betting_phase_mutation():
    """MutateBettingPhaseMutation modifies min_bet or max_raises."""
    from darwindeck.evolution.operators import MutateBettingPhaseMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase
    )

    random.seed(42)

    # Create genome with a BettingPhase
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7, starting_chips=1000),
        turn_structure=TurnStructure(phases=[
            BettingPhase(min_bet=10, max_raises=3),
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = MutateBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should still have one BettingPhase
    assert len(mutated.turn_structure.phases) == 1
    original = genome.turn_structure.phases[0]
    changed = mutated.turn_structure.phases[0]

    # Something should have changed
    assert (original.min_bet != changed.min_bet or
            original.max_raises != changed.max_raises)


def test_mutate_betting_phase_no_betting():
    """MutateBettingPhaseMutation returns unchanged if no BettingPhases."""
    from darwindeck.evolution.operators import MutateBettingPhaseMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()

    mutation = MutateBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert mutated.generation == genome.generation


def test_mutate_starting_chips_mutation():
    """MutateStartingChipsMutation modifies starting_chips."""
    from darwindeck.evolution.operators import MutateStartingChipsMutation
    from darwindeck.genome.examples import create_war_genome

    random.seed(42)
    genome = create_war_genome()
    # War has starting_chips=0 by default
    assert genome.setup.starting_chips == 0

    mutation = MutateStartingChipsMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should now have starting chips
    assert mutated.setup.starting_chips > 0


def test_mutate_starting_chips_existing():
    """MutateStartingChipsMutation mutates existing chips by +-50%."""
    from darwindeck.evolution.operators import MutateStartingChipsMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    random.seed(42)

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7, starting_chips=1000),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = MutateStartingChipsMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should have different starting chips
    assert mutated.setup.starting_chips != 1000
    # But within +-50% range (plus min of 10)
    assert 10 <= mutated.setup.starting_chips <= 1500


def test_betting_constraint_min_bet_le_starting_chips():
    """min_bet <= starting_chips is maintained after mutation."""
    from darwindeck.evolution.operators import MutateStartingChipsMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase
    )

    # Set seed to get a specific lower starting_chips value
    random.seed(5)

    # Create genome with high min_bet relative to chips
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7, starting_chips=100),
        turn_structure=TurnStructure(phases=[
            BettingPhase(min_bet=80, max_raises=3),
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = MutateStartingChipsMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Verify constraint: min_bet <= starting_chips
    betting_phase = mutated.turn_structure.phases[0]
    assert betting_phase.min_bet <= mutated.setup.starting_chips


def test_add_betting_phase_min_bet_valid():
    """AddBettingPhaseMutation creates valid min_bet <= starting_chips."""
    from darwindeck.evolution.operators import AddBettingPhaseMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase
    )

    random.seed(42)

    # Create genome with low starting chips
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7, starting_chips=5),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = AddBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Verify min_bet <= starting_chips
    betting_phase = next(
        p for p in mutated.turn_structure.phases if isinstance(p, BettingPhase)
    )
    # starting_chips is 5, so min_bet should be at most 5
    assert betting_phase.min_bet <= genome.setup.starting_chips or betting_phase.min_bet == 1
