"""Population seeding with known games (Phase 4)."""

from __future__ import annotations

import random
from typing import List, Set, Optional
from darwindeck.genome.schema import GameGenome
from darwindeck.evolution.naming import generate_unique_name
from darwindeck.genome.examples import get_seed_genomes
from darwindeck.evolution.operators import create_default_pipeline
from darwindeck.evolution.population import Individual
from darwindeck.evolution.diversity import select_diverse_subset, compute_population_diversity


def create_seed_population(
    size: int = 100,
    seed_ratio: float = 0.3,
    random_seed: int | None = None,
    player_count: int | None = None
) -> List[Individual]:
    """Create initial population with mix of known games and mutations.

    Args:
        size: Population size (default: 100)
        seed_ratio: Ratio of known games to mutants (default: 0.3 for 30%)
                   Reduced from 0.7 to encourage more exploration
        random_seed: Random seed for reproducibility
        player_count: Filter seeds by player count (2, 3, or 4). None = all games

    Returns:
        List of Individual objects with seeded genomes
    """
    if random_seed is not None:
        random.seed(random_seed)

    # Calculate counts
    n_seeds = int(size * seed_ratio)
    n_mutants = size - n_seeds

    # Load base genomes from centralized examples (16 games)
    base_genomes = get_seed_genomes()

    # Filter by player count if specified
    if player_count is not None:
        base_genomes = [g for g in base_genomes if g.player_count == player_count]
        if not base_genomes:
            raise ValueError(f"No seed games found with player_count={player_count}")

    population: List[Individual] = []
    used_names: Set[str] = set()

    # 1. Add known games (replicated to fill n_seeds slots)
    for i in range(n_seeds):
        genome = base_genomes[i % len(base_genomes)]
        # Give each copy a unique genome_id
        new_name = generate_unique_name(used_names)
        used_names.add(new_name)
        genome_copy = genome.__class__(
            schema_version=genome.schema_version,
            genome_id=new_name,
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
    # Preserve player_count if filtering is active
    mutation_pipeline = create_default_pipeline(preserve_player_count=(player_count is not None))

    for i in range(n_mutants):
        # Pick random base genome
        base_genome = random.choice(base_genomes)

        # Apply mutations (2-6 rounds for more exploration)
        mutated = base_genome
        num_rounds = random.randint(2, 6)
        for _ in range(num_rounds):
            mutated = mutation_pipeline.apply(mutated)

        # Update genome_id with random name
        new_name = generate_unique_name(used_names)
        used_names.add(new_name)
        mutated_copy = mutated.__class__(
            schema_version=mutated.schema_version,
            genome_id=new_name,
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


def create_seed_population_from_genomes(
    base_genomes: List[GameGenome],
    size: int = 100,
    seed_ratio: float = 0.3,
    random_seed: int | None = None,
    player_count: int | None = None,
    max_seeds_from_previous: int = 20,
) -> List[Individual]:
    """Create initial population from custom genomes + example games.

    Uses diversity selection to pick structurally different genomes from
    previous runs, avoiding convergence to local optima.

    Args:
        base_genomes: List of genomes to use as seeds (e.g., previous winners)
        size: Population size (default: 100)
        seed_ratio: Ratio of seeds to mutants (default: 0.3 for 30%)
        random_seed: Random seed for reproducibility
        player_count: Filter seeds by player count (2, 3, or 4). None = all games
        max_seeds_from_previous: Max diverse genomes to select from previous runs (default: 20)

    Returns:
        List of Individual objects with seeded genomes
    """
    if random_seed is not None:
        random.seed(random_seed)

    if not base_genomes:
        raise ValueError("No base genomes provided")

    # Select diverse subset from previous winners using structural distance
    # This prevents seeding with many similar converged genomes
    if len(base_genomes) > max_seeds_from_previous:
        diverse_previous = select_diverse_subset(
            base_genomes,
            target_size=max_seeds_from_previous,
            random_seed=random_seed
        )
        diversity_before = compute_population_diversity(base_genomes[:max_seeds_from_previous])
        diversity_after = compute_population_diversity(diverse_previous)
        print(f"Diversity selection: {len(base_genomes)} -> {len(diverse_previous)} genomes "
              f"(diversity: {diversity_before:.3f} -> {diversity_after:.3f})")
    else:
        diverse_previous = base_genomes

    # Always include example games for structural diversity
    example_genomes = get_seed_genomes()

    # Merge diverse previous winners with examples
    # Deduplicate by genome_id to avoid exact duplicates
    seen_ids = set()
    combined_genomes = []
    for g in diverse_previous + example_genomes:
        if g.genome_id not in seen_ids:
            combined_genomes.append(g)
            seen_ids.add(g.genome_id)

    base_genomes = combined_genomes

    # Filter by player count if specified
    if player_count is not None:
        base_genomes = [g for g in base_genomes if g.player_count == player_count]
        if not base_genomes:
            raise ValueError(f"No seed games found with player_count={player_count}")

    # Calculate counts
    n_seeds = int(size * seed_ratio)
    n_mutants = size - n_seeds

    population: List[Individual] = []
    used_names: Set[str] = set()

    # 1. Add seed genomes (replicated to fill n_seeds slots)
    for i in range(n_seeds):
        genome = base_genomes[i % len(base_genomes)]
        new_name = generate_unique_name(used_names)
        used_names.add(new_name)
        genome_copy = genome.__class__(
            schema_version=genome.schema_version,
            genome_id=new_name,
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
    # Preserve player_count if filtering is active
    mutation_pipeline = create_default_pipeline(preserve_player_count=(player_count is not None))

    for i in range(n_mutants):
        base_genome = random.choice(base_genomes)
        mutated = base_genome
        num_rounds = random.randint(2, 6)
        for _ in range(num_rounds):
            mutated = mutation_pipeline.apply(mutated)

        new_name = generate_unique_name(used_names)
        used_names.add(new_name)
        mutated_copy = mutated.__class__(
            schema_version=mutated.schema_version,
            genome_id=new_name,
            generation=0,
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

    random.shuffle(population)
    return population
