"""Tests for seed genome examples."""

import pytest
from darwindeck.genome.schema import TableauMode, SequenceDirection


def test_war_genome_has_war_mode():
    """War seed genome has explicit WAR tableau mode."""
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()
    assert genome.setup.tableau_mode == TableauMode.WAR


def test_scopa_genome_has_match_rank_mode():
    """Scopa seed genome has MATCH_RANK tableau mode."""
    from darwindeck.genome.examples import create_scopa_genome

    genome = create_scopa_genome()
    assert genome.setup.tableau_mode == TableauMode.MATCH_RANK


def test_crazy_eights_has_none_mode():
    """Crazy Eights genome has NONE tableau mode (default)."""
    from darwindeck.genome.examples import create_crazy_eights_genome

    genome = create_crazy_eights_genome()
    assert genome.setup.tableau_mode == TableauMode.NONE


def test_fan_tan_genome_has_sequence_mode():
    """Fan Tan seed genome has SEQUENCE tableau mode with BOTH direction."""
    from darwindeck.genome.examples import create_fan_tan_genome

    genome = create_fan_tan_genome()
    assert genome.setup.tableau_mode == TableauMode.SEQUENCE
    assert genome.setup.sequence_direction == SequenceDirection.BOTH


def test_betting_war_genome_has_war_mode():
    """Betting War seed genome has WAR tableau mode."""
    from darwindeck.genome.examples import create_betting_war_genome

    genome = create_betting_war_genome()
    assert genome.setup.tableau_mode == TableauMode.WAR


def test_all_genomes_have_valid_tableau_mode():
    """All seed genomes have a valid TableauMode value."""
    from darwindeck.genome.examples import get_seed_genomes

    genomes = get_seed_genomes()
    for genome in genomes:
        assert isinstance(genome.setup.tableau_mode, TableauMode), (
            f"Genome {genome.genome_id} has invalid tableau_mode type"
        )


def test_sequence_direction_only_with_sequence_mode():
    """Only SEQUENCE mode genomes should rely on sequence_direction."""
    from darwindeck.genome.examples import get_seed_genomes

    genomes = get_seed_genomes()
    for genome in genomes:
        if genome.setup.tableau_mode == TableauMode.SEQUENCE:
            # Should have explicit direction for SEQUENCE mode
            assert genome.setup.sequence_direction in (
                SequenceDirection.ASCENDING,
                SequenceDirection.DESCENDING,
                SequenceDirection.BOTH,
            ), f"Genome {genome.genome_id} has SEQUENCE mode but invalid direction"
