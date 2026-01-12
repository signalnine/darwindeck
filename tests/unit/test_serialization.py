"""Tests for genome serialization."""

import pytest

from darwindeck.genome.schema import (
    BettingPhase, SetupRules, GameGenome, TurnStructure, WinCondition,
    DrawPhase, PlayPhase, Location
)
from darwindeck.genome.serialization import (
    _phase_to_dict, _phase_from_dict, _setup_to_dict, _setup_from_dict,
    genome_to_dict, genome_from_dict, genome_to_json, genome_from_json
)
from darwindeck.genome.conditions import Condition, ConditionType


def test_betting_phase_serialization():
    """BettingPhase round-trips through dict serialization."""
    phase = BettingPhase(min_bet=20, max_raises=2)
    d = _phase_to_dict(phase)
    restored = _phase_from_dict(d)
    assert restored == phase


def test_betting_phase_serialization_defaults():
    """BettingPhase with default values round-trips correctly."""
    phase = BettingPhase()  # Uses defaults: min_bet=10, max_raises=3
    d = _phase_to_dict(phase)
    restored = _phase_from_dict(d)
    assert restored == phase
    assert restored.min_bet == 10
    assert restored.max_raises == 3


def test_betting_phase_dict_structure():
    """BettingPhase produces expected dict structure."""
    phase = BettingPhase(min_bet=50, max_raises=5)
    d = _phase_to_dict(phase)
    assert d["type"] == "BettingPhase"
    assert d["min_bet"] == 50
    assert d["max_raises"] == 5


def test_setup_rules_with_starting_chips():
    """SetupRules with starting_chips round-trips correctly."""
    setup = SetupRules(cards_per_player=5, starting_chips=1000)
    d = _setup_to_dict(setup)
    restored = _setup_from_dict(d)
    assert restored.starting_chips == 1000
    assert restored.cards_per_player == 5


def test_setup_rules_starting_chips_default():
    """SetupRules without starting_chips defaults to 0."""
    # Simulate loading old data without starting_chips field
    data = {
        "cards_per_player": 7,
        "initial_deck": "standard_52",
        "initial_discard_count": 0,
    }
    restored = _setup_from_dict(data)
    assert restored.starting_chips == 0


def test_betting_phase_from_dict_with_missing_fields():
    """BettingPhase deserialization handles missing fields with defaults."""
    # Simulate partial data
    data = {"type": "BettingPhase"}
    restored = _phase_from_dict(data)
    assert restored.min_bet == 10  # default
    assert restored.max_raises == 3  # default


def test_full_genome_with_betting_phase():
    """Full genome with BettingPhase round-trips through JSON."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test-betting-genome",
        generation=1,
        setup=SetupRules(cards_per_player=2, starting_chips=500),
        turn_structure=TurnStructure(
            phases=[
                DrawPhase(source=Location.DECK, count=1),
                BettingPhase(min_bet=10, max_raises=2),
                PlayPhase(
                    target=Location.DISCARD,
                    valid_play_condition=Condition(type=ConditionType.CARD_MATCHES_RANK),
                ),
            ]
        ),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        max_turns=100,
    )

    # Round-trip through dict
    d = genome_to_dict(genome)
    restored = genome_from_dict(d)

    assert restored.setup.starting_chips == 500
    assert len(restored.turn_structure.phases) == 3
    betting_phase = restored.turn_structure.phases[1]
    assert isinstance(betting_phase, BettingPhase)
    assert betting_phase.min_bet == 10
    assert betting_phase.max_raises == 2


def test_full_genome_with_betting_phase_json():
    """Full genome with BettingPhase round-trips through JSON string."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test-betting-genome-json",
        generation=1,
        setup=SetupRules(cards_per_player=2, starting_chips=1000),
        turn_structure=TurnStructure(
            phases=[
                BettingPhase(min_bet=25, max_raises=4),
            ]
        ),
        special_effects=[],
        win_conditions=[WinCondition(type="high_score", threshold=100)],
        scoring_rules=[],
        max_turns=50,
    )

    # Round-trip through JSON string
    json_str = genome_to_json(genome)
    restored = genome_from_json(json_str)

    assert restored.setup.starting_chips == 1000
    betting_phase = restored.turn_structure.phases[0]
    assert isinstance(betting_phase, BettingPhase)
    assert betting_phase.min_bet == 25
    assert betting_phase.max_raises == 4
