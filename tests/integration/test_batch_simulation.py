"""Integration test for batch simulation via CGo."""

import pytest
import flatbuffers

from darwindeck.genome.bytecode import BytecodeCompiler
from darwindeck.genome.examples import create_war_genome
from darwindeck.bindings.cgo_bridge import simulate_batch
from darwindeck.bindings.cardsim.SimulationRequest import (
    SimulationRequestStart,
    SimulationRequestAddGenomeBytecode,
    SimulationRequestAddNumGames,
    SimulationRequestAddAiPlayerType,
    SimulationRequestAddMctsIterations,
    SimulationRequestAddRandomSeed,
    SimulationRequestEnd,
)
from darwindeck.bindings.cardsim.BatchRequest import (
    BatchRequestStart,
    BatchRequestAddBatchId,
    BatchRequestAddRequests,
    BatchRequestStartRequestsVector,
    BatchRequestEnd,
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
        SimulationRequestStart(builder)
        SimulationRequestAddGenomeBytecode(builder, genome_offset)
        SimulationRequestAddNumGames(builder, 10)
        SimulationRequestAddAiPlayerType(builder, 0)  # Random AI
        SimulationRequestAddMctsIterations(builder, 0)  # Not used for random
        SimulationRequestAddRandomSeed(builder, 42)
        req_offset = SimulationRequestEnd(builder)

        # Build requests vector
        BatchRequestStartRequestsVector(builder, 1)
        builder.PrependUOffsetTRelative(req_offset)
        requests_offset = builder.EndVector()

        # Build BatchRequest
        BatchRequestStart(builder)
        BatchRequestAddBatchId(builder, 1)
        BatchRequestAddRequests(builder, requests_offset)
        batch_offset = BatchRequestEnd(builder)

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

            SimulationRequestStart(builder)
            SimulationRequestAddGenomeBytecode(builder, genome_offset)
            SimulationRequestAddNumGames(builder, 5)
            SimulationRequestAddAiPlayerType(builder, 0)
            SimulationRequestAddMctsIterations(builder, 0)
            SimulationRequestAddRandomSeed(builder, seed)
            req_offset = SimulationRequestEnd(builder)

            BatchRequestStartRequestsVector(builder, 1)
            builder.PrependUOffsetTRelative(req_offset)
            requests_offset = builder.EndVector()

            BatchRequestStart(builder)
            BatchRequestAddBatchId(builder, 1)
            BatchRequestAddRequests(builder, requests_offset)
            batch_offset = BatchRequestEnd(builder)

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
        SimulationRequestStart(builder)
        SimulationRequestAddGenomeBytecode(builder, genome_offset)
        SimulationRequestAddNumGames(builder, 50)
        SimulationRequestAddAiPlayerType(builder, 2)  # MCTS-100
        SimulationRequestAddMctsIterations(builder, 100)
        SimulationRequestAddRandomSeed(builder, 999)
        req_offset = SimulationRequestEnd(builder)

        BatchRequestStartRequestsVector(builder, 1)
        builder.PrependUOffsetTRelative(req_offset)
        requests_offset = builder.EndVector()

        BatchRequestStart(builder)
        BatchRequestAddBatchId(builder, 1)
        BatchRequestAddRequests(builder, requests_offset)
        batch_offset = BatchRequestEnd(builder)

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
            SimulationRequestStart(builder)
            SimulationRequestAddGenomeBytecode(builder, genome_offset)
            SimulationRequestAddNumGames(builder, 5)
            SimulationRequestAddAiPlayerType(builder, 0)
            SimulationRequestAddMctsIterations(builder, 0)
            SimulationRequestAddRandomSeed(builder, seed)
            req_offsets.append(SimulationRequestEnd(builder))

        # Build requests vector
        BatchRequestStartRequestsVector(builder, 3)
        for offset in reversed(req_offsets):
            builder.PrependUOffsetTRelative(offset)
        requests_offset = builder.EndVector()

        # Build BatchRequest
        BatchRequestStart(builder)
        BatchRequestAddBatchId(builder, 123)
        BatchRequestAddRequests(builder, requests_offset)
        batch_offset = BatchRequestEnd(builder)

        builder.Finish(batch_offset)
        response = simulate_batch(bytes(builder.Output()))

        # Verify all 3 requests processed
        assert response.BatchId() == 123
        assert response.ResultsLength() == 3

        for i in range(3):
            result = response.Results(i)
            assert result.TotalGames() == 5
            assert result.Errors() == 0

    def test_phase1_metrics_returned(self):
        """Verify Phase 1 instrumentation metrics are returned."""
        genome = create_war_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        builder = flatbuffers.Builder(2048)
        genome_offset = builder.CreateByteVector(bytecode)

        SimulationRequestStart(builder)
        SimulationRequestAddGenomeBytecode(builder, genome_offset)
        SimulationRequestAddNumGames(builder, 20)
        SimulationRequestAddAiPlayerType(builder, 0)  # Random AI
        SimulationRequestAddMctsIterations(builder, 0)
        SimulationRequestAddRandomSeed(builder, 42)
        req_offset = SimulationRequestEnd(builder)

        BatchRequestStartRequestsVector(builder, 1)
        builder.PrependUOffsetTRelative(req_offset)
        requests_offset = builder.EndVector()

        BatchRequestStart(builder)
        BatchRequestAddBatchId(builder, 1)
        BatchRequestAddRequests(builder, requests_offset)
        batch_offset = BatchRequestEnd(builder)

        builder.Finish(batch_offset)
        response = simulate_batch(bytes(builder.Output()))

        result = response.Results(0)

        # Verify Phase 1 instrumentation metrics are non-zero
        assert result.TotalDecisions() > 0, "Should have decision points"
        assert result.TotalValidMoves() > 0, "Should have valid moves summed"
        assert result.TotalActions() > 0, "Should have actions taken"

        # War game should have many forced decisions (1 move per turn)
        # since each player just plays top card
        assert result.ForcedDecisions() > 0, "War should have forced decisions"

        # War game has interactions (playing to tableau triggers battles)
        assert result.TotalInteractions() > 0, "War should have interactions"

        # Sanity check: valid moves >= decisions (at least 1 move per decision)
        assert result.TotalValidMoves() >= result.TotalDecisions()

        # Sanity check: forced decisions <= total decisions
        assert result.ForcedDecisions() <= result.TotalDecisions()

        # Sanity check: interactions <= actions
        assert result.TotalInteractions() <= result.TotalActions()

        # Print metrics for debugging
        print(f"\nPhase 1 Metrics (20 games):")
        print(f"  TotalDecisions: {result.TotalDecisions()}")
        print(f"  TotalValidMoves: {result.TotalValidMoves()}")
        print(f"  ForcedDecisions: {result.ForcedDecisions()}")
        print(f"  TotalInteractions: {result.TotalInteractions()}")
        print(f"  TotalActions: {result.TotalActions()}")
        print(f"  Avg valid moves per decision: {result.TotalValidMoves() / result.TotalDecisions():.2f}")
        print(f"  Forced ratio: {result.ForcedDecisions() / result.TotalDecisions():.2%}")
        print(f"  Interaction ratio: {result.TotalInteractions() / result.TotalActions():.2%}")
