#!/usr/bin/env python3
"""Demonstration of parallel fitness evaluation.

This example shows how to use the ParallelFitnessEvaluator to evaluate
multiple game genomes concurrently, achieving significant speedup on
multi-core systems.

Usage:
    poetry run python examples/parallel_fitness_demo.py
"""

import time
from typing import List
from darwindeck.genome.schema import GameGenome
from darwindeck.genome.examples import create_war_genome, get_seed_genomes
from darwindeck.evolution.fitness_full import FitnessEvaluator, FitnessMetrics
from darwindeck.evolution.parallel_fitness import ParallelFitnessEvaluator


def create_evaluator() -> FitnessEvaluator:
    """Factory function to create fitness evaluator instances.

    This is called once per worker process to create isolated evaluators.
    """
    return FitnessEvaluator(use_cache=False)


def create_population(size: int) -> List[GameGenome]:
    """Create a test population of diverse genomes.

    Args:
        size: Number of genomes to create

    Returns:
        List of game genomes with unique IDs
    """
    population = []
    war = create_war_genome()

    for i in range(size):
        # Create variants with different IDs
        variant = GameGenome(
            schema_version=war.schema_version,
            genome_id=f"genome-{i:03d}",
            generation=0,
            setup=war.setup,
            turn_structure=war.turn_structure,
            special_effects=war.special_effects,
            win_conditions=war.win_conditions,
            scoring_rules=war.scoring_rules,
            max_turns=war.max_turns,
            player_count=war.player_count
        )
        population.append(variant)

    return population


def demonstrate_serial_vs_parallel():
    """Compare serial vs parallel evaluation performance."""
    print("=== Parallel Fitness Evaluation Demo ===\n")

    # Create a population
    population_size = 50
    num_simulations = 100
    print(f"Creating population of {population_size} genomes...")
    population = create_population(population_size)
    print(f"Each genome will run {num_simulations} simulations\n")

    # Serial evaluation (1 worker)
    print("Running SERIAL evaluation (1 worker)...")
    serial_evaluator = ParallelFitnessEvaluator(
        evaluator_factory=create_evaluator,
        num_workers=1
    )
    start = time.time()
    serial_results = serial_evaluator.evaluate_population(
        population,
        num_simulations=num_simulations
    )
    serial_time = time.time() - start
    print(f"  Completed in {serial_time:.2f}s")
    print(f"  Average fitness: {sum(r.total_fitness for r in serial_results) / len(serial_results):.3f}\n")

    # Parallel evaluation (4 workers)
    print("Running PARALLEL evaluation (4 workers)...")
    parallel_evaluator = ParallelFitnessEvaluator(
        evaluator_factory=create_evaluator,
        num_workers=4
    )
    start = time.time()
    parallel_results = parallel_evaluator.evaluate_population(
        population,
        num_simulations=num_simulations
    )
    parallel_time = time.time() - start
    print(f"  Completed in {parallel_time:.2f}s")
    print(f"  Average fitness: {sum(r.total_fitness for r in parallel_results) / len(parallel_results):.3f}\n")

    # Calculate speedup
    speedup = serial_time / parallel_time
    print(f"=== Results ===")
    print(f"Serial time:   {serial_time:.2f}s")
    print(f"Parallel time: {parallel_time:.2f}s")
    print(f"Speedup:       {speedup:.2f}x")
    print(f"\nNote: Speedup depends on:")
    print("  - Number of CPU cores available")
    print("  - Overhead of multiprocessing (process creation, serialization)")
    print("  - Complexity of fitness evaluation (more complex = better speedup)")
    print("  - With real Go simulations, expect 4x+ speedup on 4+ core systems")


def demonstrate_usage():
    """Show basic usage patterns."""
    print("\n=== Basic Usage Example ===\n")

    # Get seed genomes
    population = create_population(10)

    # Create evaluator with custom worker count
    evaluator = ParallelFitnessEvaluator(
        evaluator_factory=create_evaluator,
        num_workers=2  # Use 2 workers
    )

    # Evaluate population
    print("Evaluating 10 genomes with 2 workers...")
    results = evaluator.evaluate_population(
        population,
        num_simulations=50,
        use_mcts=False
    )

    # Display results
    print(f"\nEvaluated {len(results)} genomes:")
    for i, (genome, metrics) in enumerate(zip(population, results)):
        print(f"  {genome.genome_id}:")
        print(f"    Total fitness: {metrics.total_fitness:.3f}")
        print(f"    Decision density: {metrics.decision_density:.3f}")
        print(f"    Comeback potential: {metrics.comeback_potential:.3f}")
        print(f"    Valid: {metrics.valid}")


def main():
    """Run all demonstrations."""
    demonstrate_serial_vs_parallel()
    demonstrate_usage()

    print("\n=== Integration Notes ===")
    print("In production genetic algorithm:")
    print("  1. Use ParallelFitnessEvaluator to evaluate entire populations")
    print("  2. Set num_workers = cpu_count() for maximum throughput")
    print("  3. Each worker gets isolated FitnessEvaluator and simulator")
    print("  4. Process-safe: no shared state, no race conditions")
    print("  5. Combined with Go-level parallelism for 5.7x+ total speedup")


if __name__ == "__main__":
    main()
