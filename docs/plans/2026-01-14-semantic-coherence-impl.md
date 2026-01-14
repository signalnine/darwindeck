# Semantic Coherence Filter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Filter semantically incoherent evolved games so only playable games reach humans.

**Architecture:** Two-layer filtering - fitness evaluation (main filter, fitness=0) and post-evolution save (safety net). Core checker class validates win conditions have supporting mechanics and resources are used.

**Tech Stack:** Python, pytest, dataclasses

---

## Task 1: Create CoherenceResult dataclass

**Files:**
- Create: `src/darwindeck/evolution/coherence.py`
- Test: `tests/unit/test_coherence.py`

**Step 1: Write the failing test**

```python
# tests/unit/test_coherence.py
"""Tests for semantic coherence checking."""

import pytest
from darwindeck.evolution.coherence import CoherenceResult


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
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_coherence.py -v`
Expected: FAIL with "ModuleNotFoundError: No module named 'darwindeck.evolution.coherence'"

**Step 3: Write minimal implementation**

```python
# src/darwindeck/evolution/coherence.py
"""Semantic coherence checking for evolved genomes."""

from dataclasses import dataclass


@dataclass
class CoherenceResult:
    """Result of semantic coherence check."""
    coherent: bool
    violations: list[str]
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_coherence.py::TestCoherenceResult -v`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/coherence.py tests/unit/test_coherence.py
git commit -m "feat(coherence): add CoherenceResult dataclass"
```

---

## Task 2: Create SemanticCoherenceChecker skeleton

**Files:**
- Modify: `src/darwindeck/evolution/coherence.py`
- Modify: `tests/unit/test_coherence.py`

**Step 1: Write the failing test**

```python
# Add to tests/unit/test_coherence.py
from darwindeck.evolution.coherence import SemanticCoherenceChecker
from darwindeck.genome.examples import create_war_genome


class TestSemanticCoherenceChecker:
    def test_checker_returns_result(self):
        """Checker returns CoherenceResult."""
        checker = SemanticCoherenceChecker()
        genome = create_war_genome()
        result = checker.check(genome)
        assert isinstance(result, CoherenceResult)
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_coherence.py::TestSemanticCoherenceChecker::test_checker_returns_result -v`
Expected: FAIL with "ImportError: cannot import name 'SemanticCoherenceChecker'"

**Step 3: Write minimal implementation**

```python
# Add to src/darwindeck/evolution/coherence.py
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from darwindeck.genome.schema import GameGenome


class SemanticCoherenceChecker:
    """Validates genome has internally consistent mechanics."""

    def check(self, genome: "GameGenome") -> CoherenceResult:
        """Check genome for semantic coherence."""
        return CoherenceResult(coherent=True, violations=[])
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_coherence.py::TestSemanticCoherenceChecker -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/coherence.py tests/unit/test_coherence.py
git commit -m "feat(coherence): add SemanticCoherenceChecker skeleton"
```

---

## Task 3: Implement capture win condition check

**Files:**
- Modify: `src/darwindeck/evolution/coherence.py`
- Modify: `tests/unit/test_coherence.py`

**Step 1: Write the failing tests**

```python
# Add to TestSemanticCoherenceChecker in tests/unit/test_coherence.py
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, PlayPhase, DiscardPhase,
    WinCondition, Location
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
        genome_id="test",
        setup=SetupRules(cards_per_player=5, starting_chips=starting_chips),
        turn_structure=TurnStructure(phases=tuple(phases), is_trick_based=is_trick_based),
        win_conditions=tuple(win_conditions),
        scoring_rules=tuple(scoring_rules or []),
        player_count=2,
    )


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
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_coherence.py::TestCaptureWinConditions -v`
Expected: FAIL (tests expect incoherent but checker returns coherent)

**Step 3: Write implementation**

```python
# Replace check method in SemanticCoherenceChecker
class SemanticCoherenceChecker:
    """Validates genome has internally consistent mechanics."""

    CAPTURE_WIN_CONDITIONS = frozenset({"capture_all", "most_captured"})

    def check(self, genome: "GameGenome") -> CoherenceResult:
        """Check genome for semantic coherence."""
        violations = []
        violations.extend(self._check_win_conditions(genome))
        return CoherenceResult(
            coherent=len(violations) == 0,
            violations=violations
        )

    def _check_win_conditions(self, genome: "GameGenome") -> list[str]:
        """Check win conditions have supporting mechanics."""
        from darwindeck.genome.schema import PlayPhase, Location

        violations = []

        has_tableau_phase = any(
            isinstance(p, PlayPhase) and p.target == Location.TABLEAU
            for p in genome.turn_structure.phases
        )

        for wc in genome.win_conditions:
            if wc.type in self.CAPTURE_WIN_CONDITIONS:
                if not has_tableau_phase:
                    violations.append(
                        f"Win condition '{wc.type}' requires PlayPhase targeting TABLEAU"
                    )

        return violations
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_coherence.py::TestCaptureWinConditions -v`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/coherence.py tests/unit/test_coherence.py
git commit -m "feat(coherence): add capture win condition validation"
```

