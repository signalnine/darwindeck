"""Tests to verify genomes are self-describing without implicit simulator mechanics.

These tests ensure that a genome contains all information needed to play the game
without relying on hardcoded behaviors in the Go simulator.
"""

import pytest
from dataclasses import dataclass
from typing import Optional
from enum import Enum

from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    TrickPhase, BettingPhase, PlayPhase, DrawPhase, ClaimPhase,
    SpecialEffect, EffectType, Rank, Location, TableauMode,
)
from darwindeck.genome.examples import (
    create_war_genome, create_scopa_genome, create_uno_genome,
    create_simple_poker_genome, get_seed_genomes,
)


class IncompleteDependency(Enum):
    """Types of implicit simulator dependencies."""
    HEARTS_SCORING = "hearts_scoring"  # Relies on hardcoded 1pt/heart, 13pt/QS
    BLACKJACK_VALUATION = "blackjack_valuation"  # Relies on hardcoded card values
    POKER_HAND_RANKING = "poker_hand_ranking"  # Relies on hardcoded hand hierarchy
    TRICK_WINNER_SCORING = "trick_winner_scoring"  # No explicit scoring for winning tricks
    SCORE_WIN_NO_SCORING = "score_win_no_scoring"  # Score-based win but no scoring rules
    CAPTURE_WIN_NO_CAPTURE = "capture_win_no_capture"  # Capture win but no capture mechanic
    BETTING_NO_SHOWDOWN = "betting_no_showdown"  # Betting but no showdown resolution
    THRESHOLD_21_BLACKJACK = "threshold_21_blackjack"  # Relies on implicit blackjack detection
    CLAIM_RANK_IMPLICIT = "claim_rank_implicit"  # Claim phase but rank is turn-number derived


@dataclass
class CompletenessResult:
    """Result of genome completeness check."""
    complete: bool
    dependencies: list[IncompleteDependency]
    warnings: list[str]

    def __str__(self) -> str:
        if self.complete:
            return "Genome is self-describing"
        deps = ", ".join(d.value for d in self.dependencies)
        return f"Incomplete - relies on: {deps}"


def check_genome_completeness(genome: GameGenome) -> CompletenessResult:
    """Check if a genome is self-describing without implicit mechanics.

    Returns CompletenessResult with list of implicit dependencies found.
    """
    dependencies = []
    warnings = []

    # Get win condition types
    win_types = {wc.type for wc in genome.win_conditions}

    # Get phase types
    has_trick_phase = any(isinstance(p, TrickPhase) for p in genome.turn_structure.phases)
    has_betting_phase = any(isinstance(p, BettingPhase) for p in genome.turn_structure.phases)
    has_claim_phase = any(isinstance(p, ClaimPhase) for p in genome.turn_structure.phases)
    has_play_to_tableau = any(
        isinstance(p, PlayPhase) and p.target == Location.TABLEAU
        for p in genome.turn_structure.phases
    )

    # Check 1: Score-based win conditions require explicit scoring rules
    score_win_types = {"high_score", "low_score", "first_to_score"}
    if win_types & score_win_types:
        if not genome.scoring_rules:
            # Trick-taking games rely on implicit Hearts scoring
            if has_trick_phase:
                dependencies.append(IncompleteDependency.HEARTS_SCORING)
                dependencies.append(IncompleteDependency.TRICK_WINNER_SCORING)
            else:
                dependencies.append(IncompleteDependency.SCORE_WIN_NO_SCORING)

    # Check 2: Threshold=21 triggers implicit blackjack detection
    for wc in genome.win_conditions:
        if wc.type == "high_score" and wc.threshold == 21:
            dependencies.append(IncompleteDependency.THRESHOLD_21_BLACKJACK)
            dependencies.append(IncompleteDependency.BLACKJACK_VALUATION)
            warnings.append("Threshold 21 triggers implicit blackjack card valuation")

    # Check 3: best_hand win condition relies on poker hand ranking
    if "best_hand" in win_types:
        dependencies.append(IncompleteDependency.POKER_HAND_RANKING)
        warnings.append("best_hand win condition relies on hardcoded poker hand rankings")

    # Check 4: Capture-based wins need explicit capture mechanic
    capture_win_types = {"capture_all", "most_captured"}
    if win_types & capture_win_types:
        # Must have WAR or MATCH_RANK tableau mode
        if genome.setup.tableau_mode == TableauMode.NONE:
            if not has_play_to_tableau:
                dependencies.append(IncompleteDependency.CAPTURE_WIN_NO_CAPTURE)
                warnings.append("Capture win condition but no capture mechanic defined")

    # Check 5: Betting without showdown resolution
    if has_betting_phase:
        # Need either best_hand or explicit scoring for showdown
        if "best_hand" not in win_types and not genome.scoring_rules:
            dependencies.append(IncompleteDependency.BETTING_NO_SHOWDOWN)
            warnings.append("Betting phase but no showdown resolution defined")

    # Check 6: Claim phase rank is implicitly derived from turn number
    if has_claim_phase:
        dependencies.append(IncompleteDependency.CLAIM_RANK_IMPLICIT)
        warnings.append("Claim phase rank is derived from turn number, not explicit")

    # Check 7: all_hands_empty with trick phase assumes Hearts-style lowest wins
    if "all_hands_empty" in win_types and has_trick_phase:
        if not genome.scoring_rules:
            dependencies.append(IncompleteDependency.HEARTS_SCORING)
            warnings.append("all_hands_empty with tricks assumes lowest score wins (Hearts)")

    return CompletenessResult(
        complete=len(dependencies) == 0,
        dependencies=dependencies,
        warnings=warnings,
    )


