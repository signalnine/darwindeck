"""Evolution engine for genetic algorithm (Phase 4)."""

from __future__ import annotations

import random
import logging
import os
from typing import List, Optional, Callable
from dataclasses import dataclass
from cards_evolve.evolution.population import Population, Individual
from cards_evolve.evolution.operators import (
    MutationPipeline,
    CrossoverOperator,
    create_default_pipeline
)
from cards_evolve.evolution.seeding import create_seed_population
from cards_evolve.evolution.parallel_fitness import ParallelFitnessEvaluator
from cards_evolve.evolution.fitness import FitnessEvaluator

logger = logging.getLogger(__name__)


@dataclass
class EvolutionConfig:
    """Configuration for evolutionary run."""

    population_size: int = 100
    max_generations: int = 100
    elitism_rate: float = 0.1  # Top 10% preserved
    crossover_rate: float = 0.7  # 70% undergo crossover
    tournament_size: int = 3
    plateau_threshold: int = 30  # Generations without 1% improvement
    improvement_threshold: float = 0.01  # 1% improvement
    diversity_threshold: float = 0.1  # Warn if diversity < 0.1
    seed_ratio: float = 0.7  # 70% known games, 30% mutants
    random_seed: Optional[int] = None


@dataclass
class GenerationStats:
    """Statistics for a single generation."""

    generation: int
    best_fitness: float
    avg_fitness: float
    diversity: float
    evaluations: int


