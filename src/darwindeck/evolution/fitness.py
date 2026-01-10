"""Fitness evaluation metrics (two-phase approach from consensus)."""

from dataclasses import dataclass
from typing import List
from darwindeck.simulation.engine import GameResult


@dataclass(frozen=True)
class CheapFitnessMetrics:
    """Cheap metrics computed for all candidates.

    These run in Phase 1 of two-phase fitness evaluation.
    Fast enough to compute for entire population.
    """

    avg_game_length: float
    completion_rate: float  # Games that didn't hit max_turns
    decision_branch_factor: float  # Legal move count (NOT outcome equivalence)


def calculate_cheap_metrics(results: List[GameResult]) -> CheapFitnessMetrics:
    """Calculate cheap fitness metrics from game results.

    Phase 1 of two-phase evaluation (consensus recommendation).

    Args:
        results: List of game simulation results

    Returns:
        Cheap metrics for filtering
    """
    if not results:
        return CheapFitnessMetrics(
            avg_game_length=0.0,
            completion_rate=0.0,
            decision_branch_factor=0.0
        )

    total_turns = sum(r.turn_count for r in results)
    avg_length = total_turns / len(results)

    # For War: hardcode decision_branch_factor = 0 (no choices)
    # TODO: Generalize when we have legal move generation
    decision_branch_factor = 0.0

    # Completion rate (didn't timeout)
    # Simplified: assume games always complete for now
    completion_rate = 1.0

    return CheapFitnessMetrics(
        avg_game_length=avg_length,
        completion_rate=completion_rate,
        decision_branch_factor=decision_branch_factor
    )
