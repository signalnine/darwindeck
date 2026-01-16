"""Tests for genome bytecode compiler."""

import struct
import pytest
from darwindeck.genome.bytecode import BytecodeCompiler, BytecodeHeader, OpCode
from darwindeck.genome.examples import create_war_genome
from darwindeck.genome.schema import (
    BettingPhase,
    SetupRules,
    GameGenome,
    TurnStructure,
    DrawPhase,
    Location,
    WinCondition,
)


def test_header_serialization() -> None:
    """Test header round-trip."""
    header = BytecodeHeader(
        version=1,
        genome_id_hash=12345678901234567890,
        player_count=2,
        max_turns=100,
        setup_offset=39,  # Updated for new header size
        turn_structure_offset=47,
        win_conditions_offset=103,
        scoring_offset=123,
        tableau_mode=1,  # WAR
        sequence_direction=0,  # ASCENDING
    )

    serialized = header.to_bytes()
    assert len(serialized) == 39  # New header size

    # First byte should be bytecode version 2
    assert serialized[0] == 2

    deserialized = BytecodeHeader.from_bytes(serialized)
    assert deserialized.version == 1
    assert deserialized.player_count == 2
    assert deserialized.max_turns == 100
    assert deserialized.tableau_mode == 1
    assert deserialized.sequence_direction == 0


def test_compile_war_genome() -> None:
    """Test compiling War genome to bytecode."""
    war = create_war_genome()
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(war)

    # Should be compact (< 500 bytes for War)
    assert len(bytecode) < 500

    # Header should parse
    header = BytecodeHeader.from_bytes(bytecode)
    assert header.version == 1
    assert header.player_count == 2
    assert header.max_turns == 1000


def test_opcode_values() -> None:
    """Test OpCode enum values are in expected ranges."""
    # Conditions: 0-19
    assert 0 <= OpCode.CHECK_HAND_SIZE <= 19
    assert 0 <= OpCode.CHECK_CARD_RANK <= 19

    # Actions: 20-39
    assert 20 <= OpCode.DRAW_CARDS <= 39
    assert 20 <= OpCode.PLAY_CARD <= 39

    # Control flow: 40-49
    assert 40 <= OpCode.AND <= 49
    assert 40 <= OpCode.OR <= 49

    # Operators: 50-55
    assert 50 <= OpCode.OP_EQ <= 55
    assert 50 <= OpCode.OP_NE <= 55


def test_effect_opcodes_exist():
    """Effect-related opcodes are defined."""
    from darwindeck.genome.bytecode import OpCode

    assert OpCode.EFFECT_HEADER.value == 60
    assert OpCode.EFFECT_ENTRY.value == 61


def test_compile_effects():
    """compile_effects produces correct bytecode."""
    from darwindeck.genome.bytecode import compile_effects
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector

    effects = [
        SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        SpecialEffect(Rank.JACK, EffectType.SKIP_NEXT, TargetSelector.NEXT_PLAYER, 1),
    ]

    bytecode = compile_effects(effects)

    # Header: opcode (60), count (2)
    assert bytecode[0] == 60  # EFFECT_HEADER
    assert bytecode[1] == 2   # count

    # Effect 1: TWO (0), DRAW_CARDS (2), NEXT_PLAYER (0), value (2)
    assert bytecode[2] == 0   # Rank.TWO -> 0
    assert bytecode[3] == 2   # EffectType.DRAW_CARDS -> 2
    assert bytecode[4] == 0   # TARGET_NEXT_PLAYER -> 0
    assert bytecode[5] == 2   # value

    # Effect 2: JACK (9), SKIP_NEXT (0), NEXT_PLAYER (0), value (1)
    assert bytecode[6] == 9   # Rank.JACK -> 9
    assert bytecode[7] == 0   # EffectType.SKIP_NEXT -> 0
    assert bytecode[8] == 0   # TARGET_NEXT_PLAYER -> 0
    assert bytecode[9] == 1   # value