class TestGenomeCompleteness:
    """Test that genomes don't rely on implicit simulator mechanics."""

    def test_war_genome_is_complete(self):
        """War genome should be self-describing (capture-based, no scoring)."""
        genome = create_war_genome()
        result = check_genome_completeness(genome)

        # War is actually complete - it uses capture_all win condition
        # and WAR tableau mode for comparison
        assert result.complete, f"War genome incomplete: {result}"

    def test_scopa_genome_is_complete(self):
        """Scopa genome should be self-describing."""
        genome = create_scopa_genome()
        result = check_genome_completeness(genome)

        # Scopa uses MATCH_RANK and most_captured - should be complete
        assert result.complete, f"Scopa genome incomplete: {result}"

    def test_uno_genome_completeness(self):
        """UNO genome - check for implicit dependencies."""
        genome = create_uno_genome()
        result = check_genome_completeness(genome)

        # UNO uses empty_hand win condition, no scoring
        # Should be complete since it's a shedding game
        assert result.complete, f"UNO genome incomplete: {result}"

    def test_poker_genome_has_implicit_hand_ranking(self):
        """Simple poker relies on implicit poker hand ranking."""
        genome = create_simple_poker_genome()
        result = check_genome_completeness(genome)

        # Poker with best_hand MUST rely on implicit hand ranking
        assert IncompleteDependency.POKER_HAND_RANKING in result.dependencies
        assert not result.complete

    def test_trick_taking_with_score_win_is_incomplete(self):
        """Trick-taking games with score wins rely on implicit Hearts scoring."""
        # Create a generic trick-taking game with low_score win
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_trick_score",
            generation=0,
            setup=SetupRules(cards_per_player=13),
            turn_structure=TurnStructure(
                phases=(TrickPhase(lead_suit_required=True, high_card_wins=True),),
            ),
            win_conditions=(WinCondition(type="low_score", threshold=100),),
            special_effects=(),
            scoring_rules=(),  # No explicit scoring!
            max_turns=200,
            player_count=2,
        )

        result = check_genome_completeness(genome)

        assert IncompleteDependency.HEARTS_SCORING in result.dependencies
        assert IncompleteDependency.TRICK_WINNER_SCORING in result.dependencies
        assert not result.complete

    def test_threshold_21_triggers_blackjack_detection(self):
        """Games with threshold=21 rely on implicit blackjack detection."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_blackjack_like",
            generation=0,
            setup=SetupRules(cards_per_player=2, starting_chips=100),
            turn_structure=TurnStructure(
                phases=(
                    BettingPhase(min_bet=10, max_raises=1),
                    DrawPhase(source=Location.DECK, count=1, mandatory=False),
                ),
            ),
            win_conditions=(WinCondition(type="high_score", threshold=21),),
            special_effects=(),
            scoring_rules=(),
            max_turns=100,
            player_count=2,
        )

        result = check_genome_completeness(genome)

        assert IncompleteDependency.THRESHOLD_21_BLACKJACK in result.dependencies
        assert IncompleteDependency.BLACKJACK_VALUATION in result.dependencies
        assert not result.complete

    def test_capture_win_without_capture_mechanic(self):
        """Capture wins without WAR/MATCH_RANK mode are incomplete."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_bad_capture",
            generation=0,
            setup=SetupRules(
                cards_per_player=10,
                tableau_mode=TableauMode.NONE,  # No capture mechanic!
            ),
            turn_structure=TurnStructure(
                phases=(DrawPhase(source=Location.DECK, count=1),),
            ),
            win_conditions=(WinCondition(type="capture_all"),),
            special_effects=(),
            scoring_rules=(),
            max_turns=200,
            player_count=2,
        )

        result = check_genome_completeness(genome)

        assert IncompleteDependency.CAPTURE_WIN_NO_CAPTURE in result.dependencies
        assert not result.complete

    def test_betting_without_showdown_resolution(self):
        """Betting games need explicit showdown rules."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_bad_betting",
            generation=0,
            setup=SetupRules(cards_per_player=5, starting_chips=100),
            turn_structure=TurnStructure(
                phases=(BettingPhase(min_bet=10, max_raises=3),),
            ),
            win_conditions=(WinCondition(type="most_chips"),),
            special_effects=(),
            scoring_rules=(),  # No showdown rules!
            max_turns=100,
            player_count=2,
        )

        result = check_genome_completeness(genome)

        assert IncompleteDependency.BETTING_NO_SHOWDOWN in result.dependencies
        assert not result.complete


class TestSeedGenomeCompleteness:
    """Test all seed genomes for completeness."""

    @pytest.mark.parametrize("genome", get_seed_genomes())
    def test_seed_genome_dependencies_documented(self, genome: GameGenome):
        """Each seed genome should have its dependencies documented."""
        result = check_genome_completeness(genome)

        # This test doesn't fail - it documents what each genome relies on
        if not result.complete:
            print(f"\n{genome.genome_id}: {result}")
            for warning in result.warnings:
                print(f"  - {warning}")


class TestCompletenessChecker:
    """Test the completeness checker itself."""

    def test_complete_shedding_game(self):
        """A shedding game with empty_hand win should be complete."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_shedding",
            generation=0,
            setup=SetupRules(cards_per_player=7),
            turn_structure=TurnStructure(
                phases=(
                    DrawPhase(source=Location.DECK, count=1),
                    PlayPhase(min_cards=1, max_cards=1, target=Location.DISCARD),
                ),
            ),
            win_conditions=(WinCondition(type="empty_hand"),),
            special_effects=(),
            scoring_rules=(),
            max_turns=200,
            player_count=2,
        )

        result = check_genome_completeness(genome)
        assert result.complete

    def test_complete_war_style_game(self):
        """A War-style game with WAR mode and capture_all should be complete."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_war_style",
            generation=0,
            setup=SetupRules(
                cards_per_player=26,
                tableau_mode=TableauMode.WAR,
            ),
            turn_structure=TurnStructure(
                phases=(
                    PlayPhase(min_cards=1, max_cards=1, target=Location.TABLEAU),
                ),
            ),
            win_conditions=(WinCondition(type="capture_all"),),
            special_effects=(),
            scoring_rules=(),
            max_turns=500,
            player_count=2,
        )

        result = check_genome_completeness(genome)
        assert result.complete

    def test_complete_scopa_style_game(self):
        """A Scopa-style game with MATCH_RANK and most_captured should be complete."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_scopa_style",
            generation=0,
            setup=SetupRules(
                cards_per_player=3,
                tableau_mode=TableauMode.MATCH_RANK,
            ),
            turn_structure=TurnStructure(
                phases=(
                    PlayPhase(min_cards=1, max_cards=1, target=Location.TABLEAU),
                ),
            ),
            win_conditions=(WinCondition(type="most_captured"),),
            special_effects=(),
            scoring_rules=(),
            max_turns=200,
            player_count=2,
        )

        result = check_genome_completeness(genome)
        assert result.complete


