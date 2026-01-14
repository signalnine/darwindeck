"""CLI command for human playtesting."""

from __future__ import annotations

import json
import logging
import sys
from pathlib import Path

import click

from darwindeck.playtest.session import PlaytestSession, SessionConfig
from darwindeck.playtest.picker import GenomePicker
from darwindeck.playtest.feedback import FeedbackCollector
from darwindeck.genome.serialization import genome_from_dict

logger = logging.getLogger(__name__)


@click.command()
@click.argument("genome_path", type=click.Path(exists=True), required=False)
@click.option(
    "-d", "--difficulty",
    type=click.Choice(["random", "greedy", "mcts"]),
    default=None,
    help="AI difficulty (prompts if not specified)",
)
@click.option("--debug", is_flag=True, help="Show AI's hand and full game state")
@click.option(
    "--results",
    type=click.Path(),
    default="playtest_results.jsonl",
    help="Where to save playtest results",
)
@click.option("--seed", type=int, default=None, help="Random seed for reproducibility")
@click.option("--max-turns", type=int, default=200, help="Turn limit before forced end")
@click.option("--show-rules/--no-rules", default=True, help="Display rules at start")
@click.option("-v", "--verbose", is_flag=True, help="Verbose logging")
def main(
    genome_path: str | None,
    difficulty: str | None,
    debug: bool,
    results: str,
    seed: int | None,
    max_turns: int,
    show_rules: bool,
    verbose: bool,
):
    """Play an evolved card game against an AI opponent.

    GENOME_PATH is optional - shows interactive picker if omitted.
    """
    if verbose:
        logging.basicConfig(level=logging.DEBUG)
    else:
        logging.basicConfig(level=logging.INFO)

    # Load genome
    if genome_path:
        path = Path(genome_path)
        with open(path) as f:
            data = json.load(f)
        genome = genome_from_dict(data)
        genome_path_str = str(path)
    else:
        picker = GenomePicker()
        result = picker.interactive_pick()
        if result is None:
            click.echo("No genome selected. Exiting.")
            sys.exit(0)
        genome, path = result
        genome_path_str = str(path)

    # Get difficulty if not specified
    if difficulty is None:
        click.echo("\nChoose AI difficulty:")
        click.echo("  [1] Random (easy)")
        click.echo("  [2] Greedy (medium)")
        click.echo("  [3] MCTS (hard)")
        try:
            choice = input("\nSelect [1-3]: ").strip()
            difficulty = {"1": "random", "2": "greedy", "3": "mcts"}.get(choice, "greedy")
        except (EOFError, KeyboardInterrupt):
            sys.exit(0)

    # Create config
    config = SessionConfig(
        difficulty=difficulty,
        debug=debug,
        max_turns=max_turns,
        seed=seed,
        show_rules=show_rules,
        results_path=Path(results),
    )

    # Run session
    click.echo(f"\nStarting {genome.genome_id}...")
    click.echo(f"Difficulty: {difficulty}")
    click.echo("")

    session = PlaytestSession(genome, config)

    try:
        result = session.run(output_fn=click.echo)
    except KeyboardInterrupt:
        click.echo("\n\nGame interrupted.")
        result = None

    # Save result
    if result:
        result.genome_path = genome_path_str
        collector = FeedbackCollector(config.results_path)
        collector.save(result)
        click.echo(f"\nResult saved to {config.results_path}")

    click.echo("\nThanks for playtesting!")


if __name__ == "__main__":
    main()
