"""Output generation for basin analysis (terminal, JSON, plots)."""

from __future__ import annotations

import json
from datetime import datetime
from pathlib import Path
from typing import Optional

import numpy as np

from darwindeck.analysis.mutation_sampler import (
    FitnessTrajectory, SamplingConfig,
    compute_mean_trajectory, compute_std_trajectory,
)
from darwindeck.analysis.random_baseline import BaselineStatistics
from darwindeck.analysis.basin_detector import (
    BasinAnalysis, interpret_silhouette, interpret_valley_depth,
)


def print_summary(
    analysis: BasinAnalysis,
    trajectories: list[FitnessTrajectory],
    baseline: Optional[BaselineStatistics] = None
) -> None:
    """Print human-readable basin analysis summary."""
    print("\n" + "=" * 50)
    print("Basin Analysis Report")
    print("=" * 50)

    # Baseline comparison (if available)
    if baseline is not None:
        print("\n--- Baseline Comparison ---")
        print("Known games vs random genomes:")
        print(f"  Known mean fitness: {baseline.known_mean_fitness:.3f} +/- {baseline.known_std_fitness:.3f}")
        print(f"  Random mean fitness: {baseline.random_mean_fitness:.3f} +/- {baseline.random_std_fitness:.3f}")

        if baseline.fitness_difference_pvalue < 0.001:
            sig_str = "p < 0.001"
        elif baseline.fitness_difference_pvalue < 0.01:
            sig_str = "p < 0.01"
        elif baseline.fitness_difference_pvalue < 0.05:
            sig_str = "p < 0.05"
        else:
            sig_str = f"p = {baseline.fitness_difference_pvalue:.3f}"

        sig_label = "SIGNIFICANT" if baseline.fitness_difference_pvalue < 0.05 else "not significant"
        print(f"  Difference: {sig_str} ({sig_label})")
        print()
        print(f"  Known decay rate: {baseline.known_mean_decay_rate:.4f}/step", end="")
        print(" (gentle slope)" if baseline.known_mean_decay_rate > -0.01 else " (steep)")
        print(f"  Random decay rate: {baseline.random_mean_decay_rate:.4f}/step", end="")
        print(" (gentle slope)" if baseline.random_mean_decay_rate > -0.01 else " (steep)")
        print(f"  Basin radius: {baseline.known_mean_basin_radius:.1f} mutations (known) vs {baseline.random_mean_basin_radius:.1f} mutations (random)")
        print()

        if baseline.known_games_are_special:
            print("Conclusion: Known games ARE in special neighborhoods")
        else:
            print("Conclusion: No significant difference from random starting points")

    # Clustering analysis
    print("\n--- Clustering Analysis ---")
    quality_desc = interpret_silhouette(analysis.silhouette_score)
    print(f"Clustering Quality: silhouette = {analysis.silhouette_score:.2f} ({quality_desc})")
    print(f"Optimal Clusters: {analysis.optimal_k}")
    print()

    # Group trajectories by seed for computing robustness
    traj_by_seed: dict[str, list[FitnessTrajectory]] = {}
    for t in trajectories:
        if t.seed_type == "known":
            if t.seed_genome_id not in traj_by_seed:
                traj_by_seed[t.seed_genome_id] = []
            traj_by_seed[t.seed_genome_id].append(t)

    # Print cluster details
    for cluster in analysis.clusters:
        print(f"Cluster {cluster.cluster_id}: {', '.join(cluster.genome_ids)}")
        print(f"  - Avg internal distance: {cluster.avg_internal_distance:.2f}")

        # Compute mutation robustness for this cluster
        cluster_trajs = []
        for gid in cluster.genome_ids:
            cluster_trajs.extend(traj_by_seed.get(gid, []))

        if cluster_trajs:
            initial_avg = sum(t.initial_fitness for t in cluster_trajs) / len(cluster_trajs)
            final_avg = sum(t.final_fitness for t in cluster_trajs) / len(cluster_trajs)
            if initial_avg > 0:
                robustness_pct = ((final_avg - initial_avg) / initial_avg) * 100
                print(f"  - Mutation robustness: {robustness_pct:+.0f}% after trajectory")
        print()

    # Valley analysis
    if analysis.valley_depths:
        print("Valley Analysis:")
        for (cid_a, cid_b), depth in sorted(analysis.valley_depths.items()):
            depth_desc = interpret_valley_depth(depth)
            print(f"  Cluster {cid_a} <-> Cluster {cid_b}: depth = {depth:.2f} ({depth_desc})")


