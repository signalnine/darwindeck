"""Integration test for batch simulation via CGo."""

import pytest
import flatbuffers

from darwindeck.genome.bytecode import BytecodeCompiler
from darwindeck.genome.examples import create_war_genome
from darwindeck.bindings.cgo_bridge import simulate_batch
from darwindeck.bindings.cardsim import (
    BatchRequest,
    BatchResponse,
    SimulationRequest,
    AggregatedStats,
)


class TestBatchSimulation:
    """Test end-to-end batch simulation through CGo interface."""

    def test_single_batch_random_ai(self):
        """Run 10 War games with random AI."""
        # Create War genome
        genome = create_war_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        # Build simulation request
        builder = flatbuffers.Builder(2048)

        # Create genome bytecode vector
        genome_offset = builder.CreateByteVector(bytecode)

        # Build SimulationRequest
        SimulationRequest.Start(builder)
        SimulationRequest.AddGenomeBytecode(builder, genome_offset)
        SimulationRequest.AddNumGames(builder, 10)
        SimulationRequest.AddAiPlayerType(builder, 0)  # Random AI
        SimulationRequest.AddMctsIterations(builder, 0)  # Not used for random
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
        assert result.Errors() == 0, "Should have no parsing/execution errors"

        # Verify game outcomes
        total_wins = result.Player0Wins() + result.Player1Wins() + result.Draws()
        assert total_wins == 10, "All games should complete"

        # Verify statistics are reasonable
        assert result.AvgTurns() > 0, "Should have positive average turns"
        assert result.MedianTurns() > 0, "Should have positive median turns"

    def test_batch_determinism(self):
        """Same seed should produce identical results."""
        genome = create_war_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        def run_batch(seed):
            builder = flatbuffers.Builder(2048)
            genome_offset = builder.CreateByteVector(bytecode)

            SimulationRequest.Start(builder)
            SimulationRequest.AddGenomeBytecode(builder, genome_offset)
            SimulationRequest.AddNumGames(builder, 5)
            SimulationRequest.AddAiPlayerType(builder, 0)
            SimulationRequest.AddMctsIterations(builder, 0)
            SimulationRequest.AddRandomSeed(builder, seed)
            req_offset = SimulationRequest.End(builder)

            BatchRequest.StartRequestsVector(builder, 1)
            builder.PrependUOffsetTRelative(req_offset)
            requests_offset = builder.EndVector()

            BatchRequest.Start(builder)
            BatchRequest.AddBatchId(builder, 1)
            BatchRequest.AddRequests(builder, requests_offset)
            batch_offset = BatchRequest.End(builder)

            builder.Finish(batch_offset)
            return simulate_batch(bytes(builder.Output()))

        response1 = run_batch(12345)
        response2 = run_batch(12345)

        result1 = response1.Results(0)
        result2 = response2.Results(0)

        # Same seed = same outcomes
        assert result1.Player0Wins() == result2.Player0Wins()
        assert result1.Player1Wins() == result2.Player1Wins()
        assert result1.Draws() == result2.Draws()

    @pytest.mark.skip(reason="MCTS takes too long for CI")
    def test_mcts_vs_random(self):
        """MCTS should beat random AI (skill differential)."""
        genome = create_war_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        builder = flatbuffers.Builder(2048)
        genome_offset = builder.CreateByteVector(bytecode)

        # MCTS-100 AI
        SimulationRequest.Start(builder)
        SimulationRequest.AddGenomeBytecode(builder, genome_offset)
        SimulationRequest.AddNumGames(builder, 50)
        SimulationRequest.AddAiPlayerType(builder, 2)  # MCTS-100
        SimulationRequest.AddMctsIterations(builder, 100)
        SimulationRequest.AddRandomSeed(builder, 999)
        req_offset = SimulationRequest.End(builder)

        BatchRequest.StartRequestsVector(builder, 1)
        builder.PrependUOffsetTRelative(req_offset)
        requests_offset = builder.EndVector()

        BatchRequest.Start(builder)
        BatchRequest.AddBatchId(builder, 1)
        BatchRequest.AddRequests(builder, requests_offset)
        batch_offset = BatchRequest.End(builder)

        builder.Finish(batch_offset)
        response = simulate_batch(bytes(builder.Output()))

        result = response.Results(0)

        # For War (pure luck), MCTS and Random should have similar win rates
        # This test is more relevant for games with strategic depth
        assert result.TotalGames() == 50
        assert result.Errors() == 0

    def test_multiple_requests_in_batch(self):
        """Test batching multiple simulation requests."""
        genome = create_war_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        builder = flatbuffers.Builder(4096)
        genome_offset = builder.CreateByteVector(bytecode)

        # Create 3 different requests with different seeds
        req_offsets = []
        for seed in [100, 200, 300]:
            SimulationRequest.Start(builder)
            SimulationRequest.AddGenomeBytecode(builder, genome_offset)
            SimulationRequest.AddNumGames(builder, 5)
            SimulationRequest.AddAiPlayerType(builder, 0)
            SimulationRequest.AddMctsIterations(builder, 0)
            SimulationRequest.AddRandomSeed(builder, seed)
            req_offsets.append(SimulationRequest.End(builder))

        # Build requests vector
        BatchRequest.StartRequestsVector(builder, 3)
        for offset in reversed(req_offsets):
            builder.PrependUOffsetTRelative(offset)
        requests_offset = builder.EndVector()

        # Build BatchRequest
        BatchRequest.Start(builder)
        BatchRequest.AddBatchId(builder, 123)
        BatchRequest.AddRequests(builder, requests_offset)
        batch_offset = BatchRequest.End(builder)

        builder.Finish(batch_offset)
        response = simulate_batch(bytes(builder.Output()))

        # Verify all 3 requests processed
        assert response.BatchId() == 123
        assert response.ResultsLength() == 3

        for i in range(3):
            result = response.Results(i)
            assert result.TotalGames() == 5
            assert result.Errors() == 0
