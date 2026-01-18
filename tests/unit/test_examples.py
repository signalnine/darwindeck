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


def test_partnership_spades_genome():
    """Partnership Spades should be a valid team game genome."""
    from darwindeck.genome.examples import create_partnership_spades_genome
    from darwindeck.genome.validator import GenomeValidator

    genome = create_partnership_spades_genome()

    # Basic structure
    assert genome.genome_id == "partnership-spades"
    assert genome.setup.cards_per_player == 13
    assert genome.player_count == 4

    # Team configuration
    assert genome.team_mode is True
    assert len(genome.teams) == 2
    assert genome.teams[0] == (0, 2)
    assert genome.teams[1] == (1, 3)

    # Trick-taking structure
    assert genome.turn_structure.is_trick_based is True
    assert genome.turn_structure.tricks_per_hand == 13

    # Validate with GenomeValidator (returns list of errors, empty = valid)
    errors = GenomeValidator.validate(genome)
    assert len(errors) == 0, f"Validation errors: {errors}"


def test_partnership_spades_in_seed_genomes():
    """Partnership Spades should be included in the seed genomes list."""
    from darwindeck.genome.examples import get_seed_genomes

    genomes = get_seed_genomes()
    genome_ids = [g.genome_id for g in genomes]
    assert "partnership-spades" in genome_ids


def test_team_genomes_have_valid_team_config():
    """All team mode genomes should have valid team configuration."""
    from darwindeck.genome.examples import get_seed_genomes
    from darwindeck.genome.validator import GenomeValidator

    genomes = get_seed_genomes()

    for genome in genomes:
        if genome.team_mode:
            # Team genomes should pass validation (returns list of errors, empty = valid)
            errors = GenomeValidator.validate(genome)
            assert len(errors) == 0, (
                f"Team genome {genome.genome_id} failed validation: {errors}"
            )

            # All players should be assigned to exactly one team
            all_players = set()
            for team in genome.teams:
                for player in team:
                    assert player not in all_players, (
                        f"Player {player} in multiple teams in {genome.genome_id}"
                    )
                    all_players.add(player)

            expected_players = set(range(genome.player_count))
            assert all_players == expected_players, (
                f"Not all players assigned to teams in {genome.genome_id}"
            )
