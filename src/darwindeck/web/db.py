"""Database setup and utilities."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker, Session as SQLSession

from darwindeck.web.models import Base

# Module-level engine cache to avoid recreating engines
_engines: dict[str, Any] = {}
_session_factories: dict[str, Any] = {}


def get_engine(db_path: str = "data/playtest.db"):
    """Create or get cached SQLAlchemy engine with WAL mode.

    Engines are cached per db_path to avoid recreating connections
    and re-running PRAGMA statements.
    """
    if db_path in _engines:
        return _engines[db_path]

    Path(db_path).parent.mkdir(parents=True, exist_ok=True)
    engine = create_engine(
        f"sqlite:///{db_path}",
        connect_args={"check_same_thread": False},
    )
    # Enable WAL mode for better concurrent access
    with engine.connect() as conn:
        conn.execute(text("PRAGMA journal_mode=WAL"))
        conn.commit()  # Explicitly commit pragma

    _engines[db_path] = engine
    return engine


def init_db(session_or_engine) -> None:
    """Create all tables."""
    if hasattr(session_or_engine, "get_bind"):
        engine = session_or_engine.get_bind()
    else:
        engine = session_or_engine
    Base.metadata.create_all(engine)


def get_session(db_path: str = "data/playtest.db") -> SQLSession:
    """Get a new database session.

    Sessions are created from a cached sessionmaker to reuse the engine.
    """
    if db_path not in _session_factories:
        engine = get_engine(db_path)
        _session_factories[db_path] = sessionmaker(bind=engine)
    return _session_factories[db_path]()


def get_test_db() -> SQLSession:
    """Get an in-memory database for testing.

    Note: Caller must call init_db(session) to create tables.
    Each call creates a fresh database.
    """
    engine = create_engine("sqlite:///:memory:")
    SessionLocal = sessionmaker(bind=engine)
    return SessionLocal()
