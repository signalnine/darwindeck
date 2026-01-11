#!/usr/bin/env python3
"""CLI for generating LLM game descriptions from saved genomes."""

import argparse
import json
import logging
import sys
from pathlib import Path

from darwindeck.genome.serialization import genome_from_json
from darwindeck.evolution.describe import describe_game
from darwindeck.evolution.skill_evaluation import SkillEvalResult

logging.basicConfig(level=logging.INFO, format='%(message)s')
logger = logging.getLogger(__name__)


def main():
    parser = argparse.ArgumentParser(
        description='Generate LLM game description from saved genome JSON'
    )
    parser.add_argument(
        'genome_file',
        type=str,
        help='Path to genome JSON file (e.g., output/run1/rank01_GameName.json)'
    )
    parser.add_argument(
        '--fitness',
        type=float,
        default=None,
        help='Fitness score (extracted from filename if not provided)'
    )
    parser.add_argument(
        '--skill-file',
        type=str,
        default=None,
        help='Path to skill evaluation JSON (optional)'
    )
    parser.add_argument(
        '--verbose', '-v',
        action='store_true',
        help='Show detailed output'
    )

    args = parser.parse_args()

    # Load genome
    genome_path = Path(args.genome_file)
    if not genome_path.exists():
        logger.error(f"File not found: {genome_path}")
        sys.exit(1)

    try:
        with open(genome_path) as f:
            genome_data = json.load(f)
    except json.JSONDecodeError as e:
        logger.error(f"Invalid JSON: {e}")
        sys.exit(1)

    # Extract fitness from file if present, otherwise use provided or default
    fitness = args.fitness
    if fitness is None:
        # Try to extract from genome data
        if 'fitness' in genome_data:
            fitness = genome_data['fitness']
        elif 'total_fitness' in genome_data:
            fitness = genome_data['total_fitness']
        else:
            fitness = 0.5  # Default
            if args.verbose:
                logger.info("No fitness score found, using default 0.5")

    # Parse genome (handle both wrapped and unwrapped formats)
    if 'genome' in genome_data:
        genome_json = json.dumps(genome_data['genome'])
    else:
        genome_json = json.dumps(genome_data)

    try:
        genome = genome_from_json(genome_json)
    except Exception as e:
        logger.error(f"Failed to parse genome: {e}")
        sys.exit(1)

    # Load skill evaluation if provided
    skill = None
    if args.skill_file:
        try:
            with open(args.skill_file) as f:
                skill_data = json.load(f)
            skill = SkillEvalResult(
                genome_id=genome.genome_id,
                greedy_wins_as_p0=skill_data.get('greedy_wins_as_p0', 0),
                greedy_wins_as_p1=skill_data.get('greedy_wins_as_p1', 0),
                greedy_win_rate=skill_data.get('greedy_win_rate', 0.5),
                mcts_wins_as_p0=skill_data.get('mcts_wins_as_p0', 0),
                mcts_wins_as_p1=skill_data.get('mcts_wins_as_p1', 0),
                mcts_win_rate=skill_data.get('mcts_win_rate', 0.5),
                total_games=skill_data.get('total_games', 0),
                skill_score=skill_data.get('skill_score', 0.5),
                first_player_advantage=skill_data.get('first_player_advantage', 0.0),
            )
        except Exception as e:
            logger.warning(f"Failed to load skill file: {e}")

    # Also check if skill data is embedded in genome file
    if skill is None and 'skill_evaluation' in genome_data:
        skill_data = genome_data['skill_evaluation']
        skill = SkillEvalResult(
            genome_id=genome.genome_id,
            greedy_wins_as_p0=skill_data.get('greedy_wins_as_p0', 0),
            greedy_wins_as_p1=skill_data.get('greedy_wins_as_p1', 0),
            greedy_win_rate=skill_data.get('greedy_win_rate', 0.5),
            mcts_wins_as_p0=skill_data.get('mcts_wins_as_p0', 0),
            mcts_wins_as_p1=skill_data.get('mcts_wins_as_p1', 0),
            mcts_win_rate=skill_data.get('mcts_win_rate', 0.5),
            total_games=skill_data.get('total_games', 0),
            skill_score=skill_data.get('skill_score', 0.5),
            first_player_advantage=skill_data.get('first_player_advantage', 0.0),
        )

    if args.verbose:
        logger.info(f"Game: {genome.genome_id}")
        logger.info(f"Fitness: {fitness:.4f}")
        logger.info(f"Players: {genome.player_count}")
        if skill:
            logger.info(f"Greedy win rate: {skill.greedy_win_rate:.1%}")
            logger.info(f"MCTS win rate: {skill.mcts_win_rate:.1%}")
        logger.info("---")

    # Generate description
    description = describe_game(genome, fitness, skill)

    if description:
        print(description)
    else:
        logger.error("Failed to generate description. Is ANTHROPIC_API_KEY set?")
        sys.exit(1)


if __name__ == '__main__':
    main()
