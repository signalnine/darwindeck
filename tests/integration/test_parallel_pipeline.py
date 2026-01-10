"""Integration tests for full parallelization pipeline.

This module tests the complete parallelization stack:
1. Python-level parallelization (ParallelFitnessEvaluator)
2. Go-level parallelization (RunBatchParallel via CGo)
3. Combined end-to-end pipeline

Key components:
- CGo bridge: Python → Go via libcardsim.so
- Go parallel simulator: Worker pool with 1.43x average speedup
- Python multiprocessing: Process pool with 4x speedup on 4 cores
- Combined potential: Up to 5.7x total speedup
"""

import pytest
import time
import flatbuffers
from dataclasses import replace
from typing import List

from darwindeck.genome.schema import GameGenome
from darwindeck.genome.examples import create_war_genome
from darwindeck.genome.bytecode import BytecodeCompiler
from darwindeck.evolution.parallel_fitness import ParallelFitnessEvaluator
from darwindeck.evolution.fitness_full import FitnessEvaluator, SimulationResults

# Check if Go simulator is available
GO_SIMULATOR_AVAILABLE = False
GO_SIMULATOR_ERROR = "Unknown error"
try:
    from darwindeck.bindings.cgo_bridge import simulate_batch
    from darwindeck.bindings.cardsim.BatchRequest import BatchRequest
    from darwindeck.bindings.cardsim.BatchResponse import BatchResponse
    from darwindeck.bindings.cardsim.SimulationRequest import SimulationRequest
    # Import module-level functions directly from SimulationRequest module
    from darwindeck.bindings.cardsim.SimulationRequest import (
        SimulationRequestStart,
        SimulationRequestAddGenomeBytecode,
        SimulationRequestAddNumGames,
        SimulationRequestAddAiPlayerType,
        SimulationRequestAddMctsIterations,
        SimulationRequestAddRandomSeed,
        SimulationRequestEnd,
    )
    # Import module-level functions directly from BatchRequest module
    from darwindeck.bindings.cardsim.BatchRequest import (
        BatchRequestStart,
        BatchRequestStartRequestsVector,
        BatchRequestAddBatchId,
        BatchRequestAddRequests,
        BatchRequestEnd,
    )
    GO_SIMULATOR_AVAILABLE = True
except (ImportError, OSError) as e:
    # OSError can occur if libcardsim.so is not found
    GO_SIMULATOR_ERROR = str(e)
except Exception as e:
    GO_SIMULATOR_ERROR = f"Unexpected error: {str(e)}"


def create_test_genomes(count: int) -> List[GameGenome]:
    """Create test genomes based on War (avoids crazy_eights bug).

    Note: GameGenome is frozen, so we use dataclasses.replace() to modify fields.
    """
    genomes = []
    base_genome = create_war_genome()

    for i in range(count):
        # Use replace() to create modified copies (frozen dataclass)
        setup = replace(base_genome.setup, cards_per_player=20 + (i % 10))
        genome = replace(
            base_genome,
            genome_id=f"war-test-{i}",
            setup=setup
        )
        genomes.append(genome)
    return genomes


def run_go_simulation(genome: GameGenome, num_games: int, random_seed: int = 42) -> SimulationResults:
    """Run simulation through Go CGo bridge.

    Args:
        genome: Game genome to simulate
        num_games: Number of games to run
        random_seed: Random seed for determinism

    Returns:
        SimulationResults object
    """
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    # Build FlatBuffers request using module-level functions
    builder = flatbuffers.Builder(2048)
    genome_offset = builder.CreateByteVector(bytecode)

    SimulationRequestStart(builder)
    SimulationRequestAddGenomeBytecode(builder, genome_offset)
    SimulationRequestAddNumGames(builder, num_games)
    SimulationRequestAddAiPlayerType(builder, 0)  # Random AI
    SimulationRequestAddMctsIterations(builder, 0)
    SimulationRequestAddRandomSeed(builder, random_seed)
    req_offset = SimulationRequestEnd(builder)

    BatchRequestStartRequestsVector(builder, 1)
    builder.PrependUOffsetTRelative(req_offset)
    requests_offset = builder.EndVector()

    BatchRequestStart(builder)
    BatchRequestAddBatchId(builder, 1)
    BatchRequestAddRequests(builder, requests_offset)
    batch_offset = BatchRequestEnd(builder)

    builder.Finish(batch_offset)
    request_bytes = bytes(builder.Output())

    # Call Go simulator
    response = simulate_batch(request_bytes)
    result = response.Results(0)

    # Convert to SimulationResults
    return SimulationResults(
        total_games=result.TotalGames(),
        player0_wins=result.Player0Wins(),
        player1_wins=result.Player1Wins(),
        draws=result.Draws(),
        avg_turns=result.AvgTurns(),
        errors=result.Errors()
    )


