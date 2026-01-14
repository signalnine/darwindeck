# src/darwindeck/cli/rulebook.py
"""CLI command for generating rulebooks."""

from __future__ import annotations

import json
import logging
import sys
from pathlib import Path

import click

from darwindeck.evolution.rulebook import RulebookGenerator
from darwindeck.genome.serialization import genome_from_dict

logger = logging.getLogger(__name__)


@click.command()
@click.argument("genome_path", type=click.Path(exists=True))
@click.option("-o", "--output", type=click.Path(), help="Output file path")
@click.option("--basic", is_flag=True, help="Skip LLM enhancement")
@click.option("--top", type=int, default=None, help="Only process top N genomes (if directory)")
@click.option("-v", "--verbose", is_flag=True, help="Verbose output")
def main(genome_path: str, output: str | None, basic: bool, top: int | None, verbose: bool):
    """Generate a rulebook from a game genome.

    GENOME_PATH can be a single JSON file or a directory containing genome files.
    """
    if verbose:
        logging.basicConfig(level=logging.DEBUG)
    else:
        logging.basicConfig(level=logging.INFO)

    path = Path(genome_path)
    generator = RulebookGenerator()

    if path.is_file():
        # Single genome
        _process_genome(path, output, basic, generator)
    elif path.is_dir():
        # Directory of genomes
        genome_files = sorted(path.glob("rank*.json"))
        if top:
            genome_files = genome_files[:top]

        if not genome_files:
            click.echo(f"No genome files found in {path}", err=True)
            sys.exit(1)

        output_dir = Path(output) if output else path / "rulebooks"
        output_dir.mkdir(exist_ok=True)

        for gf in genome_files:
            out_path = output_dir / f"{gf.stem}_rulebook.md"
            _process_genome(gf, str(out_path), basic, generator)
    else:
        click.echo(f"Invalid path: {genome_path}", err=True)
        sys.exit(1)


def _process_genome(genome_path: Path, output: str | None, basic: bool, generator: RulebookGenerator):
    """Process a single genome file."""
    click.echo(f"Processing {genome_path.name}...")

    try:
        with open(genome_path) as f:
            data = json.load(f)

        genome = genome_from_dict(data)
        markdown = generator.generate(genome, use_llm=not basic)

        if output:
            out_path = Path(output)
        else:
            out_path = genome_path.with_suffix(".md").with_stem(f"{genome_path.stem}_rulebook")

        out_path.write_text(markdown)
        click.echo(f"  Saved to {out_path}")

    except Exception as e:
        click.echo(f"  Error: {e}", err=True)


if __name__ == "__main__":
    main()
