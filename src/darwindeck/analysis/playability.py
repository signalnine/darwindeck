"""Playability analysis for game genomes.

Determines if a game is meaningfully playable by checking structural
and gameplay requirements beyond simple error rates.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from darwindeck.genome.schema import GameGenome
from darwindeck.evolution.fitness_full import SimulationResults


@dataclass
class PlayabilityIssue:
    """A specific playability problem."""
    code: str           # Short identifier (e.g., "NO_WINNER")
    severity: str       # "critical", "major", "minor"
    description: str    # Human-readable explanation
    value: float        # The problematic value
    threshold: float    # The threshold it failed


@dataclass
class PlayabilityReport:
    """Complete playability analysis for a game."""
    playable: bool                          # Overall verdict
    score: float                            # 0.0-1.0 playability score
    issues: list[PlayabilityIssue] = field(default_factory=list)

    # Raw metrics for debugging
    error_rate: float = 0.0
    draw_rate: float = 0.0
    decisions_per_game: float = 0.0
    forced_move_rate: float = 0.0
    avg_choices: float = 0.0
    avg_turns: float = 0.0
    max_win_rate: float = 0.0

    def summary(self) -> str:
        """One-line summary of playability."""
        if self.playable:
            return f"PLAYABLE (score={self.score:.2f})"
        issues_str = ", ".join(i.code for i in self.issues if i.severity == "critical")
        return f"UNPLAYABLE: {issues_str}"


# Thresholds for playability checks
THRESHOLDS = {
    # Critical issues (game is broken)
    "error_rate_critical": 0.50,      # >50% errors = broken
    "draw_rate_critical": 0.95,       # >95% draws = no winner possible
    "decisions_min_critical": 1.0,    # <1 decision/game = not a game
    "turns_min_critical": 2.0,        # <2 turns = ends immediately

    # Major issues (game is poor quality)
    "error_rate_major": 0.20,         # >20% errors = unstable
    "draw_rate_major": 0.50,          # >50% draws = winner is rare
    "forced_rate_major": 0.95,        # >95% forced = no agency
    "choices_min_major": 1.5,         # <1.5 avg choices = limited options
    "one_sided_major": 0.80,          # >80% one player wins = unbalanced
    "turns_max_major": 500,           # >500 turns = too long

    # Minor issues (game could be better)
    "error_rate_minor": 0.05,         # >5% errors = some instability
    "draw_rate_minor": 0.20,          # >20% draws = often inconclusive
    "forced_rate_minor": 0.70,        # >70% forced = mostly scripted
    "decisions_low_minor": 10.0,      # <10 decisions = very simple
}


class PlayabilityChecker:
    """Evaluates whether a game is meaningfully playable."""

    def __init__(
        self,
        num_games: int = 50,
        strict: bool = False,
    ):
        """Initialize checker.

        Args:
            num_games: Games to simulate for analysis
            strict: If True, major issues also count as unplayable
        """
        self.num_games = num_games
        self.strict = strict

    def check(
        self,
        genome: GameGenome,
        results: Optional[SimulationResults] = None,
    ) -> PlayabilityReport:
        """Check if genome produces a playable game.

        Args:
            genome: Game genome to check
            results: Pre-computed simulation results (optional)

        Returns:
            PlayabilityReport with verdict and issues
        """
        # Run simulation if results not provided
        if results is None:
            from darwindeck.simulation.go_simulator import GoSimulator
            simulator = GoSimulator(seed=42)
            results = simulator.simulate(genome, num_games=self.num_games)

        # Calculate metrics
        total_games = results.total_games
        if total_games == 0:
            return PlayabilityReport(
                playable=False,
                score=0.0,
                issues=[PlayabilityIssue(
                    code="NO_GAMES",
                    severity="critical",
                    description="No games completed",
                    value=0,
                    threshold=1,
                )],
            )

        error_rate = results.errors / total_games
        draw_rate = results.draws / total_games
        decisions_per_game = results.total_decisions / total_games
        avg_turns = results.avg_turns

        forced_rate = (
            results.forced_decisions / results.total_decisions
            if results.total_decisions > 0 else 1.0
        )
        avg_choices = (
            results.total_valid_moves / results.total_decisions
            if results.total_decisions > 0 else 0.0
        )
        max_win_rate = (
            max(results.wins) / total_games
            if results.wins else 0.0
        )

        # Collect issues
        issues: list[PlayabilityIssue] = []

        # Critical checks
        if error_rate > THRESHOLDS["error_rate_critical"]:
            issues.append(PlayabilityIssue(
                code="HIGH_ERRORS",
                severity="critical",
                description=f"Too many simulation errors ({error_rate:.0%})",
                value=error_rate,
                threshold=THRESHOLDS["error_rate_critical"],
            ))

        if draw_rate > THRESHOLDS["draw_rate_critical"]:
            issues.append(PlayabilityIssue(
                code="NO_WINNER",
                severity="critical",
                description=f"Game almost never produces a winner ({draw_rate:.0%} draws)",
                value=draw_rate,
                threshold=THRESHOLDS["draw_rate_critical"],
            ))

        if decisions_per_game < THRESHOLDS["decisions_min_critical"]:
            issues.append(PlayabilityIssue(
                code="NO_DECISIONS",
                severity="critical",
                description=f"Players make almost no decisions ({decisions_per_game:.1f}/game)",
                value=decisions_per_game,
                threshold=THRESHOLDS["decisions_min_critical"],
            ))

        if avg_turns < THRESHOLDS["turns_min_critical"]:
            issues.append(PlayabilityIssue(
                code="TOO_SHORT",
                severity="critical",
                description=f"Game ends immediately ({avg_turns:.1f} avg turns)",
                value=avg_turns,
                threshold=THRESHOLDS["turns_min_critical"],
            ))

        # Major checks
        if error_rate > THRESHOLDS["error_rate_major"]:
            issues.append(PlayabilityIssue(
                code="UNSTABLE",
                severity="major",
                description=f"Significant error rate ({error_rate:.0%})",
                value=error_rate,
                threshold=THRESHOLDS["error_rate_major"],
            ))

        if draw_rate > THRESHOLDS["draw_rate_major"]:
            issues.append(PlayabilityIssue(
                code="MANY_DRAWS",
                severity="major",
                description=f"Winner is rare ({draw_rate:.0%} draws)",
                value=draw_rate,
                threshold=THRESHOLDS["draw_rate_major"],
            ))

        if forced_rate > THRESHOLDS["forced_rate_major"]:
            issues.append(PlayabilityIssue(
                code="NO_AGENCY",
                severity="major",
                description=f"Almost all moves are forced ({forced_rate:.0%})",
                value=forced_rate,
                threshold=THRESHOLDS["forced_rate_major"],
            ))

        if avg_choices < THRESHOLDS["choices_min_major"]:
            issues.append(PlayabilityIssue(
                code="NO_CHOICE",
                severity="major",
                description=f"Too few options per decision ({avg_choices:.1f} avg)",
                value=avg_choices,
                threshold=THRESHOLDS["choices_min_major"],
            ))

        if max_win_rate > THRESHOLDS["one_sided_major"]:
            issues.append(PlayabilityIssue(
                code="ONE_SIDED",
                severity="major",
                description=f"One player wins too often ({max_win_rate:.0%})",
                value=max_win_rate,
                threshold=THRESHOLDS["one_sided_major"],
            ))

        if avg_turns > THRESHOLDS["turns_max_major"]:
            issues.append(PlayabilityIssue(
                code="TOO_LONG",
                severity="major",
                description=f"Games take too long ({avg_turns:.0f} avg turns)",
                value=avg_turns,
                threshold=THRESHOLDS["turns_max_major"],
            ))

        # Minor checks
        if error_rate > THRESHOLDS["error_rate_minor"]:
            issues.append(PlayabilityIssue(
                code="SOME_ERRORS",
                severity="minor",
                description=f"Some simulation errors ({error_rate:.0%})",
                value=error_rate,
                threshold=THRESHOLDS["error_rate_minor"],
            ))

        if draw_rate > THRESHOLDS["draw_rate_minor"]:
            issues.append(PlayabilityIssue(
                code="SOME_DRAWS",
                severity="minor",
                description=f"Notable draw rate ({draw_rate:.0%})",
                value=draw_rate,
                threshold=THRESHOLDS["draw_rate_minor"],
            ))

        if forced_rate > THRESHOLDS["forced_rate_minor"]:
            issues.append(PlayabilityIssue(
                code="MOSTLY_FORCED",
                severity="minor",
                description=f"Most moves are forced ({forced_rate:.0%})",
                value=forced_rate,
                threshold=THRESHOLDS["forced_rate_minor"],
            ))

        if decisions_per_game < THRESHOLDS["decisions_low_minor"]:
            issues.append(PlayabilityIssue(
                code="FEW_DECISIONS",
                severity="minor",
                description=f"Few decisions per game ({decisions_per_game:.1f})",
                value=decisions_per_game,
                threshold=THRESHOLDS["decisions_low_minor"],
            ))

        # Determine overall playability
        critical_issues = [i for i in issues if i.severity == "critical"]
        major_issues = [i for i in issues if i.severity == "major"]

        if critical_issues:
            playable = False
        elif self.strict and major_issues:
            playable = False
        else:
            playable = True

        # Calculate playability score (0-1)
        score = self._calculate_score(
            error_rate, draw_rate, decisions_per_game,
            forced_rate, avg_choices, avg_turns, max_win_rate
        )

        return PlayabilityReport(
            playable=playable,
            score=score,
            issues=issues,
            error_rate=error_rate,
            draw_rate=draw_rate,
            decisions_per_game=decisions_per_game,
            forced_move_rate=forced_rate,
            avg_choices=avg_choices,
            avg_turns=avg_turns,
            max_win_rate=max_win_rate,
        )

    def _calculate_score(
        self,
        error_rate: float,
        draw_rate: float,
        decisions_per_game: float,
        forced_rate: float,
        avg_choices: float,
        avg_turns: float,
        max_win_rate: float,
    ) -> float:
        """Calculate 0-1 playability score."""
        score = 1.0

        # Penalties for each issue (multiplicative)

        # Error rate: 0% = 1.0, 50% = 0.0
        error_penalty = max(0.0, 1.0 - error_rate * 2)
        score *= error_penalty

        # Draw rate: 0% = 1.0, 100% = 0.0
        draw_penalty = max(0.0, 1.0 - draw_rate)
        score *= draw_penalty

        # Decisions: 0 = 0.0, 10+ = 1.0
        decision_score = min(1.0, decisions_per_game / 10.0)
        score *= decision_score

        # Forced rate: 100% = 0.5, 0% = 1.0
        agency_score = 1.0 - (forced_rate * 0.5)
        score *= agency_score

        # Choices: 1 = 0.5, 3+ = 1.0
        choice_score = min(1.0, 0.5 + (avg_choices - 1) / 4)
        score *= max(0.5, choice_score)

        # One-sidedness: 50% = 1.0, 100% = 0.5
        balance_score = 1.0 - (max_win_rate - 0.5)
        score *= max(0.5, balance_score)

        # Game length: too short or too long is bad
        if avg_turns < 5:
            length_score = avg_turns / 5
        elif avg_turns > 200:
            length_score = max(0.5, 1.0 - (avg_turns - 200) / 800)
        else:
            length_score = 1.0
        score *= length_score

        return max(0.0, min(1.0, score))


def is_meaningfully_playable(
    genome: GameGenome,
    results: Optional[SimulationResults] = None,
    strict: bool = False,
) -> bool:
    """Quick check if a game is meaningfully playable.

    Args:
        genome: Game genome to check
        results: Pre-computed simulation results (optional)
        strict: If True, major issues also count as unplayable

    Returns:
        True if game is playable
    """
    checker = PlayabilityChecker(strict=strict)
    report = checker.check(genome, results)
    return report.playable
