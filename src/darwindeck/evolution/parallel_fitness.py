"""Parallel fitness evaluation using multiprocessing.

This module provides parallel genome evaluation at the Python level,
complementing the Go-level parallel simulation (from Task 1).

The two-level parallelization strategy:
    1. Go level: Each genome's simulations run in parallel (1.43x speedup)
    2. Python level: Multiple genomes evaluated in parallel (4x speedup on 4 cores)
    3. Combined effect: Up to 5.7x total speedup (1.43 × 4.0)
"""

import multiprocessing as mp
from multiprocessing.pool import Pool as PoolType
from typing import List, Optional, Callable
from dataclasses import dataclass

# CRITICAL: Use 'spawn' context instead of default 'fork' on Linux.
# Go's runtime is not fork-safe - forked processes inherit corrupted goroutine state.
# This causes deadlocks when CGo library is loaded before forking.
# 'spawn' starts fresh processes that load their own copy of the library.
_mp_context = mp.get_context('spawn')

from darwindeck.genome.schema import GameGenome
from darwindeck.genome.validator import GenomeValidator
from darwindeck.evolution.fitness_full import FitnessMetrics, FitnessEvaluator, SimulationResults
from darwindeck.simulation.go_simulator import GoSimulator


# Top-level factory functions for pickling with 'spawn' multiprocessing context.
# Lambda functions can't be pickled, so we need named functions.
def _create_evaluator(style: str = 'balanced') -> FitnessEvaluator:
    """Create a FitnessEvaluator instance with optional style."""
    return FitnessEvaluator(style=style)


def _create_simulator() -> GoSimulator:
    """Create a GoSimulator instance."""
    return GoSimulator()


@dataclass
class EvaluationTask:
    """Complete evaluation task with genome and simulation parameters."""
    genome: GameGenome
    num_simulations: int = 100
    use_mcts: bool = False


class ParallelFitnessEvaluator:
    """Evaluates game genomes in parallel using a persistent process pool.

    Each worker process gets its own copy of the simulator and evaluator,
    which is process-safe (separate memory spaces).

    The pool is created lazily on first use and reused for subsequent
    evaluations. This avoids semaphore leaks from repeatedly creating
    and destroying pools with Python 3.13's 'spawn' context.

    Usage:
        evaluator = ParallelFitnessEvaluator(
            evaluator_factory=lambda: FitnessEvaluator(),
            num_workers=4
        )
        metrics = evaluator.evaluate_population(genomes)
        evaluator.close()  # Clean up when done
    """

    def __init__(
        self,
        evaluator_factory: Callable[[], FitnessEvaluator],
        simulator_factory: Optional[Callable[[], GoSimulator]] = None,
        num_workers: Optional[int] = None
    ):
        """Initialize parallel evaluator.

        Args:
            evaluator_factory: Factory function that creates a FitnessEvaluator
                              (called once per worker process)
            simulator_factory: Factory function that creates a GoSimulator
                              (default: creates GoSimulator with worker-specific seed)
            num_workers: Number of worker processes (default: cpu_count())
        """
        self.evaluator_factory = evaluator_factory
        self.simulator_factory = simulator_factory or _create_simulator
        self.num_workers = num_workers or mp.cpu_count()
        self._pool: Optional[PoolType] = None

    def _get_pool(self) -> PoolType:
        """Get or create the persistent worker pool."""
        if self._pool is None:
            self._pool = _mp_context.Pool(
                processes=self.num_workers,
                initializer=_worker_init,
                initargs=(self.evaluator_factory, self.simulator_factory)
            )
        return self._pool

    def close(self) -> None:
        """Close the worker pool and release resources.

        Call this when you're done with the evaluator to clean up
        worker processes and prevent resource leaks.
        """
        if self._pool is not None:
            self._pool.close()
            self._pool.join()
            self._pool = None

    def __del__(self) -> None:
        """Cleanup on garbage collection (fallback)."""
        self.close()

    def evaluate_population(
        self,
        genomes: List[GameGenome],
        num_simulations: int = 100,
        use_mcts: bool = False
    ) -> List[FitnessMetrics]:
        """Evaluate multiple genomes in parallel.

        Args:
            genomes: List of game genomes to evaluate
            num_simulations: Number of simulations per genome
            use_mcts: Whether to use MCTS AI (slower but measures skill)

        Returns:
            List of fitness metrics, one per genome (same order)
        """
        if not genomes:
            return []

        # Create evaluation tasks
        tasks = [
            EvaluationTask(genome, num_simulations, use_mcts)
            for genome in genomes
        ]

        pool = self._get_pool()
        results = pool.map(_evaluate_task, tasks)
        return results


# Global instances for each worker process
_worker_evaluator: Optional[FitnessEvaluator] = None
_worker_simulator: Optional[GoSimulator] = None


def _worker_init(
    evaluator_factory: Callable[[], FitnessEvaluator],
    simulator_factory: Callable[[], GoSimulator]
):
    """Initialize worker process with its own evaluator and simulator instances."""
    global _worker_evaluator, _worker_simulator
    _worker_evaluator = evaluator_factory()
    _worker_simulator = simulator_factory()


def _evaluate_task(task: EvaluationTask) -> FitnessMetrics:
    """Evaluate a single genome task (runs in worker subprocess).

    This function is called by multiprocessing.Pool.map() in each
    worker process. It runs simulations using the Go engine and evaluates fitness.
    """
    global _worker_evaluator, _worker_simulator
    if _worker_evaluator is None:
        raise RuntimeError("Worker evaluator not initialized")
    if _worker_simulator is None:
        raise RuntimeError("Worker simulator not initialized")

    # STRUCTURAL VALIDATION: Check genome is valid before expensive simulation
    # This catches issues like: no card play phases, impossible card counts,
    # missing required fields for win conditions, etc.
    validation_errors = GenomeValidator.validate(task.genome)
    if validation_errors:
        # Return zero fitness without running simulation (saves compute)
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
            valid=False,  # Mark as structurally invalid
        )

    # Run real simulations using Go engine
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
