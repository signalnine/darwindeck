"""Semantic coherence checking for evolved genomes."""

from dataclasses import dataclass


@dataclass
class CoherenceResult:
    """Result of semantic coherence check."""
    coherent: bool
    violations: list[str]