---

## Task 4: Implement scoring win condition check

**Files:**
- Modify: `src/darwindeck/evolution/coherence.py`
- Modify: `tests/unit/test_coherence.py`

**Step 1: Write the failing tests**

```python
# Add to tests/unit/test_coherence.py
from darwindeck.genome.schema import ScoringRule


class TestScoringWinConditions:
    def test_high_score_with_scoring_rules_is_coherent(self):
        """high_score + scoring_rules = valid."""
        genome = _make_genome(
            phases=[DiscardPhase(target=Location.DISCARD, count=1)],
            win_conditions=[WinCondition(type="high_score", threshold=50)],
            scoring_rules=[ScoringRule(trigger="on_play", points=10)],
        )
        checker = SemanticCoherenceChecker()
        result = checker.check(genome)
        assert result.coherent is True

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
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_coherence.py::TestScoringWinConditions -v`
Expected: FAIL (tests expect incoherent for high_score without scoring)

**Step 3: Update implementation**

```python
# Update SemanticCoherenceChecker._check_win_conditions
SCORING_WIN_CONDITIONS = frozenset({"high_score", "low_score", "first_to_score"})

def _check_win_conditions(self, genome: "GameGenome") -> list[str]:
    """Check win conditions have supporting mechanics."""
    from darwindeck.genome.schema import PlayPhase, Location

    violations = []

    has_tableau_phase = any(
        isinstance(p, PlayPhase) and p.target == Location.TABLEAU
        for p in genome.turn_structure.phases
    )

    has_scoring = bool(genome.scoring_rules)
    is_trick_based = genome.turn_structure.is_trick_based

    for wc in genome.win_conditions:
        if wc.type in self.CAPTURE_WIN_CONDITIONS:
            if not has_tableau_phase:
                violations.append(
                    f"Win condition '{wc.type}' requires PlayPhase targeting TABLEAU"
                )

        elif wc.type in self.SCORING_WIN_CONDITIONS:
            if not has_scoring and not is_trick_based:
                violations.append(
                    f"Win condition '{wc.type}' requires scoring_rules or is_trick_based"
                )

    return violations
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_coherence.py::TestScoringWinConditions -v`
Expected: PASS (5 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/coherence.py tests/unit/test_coherence.py
git commit -m "feat(coherence): add scoring win condition validation"
```

---

## Task 5: Implement resource checks (chips without betting)

**Files:**
- Modify: `src/darwindeck/evolution/coherence.py`
- Modify: `tests/unit/test_coherence.py`

**Step 1: Write the failing tests**

```python
# Add to tests/unit/test_coherence.py
from darwindeck.genome.schema import BettingPhase


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
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_coherence.py::TestResourceCoherence -v`
Expected: FAIL (first test expects incoherent)

**Step 3: Add _check_resources method**

```python
# Add to SemanticCoherenceChecker
def check(self, genome: "GameGenome") -> CoherenceResult:
    """Check genome for semantic coherence."""
    violations = []
    violations.extend(self._check_win_conditions(genome))
    violations.extend(self._check_resources(genome))
    return CoherenceResult(
        coherent=len(violations) == 0,
        violations=violations
    )

def _check_resources(self, genome: "GameGenome") -> list[str]:
    """Check resources have supporting mechanics."""
    from darwindeck.genome.schema import BettingPhase

    violations = []

    has_betting_phase = any(
        isinstance(p, BettingPhase)
        for p in genome.turn_structure.phases
    )

    if genome.setup.starting_chips > 0 and not has_betting_phase:
        violations.append(
            f"starting_chips={genome.setup.starting_chips} but no BettingPhase"
        )

    return violations
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_coherence.py::TestResourceCoherence -v`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/coherence.py tests/unit/test_coherence.py
git commit -m "feat(coherence): add resource validation (chips/betting)"
```

---

## Task 6: Add GentleBlade regression test

**Files:**
- Modify: `tests/unit/test_coherence.py`

**Step 1: Write the test**

```python
# Add to tests/unit/test_coherence.py
class TestRealWorldRegressions:
    def test_gentle_blade_is_incoherent(self):
        """GentleBlade genome that prompted this feature should fail.

        Issues: capture_all without capture, high_score without scoring,
        starting_chips without betting.
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
```

**Step 2: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_coherence.py::TestRealWorldRegressions -v`
Expected: PASS (GentleBlade detected as incoherent)

**Step 3: Commit**

```bash
git add tests/unit/test_coherence.py
git commit -m "test(coherence): add GentleBlade regression test"
```

---

## Task 7: Integrate with FitnessResult

**Files:**
- Modify: `src/darwindeck/evolution/fitness_full.py`
- Test: `tests/unit/test_fitness.py`

**Step 1: Check existing FitnessResult structure**

Run: `uv run python -c "from darwindeck.evolution.fitness_full import FitnessResult; print(FitnessResult.__dataclass_fields__.keys())"`

**Step 2: Add coherence_violations field to FitnessResult**

```python
# In src/darwindeck/evolution/fitness_full.py, update FitnessResult
from dataclasses import dataclass, field
from typing import Optional

@dataclass
class FitnessResult:
    """Result of fitness evaluation."""
    fitness: float
    valid: bool
    metrics: dict
    error: Optional[str] = None
    coherence_violations: list[str] = field(default_factory=list)
```

**Step 3: Run existing fitness tests**

Run: `uv run pytest tests/unit/test_fitness.py -v`
Expected: PASS (existing tests should still work)

**Step 4: Commit**

```bash
git add src/darwindeck/evolution/fitness_full.py
git commit -m "feat(fitness): add coherence_violations to FitnessResult"
```

---

## Task 8: Add coherence check to fitness evaluator

**Files:**
- Modify: `src/darwindeck/evolution/fitness_full.py`
- Modify: `tests/unit/test_fitness.py`

**Step 1: Write the failing test**

```python
# Add to tests/unit/test_fitness.py
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, DiscardPhase, WinCondition, Location
)


