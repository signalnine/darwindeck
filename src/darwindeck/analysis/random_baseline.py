"""Random genome generation for baseline comparison."""

from __future__ import annotations

import random
import math
from dataclasses import dataclass
from typing import Optional
from scipy import stats

from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    PlayPhase, DrawPhase, DiscardPhase, TrickPhase, ClaimPhase, BettingPhase,
    Location, Suit,
)
from darwindeck.genome.conditions import Condition, ConditionType, Operator
from darwindeck.evolution.naming import generate_name
from darwindeck.evolution.fitness_full import FitnessEvaluator
from darwindeck.simulation.go_simulator import GoSimulator
from darwindeck.analysis.mutation_sampler import FitnessTrajectory


@dataclass
class BaselineConfig:
    """Configuration for random baseline generation."""
    num_random_genomes: int = 20       # How many random genomes to sample
    require_playable: bool = True       # Filter to genomes that complete without errors
    max_generation_attempts: int = 100  # Attempts before giving up on a genome
    games_for_playability: int = 10     # Quick simulation for playability check


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

    # Basin radius (mutations before significant fitness drop)
    known_mean_basin_radius: float
    random_mean_basin_radius: float

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


def _random_condition() -> Condition:
    """Generate a random condition."""
    cond_type = random.choice([
        ConditionType.HAND_SIZE,
        ConditionType.LOCATION_SIZE,
    ])
    operator = random.choice([Operator.GT, Operator.GE, Operator.LT, Operator.LE, Operator.EQ])
    value = random.randint(0, 10)
    return Condition(type=cond_type, operator=operator, value=value)


def _generate_random_phase():
    """Generate a random phase."""
    phase_type = random.choices(
        ["draw", "play", "discard", "trick", "claim", "betting"],
        weights=[25, 30, 15, 10, 10, 10],
        k=1
    )[0]

    if phase_type == "draw":
        return DrawPhase(
            source=random.choice([Location.DECK, Location.DISCARD]),
            count=random.randint(1, 5),
            mandatory=random.choice([True, False]),
            condition=_random_condition() if random.random() < 0.3 else None
        )
    elif phase_type == "play":
        return PlayPhase(
            target=random.choice([Location.DISCARD, Location.TABLEAU]),
            valid_play_condition=_random_condition(),
            min_cards=random.randint(0, 2),
            max_cards=random.randint(1, 10),
            mandatory=random.choice([True, False])
        )
    elif phase_type == "discard":
        return DiscardPhase(
            target=Location.DISCARD,
            count=random.randint(1, 3),
            mandatory=random.choice([True, False])
        )
    elif phase_type == "trick":
        return TrickPhase(
            lead_suit_required=random.choice([True, False]),
            trump_suit=random.choice([None, Suit.SPADES, Suit.HEARTS, Suit.DIAMONDS, Suit.CLUBS]),
            high_card_wins=random.choice([True, False]),
            breaking_suit=random.choice([None, Suit.HEARTS, Suit.SPADES])
        )
    elif phase_type == "claim":
        return ClaimPhase(
            min_cards=1,
            max_cards=random.choice([1, 2, 3, 4]),
            sequential_rank=random.choice([True, False]),
            allow_challenge=True,
            pile_penalty=True
        )
    else:  # betting
        return BettingPhase(
            min_bet=random.choice([5, 10, 20, 50]),
            max_raises=random.choice([1, 2, 3, 4])
        )


def generate_random_genome(random_seed: Optional[int] = None) -> GameGenome:
    """
    Generate a completely random game genome.

    Args:
        random_seed: Optional seed for reproducibility

    Returns:
        A randomly generated GameGenome
    """
    if random_seed is not None:
        random.seed(random_seed)

    # Random setup
    player_count = random.choice([2, 3, 4])
    max_cards_per_player = 52 // player_count
    cards_per_player = random.randint(3, min(13, max_cards_per_player))

    # Decide if betting game
    has_betting = random.random() < 0.2
    starting_chips = random.choice([100, 500, 1000]) if has_betting else 0

    setup = SetupRules(
        cards_per_player=cards_per_player,
        initial_deck="standard_52",
        initial_discard_count=random.choice([0, 1]),
        starting_chips=starting_chips,
    )

    # Random phases (1-4)
    num_phases = random.randint(1, 4)
    phases = [_generate_random_phase() for _ in range(num_phases)]

    # Decide if trick-based
    is_trick_based = any(isinstance(p, TrickPhase) for p in phases)

    turn_structure = TurnStructure(
        phases=phases,
        is_trick_based=is_trick_based,
        tricks_per_hand=13 if is_trick_based else None,
    )

    # Random win conditions (1-2)
    win_types = [
        "empty_hand", "high_score", "first_to_score", "capture_all",
        "low_score", "most_captured", "best_hand"
    ]
    num_win_conditions = random.randint(1, 2)
    win_conditions = []
    for _ in range(num_win_conditions):
        wc_type = random.choice(win_types)
        if wc_type in ["high_score", "first_to_score", "low_score"]:
            threshold = random.choice([50, 100, 200, 500])
        else:
            threshold = None
        win_conditions.append(WinCondition(type=wc_type, threshold=threshold))

    return GameGenome(
        schema_version="1.0",
        genome_id=generate_name(),
        generation=0,
        setup=setup,
        turn_structure=turn_structure,
        special_effects=[],
        win_conditions=win_conditions,
        scoring_rules=[],
        max_turns=random.randint(50, 200),
        min_turns=10,
        player_count=player_count,
    )


