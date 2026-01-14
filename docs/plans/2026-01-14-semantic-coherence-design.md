# Semantic Coherence Filter Design

**Date:** 2026-01-14
**Status:** Draft (Updated after multi-agent review)
**Goal:** Filter semantically incoherent evolved games to ensure only playable games reach humans

## Problem Statement

Evolved genomes can have high fitness metrics (decision density, skill gap) while being semantically incoherent - rules that don't make sense together. Example: GentleBlade has `capture_all` win condition but no capture mechanism, `high_score` win condition but empty `scoring_rules`, and `starting_chips: 5252` with no betting phase.

These games are "playable" by AI (it just picks moves) but meaningless for humans.

## Multi-Agent Review Findings (Addressed)

**STRONG issues fixed:**
1. ~~Mutation rejection breaks evolutionary dynamics~~ → Removed Layer 1 entirely
2. ~~Shallow coherence validation~~ → Added semantic reachability checks
3. ~~No tracking metrics~~ → Added coherence stats tracking

**Architecture change:** Simplified from 3 layers to 2 layers based on reviewer consensus.

## Solution: Two-Layer Filtering

### Layer 1: Fitness Evaluation (Main Filter)

Incoherent genomes receive `fitness=0` before any simulation runs.

**Benefits:**
- Fast rejection (no expensive simulation)
- Catches all incoherent genomes regardless of source (mutation, crossover, seeds)
- Clear error message in fitness result
- Metrics tracked for debugging

### Layer 2: Post-Evolution Filter (Safety Net)

Final gate before saving genomes to disk. Incoherent genomes are skipped.

**Benefits:**
- Catches any edge cases that slip through
- Ensures only coherent games reach users
- Aggregated logging (not per-genome spam)

## Coherence Rules

### Win Condition Requirements

| Win Condition | Requires |
|--------------|----------|
| `capture_all` | PlayPhase with `target: TABLEAU` |
| `most_captured` | PlayPhase with `target: TABLEAU` |
| `high_score` | Non-empty `scoring_rules` OR `is_trick_based: true` |
| `low_score` | Non-empty `scoring_rules` OR `is_trick_based: true` |
| `first_to_score` | Non-empty `scoring_rules` OR `is_trick_based: true` |
| `empty_hand` | Always valid (any play/discard phase works) |
| `all_hands_empty` | Always valid |

### Resource Requirements

| Resource | Requires |
|----------|----------|
| `starting_chips > 0` | At least one `BettingPhase` |
| `BettingPhase` present | `starting_chips > 0` (existing check in GenomeValidator) |

### Scoring Reachability (New)

| Scoring Trigger | Requires |
|----------------|----------|
| `on_capture` | Capture mechanism (tableau play + capture win condition) |
| `on_trick_win` | `is_trick_based: true` |

## Architecture

### New File: `src/darwindeck/evolution/coherence.py`

```python
from dataclasses import dataclass
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from darwindeck.genome.schema import GameGenome


@dataclass
class CoherenceResult:
    """Result of semantic coherence check."""
    coherent: bool
    violations: list[str]  # Human-readable issues


class SemanticCoherenceChecker:
    """Validates genome has internally consistent mechanics."""

    # Win conditions that require capture mechanics
    CAPTURE_WIN_CONDITIONS = frozenset({"capture_all", "most_captured"})

    # Win conditions that require scoring
    SCORING_WIN_CONDITIONS = frozenset({"high_score", "low_score", "first_to_score"})

    # Scoring triggers that require specific mechanics
    CAPTURE_TRIGGERS = frozenset({"on_capture"})
    TRICK_TRIGGERS = frozenset({"on_trick_win"})

    def check(self, genome: "GameGenome") -> CoherenceResult:
        """Check genome for semantic coherence."""
        violations = []

        # Win condition requirements
        violations.extend(self._check_win_conditions(genome))

        # Resource requirements
        violations.extend(self._check_resources(genome))

        # Scoring reachability
        violations.extend(self._check_scoring_reachability(genome))

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

    def _check_scoring_reachability(self, genome: "GameGenome") -> list[str]:
        """Verify scoring rules can actually trigger."""
        from darwindeck.genome.schema import PlayPhase, Location

        violations = []

        # Determine what mechanics exist
        has_tableau_phase = any(
            isinstance(p, PlayPhase) and p.target == Location.TABLEAU
            for p in genome.turn_structure.phases
        )
        has_capture_win = any(
            wc.type in self.CAPTURE_WIN_CONDITIONS
            for wc in genome.win_conditions
        )
        has_capture = has_tableau_phase and has_capture_win
        has_trick = genome.turn_structure.is_trick_based

        for rule in genome.scoring_rules:
            trigger = getattr(rule, 'trigger', None)
            if trigger in self.CAPTURE_TRIGGERS and not has_capture:
                violations.append(
                    f"Scoring trigger '{trigger}' requires capture mechanism"
                )
            if trigger in self.TRICK_TRIGGERS and not has_trick:
                violations.append(
                    f"Scoring trigger '{trigger}' requires is_trick_based"
                )

        return violations
```

### Integration: Fitness Evaluator

```python
# In src/darwindeck/evolution/fitness_full.py

from darwindeck.evolution.coherence import SemanticCoherenceChecker

class FullFitnessEvaluator:
    def __init__(self, ...):
        self.coherence_checker = SemanticCoherenceChecker()
        ...

    def evaluate(self, genome: GameGenome) -> FitnessResult:
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

        # Continue with normal fitness evaluation...
        return self._evaluate_coherent_genome(genome)
```

