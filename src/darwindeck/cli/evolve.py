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
    num_runs: int = 10,
) -> List[GameGenome]:
    """Load ALL genomes from the last N evolution runs.

    Loads all saved genomes for diversity selection to pick from.
    The seeding function will apply structural diversity selection
    to choose the most different genomes.

    Args:
        output_dir: Base output directory containing run subdirectories
        num_runs: Number of recent runs to load from (default: 10)

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
        # Load ALL ranked genomes from this run (for diversity selection later)
        json_files = sorted(run_dir.glob("rank*.json"))
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
        '--player-count',
        type=int,
        default=None,
        choices=[2, 3, 4],
        help='Filter seed games by player count (2, 3, or 4 players). Default: no filter (all games)'
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
    # Note: --auto-seed-top-n removed - we now load ALL genomes and use diversity selection

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
        default=100,
        choices=[100, 500, 1000, 2000],
        help='MCTS search iterations per move (default: 100)'
    )
    parser.add_argument(
        '--skip-skill-eval',
        action='store_true',
        help='Skip post-evolution skill evaluation (Greedy + MCTS vs Random)'
    )

    # In-evolution skill evaluation (to penalize unfit games during breeding)
    parser.add_argument(
        '--skill-eval-frequency',
        type=int,
        default=10,
        help='Run skill evaluation every N generations during evolution (0 = disabled)'
    )
    parser.add_argument(
        '--skill-eval-games',
        type=int,
        default=10,
        help='Games per genome for in-evolution skill evaluation'
    )
    parser.add_argument(
        '--fpa-penalty-threshold',
        type=float,
        default=0.3,
        help='Penalize games with |first_player_advantage| > this (0.3 = 30%%)'
    )
    parser.add_argument(
        '--fpa-penalty-weight',
        type=float,
        default=0.3,
        help='Fitness penalty multiplier for high FPA (0.3 = 30%% fitness reduction)'
    )
    parser.add_argument(
        '--low-skill-threshold',
        type=float,
        default=0.6,
        help='Penalize games with skill_score < this'
    )
    parser.add_argument(
        '--low-skill-penalty',
        type=float,
        default=0.2,
        help='Fitness penalty multiplier for low skill (0.2 = 20%% fitness reduction)'
    )

    # Logging
    parser.add_argument(
        '--verbose', '-v',
        action='store_true',
        help='Enable verbose logging'
    )

    args = parser.parse_args()

    # Style-based defaults for player count
    if args.player_count is None:
        if args.style == 'party':
            args.player_count = 4  # Party games should be 4+ players

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
        logging.info(f"Auto-seeding from last {args.auto_seed_runs} runs (all genomes, diversity selection applied)...")
        seed_genomes = load_seeds_from_last_runs(
            output_dir=args.output_dir,
            num_runs=args.auto_seed_runs,
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
        fitness_style=args.style,
        player_count=args.player_count,  # Filter seeds by player count
        # In-evolution skill evaluation
        skill_eval_frequency=args.skill_eval_frequency,
        skill_eval_games=args.skill_eval_games,
        skill_eval_mcts_iterations=args.mcts_iterations,
        fpa_penalty_threshold=args.fpa_penalty_threshold,
        fpa_penalty_weight=args.fpa_penalty_weight,
        low_skill_penalty_threshold=args.low_skill_threshold,
        low_skill_penalty_weight=args.low_skill_penalty
    )

    # Create evolution engine
    logging.info("Creating evolution engine...")
    logging.info(f"  Population size: {config.population_size}")
    logging.info(f"  Max generations: {config.max_generations}")
    logging.info(f"  Fitness style: {config.fitness_style}")
    if config.player_count:
        logging.info(f"  Player count filter: {config.player_count} players only")
    else:
        logging.info(f"  Player count filter: all games")
    logging.info(f"  Elitism rate: {config.elitism_rate*100:.0f}%")
    logging.info(f"  Crossover rate: {config.crossover_rate*100:.0f}%")
    logging.info(f"  Tournament size: {config.tournament_size}")
    if config.plateau_threshold:
        logging.info(f"  Plateau detection: enabled ({config.plateau_threshold} generations)")
    else:
        logging.info(f"  Plateau detection: disabled")
    if config.skill_eval_frequency > 0:
        logging.info(f"  In-evolution skill eval: every {config.skill_eval_frequency} generations")
        logging.info(f"    - Top {config.skill_eval_top_percent*100:.0f}% evaluated, {config.skill_eval_games} games each")
        logging.info(f"    - FPA penalty: >{config.fpa_penalty_threshold*100:.0f}% -> {config.fpa_penalty_weight*100:.0f}% fitness reduction")
        logging.info(f"    - Low skill penalty: <{config.low_skill_penalty_threshold:.1f} -> {config.low_skill_penalty_weight*100:.0f}% fitness reduction")
    else:
        logging.info(f"  In-evolution skill eval: disabled")

    engine = EvolutionEngine(config)

    # Run evolution
    try:
        engine.evolve()
    except KeyboardInterrupt:
        logging.info("\n\nEvolution interrupted by user")
        return 1

    # Get best genomes
    best_genomes = engine.get_best_genomes(n=args.save_top_n)

    # Two-tier skill evaluation (unless skipped)
    skill_results: Dict[str, SkillEvalResult] = {}
    if not args.skip_skill_eval:
        logging.info(f"\nSkill Evaluation: Greedy vs Random + MCTS vs Random")
        logging.info(f"  ({args.mcts_games} games per tier, MCTS {args.mcts_iterations} iterations)")
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
                    fpa = skill.first_player_advantage
                    fpa_str = f" P0+{fpa:.0%}" if fpa > 0.1 else (f" P1+{-fpa:.0%}" if fpa < -0.1 else "")
                    logging.info(f"  {ind.genome.genome_id}: greedy={skill.greedy_win_rate:.0%} mcts={skill.mcts_win_rate:.0%} skill={skill.skill_score:.2f}{fpa_str}")
        except Exception as e:
            logging.warning(f"  Skill evaluation failed: {e}")

    # Re-rank if strategic style (use combined skill_score - higher is better)
    if args.style == 'strategic' and skill_results:
        logging.info("\nRe-ranking by skill score (--style strategic)...")
        best_genomes = sorted(
            best_genomes,
            key=lambda ind: skill_results.get(ind.genome.genome_id, SkillEvalResult(
                genome_id=ind.genome.genome_id,
                greedy_wins_as_p0=0, greedy_wins_as_p1=0, greedy_win_rate=0.5,
                mcts_wins_as_p0=0, mcts_wins_as_p1=0, mcts_win_rate=0.5,
                total_games=0, skill_score=0.5, first_player_advantage=0.0
            )).skill_score,
            reverse=True
        )

    # Re-rank if party style (use combined skill_score - LOWER is better for party games)
    if args.style == 'party' and skill_results:
        logging.info("\nRe-ranking by luck-friendliness (--style party, lower skill = better)...")
        best_genomes = sorted(
            best_genomes,
            key=lambda ind: skill_results.get(ind.genome.genome_id, SkillEvalResult(
                genome_id=ind.genome.genome_id,
                greedy_wins_as_p0=0, greedy_wins_as_p1=0, greedy_win_rate=0.5,
                mcts_wins_as_p0=0, mcts_wins_as_p1=0, mcts_win_rate=0.5,
                total_games=0, skill_score=0.5, first_player_advantage=0.0
            )).skill_score,
            reverse=False  # Lower skill = higher rank for party
        )

    # Filter out games with severe first-player advantage (> 30%)
    if skill_results:
        original_count = len(best_genomes)
        best_genomes = [
            ind for ind in best_genomes
            if abs(skill_results.get(ind.genome.genome_id, SkillEvalResult(
                genome_id=ind.genome.genome_id,
                greedy_wins_as_p0=0, greedy_wins_as_p1=0, greedy_win_rate=0.5,
                mcts_wins_as_p0=0, mcts_wins_as_p1=0, mcts_win_rate=0.5,
                total_games=0, skill_score=0.5, first_player_advantage=0.0
            )).first_player_advantage) <= 0.3
        ]
        filtered_count = original_count - len(best_genomes)
        if filtered_count > 0:
            logging.info(f"\nFiltered out {filtered_count} games with first-player advantage > 30%")

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
            genome_data['skill_evaluation'] = {
                'greedy_win_rate': skill.greedy_win_rate,
                'greedy_wins_as_p0': skill.greedy_wins_as_p0,
                'greedy_wins_as_p1': skill.greedy_wins_as_p1,
                'mcts_win_rate': skill.mcts_win_rate,
                'mcts_wins_as_p0': skill.mcts_wins_as_p0,
                'mcts_wins_as_p1': skill.mcts_wins_as_p1,
                'skill_score': skill.skill_score,
                'first_player_advantage': skill.first_player_advantage,
                'total_games': skill.total_games,
                'timed_out': skill.timed_out,
            }
            genome_data['skill_rank'] = i

        # Save as JSON
        json_file = run_output_dir / f"rank{i:02d}_{individual.genome.genome_id}.json"
        with open(json_file, 'w') as f:
            json.dump(genome_data, f, indent=2)

        skill_str = f", greedy={skill.greedy_win_rate:.0%} mcts={skill.mcts_win_rate:.0%}" if skill else ""
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
                        f.write(f"**Skill Evaluation:**\n")
                        f.write(f"- Greedy vs Random: {skill.greedy_win_rate:.1%}\n")
                        f.write(f"- MCTS vs Random: {skill.mcts_win_rate:.1%}\n")
                        f.write(f"- Combined Skill Score: {skill.skill_score:.2f}\n")
                        fpa = skill.first_player_advantage
                        if abs(fpa) > 0.1:
                            f.write(f"- **First Player Advantage: {fpa:+.1%}** {'⚠️' if abs(fpa) > 0.3 else ''}\n")

                    # Include fitness metrics breakdown
                    if individual.fitness_metrics:
                        fm = individual.fitness_metrics
                        f.write("\n**Fitness Metrics:**\n")
                        f.write(f"| Metric | Score |\n")
                        f.write(f"|--------|-------|\n")
                        f.write(f"| Decision Density | {fm.decision_density:.3f} |\n")
                        f.write(f"| Comeback Potential | {fm.comeback_potential:.3f} |\n")
                        f.write(f"| Tension Curve | {fm.tension_curve:.3f} |\n")
                        f.write(f"| Interaction Frequency | {fm.interaction_frequency:.3f} |\n")
                        f.write(f"| Rules Complexity | {fm.rules_complexity:.3f} |\n")
                        f.write(f"| Skill vs Luck | {fm.skill_vs_luck:.3f} |\n")
                        f.write(f"| Bluffing Depth | {fm.bluffing_depth:.3f} |\n")
                        f.write(f"| Session Length | {fm.session_length:.3f} |\n")

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
                skill_str = f", greedy={skill.greedy_win_rate:.0%} mcts={skill.mcts_win_rate:.0%}" if skill else ""
                logging.info(f"\n{i}. {genome_id} (fitness={individual.fitness:.4f}{skill_str})")

                # Print fitness metrics breakdown
                if individual.fitness_metrics:
                    fm = individual.fitness_metrics
                    logging.info(f"   Metrics: DD={fm.decision_density:.2f} CP={fm.comeback_potential:.2f} "
                                f"TC={fm.tension_curve:.2f} IF={fm.interaction_frequency:.2f} "
                                f"RC={fm.rules_complexity:.2f} SL={fm.skill_vs_luck:.2f} "
                                f"BD={fm.bluffing_depth:.2f}")

                if genome_id in descriptions:
                    logging.info(f"   {descriptions[genome_id]}")

    logging.info(f"\n✅ Evolution complete! Best fitness: {engine.best_ever.fitness:.4f}")
    return 0


if __name__ == '__main__':
    sys.exit(main())
