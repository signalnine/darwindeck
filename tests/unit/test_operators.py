"""Test special effect mutation operators."""

import pytest
import random


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
