# Basin Analysis Tool Design (Revised)

**Date:** 2026-01-13
**Revision:** 3 (adds random genome baseline comparison)

## Motivation

Observation from evolution runs: mutations follow a gentle slope near known good games. This suggests:
- A gradient exists for the GA to follow
- Known games are in "good neighborhoods"
- Hill-climbing from seeds works

Key question: Are all known games in the same fitness basin, or are there multiple peaks with valleys between them?

**Baseline question:** Are known games actually in special neighborhoods, or is typical genome space already navigable? We need random genome baselines to validate that known games are meaningfully different from arbitrary starting points.

## Architecture (Revised)

**Modular design** instead of monolithic script:

```
src/darwindeck/analysis/
├── __init__.py
├── genome_distance.py    # Structural distance calculations
├── mutation_sampler.py   # Mutation path sampling with fitness tracking
├── random_baseline.py    # Random genome generation for baseline comparison
├── basin_detector.py     # Clustering and valley detection algorithms
└── basin_report.py       # Output generation (terminal, JSON, plots)

scripts/
└── analyze_basins.py     # CLI entry point (thin wrapper)
```

**Data flow:**
```
CLI args → validate_inputs() → compute_distance_matrix() → sample_mutation_paths()
    → generate_random_baseline() → compute_baseline_statistics()
    → detect_basins() → generate_reports()
```

## Module 1: Genome Distance (`genome_distance.py`)

### Field Weights (Configurable)

```python
DEFAULT_FIELD_WEIGHTS = {
    "cards_per_player": 1,      # Setup difference
    "starting_chips": 1,        # Betting vs non-betting
    "player_count": 2,          # Fundamental structure
    "phase_types": 3,           # Core mechanics (set comparison)
    "win_condition_types": 3,   # How you win
    "special_effects_count": 1, # Complexity
    "is_trick_based": 2,        # Major divide
}
```

### Distance Function

```python
def structural_distance(
    genome_a: GameGenome,
    genome_b: GameGenome,
    weights: dict[str, float] | None = None
) -> float:
    """
    Compute normalized structural distance between genomes.

    Args:
        genome_a: First genome
        genome_b: Second genome
        weights: Field weights (uses DEFAULT_FIELD_WEIGHTS if None)

    Returns:
        Distance in [0.0, 1.0] where 0 = identical, 1 = maximally different

    Raises:
        ValueError: If genomes have incompatible schema versions
    """
    weights = weights or DEFAULT_FIELD_WEIGHTS
    score = 0.0
    max_score = sum(weights.values())

    for field, weight in weights.items():
        if _field_differs(genome_a, genome_b, field):
            score += weight

    return score / max_score

def _field_differs(a: GameGenome, b: GameGenome, field: str) -> bool:
    """Compare field with type-appropriate logic."""
    if field == "phase_types":
        # Set comparison for phases
        types_a = {type(p).__name__ for p in a.turn_structure.phases}
        types_b = {type(p).__name__ for p in b.turn_structure.phases}
        return types_a != types_b
    elif field == "win_condition_types":
        types_a = {wc.type for wc in a.win_conditions}
        types_b = {wc.type for wc in b.win_conditions}
        return types_a != types_b
    # ... other fields use direct comparison
```

### Pairwise Matrix

```python
def compute_distance_matrix(
    genomes: list[GameGenome],
    weights: dict[str, float] | None = None
) -> tuple[np.ndarray, list[str]]:
    """
    Compute symmetric distance matrix for all genome pairs.

    Returns:
        (matrix, labels) where matrix[i,j] = distance between genomes i and j
    """
```

## Module 2: Mutation Sampler (`mutation_sampler.py`)

### Configuration

```python
@dataclass
class SamplingConfig:
    """Configuration for mutation path sampling."""
    steps_per_path: int = 50        # Mutations per trajectory
    paths_per_genome: int = 25      # Independent paths per seed
    games_per_eval: int = 50        # Simulations for fitness estimate
    random_seeds: list[int] | None = None  # For reproducibility

    def validate(self) -> None:
        """Raise ValueError if config is invalid."""
        if self.steps_per_path < 1:
            raise ValueError("steps_per_path must be >= 1")
        if self.paths_per_genome < 1:
            raise ValueError("paths_per_genome must be >= 1")
```

