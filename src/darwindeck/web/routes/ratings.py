# src/darwindeck/web/routes/ratings.py
"""Ratings API routes."""

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Query, Request
from pydantic import BaseModel, ConfigDict, Field
from sqlalchemy.orm import Session as SQLSession
from sqlalchemy import func
from sqlalchemy.exc import IntegrityError

from darwindeck.web.models import Game, Rating
from darwindeck.web.dependencies import get_db
from darwindeck.web.security import get_real_ip, hash_ip


router = APIRouter()


# Request/Response models


class RateGameRequest(BaseModel):
    """Request to rate a game."""

    rating: int = Field(..., ge=1, le=5, description="Rating from 1 to 5")
    session_id: str = Field(..., description="Session ID to prevent duplicate ratings")
    comment: Optional[str] = Field(None, max_length=1000, description="Optional comment")
    felt_broken: bool = Field(False, description="Flag if game felt broken")


class RateGameResponse(BaseModel):
    """Response after rating a game."""

    rating: int
    avg_rating: float


class LeaderboardGame(BaseModel):
    """Game entry in leaderboard."""

    model_config = ConfigDict(from_attributes=True)

    id: str
    summary: Optional[str]
    fitness: Optional[float]
    play_count: int
    avg_rating: Optional[float]
    rating_count: int


class LeaderboardResponse(BaseModel):
    """Response for leaderboard."""

    games: list[LeaderboardGame]


# Endpoints


@router.post("/games/{game_id}/rate", response_model=RateGameResponse)
async def rate_game(
    game_id: str,
    rate_request: RateGameRequest,
    request: Request,
    db: SQLSession = Depends(get_db),
):
    """Rate a game.

    Validates:
    - Game exists
    - Rating is between 1-5 (via Pydantic)
    - No duplicate rating from same IP (one rating per IP per game)

    After saving the rating, recalculates the game's average rating.
    """
    # Verify game exists
    game = db.query(Game).filter(Game.id == game_id).first()
    if not game:
        raise HTTPException(status_code=404, detail="Game not found")

    # Get player IP for tracking
    ip = get_real_ip(request)
    ip_hash = hash_ip(ip)

    # Check for duplicate rating from same IP (one rating per IP per game)
    existing_rating = (
        db.query(Rating)
        .filter(Rating.game_id == game_id, Rating.ip_hash == ip_hash)
        .first()
    )
    if existing_rating:
        raise HTTPException(
            status_code=409,
            detail="You have already rated this game from this IP address",
        )

    # Create rating record
    rating = Rating(
        game_id=game_id,
        session_id=rate_request.session_id,
        ip_hash=ip_hash,
        rating=rate_request.rating,
        comment=rate_request.comment,
        felt_broken=rate_request.felt_broken,
    )

    try:
        db.add(rating)
        db.flush()  # Flush to catch unique constraint violation
    except IntegrityError:
        db.rollback()
        raise HTTPException(
            status_code=409,
            detail="You have already rated this game",
        )

    # Calculate new average rating
    avg_rating = (
        db.query(func.avg(Rating.rating))
        .filter(Rating.game_id == game_id)
        .scalar()
    )

    db.commit()

    return RateGameResponse(
        rating=rate_request.rating,
        avg_rating=round(avg_rating, 2) if avg_rating else rate_request.rating,
    )


@router.get("/leaderboard", response_model=LeaderboardResponse)
async def get_leaderboard(
    min_plays: int = Query(0, ge=0, description="Minimum play count to be included"),
    limit: int = Query(20, ge=1, le=100, description="Maximum number of games to return"),
    db: SQLSession = Depends(get_db),
):
    """Get leaderboard of top-rated games.

    Returns games sorted by average rating descending.
    Only includes active games with at least min_plays plays.
    """
    # Query games with their ratings aggregated
    query = (
        db.query(
            Game,
            func.avg(Rating.rating).label("avg_rating"),
            func.count(Rating.id).label("rating_count"),
        )
        .outerjoin(Rating)
        .filter(
            Game.status == "active",
            Game.play_count >= min_plays,
        )
        .group_by(Game.id)
        .order_by(func.avg(Rating.rating).desc().nullslast())
        .limit(limit)
    )

    results = query.all()

    games = [
        LeaderboardGame(
            id=game.id,
            summary=game.summary,
            fitness=game.fitness,
            play_count=game.play_count,
            avg_rating=round(avg_rating, 2) if avg_rating else None,
            rating_count=rating_count,
        )
        for game, avg_rating, rating_count in results
    ]

    return LeaderboardResponse(games=games)