class TestImplicitMechanicsDocumentation:
    """Document all implicit mechanics for visibility."""

    def test_list_all_implicit_mechanics(self):
        """List all known implicit mechanics in the simulator."""
        # This is a documentation test - it lists what's implicit
        implicit_mechanics = {
            "HEARTS_SCORING": "Trick-taking games score 1pt per Heart, 13pt for QS",
            "BLACKJACK_VALUATION": "Card values: A=1/11, Face=10, Pip=face value",
            "POKER_HAND_RANKING": "Standard poker hand hierarchy (RF > SF > 4K > ...)",
            "TRICK_WINNER_SCORING": "Winning tricks scores points (Hearts-style)",
            "THRESHOLD_21_DETECTION": "Threshold=21 triggers blackjack game mode",
            "CLAIM_RANK_DERIVATION": "Claim phase rank = turn_number % 13",
            "ALL_HANDS_EMPTY_LOWEST_WINS": "Trick games with all_hands_empty: lowest score wins",
            "SEQUENCE_NO_WRAPPING": "SEQUENCE mode: K is end, no K->A wrapping",
            "CONSECUTIVE_PASS_CLEARS": "N-1 consecutive passes clears tableau",
            "BET_MATCHING_IGNORES_FOLDED": "Folded/all-in players excluded from bet matching",
        }

        print("\n=== IMPLICIT SIMULATOR MECHANICS ===")
        for name, desc in implicit_mechanics.items():
            print(f"  {name}: {desc}")

        # This test always passes - it's for documentation
        assert len(implicit_mechanics) > 0