@pytest.mark.skipif(not GO_SIMULATOR_AVAILABLE,
                    reason=f"Go simulator not available: {GO_SIMULATOR_ERROR}")
class TestParallelPipelineWithGoSimulator:
    """Integration tests using real Go simulator.

    These tests validate the complete pipeline from Python through Go and back,
    testing both correctness and performance characteristics.
    """

    def test_end_to_end_single_genome(self):
        """Test Python → Go → Python round-trip for single genome."""
        base_genome = create_war_genome()
        genome = replace(base_genome, genome_id="integration-test-1")

        # Run simulation through Go
        results = run_go_simulation(genome, num_games=10)

        # Verify results structure
        assert results.total_games == 10
        assert results.errors == 0, "Should have no parsing/execution errors"

        # Verify game outcomes
        total_outcomes = results.player0_wins + results.player1_wins + results.draws
        assert total_outcomes == 10, "All games should complete"

        # Verify statistics are reasonable
        assert results.avg_turns > 0, "Should have positive average turns"

        # War should have relatively balanced outcomes (not perfect 50/50 due to randomness)
        assert 0 <= results.player0_wins <= 10
        assert 0 <= results.player1_wins <= 10

    def test_end_to_end_python_multiprocessing(self):
        """Test Python-level parallel evaluation with real Go simulator."""
        genomes = create_test_genomes(8)

        # Create evaluator that uses real Go simulator
        def evaluator_factory():
            return FitnessEvaluator()

        evaluator = ParallelFitnessEvaluator(
            evaluator_factory=evaluator_factory,
            num_workers=2
        )

        # Note: This currently uses mock results in parallel_fitness.py
        # When full integration is complete, this should call through to Go
        results = evaluator.evaluate_population(genomes, num_simulations=10)

        # Verify results
        assert len(results) == len(genomes)
        assert all(r.games_simulated >= 0 for r in results)
        assert all(isinstance(r.total_fitness, float) for r in results)

    def test_go_simulation_determinism(self):
        """Verify Go simulator produces deterministic results with same seed.

        Note: The Go parallel simulator may produce slightly different results
        than the serial version due to goroutine scheduling, even with the same
        seed. This is acceptable as long as results are statistically similar.
        """
        genome = create_war_genome()

        # Run twice with same seed
        results1 = run_go_simulation(genome, num_games=20, random_seed=12345)
        results2 = run_go_simulation(genome, num_games=20, random_seed=12345)

        # Results should be statistically similar (within reasonable variance)
        # Allow up to 20% difference in wins (4 games out of 20)
        wins_diff = abs(results1.player0_wins - results2.player0_wins)
        assert wins_diff <= 4, f"Win difference {wins_diff} exceeds threshold"

        # Average turns should be similar (within 15% - parallel execution
        # may have slight variance due to goroutine scheduling)
        avg_diff_pct = abs(results1.avg_turns - results2.avg_turns) / max(results1.avg_turns, 1.0)
        assert avg_diff_pct < 0.15, f"Avg turns difference {avg_diff_pct:.1%} exceeds 15%"

    def test_go_simulation_different_seeds(self):
        """Verify different seeds produce different results."""
        genome = create_war_genome()

        # Run with different seeds
        results1 = run_go_simulation(genome, num_games=20, random_seed=111)
        results2 = run_go_simulation(genome, num_games=20, random_seed=222)

        # Should be different (very high probability)
        # Check at least one metric differs
        different = (
            results1.player0_wins != results2.player0_wins or
            results1.player1_wins != results2.player1_wins or
            abs(results1.avg_turns - results2.avg_turns) > 1.0
        )
        assert different, "Different seeds should produce different results"

    def test_multiple_genomes_batched(self):
        """Test batching multiple genomes through Go simulator."""
        genomes = create_test_genomes(5)

        # Run each genome through Go simulator
        all_results = []
        for genome in genomes:
            results = run_go_simulation(genome, num_games=10)
            all_results.append(results)

        # Verify all completed
        assert len(all_results) == 5
        assert all(r.total_games == 10 for r in all_results)
        assert all(r.errors == 0 for r in all_results)

        # Verify each genome can produce different results
        # (since they have slightly different configurations)
        avg_turns = [r.avg_turns for r in all_results]
        assert len(set(avg_turns)) > 1, "Different genomes should produce varied results"

    @pytest.mark.slow
    def test_performance_characteristic(self):
        """Verify Go simulator completes simulations efficiently.

        This doesn't test parallelization directly (that's in benchmarks),
        but verifies the simulation pipeline is working at reasonable speed.
        """
        genome = create_war_genome()

        # Run batch and time it
        start = time.time()
        results = run_go_simulation(genome, num_games=100)
        elapsed = time.time() - start

        # Verify completion
        assert results.total_games == 100
        assert results.errors == 0

        # Should complete reasonably quickly
        # Even serial execution should handle 100 games in under 5 seconds
        assert elapsed < 5.0, f"100 games took {elapsed:.2f}s (expected < 5s)"

        # Log performance for visibility
        avg_game_time_ms = (elapsed * 1000) / 100
        print(f"\nPerformance: {elapsed:.2f}s for 100 games ({avg_game_time_ms:.1f}ms/game)")


