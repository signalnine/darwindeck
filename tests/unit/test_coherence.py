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
