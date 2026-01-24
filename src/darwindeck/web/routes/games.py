"""Games API routes."""

from __future__ import annotations

from typing import Optional, Literal

from fastapi import APIRouter, Depends, HTTPException, Query
from pydantic import BaseModel, ConfigDict
from sqlalchemy.orm import Session as SQLSession
from sqlalchemy import func

from darwindeck.web.models import Game, Rating
from darwindeck.web.dependencies import get_db


router = APIRouter()


class GameSummary(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    summary: Optional[str]
    fitness: Optional[float]
    play_count: int
    avg_rating: Optional[float]
    status: str


class GameDetail(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    genome_json: str
    rulebook_md: Optional[str]
    summary: Optional[str]
    fitness: Optional[float]
    play_count: int
    flag_count: int
    status: str


class GamesListResponse(BaseModel):
    games: list[GameSummary]
    total: int
    offset: int
    limit: int


@router.get("", response_model=GamesListResponse)
async def list_games(
    sort: Literal["rating", "fitness", "newest", "random"] = "fitness",
    min_fitness: float = 0.0,
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    db: SQLSession = Depends(get_db),
):
    """List active games with filtering and sorting."""
    query = db.query(
        Game,
        func.avg(Rating.rating).label("avg_rating"),
    ).outerjoin(Rating).filter(
        Game.status == "active",
        Game.fitness >= min_fitness,
    ).group_by(Game.id)

    # Apply sorting
    if sort == "fitness":
        query = query.order_by(Game.fitness.desc())
    elif sort == "rating":
        query = query.order_by(func.avg(Rating.rating).desc().nullslast())
    elif sort == "newest":
        query = query.order_by(Game.created_at.desc())
    elif sort == "random":
        query = query.order_by(func.random())

    total = query.count()
    results = query.offset(offset).limit(limit).all()

    games = [
        GameSummary(
            id=game.id,
            summary=game.summary,
            fitness=game.fitness,
            play_count=game.play_count,
            avg_rating=avg_rating,
            status=game.status,
        )
        for game, avg_rating in results
    ]

    return GamesListResponse(games=games, total=total, offset=offset, limit=limit)


@router.get("/{game_id}", response_model=GameDetail)
async def get_game(game_id: str, db: SQLSession = Depends(get_db)):
    """Get game details by ID."""
    game = db.query(Game).filter(Game.id == game_id).first()
    if not game:
        raise HTTPException(status_code=404, detail="Game not found")
    return game


@router.get("/{game_id}/summary")
async def get_game_summary(game_id: str, db: SQLSession = Depends(get_db)):
    """Get AI-generated game summary."""
    game = db.query(Game).filter(Game.id == game_id).first()
    if not game:
        raise HTTPException(status_code=404, detail="Game not found")
    return {"summary": game.summary or "No summary available"}
