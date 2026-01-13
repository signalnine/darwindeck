"""Structural distance calculations between game genomes."""

from __future__ import annotations

import numpy as np
from dataclasses import dataclass

from darwindeck.genome.schema import GameGenome


DEFAULT_FIELD_WEIGHTS: dict[str, float] = {
    "cards_per_player": 1.0,      # Setup difference
    "starting_chips": 1.0,        # Betting vs non-betting
    "player_count": 2.0,          # Fundamental structure
    "phase_types": 3.0,           # Core mechanics (set comparison)
    "win_condition_types": 3.0,   # How you win
    "special_effects_count": 1.0, # Complexity
    "is_trick_based": 2.0,        # Major divide
}


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
    if genome_a.schema_version != genome_b.schema_version:
        raise ValueError(
            f"Incompatible schema versions: {genome_a.schema_version} vs {genome_b.schema_version}"
        )

    weights = weights or DEFAULT_FIELD_WEIGHTS
    score = 0.0
    max_score = sum(weights.values())

    for field_name, weight in weights.items():
        if _field_differs(genome_a, genome_b, field_name):
            score += weight

    return score / max_score if max_score > 0 else 0.0


def _field_differs(a: GameGenome, b: GameGenome, field: str) -> bool:
    """Compare field with type-appropriate logic."""
    if field == "phase_types":
        # Set comparison for phase types
        types_a = {type(p).__name__ for p in a.turn_structure.phases}
        types_b = {type(p).__name__ for p in b.turn_structure.phases}
        return types_a != types_b

    elif field == "win_condition_types":
        # Set comparison for win condition types
        types_a = {wc.type for wc in a.win_conditions}
        types_b = {wc.type for wc in b.win_conditions}
        return types_a != types_b

    elif field == "cards_per_player":
        return a.setup.cards_per_player != b.setup.cards_per_player

    elif field == "starting_chips":
        # Binary: has betting vs no betting
        has_betting_a = a.setup.starting_chips > 0
        has_betting_b = b.setup.starting_chips > 0
        return has_betting_a != has_betting_b

    elif field == "player_count":
        return a.player_count != b.player_count

    elif field == "special_effects_count":
        # Compare by count buckets: 0, 1-2, 3+
        count_a = len(a.special_effects) if a.special_effects else 0
        count_b = len(b.special_effects) if b.special_effects else 0
        bucket_a = 0 if count_a == 0 else (1 if count_a <= 2 else 2)
        bucket_b = 0 if count_b == 0 else (1 if count_b <= 2 else 2)
        return bucket_a != bucket_b

    elif field == "is_trick_based":
        return a.turn_structure.is_trick_based != b.turn_structure.is_trick_based

    else:
        # Unknown field - treat as no difference
        return False


def compute_distance_matrix(
    genomes: list[GameGenome],
    weights: dict[str, float] | None = None
) -> tuple[np.ndarray, list[str]]:
    """
    Compute symmetric distance matrix for all genome pairs.

    Args:
        genomes: List of genomes to compare
        weights: Field weights for distance calculation

    Returns:
        (matrix, labels) where matrix[i,j] = distance between genomes i and j
        Labels are genome_id values.

    Raises:
        ValueError: If fewer than 2 genomes provided
    """
    if len(genomes) < 2:
        raise ValueError("Need at least 2 genomes to compute distance matrix")

    n = len(genomes)
    matrix = np.zeros((n, n))
    labels = [g.genome_id for g in genomes]

    for i in range(n):
        for j in range(i + 1, n):
            dist = structural_distance(genomes[i], genomes[j], weights)
            matrix[i, j] = dist
            matrix[j, i] = dist  # Symmetric

    return matrix, labels


def distance_summary(matrix: np.ndarray, labels: list[str]) -> dict:
    """
    Compute summary statistics for a distance matrix.

    Args:
        matrix: Distance matrix
        labels: Genome labels

    Returns:
        Dictionary with min, max, mean, std of pairwise distances
    """
    # Extract upper triangle (excluding diagonal)
    n = len(labels)
    distances = []
    for i in range(n):
        for j in range(i + 1, n):
            distances.append(matrix[i, j])

    if not distances:
        return {"min": 0.0, "max": 0.0, "mean": 0.0, "std": 0.0}

    return {
        "min": float(np.min(distances)),
        "max": float(np.max(distances)),
        "mean": float(np.mean(distances)),
        "std": float(np.std(distances)),
    }