class EvolutionEngine:
    """Main evolution engine for genetic algorithm.

    Implements:
    - Tournament selection
    - Elitism (top 10%)
    - Crossover (70%)
    - Mutation (applied to all offspring)
    - Plateau detection (30 generations)
    """

    def __init__(
        self,
        config: EvolutionConfig,
        fitness_evaluator: Optional[Callable[[Individual], Individual]] = None,
        mutation_pipeline: Optional[MutationPipeline] = None,
        crossover_operator: Optional[CrossoverOperator] = None,
        num_workers: Optional[int] = None
    ):
        """Initialize evolution engine.

        Args:
            config: Evolution configuration
            fitness_evaluator: Function to evaluate fitness (Individual -> Individual with fitness)
            mutation_pipeline: Mutation operators (default: standard pipeline)
            crossover_operator: Crossover operator (default: standard crossover)
            num_workers: Number of parallel workers (default: os.cpu_count())
        """
        self.config = config
        self.num_workers = num_workers or int(os.environ.get('EVOLUTION_WORKERS', os.cpu_count() or 4))

        # Initialize parallel fitness evaluator
        self.parallel_evaluator = ParallelFitnessEvaluator(
            evaluator_factory=lambda: FitnessEvaluator(),
            num_workers=self.num_workers
        )

        self.fitness_evaluator = fitness_evaluator or self._default_fitness_evaluator
        self.mutation_pipeline = mutation_pipeline or create_default_pipeline()
        self.crossover = crossover_operator or CrossoverOperator(probability=config.crossover_rate)

        if config.random_seed is not None:
            random.seed(config.random_seed)

        self.population: Optional[Population] = None
        self.stats_history: List[GenerationStats] = []
        self.best_ever: Optional[Individual] = None

        logger.info(f"Evolution engine initialized with {self.num_workers} parallel workers")

    def _default_fitness_evaluator(self, individual: Individual) -> Individual:
        """Default fitness evaluator (placeholder).

        Args:
            individual: Individual to evaluate

        Returns:
            Individual with fitness = 0.5 (placeholder)
        """
        # TODO: Replace with actual fitness evaluation
        return Individual(
            genome=individual.genome,
            fitness=0.5,
            evaluated=True
        )

    def initialize_population(self) -> None:
        """Create initial population with seeding."""
        logger.info(f"Initializing population of size {self.config.population_size}")
        individuals = create_seed_population(
            size=self.config.population_size,
            seed_ratio=self.config.seed_ratio,
            random_seed=self.config.random_seed
        )
        self.population = Population(individuals=individuals)
        logger.info(f"Population initialized with {len(individuals)} individuals")

    def evaluate_population(self) -> None:
        """Evaluate fitness for all unevaluated individuals using parallel evaluation."""
        if self.population is None:
            raise ValueError("Population not initialized")

        unevaluated = [ind for ind in self.population.individuals if not ind.evaluated]
        if not unevaluated:
            logger.info("All individuals already evaluated")
            return

        logger.info(f"Evaluating {len(unevaluated)} individuals using {self.num_workers} workers...")

        # Extract genomes for batch evaluation
        genomes = [ind.genome for ind in unevaluated]

        # Batch evaluate using parallel fitness evaluator
        fitness_results = self.parallel_evaluator.evaluate_population(
            genomes,
            num_simulations=100,  # Standard simulation count
            use_mcts=False  # Start with random AI, can upgrade to MCTS later
        )

        # Update individuals with fitness scores
        for i, (individual, fitness_metrics) in enumerate(zip(unevaluated, fitness_results)):
            # Find index in population
            idx = self.population.individuals.index(individual)
            # Create evaluated individual
            evaluated = Individual(
                genome=individual.genome,
                fitness=fitness_metrics.total_fitness,
                evaluated=True
            )
            self.population.individuals[idx] = evaluated

        logger.info(f"Evaluation complete. Avg fitness: {self.population.get_average_fitness():.3f}")

    def tournament_selection(self, k: int = 3) -> Individual:
        """Select individual via tournament selection.

        Args:
            k: Tournament size (default: 3)

        Returns:
            Selected individual
        """
        if self.population is None:
            raise ValueError("Population not initialized")

        # Sample k individuals
        candidates = random.sample(self.population.individuals, k)

        # Return best
        return max(candidates, key=lambda ind: ind.fitness)

    def create_offspring(self) -> List[Individual]:
        """Create offspring via selection, crossover, and mutation.

        Returns:
            List of offspring individuals
        """
        if self.population is None:
            raise ValueError("Population not initialized")

        offspring: List[Individual] = []

        # 1. Elitism - preserve top individuals
        n_elite = int(self.config.population_size * self.config.elitism_rate)
        elite = sorted(self.population.individuals, key=lambda ind: ind.fitness, reverse=True)[:n_elite]
        offspring.extend(elite)
        logger.debug(f"Preserved {n_elite} elite individuals")

        # 2. Create remaining offspring via selection + crossover + mutation
        n_offspring = self.config.population_size - n_elite

        while len(offspring) < self.config.population_size:
            # Select two parents
            parent1 = self.tournament_selection(k=self.config.tournament_size)
            parent2 = self.tournament_selection(k=self.config.tournament_size)

            # Crossover
            child1, child2 = self.crossover.crossover(parent1.genome, parent2.genome)

            # Mutation
            child1 = self.mutation_pipeline.apply(child1)
            child2 = self.mutation_pipeline.apply(child2)

            # Add to offspring (mark as unevaluated)
            offspring.append(Individual(genome=child1, fitness=0.0, evaluated=False))
            if len(offspring) < self.config.population_size:
                offspring.append(Individual(genome=child2, fitness=0.0, evaluated=False))

        return offspring[:self.config.population_size]

    def check_plateau(self) -> bool:
        """Check if evolution has plateaued.

        Returns:
            True if plateaued (no improvement for plateau_threshold generations)
        """
        if len(self.stats_history) < self.config.plateau_threshold:
            return False

        # Check if best fitness improved by > 1% in last N generations
        recent_stats = self.stats_history[-self.config.plateau_threshold:]
        best_recent = max(s.best_fitness for s in recent_stats)
        oldest_recent = recent_stats[0].best_fitness

        if oldest_recent == 0:
            return False

        improvement = (best_recent - oldest_recent) / oldest_recent

        if improvement < self.config.improvement_threshold:
            logger.info(f"Plateau detected: {improvement*100:.2f}% improvement in last {self.config.plateau_threshold} generations")
            return True

        return False

    def evolve(self) -> None:
        """Run evolutionary loop."""
        logger.info("Starting evolutionary loop...")

        # Initialize population if not already done
        if self.population is None:
            self.initialize_population()

        # Evaluate initial population
        self.evaluate_population()

        # Evolution loop
        for generation in range(self.config.max_generations):
            logger.info(f"\n{'='*60}")
            logger.info(f"Generation {generation + 1}/{self.config.max_generations}")
            logger.info(f"{'='*60}")

            # Compute statistics
            best = self.population.get_best_individual()
            avg_fitness = self.population.get_average_fitness()
            diversity = self.population.compute_diversity()

            # Update best ever
            if self.best_ever is None or best.fitness > self.best_ever.fitness:
                self.best_ever = best
                logger.info(f"🏆 New best fitness: {best.fitness:.4f} (genome: {best.genome.genome_id})")

            # Store stats
            stats = GenerationStats(
                generation=generation,
                best_fitness=best.fitness,
                avg_fitness=avg_fitness,
                diversity=diversity,
                evaluations=len([ind for ind in self.population.individuals if ind.evaluated])
            )
            self.stats_history.append(stats)

            # Log stats
            logger.info(f"Best fitness: {best.fitness:.4f}")
            logger.info(f"Avg fitness: {avg_fitness:.4f}")
            logger.info(f"Diversity: {diversity:.4f}")

            # Check diversity crisis
            if diversity < self.config.diversity_threshold:
                logger.warning(f"⚠️  Low diversity: {diversity:.4f} (threshold: {self.config.diversity_threshold})")

            # Check plateau
            if self.check_plateau():
                logger.info("Stopping due to plateau")
                break

            # Create next generation
            offspring = self.create_offspring()
            self.population = Population(individuals=offspring)
            self.population.generation = generation + 1

            # Evaluate new individuals
            self.evaluate_population()

        logger.info("\n" + "="*60)
        logger.info("Evolution complete!")
        logger.info(f"Best fitness: {self.best_ever.fitness:.4f}")
        logger.info(f"Best genome: {self.best_ever.genome.genome_id}")
        logger.info("="*60)

    def get_best_genomes(self, n: int = 10) -> List[Individual]:
        """Get top N genomes from all generations.

        Args:
            n: Number of top genomes to return

        Returns:
            List of top individuals
        """
        if self.population is None:
            return []

        # Collect all evaluated individuals from history
        all_individuals = [self.best_ever] if self.best_ever else []
        all_individuals.extend(self.population.individuals)

        # Sort by fitness and return top N
        sorted_inds = sorted(all_individuals, key=lambda ind: ind.fitness, reverse=True)
        return sorted_inds[:n]
