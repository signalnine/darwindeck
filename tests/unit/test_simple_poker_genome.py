"""Tests for Simple Poker game genome with betting."""

import pytest
from darwindeck.genome.examples import create_simple_poker_genome, get_seed_genomes
from darwindeck.genome.schema import GameGenome, BettingPhase
from darwindeck.genome.serialization import genome_to_json, genome_from_json


def test_simple_poker_genome_creates_valid_genome() -> None:
    """Test Simple Poker genome creates a valid GameGenome."""
    genome = create_simple_poker_genome()

    assert isinstance(genome, GameGenome)
    assert genome.schema_version == "1.0"
    assert genome.genome_id == "simple-poker"
    assert genome.generation == 0


def test_simple_poker_genome_has_betting_phase() -> None:
    """Test Simple Poker genome has BettingPhase in turn structure."""
    genome = create_simple_poker_genome()

    assert len(genome.turn_structure.phases) == 1
    phase = genome.turn_structure.phases[0]
    assert isinstance(phase, BettingPhase)
    assert phase.min_bet == 10
    assert phase.max_raises == 3


def test_simple_poker_genome_has_starting_chips() -> None:
    """Test Simple Poker genome has starting_chips enabled."""
    genome = create_simple_poker_genome()

    assert genome.setup.starting_chips == 1000
    assert genome.setup.starting_chips > 0  # Betting is enabled


def test_simple_poker_genome_setup() -> None:
    """Test Simple Poker genome setup rules are correct."""
    genome = create_simple_poker_genome()

    assert genome.setup.cards_per_player == 5
    assert genome.setup.initial_deck == "standard_52"
    assert genome.setup.initial_discard_count == 0


def test_simple_poker_genome_win_condition() -> None:
    """Test Simple Poker genome has best_hand win condition."""
    genome = create_simple_poker_genome()

    assert len(genome.win_conditions) == 1
    assert genome.win_conditions[0].type == "best_hand"


def test_simple_poker_genome_player_count() -> None:
    """Test Simple Poker genome is configured for 2 players."""
    genome = create_simple_poker_genome()

    assert genome.player_count == 2


def test_simple_poker_genome_serialization_roundtrip() -> None:
    """Test Simple Poker genome round-trips through JSON serialization."""
    genome = create_simple_poker_genome()

    # Round-trip through JSON string
    json_str = genome_to_json(genome)
    restored = genome_from_json(json_str)

    # Verify key properties are preserved
    assert restored.genome_id == "simple-poker"
    assert restored.setup.starting_chips == 1000
    assert restored.setup.cards_per_player == 5

    # Verify BettingPhase is preserved
    assert len(restored.turn_structure.phases) == 1
    phase = restored.turn_structure.phases[0]
    assert isinstance(phase, BettingPhase)
    assert phase.min_bet == 10
    assert phase.max_raises == 3

    # Verify win condition preserved
    assert restored.win_conditions[0].type == "best_hand"


def test_simple_poker_genome_in_seed_genomes() -> None:
    """Test Simple Poker genome is included in get_seed_genomes."""
    seed_genomes = get_seed_genomes()

    poker_genomes = [g for g in seed_genomes if g.genome_id == "simple-poker"]
    assert len(poker_genomes) == 1

    poker = poker_genomes[0]
    assert isinstance(poker.turn_structure.phases[0], BettingPhase)
    assert poker.setup.starting_chips == 1000


def test_simple_poker_genome_no_special_effects() -> None:
    """Test Simple Poker genome has no special effects."""
    genome = create_simple_poker_genome()

    assert len(genome.special_effects) == 0


def test_simple_poker_genome_max_turns() -> None:
    """Test Simple Poker genome has appropriate max_turns for quick hands."""
    genome = create_simple_poker_genome()

    # Poker hands should be quick
    assert genome.max_turns == 10