def save_json(
    analysis: BasinAnalysis,
    trajectories: list[FitnessTrajectory],
    distance_matrix: np.ndarray,
    genome_labels: list[str],
    config: SamplingConfig,
    output_path: Path,
    baseline: Optional[BaselineStatistics] = None,
) -> None:
    """Save complete analysis data as JSON."""
    data = {
        "metadata": {
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "num_genomes": len(genome_labels),
            "num_random_baseline": sum(1 for t in trajectories if t.seed_type == "baseline"),
            "config": {
                "steps_per_path": config.steps_per_path,
                "paths_per_genome": config.paths_per_genome,
                "games_per_eval": config.games_per_eval,
            }
        },
        "distance_matrix": distance_matrix.tolist(),
        "genome_labels": genome_labels,
        "clusters": [
            {
                "id": c.cluster_id,
                "members": c.genome_ids,
                "centroid": c.centroid_genome_id,
                "avg_internal_distance": c.avg_internal_distance,
            }
            for c in analysis.clusters
        ],
        "clustering": {
            "silhouette_score": analysis.silhouette_score,
            "optimal_k": analysis.optimal_k,
        },
        "trajectories": [
            {
                "seed": t.seed_genome_id,
                "type": t.seed_type,
                "steps": t.steps,
            }
            for t in trajectories
        ],
        "valley_depths": {
            f"{k[0]},{k[1]}": v for k, v in analysis.valley_depths.items()
        },
    }

    if baseline is not None:
        data["baseline"] = {
            "known_mean_fitness": float(baseline.known_mean_fitness),
            "known_std_fitness": float(baseline.known_std_fitness),
            "random_mean_fitness": float(baseline.random_mean_fitness),
            "random_std_fitness": float(baseline.random_std_fitness),
            "fitness_pvalue": float(baseline.fitness_difference_pvalue),
            "known_decay_rate": float(baseline.known_mean_decay_rate),
            "random_decay_rate": float(baseline.random_mean_decay_rate),
            "decay_rate_pvalue": float(baseline.decay_rate_difference_pvalue),
            "known_mean_basin_radius": float(baseline.known_mean_basin_radius),
            "random_mean_basin_radius": float(baseline.random_mean_basin_radius),
            "known_games_are_special": bool(baseline.known_games_are_special),
        }

    output_path.parent.mkdir(parents=True, exist_ok=True)
    with open(output_path, 'w') as f:
        json.dump(data, f, indent=2)

    print(f"JSON saved to: {output_path}")


def plot_heatmap(
    distance_matrix: np.ndarray,
    labels: list[str],
    linkage_matrix: np.ndarray,
    output_path: Path
) -> None:
    """Clustered heatmap with dendrogram."""
    try:
        import matplotlib.pyplot as plt
        from scipy.cluster.hierarchy import dendrogram
    except ImportError:
        print("matplotlib not available, skipping heatmap")
        return

    fig, (ax_dendro, ax_heat) = plt.subplots(
        1, 2, figsize=(14, 8),
        gridspec_kw={'width_ratios': [1, 3]}
    )

    # Dendrogram
    dendro = dendrogram(
        linkage_matrix,
        orientation='left',
        labels=labels,
        ax=ax_dendro,
        leaf_font_size=8,
    )
    ax_dendro.set_xlabel('Distance')
    ax_dendro.set_title('Hierarchical Clustering')

    # Reorder matrix according to dendrogram
    order = dendro['leaves']
    reordered_matrix = distance_matrix[order][:, order]
    reordered_labels = [labels[i] for i in order]

    # Heatmap
    im = ax_heat.imshow(reordered_matrix, cmap='viridis', aspect='auto')
    ax_heat.set_xticks(range(len(reordered_labels)))
    ax_heat.set_yticks(range(len(reordered_labels)))
    ax_heat.set_xticklabels(reordered_labels, rotation=45, ha='right', fontsize=8)
    ax_heat.set_yticklabels(reordered_labels, fontsize=8)
    ax_heat.set_title('Structural Distance Matrix')

    fig.colorbar(im, ax=ax_heat, label='Distance')

    plt.tight_layout()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    plt.savefig(output_path, dpi=150, bbox_inches='tight')
    plt.close()

    print(f"Heatmap saved to: {output_path}")


