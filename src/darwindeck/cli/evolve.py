"""CLI command for running evolution (Phase 4)."""

from __future__ import annotations

import argparse
import logging
import sys
import json
from pathlib import Path
from typing import List, Optional
from darwindeck.evolution.engine import EvolutionEngine, EvolutionConfig
from darwindeck.genome.serialization import genome_to_json, genome_from_json
from darwindeck.genome.schema import GameGenome


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


def load_seed_genomes(seed_dir: Path) -> List[GameGenome]:
    """Load genomes from JSON files in a directory.

    Args:
        seed_dir: Directory containing .json genome files

    Returns:
        List of loaded GameGenome objects
    """
    genomes = []
    json_files = sorted(seed_dir.glob("*.json"))

    for json_file in json_files:
        try:
            with open(json_file) as f:
                genome = genome_from_json(f.read())
                genomes.append(genome)
                logging.debug(f"  Loaded {genome.genome_id} from {json_file.name}")
        except Exception as e:
            logging.warning(f"  Failed to load {json_file.name}: {e}")

    return genomes


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
    parser.add_argument(
        '--seed-from',
        type=Path,
        default=None,
        help='Directory containing JSON genomes to use as seeds (replaces default seeds)'
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

    # Load seed genomes if specified
    seed_genomes = None
    if args.seed_from:
        if not args.seed_from.exists():
            logging.error(f"Seed directory not found: {args.seed_from}")
            return 1
        logging.info(f"Loading seed genomes from {args.seed_from}")
        seed_genomes = load_seed_genomes(args.seed_from)
        if not seed_genomes:
            logging.error("No valid genomes found in seed directory")
            return 1
        logging.info(f"  Loaded {len(seed_genomes)} seed genomes")

    # Create configuration
    config = EvolutionConfig(
        population_size=args.population_size,
        max_generations=args.generations,
        elitism_rate=args.elitism_rate,
        crossover_rate=args.crossover_rate,
        tournament_size=args.tournament_size,
        plateau_threshold=args.plateau_threshold,
        seed_ratio=args.seed_ratio,
        random_seed=args.random_seed,
        seed_genomes=seed_genomes
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

    # Save best genomes as JSON (can be reused as seeds)
    args.output_dir.mkdir(parents=True, exist_ok=True)
    best_genomes = engine.get_best_genomes(n=args.save_top_n)

    logging.info(f"\nSaving top {len(best_genomes)} genomes to {args.output_dir}")
    for i, individual in enumerate(best_genomes, 1):
        # Save as JSON for reuse as seeds
        json_file = args.output_dir / f"rank{i:02d}_{individual.genome.genome_id}.json"
        with open(json_file, 'w') as f:
            f.write(genome_to_json(individual.genome))
        logging.info(f"  {i}. {individual.genome.genome_id} (fitness={individual.fitness:.4f})")

    logging.info(f"\n✅ Evolution complete! Best fitness: {engine.best_ever.fitness:.4f}")
    return 0


if __name__ == '__main__':
    sys.exit(main())
