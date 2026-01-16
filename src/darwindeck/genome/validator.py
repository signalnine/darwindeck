"""Genome validation to catch invalid field combinations."""

from typing import List
from darwindeck.genome.schema import (
    GameGenome, BettingPhase, HandEvaluationMethod, TableauMode,
    ShowdownMethod,
)


class GenomeValidator:
    """Validates genome consistency at parse time."""

    @staticmethod
    def validate(genome: GameGenome) -> List[str]:
        """Return list of validation errors (empty = valid)."""
        errors: List[str] = []

        # Get win condition types
        win_types = {wc.type for wc in genome.win_conditions}

        # Check 1: Score-based wins require scoring rules
        score_wins = {"high_score", "low_score", "first_to_score"}
        if win_types & score_wins:
            has_scoring = bool(genome.card_scoring) or bool(genome.scoring_rules)
            if not has_scoring:
                errors.append(
                    "Score-based win condition requires card_scoring or scoring_rules"
                )

        # Check 2: best_hand win requires hand_evaluation with PATTERN_MATCH
        if "best_hand" in win_types:
            has_pattern_eval = (
                genome.hand_evaluation is not None
                and genome.hand_evaluation.method == HandEvaluationMethod.PATTERN_MATCH
            )
            if not has_pattern_eval:
                errors.append(
                    "best_hand win condition requires hand_evaluation with PATTERN_MATCH"
                )

        # Check 3: Betting phase requires starting_chips > 0
        has_betting = any(
            isinstance(p, BettingPhase)
            for p in genome.turn_structure.phases
        )
        if has_betting and genome.setup.starting_chips <= 0:
            errors.append(
                "BettingPhase requires setup.starting_chips > 0"
            )

        # Check 4: Betting showdown=HAND_EVALUATION requires hand_evaluation
        for phase in genome.turn_structure.phases:
            if isinstance(phase, BettingPhase):
                if phase.showdown_method == ShowdownMethod.HAND_EVALUATION:
                    if genome.hand_evaluation is None:
                        errors.append(
                            "BettingPhase with HAND_EVALUATION showdown requires hand_evaluation"
                        )

        # Check 5: Capture wins require capture mechanic
        capture_wins = {"capture_all", "most_captured"}
        if win_types & capture_wins:
            has_capture = genome.setup.tableau_mode in {TableauMode.WAR, TableauMode.MATCH_RANK}
            if not has_capture:
                errors.append(
                    "Capture win condition requires tableau_mode WAR or MATCH_RANK"
                )

        # Check 6: HandPattern constraints must be internally consistent
        if genome.hand_evaluation and genome.hand_evaluation.patterns:
            for pattern in genome.hand_evaluation.patterns:
                if pattern.same_rank_groups and pattern.required_count:
                    group_sum = sum(pattern.same_rank_groups)
                    if group_sum > pattern.required_count:
                        errors.append(
                            f"HandPattern '{pattern.name}': same_rank_groups sum "
                            f"({group_sum}) exceeds required_count ({pattern.required_count})"
                        )

        return errors
