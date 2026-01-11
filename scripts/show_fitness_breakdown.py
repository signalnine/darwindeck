#!/usr/bin/env python3
"""Show fitness breakdown for all example games."""

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
)
from darwindeck.evolution.fitness_full import (
    FitnessEvaluator,
    SimulationResults,
    STYLE_PRESETS,
)
from darwindeck.simulation.go_simulator import GoSimulator


def simulate_game(genome, num_games: int = 100) -> SimulationResults:
    """Run simulations and return results."""
    simulator = GoSimulator()
    return simulator.simulate(genome, num_games=num_games)


def print_fitness_breakdown(name: str, genome, results: SimulationResults) -> None:
    """Print detailed fitness breakdown for a game."""
    evaluator = FitnessEvaluator(style='balanced')
    metrics = evaluator.evaluate(genome, results)

    print(f"\n{'='*60}")
    print(f"  {name}")
    print(f"{'='*60}")

    # Simulation stats
    print(f"\n  Simulation Results ({results.total_games} games):")
    print(f"    P0 wins: {results.player0_wins} ({100*results.player0_wins/results.total_games:.1f}%)")
    print(f"    P1 wins: {results.player1_wins} ({100*results.player1_wins/results.total_games:.1f}%)")
    print(f"    Draws:   {results.draws} ({100*results.draws/results.total_games:.1f}%)")
    print(f"    Avg turns: {results.avg_turns:.1f}")
    print(f"    Errors: {results.errors}")

    # Fitness metrics
    print(f"\n  Fitness Metrics (0.0 - 1.0 scale):")
    print(f"    Decision Density:      {metrics.decision_density:.3f}  (choices per decision)")
    print(f"    Comeback Potential:    {metrics.comeback_potential:.3f}  (game balance)")
    print(f"    Tension Curve:         {metrics.tension_curve:.3f}  (suspense over time)")
    print(f"    Interaction Frequency: {metrics.interaction_frequency:.3f}  (player interaction)")
    print(f"    Rules Complexity:      {metrics.rules_complexity:.3f}  (simplicity score)")
    print(f"    Skill vs Luck:         {metrics.skill_vs_luck:.3f}  (skill influence)")
    print(f"    Session Length:        {metrics.session_length:.3f}  (constraint, not weighted)")

    print(f"\n  Total Fitness (weighted): {metrics.total_fitness:.3f}")
    print(f"  Valid: {metrics.valid}")


def main():
    print("\n" + "="*60)
    print("  FITNESS BREAKDOWN FOR EXAMPLE GAMES")
    print("  Using 'balanced' style weights")
    print("="*60)

    # Show weight configuration
    weights = STYLE_PRESETS['balanced']
    print("\n  Balanced Style Weights:")
    for metric, weight in sorted(weights.items(), key=lambda x: -x[1]):
        print(f"    {metric}: {weight:.0%}")

    # List of games to evaluate
    games = [
        ("War", create_war_genome),
        ("Hearts", create_hearts_genome),
        ("Crazy Eights", create_crazy_eights_genome),
        ("Gin Rummy", create_gin_rummy_genome),
        ("Old Maid", create_old_maid_genome),
        ("Go Fish", create_go_fish_genome),
        ("Betting War", create_betting_war_genome),
        ("Cheat (BS)", create_cheat_genome),
        ("Scopa", create_scopa_genome),
        ("Draw Poker", create_draw_poker_genome),
        ("Scotch Whist", create_scotch_whist_genome),
    ]

    all_metrics = []

    for name, creator in games:
        try:
            genome = creator()
            results = simulate_game(genome, num_games=100)
            print_fitness_breakdown(name, genome, results)

            evaluator = FitnessEvaluator(style='balanced')
            metrics = evaluator.evaluate(genome, results)
            all_metrics.append((name, metrics))
        except Exception as e:
            print(f"\n  {name}: ERROR - {e}")

    # Summary table
    print("\n" + "="*60)
    print("  SUMMARY TABLE")
    print("="*60)
    print(f"\n  {'Game':<15} {'Fitness':>8} {'Decision':>9} {'Comeback':>9} {'Interact':>9} {'Skill':>7}")
    print(f"  {'-'*15} {'-'*8} {'-'*9} {'-'*9} {'-'*9} {'-'*7}")

    for name, m in sorted(all_metrics, key=lambda x: -x[1].total_fitness):
        print(f"  {name:<15} {m.total_fitness:>8.3f} {m.decision_density:>9.3f} "
              f"{m.comeback_potential:>9.3f} {m.interaction_frequency:>9.3f} {m.skill_vs_luck:>7.3f}")

    print()


if __name__ == "__main__":
    main()
