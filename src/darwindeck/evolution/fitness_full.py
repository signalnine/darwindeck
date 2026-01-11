"""Full fitness evaluation with session length constraint (Phase 4)."""

from dataclasses import dataclass
from typing import Dict, Optional
from darwindeck.genome.schema import GameGenome, PlayPhase, DrawPhase


# Preset weight configurations for different game styles
STYLE_PRESETS = {
    'balanced': {
        'decision_density': 0.20,
        'comeback_potential': 0.15,
        'tension_curve': 0.10,
        'interaction_frequency': 0.20,
        'rules_complexity': 0.05,
        'skill_vs_luck': 0.30,
        'bluffing_depth': 0.00,  # Not required for balanced games
    },
    'bluffing': {
        # Favor hidden information, betting, and player interaction
        'decision_density': 0.15,
        'comeback_potential': 0.10,
        'tension_curve': 0.10,
        'interaction_frequency': 0.20,
        'rules_complexity': 0.05,
        'skill_vs_luck': 0.10,
        'bluffing_depth': 0.30,  # High - must have quality bluffing mechanics
    },
    'strategic': {
        # Favor deep thinking, skill-based play
        'decision_density': 0.25,
        'comeback_potential': 0.10,
        'tension_curve': 0.05,
        'interaction_frequency': 0.15,
        'rules_complexity': 0.05,
        'skill_vs_luck': 0.40,  # High skill emphasis
        'bluffing_depth': 0.00,  # Not required for strategic games
    },
    'party': {
        # Favor quick, interactive, accessible games
        # Note: skill_vs_luck is INVERTED for party - higher = more luck-friendly
        'decision_density': 0.10,
        'comeback_potential': 0.20,  # Everyone can win
        'tension_curve': 0.10,
        'interaction_frequency': 0.25,  # High interaction
        'rules_complexity': 0.20,  # Simple rules
        'skill_vs_luck': 0.15,  # Luck-friendly (inverted: rewards low skill dominance)
        'bluffing_depth': 0.00,  # Not required for party games
    },
    'trick-taking': {
        # Favor trick-based mechanics
        'decision_density': 0.20,
        'comeback_potential': 0.15,
        'tension_curve': 0.15,
        'interaction_frequency': 0.25,
        'rules_complexity': 0.05,
        'skill_vs_luck': 0.20,
        'bluffing_depth': 0.00,  # Not required for trick-taking
    },
}


@dataclass(frozen=True)
class SimulationResults:
    """Results from batch simulation."""
    total_games: int
    wins: tuple[int, ...]  # Wins per player (index = player ID)
    player_count: int  # Number of players (2-4)
    draws: int
    avg_turns: float
    errors: int

    # Phase 1 instrumentation (optional, defaults to 0 for backward compatibility)
    total_decisions: int = 0
    total_valid_moves: int = 0
    forced_decisions: int = 0
    total_hand_size: int = 0  # For filtering ratio calculation
    total_interactions: int = 0
    total_actions: int = 0

    # Bluffing metrics (ClaimPhase games)
    total_claims: int = 0
    total_bluffs: int = 0
    total_challenges: int = 0
    successful_bluffs: int = 0
    successful_catches: int = 0

    @property
    def player0_wins(self) -> int:
        """Backward compatibility property."""
        return self.wins[0] if self.wins else 0

    @property
    def player1_wins(self) -> int:
        """Backward compatibility property."""
        return self.wins[1] if len(self.wins) > 1 else 0


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
    bluffing_depth: float       # Quality of bluffing mechanics (0 for non-bluffing games)
    total_fitness: float
    games_simulated: int
    valid: bool


