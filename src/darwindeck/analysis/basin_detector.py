"""Clustering and valley detection algorithms for basin analysis."""

from __future__ import annotations

import numpy as np
from dataclasses import dataclass
from typing import Optional

from scipy.cluster.hierarchy import linkage, fcluster
from scipy.spatial.distance import squareform
from sklearn.metrics import silhouette_score

from darwindeck.analysis.mutation_sampler import FitnessTrajectory


class ClusteringError(Exception):
    """Clustering algorithm failed."""
    pass


@dataclass
class BasinCluster:
    """A detected basin (cluster of related games)."""
    cluster_id: int
    genome_ids: list[str]
    centroid_genome_id: str  # Most central member
    avg_internal_distance: float

    @property
    def size(self) -> int:
        """Number of genomes in this cluster."""
        return len(self.genome_ids)


@dataclass
class BasinAnalysis:
    """Complete basin detection results."""
    clusters: list[BasinCluster]
    cluster_assignments: dict[str, int]  # genome_id -> cluster_id
    silhouette_score: float  # Clustering quality [-1, 1]
    linkage_matrix: np.ndarray  # For dendrogram
    valley_depths: dict[tuple[str, str], float]  # Pairwise valley depths
    optimal_k: int  # Optimal number of clusters

    def get_cluster(self, genome_id: str) -> Optional[BasinCluster]:
        """Get the cluster containing a specific genome."""
        cluster_id = self.cluster_assignments.get(genome_id)
        if cluster_id is None:
            return None
        for cluster in self.clusters:
            if cluster.cluster_id == cluster_id:
                return cluster
        return None


def _find_centroid(
    cluster_indices: list[int],
    distance_matrix: np.ndarray
) -> int:
    """Find the most central member of a cluster (minimum sum of distances)."""
    if len(cluster_indices) == 1:
        return cluster_indices[0]

    min_sum = float('inf')
    centroid_idx = cluster_indices[0]

    for i in cluster_indices:
        dist_sum = sum(distance_matrix[i, j] for j in cluster_indices if i != j)
        if dist_sum < min_sum:
            min_sum = dist_sum
            centroid_idx = i

    return centroid_idx


def _compute_avg_internal_distance(
    cluster_indices: list[int],
    distance_matrix: np.ndarray
) -> float:
    """Compute average pairwise distance within a cluster."""
    if len(cluster_indices) < 2:
        return 0.0

    total = 0.0
    count = 0
    for i, idx_i in enumerate(cluster_indices):
        for idx_j in cluster_indices[i + 1:]:
            total += distance_matrix[idx_i, idx_j]
            count += 1

    return total / count if count > 0 else 0.0


