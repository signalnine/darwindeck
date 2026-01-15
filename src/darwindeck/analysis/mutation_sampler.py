"""Mutation path sampling with fitness tracking."""

from __future__ import annotations

import random
import multiprocessing as mp
from dataclasses import dataclass, field
from typing import Callable, Optional

from darwindeck.genome.schema import GameGenome
from darwindeck.evolution.operators import create_default_pipeline, MutationPipeline
from darwindeck.evolution.fitness_full import FitnessEvaluator, FitnessMetrics
from darwindeck.simulation.go_simulator import GoSimulator

# Use 'spawn' context for CGo safety (Go runtime is not fork-safe)
_mp_context = mp.get_context('spawn')


@dataclass
class SamplingConfig:
    """Configuration for mutation path sampling."""
    steps_per_path: int = 50        # Mutations per trajectory
    paths_per_genome: int = 25      # Independent paths per seed
    games_per_eval: int = 50        # Simulations for fitness estimate
    random_seeds: list[int] | None = None  # For reproducibility

    def validate(self) -> None:
        """Raise ValueError if config is invalid."""
        if self.steps_per_path < 1:
            raise ValueError("steps_per_path must be >= 1")
        if self.paths_per_genome < 1:
            raise ValueError("paths_per_genome must be >= 1")
        if self.games_per_eval < 1:
            raise ValueError("games_per_eval must be >= 1")


@dataclass
class FitnessTrajectory:
    """A single mutation path with fitness at each step."""
    seed_genome_id: str
    seed_type: str  # "known" or "baseline"
    random_seed: int
    steps: list[float] = field(default_factory=list)  # fitness at each mutation step
    final_genome: Optional[GameGenome] = None  # Endpoint for analysis

    @property
    def initial_fitness(self) -> float:
        """Fitness at step 0 (the seed genome)."""
        return self.steps[0] if self.steps else 0.0

    @property
    def final_fitness(self) -> float:
        """Fitness at final step."""
        return self.steps[-1] if self.steps else 0.0

    @property
    def fitness_delta(self) -> float:
        """Change in fitness from start to end."""
        if len(self.steps) < 2:
            return 0.0
        return self.steps[-1] - self.steps[0]


def _evaluate_fitness(
    genome: GameGenome,
    simulator: GoSimulator,
    evaluator: FitnessEvaluator,
    games_per_eval: int
) -> float:
    """Evaluate fitness for a single genome."""
    try:
        results = simulator.simulate(genome, num_games=games_per_eval)
        if results.errors > results.total_games * 0.5:
            # Too many errors - invalid genome
            return 0.0
        metrics = evaluator.evaluate(genome, results)
        return metrics.total_fitness
    except Exception:
        return 0.0


def sample_single_trajectory(
    seed_genome: GameGenome,
    seed_type: str,
    config: SamplingConfig,
    simulator: GoSimulator,
    evaluator: FitnessEvaluator,
    mutation_pipeline: MutationPipeline,
    random_seed: int,
) -> FitnessTrajectory:
    """
    Sample a single mutation trajectory from a seed genome.

    Args:
        seed_genome: Starting genome
        seed_type: "known" or "baseline"
        config: Sampling parameters
        simulator: Go simulator instance
        evaluator: Fitness evaluator
        mutation_pipeline: Mutation operators
        random_seed: Random seed for reproducibility

    Returns:
        FitnessTrajectory with fitness at each step
    """
    random.seed(random_seed)

    trajectory = FitnessTrajectory(
        seed_genome_id=seed_genome.genome_id,
        seed_type=seed_type,
        random_seed=random_seed,
        steps=[],
        final_genome=None,
    )

    # Evaluate initial fitness
    initial_fitness = _evaluate_fitness(
        seed_genome, simulator, evaluator, config.games_per_eval
    )
    trajectory.steps.append(initial_fitness)

    # Walk mutation path
    current_genome = seed_genome
    for _ in range(config.steps_per_path):
        # Apply mutations
        mutated = mutation_pipeline.apply(current_genome)

        # Evaluate fitness
        fitness = _evaluate_fitness(
            mutated, simulator, evaluator, config.games_per_eval
        )
        trajectory.steps.append(fitness)

        current_genome = mutated

    trajectory.final_genome = current_genome
    return trajectory