def plot_trajectories(
    trajectories: list[FitnessTrajectory],
    output_path: Path
) -> None:
    """Fitness vs mutation step, faceted by seed genome."""
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("matplotlib not available, skipping trajectories plot")
        return

    # Group by seed
    by_seed: dict[str, list[FitnessTrajectory]] = {}
    for t in trajectories:
        key = f"{t.seed_genome_id} ({t.seed_type})"
        if key not in by_seed:
            by_seed[key] = []
        by_seed[key].append(t)

    # Determine grid size
    n_seeds = len(by_seed)
    n_cols = min(4, n_seeds)
    n_rows = (n_seeds + n_cols - 1) // n_cols

    fig, axes = plt.subplots(n_rows, n_cols, figsize=(4 * n_cols, 3 * n_rows))
    if n_seeds == 1:
        axes = [[axes]]
    elif n_rows == 1:
        axes = [axes]

    for idx, (seed_name, seed_trajs) in enumerate(sorted(by_seed.items())):
        row, col = idx // n_cols, idx % n_cols
        ax = axes[row][col]

        # Plot individual trajectories (light)
        for t in seed_trajs:
            ax.plot(t.steps, alpha=0.2, color='blue', linewidth=0.5)

        # Plot mean trajectory
        mean_traj = compute_mean_trajectory(seed_trajs)
        std_traj = compute_std_trajectory(seed_trajs)
        steps = list(range(len(mean_traj)))

        ax.plot(steps, mean_traj, color='blue', linewidth=2, label='Mean')

        # Confidence band
        lower = [m - s for m, s in zip(mean_traj, std_traj)]
        upper = [m + s for m, s in zip(mean_traj, std_traj)]
        ax.fill_between(steps, lower, upper, alpha=0.3, color='blue')

        ax.set_title(seed_name, fontsize=10)
        ax.set_xlabel('Mutation Step')
        ax.set_ylabel('Fitness')
        ax.set_ylim(0, 1)

    # Hide empty subplots
    for idx in range(n_seeds, n_rows * n_cols):
        row, col = idx // n_cols, idx % n_cols
        axes[row][col].set_visible(False)

    plt.tight_layout()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    plt.savefig(output_path, dpi=150, bbox_inches='tight')
    plt.close()

    print(f"Trajectories plot saved to: {output_path}")


def plot_basin_scatter(
    distance_matrix: np.ndarray,
    labels: list[str],
    cluster_assignments: dict[str, int],
    output_path: Path
) -> None:
    """2D MDS projection colored by cluster."""
    try:
        import matplotlib.pyplot as plt
        from sklearn.manifold import MDS
    except ImportError:
        print("matplotlib/sklearn not available, skipping basin scatter")
        return

    # MDS projection
    mds = MDS(n_components=2, dissimilarity='precomputed', random_state=42)
    coords = mds.fit_transform(distance_matrix)

    # Get cluster colors
    cluster_ids = [cluster_assignments.get(label, 0) for label in labels]
    unique_clusters = sorted(set(cluster_ids))
    colors = plt.cm.tab10(np.linspace(0, 1, len(unique_clusters)))
    color_map = {cid: colors[i] for i, cid in enumerate(unique_clusters)}

    fig, ax = plt.subplots(figsize=(10, 8))

    for i, (label, cid) in enumerate(zip(labels, cluster_ids)):
        ax.scatter(
            coords[i, 0], coords[i, 1],
            c=[color_map[cid]], s=100, alpha=0.7,
            label=f"Cluster {cid}" if i == cluster_ids.index(cid) else None
        )
        ax.annotate(label, (coords[i, 0], coords[i, 1]), fontsize=8, alpha=0.8)

    ax.set_xlabel('MDS Dimension 1')
    ax.set_ylabel('MDS Dimension 2')
    ax.set_title('Basin Structure (MDS Projection)')
    ax.legend(loc='best')

    plt.tight_layout()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    plt.savefig(output_path, dpi=150, bbox_inches='tight')
    plt.close()

    print(f"Basin scatter saved to: {output_path}")