### Integration: Post-Evolution Filter

```python
# In src/darwindeck/evolution/runner.py

def save_top_genomes(self, population: list[GameGenome], output_dir: Path):
    checker = SemanticCoherenceChecker()

    saved_count = 0
    skipped_count = 0

    for genome in self._rank_by_fitness(population):
        result = checker.check(genome)

        if not result.coherent:
            skipped_count += 1
            continue

        self._save_genome(genome, output_dir, rank=saved_count + 1)
        saved_count += 1

        if saved_count >= self.top_n:
            break

    if skipped_count > 0:
        logger.warning(f"Skipped {skipped_count} incoherent genomes during save")
```

### Coherence Tracking Metrics

```python
# In src/darwindeck/evolution/runner.py

from collections import defaultdict

class EvolutionRunner:
    def __init__(self, ...):
        self.coherence_stats = {
            "total_evaluated": 0,
            "incoherent_count": 0,
            "violation_counts": defaultdict(int),
        }

    def _track_coherence(self, result: FitnessResult):
        """Track coherence statistics for debugging."""
        self.coherence_stats["total_evaluated"] += 1
        if result.coherence_violations:
            self.coherence_stats["incoherent_count"] += 1
            for violation in result.coherence_violations:
                # Extract violation type (first part before specific values)
                violation_type = violation.split("=")[0].strip()
                self.coherence_stats["violation_counts"][violation_type] += 1

    def _log_coherence_summary(self):
        """Log coherence summary at end of generation."""
        total = self.coherence_stats["total_evaluated"]
        incoherent = self.coherence_stats["incoherent_count"]

        if total == 0:
            return

        rate = incoherent / total
        logger.info(f"Coherence: {incoherent}/{total} ({rate:.1%}) incoherent this generation")

        if rate > 0.5:
            logger.warning("High incoherence rate - check mutation operators")

        for violation, count in sorted(
            self.coherence_stats["violation_counts"].items(),
            key=lambda x: -x[1]
        ):
            logger.debug(f"  {violation}: {count}")

    def _reset_coherence_stats(self):
        """Reset stats at start of generation."""
        self.coherence_stats["total_evaluated"] = 0
        self.coherence_stats["incoherent_count"] = 0
        self.coherence_stats["violation_counts"].clear()
```

### FitnessResult Extension

```python
# In src/darwindeck/evolution/fitness_full.py

@dataclass
class FitnessResult:
    fitness: float
    valid: bool
    metrics: dict
    error: Optional[str] = None
    coherence_violations: list[str] = field(default_factory=list)  # NEW
```

## Testing Strategy

### Unit Tests: `tests/unit/test_coherence.py`

```python
class TestSemanticCoherenceChecker:

    # Win condition coherence
    def test_capture_all_with_tableau_phase_is_coherent(self):
        """capture_all + PlayPhase(target=TABLEAU) = valid"""

    def test_capture_all_without_tableau_phase_is_incoherent(self):
        """capture_all + only discard phases = invalid"""

    def test_high_score_with_scoring_rules_is_coherent(self):
        """high_score + scoring_rules = valid"""

    def test_high_score_with_trick_based_is_coherent(self):
        """high_score + is_trick_based = valid (tricks score)"""

    def test_high_score_without_scoring_is_incoherent(self):
        """high_score + empty scoring_rules + not trick_based = invalid"""

    def test_empty_hand_always_coherent(self):
        """empty_hand works with any phase configuration"""

    # Resource coherence
    def test_chips_without_betting_is_incoherent(self):
        """starting_chips > 0 but no BettingPhase = invalid"""

    def test_chips_with_betting_is_coherent(self):
        """starting_chips > 0 + BettingPhase = valid"""

    # Scoring reachability
    def test_capture_scoring_without_capture_is_incoherent(self):
        """on_capture trigger but no capture mechanism = invalid"""

    def test_trick_scoring_without_tricks_is_incoherent(self):
        """on_trick_win trigger but not trick_based = invalid"""

    # Real-world regression
    def test_gentle_blade_is_incoherent(self):
        """The genome that prompted this feature should fail"""


class TestCoherenceTracking:

    def test_tracks_violation_counts(self):
        """Verify violation counting works"""

    def test_logs_high_incoherence_warning(self):
        """Warn when >50% of genomes fail coherence"""
```

### Integration Tests

- Verify fitness=0 for incoherent genomes in real evaluation
- Verify post-evolution filter skips bad genomes
- Verify coherence stats are logged correctly

## Files Modified

| File | Change |
|------|--------|
| `src/darwindeck/evolution/coherence.py` | New file |
| `src/darwindeck/evolution/fitness_full.py` | Add coherence check, extend FitnessResult |
| `src/darwindeck/evolution/runner.py` | Add filter and tracking metrics |
| `tests/unit/test_coherence.py` | New test file |

## Edge Cases

- **All genomes incoherent:** Log error, save nothing, warn about mutation operators
- **Crossover creating incoherent child:** Caught by fitness layer
- **Seed genomes that are incoherent:** Caught by fitness layer, logged as warning
- **High incoherence rate (>50%):** Log warning suggesting mutation operator review
- **Unknown win condition types:** Pass validation (future-proof, but log debug message)
