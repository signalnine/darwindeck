"""Full fitness evaluation with session length constraint (Phase 4)."""

from dataclasses import dataclass
from typing import Dict, Optional
from cards_evolve.genome.schema import GameGenome


@dataclass(frozen=True)
class SimulationResults:
    """Results from batch simulation."""
    total_games: int
    player0_wins: int
    player1_wins: int
    draws: int
    avg_turns: float
    errors: int


@dataclass(frozen=True)
class FitnessMetrics:
    """Complete fitness evaluation metrics."""
    decision_density: float
    comeback_potential: float
    tension_curve: float
    interaction_frequency: float
    rules_complexity: float
    session_length: float       # Tracked but not averaged (constraint only)
    skill_vs_luck: float
    total_fitness: float
    games_simulated: int
    valid: bool


class FitnessEvaluator:
    """Evaluates game fitness with session length as constraint."""

    def __init__(self,
                 weights: Optional[Dict[str, float]] = None,
                 use_cache: bool = True):
        """Initialize fitness evaluator.

        Args:
            weights: Metric weights (default: equal weights, session_length excluded)
            use_cache: Enable fitness caching
        """
        # Session length excluded from weights (it's a constraint, not a metric)
        self.weights = weights or {
            'decision_density': 1.0,
            'comeback_potential': 1.0,
            'tension_curve': 1.0,
            'interaction_frequency': 1.0,
            'rules_complexity': 1.0,
            'skill_vs_luck': 1.0,
        }

        # Normalize weights to sum to 1.0
        total_weight = sum(self.weights.values())
        self.weights = {k: v / total_weight for k, v in self.weights.items()}

        self.cache: Dict[str, FitnessMetrics] = {} if use_cache else {}

    def evaluate(self,
                 genome: GameGenome,
                 results: SimulationResults,
                 use_mcts: bool = False) -> FitnessMetrics:
        """Evaluate fitness for a game genome.

        Args:
            genome: Game genome to evaluate
            results: Simulation results
            use_mcts: Whether MCTS was used (for skill_vs_luck metric)

        Returns:
            Fitness metrics
        """
        # Check cache
        if self.cache:
            cache_key = f"{genome.genome_id}_{results.total_games}"
            if cache_key in self.cache:
                return self.cache[cache_key]

        metrics = self._compute_metrics(genome, results, use_mcts)

        # Cache result
        if self.cache:
            self.cache[cache_key] = metrics

        return metrics

    def _compute_metrics(self,
                        genome: GameGenome,
                        results: SimulationResults,
                        use_mcts: bool) -> FitnessMetrics:
        """Compute fitness metrics from simulation results."""

        # 1. Decision density (placeholder - needs instrumentation)
        decision_density = min(1.0, len(genome.turn_structure.phases) / 5.0)

        # 2. Comeback potential (how balanced is the game?)
        win_rate_p0 = results.player0_wins / results.total_games if results.total_games > 0 else 0.5
        comeback_potential = 1.0 - abs(win_rate_p0 - 0.5) * 2

        # 3. Tension curve (placeholder - needs win prob trace)
        tension_curve = min(1.0, results.avg_turns / 100.0)

        # 4. Interaction frequency (placeholder - needs instrumentation)
        interaction_frequency = min(1.0, len(genome.special_effects) / 3.0)

        # 5. Rules complexity (inverse - simpler is better)
        complexity = (
            len(genome.turn_structure.phases) +
            len(genome.special_effects) * 2 +
            len(genome.scoring_rules) +
            len(genome.win_conditions)
        )
        rules_complexity = max(0.0, 1.0 - complexity / 20.0)

        # 6. Session length - CONSTRAINT, not metric
        estimated_duration_sec = results.avg_turns * 2  # 2 sec per turn
        target_min = 3 * 60   # 3 minutes
        target_max = 20 * 60  # 20 minutes

        # If outside acceptable range, return invalid fitness
        if estimated_duration_sec < target_min or estimated_duration_sec > target_max:
            return FitnessMetrics(
                decision_density=0.0,
                comeback_potential=0.0,
                tension_curve=0.0,
                interaction_frequency=0.0,
                rules_complexity=0.0,
                session_length=0.0,  # Violates constraint
                skill_vs_luck=0.0,
                total_fitness=0.0,   # Failed constraint
                games_simulated=results.total_games,
                valid=False  # Mark as invalid
            )

        # Within range: compute normalized score (1.0 = perfect 10 min)
        if estimated_duration_sec < 600:
            session_length = estimated_duration_sec / 600  # 0.5-1.0 for 3-10 min
        else:
            session_length = 1.0 - (estimated_duration_sec - 600) / 600  # 1.0-0.5 for 10-20 min

        # 7. Skill vs luck (only if MCTS used)
        skill_vs_luck = 0.5  # Neutral if not measured
        if use_mcts:
            # TODO: Compare MCTS win rate vs random baseline
            skill_vs_luck = 0.6  # Placeholder

        # Check validity
        valid = results.errors == 0 and results.total_games > 0

        # Compute weighted total (session_length removed from average)
        # Only 6 metrics now (session_length is a constraint)
        total_fitness = (
            self.weights['decision_density'] * decision_density +
            self.weights['comeback_potential'] * comeback_potential +
            self.weights['tension_curve'] * tension_curve +
            self.weights['interaction_frequency'] * interaction_frequency +
            self.weights['rules_complexity'] * rules_complexity +
            self.weights['skill_vs_luck'] * skill_vs_luck
        )

        # No need to renormalize - weights already sum to 1.0

        return FitnessMetrics(
            decision_density=decision_density,
            comeback_potential=comeback_potential,
            tension_curve=tension_curve,
            interaction_frequency=interaction_frequency,
            rules_complexity=rules_complexity,
            session_length=session_length,  # Keep for reporting
            skill_vs_luck=skill_vs_luck,
            total_fitness=total_fitness,
            games_simulated=results.total_games,
            valid=valid
        )
