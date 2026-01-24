# tests/unit/test_web_routes_ratings.py
"""Tests for ratings API routes."""

import os
import tempfile
import pytest
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from fastapi.testclient import TestClient
from fastapi import FastAPI

from darwindeck.web.models import Base, Game, Rating, GameSession
from darwindeck.web.dependencies import get_db


def create_test_app():
    """Create a minimal test app without lifespan."""
    app = FastAPI(title="DarwinDeck Test")

    # Register routes
    from darwindeck.web.routes import ratings
    app.include_router(ratings.router, prefix="/api", tags=["ratings"])

    return app


@pytest.fixture
def client():
    """Create test client with mocked dependencies."""
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

        # Games for testing
        db.add(Game(id="TestGame1", genome_json='{}', fitness=0.8, status="active", play_count=10))
        db.add(Game(id="TestGame2", genome_json='{}', fitness=0.6, status="active", play_count=5))
        db.add(Game(id="LowPlayGame", genome_json='{}', fitness=0.7, status="active", play_count=1))
        db.add(Game(id="DemotedGame", genome_json='{}', fitness=0.3, status="demoted", play_count=20))

        # Completed game session for rating validation
        db.add(
            GameSession(
                id="completed-session-123",
                game_id="TestGame1",
                session_id="browser-session-abc",
                state_json='{"turn": 10, "winner": 0}',
                version=10,
                completed=True,
            )
        )
        db.add(
            GameSession(
                id="incomplete-session-456",
                game_id="TestGame1",
                session_id="browser-session-def",
                state_json='{"turn": 5, "hands": [["Ah"], ["2s"]]}',
                version=5,
                completed=False,
            )
        )

        # Existing ratings for leaderboard tests
        db.add(Rating(game_id="TestGame1", session_id="session-1", rating=5, ip_hash="hash1"))
        db.add(Rating(game_id="TestGame1", session_id="session-2", rating=4, ip_hash="hash2"))
        db.add(Rating(game_id="TestGame2", session_id="session-3", rating=3, ip_hash="hash3"))

        db.commit()
        db.close()

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
        if os.path.exists(db_path):
            os.unlink(db_path)


class TestRateGame:
    """Tests for POST /api/games/{game_id}/rate"""

    def test_rate_game_success(self, client):
        """Rating a game should create a rating record."""
        response = client.post(
            "/api/games/TestGame1/rate",
            json={"rating": 4, "session_id": "completed-session-123"},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["rating"] == 4
        assert "avg_rating" in data

    def test_rate_game_updates_avg_rating(self, client):
        """Rating should update the game's average rating."""
        # TestGame1 has ratings 5 and 4, avg = 4.5
        # Adding a 3 should make avg = (5+4+3)/3 = 4.0
        response = client.post(
            "/api/games/TestGame1/rate",
            json={"rating": 3, "session_id": "completed-session-123"},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["avg_rating"] == 4.0

    def test_rate_game_invalid_rating_low(self, client):
        """Rating below 1 should be rejected."""
        response = client.post(
            "/api/games/TestGame1/rate",
            json={"rating": 0, "session_id": "completed-session-123"},
        )
        assert response.status_code == 422  # Validation error

    def test_rate_game_invalid_rating_high(self, client):
        """Rating above 5 should be rejected."""
        response = client.post(
            "/api/games/TestGame1/rate",
            json={"rating": 6, "session_id": "completed-session-123"},
        )
        assert response.status_code == 422  # Validation error

    def test_rate_game_nonexistent_game_returns_404(self, client):
        """Rating a nonexistent game should return 404."""
        response = client.post(
            "/api/games/NoSuchGame/rate",
            json={"rating": 4, "session_id": "some-session"},
        )
        assert response.status_code == 404

    def test_rate_game_duplicate_ip_returns_409(self, client):
        """Rating twice from same IP should return 409 (one rating per IP per game)."""
        # Use LowPlayGame which has no existing ratings from this IP
        # First rating
        response1 = client.post(
            "/api/games/LowPlayGame/rate",
            json={"rating": 4, "session_id": "session-1"},
        )
        assert response1.status_code == 200

        # Second rating from same IP (different session_id, but same IP)
        response2 = client.post(
            "/api/games/LowPlayGame/rate",
            json={"rating": 5, "session_id": "session-2"},
        )
        assert response2.status_code == 409
        assert "already" in response2.json()["detail"].lower()

    def test_rate_game_with_comment(self, client):
        """Rating with optional comment should work."""
        response = client.post(
            "/api/games/TestGame1/rate",
            json={
                "rating": 5,
                "session_id": "session-with-comment",
                "comment": "Great game!",
            },
        )
        assert response.status_code == 200

    def test_rate_game_with_felt_broken(self, client):
        """Rating with felt_broken flag should work."""
        response = client.post(
            "/api/games/TestGame1/rate",
            json={
                "rating": 2,
                "session_id": "session-broken",
                "felt_broken": True,
            },
        )
        assert response.status_code == 200

    def test_rate_game_incomplete_session_allowed(self, client):
        """Rating with incomplete session should be allowed (session validation optional)."""
        response = client.post(
            "/api/games/TestGame1/rate",
            json={"rating": 3, "session_id": "incomplete-session-456"},
        )
        # Should succeed - we don't require completed sessions by default
        assert response.status_code == 200


class TestLeaderboard:
    """Tests for GET /api/leaderboard"""

    def test_leaderboard_returns_games(self, client):
        """Leaderboard should return games sorted by average rating."""
        response = client.get("/api/leaderboard")
        assert response.status_code == 200
        data = response.json()
        assert "games" in data
        assert len(data["games"]) > 0

    def test_leaderboard_sorted_by_avg_rating(self, client):
        """Leaderboard should be sorted by average rating descending."""
        response = client.get("/api/leaderboard")
        assert response.status_code == 200
        data = response.json()
        games = data["games"]

        # Verify sorted in descending order
        for i in range(len(games) - 1):
            if games[i]["avg_rating"] is not None and games[i + 1]["avg_rating"] is not None:
                assert games[i]["avg_rating"] >= games[i + 1]["avg_rating"]

    def test_leaderboard_includes_avg_rating(self, client):
        """Each game in leaderboard should have avg_rating."""
        response = client.get("/api/leaderboard")
        assert response.status_code == 200
        data = response.json()
        for game in data["games"]:
            assert "avg_rating" in game
            assert "id" in game

    def test_leaderboard_respects_min_plays(self, client):
        """Leaderboard should filter out games below minimum play threshold."""
        # LowPlayGame has only 1 play, should be excluded with min_plays=3
        response = client.get("/api/leaderboard?min_plays=3")
        assert response.status_code == 200
        data = response.json()
        game_ids = [g["id"] for g in data["games"]]
        assert "LowPlayGame" not in game_ids

    def test_leaderboard_excludes_demoted_games(self, client):
        """Leaderboard should only include active games."""
        response = client.get("/api/leaderboard")
        assert response.status_code == 200
        data = response.json()
        game_ids = [g["id"] for g in data["games"]]
        assert "DemotedGame" not in game_ids

    def test_leaderboard_respects_limit(self, client):
        """Leaderboard should respect limit parameter."""
        response = client.get("/api/leaderboard?limit=1")
        assert response.status_code == 200
        data = response.json()
        assert len(data["games"]) <= 1

    def test_leaderboard_includes_rating_count(self, client):
        """Each game should include the number of ratings."""
        response = client.get("/api/leaderboard")
        assert response.status_code == 200
        data = response.json()
        for game in data["games"]:
            assert "rating_count" in game