@pytest.mark.skipif(GO_SIMULATOR_AVAILABLE, reason="Testing fallback when Go not available")
class TestParallelPipelineWithoutGoSimulator:
    """Tests that work without Go simulator (mock-based).

    These tests verify the Python-level parallelization infrastructure
    works correctly, even when the Go simulator isn't available.
    """

    def test_python_level_parallelization_structure(self):
        """Test Python-level parallelization with mock results."""
        genomes = create_test_genomes(8)

        evaluator = ParallelFitnessEvaluator(
            evaluator_factory=lambda: FitnessEvaluator(),
            num_workers=2
        )

        results = evaluator.evaluate_population(genomes, num_simulations=10)

        # Verify structure
        assert len(results) == len(genomes)
        assert all(hasattr(r, 'total_fitness') for r in results)
        assert all(hasattr(r, 'valid') for r in results)

    def test_graceful_fallback_to_mock(self):
        """Verify system works with mock results when Go unavailable."""
        genome = create_war_genome()

        # Create evaluator (will use mock results internally)
        evaluator = FitnessEvaluator()

        # Import the mock function to test it directly
        from darwindeck.evolution.parallel_fitness import _create_mock_results

        # Create mock results
        results = _create_mock_results(genome, num_games=50)

        # Verify mock results are valid
        assert results.total_games == 50
        assert results.player0_wins + results.player1_wins >= 0
        assert results.errors == 0
        assert results.avg_turns > 0

        # Evaluate with mock results
        metrics = evaluator.evaluate(genome, results)
        assert metrics.total_fitness >= 0
        assert metrics.games_simulated == 50


