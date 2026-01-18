"""Genome validation to catch invalid field combinations."""

from typing import List
from darwindeck.genome.schema import (
    GameGenome, BettingPhase, HandEvaluationMethod, TableauMode,
    ShowdownMethod, TrickPhase, PlayPhase, DiscardPhase, DrawPhase,
)

# Standard deck size
STANDARD_DECK_SIZE = 52


class GenomeValidator:
    """Validates genome consistency at parse time."""

    @staticmethod
    def validate(genome: GameGenome) -> List[str]:
        """Return list of validation errors (empty = valid)."""
        errors: List[str] = []

        # Check 0: Setup requires valid number of cards
        cards_needed = genome.setup.cards_per_player * genome.player_count
        cards_needed += genome.setup.initial_discard_count
        if cards_needed > STANDARD_DECK_SIZE:
            errors.append(
                f"Setup requires {cards_needed} cards but deck only has {STANDARD_DECK_SIZE}"
            )

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

        # Check 7: Game must have card play phases (not just betting)
        card_play_phases = (TrickPhase, PlayPhase, DiscardPhase, DrawPhase)
        has_card_play = any(
            isinstance(p, card_play_phases)
            for p in genome.turn_structure.phases
        )
        if not has_card_play:
            errors.append(
                "Game has no card play phases (needs TrickPhase, PlayPhase, DiscardPhase, or DrawPhase)"
            )

        # Check 8: Betting min_bet should allow meaningful play
        for phase in genome.turn_structure.phases:
            if isinstance(phase, BettingPhase):
                starting = genome.setup.starting_chips
                if starting > 0 and phase.min_bet > 0:
                    # If min_bet > starting_chips / 2, players can only bet once
                    # This allows 50/100 (2 bets possible) but catches 67/100 (1 bet)
                    if phase.min_bet > starting // 2:
                        errors.append(
                            f"BettingPhase min_bet ({phase.min_bet}) is too high "
                            f"relative to starting_chips ({starting}) - limits meaningful betting"
                        )

        return errors