### Trajectory Sampling

```python
@dataclass
class FitnessTrajectory:
    """A single mutation path with fitness at each step."""
    seed_genome_id: str
    random_seed: int
    steps: list[float]  # fitness at each mutation step
    final_genome: GameGenome  # Endpoint for analysis

def sample_trajectories(
    seed_genomes: list[GameGenome],
    config: SamplingConfig,
    evaluator: FitnessEvaluator,
    progress_callback: Callable[[int, int], None] | None = None
) -> list[FitnessTrajectory]:
    """
    Sample mutation paths from each seed genome.

    Args:
        seed_genomes: Starting genomes
        config: Sampling parameters
        evaluator: Fitness evaluator instance
        progress_callback: Optional (current, total) progress reporter

    Returns:
        List of trajectories (len = seeds * paths_per_genome)

    Raises:
        RuntimeError: If fitness evaluation fails consistently
    """
```

## Module 3: Random Baseline (`random_baseline.py`)

Provides null hypothesis comparison to validate that known games are in special neighborhoods.

### Random Genome Generation

```python
@dataclass
class BaselineConfig:
    """Configuration for random baseline generation."""
    num_random_genomes: int = 20       # How many random genomes to sample
    require_playable: bool = True       # Filter to genomes that complete without errors
    max_generation_attempts: int = 100  # Attempts before giving up on a genome

def generate_random_genomes(
    config: BaselineConfig,
    evaluator: FitnessEvaluator,
    random_seed: int | None = None
) -> list[GameGenome]:
    """
    Generate random valid genomes for baseline comparison.

    Args:
        config: Generation parameters
        evaluator: For playability validation
        random_seed: For reproducibility

    Returns:
        List of random genomes that pass playability check

    Notes:
        Uses existing mutation operators with maximal randomization.
        Filters out genomes with >50% error rate in quick simulation.
    """
```

### Baseline Statistics

```python
@dataclass
class BaselineStatistics:
    """Comparison statistics between known and random genomes."""
    known_mean_fitness: float
    known_std_fitness: float
    random_mean_fitness: float
    random_std_fitness: float

    # Trajectory statistics
    known_mean_decay_rate: float    # Fitness loss per mutation step
    random_mean_decay_rate: float

    # Statistical tests
    fitness_difference_pvalue: float  # Mann-Whitney U test
    decay_rate_difference_pvalue: float

    @property
    def known_games_are_special(self) -> bool:
        """Returns True if known games are significantly better starting points."""
        return (
            self.fitness_difference_pvalue < 0.05 and
            self.known_mean_fitness > self.random_mean_fitness
        )

def compute_baseline_statistics(
    known_trajectories: list[FitnessTrajectory],
    random_trajectories: list[FitnessTrajectory]
) -> BaselineStatistics:
    """
    Compare mutation trajectories from known games vs random genomes.

    Tests hypotheses:
    1. Known games have higher initial fitness than random
    2. Known games have gentler fitness decay under mutation
    3. Known games are in larger basins (can wander further before falling off)
    """
```

### Decay Rate Analysis

```python
def compute_decay_rate(trajectory: FitnessTrajectory) -> float:
    """
    Compute fitness decay rate for a trajectory.

    Returns slope of linear regression: fitness ~ mutation_step
    Negative values indicate fitness decreasing with mutations.
    """

def compute_basin_radius(
    trajectory: FitnessTrajectory,
    threshold: float = 0.1
) -> int:
    """
    Estimate basin radius: mutations before fitness drops by threshold.

    Args:
        trajectory: Fitness trajectory from seed
        threshold: Relative fitness drop that defines "leaving basin"

    Returns:
        Number of mutations before fitness drops by threshold fraction
    """
```

## Module 4: Basin Detector (`basin_detector.py`)

### Clustering Algorithm

Uses **hierarchical clustering** with automatic cluster count selection:

```python
from scipy.cluster.hierarchy import linkage, fcluster
from sklearn.metrics import silhouette_score

@dataclass
class BasinCluster:
    """A detected basin (cluster of related games)."""
    cluster_id: int
    genome_ids: list[str]
    centroid_genome_id: str  # Most central member
    avg_internal_distance: float

@dataclass
class BasinAnalysis:
    """Complete basin detection results."""
    clusters: list[BasinCluster]
    cluster_assignments: dict[str, int]  # genome_id -> cluster_id
    silhouette_score: float  # Clustering quality [-1, 1]
    linkage_matrix: np.ndarray  # For dendrogram
    valley_depths: dict[tuple[str, str], float]  # Pairwise valley depths

def detect_basins(
    distance_matrix: np.ndarray,
    genome_labels: list[str],
    method: str = "ward",
    max_clusters: int | None = None
) -> BasinAnalysis:
    """
    Detect basins using hierarchical clustering.

    Automatically selects optimal cluster count using silhouette score.

    Args:
        distance_matrix: Pairwise distances
        genome_labels: Names for each genome
        method: Linkage method ('ward', 'complete', 'average')
        max_clusters: Maximum clusters to consider (default: n_genomes // 2)

    Returns:
        BasinAnalysis with clusters and quality metrics
    """
    Z = linkage(squareform(distance_matrix), method=method)

    # Find optimal cluster count via silhouette score
    best_k, best_score = 2, -1
    max_k = max_clusters or len(genome_labels) // 2

    for k in range(2, max_k + 1):
        labels = fcluster(Z, k, criterion='maxclust')
        score = silhouette_score(distance_matrix, labels, metric='precomputed')
        if score > best_score:
            best_k, best_score = k, score

    # ... build BasinAnalysis
```

### Valley Detection

```python
def compute_valley_depths(
    trajectories: list[FitnessTrajectory],
    genome_labels: list[str]
) -> dict[tuple[str, str], float]:
    """
    Estimate valley depth between each pair of seed genomes.

    Valley depth = max(fitness_a, fitness_b) - min(fitness on paths between)

    Uses trajectory endpoints to estimate inter-basin fitness.
    """
```

## Module 5: Basin Report (`basin_report.py`)

### Terminal Output

```python
def print_summary(
    analysis: BasinAnalysis,
    trajectories: list[FitnessTrajectory],
    baseline: BaselineStatistics | None = None
) -> None:
    """Print human-readable basin analysis summary."""
```

Output format:
```
=== Basin Analysis Report ===

--- Baseline Comparison ---
Known games vs random genomes:
  Known mean fitness: 0.72 ± 0.08
  Random mean fitness: 0.31 ± 0.15
  Difference: p < 0.001 (SIGNIFICANT)

  Known decay rate: -0.004/step (gentle slope)
  Random decay rate: -0.018/step (steep cliff)
  Basin radius: 42 mutations (known) vs 8 mutations (random)

Conclusion: Known games ARE in special neighborhoods ✓

--- Clustering Analysis ---
Clustering Quality: silhouette = 0.72 (good separation)
Optimal Clusters: 3

Cluster 1 (Simple Games): War, Go-Fish, Old-Maid
  - Avg internal distance: 0.18
  - Mutation robustness: -28% after 50 mutations

Cluster 2 (Trick-Taking): Hearts, Spades, Whist
  - Avg internal distance: 0.21
  - Mutation robustness: -35% after 50 mutations

Cluster 3 (Betting): Poker, Blackjack, Betting-War
  - Avg internal distance: 0.24
  - Mutation robustness: -42% after 50 mutations

Valley Analysis:
  Cluster 1 ↔ Cluster 2: depth = 0.31 (moderate valley)
  Cluster 1 ↔ Cluster 3: depth = 0.45 (deep valley)
  Cluster 2 ↔ Cluster 3: depth = 0.28 (shallow valley)
```

### JSON Output

```python
def save_json(
    analysis: BasinAnalysis,
    trajectories: list[FitnessTrajectory],
    distance_matrix: np.ndarray,
    output_path: Path
) -> None:
    """Save complete analysis data as JSON."""
```