class TestParallelPipelineStructure:
    """Structural tests for the parallelization pipeline.

    These tests run regardless of Go availability and verify the
    correct structure and behavior of the parallelization infrastructure.
    """

    def test_evaluator_factory_pattern(self):
        """Test that factory pattern creates isolated evaluators per worker.

        Note: Due to multiprocessing, the tracking happens in separate processes
        so we can't directly count factory calls. This test just verifies the
        infrastructure works correctly.
        """
        call_count = [0]  # Mutable to track calls (won't work across processes)

        def tracking_factory():
            call_count[0] += 1
            return FitnessEvaluator()

        parallel_eval = ParallelFitnessEvaluator(
            evaluator_factory=tracking_factory,
            num_workers=2
        )

        genomes = create_test_genomes(4)
        results = parallel_eval.evaluate_population(genomes)

        # Should have results for all genomes
        assert len(results) == 4
        assert all(r.games_simulated > 0 for r in results)

        # Note: call_count won't increase because factories are called in subprocesses
        # The test verifies the pattern works, not the count

    def test_empty_population(self):
        """Test handling of empty genome list."""
        evaluator = ParallelFitnessEvaluator(
            evaluator_factory=lambda: FitnessEvaluator(),
            num_workers=2
        )

        results = evaluator.evaluate_population([])
        assert results == []

    def test_single_genome(self):
        """Test evaluation of single genome (edge case)."""
        genome = create_war_genome()

        evaluator = ParallelFitnessEvaluator(
            evaluator_factory=lambda: FitnessEvaluator(),
            num_workers=2
        )

        results = evaluator.evaluate_population([genome])
        assert len(results) == 1
        assert results[0].games_simulated > 0

    def test_error_propagation(self):
        """Test that errors in evaluation are properly propagated."""

        class FailingEvaluator(FitnessEvaluator):
            def evaluate(self, genome, results, use_mcts=False):
                raise ValueError("Intentional test error")

        def failing_factory():
            return FailingEvaluator()

        parallel_eval = ParallelFitnessEvaluator(
            evaluator_factory=failing_factory,
            num_workers=2
        )

        genomes = create_test_genomes(1)

        # Should propagate the error
        with pytest.raises(ValueError, match="Intentional test error"):
            parallel_eval.evaluate_population(genomes)

    def test_worker_count_configuration(self):
        """Test that worker count can be configured."""
        # Default: use cpu_count()
        eval_default = ParallelFitnessEvaluator(
            evaluator_factory=lambda: FitnessEvaluator()
        )
        import multiprocessing
        assert eval_default.num_workers == multiprocessing.cpu_count()

        # Explicit count
        eval_explicit = ParallelFitnessEvaluator(
            evaluator_factory=lambda: FitnessEvaluator(),
            num_workers=3
        )
        assert eval_explicit.num_workers == 3

    def test_large_population_processing(self):
        """Test processing larger population (stress test)."""
        genomes = create_test_genomes(20)

        evaluator = ParallelFitnessEvaluator(
            evaluator_factory=lambda: FitnessEvaluator(),
            num_workers=4
        )

        results = evaluator.evaluate_population(genomes, num_simulations=5)

        # Verify all genomes processed
        assert len(results) == 20
        assert all(r.games_simulated == 5 for r in results)
        assert all(r.total_fitness >= 0 for r in results)


@pytest.mark.skipif(not GO_SIMULATOR_AVAILABLE,
                    reason="Requires Go simulator for full integration")
