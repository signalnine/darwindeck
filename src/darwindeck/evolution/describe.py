"""LLM-powered game description generator."""

from __future__ import annotations

import logging
import os
from typing import Optional

from darwindeck.genome.schema import GameGenome
from darwindeck.genome.serialization import genome_to_json
from darwindeck.evolution.skill_evaluation import SkillEvalResult

logger = logging.getLogger(__name__)

# Suppress verbose HTTP logging from anthropic SDK
logging.getLogger("httpx").setLevel(logging.WARNING)
logging.getLogger("httpcore").setLevel(logging.WARNING)
logging.getLogger("anthropic").setLevel(logging.WARNING)


def describe_game(
    genome: GameGenome,
    fitness: float,
    skill: Optional[SkillEvalResult] = None
) -> Optional[str]:
    """Generate a human-readable description of a game using an LLM.

    Args:
        genome: The game genome to describe
        fitness: The fitness score of the genome
        skill: Optional skill evaluation result with MCTS win rate

    Returns:
        A short description of the game, or None if generation fails
    """
    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        logger.warning("ANTHROPIC_API_KEY not set, skipping game descriptions")
        return None

    try:
        import anthropic
        client = anthropic.Anthropic(api_key=api_key)

        # Convert genome to JSON for the prompt
        genome_json = genome_to_json(genome)

        # Build skill info section if available
        skill_info = ""
        if skill:
            skill_info = f"""
Skill Evaluation:
- Greedy vs Random: {skill.greedy_win_rate:.1%}
- MCTS vs Random: {skill.mcts_win_rate:.1%}
- Combined Skill Score: {skill.skill_score:.2f}
- First Player Advantage: {skill.first_player_advantage:+.1%}
"""

        prompt = f"""Analyze this evolved card game genome and provide a brief, engaging description.

Game Name: {genome.genome_id}
Fitness Score: {fitness:.4f}
Player Count: {genome.player_count}
{skill_info}
Genome JSON:
{genome_json}

Write a 2-3 sentence description that:
1. Summarizes the core gameplay mechanic (what players do on their turn)
2. Explains the win condition
3. Notes any interesting or unique aspects
4. If skill evaluation data is provided, briefly mention whether strategy matters (high MCTS win rate = more skill-based)

Keep it concise and accessible to someone unfamiliar with the technical genome format.
Do not include the game name in your response - just the description."""

        message = client.messages.create(
            model="claude-sonnet-4-20250514",
            max_tokens=200,
            messages=[
                {"role": "user", "content": prompt}
            ]
        )

        return message.content[0].text.strip()

    except Exception as e:
        logger.warning(f"Failed to generate description for {genome.genome_id}: {e}")
        return None


def describe_top_games(
    genomes_with_fitness: list[tuple[GameGenome, float]],
    top_n: int = 5,
    skill_results: Optional[dict[str, SkillEvalResult]] = None
) -> dict[str, str]:
    """Generate descriptions for top N games.

    Args:
        genomes_with_fitness: List of (genome, fitness) tuples
        top_n: Number of top games to describe
        skill_results: Optional dict mapping genome_id to SkillEvalResult

    Returns:
        Dict mapping genome_id to description
    """
    descriptions = {}
    skill_results = skill_results or {}

    for genome, fitness in genomes_with_fitness[:top_n]:
        logger.info(f"  Generating description for {genome.genome_id}...")
        skill = skill_results.get(genome.genome_id)
        desc = describe_game(genome, fitness, skill)
        if desc:
            descriptions[genome.genome_id] = desc

    return descriptions
