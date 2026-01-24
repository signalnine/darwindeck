"""CLI commands for DarwinDeck web UI management."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Optional

import click

from darwindeck.genome.serialization import genome_from_dict, genome_to_json
from darwindeck.genome.validator import GenomeValidator


@click.group()
def cli():
    """DarwinDeck Web UI management commands."""
    pass


@cli.command()
@click.option(
    "--host",
    default="127.0.0.1",
    help="Host to bind to",
)
@click.option(
    "--port",
    default=8000,
    type=int,
    help="Port to bind to",
)
@click.option(
    "--reload",
    is_flag=True,
    help="Enable auto-reload for development",
)
def serve(host: str, port: int, reload: bool) -> None:
    """Start the web server."""
    import uvicorn

    click.echo(f"Starting DarwinDeck web server on http://{host}:{port}")
    uvicorn.run(
        "darwindeck.web.app:create_app",
        host=host,
        port=port,
        reload=reload,
        factory=True,
    )


@cli.command("import")
@click.argument("path", type=click.Path(exists=True, path_type=Path))
@click.option(
    "--db",
    default="data/playtest.db",
    help="Database path",
)
def import_genome(path: Path, db: str) -> None:
    """Import a genome JSON file into the database.

    PATH is the path to a genome JSON file.
    """
    from darwindeck.web.db import get_session, init_db, get_engine
    from darwindeck.web.models import Game

    # Load JSON
    try:
        with open(path) as f:
            data = json.load(f)
    except json.JSONDecodeError as e:
        click.echo(f"Error: Invalid JSON file: {e}", err=True)
        sys.exit(1)
    except IOError as e:
        click.echo(f"Error: Could not read file: {e}", err=True)
        sys.exit(1)

    # Parse and validate genome
    try:
        genome = genome_from_dict(data)
    except (KeyError, TypeError, ValueError) as e:
        click.echo(f"Error: Invalid genome structure: {e}", err=True)
        sys.exit(1)

    errors = GenomeValidator.validate(genome)
    if errors:
        click.echo(f"Warning: Validation errors: {errors}", err=True)
        # Continue anyway for import, just warn

    # Initialize database
    engine = get_engine(db)
    init_db(engine)
    session = get_session(db)

    try:
        game_id = genome.genome_id
        fitness = data.get("fitness")

        # Check if game already exists
        existing = session.query(Game).filter(Game.id == game_id).first()

        # Serialize genome to normalized JSON
        genome_json = genome_to_json(genome)

        if existing:
            existing.genome_json = genome_json
            if fitness is not None:
                existing.fitness = fitness
            click.echo(f"Updated: {game_id}")
        else:
            game = Game(
                id=game_id,
                genome_json=genome_json,
                fitness=fitness,
                status="active",
            )
            session.add(game)
            click.echo(f"Imported: {game_id}")

        session.commit()

        if fitness is not None:
            click.echo(f"  Fitness: {fitness:.4f}")
    except Exception as e:
        session.rollback()
        click.echo(f"Error: Failed to import genome: {e}", err=True)
        sys.exit(1)
    finally:
        session.close()


@cli.command()
@click.argument("directory", type=click.Path(exists=True, file_okay=False, path_type=Path))
@click.option(
    "--db",
    default="data/playtest.db",
    help="Database path",
)
@click.option(
    "--recursive",
    is_flag=True,
    help="Recursively search subdirectories",
)
def sync(directory: Path, db: str, recursive: bool) -> None:
    """Sync genomes from a directory.

    DIRECTORY is the path to a directory containing genome JSON files.
    """
    from darwindeck.web.db import get_session, init_db, get_engine
    from darwindeck.web.models import Game

    # Find JSON files
    if recursive:
        json_files = list(directory.rglob("*.json"))
    else:
        json_files = list(directory.glob("*.json"))

    if not json_files:
        click.echo(f"No JSON files found in {directory}")
        return

    click.echo(f"Found {len(json_files)} JSON files")

    # Initialize database
    engine = get_engine(db)
    init_db(engine)
    session = get_session(db)

    imported = 0
    updated = 0
    failed = 0
    errors = []

    try:
        for json_path in json_files:
            try:
                with open(json_path) as f:
                    data = json.load(f)
            except (json.JSONDecodeError, IOError) as e:
                failed += 1
                errors.append(f"{json_path.name}: {e}")
                continue

            # Parse genome
            try:
                genome = genome_from_dict(data)
            except (KeyError, TypeError, ValueError) as e:
                failed += 1
                errors.append(f"{json_path.name}: Invalid genome structure: {e}")
                continue

            # Import into database
            game_id = genome.genome_id
            fitness = data.get("fitness")

            existing = session.query(Game).filter(Game.id == game_id).first()
            genome_json = genome_to_json(genome)

            if existing:
                existing.genome_json = genome_json
                if fitness is not None:
                    existing.fitness = fitness
                updated += 1
            else:
                game = Game(
                    id=game_id,
                    genome_json=genome_json,
                    fitness=fitness,
                    status="active",
                )
                session.add(game)
                imported += 1

        session.commit()
    except Exception as e:
        session.rollback()
        click.echo(f"Error: Database error: {e}", err=True)
        sys.exit(1)
    finally:
        session.close()

    # Print summary
    click.echo(f"\nSync complete:")
    click.echo(f"  Imported: {imported}")
    click.echo(f"  Updated: {updated}")
    click.echo(f"  Failed: {failed}")

    if errors:
        click.echo(f"\nErrors:")
        for error in errors[:10]:  # Show first 10 errors
            click.echo(f"  - {error}")
        if len(errors) > 10:
            click.echo(f"  ... and {len(errors) - 10} more")


if __name__ == "__main__":
    cli()