Schema:
```json
{
  "metadata": {
    "timestamp": "2026-01-13T10:00:00Z",
    "num_genomes": 18,
    "num_random_baseline": 20,
    "config": { "steps": 50, "paths": 25 }
  },
  "baseline": {
    "known_mean_fitness": 0.72,
    "known_std_fitness": 0.08,
    "random_mean_fitness": 0.31,
    "random_std_fitness": 0.15,
    "fitness_pvalue": 0.0001,
    "known_decay_rate": -0.004,
    "random_decay_rate": -0.018,
    "decay_rate_pvalue": 0.003,
    "known_games_are_special": true
  },
  "distance_matrix": [[0.0, 0.12, ...], ...],
  "genome_labels": ["war", "hearts", ...],
  "clusters": [
    {"id": 1, "members": ["war", "go-fish"], "silhouette": 0.72}
  ],
  "trajectories": [
    {"seed": "war", "type": "known", "steps": [0.58, 0.55, 0.51, ...]},
    {"seed": "random_001", "type": "baseline", "steps": [0.31, 0.28, 0.19, ...]}
  ],
  "valley_depths": {"war,hearts": 0.31, ...}
}
```

### Matplotlib Visualizations

```python
def plot_heatmap(
    distance_matrix: np.ndarray,
    labels: list[str],
    linkage_matrix: np.ndarray,
    output_path: Path
) -> None:
    """Clustered heatmap with dendrogram."""

def plot_trajectories(
    trajectories: list[FitnessTrajectory],
    output_path: Path
) -> None:
    """Fitness vs mutation step, faceted by seed genome."""

def plot_basin_scatter(
    distance_matrix: np.ndarray,
    labels: list[str],
    cluster_assignments: dict[str, int],
    output_path: Path
) -> None:
    """2D MDS projection colored by cluster."""

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
```

## CLI Interface (`scripts/analyze_basins.py`)

```python
def main():
    parser = argparse.ArgumentParser(description="Analyze fitness basin structure")

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
    parser.add_argument("--seed", type=int, default=None,
                        help="Random seed for reproducibility")

    # Execution
    parser.add_argument("--dry-run", action="store_true",
                        help="Validate config without running")
    parser.add_argument("-v", "--verbose", action="store_true",
                        help="Verbose output")
```

## Error Handling

All modules implement consistent error handling:

```python
class BasinAnalysisError(Exception):
    """Base exception for basin analysis."""
    pass

class InvalidGenomeError(BasinAnalysisError):
    """Genome failed validation."""
    pass

class EvaluationError(BasinAnalysisError):
    """Fitness evaluation failed."""
    pass

class ClusteringError(BasinAnalysisError):
    """Clustering algorithm failed."""
    pass
```

Each function:
1. Validates inputs before processing
2. Catches and wraps low-level exceptions
3. Provides actionable error messages
4. Logs warnings for non-fatal issues

## Testing Strategy

```
tests/unit/analysis/
├── test_genome_distance.py   # Distance calculations
├── test_mutation_sampler.py  # Trajectory sampling
├── test_random_baseline.py   # Random generation and statistics
├── test_basin_detector.py    # Clustering logic
└── test_basin_report.py      # Output formatting

tests/integration/
└── test_basin_analysis.py    # End-to-end with small sample
```

Key test cases:
- Distance metric symmetry and triangle inequality
- Trajectory reproducibility with fixed seeds
- Random genome generation produces valid playable games
- Baseline statistics correctly identify known games as special (with synthetic data)
- Statistical tests have correct p-values with known distributions
- Clustering stability with noisy data
- Report generation with edge cases (1 cluster, all same distance, no baseline)

## Usage

```bash
# Default analysis
python scripts/analyze_basins.py

# Custom config
python scripts/analyze_basins.py --steps 100 --paths 50 --output-dir output/deep_analysis

# Reproducible run
python scripts/analyze_basins.py --seed 42 --verbose

# Quick validation
python scripts/analyze_basins.py --dry-run
```

## Estimated Runtime

| Config | Evaluations | Time (est.) |
|--------|-------------|-------------|
| Default (50×25×18 + 20 baseline) | 47,500 | ~25 min |
| Deep (100×50×18 + 20 baseline) | 140,000 | ~70 min |
| Quick (20×10×18 + 10 baseline) | 6,200 | ~4 min |
| No baseline (50×25×18) | 22,500 | ~12 min |

Note: Baseline adds ~1,250 evaluations per random genome (50 steps × 25 paths).
