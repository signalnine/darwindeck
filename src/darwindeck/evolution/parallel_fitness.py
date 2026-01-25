"""Parallel fitness evaluation using multiprocessing.

This module provides parallel genome evaluation at the Python level,
complementing the Go-level parallel simulation (from Task 1).

The two-level parallelization strategy:
    1. Go level: Each genome's simulations run in parallel
    2. Python level: Multiple genomes evaluated in parallel

Key implementation notes:
    - Uses 'spawn' context (fork-unsafe with Go runtime)
    - maxtasksperchild=50 forces worker recycling to prevent state accumulation
    - imap_unordered with chunking for better throughput
"""

import gc
import os
import time
import multiprocessing as mp
from multiprocessing.pool import Pool
from concurrent.futures import ProcessPoolExecutor, as_completed
from typing import List, Optional, Callable
from dataclasses import dataclass

# Try 'fork' context first - while technically unsafe with Go, it may work
# better than 'spawn' which has semaphore leak issues in Python 3.13
# Fall back to 'spawn' if fork causes issues
try:
    _mp_context = mp.get_context('fork')
except ValueError:
    _mp_context = mp.get_context('spawn')

# Worker recycling: restart workers after N tasks to prevent CGo state accumulation
# Testing showed this helps with stability when using spawn context
_MAX_TASKS_PER_CHILD = 50

from darwindeck.genome.schema import GameGenome
from darwindeck.genome.validator import GenomeValidator
from darwindeck.evolution.coherence import SemanticCoherenceChecker
from darwindeck.evolution.fitness_full import FitnessMetrics, FitnessEvaluator, SimulationResults
from darwindeck.simulation.go_simulator import GoSimulator


# Top-level factory functions for pickling with 'spawn' multiprocessing context.
# NOTE: Using partial() causes hangs with Python 3.13 spawn context + CGo.
# Instead, we define explicit factory functions for each style.
def _create_evaluator(style: str = 'balanced') -> FitnessEvaluator:
    """Create a FitnessEvaluator instance with optional style."""
    return FitnessEvaluator(style=style)


def _create_evaluator_balanced() -> FitnessEvaluator:
    """Create a balanced FitnessEvaluator."""
    return FitnessEvaluator(style='balanced')


def _create_evaluator_bluffing() -> FitnessEvaluator:
    """Create a bluffing FitnessEvaluator."""
    return FitnessEvaluator(style='bluffing')


def _create_evaluator_strategic() -> FitnessEvaluator:
    """Create a strategic FitnessEvaluator."""
    return FitnessEvaluator(style='strategic')


def _create_evaluator_party() -> FitnessEvaluator:
    """Create a party FitnessEvaluator."""
    return FitnessEvaluator(style='party')


def _create_evaluator_trick_taking() -> FitnessEvaluator:
    """Create a trick-taking FitnessEvaluator."""
    return FitnessEvaluator(style='trick-taking')


def get_evaluator_factory(style: str) -> Callable[[], FitnessEvaluator]:
    """Get the factory function for a given fitness style.

    Using explicit factory functions instead of partial() avoids hangs
    with Python 3.13 spawn context + CGo.
    """
    factories = {
        'balanced': _create_evaluator_balanced,
        'bluffing': _create_evaluator_bluffing,
        'strategic': _create_evaluator_strategic,
        'party': _create_evaluator_party,
        'trick-taking': _create_evaluator_trick_taking,
    }
    if style not in factories:
        raise ValueError(f"Unknown fitness style: {style}. Valid: {list(factories.keys())}")
    return factories[style]


def _create_simulator() -> GoSimulator:
    """Create a GoSimulator instance."""
    return GoSimulator()


@dataclass
class EvaluationTask:
    """Complete evaluation task with genome and simulation parameters."""
    genome: GameGenome
    num_simulations: int = 100
    use_mcts: bool = False


# Global instances for each worker process
_worker_evaluator: Optional[FitnessEvaluator] = None
_worker_simulator: Optional[GoSimulator] = None
_worker_coherence_checker: Optional[SemanticCoherenceChecker] = None


def _worker_init(
    evaluator_factory: Callable[[], FitnessEvaluator],
    simulator_factory: Callable[[], GoSimulator]
):
    """Initialize worker process with its own evaluator and simulator instances."""
    global _worker_evaluator, _worker_simulator, _worker_coherence_checker
    _worker_evaluator = evaluator_factory()
    _worker_simulator = simulator_factory()
    _worker_coherence_checker = SemanticCoherenceChecker()


def _evaluate_task(task: EvaluationTask) -> FitnessMetrics:
    """Evaluate a single genome task (runs in worker subprocess)."""
    global _worker_evaluator, _worker_simulator, _worker_coherence_checker
    if _worker_evaluator is None:
        raise RuntimeError("Worker evaluator not initialized")
    if _worker_simulator is None:
        raise RuntimeError("Worker simulator not initialized")
    if _worker_coherence_checker is None:
        raise RuntimeError("Worker coherence checker not initialized")

    # STRUCTURAL VALIDATION: Check genome is valid before expensive simulation
    validation_errors = GenomeValidator.validate(task.genome)
    if validation_errors:
        return FitnessMetrics(
            decision_density=0.0,
            comeback_potential=0.0,
            tension_curve=0.0,
            interaction_frequency=0.0,
            rules_complexity=0.0,
            session_length=0.0,
            skill_vs_luck=0.0,
            bluffing_depth=0.0,
            betting_engagement=0.0,
            total_fitness=0.0,
            games_simulated=0,
            valid=False,
        )

    # SEMANTIC COHERENCE: Check genome is semantically coherent to avoid hangs
    # Incoherent genomes (e.g., chips but no betting phase) can cause infinite loops
    coherence_result = _worker_coherence_checker.check(task.genome)
    if not coherence_result.coherent:
        return FitnessMetrics(
            decision_density=0.0,
            comeback_potential=0.0,
            tension_curve=0.0,
            interaction_frequency=0.0,
            rules_complexity=0.0,
            session_length=0.0,
            skill_vs_luck=0.0,
            bluffing_depth=0.0,
            betting_engagement=0.0,
            total_fitness=0.0,
            games_simulated=0,
            valid=False,
        )

    # Run simulations using Go engine
    results = _worker_simulator.simulate(
        task.genome,
        num_games=task.num_simulations,
        use_mcts=task.use_mcts
    )

    return _worker_evaluator.evaluate(
        task.genome,
        results,
        use_mcts=task.use_mcts
    )


