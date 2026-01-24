# src/darwindeck/web/routes/admin.py
"""Admin API routes for genome import and management."""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any, Dict, List, Optional

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session as SQLSession

from darwindeck.web.models import Game
from darwindeck.web.dependencies import get_db, verify_admin_dependency
from darwindeck.genome.serialization import genome_from_dict, genome_to_json
from darwindeck.genome.validator import GenomeValidator


# Security: Restrict sync endpoint to an allowed base directory
# Set DARWINDECK_IMPORT_DIR environment variable to customize
ALLOWED_IMPORT_DIR = Path(os.environ.get("DARWINDECK_IMPORT_DIR", "output")).resolve()

# Security: Cap the number of files to prevent DoS
MAX_SYNC_FILES = 1000

router = APIRouter()


class ImportResponse(BaseModel):
    """Response from import endpoint."""
    success: bool
    game_id: str
    updated: bool = False
    errors: Optional[List[str]] = None


class SyncRequest(BaseModel):
    """Request body for sync endpoint."""
    directory: str


class SyncResponse(BaseModel):
    """Response from sync endpoint."""
    imported: int
    failed: int
    game_ids: List[str]
    errors: List[str]


def _validate_and_parse_genome(data: Dict[str, Any]) -> tuple[Any, List[str]]:
    """Validate genome data and return parsed genome or errors.

    Returns:
        Tuple of (genome, errors). If errors is non-empty, genome may be None.
    """
    try:
        genome = genome_from_dict(data)
    except (KeyError, TypeError, ValueError) as e:
        return None, [f"Invalid genome structure: {e}"]

    # Run validation
    errors = GenomeValidator.validate(genome)
    return genome, errors


def _import_genome(
    data: Dict[str, Any],
    db: SQLSession,
    allow_validation_warnings: bool = False,
) -> tuple[Optional[str], bool, List[str]]:
    """Import a single genome into the database.

    Args:
        data: Raw genome dict from JSON
        db: Database session
        allow_validation_warnings: If True, import genomes with validation warnings

    Returns:
        Tuple of (game_id, was_updated, errors)
    """
    genome, errors = _validate_and_parse_genome(data)

    if genome is None:
        return None, False, errors

    if errors and not allow_validation_warnings:
        return None, False, errors

    game_id = genome.genome_id
    fitness = data.get("fitness")

    # Check if game already exists
    existing = db.query(Game).filter(Game.id == game_id).first()
    was_updated = existing is not None

    # Serialize genome back to JSON (normalized form)
    genome_json = genome_to_json(genome)

    if existing:
        # Update existing game
        existing.genome_json = genome_json
        if fitness is not None:
            existing.fitness = fitness
    else:
        # Create new game
        game = Game(
            id=game_id,
            genome_json=genome_json,
            fitness=fitness,
            status="active",
        )
        db.add(game)

    db.commit()
    return game_id, was_updated, errors


@router.post("/import", response_model=ImportResponse)
async def import_genome(
    data: Dict[str, Any],
    db: SQLSession = Depends(get_db),
    _admin: bool = Depends(verify_admin_dependency),
):
    """Import a single genome from JSON.

    Validates the genome structure and creates or updates the Game record.

    Requires admin authentication (localhost or API key).
    """
    game_id, was_updated, errors = _import_genome(data, db)

    if errors:
        raise HTTPException(status_code=400, detail=f"Genome validation errors: {errors}")

    return ImportResponse(
        success=True,
        game_id=game_id,
        updated=was_updated,
    )


@router.post("/sync", response_model=SyncResponse)
async def sync_games(
    request: SyncRequest,
    db: SQLSession = Depends(get_db),
    _admin: bool = Depends(verify_admin_dependency),
):
    """Sync games from a directory of genome JSON files.

    Scans the directory for *.json files, validates each, and imports valid ones.
    Directory must be within DARWINDECK_IMPORT_DIR (default: ./output).

    Requires admin authentication (localhost or API key).
    """
    # Security: Resolve and validate path is within allowed directory
    try:
        directory = (ALLOWED_IMPORT_DIR / request.directory).resolve()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid directory path")

    if not directory.is_relative_to(ALLOWED_IMPORT_DIR):
        raise HTTPException(
            status_code=400,
            detail="Directory must be within allowed import path"
        )

    if not directory.exists():
        raise HTTPException(status_code=400, detail=f"Directory not found: {request.directory}")

    if not directory.is_dir():
        raise HTTPException(status_code=400, detail=f"Not a directory: {request.directory}")

    imported = 0
    failed = 0
    game_ids: List[str] = []
    errors: List[str] = []

    # Find all JSON files in directory (with cap to prevent DoS)
    files = list(directory.glob("*.json"))
    if len(files) > MAX_SYNC_FILES:
        raise HTTPException(
            status_code=400,
            detail=f"Too many files ({len(files)}). Maximum is {MAX_SYNC_FILES}"
        )

    for json_path in files:
        try:
            with open(json_path) as f:
                data = json.load(f)
        except (json.JSONDecodeError, IOError) as e:
            failed += 1
            errors.append(f"{json_path.name}: Failed to read JSON: {e}")
            continue

        game_id, was_updated, validation_errors = _import_genome(data, db)

        if game_id:
            imported += 1
            game_ids.append(game_id)
        else:
            failed += 1
            errors.append(f"{json_path.name}: {validation_errors}")

    return SyncResponse(
        imported=imported,
        failed=failed,
        game_ids=game_ids,
        errors=errors,
    )