class TestFitnessCoherenceIntegration:
    def test_incoherent_genome_gets_zero_fitness(self):
        """Incoherent genome should have fitness=0."""
        # Genome with high_score but no scoring rules
        genome = GameGenome(
            genome_id="incoherent_test",
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(
                phases=(DiscardPhase(target=Location.DISCARD, count=1),)
            ),
            win_conditions=(WinCondition(type="high_score", threshold=50),),
            scoring_rules=(),
            player_count=2,
        )

        from darwindeck.evolution.fitness_full import FullFitnessEvaluator
        evaluator = FullFitnessEvaluator()
        result = evaluator.evaluate(genome)

        assert result.fitness == 0.0
        assert result.valid is False
        assert len(result.coherence_violations) > 0
        assert "high_score" in result.coherence_violations[0]
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_fitness.py::TestFitnessCoherenceIntegration -v`
Expected: FAIL (evaluator doesn't check coherence yet)

**Step 3: Add coherence check to evaluator**

```python
# In src/darwindeck/evolution/fitness_full.py
from darwindeck.evolution.coherence import SemanticCoherenceChecker

class FullFitnessEvaluator:
    def __init__(self, ...):
        self.coherence_checker = SemanticCoherenceChecker()
        # ... existing init code ...

    def evaluate(self, genome: GameGenome) -> FitnessResult:
        """Evaluate genome fitness."""
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

        # Continue with existing evaluation...
        # (rest of existing code)
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_fitness.py::TestFitnessCoherenceIntegration -v`
Expected: PASS

**Step 5: Run all fitness tests**

Run: `uv run pytest tests/unit/test_fitness.py -v`
Expected: PASS (no regressions)

**Step 6: Commit**

```bash
git add src/darwindeck/evolution/fitness_full.py tests/unit/test_fitness.py
git commit -m "feat(fitness): add coherence check before simulation"
```

---

## Task 9: Add coherence filter to evolution runner save

**Files:**
- Modify: `src/darwindeck/evolution/runner.py`

**Step 1: Find save_top_genomes or equivalent method**

Run: `uv run grep -n "def save" src/darwindeck/evolution/runner.py` or explore the file

**Step 2: Add coherence filter before saving**

```python
# Add to runner.py (in the method that saves top genomes)
from darwindeck.evolution.coherence import SemanticCoherenceChecker

