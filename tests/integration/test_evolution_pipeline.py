"""Integration tests for evolution pipeline (Phase 4)."""

import pytest
from darwindeck.evolution.engine import EvolutionEngine, EvolutionConfig
from darwindeck.evolution.population import Individual
from darwindeck.evolution.seeding import create_seed_population
from darwindeck.evolution.operators import create_default_pipeline, CrossoverOperator
from darwindeck.genome.examples import create_war_genome, create_hearts_genome


def test_seed_population_creation():
    """Test that seed population can be created with correct ratios."""
    population = create_seed_population(size=100, seed_ratio=0.7, random_seed=42)

    assert len(population) == 100
    assert all(isinstance(ind, Individual) for ind in population)

    # Check ratio (approximately 70/30)
    seeds = [ind for ind in population if 'seed' in ind.genome.genome_id]
    mutants = [ind for ind in population if 'mutant' in ind.genome.genome_id]

    assert len(seeds) == 70
    assert len(mutants) == 30


def test_mutation_pipeline_applies():
    """Test that mutation pipeline modifies genomes."""
    genome = create_war_genome()
    pipeline = create_default_pipeline()

    # Apply mutations multiple times
    mutated = genome
    for _ in range(10):
        new_mutated = pipeline.apply(mutated)
        mutated = new_mutated

    # Should have changed generation
    assert mutated.generation > genome.generation


def test_crossover_produces_offspring():
    """Test that crossover produces valid offspring."""
    parent1 = create_war_genome()
    parent2 = create_hearts_genome()

    crossover = CrossoverOperator(probability=1.0)  # Always apply
    child1, child2 = crossover.crossover(parent1, parent2)

    # Children should have incremented generation
    assert child1.generation == parent1.generation + 1
    assert child2.generation == parent2.generation + 1

    # Children should have different genome IDs
    assert child1.genome_id != parent1.genome_id
    assert child2.genome_id != parent2.genome_id


def test_evolution_engine_initialization():
    """Test that evolution engine initializes correctly."""
    config = EvolutionConfig(
        population_size=10,
        max_generations=5,
        random_seed=42
    )

    engine = EvolutionEngine(config)
    engine.initialize_population()

    assert engine.population is not None
    assert len(engine.population.individuals) == 10
    assert all(not ind.evaluated for ind in engine.population.individuals)


def test_evolution_engine_evaluation():
    """Test that evolution engine evaluates population."""
    config = EvolutionConfig(
        population_size=10,
        max_generations=5,
        random_seed=42
    )

    engine = EvolutionEngine(config)
    engine.initialize_population()
    engine.evaluate_population()

    # All individuals should be evaluated
    assert all(ind.evaluated for ind in engine.population.individuals)

    # All should have fitness assigned
    assert all(ind.fitness > 0 for ind in engine.population.individuals)


def test_evolution_engine_tournament_selection():
    """Test tournament selection."""
    config = EvolutionConfig(
        population_size=20,
        tournament_size=3,
        random_seed=42
    )

    engine = EvolutionEngine(config)
    engine.initialize_population()

    # Manually set fitness values for testing
    for i, ind in enumerate(engine.population.individuals):
        engine.population.individuals[i] = Individual(
            genome=ind.genome,
            fitness=float(i) / 20.0,  # 0.0 to 0.95
            evaluated=True
        )

    # Run tournament selection multiple times
    selected_fitnesses = []
    for _ in range(100):
        selected = engine.tournament_selection(k=3)
        selected_fitnesses.append(selected.fitness)

    # Average selected fitness should be higher than population average
    avg_selected = sum(selected_fitnesses) / len(selected_fitnesses)
    avg_population = sum(ind.fitness for ind in engine.population.individuals) / len(engine.population.individuals)

    assert avg_selected > avg_population


def test_evolution_engine_offspring_creation():
    """Test offspring creation."""
    config = EvolutionConfig(
        population_size=10,
        elitism_rate=0.2,
        random_seed=42
    )

    engine = EvolutionEngine(config)
    engine.initialize_population()
    engine.evaluate_population()

    # Create offspring
    offspring = engine.create_offspring()

    assert len(offspring) == 10

    # Elite should be preserved (top 2 out of 10)
    # Check that best individual is in offspring
    best = engine.population.get_best_individual()
    best_in_offspring = any(ind.genome.genome_id == best.genome.genome_id for ind in offspring)
    assert best_in_offspring


def test_evolution_engine_full_run():
    """Test complete evolution run with minimal config."""
    config = EvolutionConfig(
        population_size=5,
        max_generations=3,
        plateau_threshold=2,
        random_seed=42
    )

    engine = EvolutionEngine(config)
    engine.evolve()

    # Should have run at least 1 generation
    assert len(engine.stats_history) >= 1

    # Should have best individual
    assert engine.best_ever is not None
    assert engine.best_ever.evaluated
    assert engine.best_ever.fitness > 0


def test_evolution_engine_plateau_detection():
    """Test plateau detection stops evolution early."""
    config = EvolutionConfig(
        population_size=5,
        max_generations=100,  # Would run long without plateau
        plateau_threshold=3,  # Stop after 3 generations without improvement
        random_seed=42
    )

    engine = EvolutionEngine(config)
    engine.evolve()

    # Should stop before max generations (placeholder fitness never improves)
    assert len(engine.stats_history) < 100


def test_get_best_genomes():
    """Test retrieving top N genomes."""
    config = EvolutionConfig(
        population_size=10,
        max_generations=2,
        random_seed=42
    )

    engine = EvolutionEngine(config)
    engine.evolve()

    # Get top 5
    best = engine.get_best_genomes(n=5)

    assert len(best) <= 5
    assert all(isinstance(ind, Individual) for ind in best)

    # Should be sorted by fitness (descending)
    fitnesses = [ind.fitness for ind in best]
    assert fitnesses == sorted(fitnesses, reverse=True)
