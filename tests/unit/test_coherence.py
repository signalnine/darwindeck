"""Tests for semantic coherence checking."""

import pytest
from darwindeck.evolution.coherence import SemanticCoherenceChecker, CoherenceResult
from darwindeck.genome.examples import create_war_genome


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
