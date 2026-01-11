"""Full fitness evaluation with session length constraint (Phase 4)."""

from dataclasses import dataclass
from typing import Dict, Optional
from darwindeck.genome.schema import GameGenome


@dataclass(frozen=True)
class SimulationResults:
    """Results from batch simulation."""
    total_games: int
    player0_wins: int
    player1_wins: int
    draws: int
    avg_turns: float
    errors: int

    # Phase 1 instrumentation (optional, defaults to 0 for backward compatibility)
    total_decisions: int = 0
    total_valid_moves: int = 0
    forced_decisions: int = 0
    total_interactions: int = 0
    total_actions: int = 0


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
            weights: Metric weights (default: reweighted to favor skill and interaction)
            use_cache: Enable fitness caching
        """
        # Session length excluded from weights (it's a constraint, not a metric)
        # Reweighted based on evolved game analysis (see docs/analysis-0.8403-ceiling.md):
        # - Increased skill_vs_luck (0.30): Most important for game quality
        # - Increased interaction_frequency (0.20): Enables interesting gameplay
        # - Increased decision_density (0.20): Meaningful choices matter
        # - Decreased rules_complexity (0.05): Was blocking special effects
        # - Decreased tension_curve (0.10): Less critical, often saturated
        # - Maintained comeback_potential (0.15): Balance still important
        self.weights = weights or {
            'decision_density': 0.20,
            'comeback_potential': 0.15,
            'tension_curve': 0.10,
            'interaction_frequency': 0.20,
            'rules_complexity': 0.05,
            'skill_vs_luck': 0.30,
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

        # 1. Decision density - use real data if available, else heuristic
        if hasattr(results, 'total_decisions') and results.total_decisions > 0:
            # Real instrumentation available (Phase 1)
            avg_valid_moves = results.total_valid_moves / results.total_decisions
            forced_ratio = results.forced_decisions / results.total_decisions

            # Score: High when many moves available, low when forced
            # avg_valid_moves: 1 = forced, 2-3 = some choice, 4+ = rich decisions
            # Cap at 6 valid moves as "perfect"
            decision_density = min(1.0, (
                min(1.0, (avg_valid_moves - 1) / 5.0) * 0.7 +  # Reward choice
                (1.0 - forced_ratio) * 0.3  # Penalty for forced moves
            ))
        else:
            # Fallback to heuristic (current implementation)
            optional_phases = sum(1 for p in genome.turn_structure.phases
                                if hasattr(p, 'mandatory') and not p.mandatory)
            phase_count = len(genome.turn_structure.phases)
            has_conditions = sum(1 for p in genome.turn_structure.phases
                               if hasattr(p, 'condition') and p.condition is not None)

            decision_density = min(1.0, (
                min(1.0, phase_count / 6.0) * 0.5 +
                min(1.0, optional_phases / 3.0) * 0.3 +
                min(1.0, has_conditions / 3.0) * 0.2
            ))

        # 2. Comeback potential (how balanced is the game?)
        win_rate_p0 = results.player0_wins / results.total_games if results.total_games > 0 else 0.5
        comeback_potential = 1.0 - abs(win_rate_p0 - 0.5) * 2

        # 3. Tension curve - improved with game length variance proxy
        # Games with variable length tend to have more tension
        # Use average turns as proxy for variability (longer games = more room for variance)
        turn_score = min(1.0, results.avg_turns / 100.0)
        # Bonus if game isn't too short (too short = less room for tension)
        length_bonus = min(1.0, max(0.0, (results.avg_turns - 20) / 50.0))
        tension_curve = min(1.0, turn_score * 0.6 + length_bonus * 0.4)

        # 4. Interaction frequency - use real data if available, else heuristic
        if hasattr(results, 'total_actions') and results.total_actions > 0:
            # Real instrumentation available (Phase 1)
            interaction_ratio = results.total_interactions / results.total_actions

            # Score: Direct ratio of interactions to actions
            # 0.0 = no interaction (solitaire), 0.5 = half actions interactive,
            # 1.0 = all actions affect opponents (very interactive)
            interaction_frequency = min(1.0, interaction_ratio)
        else:
            # Fallback to heuristic (current implementation)
            special_effects_score = min(1.0, len(genome.special_effects) / 3.0)
            trick_based_score = 0.3 if genome.turn_structure.is_trick_based else 0.0
            multi_phase_score = min(0.4, len(genome.turn_structure.phases) / 10.0)

            interaction_frequency = min(1.0,
                special_effects_score * 0.4 +
                trick_based_score +
                multi_phase_score
            )

        # 5. Rules complexity - separated into mechanical vs gameplay complexity
        # Mechanical complexity (simpler = better): phase count, mandatory steps
        mechanical_complexity = len(genome.turn_structure.phases)
        mechanical_score = max(0.0, 1.0 - mechanical_complexity / 8.0)  # Cap at 8 phases

        # Gameplay richness (richer = better, but balanced): special effects, scoring
        # Special effects add interaction and interest (positive!)
        # But too many can make game confusing (diminishing returns)
        special_effects_count = len(genome.special_effects)
        scoring_rules_count = len(genome.scoring_rules)

        # Reward 1-3 special effects, neutral at 0, penalty beyond 5
        effects_score = min(1.0, max(0.0, (
            0.7 +  # Baseline for no effects
            (special_effects_count / 3.0) * 0.5 -  # Reward up to 3 effects
            max(0.0, (special_effects_count - 3) * 0.1)  # Penalty beyond 3
        )))

        # Combine: 60% mechanical simplicity, 40% gameplay richness
        rules_complexity = mechanical_score * 0.6 + effects_score * 0.4

        # 6. Session length - CONSTRAINT, not metric
        estimated_duration_sec = results.avg_turns * 2  # 2 sec per turn
        target_min = 0        # No minimum
        target_max = 30 * 60  # 30 minutes

        # If outside acceptable range, return invalid fitness
        if estimated_duration_sec > target_max:
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

        # Within range: compute normalized score (1.0 = perfect 15 min)
        optimal_sec = 15 * 60  # 15 minutes is ideal
        if estimated_duration_sec < optimal_sec:
            session_length = estimated_duration_sec / optimal_sec  # 0.0-1.0 for 0-15 min
        else:
            # Gradual decline from 15-30 min (1.0 to 0.5)
            session_length = 1.0 - (estimated_duration_sec - optimal_sec) / (target_max - optimal_sec) * 0.5

        # 7. Skill vs luck - improved heuristic
        # Use win rate variance as proxy: balanced games suggest more skill
        # (Pure luck games tend to have ~50/50 win rates, but so do balanced skill games)
        # Combined with game length: longer games with balance = more skill opportunity

        if use_mcts:
            # TODO: Compare MCTS win rate vs random baseline
            # For now, use improved heuristic
            skill_vs_luck = 0.7  # Assume MCTS testing means we're measuring skill
        else:
            # Without MCTS, estimate skill potential from game structure
            # Factors: game length (more turns = more decisions = more skill)
            #          balance (too imbalanced = luck or broken)
            #          complexity (more complex = more skill ceiling)

            length_factor = min(1.0, results.avg_turns / 80.0)  # Cap at 80 turns
            balance_factor = comeback_potential  # Already measures balance (0-1)
            complexity_factor = min(1.0, (
                len(genome.turn_structure.phases) +
                len(genome.special_effects) +
                (1 if genome.turn_structure.is_trick_based else 0)
            ) / 8.0)

            # Weighted combination: longer balanced complex games = more skill
            skill_vs_luck = min(1.0,
                length_factor * 0.4 +
                balance_factor * 0.3 +
                complexity_factor * 0.3
            )

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