def _is_playable(
    genome: GameGenome,
    simulator: GoSimulator,
    games: int = 10
) -> bool:
    """Check if genome produces playable games (< 50% error rate)."""
    try:
        results = simulator.simulate(genome, num_games=games)
        error_rate = results.errors / results.total_games if results.total_games > 0 else 1.0
        return error_rate < 0.5
    except Exception:
        return False


def generate_random_genomes(
    config: BaselineConfig,
    evaluator: FitnessEvaluator,
    random_seed: Optional[int] = None
) -> list[GameGenome]:
    """
    Generate random valid genomes for baseline comparison.

    Args:
        config: Generation parameters
        evaluator: For playability validation
        random_seed: For reproducibility

    Returns:
        List of random genomes that pass playability check
    """
    if random_seed is not None:
        random.seed(random_seed)

    genomes: list[GameGenome] = []
    simulator = GoSimulator(seed=random_seed or 42)
    attempts = 0

    while len(genomes) < config.num_random_genomes and attempts < config.max_generation_attempts:
        genome = generate_random_genome()
        attempts += 1

        if config.require_playable:
            if _is_playable(genome, simulator, config.games_for_playability):
                genomes.append(genome)
        else:
            genomes.append(genome)

    return genomes


def compute_decay_rate(trajectory: FitnessTrajectory) -> float:
    """
    Compute fitness decay rate for a trajectory.

    Returns slope of linear regression: fitness ~ mutation_step
    Negative values indicate fitness decreasing with mutations.
    """
    if len(trajectory.steps) < 2:
        return 0.0

    n = len(trajectory.steps)
    x = list(range(n))
    y = trajectory.steps

    # Linear regression slope
    x_mean = sum(x) / n
    y_mean = sum(y) / n

    numerator = sum((x[i] - x_mean) * (y[i] - y_mean) for i in range(n))
    denominator = sum((x[i] - x_mean) ** 2 for i in range(n))

    if denominator == 0:
        return 0.0

    return numerator / denominator


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
    if len(trajectory.steps) < 2:
        return 0

    initial = trajectory.steps[0]
    if initial <= 0:
        return 0

    drop_threshold = initial * (1.0 - threshold)

    for i, fitness in enumerate(trajectory.steps[1:], start=1):
        if fitness < drop_threshold:
            return i - 1

    # Never dropped - return full length
    return len(trajectory.steps) - 1


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
    # Initial fitness statistics
    known_initial = [t.initial_fitness for t in known_trajectories]
    random_initial = [t.initial_fitness for t in random_trajectories]

    known_mean_fitness = sum(known_initial) / len(known_initial) if known_initial else 0.0
    random_mean_fitness = sum(random_initial) / len(random_initial) if random_initial else 0.0

    known_std_fitness = _std(known_initial)
    random_std_fitness = _std(random_initial)

    # Decay rate statistics
    known_decay_rates = [compute_decay_rate(t) for t in known_trajectories]
    random_decay_rates = [compute_decay_rate(t) for t in random_trajectories]

    known_mean_decay = sum(known_decay_rates) / len(known_decay_rates) if known_decay_rates else 0.0
    random_mean_decay = sum(random_decay_rates) / len(random_decay_rates) if random_decay_rates else 0.0

    # Basin radius statistics
    known_radii = [compute_basin_radius(t) for t in known_trajectories]
    random_radii = [compute_basin_radius(t) for t in random_trajectories]

    known_mean_radius = sum(known_radii) / len(known_radii) if known_radii else 0.0
    random_mean_radius = sum(random_radii) / len(random_radii) if random_radii else 0.0

    # Statistical tests (Mann-Whitney U)
    if known_initial and random_initial:
        try:
            _, fitness_pvalue = stats.mannwhitneyu(
                known_initial, random_initial, alternative='greater'
            )
        except Exception:
            fitness_pvalue = 1.0
    else:
        fitness_pvalue = 1.0

    if known_decay_rates and random_decay_rates:
        try:
            # For decay rates, we want known to be LESS negative (gentler slope)
            # So we test if known > random
            _, decay_pvalue = stats.mannwhitneyu(
                known_decay_rates, random_decay_rates, alternative='greater'
            )
        except Exception:
            decay_pvalue = 1.0
    else:
        decay_pvalue = 1.0

    return BaselineStatistics(
        known_mean_fitness=known_mean_fitness,
        known_std_fitness=known_std_fitness,
        random_mean_fitness=random_mean_fitness,
        random_std_fitness=random_std_fitness,
        known_mean_decay_rate=known_mean_decay,
        random_mean_decay_rate=random_mean_decay,
        known_mean_basin_radius=known_mean_radius,
        random_mean_basin_radius=random_mean_radius,
        fitness_difference_pvalue=fitness_pvalue,
        decay_rate_difference_pvalue=decay_pvalue,
    )


def _std(values: list[float]) -> float:
    """Compute standard deviation."""
    if len(values) < 2:
        return 0.0
    mean = sum(values) / len(values)
    variance = sum((v - mean) ** 2 for v in values) / (len(values) - 1)
    return math.sqrt(variance)
