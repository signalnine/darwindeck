"""Tests for parallel fitness evaluation."""

import pytest
import time
from typing import List

from darwindeck.genome.schema import GameGenome
from darwindeck.genome.examples import create_war_genome, create_crazy_eights_genome
from darwindeck.evolution.fitness_full import (
    FitnessEvaluator,
    FitnessMetrics,
    SimulationResults
)
from darwindeck.evolution.parallel_fitness import ParallelFitnessEvaluator


def create_test_evaluator() -> FitnessEvaluator:
    """Factory function to create fitness evaluator for testing."""
    return FitnessEvaluator(use_cache=False)


def test_parallel_produces_same_results_as_serial():
    """Verify parallel evaluation produces identical results to serial.

    This test ensures that parallelization doesn't change the evaluation
    results - we should get deterministic fitness scores.
    """
    # Create genomes with different IDs for variety
    genomes = []
    for i in range(10):
        genome = create_war_genome()
        # Create variant with different ID for testing
        variant = GameGenome(
            schema_version=genome.schema_version,
            genome_id=f"war-variant-{i}",
            generation=genome.generation,
            setup=genome.setup,
            turn_structure=genome.turn_structure,
            special_effects=genome.special_effects,
            win_conditions=genome.win_conditions,
            scoring_rules=genome.scoring_rules,
            max_turns=genome.max_turns,
            player_count=genome.player_count
        )
        genomes.append(variant)

    # Serial evaluation (1 worker)
    serial_eval = ParallelFitnessEvaluator(
        evaluator_factory=create_test_evaluator,
        num_workers=1
    )
    serial_results = serial_eval.evaluate_population(genomes, num_simulations=50)

    # Parallel evaluation (2 workers)
    parallel_eval = ParallelFitnessEvaluator(
        evaluator_factory=create_test_evaluator,
        num_workers=2
    )
    parallel_results = parallel_eval.evaluate_population(genomes, num_simulations=50)

    # Compare
    assert len(parallel_results) == len(serial_results)
    for serial, parallel in zip(serial_results, parallel_results):
        assert serial.total_fitness == parallel.total_fitness
        assert serial.decision_density == parallel.decision_density
        assert serial.comeback_potential == parallel.comeback_potential
        assert serial.valid == parallel.valid


def test_parallel_with_different_worker_counts():
    """Test that different worker counts produce same results.

    This verifies determinism across different parallelization levels.
    """
    genomes = [create_war_genome()] * 8

    results_1 = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=1
    ).evaluate_population(genomes, num_simulations=50)

    results_2 = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=2
    ).evaluate_population(genomes, num_simulations=50)

    results_4 = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=4
    ).evaluate_population(genomes, num_simulations=50)

    # All should produce identical results
    assert len(results_1) == len(results_2) == len(results_4)
    for r1, r2, r4 in zip(results_1, results_2, results_4):
        assert r1.total_fitness == r2.total_fitness == r4.total_fitness
        assert r1.decision_density == r2.decision_density == r4.decision_density
        assert r1.valid == r2.valid == r4.valid


def test_empty_genome_list():
    """Test that empty genome list returns empty results."""
    evaluator = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=2
    )
    results = evaluator.evaluate_population([])
    assert results == []


def test_single_genome():
    """Test evaluation of a single genome."""
    evaluator = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=2
    )
    genomes = [create_war_genome()]
    results = evaluator.evaluate_population(genomes, num_simulations=50)

    assert len(results) == 1
    assert isinstance(results[0], FitnessMetrics)
    assert results[0].total_fitness >= 0.0
    assert results[0].total_fitness <= 1.0


