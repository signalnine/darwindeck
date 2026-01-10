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
