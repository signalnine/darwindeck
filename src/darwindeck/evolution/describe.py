"""LLM-powered game description generator."""

from __future__ import annotations

import logging
import os
from typing import Optional

from darwindeck.genome.schema import GameGenome
from darwindeck.genome.serialization import genome_to_json

logger = logging.getLogger(__name__)

# Suppress verbose HTTP logging from anthropic SDK
logging.getLogger("httpx").setLevel(logging.WARNING)
logging.getLogger("httpcore").setLevel(logging.WARNING)
logging.getLogger("anthropic").setLevel(logging.WARNING)


def describe_game(genome: GameGenome, fitness: float) -> Optional[str]:
    """Generate a human-readable description of a game using an LLM.

    Args:
        genome: The game genome to describe
        fitness: The fitness score of the genome

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

        prompt = f"""Analyze this evolved card game genome and provide a brief, engaging description.

Game Name: {genome.genome_id}
Fitness Score: {fitness:.4f}
Player Count: {genome.player_count}

Genome JSON:
{genome_json}

Write a 2-3 sentence description that:
1. Summarizes the core gameplay mechanic (what players do on their turn)
2. Explains the win condition
3. Notes any interesting or unique aspects

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
    top_n: int = 5
) -> dict[str, str]:
    """Generate descriptions for top N games.

    Args:
        genomes_with_fitness: List of (genome, fitness) tuples
        top_n: Number of top games to describe

    Returns:
        Dict mapping genome_id to description
    """
    descriptions = {}

    for genome, fitness in genomes_with_fitness[:top_n]:
        logger.info(f"  Generating description for {genome.genome_id}...")
        desc = describe_game(genome, fitness)
        if desc:
            descriptions[genome.genome_id] = desc

    return descriptions
