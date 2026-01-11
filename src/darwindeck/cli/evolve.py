"""CLI command for running evolution (Phase 4)."""

from __future__ import annotations

import argparse
import logging
import sys
import json
from datetime import datetime
from pathlib import Path
from typing import List, Optional, Tuple, Dict
from darwindeck.evolution.engine import EvolutionEngine, EvolutionConfig
from darwindeck.evolution.describe import describe_top_games
from darwindeck.evolution.skill_evaluation import SkillEvalResult
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


def load_seeds_from_last_runs(
    output_dir: Path,
    num_runs: int = 5,
    top_n_per_run: int = 10
) -> List[GameGenome]:
    """Load top genomes from the last N evolution runs.

    Args:
        output_dir: Base output directory containing run subdirectories
        num_runs: Number of recent runs to load from (default: 5)
        top_n_per_run: Number of top genomes to load per run (default: 10)

    Returns:
        List of loaded GameGenome objects
    """
    # Search directories to check (output_dir and its parent)
    # This handles cases like --output-dir output/evolution-xxx where we still
    # want to find runs in output/other-evolution-yyy/
    search_dirs = [output_dir]
    if output_dir.parent != output_dir and output_dir.parent.exists():
        search_dirs.append(output_dir.parent)

    # Find all directories containing rank*.json files (recursively)
    run_dirs_with_times: List[Tuple[Path, float]] = []

    for search_dir in search_dirs:
        if not search_dir.exists():
            continue
        for json_file in search_dir.rglob("rank01_*.json"):
            run_dir = json_file.parent
            # Avoid duplicates
            if any(run_dir == existing[0] for existing in run_dirs_with_times):
                continue
            # Use the modification time of rank01 as the run time
            run_time = json_file.stat().st_mtime
            run_dirs_with_times.append((run_dir, run_time))

    if not run_dirs_with_times:
        logging.info("No previous runs found in output directory")
        return []

    # Sort by time (newest first) and take last N runs
    run_dirs_with_times.sort(key=lambda x: x[1], reverse=True)
    recent_runs = run_dirs_with_times[:num_runs]

    genomes = []
    for run_dir, _ in recent_runs:
        # Load ranked genomes (rank01, rank02, etc.)
        json_files = sorted(run_dir.glob("rank*.json"))[:top_n_per_run]
        for json_file in json_files:
            try:
                with open(json_file) as f:
                    genome = genome_from_json(f.read())
                    genomes.append(genome)
                    logging.debug(f"  Loaded {genome.genome_id} from {run_dir.name}/{json_file.name}")
            except Exception as e:
                logging.warning(f"  Failed to load {json_file.name}: {e}")

    logging.info(f"  Found {len(recent_runs)} recent runs")
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
        '--style', '-s',
        type=str,
        default='balanced',
        choices=['balanced', 'bluffing', 'strategic', 'party', 'trick-taking'],
        help='Fitness style preset (affects what kinds of games are favored)'
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
        '--enable-plateau',
        type=int,
        default=None,
        metavar='N',
        help='Enable plateau detection: stop after N generations without improvement (disabled by default)'
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
    parser.add_argument(
        '--auto-seed',
        action='store_true',
        default=True,
        help='Automatically load seeds from last N runs (default: enabled)'
    )
    parser.add_argument(
        '--no-auto-seed',
        action='store_true',
        help='Disable auto-seeding from previous runs'
    )
    parser.add_argument(
        '--auto-seed-runs',
        type=int,
        default=5,
        help='Number of previous runs to load seeds from'
    )
    parser.add_argument(
        '--auto-seed-top-n',
        type=int,
        default=10,
        help='Number of top genomes to load per previous run'
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
    parser.add_argument(
        '--no-describe',
        action='store_true',
        help='Skip LLM-generated game descriptions'
    )

    # Skill evaluation options
    parser.add_argument(
        '--mcts-games',
        type=int,
        default=100,
        help='Games per genome for MCTS skill evaluation (default: 100)'
    )
    parser.add_argument(
        '--mcts-iterations',
        type=int,
        default=500,
        choices=[100, 500, 1000, 2000],
        help='MCTS search iterations per move (default: 500)'
    )
    parser.add_argument(
        '--skip-skill-eval',
        action='store_true',
        help='Skip post-evolution MCTS skill evaluation'
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

    # Load seed genomes
    seed_genomes = None

    if args.seed_from:
        # Explicit seed directory takes precedence
        if not args.seed_from.exists():
            logging.error(f"Seed directory not found: {args.seed_from}")
            return 1
        logging.info(f"Loading seed genomes from {args.seed_from}")
        seed_genomes = load_seed_genomes(args.seed_from)
        if not seed_genomes:
            logging.error("No valid genomes found in seed directory")
            return 1
        logging.info(f"  Loaded {len(seed_genomes)} seed genomes")

    elif args.auto_seed and not args.no_auto_seed:
        # Auto-seed from last N runs
        logging.info(f"Auto-seeding from last {args.auto_seed_runs} runs (top {args.auto_seed_top_n} per run)...")
        seed_genomes = load_seeds_from_last_runs(
            output_dir=args.output_dir,
            num_runs=args.auto_seed_runs,
            top_n_per_run=args.auto_seed_top_n
        )
        if seed_genomes:
            logging.info(f"  Loaded {len(seed_genomes)} seed genomes from previous runs")
        else:
            logging.info("  No previous runs found, using default seeds")

    # Create configuration
    config = EvolutionConfig(
        population_size=args.population_size,
        max_generations=args.generations,
        elitism_rate=args.elitism_rate,
        crossover_rate=args.crossover_rate,
        tournament_size=args.tournament_size,
        plateau_threshold=args.enable_plateau,  # None = disabled
        seed_ratio=args.seed_ratio,
        random_seed=args.random_seed,
        seed_genomes=seed_genomes,
        fitness_style=args.style
    )

    # Create evolution engine
    logging.info("Creating evolution engine...")
    logging.info(f"  Population size: {config.population_size}")
    logging.info(f"  Max generations: {config.max_generations}")
    logging.info(f"  Fitness style: {config.fitness_style}")
    logging.info(f"  Elitism rate: {config.elitism_rate*100:.0f}%")
    logging.info(f"  Crossover rate: {config.crossover_rate*100:.0f}%")
    logging.info(f"  Tournament size: {config.tournament_size}")
    if config.plateau_threshold:
        logging.info(f"  Plateau detection: enabled ({config.plateau_threshold} generations)")
    else:
        logging.info(f"  Plateau detection: disabled")

    engine = EvolutionEngine(config)

    # Run evolution
    try:
        engine.evolve()
    except KeyboardInterrupt:
        logging.info("\n\nEvolution interrupted by user")
        return 1

    # Get best genomes
    best_genomes = engine.get_best_genomes(n=args.save_top_n)

    # MCTS Skill evaluation (unless skipped)
    skill_results: Dict[str, SkillEvalResult] = {}
    if not args.skip_skill_eval:
        logging.info(f"\nMCTS Skill Evaluation ({args.mcts_games} games, {args.mcts_iterations} iterations)")
        try:
            skill_results = engine.evaluate_skill_gaps(
                top_n=args.save_top_n,
                num_games=args.mcts_games,
                mcts_iterations=args.mcts_iterations
            )
            logging.info(f"  Completed skill evaluation for {len(skill_results)} genomes")

            # Show results
            logging.info("\nSkill Evaluation Results:")
            for ind in best_genomes:
                skill = skill_results.get(ind.genome.genome_id)
                if skill:
                    logging.info(f"  {ind.genome.genome_id}: mcts_win_rate={skill.mcts_win_rate:.2f}")
        except Exception as e:
            logging.warning(f"  Skill evaluation failed: {e}")

    # Re-rank if strategic style
    if args.style == 'strategic' and skill_results:
        logging.info("\nRe-ranking by skill (--style strategic)...")
        best_genomes = sorted(
            best_genomes,
            key=lambda ind: skill_results.get(ind.genome.genome_id, SkillEvalResult(
                genome_id=ind.genome.genome_id,
                mcts_wins_as_p1=0, mcts_wins_as_p2=0,
                total_mcts_wins=0, total_games=0,
                mcts_win_rate=0.0
            )).mcts_win_rate,
            reverse=True
        )

    # Save best genomes as JSON to timestamped subdirectory
    timestamp = datetime.now().strftime("%Y-%m-%d_%H-%M-%S")
    run_output_dir = args.output_dir / timestamp
    run_output_dir.mkdir(parents=True, exist_ok=True)

    logging.info(f"\nSaving top {len(best_genomes)} genomes to {run_output_dir}")
    for i, individual in enumerate(best_genomes, 1):
        skill = skill_results.get(individual.genome.genome_id)

        # Create extended data dict with fitness and skill info
        genome_data = json.loads(genome_to_json(individual.genome))
        genome_data['fitness'] = individual.fitness
        genome_data['fitness_rank'] = i

        # Include full fitness metrics breakdown if available
        if individual.fitness_metrics:
            fm = individual.fitness_metrics
            genome_data['fitness_metrics'] = {
                'decision_density': fm.decision_density,
                'comeback_potential': fm.comeback_potential,
                'tension_curve': fm.tension_curve,
                'interaction_frequency': fm.interaction_frequency,
                'rules_complexity': fm.rules_complexity,
                'session_length': fm.session_length,
                'skill_vs_luck': fm.skill_vs_luck,
                'bluffing_depth': fm.bluffing_depth,
                'games_simulated': fm.games_simulated,
                'valid': fm.valid,
            }

        if skill:
            genome_data['mcts_win_rate'] = skill.mcts_win_rate
            genome_data['skill_rank'] = i
            genome_data['mcts_wins_as_p1'] = skill.mcts_wins_as_p1
            genome_data['mcts_wins_as_p2'] = skill.mcts_wins_as_p2
            genome_data['skill_eval_timed_out'] = skill.timed_out

        # Save as JSON
        json_file = run_output_dir / f"rank{i:02d}_{individual.genome.genome_id}.json"
        with open(json_file, 'w') as f:
            json.dump(genome_data, f, indent=2)

        skill_str = f", mcts_win_rate={skill.mcts_win_rate:.2f}" if skill else ""
        logging.info(f"  {i}. {individual.genome.genome_id} (fitness={individual.fitness:.4f}{skill_str})")

    # Generate LLM descriptions for top 5 games
    if not args.no_describe:
        logging.info("\nGenerating descriptions for top 5 games...")
        top_5 = [(ind.genome, ind.fitness) for ind in best_genomes[:5]]
        descriptions = describe_top_games(top_5, top_n=5, skill_results=skill_results)

        if descriptions:
            # Save descriptions to markdown file
            desc_file = run_output_dir / "descriptions.md"
            with open(desc_file, 'w') as f:
                f.write(f"# Top 5 Evolved Games\n\n")
                f.write(f"Run: {timestamp}\n\n")
                for i, individual in enumerate(best_genomes[:5], 1):
                    genome_id = individual.genome.genome_id
                    skill = skill_results.get(genome_id)
                    f.write(f"## {i}. {genome_id}\n\n")
                    f.write(f"**Fitness:** {individual.fitness:.4f}\n")
                    if skill:
                        f.write(f"**MCTS Win Rate:** {skill.mcts_win_rate:.1%}\n")
                    f.write("\n")
                    if genome_id in descriptions:
                        f.write(f"{descriptions[genome_id]}\n\n")
                    else:
                        f.write("*Description not available*\n\n")
                    f.write("---\n\n")

            logging.info(f"  Saved descriptions to {desc_file}")

            # Also print descriptions to console
            logging.info("\n" + "="*60)
            logging.info("TOP 5 GAME DESCRIPTIONS")
            logging.info("="*60)
            for i, individual in enumerate(best_genomes[:5], 1):
                genome_id = individual.genome.genome_id
                skill = skill_results.get(genome_id)
                skill_str = f", skill={skill.mcts_win_rate:.1%}" if skill else ""
                logging.info(f"\n{i}. {genome_id} (fitness={individual.fitness:.4f}{skill_str})")
                if genome_id in descriptions:
                    logging.info(f"   {descriptions[genome_id]}")

    logging.info(f"\n✅ Evolution complete! Best fitness: {engine.best_ever.fitness:.4f}")
    return 0


if __name__ == '__main__':
    sys.exit(main())
