"""Population management with diversity tracking (Phase 4)."""

from dataclasses import dataclass
from typing import List
import random
from darwindeck.genome.schema import GameGenome


# Diversity threshold for warning
DIVERSITY_THRESHOLD = 0.1


@dataclass
class Individual:
    """Individual in population with genome and fitness."""
    genome: GameGenome
    fitness: float = 0.0
    evaluated: bool = False


def genome_distance(g1: GameGenome, g2: GameGenome) -> float:
    """
    Compute distance between two genomes (0.0 = identical, 1.0 = maximally different).

    Uses Hamming distance on key structural features.

    Args:
        g1: First genome
        g2: Second genome

    Returns:
        Distance in range [0.0, 1.0]
    """
    distance = 0.0
    total_features = 0

    # 1. Turn structure phase count
    phase_diff = abs(len(g1.turn_structure.phases) - len(g2.turn_structure.phases))
    distance += min(1.0, phase_diff / 5.0)  # Normalize by max expected diff
    total_features += 1

    # 2. Special effects count
    effect_diff = abs(len(g1.special_effects) - len(g2.special_effects))
    distance += min(1.0, effect_diff / 3.0)
    total_features += 1

    # 3. Win conditions count
    win_diff = abs(len(g1.win_conditions) - len(g2.win_conditions))
    distance += min(1.0, win_diff / 2.0)
    total_features += 1

    # 4. Max turns (normalized)
    turns_diff = abs(g1.max_turns - g2.max_turns) / 1000.0
    distance += min(1.0, turns_diff)
    total_features += 1

    # 5. Setup differences
    card_diff = abs(g1.setup.cards_per_player - g2.setup.cards_per_player)
    distance += min(1.0, card_diff / 26.0)
    total_features += 1

    return distance / total_features


class Population:
    """Population of game genomes with diversity tracking."""

    def __init__(self, individuals: List[Individual]):
        """Initialize population.

        Args:
            individuals: List of individuals in population
        """
        self.individuals = individuals
        self.generation = 0

    def compute_diversity(self) -> float:
        """
        Compute population diversity metric using pairwise distances.

        Higher = more diverse, Lower = converged

        Returns:
            Diversity score in range [0.0, 1.0]
        """
        if len(self.individuals) < 2:
            return 0.0

        # Compute average pairwise distance
        total_distance = 0.0
        pair_count = 0

        # Sample pairs (all pairs for small populations, random sample for large)
        if len(self.individuals) <= 50:
            # Small population: check all pairs
            for i in range(len(self.individuals)):
                for j in range(i + 1, len(self.individuals)):
                    total_distance += genome_distance(
                        self.individuals[i].genome,
                        self.individuals[j].genome
                    )
                    pair_count += 1
        else:
            # Large population: sample 100 random pairs
            for _ in range(100):
                i, j = random.sample(range(len(self.individuals)), 2)
                total_distance += genome_distance(
                    self.individuals[i].genome,
                    self.individuals[j].genome
                )
                pair_count += 1

        if pair_count == 0:
            return 0.0

        avg_distance = total_distance / pair_count
        return avg_distance  # Already in 0-1 range

    def check_diversity_crisis(self) -> bool:
        """Check if diversity has collapsed.

        Returns:
            True if diversity below threshold (crisis), False otherwise
        """
        diversity = self.compute_diversity()
        return diversity < DIVERSITY_THRESHOLD

    def get_best_individual(self) -> Individual:
        """Get individual with highest fitness.

        Returns:
            Best individual
        """
        return max(self.individuals, key=lambda ind: ind.fitness)

    def get_average_fitness(self) -> float:
        """Get average fitness across population.

        Returns:
            Average fitness
        """
        if not self.individuals:
            return 0.0

        evaluated = [ind for ind in self.individuals if ind.evaluated]
        if not evaluated:
            return 0.0

        return sum(ind.fitness for ind in evaluated) / len(evaluated)
