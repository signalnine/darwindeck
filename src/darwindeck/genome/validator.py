"""Genome validation to catch invalid field combinations."""

from typing import List, Set
from darwindeck.genome.schema import (
    GameGenome, BettingPhase, HandEvaluationMethod, TableauMode,
    ShowdownMethod, TrickPhase, PlayPhase, DiscardPhase, DrawPhase,
    BiddingPhase,
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

        # Check 9: Team configuration validation
        GenomeValidator._validate_teams(genome, errors)

        # Check 10: Bidding configuration validation
        errors.extend(GenomeValidator._validate_bidding(genome))

        return errors

    @staticmethod
    def _validate_bidding(genome: GameGenome) -> List[str]:
        """Validate bidding phase configuration."""
        errors: List[str] = []

        has_bidding_phase = any(
            isinstance(p, BiddingPhase) for p in genome.turn_structure.phases
        )
        has_trick_phase = any(
            isinstance(p, TrickPhase) for p in genome.turn_structure.phases
        )

        if has_bidding_phase and not has_trick_phase:
            errors.append("BiddingPhase requires at least one TrickPhase (contracts need tricks)")

        if genome.contract_scoring is not None and not has_bidding_phase:
            errors.append("ContractScoring requires BiddingPhase")

        return errors

    @staticmethod
    def _validate_teams(genome: GameGenome, errors: List[str]) -> None:
        """Validate team configuration if team_mode is enabled."""
        if not genome.team_mode:
            return  # Skip validation if team_mode is False

        num_players = genome.player_count

        # Must have at least 2 teams
        if len(genome.teams) < 2:
            errors.append(f"Team mode requires at least 2 teams, got {len(genome.teams)}")
            return

        # Collect all player indices
        all_players: Set[int] = set()
        for team_idx, team in enumerate(genome.teams):
            if len(team) == 0:
                errors.append(f"Team {team_idx} is empty")
                continue
            for player_idx in team:
                # Check for out-of-range
                if player_idx < 0 or player_idx >= num_players:
                    errors.append(
                        f"Player index {player_idx} out of range [0, {num_players})"
                    )
                # Check for duplicates
                if player_idx in all_players:
                    errors.append(
                        f"Duplicate player {player_idx} appears in multiple teams"
                    )
                all_players.add(player_idx)

        # Check all players are assigned
        expected_players = set(range(num_players))
        missing = expected_players - all_players
        if missing:
            errors.append(f"Players not assigned to any team: {sorted(missing)}")