def test_compile_betting_phase() -> None:
    """Test BettingPhase compilation produces correct bytes."""
    compiler = BytecodeCompiler()
    phase = BettingPhase(min_bet=25, max_raises=4)

    bytecode = compiler._compile_betting_phase(phase)

    # Should be 9 bytes: phase_type:1 + min_bet:4 + max_raises:4
    assert len(bytecode) == 9

    # Parse the bytecode
    phase_type = bytecode[0]
    min_bet = struct.unpack("!I", bytecode[1:5])[0]
    max_raises = struct.unpack("!I", bytecode[5:9])[0]

    assert phase_type == 5  # PhaseTypeBetting
    assert min_bet == 25
    assert max_raises == 4


def test_compile_setup_includes_starting_chips() -> None:
    """Test that starting_chips is included in setup bytes."""
    compiler = BytecodeCompiler()
    setup = SetupRules(
        cards_per_player=5,
        initial_discard_count=1,
        starting_chips=1000,
    )

    bytecode = compiler._compile_setup(setup)

    # Should be 12 bytes: cards_per_player:4 + initial_discard_count:4 + starting_chips:4
    assert len(bytecode) == 12

    # Parse the bytecode
    cards_per_player, initial_discard_count, starting_chips = struct.unpack("!iii", bytecode)

    assert cards_per_player == 5
    assert initial_discard_count == 1
    assert starting_chips == 1000


def test_compile_genome_with_betting_phase() -> None:
    """Test genome with BettingPhase compiles correctly."""
    from darwindeck.genome.conditions import Condition, ConditionType, Operator

    # Create a simple genome with a betting phase
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test-betting-genome",
        generation=0,
        player_count=2,
        max_turns=100,
        setup=SetupRules(
            cards_per_player=5,
            initial_discard_count=0,
            starting_chips=500,
        ),
        turn_structure=TurnStructure(
            phases=[
                DrawPhase(source=Location.DECK, count=1),
                BettingPhase(min_bet=10, max_raises=3),
            ]
        ),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    # Should compile without error
    assert len(bytecode) > 36  # Header + data

    # Header should parse
    header = BytecodeHeader.from_bytes(bytecode)
    assert header.version == 1
    assert header.player_count == 2
    assert header.max_turns == 100

    # Setup should contain starting_chips
    setup_start = header.setup_offset
    setup_bytes = bytecode[setup_start:setup_start + 12]
    _, _, starting_chips = struct.unpack("!iii", setup_bytes)
    assert starting_chips == 500


def test_bytecode_version_header():
    """Bytecode starts with version byte."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, TableauMode
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.WAR),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    # First byte should be version 2
    assert bytecode[0] == 2


def test_bytecode_tableau_mode_encoding():
    """Bytecode encodes tableau_mode at correct offset."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition,
        TableauMode, SequenceDirection
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(
            cards_per_player=7,
            tableau_mode=TableauMode.SEQUENCE,
            sequence_direction=SequenceDirection.DESCENDING
        ),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    # Offset 37: tableau_mode (3=SEQUENCE)
    # Offset 38: sequence_direction (1=DESCENDING)
    assert bytecode[37] == 3  # SEQUENCE
    assert bytecode[38] == 1  # DESCENDING


def test_compile_card_scoring_hearts():
    """Test encoding Hearts-style card scoring rules."""
    from darwindeck.genome.schema import CardScoringRule, CardCondition, ScoringTrigger, Suit, Rank
    from darwindeck.genome.bytecode import compile_card_scoring

    rules = (
        CardScoringRule(
            condition=CardCondition(suit=Suit.HEARTS),
            points=1,
            trigger=ScoringTrigger.TRICK_WIN,
        ),
        CardScoringRule(
            condition=CardCondition(suit=Suit.SPADES, rank=Rank.QUEEN),
            points=13,
            trigger=ScoringTrigger.TRICK_WIN,
        ),
    )

    bytecode = compile_card_scoring(rules)

    # Format: count:2 + [suit:1, rank:1, points:2, trigger:1] * count
    assert len(bytecode) == 2 + (5 * 2)  # 12 bytes
    assert bytecode[0:2] == b'\x00\x02'  # 2 rules (big-endian)