def sample_trajectories(
    seed_genomes: list[GameGenome],
    config: SamplingConfig,
    evaluator: FitnessEvaluator,
    seed_type: str = "known",
    progress_callback: Callable[[int, int], None] | None = None,
    base_random_seed: int = 42,
) -> list[FitnessTrajectory]:
    """
    Sample mutation paths from each seed genome.

    Args:
        seed_genomes: Starting genomes
        config: Sampling parameters
        evaluator: Fitness evaluator instance
        seed_type: "known" or "baseline" for trajectory labeling
        progress_callback: Optional (current, total) progress reporter
        base_random_seed: Base seed for reproducibility

    Returns:
        List of trajectories (len = seeds * paths_per_genome)

    Raises:
        RuntimeError: If fitness evaluation fails consistently
    """
    config.validate()

    trajectories: list[FitnessTrajectory] = []
    total_paths = len(seed_genomes) * config.paths_per_genome
    current_path = 0

    # Create simulator and mutation pipeline
    simulator = GoSimulator(seed=base_random_seed)
    mutation_pipeline = create_default_pipeline()

    # IMPORTANT: Create evaluator with caching disabled
    # Mutated genomes keep same genome_id, so cache would return wrong values
    style = getattr(evaluator, 'style', 'balanced')
    evaluator = FitnessEvaluator(style=style, use_cache=False)

    # Generate random seeds if not provided
    if config.random_seeds is None:
        rng = random.Random(base_random_seed)
        random_seeds = [rng.randint(0, 2**31) for _ in range(total_paths)]
    else:
        random_seeds = config.random_seeds
        if len(random_seeds) < total_paths:
            # Extend if not enough seeds
            rng = random.Random(random_seeds[-1] if random_seeds else 42)
            while len(random_seeds) < total_paths:
                random_seeds.append(rng.randint(0, 2**31))

    seed_idx = 0
    for genome in seed_genomes:
        for path_num in range(config.paths_per_genome):
            random_seed = random_seeds[seed_idx]

            trajectory = sample_single_trajectory(
                seed_genome=genome,
                seed_type=seed_type,
                config=config,
                simulator=simulator,
                evaluator=evaluator,
                mutation_pipeline=mutation_pipeline,
                random_seed=random_seed,
            )
            trajectories.append(trajectory)

            current_path += 1
            seed_idx += 1

            if progress_callback:
                progress_callback(current_path, total_paths)

    return trajectories


# Worker function for parallel sampling (must be at module level for pickling)
def _sample_trajectory_worker(args: tuple) -> FitnessTrajectory:
    """Worker function that samples a single trajectory."""
    genome, seed_type, config_dict, random_seed, style = args

    # Recreate objects in worker process
    # IMPORTANT: use_cache=False because mutated genomes keep same genome_id
    # and would incorrectly return cached fitness of the original seed
    config = SamplingConfig(**config_dict)
    simulator = GoSimulator(seed=random_seed)
    evaluator = FitnessEvaluator(style=style, use_cache=False)
    mutation_pipeline = create_default_pipeline()

    return sample_single_trajectory(
        seed_genome=genome,
        seed_type=seed_type,
        config=config,
        simulator=simulator,
        evaluator=evaluator,
        mutation_pipeline=mutation_pipeline,
        random_seed=random_seed,
    )


def sample_trajectories_parallel(
    seed_genomes: list[GameGenome],
    config: SamplingConfig,
    evaluator: FitnessEvaluator,
    seed_type: str = "known",
    progress_callback: Callable[[int, int], None] | None = None,
    base_random_seed: int = 42,
    num_workers: int | None = None,
) -> list[FitnessTrajectory]:
    """
    Sample mutation paths from each seed genome IN PARALLEL.

    Args:
        seed_genomes: Starting genomes
        config: Sampling parameters
        evaluator: Fitness evaluator instance (style is extracted)
        seed_type: "known" or "baseline" for trajectory labeling
        progress_callback: Optional (current, total) progress reporter
        base_random_seed: Base seed for reproducibility
        num_workers: Number of parallel workers (default: cpu_count)

    Returns:
        List of trajectories (len = seeds * paths_per_genome)
    """
    config.validate()

    total_paths = len(seed_genomes) * config.paths_per_genome
    num_workers = num_workers or mp.cpu_count()

    # Generate random seeds
    rng = random.Random(base_random_seed)
    random_seeds = [rng.randint(0, 2**31) for _ in range(total_paths)]

    # Prepare work items (must be picklable)
    config_dict = {
        'steps_per_path': config.steps_per_path,
        'paths_per_genome': config.paths_per_genome,
        'games_per_eval': config.games_per_eval,
    }
    style = getattr(evaluator, 'style', 'balanced')

    work_items = []
    seed_idx = 0
    for genome in seed_genomes:
        for _ in range(config.paths_per_genome):
            work_items.append((
                genome,
                seed_type,
                config_dict,
                random_seeds[seed_idx],
                style,
            ))
            seed_idx += 1

    # Run in parallel
    trajectories: list[FitnessTrajectory] = []

    with _mp_context.Pool(num_workers) as pool:
        for i, result in enumerate(pool.imap_unordered(_sample_trajectory_worker, work_items)):
            trajectories.append(result)
            if progress_callback:
                progress_callback(i + 1, total_paths)

    return trajectories


def compute_mean_trajectory(trajectories: list[FitnessTrajectory]) -> list[float]:
    """
    Compute mean fitness at each step across trajectories.

    Args:
        trajectories: List of trajectories (should have same length)

    Returns:
        Mean fitness at each step
    """
    if not trajectories:
        return []

    max_len = max(len(t.steps) for t in trajectories)
    means = []

    for step in range(max_len):
        values = [t.steps[step] for t in trajectories if step < len(t.steps)]
        if values:
            means.append(sum(values) / len(values))
        else:
            means.append(0.0)

    return means


def compute_std_trajectory(trajectories: list[FitnessTrajectory]) -> list[float]:
    """
    Compute standard deviation of fitness at each step across trajectories.

    Args:
        trajectories: List of trajectories

    Returns:
        Std dev at each step
    """
    if not trajectories:
        return []

    import math

    max_len = max(len(t.steps) for t in trajectories)
    stds = []

    for step in range(max_len):
        values = [t.steps[step] for t in trajectories if step < len(t.steps)]
        if len(values) > 1:
            mean = sum(values) / len(values)
            variance = sum((v - mean) ** 2 for v in values) / (len(values) - 1)
            stds.append(math.sqrt(variance))
        else:
            stds.append(0.0)

    return stds
