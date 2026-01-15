"""Full fitness evaluation with session length constraint (Phase 4)."""

from dataclasses import dataclass, field
from typing import Dict, Optional
from darwindeck.genome.schema import GameGenome, PlayPhase, DrawPhase


# Preset weight configurations for different game styles
# IMPORTANT: Rules complexity is heavily weighted because complex games
# don't get played - people won't learn rules they can't quickly understand
STYLE_PRESETS = {
    'balanced': {
        # Balanced preset: games need meaningful decisions AND be learnable
        # A game without decisions (War) shouldn't rank high even if simple
        'decision_density': 0.25,    # PRIMARY - no decisions = not a game
        'skill_vs_luck': 0.20,       # Skill should matter
        'rules_complexity': 0.18,    # Learnable but not dominant
        'comeback_potential': 0.12,  # Games should feel winnable
        'interaction_frequency': 0.10,  # Social element
        'tension_curve': 0.08,       # Nice to have drama
        'bluffing_depth': 0.00,
        'betting_engagement': 0.07,
    },
    'bluffing': {
        # Bluffing games can be slightly more complex, but still need to be learnable
        'rules_complexity': 0.35,
        'decision_density': 0.05,
        'comeback_potential': 0.05,
        'tension_curve': 0.05,
        'interaction_frequency': 0.08,
        'skill_vs_luck': 0.05,
        'bluffing_depth': 0.18,  # Quality bluffing mechanics
        'betting_engagement': 0.19,  # Betting psychology
    },
    'strategic': {
        # Strategy gamers tolerate MORE complexity, but it still matters a lot
        'rules_complexity': 0.30,  # Lower than others, but still significant
        'decision_density': 0.20,
        'comeback_potential': 0.08,
        'tension_curve': 0.05,
        'interaction_frequency': 0.10,
        'skill_vs_luck': 0.27,  # High skill emphasis
        'bluffing_depth': 0.00,
        'betting_engagement': 0.00,
    },
    'party': {
        # Party games MUST be dead simple - complexity is the killer
        'rules_complexity': 0.50,  # Half of fitness! Must explain in 1-2 minutes
        'decision_density': 0.04,
        'comeback_potential': 0.12,  # Everyone can win
        'tension_curve': 0.06,
        'interaction_frequency': 0.14,  # High interaction
        'skill_vs_luck': 0.04,  # Luck-friendly
        'bluffing_depth': 0.00,
        'betting_engagement': 0.10,
    },
    'trick-taking': {
        # Trick-taking is familiar, so complexity is less of a barrier
        'rules_complexity': 0.30,  # Familiar pattern helps, but still important
        'decision_density': 0.15,
        'comeback_potential': 0.10,
        'tension_curve': 0.12,
        'interaction_frequency': 0.18,
        'skill_vs_luck': 0.15,
        'bluffing_depth': 0.00,
        'betting_engagement': 0.00,
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

    # Betting metrics (BettingPhase games)
    total_bets: int = 0
    betting_bluffs: int = 0
    fold_wins: int = 0
    showdown_wins: int = 0
    all_in_count: int = 0

    # Tension curve metrics
    lead_changes: int = 0
    decisive_turn_pct: float = 1.0
    closest_margin: float = 1.0
    trailing_winners: int = 0  # Games where winner was behind at midpoint

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
    betting_engagement: float   # Psychological appeal of betting (0 for non-betting games)
    total_fitness: float
    games_simulated: int
    valid: bool


@dataclass
class FitnessResult:
    """Result of fitness evaluation."""
    fitness: float
    valid: bool
    metrics: dict
    error: Optional[str] = None
    coherence_violations: list[str] = field(default_factory=list)


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

            # Calculate decision quality score based on move variety
            # Key insight: meaningful decisions come from CONSTRAINED choices,
            # not just having lots of options
            if results.total_hand_size > 0:
                moves_per_card = results.total_valid_moves / results.total_hand_size

                if moves_per_card <= 1.0:
                    # Filtered game: fewer valid moves than cards in hand
                    # Higher filtering = more meaningful constraints
                    filtering_score = 1.0 - moves_per_card  # 0.0-1.0
                    variety_score = 0.0
                else:
                    # Multi-option game: more moves than cards (draw, pass, phases)
                    # These ARE meaningful decisions, not unfiltered chaos
                    filtering_score = 0.3  # Baseline - some implicit filtering
                    # Extra options beyond hand = decision variety
                    extra_options = moves_per_card - 1.0
                    variety_score = min(0.5, extra_options * 0.15)
            else:
                filtering_score = 0.0
                variety_score = 0.0

            # Choice score: how many options per decision point
            # More options = more interesting (if not forced)
            raw_choice_score = min(1.0, (avg_valid_moves - 1) / 6.0)

            # KEY INSIGHT: Unconstrained choices are NOT meaningful decisions.
            # If filtering_score is low (no play constraints), having many options
            # doesn't create meaningful decisions - they're all equivalent.
            # Example: War lets you play any of 26 cards, but they're all equivalent
            # because you don't know what opponent will play.
            #
            # Scale choice_score by filtering: more constraints = more meaningful choices
            # With no filtering, choice_score is heavily penalized
            constraint_multiplier = 0.2 + (filtering_score * 0.8)  # Range: 0.2 to 1.0
            choice_score = raw_choice_score * constraint_multiplier

            # Final decision density combines:
            # - Choice availability (scaled by constraints)
            # - Filtering quality (constraints make choices meaningful)
            # - Variety bonus (draw/pass/phase options)
            # - Not being forced
            decision_density = min(1.0, (
                choice_score * 0.35 +           # Constrained choices
                filtering_score * 0.30 +        # Constraint quality (increased weight)
                variety_score +                 # Multi-option bonus (up to 0.5)
                (1.0 - forced_ratio) * 0.20     # Not being forced (reduced weight)
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

        # 2. Comeback potential combines two signals:
        #    a) Win rate balance: Are wins evenly distributed among players?
        #    b) Trailing winner frequency: How often does the trailing player come back to win?

        # 2a. Win rate balance (expected win rate: 1/N for N players)
        expected_rate = 1.0 / results.player_count if results.player_count > 0 else 0.5
        max_deviation = 1.0 - expected_rate  # Maximum possible deviation from expected

        if results.total_games > 0:
            deviations = []
            for wins in results.wins:
                actual_rate = wins / results.total_games
                deviation = abs(actual_rate - expected_rate) / max_deviation if max_deviation > 0 else 0
                deviations.append(deviation)
            avg_deviation = sum(deviations) / len(deviations) if deviations else 0
        else:
            avg_deviation = 0

        balance_score = 1.0 - avg_deviation

        # 2b. Trailing winner frequency: proportion of games where winner was behind at midpoint
        # This is the true "comeback" metric - how often does the trailing player win?
        decisive_games = results.total_games - results.draws - results.errors
        if decisive_games > 0 and results.trailing_winners > 0:
            # trailing_winners / decisive_games gives comeback frequency (0 to 1)
            # 50% comebacks is ideal (maximum uncertainty), so we scale to [0, 1] with 0.5 = optimal
            trailing_freq = results.trailing_winners / decisive_games
            # Transform: 0% comebacks -> 0, 50% comebacks -> 1, 100% comebacks -> 0
            # (100% comebacks means the midpoint leader NEVER wins, also not ideal)
            trailing_score = 1.0 - abs(0.5 - trailing_freq) * 2
        else:
            # No trailing winner data available - fall back to balance score only
            trailing_score = balance_score

        # Combine: 60% trailing winner frequency (true comebacks) + 40% balance
        comeback_potential = trailing_score * 0.6 + balance_score * 0.4

        # 3. Tension curve - use real instrumentation if available
        # Key insight: lead_changes = 0 with closest_margin = 0 means "always tied"
        # which indicates the leader detector couldn't track progress, not high tension.
        #
        # SPECIAL CASE: Betting games derive tension from betting dynamics, not lead changes.
        # Poker tension comes from: pot commitment, all-in moments, fold equity pressure.
        # Use betting activity as a proxy for tension in betting games.
        is_betting_game = results.total_bets > 0
        has_meaningful_tracking = results.lead_changes > 0

        if is_betting_game and not has_meaningful_tracking:
            # Betting game with no lead tracking: use betting-based tension
            # Tension components:
            # 1. Betting activity: bets per decision (more bets = more pressure points)
            # 2. All-in frequency: high-stakes moments (all_in / hands played)
            # 3. Showdown rate: games that went to showdown had sustained tension
            games_played = max(1, results.total_games - results.draws - results.errors)
            bets_per_game = results.total_bets / games_played if games_played > 0 else 0
            all_in_rate = results.all_in_count / games_played if games_played > 0 else 0
            showdown_rate = results.showdown_wins / games_played if games_played > 0 else 0

            # Scoring:
            # - 3+ bets per game = active betting (score 1.0)
            # - All-in moments create peak tension (even 1 per game is significant)
            # - Showdowns indicate sustained uncertainty (didn't fold early)
            bet_activity_score = min(1.0, bets_per_game / 3.0)
            all_in_score = min(1.0, all_in_rate * 2)  # 50% all-in rate = max
            showdown_score = min(1.0, showdown_rate)

            tension_curve = (
                bet_activity_score * 0.4 +
                all_in_score * 0.3 +
                showdown_score * 0.3
            )
        elif has_meaningful_tracking:
            # Real back-and-forth detected - use full formula
            turns_per_expected_change = 20
            expected_changes = max(1, results.avg_turns / turns_per_expected_change)
            lead_change_score = min(1.0, results.lead_changes / expected_changes)
            decisive_turn_score = results.decisive_turn_pct
            margin_score = 1.0 - results.closest_margin

            tension_curve = (
                lead_change_score * 0.4 +
                decisive_turn_score * 0.4 +
                margin_score * 0.2
            )
        elif results.closest_margin > 0 and results.closest_margin < 1.0:
            # No lead changes but non-zero margin = one player always ahead (runaway)
            # Lower tension because outcome was predictable
            margin_score = 1.0 - results.closest_margin  # Smaller margin = closer game
            decisive_score = results.decisive_turn_pct
            tension_curve = margin_score * 0.5 + decisive_score * 0.5
        else:
            # No tracking data (always tied or no meaningful leader detection)
            # Fall back to heuristic based on game length, but cap at 0.6
            # since we can't verify actual back-and-forth tension
            turn_score = min(1.0, results.avg_turns / 100.0)
            length_bonus = min(1.0, max(0.0, (results.avg_turns - 20) / 50.0))
            tension_curve = min(0.6, turn_score * 0.6 + length_bonus * 0.4)

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

        # 5. Rules complexity - cognitive load estimation
        # Uses the complexity module which accounts for:
        # - Condition nesting/conjunction depth
        # - Familiar pattern discounts (trick-taking, draw-play)
        # - Memory requirements (card counting, hidden info)
        # - State tracking (trump suit, direction, betting state)
        from darwindeck.evolution.complexity import get_rules_complexity_score
        rules_complexity = get_rules_complexity_score(genome)

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
                betting_engagement=0.0,
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

        # 8. Bluffing depth - quality of bluffing/betting mechanics
        # Relevant for both ClaimPhase (verbal bluffing) and BettingPhase (betting bluffs)
        bluffing_depth = 0.0

        if results.total_claims > 0:
            # ClaimPhase bluffing (e.g., Cheat/BS)
            bluff_rate = results.total_bluffs / results.total_claims
            challenge_rate = results.total_challenges / results.total_claims

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
                balance_score = 1.0 - abs(bluff_success_rate - 0.5) * 2
                balance_score = max(0.0, min(1.0, balance_score))
            else:
                balance_score = 0.0

            bluffing_depth = (
                bluff_score * 0.3 +
                challenge_score * 0.3 +
                balance_score * 0.4
            )

        elif results.total_bets > 0:
            # BettingPhase bluffing (e.g., Poker, Blackjack)
            # Good betting games have:
            # - Some betting bluffs (betting with weak hands creates uncertainty)
            # - Mix of fold wins and showdown wins (bluffs work sometimes)
            # - Reasonable all-in frequency (not too rare, not every hand)

            # Betting bluff rate: Best around 20-40% (some honest bets, some bluffs)
            betting_bluff_rate = results.betting_bluffs / results.total_bets
            bluff_score = 1.0 - abs(betting_bluff_rate - 0.3) * 3
            bluff_score = max(0.0, min(1.0, bluff_score))

            # Fold win ratio: Best around 30-50% (bluffs sometimes work)
            total_wins = results.fold_wins + results.showdown_wins
            if total_wins > 0:
                fold_win_rate = results.fold_wins / total_wins
                # Best around 35% - bluffs work, but showdowns are common
                fold_score = 1.0 - abs(fold_win_rate - 0.35) * 3
                fold_score = max(0.0, min(1.0, fold_score))
            else:
                fold_score = 0.0

            # All-in frequency: Best around 5-15% of bets (dramatic but not constant)
            all_in_rate = results.all_in_count / results.total_bets
            # Best around 10%
            all_in_score = 1.0 - abs(all_in_rate - 0.10) * 10
            all_in_score = max(0.0, min(1.0, all_in_score))

            bluffing_depth = (
                bluff_score * 0.35 +     # Quality of betting bluffs
                fold_score * 0.40 +      # Bluffs work sometimes (most important)
                all_in_score * 0.25      # Dramatic moments without excess
            )

        # 9. Betting engagement - psychological appeal of betting games
        # Captures the addictive reward loop that makes blackjack/poker popular
        # regardless of strategic depth
        betting_engagement = 0.0

        if results.total_bets > 0:
            total_games = results.total_games
            total_wins = sum(results.wins)

            # Resolution rate: games should have winners, not endless draws
            # This is key for blackjack where random AI often leads to double-busts
            resolution_rate = total_wins / total_games if total_games > 0 else 0
            resolution_score = min(1.0, resolution_rate * 1.5)  # Scale up, cap at 1.0

            # All-in drama: occasional dramatic moments are exciting
            # Ideal around 10-20% of games have an all-in
            all_in_rate = results.all_in_count / total_games if total_games > 0 else 0
            if all_in_rate < 0.05:
                drama_score = all_in_rate / 0.05  # Too few all-ins
            elif all_in_rate <= 0.25:
                drama_score = 1.0  # Sweet spot
            else:
                drama_score = max(0.3, 1.0 - (all_in_rate - 0.25) * 2)  # Too many

            # Betting activity: enough betting decisions to be engaging
            bets_per_game = results.total_bets / total_games if total_games > 0 else 0
            if bets_per_game < 2:
                activity_score = bets_per_game / 2  # Too few bets
            elif bets_per_game <= 20:
                activity_score = 1.0  # Good activity level
            else:
                activity_score = max(0.5, 1.0 - (bets_per_game - 20) / 50)  # Diminishing returns

            # Win variance: back-and-forth is more engaging than one-sided
            # Check if wins are reasonably balanced (not 100-0)
            if total_wins > 0:
                max_wins = max(results.wins)
                balance = 1.0 - (max_wins / total_wins)  # 0 = one player wins all, 0.5 = even
                variance_score = balance * 2  # Scale to 0-1, perfect balance = 1.0
            else:
                variance_score = 0.5  # No data, neutral

            # Showdown excitement: mix of showdowns and folds is ideal
            total_resolved = results.fold_wins + results.showdown_wins
            if total_resolved > 0:
                showdown_rate = results.showdown_wins / total_resolved
                # Ideal around 70-80% showdowns (some bluffs work, but not too many)
                showdown_score = 1.0 - abs(showdown_rate - 0.75) * 2
                showdown_score = max(0.0, min(1.0, showdown_score))
            else:
                showdown_score = 0.5  # No data, neutral

            betting_engagement = (
                resolution_score * 0.30 +    # Games resolve with winners
                drama_score * 0.20 +         # Occasional all-in drama
                activity_score * 0.15 +      # Enough betting action
                variance_score * 0.15 +      # Back-and-forth wins
                showdown_score * 0.20        # Mix of showdowns and folds
            )

        # Check validity
        valid = results.errors == 0 and results.total_games > 0

        # Compute weighted total (session_length removed from average)
        # 8 metrics now (session_length is a constraint)
        #
        # KEY INSIGHT: Tension × decision_density as INTERACTION TERM
        # High tension only matters if you can ACT on it.
        # War has dramatic lead changes (0.87 tension) but zero decisions (0.27 density)
        # → tension contribution = 0.87 × 0.27 = 0.23 (heavily penalized)
        # Spades has tension (0.98) AND decisions (0.41)
        # → tension contribution = 0.98 × 0.41 = 0.40 (properly rewarded)
        effective_tension = tension_curve * decision_density

        total_fitness = (
            self.weights['decision_density'] * decision_density +
            self.weights['comeback_potential'] * comeback_potential +
            self.weights['tension_curve'] * effective_tension +
            self.weights['interaction_frequency'] * interaction_frequency +
            self.weights['rules_complexity'] * rules_complexity +
            self.weights['skill_vs_luck'] * skill_vs_luck +
            self.weights['bluffing_depth'] * bluffing_depth +
            self.weights['betting_engagement'] * betting_engagement
        )

        # QUALITY GATES: Apply multiplier penalties for games failing minimum thresholds
        # These are the best discriminators between known games and random garbage
        # Random games typically score: comeback~0.05, skill~0.27, tension~0.43
        # Known games typically score: comeback~0.71, skill~0.53, tension~0.63
        quality_multiplier = 1.0

        # Comeback potential: Random games almost always score ~0 here
        # Threshold 0.15 catches most broken games while allowing edge cases
        if comeback_potential < 0.15:
            quality_multiplier *= 0.5  # 50% penalty for no comeback

        # Skill vs luck: Random games average 0.27, known average 0.53
        # Below 0.15 means game is essentially pure luck
        if skill_vs_luck < 0.15:
            quality_multiplier *= 0.7  # 30% penalty for pure luck

        # One-sidedness check: If one player wins >80% of games, it's broken
        # (This catches degenerate games that always favor first/second player)
        if results.total_games > 0 and len(results.wins) >= 2:
            max_win_rate = max(results.wins) / results.total_games
            if max_win_rate > 0.80:
                quality_multiplier *= 0.6  # 40% penalty for one-sided

        total_fitness *= quality_multiplier

        return FitnessMetrics(
            decision_density=decision_density,
            comeback_potential=comeback_potential,
            tension_curve=tension_curve,
            interaction_frequency=interaction_frequency,
            rules_complexity=rules_complexity,
            session_length=session_length,  # Keep for reporting
            skill_vs_luck=skill_vs_luck,
            bluffing_depth=bluffing_depth,
            betting_engagement=betting_engagement,
            total_fitness=total_fitness,
            games_simulated=results.total_games,
            valid=valid
        )


class FullFitnessEvaluator:
    """Evaluates game fitness with coherence check before simulation.

    This evaluator performs semantic coherence checking before running
    any simulations. Incoherent genomes get fitness=0 immediately,
    saving simulation costs.

    Usage:
        evaluator = FullFitnessEvaluator()
        result = evaluator.evaluate(genome)

        if not result.valid:
            print(f"Incoherent: {result.coherence_violations}")
    """

    def __init__(
        self,
        style: Optional[str] = None,
        num_simulations: int = 100,
        use_mcts: bool = False,
    ):
        """Initialize the full fitness evaluator.

        Args:
            style: Style preset name (balanced, bluffing, strategic, party, trick-taking)
            num_simulations: Number of simulations per evaluation
            use_mcts: Whether to use MCTS AI for skill measurement
        """
        from darwindeck.evolution.coherence import SemanticCoherenceChecker
        from darwindeck.simulation.go_simulator import GoSimulator

        self.coherence_checker = SemanticCoherenceChecker()
        self.fitness_evaluator = FitnessEvaluator(style=style)
        self.simulator = GoSimulator()
        self.num_simulations = num_simulations
        self.use_mcts = use_mcts

    def evaluate(self, genome: GameGenome) -> FitnessResult:
        """Evaluate genome fitness.

        Checks semantic coherence first. If genome is incoherent,
        returns fitness=0 without running simulations.

        Args:
            genome: Game genome to evaluate

        Returns:
            FitnessResult with fitness score and coherence violations
        """
        # Check coherence FIRST (fast, no simulation needed)
        coherence = self.coherence_checker.check(genome)
        if not coherence.coherent:
            return FitnessResult(
                fitness=0.0,
                valid=False,
                metrics={},
                error=f"Incoherent: {'; '.join(coherence.violations)}",
                coherence_violations=coherence.violations,
            )

        # Run simulations
        try:
            sim_results = self.simulator.simulate(
                genome,
                num_games=self.num_simulations,
                use_mcts=self.use_mcts,
            )

            # Evaluate fitness
            metrics = self.fitness_evaluator.evaluate(
                genome, sim_results, use_mcts=self.use_mcts
            )

            return FitnessResult(
                fitness=metrics.total_fitness,
                valid=metrics.valid,
                metrics={
                    "decision_density": metrics.decision_density,
                    "comeback_potential": metrics.comeback_potential,
                    "tension_curve": metrics.tension_curve,
                    "interaction_frequency": metrics.interaction_frequency,
                    "rules_complexity": metrics.rules_complexity,
                    "session_length": metrics.session_length,
                    "skill_vs_luck": metrics.skill_vs_luck,
                    "bluffing_depth": metrics.bluffing_depth,
                    "betting_engagement": metrics.betting_engagement,
                },
                coherence_violations=[],
            )
        except Exception as e:
            return FitnessResult(
                fitness=0.0,
                valid=False,
                metrics={},
                error=f"Simulation error: {str(e)}",
                coherence_violations=[],
            )
