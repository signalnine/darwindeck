"""Basin analysis tools for fitness landscape exploration."""

from darwindeck.analysis.genome_distance import (
    structural_distance,
    compute_distance_matrix,
    DEFAULT_FIELD_WEIGHTS,
)
from darwindeck.analysis.mutation_sampler import (
    SamplingConfig,
    FitnessTrajectory,
    sample_trajectories,
)
from darwindeck.analysis.random_baseline import (
    BaselineConfig,
    BaselineStatistics,
    generate_random_genomes,
    compute_baseline_statistics,
)
from darwindeck.analysis.basin_detector import (
    BasinCluster,
    BasinAnalysis,
    detect_basins,
    compute_valley_depths,
)
from darwindeck.analysis.basin_report import (
    print_summary,
    save_json,
    plot_heatmap,
    plot_trajectories,
    plot_basin_scatter,
    plot_baseline_comparison,
)

__all__ = [
    # Distance
    "structural_distance",
    "compute_distance_matrix",
    "DEFAULT_FIELD_WEIGHTS",
    # Sampling
    "SamplingConfig",
    "FitnessTrajectory",
    "sample_trajectories",
    # Baseline
    "BaselineConfig",
    "BaselineStatistics",
    "generate_random_genomes",
    "compute_baseline_statistics",
    # Detection
    "BasinCluster",
    "BasinAnalysis",
    "detect_basins",
    "compute_valley_depths",
    # Reporting
    "print_summary",
    "save_json",
    "plot_heatmap",
    "plot_trajectories",
    "plot_basin_scatter",
    "plot_baseline_comparison",
]