def detect_basins(
    distance_matrix: np.ndarray,
    genome_labels: list[str],
    method: str = "ward",
    max_clusters: Optional[int] = None
) -> BasinAnalysis:
    """
    Detect basins using hierarchical clustering.

    Automatically selects optimal cluster count using silhouette score.

    Args:
        distance_matrix: Pairwise distances (symmetric, zero diagonal)
        genome_labels: Names for each genome
        method: Linkage method ('ward', 'complete', 'average')
        max_clusters: Maximum clusters to consider (default: n_genomes // 2)

    Returns:
        BasinAnalysis with clusters and quality metrics

    Raises:
        ClusteringError: If clustering fails
    """
    n = len(genome_labels)

    if n < 2:
        raise ClusteringError("Need at least 2 genomes for clustering")

    if distance_matrix.shape != (n, n):
        raise ClusteringError(
            f"Distance matrix shape {distance_matrix.shape} doesn't match "
            f"{n} genome labels"
        )

    try:
        # Convert to condensed form for linkage
        condensed = squareform(distance_matrix)

        # Perform hierarchical clustering
        Z = linkage(condensed, method=method)

    except Exception as e:
        raise ClusteringError(f"Linkage computation failed: {e}")

    # Find optimal cluster count via silhouette score
    max_k = max_clusters or max(2, n // 2)
    max_k = min(max_k, n - 1)  # Can't have more clusters than samples - 1

    best_k = 2
    best_score = -1.0

    for k in range(2, max_k + 1):
        try:
            labels = fcluster(Z, k, criterion='maxclust')
            # Need at least 2 samples per cluster for silhouette
            if len(set(labels)) < 2:
                continue
            score = silhouette_score(distance_matrix, labels, metric='precomputed')
            if score > best_score:
                best_k, best_score = k, score
        except Exception:
            continue

    # Get final cluster assignments
    final_labels = fcluster(Z, best_k, criterion='maxclust')

    # Build cluster objects
    cluster_assignments: dict[str, int] = {}
    clusters_dict: dict[int, list[int]] = {}  # cluster_id -> indices

    for i, (label, genome_id) in enumerate(zip(final_labels, genome_labels)):
        cluster_id = int(label)
        cluster_assignments[genome_id] = cluster_id
        if cluster_id not in clusters_dict:
            clusters_dict[cluster_id] = []
        clusters_dict[cluster_id].append(i)

    clusters: list[BasinCluster] = []
    for cluster_id, indices in sorted(clusters_dict.items()):
        centroid_idx = _find_centroid(indices, distance_matrix)
        avg_dist = _compute_avg_internal_distance(indices, distance_matrix)

        cluster = BasinCluster(
            cluster_id=cluster_id,
            genome_ids=[genome_labels[i] for i in indices],
            centroid_genome_id=genome_labels[centroid_idx],
            avg_internal_distance=avg_dist,
        )
        clusters.append(cluster)

    return BasinAnalysis(
        clusters=clusters,
        cluster_assignments=cluster_assignments,
        silhouette_score=best_score,
        linkage_matrix=Z,
        valley_depths={},  # Computed separately
        optimal_k=best_k,
    )


def compute_valley_depths(
    trajectories: list[FitnessTrajectory],
    analysis: BasinAnalysis,
) -> dict[tuple[str, str], float]:
    """
    Estimate valley depth between each pair of clusters.

    Valley depth = max(cluster_fitness_a, cluster_fitness_b) - min(fitness between)

    Args:
        trajectories: Fitness trajectories from seed genomes
        analysis: Basin analysis with cluster assignments

    Returns:
        Dictionary mapping cluster pairs to valley depths
    """
    # Group trajectories by cluster
    cluster_trajectories: dict[int, list[FitnessTrajectory]] = {}

    for traj in trajectories:
        cluster_id = analysis.cluster_assignments.get(traj.seed_genome_id)
        if cluster_id is not None:
            if cluster_id not in cluster_trajectories:
                cluster_trajectories[cluster_id] = []
            cluster_trajectories[cluster_id].append(traj)

    # Compute average initial fitness per cluster
    cluster_fitness: dict[int, float] = {}
    for cluster_id, trajs in cluster_trajectories.items():
        initial_fitnesses = [t.initial_fitness for t in trajs]
        cluster_fitness[cluster_id] = (
            sum(initial_fitnesses) / len(initial_fitnesses) if initial_fitnesses else 0.0
        )

    # Compute valley depths between cluster pairs
    valley_depths: dict[tuple[str, str], float] = {}
    cluster_ids = sorted(cluster_trajectories.keys())

    for i, cid_a in enumerate(cluster_ids):
        for cid_b in cluster_ids[i + 1:]:
            # Valley depth estimation:
            # Look at final fitness of trajectories from each cluster
            # The minimum final fitness indicates how low you'd have to go
            # to travel between clusters

            trajs_a = cluster_trajectories[cid_a]
            trajs_b = cluster_trajectories[cid_b]

            # Use minimum final fitness as proxy for valley floor
            min_final_a = min(t.final_fitness for t in trajs_a) if trajs_a else 0.0
            min_final_b = min(t.final_fitness for t in trajs_b) if trajs_b else 0.0
            valley_floor = min(min_final_a, min_final_b)

            # Peak is the higher of the two cluster starting fitnesses
            peak = max(cluster_fitness.get(cid_a, 0.0), cluster_fitness.get(cid_b, 0.0))

            # Valley depth is peak - floor
            depth = max(0.0, peak - valley_floor)

            # Store with cluster IDs as key
            key = (str(cid_a), str(cid_b))
            valley_depths[key] = depth

    return valley_depths


def interpret_silhouette(score: float) -> str:
    """Interpret silhouette score quality."""
    if score >= 0.7:
        return "strong separation"
    elif score >= 0.5:
        return "good separation"
    elif score >= 0.25:
        return "weak separation"
    else:
        return "poor separation (clusters may overlap)"


def interpret_valley_depth(depth: float) -> str:
    """Interpret valley depth."""
    if depth >= 0.4:
        return "deep valley"
    elif depth >= 0.2:
        return "moderate valley"
    elif depth >= 0.1:
        return "shallow valley"
    else:
        return "negligible valley"
