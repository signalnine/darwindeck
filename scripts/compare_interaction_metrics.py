#!/usr/bin/env python3
"""Compare old vs new interaction_frequency on seed games."""
import json
from pathlib import Path
from darwindeck.genome.examples import create_war_genome, create_hearts_genome, create_crazy_eights_genome
from darwindeck.simulation.go_simulator import GoSimulator
from darwindeck.evolution.fitness_full import FitnessEvaluator

def main():
    simulator = GoSimulator(seed=42)
    evaluator = FitnessEvaluator(style='balanced')

    # Use example genomes since seed JSON files may not exist
    genomes = {
        'War': create_war_genome(),
        'Hearts': create_hearts_genome(),
        'Crazy8s': create_crazy_eights_genome(),
    }

    print("Game | Move Disruption | Contention | Forced Response | Total Interaction")
    print("-" * 80)

    for name, genome in genomes.items():
        try:
            results = simulator.simulate(genome, num_games=100)

            # Calculate component metrics
            if results.opponent_turn_count > 0:
                disruption = results.move_disruption_events / results.opponent_turn_count
                forced = results.forced_response_events / results.opponent_turn_count
            else:
                disruption = 0.0
                forced = 0.0

            if results.total_actions > 0:
                contention = results.contention_events / results.total_actions
            else:
                contention = 0.0

            # Get the final interaction_frequency from evaluator
            metrics = evaluator.evaluate(genome, results)
            interaction = metrics.interaction_frequency

            print(f"{name:12} | {disruption:15.3f} | {contention:10.3f} | {forced:15.3f} | {interaction:17.3f}")
        except Exception as e:
            print(f"{name:12} | ERROR: {e}")

if __name__ == "__main__":
    main()
