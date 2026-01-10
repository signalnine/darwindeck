"""Population seeding with known games (Phase 4)."""

from __future__ import annotations

import random
from typing import List
from cards_evolve.genome.schema import GameGenome
from cards_evolve.genome.examples import (
    create_war_genome,
    create_hearts_genome
)
from cards_evolve.evolution.operators import create_default_pipeline
from cards_evolve.evolution.population import Individual


def create_seed_population(
    size: int = 100,
    seed_ratio: float = 0.7,
    random_seed: int | None = None
) -> List[Individual]:
    """Create initial population with mix of known games and mutations.

    Args:
        size: Population size (default: 100)
        seed_ratio: Ratio of known games to mutants (default: 0.7 for 70%)
        random_seed: Random seed for reproducibility

    Returns:
        List of Individual objects with seeded genomes
    """
    if random_seed is not None:
        random.seed(random_seed)

    # Calculate counts
    n_seeds = int(size * seed_ratio)
    n_mutants = size - n_seeds

    # Load base genomes
    base_genomes = [
        create_war_genome(),
        create_hearts_genome(),
        # TODO: Add more when examples are available:
        # create_crazy_eights_genome(),
        # create_gin_rummy_genome(),
    ]

    population: List[Individual] = []

    # 1. Add known games (replicated to fill n_seeds slots)
    for i in range(n_seeds):
        genome = base_genomes[i % len(base_genomes)]
        # Give each copy a unique genome_id
        genome_copy = genome.__class__(
            schema_version=genome.schema_version,
            genome_id=f"{genome.genome_id}-seed-{i}",
            generation=genome.generation,
            setup=genome.setup,
            turn_structure=genome.turn_structure,
            special_effects=genome.special_effects,
            win_conditions=genome.win_conditions,
            scoring_rules=genome.scoring_rules,
            max_turns=genome.max_turns,
            min_turns=genome.min_turns,
            player_count=genome.player_count,
        )
        population.append(Individual(genome=genome_copy, fitness=0.0, evaluated=False))

    # 2. Add mutated variants
    mutation_pipeline = create_default_pipeline()

    for i in range(n_mutants):
        # Pick random base genome
        base_genome = random.choice(base_genomes)

        # Apply mutations (1-3 rounds)
        mutated = base_genome
        num_rounds = random.randint(1, 3)
        for _ in range(num_rounds):
            mutated = mutation_pipeline.apply(mutated)

        # Update genome_id
        mutated_copy = mutated.__class__(
            schema_version=mutated.schema_version,
            genome_id=f"{base_genome.genome_id}-mutant-{i}",
            generation=0,  # Reset generation for seed population
            setup=mutated.setup,
            turn_structure=mutated.turn_structure,
            special_effects=mutated.special_effects,
            win_conditions=mutated.win_conditions,
            scoring_rules=mutated.scoring_rules,
            max_turns=mutated.max_turns,
            min_turns=mutated.min_turns,
            player_count=mutated.player_count,
        )
        population.append(Individual(genome=mutated_copy, fitness=0.0, evaluated=False))

    # Shuffle population
    random.shuffle(population)

    return population


def create_minimal_seed_population(size: int = 10) -> List[Individual]:
    """Create minimal population for testing.

    Args:
        size: Population size (default: 10)

    Returns:
        List of Individual objects
    """
    return create_seed_population(size=size, seed_ratio=1.0)
