"""Golden test suite for Python↔Go bytecode equivalence.

This test ensures that Python-compiled bytecode is correctly parsed and
executed by the Go engine, validating the entire pipeline.
"""

import pytest
from darwindeck.genome.bytecode import BytecodeCompiler
from darwindeck.genome.examples import create_war_genome


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

        # Should start with version (1) in first 4 bytes
        import struct

        version = struct.unpack("!I", bytecode[0:4])[0]
        assert version == 1, f"Expected version 1, got {version}"

    def test_header_structure(self) -> None:
        """Bytecode header should match Go expectations (36 bytes)."""
        genome = create_war_genome()
        bytecode = compile_genome(genome)

        # Header is exactly 36 bytes
        assert len(bytecode) >= 36

        # Parse offsets (bytes 20-36)
        import struct

        offsets = struct.unpack("!iiii", bytecode[20:36])
        setup_offset, turn_offset, win_offset, score_offset = offsets

        # All offsets should be within bytecode length
        assert 0 <= setup_offset < len(bytecode)
        assert 0 <= turn_offset < len(bytecode)
        assert 0 <= win_offset < len(bytecode)
        # score_offset may be -1 if not used

    def test_turn_structure_encoding(self) -> None:
        """Turn structure phases should be encoded correctly."""
        genome = create_war_genome()
        bytecode = compile_genome(genome)

        import struct

        # Get turn structure offset (byte 24-28)
        turn_offset = struct.unpack("!i", bytecode[24:28])[0]

        # First 4 bytes at offset = phase count
        phase_count = struct.unpack("!I", bytecode[turn_offset : turn_offset + 4])[0]

        assert phase_count > 0, "War should have at least one phase"
        assert phase_count <= 5, "Unexpected number of phases"

    def test_win_conditions_encoding(self) -> None:
        """Win conditions should be encoded correctly."""
        genome = create_war_genome()
        bytecode = compile_genome(genome)

        import struct

        # Get win conditions offset (byte 28-32)
        win_offset = struct.unpack("!i", bytecode[28:32])[0]

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
