"""Tests for War game genome definition."""

import pytest
from darwindeck.genome.examples import create_war_genome
from darwindeck.genome.schema import GameGenome


def test_war_genome_structure() -> None:
    """Test War genome has correct structure."""
    genome = create_war_genome()

    assert isinstance(genome, GameGenome)
    assert genome.schema_version == "1.0"
    assert genome.genome_id == "war-baseline"


def test_war_genome_setup() -> None:
    """Test War genome setup rules."""
    genome = create_war_genome()

    assert genome.setup.cards_per_player == 26
    assert genome.setup.initial_deck == "standard_52"


def test_war_genome_turn_structure() -> None:
    """Test War genome has simple turn structure."""
    genome = create_war_genome()

    # War has only a play phase (no draw, no choice)
    assert len(genome.turn_structure.phases) == 1


def test_war_genome_no_special_effects() -> None:
    """Test War has no special card effects."""
    genome = create_war_genome()

    assert len(genome.special_effects) == 0
