"""Database setup and utilities."""

from pathlib import Path
from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker, Session as SQLSession

from darwindeck.web.models import Base


def get_engine(db_path: str = "data/playtest.db"):
    """Create SQLAlchemy engine with WAL mode."""
    Path(db_path).parent.mkdir(parents=True, exist_ok=True)
    engine = create_engine(
        f"sqlite:///{db_path}",
        connect_args={"check_same_thread": False},
    )
    # Enable WAL mode for better concurrent access
    with engine.connect() as conn:
        conn.execute(text("PRAGMA journal_mode=WAL"))
    return engine


def init_db(session_or_engine) -> None:
    """Create all tables."""
    if hasattr(session_or_engine, "get_bind"):
        engine = session_or_engine.get_bind()
    else:
        engine = session_or_engine
    Base.metadata.create_all(engine)


def get_session(db_path: str = "data/playtest.db") -> SQLSession:
    """Get a new database session."""
    engine = get_engine(db_path)
    SessionLocal = sessionmaker(bind=engine)
    return SessionLocal()


def get_test_db() -> SQLSession:
    """Get an in-memory database for testing."""
    engine = create_engine("sqlite:///:memory:")
    SessionLocal = sessionmaker(bind=engine)
    return SessionLocal()
