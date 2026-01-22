"""SQLAlchemy models for web UI persistence."""

from datetime import datetime, timezone
from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    Text,
    DateTime,
    ForeignKey,
    Index,
    CheckConstraint,
    UniqueConstraint,
)
from sqlalchemy.orm import declarative_base, relationship


def utc_now() -> datetime:
    """Return current UTC time (timezone-aware)."""
    return datetime.now(timezone.utc)


Base = declarative_base()


class Game(Base):
    """Evolved game imported from evolution output."""

    __tablename__ = "games"

    id = Column(String, primary_key=True)  # genome_id e.g., "GreenJack"
    genome_json = Column(Text, nullable=False)
    rulebook_md = Column(Text)
    summary = Column(Text)
    fitness = Column(Float)
    created_at = Column(DateTime, default=utc_now)
    play_count = Column(Integer, default=0)
    flag_count = Column(Integer, default=0)
    status = Column(String, default="active")  # active|demoted|archived

    ratings = relationship("Rating", back_populates="game")
    game_sessions = relationship("GameSession", back_populates="game")

    __table_args__ = (Index("idx_games_status_fitness", "status", "fitness"),)


class Rating(Base):
    """Player rating for a game."""

    __tablename__ = "ratings"

    id = Column(Integer, primary_key=True, autoincrement=True)
    game_id = Column(String, ForeignKey("games.id"), nullable=False)
    session_id = Column(String, nullable=False)
    ip_hash = Column(String)
    rating = Column(Integer, nullable=False)
    comment = Column(Text)
    felt_broken = Column(Boolean, default=False)
    created_at = Column(DateTime, default=utc_now)

    game = relationship("Game", back_populates="ratings")

    __table_args__ = (
        CheckConstraint("rating >= 1 AND rating <= 5", name="rating_range"),
        UniqueConstraint("game_id", "session_id", name="unique_rating_per_session"),
        Index("idx_ratings_game", "game_id"),
    )


class GameSession(Base):
    """In-progress game session."""

    __tablename__ = "game_sessions"

    id = Column(String, primary_key=True)  # UUID
    game_id = Column(String, ForeignKey("games.id"), nullable=False)
    session_id = Column(String, nullable=False)
    state_json = Column(Text)
    version = Column(Integer, default=1)
    created_at = Column(DateTime, default=utc_now)
    updated_at = Column(DateTime, default=utc_now, onupdate=utc_now)
    completed = Column(Boolean, default=False)
    duration_seconds = Column(Integer)

    game = relationship("Game", back_populates="game_sessions")

    __table_args__ = (Index("idx_game_sessions_session", "session_id"),)


class Session(Base):
    """Browser session for anonymous identity."""

    __tablename__ = "sessions"

    id = Column(String, primary_key=True)  # Cookie value
    ip_hash = Column(String)
    created_at = Column(DateTime, default=utc_now)
    last_seen = Column(DateTime, default=utc_now)
    expires_at = Column(DateTime)
    games_played = Column(Integer, default=0)
    games_rated = Column(Integer, default=0)

    __table_args__ = (Index("idx_sessions_expires", "expires_at"),)
