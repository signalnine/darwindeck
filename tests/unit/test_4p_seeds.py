"""Tests for 4-player non-trick-taking seed genomes."""

from darwindeck.genome.examples import (
    create_crazy_eights_4p_genome,
    create_cheat_4p_genome,
    create_uno_4p_genome,
    get_seed_genomes,
)


# Only include seeds that actually work in the Go simulator
FACTORIES = [
    create_crazy_eights_4p_genome,
    create_cheat_4p_genome,
    create_uno_4p_genome,
]


def test_4p_seed_genomes_valid():
    for factory in FACTORIES:
        g = factory()
        assert g.player_count == 4
        assert g.setup.cards_per_player * 4 <= 52
        assert len(g.turn_structure.phases) > 0
        assert len(g.win_conditions) > 0


def test_4p_seed_genomes_unique_ids():
    ids = [factory().genome_id for factory in FACTORIES]
    assert len(ids) == len(set(ids)), f"Duplicate genome IDs: {ids}"


def test_4p_seeds_in_seed_list():
    """All new 4p genomes should appear in get_seed_genomes()."""
    seeds = get_seed_genomes()
    seed_ids = {g.genome_id for g in seeds}
    for factory in FACTORIES:
        g = factory()
        assert g.genome_id in seed_ids, f"{g.genome_id} missing from get_seed_genomes()"


def test_uno_4p_has_special_effects():
    g = create_uno_4p_genome()
    assert len(g.special_effects) == 4


def test_cheat_4p_uses_claim_phase():
    from darwindeck.genome.schema import ClaimPhase

    g = create_cheat_4p_genome()
    assert any(isinstance(p, ClaimPhase) for p in g.turn_structure.phases)