class FitnessEvaluator:
    """Evaluates game fitness with session length as constraint."""

    def __init__(self,
                 weights: Optional[Dict[str, float]] = None,
                 style: Optional[str] = None,
                 use_cache: bool = True):
        """Initialize fitness evaluator.

        Args:
            weights: Metric weights (overrides style if provided)
            style: Style preset name (balanced, bluffing, strategic, party, trick-taking)
            use_cache: Enable fitness caching
        """
        # Use style preset if specified, otherwise use weights or default
        if style and style in STYLE_PRESETS:
            self.weights = STYLE_PRESETS[style].copy()
            self.style = style
        elif weights:
            self.weights = weights.copy()
            self.style = 'custom'
        else:
            self.weights = STYLE_PRESETS['balanced'].copy()
            self.style = 'balanced'

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

            # Calculate filtering ratio: how much the game filters available moves
            # If valid_moves == hand_size, no filtering is happening (meaningless choice)
            # filtering_ratio = 0 means all cards always valid (like War)
            # filtering_ratio = 0.8 means only 20% of hand is valid (real constraints)
            if results.total_hand_size > 0:
                raw_ratio = 1.0 - (results.total_valid_moves / results.total_hand_size)
                if raw_ratio < 0:
                    # More moves than cards = multi-phase game or draw options
                    # This indicates meaningful choices exist (not just "play any card")
                    filtering_ratio = 0.5  # Baseline for games with phase variety
                else:
                    filtering_ratio = min(1.0, raw_ratio)
            else:
                filtering_ratio = 0.0

            # Score: High when many FILTERED moves available, low when forced or unfiltered
            # avg_valid_moves matters only if there's actual filtering
            # Without filtering, having many options is meaningless (like War)
            choice_score = min(1.0, (avg_valid_moves - 1) / 5.0)

            # Apply filtering penalty: unfiltered games get low decision density
            # even if they have many valid moves
            decision_density = min(1.0, (
                choice_score * filtering_ratio * 0.5 +  # Filtered choices (most important)
                (1.0 - forced_ratio) * 0.3 +  # Not being forced is still good
                filtering_ratio * 0.2  # Raw filtering score
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
        # Expected win rate: 1/N for N players (50% for 2, 33% for 3, 25% for 4)
        expected_rate = 1.0 / results.player_count if results.player_count > 0 else 0.5
        max_deviation = 1.0 - expected_rate  # Maximum possible deviation from expected

        # Calculate average deviation from expected across all players
        if results.total_games > 0:
            deviations = []
            for wins in results.wins:
                actual_rate = wins / results.total_games
                deviation = abs(actual_rate - expected_rate) / max_deviation if max_deviation > 0 else 0
                deviations.append(deviation)
            avg_deviation = sum(deviations) / len(deviations) if deviations else 0
        else:
            avg_deviation = 0

        comeback_potential = 1.0 - avg_deviation

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
        target_max = 60 * 60  # 60 minutes

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
                bluffing_depth=0.0,
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

        # For party style, invert skill metric: we want luck-friendly games
        # where casual players can win, not games where skill dominates
        if self.style == 'party':
            skill_vs_luck = 1.0 - skill_vs_luck

        # 8. Bluffing depth - quality of bluffing mechanics
        # Only relevant for games with ClaimPhase (bluffing mechanics)
        if results.total_claims > 0:
            # Game has bluffing - evaluate quality
            bluff_rate = results.total_bluffs / results.total_claims
            challenge_rate = results.total_challenges / results.total_claims if results.total_claims > 0 else 0

            # Good bluffing games have:
            # - Mix of bluffs and honest claims (not 100% bluffs)
            # - Meaningful challenge decisions (not 0% or 100%)
            # - Both successful bluffs and catches (skill in reading opponents)

            # Bluff rate score: Best around 50-70% (some honest, some bluff)
            bluff_score = 1.0 - abs(bluff_rate - 0.6) * 2
            bluff_score = max(0.0, min(1.0, bluff_score))

            # Challenge rate score: Best around 30-50% (some trust, some skepticism)
            challenge_score = 1.0 - abs(challenge_rate - 0.4) * 2
            challenge_score = max(0.0, min(1.0, challenge_score))

            # Success balance: Both bluffs and catches should succeed sometimes
            total_outcomes = results.successful_bluffs + results.successful_catches
            if total_outcomes > 0:
                bluff_success_rate = results.successful_bluffs / total_outcomes
                # Best around 50% - neither side dominates
                balance_score = 1.0 - abs(bluff_success_rate - 0.5) * 2
                balance_score = max(0.0, min(1.0, balance_score))
            else:
                balance_score = 0.0

            # Combine scores
            bluffing_depth = (
                bluff_score * 0.3 +      # Good bluff rate
                challenge_score * 0.3 +  # Good challenge rate
                balance_score * 0.4      # Balanced outcomes (most important)
            )
        else:
            # No bluffing mechanics
            bluffing_depth = 0.0

        # Check validity
        valid = results.errors == 0 and results.total_games > 0

        # Compute weighted total (session_length removed from average)
        # 7 metrics now (session_length is a constraint)
        total_fitness = (
            self.weights['decision_density'] * decision_density +
            self.weights['comeback_potential'] * comeback_potential +
            self.weights['tension_curve'] * tension_curve +
            self.weights['interaction_frequency'] * interaction_frequency +
            self.weights['rules_complexity'] * rules_complexity +
            self.weights['skill_vs_luck'] * skill_vs_luck +
            self.weights['bluffing_depth'] * bluffing_depth
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
            bluffing_depth=bluffing_depth,
            total_fitness=total_fitness,
            games_simulated=results.total_games,
            valid=valid
        )