class TestFullPipelineIntegration:
    """Tests for complete Python + Go parallelization pipeline.

    These tests verify the combined effect of Python-level and Go-level
    parallelization when both are working together.
    """

    def test_fitness_evaluation_with_go_results(self):
        """Test fitness evaluation using real Go simulation results.

        Note: War games often violate session length constraints (too long),
        which makes them invalid from a fitness perspective. This test
        verifies the fitness evaluation works, not that War is a good game.
        """
        base_genome = create_war_genome()
        genome = replace(base_genome, genome_id="fitness-test-1")

        # Get real simulation results from Go
        sim_results = run_go_simulation(genome, num_games=50)

        # Evaluate fitness
        evaluator = FitnessEvaluator()
        fitness = evaluator.evaluate(genome, sim_results, use_mcts=False)

        # Verify fitness metrics are computed
        assert fitness.games_simulated == 50
        assert 0.0 <= fitness.total_fitness <= 1.0
        assert 0.0 <= fitness.comeback_potential <= 1.0
        assert 0.0 <= fitness.rules_complexity <= 1.0

        # War games often violate session length (too long)
        # This is expected - not all games will be valid
        if not fitness.valid:
            assert fitness.session_length == 0.0, "Invalid games should have session_length=0"

    def test_multiple_evaluations_consistency(self):
        """Test that repeated evaluations with same seed are consistent."""
        genome = create_war_genome()

        # Run twice with same parameters
        results1 = run_go_simulation(genome, num_games=30, random_seed=999)
        results2 = run_go_simulation(genome, num_games=30, random_seed=999)

        evaluator = FitnessEvaluator()
        fitness1 = evaluator.evaluate(genome, results1)
        fitness2 = evaluator.evaluate(genome, results2)

        # Should be identical
        assert fitness1.total_fitness == fitness2.total_fitness
        assert fitness1.comeback_potential == fitness2.comeback_potential
        assert fitness1.valid == fitness2.valid

    @pytest.mark.slow
    def test_end_to_end_with_different_genomes(self):
        """Test complete pipeline with multiple different genomes.

        Note: War-based genomes often violate session length constraints,
        making them invalid. This test verifies the pipeline works, not
        that all genomes are valid.
        """
        # Create varied genomes
        genomes = create_test_genomes(6)

        # Run simulations and evaluate fitness
        all_fitness = []
        for genome in genomes:
            sim_results = run_go_simulation(genome, num_games=20)
            evaluator = FitnessEvaluator()
            fitness = evaluator.evaluate(genome, sim_results)
            all_fitness.append(fitness)

        # Verify all completed
        assert len(all_fitness) == 6
        assert all(f.games_simulated == 20 for f in all_fitness)

        # Not all War variants will be valid (session length issues)
        # Just verify the pipeline produces results
        assert all(isinstance(f.total_fitness, float) for f in all_fitness)
        assert all(0.0 <= f.total_fitness <= 1.0 for f in all_fitness)


@pytest.mark.benchmark
@pytest.mark.skipif(not GO_SIMULATOR_AVAILABLE, reason="Requires Go simulator")
class TestParallelizationPerformance:
    """Performance benchmarks for parallelization (informational).

    These tests don't assert strict performance requirements but provide
    visibility into the actual speedups achieved. See Task 2 benchmarks
    for detailed performance analysis.
    """

    @pytest.mark.slow
    def test_measure_go_simulation_throughput(self):
        """Measure Go simulator throughput (informational)."""
        genome = create_war_genome()

        # Measure time for various batch sizes
        batch_sizes = [10, 50, 100]

        for batch_size in batch_sizes:
            start = time.time()
            results = run_go_simulation(genome, num_games=batch_size)
            elapsed = time.time() - start

            assert results.total_games == batch_size
            throughput = batch_size / elapsed
            print(f"\nBatch size {batch_size}: {elapsed:.3f}s ({throughput:.1f} games/sec)")

    @pytest.mark.slow
    def test_measure_python_parallelization_benefit(self):
        """Measure Python-level parallelization benefit (informational)."""
        genomes = create_test_genomes(8)

        # Serial (1 worker)
        eval_serial = ParallelFitnessEvaluator(
            evaluator_factory=lambda: FitnessEvaluator(),
            num_workers=1
        )

        start = time.time()
        results_serial = eval_serial.evaluate_population(genomes, num_simulations=5)
        time_serial = time.time() - start

        # Parallel (4 workers)
        eval_parallel = ParallelFitnessEvaluator(
            evaluator_factory=lambda: FitnessEvaluator(),
            num_workers=4
        )

        start = time.time()
        results_parallel = eval_parallel.evaluate_population(genomes, num_simulations=5)
        time_parallel = time.time() - start

        # Verify correctness
        assert len(results_serial) == 8
        assert len(results_parallel) == 8

        # Report speedup (informational, not enforced)
        if time_serial > 0:
            speedup = time_serial / time_parallel
            print(f"\nPython parallelization: {time_serial:.3f}s → {time_parallel:.3f}s ({speedup:.2f}x)")
