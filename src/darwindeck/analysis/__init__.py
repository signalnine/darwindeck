"""Basin analysis tools for fitness landscape exploration.

The analysis package includes optional simulation-backed tools.  Keep its
public exports lazy so importing a pure analysis submodule does not require the
CGo simulator (and its local shared library) to be available.
"""

from __future__ import annotations

from importlib import import_module
from typing import Any


_EXPORT_MODULES = {
    # Distance
    "structural_distance": "genome_distance",
    "compute_distance_matrix": "genome_distance",
    "DEFAULT_FIELD_WEIGHTS": "genome_distance",
    # Sampling
    "SamplingConfig": "mutation_sampler",
    "FitnessTrajectory": "mutation_sampler",
    "sample_trajectories": "mutation_sampler",
    "sample_trajectories_parallel": "mutation_sampler",
    # Baseline
    "BaselineConfig": "random_baseline",
    "BaselineStatistics": "random_baseline",
    "generate_random_genomes": "random_baseline",
    "compute_baseline_statistics": "random_baseline",
    # Detection
    "BasinCluster": "basin_detector",
    "BasinAnalysis": "basin_detector",
    "detect_basins": "basin_detector",
    "compute_valley_depths": "basin_detector",
    # Reporting
    "print_summary": "basin_report",
    "save_json": "basin_report",
    "plot_heatmap": "basin_report",
    "plot_trajectories": "basin_report",
    "plot_basin_scatter": "basin_report",
    "plot_baseline_comparison": "basin_report",
}

__all__ = list(_EXPORT_MODULES)


def __getattr__(name: str) -> Any:
    """Load public analysis helpers only when callers request them."""
    module_name = _EXPORT_MODULES.get(name)
    if module_name is None:
        raise AttributeError(f"module {__name__!r} has no attribute {name!r}")

    value = getattr(import_module(f"{__name__}.{module_name}"), name)
    globals()[name] = value
    return value


def __dir__() -> list[str]:
    return sorted(set(globals()) | set(__all__))
