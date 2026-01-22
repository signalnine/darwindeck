# tests/unit/test_web_routes_games.py
"""Tests for games API routes."""

import os
import tempfile
import pytest
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from fastapi.testclient import TestClient
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from darwindeck.web.models import Base, Game
from darwindeck.web.dependencies import get_db


def create_test_app():
    """Create a minimal test app without lifespan (avoids DB conflicts)."""
    app = FastAPI(title="DarwinDeck Test")

    # Register routes
    from darwindeck.web.routes import games
    app.include_router(games.router, prefix="/api/games", tags=["games"])

    return app


@pytest.fixture
def client():
    # Use a temporary file for the test database
    fd, db_path = tempfile.mkstemp(suffix=".db")
    os.close(fd)

    try:
        engine = create_engine(
            f"sqlite:///{db_path}",
            connect_args={"check_same_thread": False},
        )
        Base.metadata.create_all(bind=engine)
        SessionLocal = sessionmaker(bind=engine)

        # Create a session and add test data
        db = SessionLocal()
        db.add(Game(id="TestGame1", genome_json='{}', fitness=0.8, status="active"))
        db.add(Game(id="TestGame2", genome_json='{}', fitness=0.6, status="active"))
        db.add(Game(id="DemotedGame", genome_json='{}', fitness=0.3, status="demoted"))
        db.commit()
        db.close()

        # Create app and override dependency
        app = create_test_app()

        def override_get_db():
            session = SessionLocal()
            try:
                yield session
            finally:
                session.close()

        app.dependency_overrides[get_db] = override_get_db

        with TestClient(app) as test_client:
            yield test_client

    finally:
        # Clean up temp file
        if os.path.exists(db_path):
            os.unlink(db_path)


class TestGamesAPI:
    def test_list_games_returns_active_only(self, client):
        response = client.get("/api/games")
        assert response.status_code == 200
        data = response.json()
        assert len(data["games"]) == 2
        assert all(g["status"] == "active" for g in data["games"])

    def test_list_games_sorted_by_fitness(self, client):
        response = client.get("/api/games?sort=fitness")
        assert response.status_code == 200
        data = response.json()
        assert data["games"][0]["id"] == "TestGame1"

    def test_get_game_by_id(self, client):
        response = client.get("/api/games/TestGame1")
        assert response.status_code == 200
        data = response.json()
        assert data["id"] == "TestGame1"
        assert data["fitness"] == 0.8

    def test_get_nonexistent_game_returns_404(self, client):
        response = client.get("/api/games/NoSuchGame")
        assert response.status_code == 404