def _evaluate_indexed_task(indexed_task: tuple[int, EvaluationTask]) -> tuple[int, FitnessMetrics]:
    """Evaluate a single indexed genome task (runs in worker subprocess).

    Returns (index, result) to allow imap_unordered to maintain ordering.
    """
    index, task = indexed_task
    result = _evaluate_task(task)
    return (index, result)


def evaluate_genome_standalone(
    genome: GameGenome,
    num_simulations: int,
    use_mcts: bool,
    style: str
) -> FitnessMetrics:
    """Evaluate a single genome (standalone, no worker state required)."""
    evaluator = FitnessEvaluator(style=style)
    simulator = GoSimulator()
    coherence_checker = SemanticCoherenceChecker()

    # STRUCTURAL VALIDATION
    validation_errors = GenomeValidator.validate(genome)
    if validation_errors:
        return FitnessMetrics(
            decision_density=0.0, comeback_potential=0.0, tension_curve=0.0,
            interaction_frequency=0.0, rules_complexity=0.0, session_length=0.0,
            skill_vs_luck=0.0, bluffing_depth=0.0, betting_engagement=0.0,
            total_fitness=0.0, games_simulated=0, valid=False,
        )

    # SEMANTIC COHERENCE
    coherence_result = coherence_checker.check(genome)
    if not coherence_result.coherent:
        return FitnessMetrics(
            decision_density=0.0, comeback_potential=0.0, tension_curve=0.0,
            interaction_frequency=0.0, rules_complexity=0.0, session_length=0.0,
            skill_vs_luck=0.0, bluffing_depth=0.0, betting_engagement=0.0,
            total_fitness=0.0, games_simulated=0, valid=False,
        )

    # Run simulations
    results = simulator.simulate(genome, num_games=num_simulations, use_mcts=use_mcts)
    return evaluator.evaluate(genome, results, use_mcts=use_mcts)


class ParallelFitnessEvaluator:
    """Evaluates game genomes.

    Due to Python 3.13 + CGo compatibility issues, this evaluator runs
    serially in the main process. The Go engine itself runs simulations
    in parallel internally, providing good throughput.
    """

    def __init__(
        self,
        evaluator_factory: Callable[[], FitnessEvaluator],
        simulator_factory: Optional[Callable[[], GoSimulator]] = None,
        num_workers: Optional[int] = None
    ):
        """Initialize evaluator.

        Args:
            evaluator_factory: Factory function that creates a FitnessEvaluator
            simulator_factory: Factory function that creates a GoSimulator
            num_workers: Number of worker processes (ignored - runs serially)
        """
        self.evaluator_factory = evaluator_factory
        self.simulator_factory = simulator_factory or _create_simulator
        self.num_workers = num_workers or mp.cpu_count()
        # Initialize evaluator and simulator in main process
        self._evaluator = evaluator_factory()
        self._simulator = simulator_factory() if simulator_factory else GoSimulator()
        self._coherence_checker = SemanticCoherenceChecker()

    def close(self) -> None:
        """No-op for API compatibility."""
        pass

    def evaluate_population(
        self,
        genomes: List[GameGenome],
        num_simulations: int = 100,
        use_mcts: bool = False
    ) -> List[FitnessMetrics]:
        """Evaluate multiple genomes serially.

        Due to Python 3.13 multiprocessing + CGo compatibility issues,
        evaluation runs serially. The Go engine provides internal parallelism.

        Args:
            genomes: List of game genomes to evaluate
            num_simulations: Number of simulations per genome
            use_mcts: Whether to use MCTS AI

        Returns:
            List of fitness metrics, one per genome (same order)
        """
        if not genomes:
            return []

        results: List[FitnessMetrics] = []
        for genome in genomes:
            # STRUCTURAL VALIDATION
            validation_errors = GenomeValidator.validate(genome)
            if validation_errors:
                results.append(FitnessMetrics(
                    decision_density=0.0, comeback_potential=0.0, tension_curve=0.0,
                    interaction_frequency=0.0, rules_complexity=0.0, session_length=0.0,
                    skill_vs_luck=0.0, bluffing_depth=0.0, betting_engagement=0.0,
                    total_fitness=0.0, games_simulated=0, valid=False,
                ))
                continue

            # SEMANTIC COHERENCE
            coherence_result = self._coherence_checker.check(genome)
            if not coherence_result.coherent:
                results.append(FitnessMetrics(
                    decision_density=0.0, comeback_potential=0.0, tension_curve=0.0,
                    interaction_frequency=0.0, rules_complexity=0.0, session_length=0.0,
                    skill_vs_luck=0.0, bluffing_depth=0.0, betting_engagement=0.0,
                    total_fitness=0.0, games_simulated=0, valid=False,
                ))
                continue

            # Run simulations
            sim_results = self._simulator.simulate(
                genome, num_games=num_simulations, use_mcts=use_mcts
            )
            metrics = self._evaluator.evaluate(genome, sim_results, use_mcts=use_mcts)
            results.append(metrics)

        return results
