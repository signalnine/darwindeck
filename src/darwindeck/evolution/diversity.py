"""Diversity measurement and selection for genome populations."""

from __future__ import annotations

from dataclasses import dataclass
from typing import List, Set, Tuple, Dict, Any
import random

from darwindeck.genome.schema import (
    GameGenome, DrawPhase, PlayPhase, DiscardPhase, TrickPhase, ClaimPhase
)
from darwindeck.genome.conditions import (
    Condition, CompoundCondition, ConditionType
)


@dataclass
class GenomeFeatures:
    """Structural features extracted from a genome for diversity comparison."""

    # Phase structure
    phase_types: frozenset[str]  # {"DrawPhase", "PlayPhase", ...}
    num_phases: int

    # Game type indicators
    is_trick_based: bool
    has_trump: bool
    has_bluffing: bool  # ClaimPhase present

    # Player/card structure
    player_count: int
    cards_per_player: int

    # Win conditions
    win_condition_types: frozenset[str]

    # Condition types used (what mechanics the game uses)
    condition_types: frozenset[str]

    # Turn limits (normalized to buckets)
    max_turns_bucket: int  # 0: <100, 1: 100-500, 2: 500-1000, 3: >1000


def extract_features(genome: GameGenome) -> GenomeFeatures:
    """Extract structural features from a genome."""

    # Phase types
    phase_types = set()
    has_bluffing = False
    has_trump = False

    for phase in genome.turn_structure.phases:
        phase_types.add(type(phase).__name__)
        if isinstance(phase, ClaimPhase):
            has_bluffing = True
        if isinstance(phase, TrickPhase) and phase.trump_suit is not None:
            has_trump = True

    # Also check setup for trump
    if genome.setup.trump_suit is not None:
        has_trump = True

    # Win condition types
    win_types = frozenset(wc.type for wc in genome.win_conditions)

    # Collect all condition types used
    condition_types = set()
    for phase in genome.turn_structure.phases:
        _collect_condition_types(phase, condition_types)

    # Max turns bucket
    if genome.max_turns < 100:
        bucket = 0
    elif genome.max_turns < 500:
        bucket = 1
    elif genome.max_turns < 1000:
        bucket = 2
    else:
        bucket = 3

    return GenomeFeatures(
        phase_types=frozenset(phase_types),
        num_phases=len(genome.turn_structure.phases),
        is_trick_based=genome.turn_structure.is_trick_based,
        has_trump=has_trump,
        has_bluffing=has_bluffing,
        player_count=genome.player_count,
        cards_per_player=genome.setup.cards_per_player,
        win_condition_types=win_types,
        condition_types=frozenset(condition_types),
        max_turns_bucket=bucket,
    )


def _collect_condition_types(phase: Any, condition_types: Set[str]) -> None:
    """Recursively collect condition types from a phase."""
    if isinstance(phase, PlayPhase):
        _collect_from_condition(phase.valid_play_condition, condition_types)
    elif isinstance(phase, DrawPhase) and phase.condition:
        _collect_from_condition(phase.condition, condition_types)
    elif isinstance(phase, DiscardPhase) and phase.matching_condition:
        _collect_from_condition(phase.matching_condition, condition_types)


def _collect_from_condition(cond: Any, condition_types: Set[str]) -> None:
    """Recursively collect condition types from a condition tree."""
    if isinstance(cond, Condition):
        condition_types.add(cond.type.name)
    elif isinstance(cond, CompoundCondition):
        for sub in cond.conditions:
            _collect_from_condition(sub, condition_types)


def compute_distance(f1: GenomeFeatures, f2: GenomeFeatures) -> float:
    """Compute structural distance between two genomes.

    Returns a value between 0 (identical) and 1 (maximally different).
    """
    distances = []

    # Jaccard distance for sets (0 = identical, 1 = no overlap)
    distances.append(_jaccard_distance(f1.phase_types, f2.phase_types) * 1.5)  # Weight phase types
    distances.append(_jaccard_distance(f1.win_condition_types, f2.win_condition_types))
    distances.append(_jaccard_distance(f1.condition_types, f2.condition_types) * 1.2)  # Weight mechanics

    # Boolean differences (0 or 1)
    distances.append(1.0 if f1.is_trick_based != f2.is_trick_based else 0.0)
    distances.append(1.0 if f1.has_trump != f2.has_trump else 0.0)
    distances.append(1.0 if f1.has_bluffing != f2.has_bluffing else 0.0)

    # Numeric differences (normalized)
    distances.append(abs(f1.player_count - f2.player_count) / 3.0)  # Max diff is 4-2=2, but use 3 for headroom
    distances.append(min(1.0, abs(f1.cards_per_player - f2.cards_per_player) / 20.0))
    distances.append(abs(f1.num_phases - f2.num_phases) / 5.0)  # Normalize by typical max phases
    distances.append(abs(f1.max_turns_bucket - f2.max_turns_bucket) / 3.0)

    # Average all distances
    return sum(distances) / len(distances)


def _jaccard_distance(s1: frozenset, s2: frozenset) -> float:
    """Compute Jaccard distance between two sets."""
    if not s1 and not s2:
        return 0.0  # Both empty = identical
    union = len(s1 | s2)
    intersection = len(s1 & s2)
    return 1.0 - (intersection / union)


def select_diverse_subset(
    genomes: List[GameGenome],
    target_size: int,
    random_seed: int | None = None
) -> List[GameGenome]:
    """Select a maximally diverse subset using greedy farthest-point sampling.

    Algorithm:
    1. Start with a random genome
    2. Repeatedly add the genome that is most different from the current set
    3. Continue until we have target_size genomes

    Args:
        genomes: List of genomes to select from
        target_size: Number of genomes to select
        random_seed: Random seed for reproducibility

    Returns:
        List of diverse genomes
    """
    if len(genomes) <= target_size:
        return genomes

    if random_seed is not None:
        random.seed(random_seed)

    # Extract features for all genomes
    features = [extract_features(g) for g in genomes]

    # Precompute distance matrix (expensive but worth it for greedy selection)
    n = len(genomes)
    dist_matrix: Dict[Tuple[int, int], float] = {}
    for i in range(n):
        for j in range(i + 1, n):
            d = compute_distance(features[i], features[j])
            dist_matrix[(i, j)] = d
            dist_matrix[(j, i)] = d

    # Start with random genome
    selected_indices: List[int] = [random.randint(0, n - 1)]
    remaining_indices = set(range(n)) - set(selected_indices)

    # Greedy farthest-point sampling
    while len(selected_indices) < target_size and remaining_indices:
        best_idx = -1
        best_min_dist = -1.0

        for candidate in remaining_indices:
            # Find minimum distance to any selected genome
            min_dist = min(
                dist_matrix.get((candidate, sel), 0.0)
                for sel in selected_indices
            )
            # We want the candidate with maximum min_dist (farthest from all selected)
            if min_dist > best_min_dist:
                best_min_dist = min_dist
                best_idx = candidate

        if best_idx >= 0:
            selected_indices.append(best_idx)
            remaining_indices.remove(best_idx)

    return [genomes[i] for i in selected_indices]


def compute_population_diversity(genomes: List[GameGenome]) -> float:
    """Compute average pairwise diversity of a population.

    Returns a value between 0 (all identical) and 1 (maximally diverse).
    """
    if len(genomes) < 2:
        return 0.0

    features = [extract_features(g) for g in genomes]

    total_dist = 0.0
    count = 0
    for i in range(len(features)):
        for j in range(i + 1, len(features)):
            total_dist += compute_distance(features[i], features[j])
            count += 1

    return total_dist / count if count > 0 else 0.0
