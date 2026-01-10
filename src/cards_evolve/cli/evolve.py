"""CLI command for running evolution (Phase 4)."""

from __future__ import annotations

import argparse
import logging
import sys
from pathlib import Path
from cards_evolve.evolution.engine import EvolutionEngine, EvolutionConfig


def setup_logging(verbose: bool = False) -> None:
    """Setup logging configuration.

    Args:
        verbose: Enable verbose (DEBUG) logging
    """
    level = logging.DEBUG if verbose else logging.INFO
    logging.basicConfig(
        level=level,
        format='%(message)s',
        handlers=[
            logging.StreamHandler(sys.stdout)
        ]
    )


def main() -> int:
    """Run evolution CLI."""
    parser = argparse.ArgumentParser(
        description='Evolve novel card games using genetic algorithms',
        formatter_class=argparse.ArgumentDefaultsHelpFormatter
    )

    # Evolution parameters
    parser.add_argument(
        '--population-size', '-p',
        type=int,
        default=100,
        help='Population size'
    )
    parser.add_argument(
        '--generations', '-g',
        type=int,
        default=100,
        help='Maximum number of generations'
    )
    parser.add_argument(
        '--elitism-rate', '-e',
        type=float,
        default=0.1,
        help='Elitism rate (fraction of top individuals preserved)'
    )
    parser.add_argument(
        '--crossover-rate', '-c',
        type=float,
        default=0.7,
        help='Crossover probability'
    )
    parser.add_argument(
        '--tournament-size', '-t',
        type=int,
        default=3,
        help='Tournament selection size'
    )
    parser.add_argument(
        '--plateau-threshold',
        type=int,
        default=30,
        help='Generations without improvement before stopping'
    )
    parser.add_argument(
        '--seed-ratio',
        type=float,
        default=0.7,
        help='Ratio of known games to mutants in initial population'
    )
    parser.add_argument(
        '--random-seed',
        type=int,
        default=None,
        help='Random seed for reproducibility'
    )

    # Output options
    parser.add_argument(
        '--output-dir', '-o',
        type=Path,
        default=Path('output'),
        help='Output directory for best genomes'
    )
    parser.add_argument(
        '--save-top-n',
        type=int,
        default=10,
        help='Number of top genomes to save'
    )

    # Logging
    parser.add_argument(
        '--verbose', '-v',
        action='store_true',
        help='Enable verbose logging'
    )

    args = parser.parse_args()

    # Setup logging
    setup_logging(args.verbose)

    # Create configuration
    config = EvolutionConfig(
        population_size=args.population_size,
        max_generations=args.generations,
        elitism_rate=args.elitism_rate,
        crossover_rate=args.crossover_rate,
        tournament_size=args.tournament_size,
        plateau_threshold=args.plateau_threshold,
        seed_ratio=args.seed_ratio,
        random_seed=args.random_seed
    )

    # Create evolution engine
    logging.info("Creating evolution engine...")
    logging.info(f"  Population size: {config.population_size}")
    logging.info(f"  Max generations: {config.max_generations}")
    logging.info(f"  Elitism rate: {config.elitism_rate*100:.0f}%")
    logging.info(f"  Crossover rate: {config.crossover_rate*100:.0f}%")
    logging.info(f"  Tournament size: {config.tournament_size}")
    logging.info(f"  Plateau threshold: {config.plateau_threshold}")

    engine = EvolutionEngine(config)

    # Run evolution
    try:
        engine.evolve()
    except KeyboardInterrupt:
        logging.info("\n\nEvolution interrupted by user")
        return 1

    # Save best genomes
    args.output_dir.mkdir(parents=True, exist_ok=True)
    best_genomes = engine.get_best_genomes(n=args.save_top_n)

    logging.info(f"\nSaving top {len(best_genomes)} genomes to {args.output_dir}")
    for i, individual in enumerate(best_genomes, 1):
        output_file = args.output_dir / f"genome_rank{i:02d}_fitness{individual.fitness:.4f}.txt"
        with open(output_file, 'w') as f:
            f.write(f"Genome ID: {individual.genome.genome_id}\n")
            f.write(f"Fitness: {individual.fitness:.4f}\n")
            f.write(f"Generation: {individual.genome.generation}\n")
            f.write(f"\n{individual.genome}\n")
        logging.info(f"  {i}. {individual.genome.genome_id} (fitness={individual.fitness:.4f})")

    logging.info(f"\n✅ Evolution complete! Best fitness: {engine.best_ever.fitness:.4f}")
    return 0


if __name__ == '__main__':
    sys.exit(main())
