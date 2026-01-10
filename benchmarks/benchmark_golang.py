"""Benchmark Python vs Golang simulation performance.

Compares War game simulation speeds to verify 10-50x speedup target.
"""

import time
import flatbuffers

from cards_evolve.genome.bytecode import BytecodeCompiler
from cards_evolve.genome.examples import create_war_genome
from cards_evolve.bindings.cgo_bridge import simulate_batch
from cards_evolve.bindings.cardsim import (
    SimulationRequest,
    BatchRequest,
    BatchResponse,
)


def benchmark_go_war(num_games: int = 1000, batch_size: int = 100) -> dict:
    """Benchmark Go implementation via CGo."""
    genome = create_war_genome()
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    num_batches = num_games // batch_size
    total_duration = 0
    total_games = 0

    print(f"Running {num_games} War games through Go (batches of {batch_size})...")

    start_time = time.perf_counter()

    for batch_idx in range(num_batches):
        builder = flatbuffers.Builder(2048)

        # Create genome bytecode vector
        genome_offset = builder.CreateByteVector(bytecode)

        # Build SimulationRequest
        SimulationRequest.SimulationRequestStart(builder)
        SimulationRequest.SimulationRequestAddGenomeBytecode(builder, genome_offset)
        SimulationRequest.SimulationRequestAddNumGames(builder, batch_size)
        SimulationRequest.SimulationRequestAddAiPlayerType(builder, 0)  # Random AI
        SimulationRequest.SimulationRequestAddMctsIterations(builder, 0)
        SimulationRequest.SimulationRequestAddRandomSeed(builder, 42 + batch_idx)
        req_offset = SimulationRequest.SimulationRequestEnd(builder)

        # Build requests vector
        BatchRequest.BatchRequestStartRequestsVector(builder, 1)
        builder.PrependUOffsetTRelative(req_offset)
        requests_offset = builder.EndVector()

        # Build BatchRequest
        BatchRequest.BatchRequestStart(builder)
        BatchRequest.BatchRequestAddBatchId(builder, batch_idx)
        BatchRequest.BatchRequestAddRequests(builder, requests_offset)
        batch_offset = BatchRequest.BatchRequestEnd(builder)

        builder.Finish(batch_offset)
        request_bytes = bytes(builder.Output())

        # Send to Go
        response = simulate_batch(request_bytes)
        result = response.Results(0)

        total_games += result.TotalGames()

    end_time = time.perf_counter()
    total_duration = end_time - start_time

    return {
        "total_games": total_games,
        "total_duration_s": total_duration,
        "avg_ms_per_game": (total_duration * 1000) / total_games,
        "games_per_second": total_games / total_duration,
    }


def benchmark_python_war_stub(num_games: int = 1000) -> dict:
    """Stub for Python implementation benchmark.

    NOTE: This requires implementing a Python simulation engine,
    which is not part of Phase 3. For now, we use the Phase 1
    benchmark numbers.
    """
    # From Phase 1 benchmarks: Python War ~0.07ms per game
    python_ms_per_game = 0.07

    total_duration_s = (num_games * python_ms_per_game) / 1000

    return {
        "total_games": num_games,
        "total_duration_s": total_duration_s,
        "avg_ms_per_game": python_ms_per_game,
        "games_per_second": num_games / total_duration_s,
        "note": "Using Phase 1 benchmark numbers (stub)",
    }


def main():
    """Run benchmarks and report results."""
    print("=" * 60)
    print("GOLANG PERFORMANCE CORE BENCHMARK")
    print("=" * 60)
    print()

    # Warm-up run
    print("Warming up Go environment...")
    benchmark_go_war(num_games=100, batch_size=50)
    print()

    # Main benchmark
    num_games = 2000
    go_results = benchmark_go_war(num_games=num_games, batch_size=100)

    print()
    print("=" * 60)
    print("RESULTS")
    print("=" * 60)
    print()

    print(f"Golang (War game, {num_games} simulations):")
    print(f"  Total duration: {go_results['total_duration_s']:.3f}s")
    print(f"  Avg per game:   {go_results['avg_ms_per_game']:.4f}ms")
    print(f"  Throughput:     {go_results['games_per_second']:.0f} games/sec")
    print()

    # Compare with Python baseline
    python_results = benchmark_python_war_stub(num_games=num_games)
    print(f"Python baseline (War game, estimated):")
    print(f"  Avg per game:   {python_results['avg_ms_per_game']:.4f}ms")
    print(f"  Throughput:     {python_results['games_per_second']:.0f} games/sec")
    print(f"  Note: {python_results['note']}")
    print()

    # Calculate speedup
    speedup = python_results["avg_ms_per_game"] / go_results["avg_ms_per_game"]

    print("=" * 60)
    print(f"SPEEDUP: {speedup:.1f}x")
    print("=" * 60)
    print()

    target_met = 10 <= speedup <= 50
    if target_met:
        print(f"✅ Target met! Go is {speedup:.1f}x faster than Python.")
    else:
        if speedup < 10:
            print(f"⚠️  Below target: {speedup:.1f}x < 10x")
        else:
            print(f"🎉 Exceeded target: {speedup:.1f}x > 50x!")

    print()
    print("Notes:")
    print("- War is a simple game (pure luck, minimal logic)")
    print("- Speedup will be higher for complex games with MCTS")
    print("- Memory pooling eliminates GC pressure at scale")
    print()


if __name__ == "__main__":
    main()
