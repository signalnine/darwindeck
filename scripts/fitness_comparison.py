#!/usr/bin/env python3
"""Compare fitness scores for example games across different style presets."""

import sys
from dataclasses import dataclass
from typing import Dict, List, Tuple

from darwindeck.genome.examples import (
    create_war_genome,
    create_hearts_genome,
    create_crazy_eights_genome,
    create_gin_rummy_genome,
    create_old_maid_genome,
    create_go_fish_genome,
    create_betting_war_genome,
    create_cheat_genome,
    create_scopa_genome,
    create_draw_poker_genome,
    create_scotch_whist_genome,
    create_knockout_whist_genome,
    create_blackjack_genome,
    create_fan_tan_genome,
    create_president_genome,
    create_spades_genome,
    create_uno_genome,
    create_simple_poker_genome,
)
from darwindeck.simulation.go_simulator import GoSimulator
from darwindeck.evolution.fitness_full import FitnessEvaluator, STYLE_PRESETS, SimulationResults


# All available games
GAMES = {
    'war': create_war_genome,
    'hearts': create_hearts_genome,
    'crazy_eights': create_crazy_eights_genome,
    'gin_rummy': create_gin_rummy_genome,
    'old_maid': create_old_maid_genome,
    'go_fish': create_go_fish_genome,
    'betting_war': create_betting_war_genome,
    'cheat': create_cheat_genome,
    'scopa': create_scopa_genome,
    'draw_poker': create_draw_poker_genome,
    'scotch_whist': create_scotch_whist_genome,
    'knockout_whist': create_knockout_whist_genome,
    'blackjack': create_blackjack_genome,
    'fan_tan': create_fan_tan_genome,
    'president': create_president_genome,
    'spades': create_spades_genome,
    'uno': create_uno_genome,
    'simple_poker': create_simple_poker_genome,
}


def run_simulations(num_games: int = 500, seed: int = 42) -> Dict[str, Tuple[SimulationResults, any]]:
    """Run simulations for all games and return results."""
    sim = GoSimulator(seed=seed)
    results = {}

    print(f"Running {num_games} simulations per game...\n")

    for name, create_fn in GAMES.items():
        try:
            genome = create_fn()
            sim_result = sim.simulate(genome, num_games=num_games)
            results[name] = (sim_result, genome)

            # Show progress
            wins = sum(sim_result.wins)
            status = "OK" if sim_result.errors == 0 else f"ERR:{sim_result.errors}"
            print(f"  {name:20s} wins={wins:3d} draws={sim_result.draws:3d} [{status}]")
        except Exception as e:
            print(f"  {name:20s} FAILED: {e}")
            results[name] = None

    print()
    return results


def evaluate_fitness(results: Dict, style: str) -> Dict[str, float]:
    """Evaluate fitness for all games with a given style."""
    evaluator = FitnessEvaluator(style=style)
    fitness_scores = {}

    for name, data in results.items():
        if data is None:
            fitness_scores[name] = None
            continue

        sim_result, genome = data
        if sim_result.errors > 0:
            fitness_scores[name] = None
            continue

        try:
            metrics = evaluator.evaluate(genome, sim_result)
            fitness_scores[name] = metrics.total_fitness
        except Exception as e:
            fitness_scores[name] = None

    return fitness_scores


def print_table(all_fitness: Dict[str, Dict[str, float]], results: Dict):
    """Print a formatted comparison table."""
    styles = list(STYLE_PRESETS.keys())

    # Header
    print("=" * 100)
    print(f"{'Game':<20s}", end="")
    for style in styles:
        print(f"{style:>14s}", end="")
    print(f"{'wins':>10s}{'draws':>8s}")
    print("=" * 100)

    # Sort games by balanced fitness
    sorted_games = sorted(
        GAMES.keys(),
        key=lambda g: all_fitness['balanced'].get(g) or 0,
        reverse=True
    )

    for game in sorted_games:
        print(f"{game:<20s}", end="")

        for style in styles:
            score = all_fitness[style].get(game)
            if score is None:
                print(f"{'--':>14s}", end="")
            else:
                print(f"{score:>14.4f}", end="")

        # Add win/draw info
        if results.get(game):
            sim_result, _ = results[game]
            wins = sum(sim_result.wins)
            print(f"{wins:>10d}{sim_result.draws:>8d}", end="")

        print()

    print("=" * 100)


def print_detailed_metrics(results: Dict, style: str = 'balanced'):
    """Print detailed metrics for each game."""
    evaluator = FitnessEvaluator(style=style)

    print(f"\nDetailed Metrics ({style} style)")
    print("=" * 130)
    print(f"{'Game':<18s}{'fitness':>9s}{'decision':>10s}{'comeback':>10s}{'tension':>10s}{'interact':>10s}{'complex':>10s}{'skill':>10s}{'betting':>10s}")
    print("-" * 130)

    sorted_games = []
    for name, data in results.items():
        if data is None:
            continue
        sim_result, genome = data
        if sim_result.errors > 0:
            continue
        try:
            metrics = evaluator.evaluate(genome, sim_result)
            sorted_games.append((name, metrics))
        except:
            pass

    # Sort by total fitness
    sorted_games.sort(key=lambda x: x[1].total_fitness, reverse=True)

    for name, m in sorted_games:
        print(f"{name:<18s}{m.total_fitness:>9.4f}{m.decision_density:>10.4f}{m.comeback_potential:>10.4f}"
              f"{m.tension_curve:>10.4f}{m.interaction_frequency:>10.4f}{m.rules_complexity:>10.4f}"
              f"{m.skill_vs_luck:>10.4f}{m.betting_engagement:>10.4f}")

    print("=" * 130)


def main():
    import argparse

    parser = argparse.ArgumentParser(description='Compare fitness scores for example games')
    parser.add_argument('--games', '-g', type=int, default=500,
                        help='Number of games to simulate per genome (default: 500)')
    parser.add_argument('--seed', '-s', type=int, default=42,
                        help='Random seed (default: 42)')
    parser.add_argument('--style', type=str, default=None,
                        help='Show detailed metrics for a specific style')
    parser.add_argument('--detailed', '-d', action='store_true',
                        help='Show detailed metrics for balanced style')

    args = parser.parse_args()

    # Run simulations
    results = run_simulations(num_games=args.games, seed=args.seed)

    # Evaluate fitness for each style
    all_fitness = {}
    for style in STYLE_PRESETS.keys():
        all_fitness[style] = evaluate_fitness(results, style)

    # Print comparison table
    print_table(all_fitness, results)

    # Print detailed metrics if requested
    if args.detailed or args.style:
        style = args.style or 'balanced'
        print_detailed_metrics(results, style)


if __name__ == '__main__':
    main()
