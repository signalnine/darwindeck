"""Tests for genome bytecode compiler."""

import pytest
from darwindeck.genome.bytecode import BytecodeCompiler, BytecodeHeader, OpCode
from darwindeck.genome.examples import create_war_genome


def test_header_serialization() -> None:
    """Test header round-trip."""
    header = BytecodeHeader(
        version=1,
        genome_id_hash=12345678901234567890,
        player_count=2,
        max_turns=100,
        setup_offset=36,
        turn_structure_offset=44,
        win_conditions_offset=100,
        scoring_offset=120
    )

    serialized = header.to_bytes()
    assert len(serialized) == 36

    deserialized = BytecodeHeader.from_bytes(serialized)
    assert deserialized.version == 1
    assert deserialized.player_count == 2
    assert deserialized.max_turns == 100


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
