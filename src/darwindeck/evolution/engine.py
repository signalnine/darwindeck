"""Evolution engine for genetic algorithm (Phase 4)."""

from __future__ import annotations

import random
import logging
import os
from functools import partial
from typing import List, Optional, Callable, Dict
from dataclasses import dataclass, field
from darwindeck.evolution.population import Population, Individual
from darwindeck.evolution.skill_evaluation import evaluate_batch_skill, SkillEvalResult
from darwindeck.evolution.operators import (
    MutationPipeline,
    CrossoverOperator,
    create_default_pipeline,
    create_aggressive_pipeline
)
from darwindeck.evolution.seeding import create_seed_population, create_seed_population_from_genomes
from darwindeck.evolution.parallel_fitness import ParallelFitnessEvaluator, _create_evaluator
from darwindeck.evolution.fitness_full import FitnessEvaluator
from darwindeck.genome.schema import GameGenome

logger = logging.getLogger(__name__)


@dataclass
class EvolutionConfig:
    """Configuration for evolutionary run."""

    population_size: int = 100
    max_generations: int = 100
    elitism_rate: float = 0.1  # Top 10% preserved
    crossover_rate: float = 0.7  # 70% undergo crossover
    tournament_size: int = 3
    plateau_threshold: Optional[int] = None  # None = disabled, or N generations without improvement
    improvement_threshold: float = 0.005  # 0.5% improvement (relaxed from 1%)
    diversity_threshold: float = 0.1  # Warn if diversity < 0.1
    seed_ratio: float = 0.7  # 70% known games, 30% mutants
    random_seed: Optional[int] = None
    seed_genomes: Optional[List[GameGenome]] = None  # Custom genomes to seed from
    fitness_style: str = 'balanced'  # Fitness weight preset (balanced, bluffing, strategic, party, trick-taking)
    player_count: Optional[int] = None  # Filter seeds by player count (2, 3, or 4). None = all games
    # Skill evaluation during evolution
    skill_eval_frequency: int = 10  # Run skill eval every N generations (0 = disabled)
    skill_eval_top_percent: float = 0.1  # Evaluate top 10% (matches elitism rate)
    skill_eval_games: int = 10  # Games per skill evaluation (fast: 10, thorough: 50)
    skill_eval_mcts_iterations: int = 100  # MCTS iterations for skill eval
    fpa_penalty_threshold: float = 0.3  # Penalize if |first_player_advantage| > this
    fpa_penalty_weight: float = 0.3  # Fitness multiplier for FPA penalty (0.3 = 30% reduction)
    low_skill_penalty_threshold: float = 0.6  # Penalize if skill_score < this
    low_skill_penalty_weight: float = 0.2  # Fitness multiplier for low skill penalty
    # Party style: penalize high skill (we want luck-friendly games)
    high_skill_penalty_threshold: float = 0.85  # Penalize if skill_score > this (party style only)
    high_skill_penalty_weight: float = 0.3  # Fitness multiplier for high skill penalty


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
        # Cap default workers at 64 to avoid massive spawn overhead on high-core machines
        # 256 workers with 'spawn' context causes ~30s+ startup time and potential hangs
        default_workers = min(os.cpu_count() or 4, 64)
        self.num_workers = num_workers or int(os.environ.get('EVOLUTION_WORKERS', default_workers))

        # Initialize parallel fitness evaluator with style preset
        # Use partial to pass style to the factory function (picklable unlike lambdas)
        evaluator_factory = partial(_create_evaluator, style=config.fitness_style)
        self.parallel_evaluator = ParallelFitnessEvaluator(
            evaluator_factory=evaluator_factory,
            num_workers=self.num_workers
        )

        logger.info(f"Fitness style: {config.fitness_style}")

        self.fitness_evaluator = fitness_evaluator or self._default_fitness_evaluator

        # Create mutation pipelines, preserving player_count if filtered
        preserve_player_count = config.player_count is not None
        self.mutation_pipeline = mutation_pipeline or create_default_pipeline(
            preserve_player_count=preserve_player_count
        )
        self.aggressive_pipeline = create_aggressive_pipeline(
            preserve_player_count=preserve_player_count
        )
        self.crossover = crossover_operator or CrossoverOperator(probability=config.crossover_rate)

        if config.random_seed is not None:
            random.seed(config.random_seed)

        self.population: Optional[Population] = None
        self.stats_history: List[GenerationStats] = []
        self.best_ever: Optional[Individual] = None
        self.use_aggressive_mutation = False  # Switch to True when diversity drops
        self._skill_eval_cache: Dict[str, SkillEvalResult] = {}  # Cache skill results by genome_id

        logger.info(f"Evolution engine initialized with {self.num_workers} parallel workers")

    def close(self) -> None:
        """Clean up resources (worker pools, etc).

        Call this when done with evolution to prevent resource leaks.
        """
        if self.parallel_evaluator is not None:
            self.parallel_evaluator.close()
            logger.debug("Closed parallel fitness evaluator")

    def __del__(self) -> None:
        """Cleanup on garbage collection (fallback)."""
        self.close()

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

        if self.config.seed_genomes:
            # Use custom seed genomes
            logger.info(f"Using {len(self.config.seed_genomes)} custom seed genomes")
            individuals = create_seed_population_from_genomes(
                base_genomes=self.config.seed_genomes,
                size=self.config.population_size,
                seed_ratio=self.config.seed_ratio,
                random_seed=self.config.random_seed,
                player_count=self.config.player_count
            )
        else:
            # Use default example games
            individuals = create_seed_population(
                size=self.config.population_size,
                seed_ratio=self.config.seed_ratio,
                random_seed=self.config.random_seed,
                player_count=self.config.player_count
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

        # Update individuals with fitness scores and full metrics
        for i, (individual, fitness_metrics) in enumerate(zip(unevaluated, fitness_results)):
            # Find index in population
            idx = self.population.individuals.index(individual)
            # Create evaluated individual with full metrics breakdown
            evaluated = Individual(
                genome=individual.genome,
                fitness=fitness_metrics.total_fitness,
                evaluated=True,
                fitness_metrics=fitness_metrics  # Store full metrics for saving
            )
            self.population.individuals[idx] = evaluated

        logger.info(f"Evaluation complete. Avg fitness: {self.population.get_average_fitness():.3f}")

    def evaluate_skill_and_penalize(self, generation: int) -> None:
        """Run skill evaluation on top performers and penalize unfit games.

        Penalizes games with:
        - High first-player advantage (|FPA| > threshold)
        - Low skill scores (skill_score < threshold)

        Args:
            generation: Current generation number (for logging)
        """
        if self.population is None:
            return

        # Check if skill eval is enabled and it's the right generation
        if self.config.skill_eval_frequency <= 0:
            return
        if generation % self.config.skill_eval_frequency != 0:
            return

        # Get top N% of population for skill evaluation
        n_to_evaluate = max(1, int(len(self.population.individuals) * self.config.skill_eval_top_percent))
        sorted_pop = sorted(self.population.individuals, key=lambda ind: ind.fitness, reverse=True)
        top_individuals = sorted_pop[:n_to_evaluate]

        logger.info(f"🎯 Running skill evaluation on top {n_to_evaluate} individuals...")

        # Check cache for already-evaluated genomes
        uncached_genomes = []
        cached_results = []
        for ind in top_individuals:
            genome_id = ind.genome.genome_id
            if genome_id in self._skill_eval_cache:
                cached_results.append(self._skill_eval_cache[genome_id])
            else:
                uncached_genomes.append(ind.genome)

        cache_hits = len(cached_results)
        if cache_hits > 0:
            logger.info(f"  Cache hits: {cache_hits}/{n_to_evaluate}")

        # Run skill evaluation only on uncached genomes
        from darwindeck.evolution.skill_evaluation import evaluate_batch_skill

        new_results = []
        if uncached_genomes:
            def progress(completed: int, total: int) -> None:
                if completed % 5 == 0 or completed == total:
                    logger.info(f"  Skill eval: {completed}/{total}")

            new_results = evaluate_batch_skill(
                genomes=uncached_genomes,
                num_games=self.config.skill_eval_games,
                mcts_iterations=self.config.skill_eval_mcts_iterations,
                timeout_sec=30.0,  # Shorter timeout for in-evolution eval
                num_workers=self.num_workers,
                progress_callback=progress
            )

            # Update cache with new results
            for result in new_results:
                self._skill_eval_cache[result.genome_id] = result

        # Build result lookup from cached + new
        all_results = cached_results + new_results
        skill_by_id = {r.genome_id: r for r in all_results}

        # Apply penalties
        penalties_applied = 0
        fpa_penalties = 0
        skill_penalties = 0
        is_party_style = self.config.fitness_style == 'party'

        for i, ind in enumerate(self.population.individuals):
            skill_result = skill_by_id.get(ind.genome.genome_id)
            if skill_result is None:
                continue

            penalty_multiplier = 1.0

            # Penalize high first-player advantage
            if abs(skill_result.first_player_advantage) > self.config.fpa_penalty_threshold:
                penalty_multiplier *= (1.0 - self.config.fpa_penalty_weight)
                fpa_penalties += 1

            # Style-aware skill penalty
            if is_party_style:
                # Party games: penalize HIGH skill (we want luck-friendly games)
                if skill_result.skill_score > self.config.high_skill_penalty_threshold:
                    penalty_multiplier *= (1.0 - self.config.high_skill_penalty_weight)
                    skill_penalties += 1
            else:
                # Other styles: penalize LOW skill (we want skill-rewarding games)
                if skill_result.skill_score < self.config.low_skill_penalty_threshold:
                    penalty_multiplier *= (1.0 - self.config.low_skill_penalty_weight)
                    skill_penalties += 1

            # Apply penalty if any
            if penalty_multiplier < 1.0:
                penalties_applied += 1
                old_fitness = ind.fitness
                new_fitness = ind.fitness * penalty_multiplier
                # Update individual in population
                self.population.individuals[i] = Individual(
                    genome=ind.genome,
                    fitness=new_fitness,
                    evaluated=True,
                    fitness_metrics=ind.fitness_metrics
                )
                logger.debug(f"  Penalized {ind.genome.genome_id}: {old_fitness:.4f} -> {new_fitness:.4f} "
                           f"(FPA={skill_result.first_player_advantage:+.2f}, skill={skill_result.skill_score:.2f})")

        skill_label = "high-skill" if is_party_style else "low-skill"
        logger.info(f"  Skill eval complete: {penalties_applied} penalties applied "
                   f"({fpa_penalties} FPA, {skill_penalties} {skill_label})")

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

        # Select mutation pipeline based on diversity
        pipeline = self.aggressive_pipeline if self.use_aggressive_mutation else self.mutation_pipeline

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

            # Mutation (use selected pipeline)
            child1 = pipeline.apply(child1)
            child2 = pipeline.apply(child2)

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
        # Plateau detection disabled
        if self.config.plateau_threshold is None:
            return False

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

        # Run initial skill evaluation if enabled
        if self.config.skill_eval_frequency > 0:
            self.evaluate_skill_and_penalize(0)

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
            mode_indicator = " [AGGRESSIVE]" if self.use_aggressive_mutation else ""
            logger.info(f"Best fitness: {best.fitness:.4f}")
            logger.info(f"Avg fitness: {avg_fitness:.4f}")
            logger.info(f"Diversity: {diversity:.4f}{mode_indicator}")

            # Check diversity and switch mutation mode
            if diversity < self.config.diversity_threshold:
                if not self.use_aggressive_mutation:
                    logger.warning(f"⚠️  Low diversity ({diversity:.4f}) - switching to AGGRESSIVE mutation mode")
                    self.use_aggressive_mutation = True
            elif diversity > self.config.diversity_threshold * 1.5:
                # Diversity recovered - switch back to normal mode
                if self.use_aggressive_mutation:
                    logger.info(f"✓ Diversity recovered ({diversity:.4f}) - switching back to normal mutation mode")
                    self.use_aggressive_mutation = False

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

            # Run skill evaluation and penalize unfit games (every N generations)
            self.evaluate_skill_and_penalize(generation + 1)

        logger.info("\n" + "="*60)
        logger.info("Evolution complete!")
        logger.info(f"Best fitness: {self.best_ever.fitness:.4f}")
        logger.info(f"Best genome: {self.best_ever.genome.genome_id}")
        logger.info("="*60)

    def get_best_genomes(self, n: int = 10) -> List[Individual]:
        """Get top N unique genomes from all generations.

        Args:
            n: Number of top genomes to return

        Returns:
            List of top individuals (deduplicated by genome_id)
        """
        if self.population is None:
            return []

        # Collect all evaluated individuals from history
        all_individuals = [self.best_ever] if self.best_ever else []
        all_individuals.extend(self.population.individuals)

        # Sort by fitness
        sorted_inds = sorted(all_individuals, key=lambda ind: ind.fitness, reverse=True)

        # Deduplicate by genome_id, keeping highest fitness version
        seen_ids: set = set()
        unique_inds: List[Individual] = []
        for ind in sorted_inds:
            if ind.genome.genome_id not in seen_ids:
                seen_ids.add(ind.genome.genome_id)
                unique_inds.append(ind)
                if len(unique_inds) >= n:
                    break

        return unique_inds

    def evaluate_skill_gaps(
        self,
        top_n: int = 20,
        num_games: int = 100,
        mcts_iterations: int = 500,
        timeout_sec: float = 60.0
    ) -> Dict[str, SkillEvalResult]:
        """Run MCTS skill evaluation on top genomes.

        Args:
            top_n: Number of top genomes to evaluate
            num_games: Games per genome for evaluation
            mcts_iterations: MCTS search iterations
            timeout_sec: Timeout per genome

        Returns:
            Dict mapping genome_id to SkillEvalResult
        """
        best_genomes = self.get_best_genomes(n=top_n)
        genomes = [ind.genome for ind in best_genomes]

        def progress(completed: int, total: int) -> None:
            logger.info(f"  Evaluating genome {completed}/{total}...")

        results = evaluate_batch_skill(
            genomes=genomes,
            num_games=num_games,
            mcts_iterations=mcts_iterations,
            timeout_sec=timeout_sec,
            num_workers=self.num_workers,
            progress_callback=progress
        )

        return {result.genome_id: result for result in results}
