"""Golden test suite for Python↔Go bytecode equivalence.

This test ensures that Python-compiled bytecode is correctly parsed and
executed by the Go engine, validating the entire pipeline.
"""

import pytest
from darwindeck.genome.bytecode import BytecodeCompiler, BytecodeHeader, BYTECODE_VERSION
from darwindeck.genome.examples import create_war_genome, create_hearts_genome


def compile_genome(genome):
    """Helper to compile genome to bytecode."""
    compiler = BytecodeCompiler()
    return compiler.compile_genome(genome)


class TestBytecodeEquivalence:
    """Verify Python bytecode compiles and can be parsed by Go."""

    def test_war_genome_compilation(self) -> None:
        """War genome should compile to valid bytecode."""
        genome = create_war_genome()
        bytecode = compile_genome(genome)

        # Should be compact (<500 bytes for simple game)
        assert len(bytecode) < 500, f"Bytecode too large: {len(bytecode)} bytes"

        # Check bytecode format version at byte 0
        bytecode_format_version = bytecode[0]
        assert bytecode_format_version == BYTECODE_VERSION, \
            f"Expected bytecode version {BYTECODE_VERSION}, got {bytecode_format_version}"

    def test_header_structure(self) -> None:
        """Bytecode header should match expected structure."""
        genome = create_war_genome()
        bytecode = compile_genome(genome)

        # Header is exactly HEADER_SIZE bytes
        assert len(bytecode) >= BytecodeHeader.HEADER_SIZE

        # Parse header using BytecodeHeader class
        header = BytecodeHeader.from_bytes(bytecode)

        # All offsets should be within bytecode length
        assert 0 <= header.setup_offset < len(bytecode)
        assert 0 <= header.turn_structure_offset < len(bytecode)
        assert 0 <= header.win_conditions_offset < len(bytecode)
        # scoring_offset may be -1 if not used

    def test_turn_structure_encoding(self) -> None:
        """Turn structure phases should be encoded correctly."""
        genome = create_war_genome()
        bytecode = compile_genome(genome)

        import struct

        # Get turn structure offset from header
        header = BytecodeHeader.from_bytes(bytecode)
        turn_offset = header.turn_structure_offset

        # First 4 bytes at offset = phase count
        phase_count = struct.unpack("!I", bytecode[turn_offset : turn_offset + 4])[0]

        assert phase_count > 0, "War should have at least one phase"
        assert phase_count <= 5, "Unexpected number of phases"

    def test_win_conditions_encoding(self) -> None:
        """Win conditions should be encoded correctly."""
        genome = create_war_genome()
        bytecode = compile_genome(genome)

        import struct

        # Get win conditions offset from header
        header = BytecodeHeader.from_bytes(bytecode)
        win_offset = header.win_conditions_offset

        # First 4 bytes at offset = win condition count
        win_count = struct.unpack("!I", bytecode[win_offset : win_offset + 4])[0]

        assert win_count > 0, "War should have at least one win condition"
        assert win_count <= 3, "Unexpected number of win conditions"

        # First win condition (5 bytes: type + threshold)
        win_type = bytecode[win_offset + 4]
        threshold = struct.unpack("!i", bytecode[win_offset + 5 : win_offset + 9])[0]

        # War uses empty_hand (type 0) or capture_all (type 3)
        assert win_type in [0, 3], f"Unexpected win type: {win_type}"

    def test_bytecode_determinism(self) -> None:
        """Compiling same genome should produce identical bytecode."""
        genome = create_war_genome()

        bytecode1 = compile_genome(genome)
        bytecode2 = compile_genome(genome)

        assert bytecode1 == bytecode2, "Bytecode compilation is not deterministic"

    def test_hearts_bytecode_includes_card_scoring(self) -> None:
        """Hearts genome bytecode includes explicit card_scoring section.

        Hearts has two card scoring rules:
        1. Any Heart card = 1 point (on TRICK_WIN)
        2. Queen of Spades = 13 points (on TRICK_WIN)

        This test verifies the bytecode header has a valid card_scoring_offset
        and the section contains the expected number of rules.
        """
        genome = create_hearts_genome()
        bytecode = compile_genome(genome)

        header = BytecodeHeader.from_bytes(bytecode)

        # Card scoring offset should be set (non-zero, positive)
        assert header.card_scoring_offset > 0, "card_scoring_offset should be positive"

        # Offset must be within bytecode bounds
        assert header.card_scoring_offset < len(bytecode), \
            f"card_scoring_offset {header.card_scoring_offset} exceeds bytecode length {len(bytecode)}"

        # Read rule count at offset (2 bytes, big-endian)
        offset = header.card_scoring_offset
        rule_count = int.from_bytes(bytecode[offset:offset+2], 'big')
        assert rule_count == 2, f"Expected 2 card scoring rules (Hearts + QS), got {rule_count}"

        # Verify genome has the expected card_scoring rules for sanity
        assert len(genome.card_scoring) == 2
        # Rule 1: Hearts suit = 1 point
        assert genome.card_scoring[0].points == 1
        # Rule 2: Queen of Spades = 13 points
        assert genome.card_scoring[1].points == 13

    @pytest.mark.skip(reason="Requires libcardsim.so to be built")
    def test_go_can_parse_bytecode(self) -> None:
        """Go engine should successfully parse Python-compiled bytecode.

        This test requires CGo bindings to be built.
        """
        from darwindeck.bindings.cgo_bridge import simulate_batch
        from darwindeck.bindings.cardsim import (
            AggregatedStats,
            BatchRequest,
            SimulationRequest,
        )
        import flatbuffers

        genome = create_war_genome()
        bytecode = compile_genome(genome)

        # Build simulation request
        builder = flatbuffers.Builder(1024)

        # Create genome bytecode vector
        genome_offset = builder.CreateByteVector(bytecode)

        # Build SimulationRequest
        SimulationRequest.Start(builder)
        SimulationRequest.AddGenomeBytecode(builder, genome_offset)
        SimulationRequest.AddNumGames(builder, 10)
        SimulationRequest.AddAiPlayerType(builder, 0)  # Random
        SimulationRequest.AddMctsIterations(builder, 100)
        SimulationRequest.AddRandomSeed(builder, 42)
        req_offset = SimulationRequest.End(builder)

        # Build requests vector
        BatchRequest.StartRequestsVector(builder, 1)
        builder.PrependUOffsetTRelative(req_offset)
        requests_offset = builder.EndVector()

        # Build BatchRequest
        BatchRequest.Start(builder)
        BatchRequest.AddBatchId(builder, 1)
        BatchRequest.AddRequests(builder, requests_offset)
        batch_offset = BatchRequest.End(builder)

        builder.Finish(batch_offset)
        request_bytes = bytes(builder.Output())

        # Send to Go
        response = simulate_batch(request_bytes)

        # Verify response
        assert response.BatchId() == 1
        assert response.ResultsLength() == 1

        result = response.Results(0)
        assert result.TotalGames() == 10
        assert result.Errors() == 0  # No parsing errors


class TestGameStateEquivalence:
    """Verify Python and Go game state representations match."""

    @pytest.mark.skip(reason="Requires libcardsim.so to be built")
    def test_war_simulation_matches(self) -> None:
        """Python and Go should produce same results for War.

        This is the critical integration test - if Python random AI
        gets same results as Go random AI with same seed, the implementations
        are equivalent.
        """
        # This will be implemented in Task 8 when batch engine is ready
        pytest.skip("Deferred to Task 8")
