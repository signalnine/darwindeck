"""Tests for semantic coherence checking."""

import pytest
from darwindeck.evolution.coherence import SemanticCoherenceChecker, CoherenceResult
from darwindeck.genome.examples import create_war_genome
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, PlayPhase, DiscardPhase,
    WinCondition, Location, BettingPhase
)


def _make_genome(
    phases: list,
    win_conditions: list[WinCondition],
    starting_chips: int = 0,
    scoring_rules: list = None,
    is_trick_based: bool = False,
) -> GameGenome:
    """Helper to create test genomes."""
    return GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=5, starting_chips=starting_chips),
        turn_structure=TurnStructure(phases=tuple(phases), is_trick_based=is_trick_based),
        win_conditions=tuple(win_conditions),
        scoring_rules=tuple(scoring_rules or []),
        special_effects=[],
        player_count=2,
    )


class TestCoherenceResult:
    def test_coherent_result(self):
        """Coherent result has no violations."""
        result = CoherenceResult(coherent=True, violations=[])
        assert result.coherent is True
        assert result.violations == []

    def test_incoherent_result(self):
        """Incoherent result has violations."""
        result = CoherenceResult(
            coherent=False,
            violations=["Win condition 'capture_all' requires TABLEAU"]
        )
        assert result.coherent is False
        assert len(result.violations) == 1


class TestSemanticCoherenceChecker:
    def test_checker_returns_result(self):
        """Checker returns CoherenceResult."""
        checker = SemanticCoherenceChecker()
        genome = create_war_genome()
        result = checker.check(genome)
        assert isinstance(result, CoherenceResult)


class TestCaptureWinConditions:
    def test_capture_all_with_tableau_is_coherent(self):
        """capture_all + PlayPhase(target=TABLEAU) = valid."""
        genome = _make_genome(
            phases=[PlayPhase(target=Location.TABLEAU, min_cards=1, max_cards=1)],
            win_conditions=[WinCondition(type="capture_all")],
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is True

    def test_capture_all_without_tableau_is_incoherent(self):
        """capture_all + only discard phases = invalid."""
        genome = _make_genome(
            phases=[DiscardPhase(target=Location.DISCARD, count=1)],
            win_conditions=[WinCondition(type="capture_all")],
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is False
        assert "capture_all" in result.violations[0]
        assert "TABLEAU" in result.violations[0]

    def test_most_captured_without_tableau_is_incoherent(self):
        """most_captured + no tableau = invalid."""
        genome = _make_genome(
            phases=[DiscardPhase(target=Location.DISCARD, count=1)],
            win_conditions=[WinCondition(type="most_captured")],
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is False


class TestScoringWinConditions:
    def test_high_score_with_trick_based_is_coherent(self):
        """high_score + is_trick_based = valid."""
        genome = _make_genome(
            phases=[PlayPhase(target=Location.TABLEAU, min_cards=1, max_cards=1)],
            win_conditions=[WinCondition(type="high_score", threshold=50)],
            is_trick_based=True,
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is True

    def test_high_score_without_scoring_is_incoherent(self):
        """high_score + no scoring_rules + not trick_based = invalid."""
        genome = _make_genome(
            phases=[DiscardPhase(target=Location.DISCARD, count=1)],
            win_conditions=[WinCondition(type="high_score", threshold=50)],
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is False
        assert "high_score" in result.violations[0]

    def test_low_score_without_scoring_is_incoherent(self):
        """low_score + no scoring = invalid."""
        genome = _make_genome(
            phases=[DiscardPhase(target=Location.DISCARD, count=1)],
            win_conditions=[WinCondition(type="low_score", threshold=10)],
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is False

    def test_empty_hand_always_coherent(self):
        """empty_hand works with any configuration."""
        genome = _make_genome(
            phases=[DiscardPhase(target=Location.DISCARD, count=1)],
            win_conditions=[WinCondition(type="empty_hand")],
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is True


class TestResourceCoherence:
    def test_chips_without_betting_is_incoherent(self):
        """starting_chips > 0 but no BettingPhase = invalid."""
        genome = _make_genome(
            phases=[DiscardPhase(target=Location.DISCARD, count=1)],
            win_conditions=[WinCondition(type="empty_hand")],
            starting_chips=1000,
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is False
        assert "starting_chips" in result.violations[0]
        assert "BettingPhase" in result.violations[0]

    def test_chips_with_betting_is_coherent(self):
        """starting_chips > 0 + BettingPhase = valid."""
        genome = _make_genome(
            phases=[
                BettingPhase(min_bet=10, max_raises=3),
                PlayPhase(target=Location.TABLEAU, min_cards=1, max_cards=1),
            ],
            win_conditions=[WinCondition(type="empty_hand")],
            starting_chips=1000,
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is True

    def test_zero_chips_without_betting_is_coherent(self):
        """No chips + no betting = valid (not a betting game)."""
        genome = _make_genome(
            phases=[DiscardPhase(target=Location.DISCARD, count=1)],
            win_conditions=[WinCondition(type="empty_hand")],
            starting_chips=0,
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is True


class TestRealWorldRegressions:
    def test_gentle_blade_is_incoherent(self):
        """GentleBlade genome that prompted this feature should fail.

        Issues: capture_all is OK (has tableau), but high_score without scoring,
        and starting_chips without betting.
        """
        genome = _make_genome(
            phases=[
                DiscardPhase(target=Location.DISCARD, count=1),
                PlayPhase(target=Location.TABLEAU, min_cards=1, max_cards=1),
                PlayPhase(target=Location.TABLEAU, min_cards=1, max_cards=1),
                PlayPhase(target=Location.TABLEAU, min_cards=1, max_cards=1),
            ],
            win_conditions=[
                WinCondition(type="capture_all"),
                WinCondition(type="empty_hand"),
                WinCondition(type="high_score", threshold=89),
            ],
            starting_chips=5252,
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)

        # Should fail due to: high_score without scoring, chips without betting
        assert result.coherent is False
        assert len(result.violations) >= 2

        # Check specific violations
        violation_text = " ".join(result.violations)
        assert "high_score" in violation_text
        assert "starting_chips" in violation_text or "BettingPhase" in violation_text