def _save_generation_results(self, population, generation, output_dir):
    """Save top genomes from generation."""
    checker = SemanticCoherenceChecker()

    saved_count = 0
    skipped_count = 0

    for genome in sorted(population, key=lambda g: g.fitness, reverse=True):
        if saved_count >= self.config.top_n:
            break

        result = checker.check(genome)
        if not result.coherent:
            skipped_count += 1
            continue

        self._save_genome(genome, output_dir, rank=saved_count + 1)
        saved_count += 1

    if skipped_count > 0:
        logger.warning(f"Skipped {skipped_count} incoherent genomes during save")
```

**Step 3: Run evolution tests**

Run: `uv run pytest tests/ -k "evolution" -v --ignore=tests/unit/test_tension.py`
Expected: PASS

**Step 4: Commit**

```bash
git add src/darwindeck/evolution/runner.py
git commit -m "feat(evolution): add coherence filter before saving genomes"
```

---

## Task 10: Run full test suite and verify

**Files:**
- None (verification only)

**Step 1: Run all coherence tests**

Run: `uv run pytest tests/unit/test_coherence.py -v`
Expected: All tests PASS

**Step 2: Run all unit tests**

Run: `uv run pytest tests/unit/ -v --ignore=tests/unit/test_tension.py`
Expected: All tests PASS

**Step 3: Run integration tests**

Run: `uv run pytest tests/integration/ -v`
Expected: All tests PASS (may have some skipped)

**Step 4: Final commit if any cleanup needed**

```bash
git status
# If clean, done. If changes, commit them.
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | CoherenceResult dataclass | coherence.py, test_coherence.py |
| 2 | SemanticCoherenceChecker skeleton | coherence.py, test_coherence.py |
| 3 | Capture win condition check | coherence.py, test_coherence.py |
| 4 | Scoring win condition check | coherence.py, test_coherence.py |
| 5 | Resource checks (chips/betting) | coherence.py, test_coherence.py |
| 6 | GentleBlade regression test | test_coherence.py |
| 7 | FitnessResult extension | fitness_full.py |
| 8 | Fitness evaluator integration | fitness_full.py, test_fitness.py |
| 9 | Evolution runner filter | runner.py |
| 10 | Full test verification | - |

**Total: 10 tasks**
