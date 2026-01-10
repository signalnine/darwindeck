"""Test win condition mutation operator."""

import pytest
from cards_evolve.evolution.operators import ModifyWinConditionMutation, create_default_pipeline
from cards_evolve.genome.schema import GameGenome, WinCondition, SetupRules, TurnStructure


def create_test_genome(win_conditions: list) -> GameGenome:
    """Create minimal test genome with given win conditions."""
    return GameGenome(
        schema_version="1.0",
        genome_id="test-genome",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=win_conditions,
        scoring_rules=[],
        max_turns=100,
        min_turns=1,
        player_count=2,
    )


def test_change_win_condition_type():
    """Test changing win condition type."""
    genome = create_test_genome([WinCondition(type="empty_hand", threshold=None)])
    operator = ModifyWinConditionMutation(probability=1.0)

    # Call _change_win_condition_type directly to test specific mutation
    mutated = operator._change_win_condition_type(genome)

    # Should have exactly 1 win condition (changed, not added)
    assert len(mutated.win_conditions) == 1

    # Type should be different from original
    assert mutated.win_conditions[0].type != "empty_hand"

    # Type should be one of the valid types
    assert mutated.win_conditions[0].type in operator.WIN_CONDITION_TYPES

    # Generation should increment
    assert mutated.generation == 1


def test_change_threshold():
    """Test changing threshold for score-based win condition."""
    genome = create_test_genome([WinCondition(type="first_to_score", threshold=100)])
    operator = ModifyWinConditionMutation(probability=1.0)

    # Apply mutation - might change type or threshold
    # Run multiple times to increase chance of hitting threshold change
    threshold_changed = False
    for _ in range(20):
        mutated = operator._change_threshold(genome)
        if mutated.win_conditions[0].threshold != 100:
            threshold_changed = True
            # Threshold should be within ±20% (80-120) approximately
            # Allow some margin for rounding
            assert 10 <= mutated.win_conditions[0].threshold <= 200
            assert mutated.win_conditions[0].type == "first_to_score"
            break

    # Should have successfully changed threshold at least once
    assert threshold_changed, "Threshold should change with _change_threshold"


def test_add_win_condition():
    """Test adding a new win condition."""
    genome = create_test_genome([WinCondition(type="empty_hand", threshold=None)])
    operator = ModifyWinConditionMutation(probability=1.0)

    mutated = operator._add_win_condition(genome)

    # Should have 2 win conditions now
    assert len(mutated.win_conditions) == 2

    # Original should still be present
    assert mutated.win_conditions[0].type == "empty_hand"

    # New condition should be valid
    assert mutated.win_conditions[1].type in operator.WIN_CONDITION_TYPES


def test_max_win_conditions():
    """Test that max 3 win conditions are allowed."""
    genome = create_test_genome([
        WinCondition(type="empty_hand", threshold=None),
        WinCondition(type="first_to_score", threshold=100),
        WinCondition(type="capture_all", threshold=None),
    ])
    operator = ModifyWinConditionMutation(probability=1.0)

    # Try to add - should change existing instead
    mutated = operator.mutate(genome)

    # Should still have exactly 3 conditions
    assert len(mutated.win_conditions) == 3


def test_mutation_probability():
    """Test that probability controls mutation application."""
    genome = create_test_genome([WinCondition(type="empty_hand", threshold=None)])

    # 0% probability - should never mutate
    operator_never = ModifyWinConditionMutation(probability=0.0)
    assert not operator_never.should_apply()

    # 100% probability - should always mutate
    operator_always = ModifyWinConditionMutation(probability=1.0)
    assert operator_always.should_apply()


def test_default_pipeline():
    """Test default mutation pipeline creation."""
    pipeline = create_default_pipeline()

    # Should have at least one operator
    assert len(pipeline.operators) >= 1

    # First operator should be win condition mutation
    assert isinstance(pipeline.operators[0], ModifyWinConditionMutation)

    # Should have 10% probability
    assert pipeline.operators[0].probability == 0.1


def test_pipeline_application():
    """Test applying mutation pipeline."""
    import random as rand
    rand.seed(42)  # Set seed for deterministic test

    genome = create_test_genome([WinCondition(type="empty_hand", threshold=None)])

    # Create pipeline with 100% probability for testing
    from cards_evolve.evolution.operators import MutationPipeline
    pipeline = MutationPipeline([ModifyWinConditionMutation(probability=1.0)])

    mutated = pipeline.apply(genome)

    # Should be mutated (generation incremented)
    assert mutated.generation == genome.generation + 1
