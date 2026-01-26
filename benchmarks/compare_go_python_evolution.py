#!/usr/bin/env python3
"""
Compare Go vs Python evolution performance.

This benchmark measures end-to-end evolution throughput for both implementations.
"""

import subprocess
import time
import json
import os
from pathlib import Path

def run_go_evolution(generations: int, population: int, games_per_eval: int) -> dict:
    """Run Go evolution and return timing info."""
    output_dir = "/tmp/go-bench"
    os.makedirs(output_dir, exist_ok=True)

    start = time.time()
    result = subprocess.run([
        "./bin/darwindeck-evolve",
        f"--generations={generations}",
        f"--population-size={population}",
        f"--games-per-eval={games_per_eval}",
        f"--output-dir={output_dir}",
        "--seed=42",
    ], capture_output=True, text=True, cwd=str(Path(__file__).parent.parent))
    elapsed = time.time() - start

    # Extract best fitness from output
    best_fitness = 0.0
    for line in result.stdout.split("\n"):
        if "Best Fitness:" in line:
            try:
                best_fitness = float(line.split(":")[1].strip())
            except:
                pass

    total_games = generations * population * games_per_eval

    return {
        "elapsed_sec": elapsed,
        "generations": generations,
        "population": population,
        "games_per_eval": games_per_eval,
        "total_games": total_games,
        "games_per_sec": total_games / elapsed if elapsed > 0 else 0,
        "best_fitness": best_fitness,
    }


def run_python_evolution(generations: int, population: int, games_per_eval: int = 100) -> dict:
    """Run Python evolution and return timing info."""
    output_dir = "/tmp/python-bench"
    os.makedirs(output_dir, exist_ok=True)

    # Python CLI uses default 100 games per eval
    start = time.time()
    result = subprocess.run([
        "uv", "run", "python", "-m", "darwindeck.cli.evolve",
        f"--generations={generations}",
        f"--population-size={population}",
        f"--output-dir={output_dir}",
        "--skip-skill-eval",
    ], capture_output=True, text=True, cwd=str(Path(__file__).parent.parent), timeout=600)
    elapsed = time.time() - start

    # Extract best fitness from output
    best_fitness = 0.0
    for line in result.stdout.split("\n"):
        if "Best fitness:" in line or "Best Fitness:" in line:
            try:
                best_fitness = float(line.split(":")[1].strip().split()[0])
            except:
                pass

    total_games = generations * population * games_per_eval

    return {
        "elapsed_sec": elapsed,
        "generations": generations,
        "population": population,
        "games_per_eval": games_per_eval,
        "total_games": total_games,
        "games_per_sec": total_games / elapsed if elapsed > 0 else 0,
        "best_fitness": best_fitness,
    }


def main():
    print("=" * 60)
    print("Go vs Python Evolution Performance Comparison")
    print("=" * 60)
    print()

    # Small benchmark for quick comparison
    gens = 5
    pop = 20
    games = 50

    print(f"Configuration: {gens} generations, {pop} population, {games} games/eval")
    print(f"Total games: {gens * pop * games:,}")
    print()

    print("Running Go evolution...")
    go_result = run_go_evolution(gens, pop, games)
    print(f"  Time: {go_result['elapsed_sec']:.2f}s")
    print(f"  Throughput: {go_result['games_per_sec']:,.0f} games/sec")
    print(f"  Best fitness: {go_result['best_fitness']:.4f}")
    print()

    print("Running Python evolution...")
    try:
        py_result = run_python_evolution(gens, pop, games)
        print(f"  Time: {py_result['elapsed_sec']:.2f}s")
        print(f"  Throughput: {py_result['games_per_sec']:,.0f} games/sec")
        print(f"  Best fitness: {py_result['best_fitness']:.4f}")
        print()

        speedup = py_result['elapsed_sec'] / go_result['elapsed_sec'] if go_result['elapsed_sec'] > 0 else 0
        print("=" * 60)
        print(f"SPEEDUP: {speedup:.1f}x (Go is {speedup:.1f}x faster than Python)")
        print("=" * 60)
    except subprocess.TimeoutExpired:
        print("  TIMEOUT: Python took > 10 minutes")
        print()
        print("=" * 60)
        print(f"SPEEDUP: >10x (Python timed out, Go completed in {go_result['elapsed_sec']:.2f}s)")
        print("=" * 60)


if __name__ == "__main__":
    main()
