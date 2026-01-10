"""Benchmark Python genome-based simulation."""

import time
from cards_evolve.genome.examples import create_war_genome
from cards_evolve.simulation.engine import GameEngine


def benchmark_python_war(num_games: int = 100) -> dict:
    """Benchmark Python genome-based War simulation."""
    genome = create_war_genome()
    engine = GameEngine()

    start_time = time.perf_counter()

    results = []
    for seed in range(num_games):
        result = engine.simulate_game(genome, [], seed=seed)
        results.append(result)

    end_time = time.perf_counter()
    total_duration_s = end_time - start_time

    # Calculate statistics
    avg_turns = sum(r.turn_count for r in results) / len(results)
    player0_wins = sum(1 for r in results if r.winner == 0)
    player1_wins = sum(1 for r in results if r.winner == 1)

    return {
        "total_games": num_games,
        "total_duration_s": total_duration_s,
        "avg_ms_per_game": (total_duration_s * 1000) / num_games,
        "games_per_second": num_games / total_duration_s,
        "avg_turns": avg_turns,
        "player0_wins": player0_wins,
        "player1_wins": player1_wins,
    }


def main():
    """Run Python genome-based benchmark."""
    print("=" * 60)
    print("PYTHON GENOME-BASED BENCHMARK")
    print("=" * 60)
    print()

    # Warm-up run
    print("Warming up Python environment...")
    benchmark_python_war(num_games=10)
    print()

    # Main benchmark
    num_games = 100
    print(f"Running {num_games} War games (genome-based)...")
    py_results = benchmark_python_war(num_games=num_games)

    print()
    print("=" * 60)
    print("RESULTS")
    print("=" * 60)
    print()
    print(f"Python (War game, genome-based, {num_games} simulations):")
    print(f"  Total duration: {py_results['total_duration_s']:.3f}s")
    print(f"  Avg per game:   {py_results['avg_ms_per_game']:.4f}ms")
    print(f"  Throughput:     {py_results['games_per_second']:.0f} games/sec")
    print(f"  Avg turns:      {py_results['avg_turns']:.1f}")
    print(f"  P0 wins: {py_results['player0_wins']}, P1 wins: {py_results['player1_wins']}")


if __name__ == "__main__":
    main()