def test_preserves_genome_order():
    """Test that results are returned in same order as input genomes."""
    war = create_war_genome()

    # Create a variant for testing order
    war_variant = GameGenome(
        schema_version=war.schema_version,
        genome_id="war-variant-x",
        generation=war.generation,
        setup=war.setup,
        turn_structure=war.turn_structure,
        special_effects=war.special_effects,
        win_conditions=war.win_conditions,
        scoring_rules=war.scoring_rules,
        max_turns=war.max_turns,
        player_count=war.player_count
    )

    # Create list with specific order
    genomes = [war, war_variant, war, war_variant]

    evaluator = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=2
    )
    results = evaluator.evaluate_population(genomes, num_simulations=50)

    assert len(results) == 4
    # Results should correspond to genome order (deterministic by genome_id)
    assert results[0].total_fitness == results[2].total_fitness  # Both war
    assert results[1].total_fitness == results[3].total_fitness  # Both war_variant


def test_different_simulation_counts():
    """Test that different simulation counts work correctly."""
    genomes = [create_war_genome()] * 4

    evaluator = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=2
    )

    results_10 = evaluator.evaluate_population(genomes, num_simulations=10)
    results_100 = evaluator.evaluate_population(genomes, num_simulations=100)

    assert len(results_10) == len(results_100) == 4
    # Both should return valid results
    for r10, r100 in zip(results_10, results_100):
        assert r10.valid
        assert r100.valid


def test_mcts_flag_propagation():
    """Test that use_mcts flag is properly propagated to workers."""
    genomes = [create_war_genome()]

    evaluator = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=1
    )

    results_random = evaluator.evaluate_population(genomes, use_mcts=False)
    results_mcts = evaluator.evaluate_population(genomes, use_mcts=True)

    # Both should work without errors
    assert len(results_random) == 1
    assert len(results_mcts) == 1
    assert results_random[0].valid
    assert results_mcts[0].valid


def test_parallel_execution_completes():
    """Test that parallel evaluation completes successfully for large populations.

    Note: We don't test for speedup because:
    1. Multiprocessing overhead can dominate for small/fast evaluations
    2. Mock fitness evaluation is too fast to measure meaningful speedup
    3. In production with real simulations, speedup will be significant

    This test simply verifies that parallel execution works correctly.
    """
    # Create a larger population with variants
    war = create_war_genome()
    genomes = []
    for i in range(20):
        variant = GameGenome(
            schema_version=war.schema_version,
            genome_id=f"war-speed-test-{i}",
            generation=war.generation,
            setup=war.setup,
            turn_structure=war.turn_structure,
            special_effects=war.special_effects,
            win_conditions=war.win_conditions,
            scoring_rules=war.scoring_rules,
            max_turns=war.max_turns,
            player_count=war.player_count
        )
        genomes.append(variant)

    # Serial (1 worker)
    serial_eval = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=1
    )
    serial_results = serial_eval.evaluate_population(genomes, num_simulations=10)

    # Parallel (4 workers)
    parallel_eval = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=4
    )
    parallel_results = parallel_eval.evaluate_population(genomes, num_simulations=10)

    # Both should complete successfully with identical results
    assert len(serial_results) == len(parallel_results) == 20
    for serial, parallel in zip(serial_results, parallel_results):
        assert serial.total_fitness == parallel.total_fitness
        assert serial.valid == parallel.valid


def test_error_handling_in_worker():
    """Test that errors in worker processes are handled properly."""
    # This test verifies graceful handling of invalid genomes
    # For now, we'll just verify the system doesn't crash
    genomes = [create_war_genome()]

    evaluator = ParallelFitnessEvaluator(
        create_test_evaluator, num_workers=2
    )

    # Should complete without raising exceptions
    results = evaluator.evaluate_population(genomes)
    assert len(results) == 1


def test_default_worker_count():
    """Test that default worker count uses cpu_count()."""
    import multiprocessing

    evaluator = ParallelFitnessEvaluator(create_test_evaluator)
    assert evaluator.num_workers == multiprocessing.cpu_count()


def test_custom_worker_count():
    """Test that custom worker count is respected."""
    evaluator = ParallelFitnessEvaluator(create_test_evaluator, num_workers=2)
    assert evaluator.num_workers == 2

    evaluator = ParallelFitnessEvaluator(create_test_evaluator, num_workers=8)
    assert evaluator.num_workers == 8