def plot_baseline_comparison(
    known_trajectories: list[FitnessTrajectory],
    random_trajectories: list[FitnessTrajectory],
    baseline_stats: BaselineStatistics,
    output_path: Path
) -> None:
    """
    Visualize known vs random genome trajectory comparison.

    Creates 2x2 subplot:
    - Top-left: Box plot of initial fitness (known vs random)
    - Top-right: Histogram of decay rates
    - Bottom-left: Mean trajectory curves with confidence bands
    - Bottom-right: Basin radius distribution
    """
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("matplotlib not available, skipping baseline comparison plot")
        return

    from darwindeck.analysis.random_baseline import compute_decay_rate, compute_basin_radius

    fig, axes = plt.subplots(2, 2, figsize=(12, 10))

    # Top-left: Box plot of initial fitness
    ax = axes[0, 0]
    known_initial = [t.initial_fitness for t in known_trajectories]
    random_initial = [t.initial_fitness for t in random_trajectories]
    ax.boxplot([known_initial, random_initial], labels=['Known Games', 'Random Genomes'])
    ax.set_ylabel('Initial Fitness')
    ax.set_title('Starting Fitness Distribution')
    sig_str = f"p={baseline_stats.fitness_difference_pvalue:.4f}"
    ax.text(0.5, 0.95, sig_str, transform=ax.transAxes, ha='center', fontsize=10)

    # Top-right: Histogram of decay rates
    ax = axes[0, 1]
    known_decay = [compute_decay_rate(t) for t in known_trajectories]
    random_decay = [compute_decay_rate(t) for t in random_trajectories]
    ax.hist(known_decay, bins=20, alpha=0.5, label='Known', color='blue')
    ax.hist(random_decay, bins=20, alpha=0.5, label='Random', color='orange')
    ax.axvline(baseline_stats.known_mean_decay_rate, color='blue', linestyle='--', linewidth=2)
    ax.axvline(baseline_stats.random_mean_decay_rate, color='orange', linestyle='--', linewidth=2)
    ax.set_xlabel('Decay Rate (fitness/step)')
    ax.set_ylabel('Count')
    ax.set_title('Fitness Decay Rate Distribution')
    ax.legend()

    # Bottom-left: Mean trajectory curves
    ax = axes[1, 0]
    known_mean = compute_mean_trajectory(known_trajectories)
    known_std = compute_std_trajectory(known_trajectories)
    random_mean = compute_mean_trajectory(random_trajectories)
    random_std = compute_std_trajectory(random_trajectories)

    steps_known = list(range(len(known_mean)))
    steps_random = list(range(len(random_mean)))

    ax.plot(steps_known, known_mean, color='blue', linewidth=2, label='Known Games')
    ax.fill_between(
        steps_known,
        [m - s for m, s in zip(known_mean, known_std)],
        [m + s for m, s in zip(known_mean, known_std)],
        alpha=0.3, color='blue'
    )

    ax.plot(steps_random, random_mean, color='orange', linewidth=2, label='Random Genomes')
    ax.fill_between(
        steps_random,
        [m - s for m, s in zip(random_mean, random_std)],
        [m + s for m, s in zip(random_mean, random_std)],
        alpha=0.3, color='orange'
    )

    ax.set_xlabel('Mutation Step')
    ax.set_ylabel('Fitness')
    ax.set_title('Mean Fitness Trajectory')
    ax.set_ylim(0, 1)
    ax.legend()

    # Bottom-right: Basin radius distribution
    ax = axes[1, 1]
    known_radii = [compute_basin_radius(t) for t in known_trajectories]
    random_radii = [compute_basin_radius(t) for t in random_trajectories]
    ax.boxplot([known_radii, random_radii], labels=['Known Games', 'Random Genomes'])
    ax.set_ylabel('Basin Radius (mutations)')
    ax.set_title('Basin Radius Distribution')
    ax.text(
        0.5, 0.95,
        f"Known: {baseline_stats.known_mean_basin_radius:.1f}, Random: {baseline_stats.random_mean_basin_radius:.1f}",
        transform=ax.transAxes, ha='center', fontsize=10
    )

    plt.tight_layout()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    plt.savefig(output_path, dpi=150, bbox_inches='tight')
    plt.close()

    print(f"Baseline comparison plot saved to: {output_path}")
