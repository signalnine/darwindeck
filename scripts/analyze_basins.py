#!/usr/bin/env python3
"""CLI entry point for basin analysis."""

import argparse
import json
import sys
from pathlib import Path

# Add src to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent / "src"))

from darwindeck.genome.examples import get_seed_genomes
from darwindeck.evolution.fitness_full import FitnessEvaluator
from darwindeck.analysis.genome_distance import compute_distance_matrix
from darwindeck.analysis.mutation_sampler import SamplingConfig, sample_trajectories
from darwindeck.analysis.random_baseline import (
    BaselineConfig, generate_random_genomes, compute_baseline_statistics
)
from darwindeck.analysis.basin_detector import detect_basins, compute_valley_depths
from darwindeck.analysis.basin_report import (
    print_summary, save_json, plot_heatmap, plot_trajectories,
    plot_basin_scatter, plot_baseline_comparison,
)


def progress_reporter(current: int, total: int) -> None:
    """Print progress updates."""
    pct = (current / total) * 100 if total > 0 else 0
    print(f"\rSampling trajectories: {current}/{total} ({pct:.1f}%)", end="", flush=True)
    if current == total:
        print()  # Newline at end


def main():
    parser = argparse.ArgumentParser(
        description="Analyze fitness basin structure of card game genomes"
    )

    # Sampling config
    parser.add_argument("--steps", type=int, default=50,
                        help="Mutations per path (default: 50)")
    parser.add_argument("--paths", type=int, default=25,
                        help="Paths per genome (default: 25)")
    parser.add_argument("--games", type=int, default=50,
                        help="Games per fitness eval (default: 50)")

    # Distance config
    parser.add_argument("--weights", type=str, default=None,
                        help="JSON file with custom field weights")

    # Clustering config
    parser.add_argument("--linkage", choices=["ward", "complete", "average"],
                        default="ward", help="Clustering linkage method")
    parser.add_argument("--max-clusters", type=int, default=None,
                        help="Maximum clusters to consider")

    # Baseline config
    parser.add_argument("--baseline", type=int, default=20,
                        help="Number of random genomes for baseline (0 to skip)")
    parser.add_argument("--no-baseline", action="store_true",
                        help="Skip baseline comparison entirely")

    # Output config
    parser.add_argument("--output-dir", type=Path,
                        default=Path("output/basin_analysis"),
                        help="Output directory")
    parser.add_argument("--seed", type=int, default=42,
                        help="Random seed for reproducibility")

    # Execution
    parser.add_argument("--dry-run", action="store_true",
                        help="Validate config without running")
    parser.add_argument("-v", "--verbose", action="store_true",
                        help="Verbose output")

    # Style preset
    parser.add_argument("--style", type=str, default="balanced",
                        choices=["balanced", "bluffing", "strategic", "party", "trick-taking"],
                        help="Fitness style preset (default: balanced)")

    args = parser.parse_args()

    # Load field weights if provided
    field_weights = None
    if args.weights:
        with open(args.weights) as f:
            field_weights = json.load(f)

    # Create config
    sampling_config = SamplingConfig(
        steps_per_path=args.steps,
        paths_per_genome=args.paths,
        games_per_eval=args.games,
    )

    baseline_config = BaselineConfig(
        num_random_genomes=0 if args.no_baseline else args.baseline,
    )

    # Validate config
    try:
        sampling_config.validate()
    except ValueError as e:
        print(f"Invalid config: {e}")
        sys.exit(1)

    # Dry run - just print config and exit
    if args.dry_run:
        print("=== Basin Analysis Config (Dry Run) ===")
        print(f"Sampling: {args.steps} steps x {args.paths} paths x {args.games} games/eval")
        print(f"Linkage method: {args.linkage}")
        print(f"Baseline genomes: {baseline_config.num_random_genomes}")
        print(f"Output directory: {args.output_dir}")
        print(f"Random seed: {args.seed}")
        print(f"Fitness style: {args.style}")

        # Load genomes to show count
        genomes = get_seed_genomes()
        print(f"Known genomes: {len(genomes)}")

        total_known_evals = len(genomes) * args.paths * (args.steps + 1) * args.games
        total_baseline_evals = baseline_config.num_random_genomes * args.paths * (args.steps + 1) * args.games
        total_evals = total_known_evals + total_baseline_evals
        print(f"Estimated evaluations: {total_evals:,}")
        print(f"Estimated time: ~{total_evals / 2000:.0f} minutes")
        return

    # Run analysis
    print("=== Basin Analysis ===")
    print(f"Config: {args.steps} steps x {args.paths} paths")

    # 1. Load known genomes
    print("\n1. Loading known genomes...")
    known_genomes = get_seed_genomes()
    print(f"   Loaded {len(known_genomes)} known games")

    # 2. Compute distance matrix
    print("\n2. Computing distance matrix...")
    distance_matrix, genome_labels = compute_distance_matrix(known_genomes, field_weights)
    print(f"   Matrix shape: {distance_matrix.shape}")

    # 3. Sample mutation trajectories for known genomes
    print("\n3. Sampling mutation trajectories (known games)...")
    evaluator = FitnessEvaluator(style=args.style)

    known_trajectories = sample_trajectories(
        seed_genomes=known_genomes,
        config=sampling_config,
        evaluator=evaluator,
        seed_type="known",
        progress_callback=progress_reporter if args.verbose else None,
        base_random_seed=args.seed,
    )
    print(f"   Sampled {len(known_trajectories)} trajectories")

    # 4. Generate and sample random baseline (if enabled)
    baseline_stats = None
    random_trajectories = []

    if baseline_config.num_random_genomes > 0:
        print(f"\n4. Generating {baseline_config.num_random_genomes} random genomes for baseline...")
        random_genomes = generate_random_genomes(
            baseline_config, evaluator, random_seed=args.seed + 1000
        )
        print(f"   Generated {len(random_genomes)} playable random genomes")

        if random_genomes:
            print("\n5. Sampling mutation trajectories (random baseline)...")
            random_trajectories = sample_trajectories(
                seed_genomes=random_genomes,
                config=sampling_config,
                evaluator=evaluator,
                seed_type="baseline",
                progress_callback=progress_reporter if args.verbose else None,
                base_random_seed=args.seed + 2000,
            )
            print(f"   Sampled {len(random_trajectories)} baseline trajectories")

            print("\n6. Computing baseline statistics...")
            baseline_stats = compute_baseline_statistics(known_trajectories, random_trajectories)

    # 5. Detect basins
    step_num = 7 if baseline_config.num_random_genomes > 0 else 4
    print(f"\n{step_num}. Detecting basins...")
    analysis = detect_basins(
        distance_matrix, genome_labels,
        method=args.linkage,
        max_clusters=args.max_clusters,
    )
    print(f"   Found {analysis.optimal_k} clusters (silhouette={analysis.silhouette_score:.2f})")

    # 6. Compute valley depths
    step_num += 1
    print(f"\n{step_num}. Computing valley depths...")
    analysis.valley_depths.update(compute_valley_depths(known_trajectories, analysis))
    print(f"   Computed {len(analysis.valley_depths)} valley depths")

    # 7. Generate outputs
    step_num += 1
    print(f"\n{step_num}. Generating outputs...")

    # Create output directory
    args.output_dir.mkdir(parents=True, exist_ok=True)

    # Terminal summary
    all_trajectories = known_trajectories + random_trajectories
    print_summary(analysis, all_trajectories, baseline_stats)

    # JSON output
    save_json(
        analysis, all_trajectories, distance_matrix, genome_labels,
        sampling_config, args.output_dir / "basin_analysis.json",
        baseline_stats
    )

    # Plots
    plot_heatmap(
        distance_matrix, genome_labels, analysis.linkage_matrix,
        args.output_dir / "heatmap.png"
    )

    plot_trajectories(
        known_trajectories,
        args.output_dir / "trajectories.png"
    )

    plot_basin_scatter(
        distance_matrix, genome_labels, analysis.cluster_assignments,
        args.output_dir / "basin_scatter.png"
    )

    if baseline_stats is not None and random_trajectories:
        plot_baseline_comparison(
            known_trajectories, random_trajectories, baseline_stats,
            args.output_dir / "baseline_comparison.png"
        )

    print(f"\nAnalysis complete! Results saved to: {args.output_dir}")


if __name__ == "__main__":
    main()
