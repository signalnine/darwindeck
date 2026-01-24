# tests/unit/test_web_routes_sessions.py
"""Tests for sessions API routes."""

import os
import tempfile
import pytest
from unittest.mock import AsyncMock, MagicMock
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from fastapi.testclient import TestClient
from fastapi import FastAPI

from darwindeck.web.models import Base, Game, GameSession
from darwindeck.web.dependencies import get_db, get_worker


def create_test_app():
    """Create a minimal test app without lifespan."""
    app = FastAPI(title="DarwinDeck Test")

    # Register routes
    from darwindeck.web.routes import sessions
    app.include_router(sessions.router, prefix="/api", tags=["sessions"])

    return app


class MockWorker:
    """Mock simulation worker for testing."""

    def __init__(self):
        self.start_game_result = {
            "state": {
                "turn": 0,
                "active_player": 0,
                "hands": [["Ah", "2s"], ["3d", "4c"]],
                "legal_moves": [{"type": "play", "card": "Ah"}, {"type": "play", "card": "2s"}],
            }
        }
        self.apply_move_result = {
            "state": {
                "turn": 1,
                "active_player": 1,
                "hands": [["2s"], ["3d", "4c"]],
                "legal_moves": [{"type": "play", "card": "3d"}],
            }
        }

    async def execute(self, command: dict, timeout: float = 5.0) -> dict:
        """Mock execute that returns predefined results."""
        if command.get("action") == "start_game":
            return self.start_game_result
        elif command.get("action") == "apply_move":
            return self.apply_move_result
        return {"error": "Unknown command"}


@pytest.fixture
def mock_worker():
    """Create a mock worker."""
    return MockWorker()


@pytest.fixture
def client(mock_worker):
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
        db.add(Game(id="TestGame", genome_json='{"name": "Test"}', fitness=0.8, status="active"))
        db.add(
            GameSession(
                id="existing-session-123",
                game_id="TestGame",
                session_id="browser-session-abc",
                state_json='{"turn": 5, "hands": [["Ah"], ["2s"]]}',
                version=5,
                completed=False,
            )
        )
        db.add(
            GameSession(
                id="completed-session-456",
                game_id="TestGame",
                session_id="browser-session-abc",
                state_json='{"turn": 10, "winner": 0}',
                version=10,
                completed=True,
            )
        )
        db.commit()
        db.close()

        app = create_test_app()

        def override_get_db():
            session = SessionLocal()
            try:
                yield session
            finally:
                session.close()

        def override_get_worker():
            return mock_worker

        app.dependency_overrides[get_db] = override_get_db
        app.dependency_overrides[get_worker] = override_get_worker

        with TestClient(app) as test_client:
            yield test_client

    finally:
        if os.path.exists(db_path):
            os.unlink(db_path)


class TestStartGame:
    """Tests for POST /api/games/{game_id}/start"""

    def test_start_game_creates_session(self, client):
        """Starting a game should create a new session and return initial state."""
        response = client.post("/api/games/TestGame/start")
        assert response.status_code == 200
        data = response.json()

        # Should return session ID and initial state
        assert "session_id" in data
        assert "state" in data
        assert data["state"]["turn"] == 0
        assert data["version"] == 1

    def test_start_game_nonexistent_game_returns_404(self, client):
        """Starting a game that doesn't exist should return 404."""
        response = client.post("/api/games/NoSuchGame/start")
        assert response.status_code == 404

    def test_start_game_stores_state_in_db(self, client):
        """Starting a game should persist the session in database."""
        response = client.post("/api/games/TestGame/start")
        assert response.status_code == 200
        session_id = response.json()["session_id"]

        # Verify we can retrieve it
        get_response = client.get(f"/api/sessions/{session_id}")
        assert get_response.status_code == 200
        data = get_response.json()
        assert data["game_id"] == "TestGame"


class TestGetSession:
    """Tests for GET /api/sessions/{session_id}"""

    def test_get_session_returns_state(self, client):
        """Getting a session should return current state for resuming."""
        response = client.get("/api/sessions/existing-session-123")
        assert response.status_code == 200
        data = response.json()

        assert data["session_id"] == "existing-session-123"
        assert data["game_id"] == "TestGame"
        assert data["version"] == 5
        assert data["completed"] is False
        assert "state" in data

    def test_get_session_nonexistent_returns_404(self, client):
        """Getting a session that doesn't exist should return 404."""
        response = client.get("/api/sessions/no-such-session")
        assert response.status_code == 404

    def test_get_completed_session(self, client):
        """Getting a completed session should return completed=True."""
        response = client.get("/api/sessions/completed-session-456")
        assert response.status_code == 200
        data = response.json()
        assert data["completed"] is True


class TestApplyMove:
    """Tests for POST /api/sessions/{session_id}/move"""

    def test_apply_move_updates_state(self, client):
        """Applying a move should update state and increment version."""
        response = client.post(
            "/api/sessions/existing-session-123/move",
            json={"move": {"type": "play", "card": "Ah"}, "version": 5},
        )
        assert response.status_code == 200
        data = response.json()

        # State should be updated
        assert data["state"]["turn"] == 1
        assert data["version"] == 6

    def test_apply_move_wrong_version_returns_409(self, client):
        """Applying a move with wrong version should return 409 Conflict."""
        response = client.post(
            "/api/sessions/existing-session-123/move",
            json={"move": {"type": "play", "card": "Ah"}, "version": 3},
        )
        assert response.status_code == 409
        data = response.json()
        assert "version" in data["detail"].lower() or "conflict" in data["detail"].lower()

    def test_apply_move_nonexistent_session_returns_404(self, client):
        """Applying a move to nonexistent session should return 404."""
        response = client.post(
            "/api/sessions/no-such-session/move",
            json={"move": {"type": "play", "card": "Ah"}, "version": 1},
        )
        assert response.status_code == 404

    def test_apply_move_to_completed_session_returns_400(self, client):
        """Applying a move to completed session should return 400."""
        response = client.post(
            "/api/sessions/completed-session-456/move",
            json={"move": {"type": "play", "card": "Ah"}, "version": 10},
        )
        assert response.status_code == 400
        data = response.json()
        assert "completed" in data["detail"].lower() or "finished" in data["detail"].lower()


class TestFlagSession:
    """Tests for POST /api/sessions/{session_id}/flag"""

    def test_flag_session_marks_as_flagged(self, client):
        """Flagging a session should mark it and increment game flag_count."""
        response = client.post(
            "/api/sessions/existing-session-123/flag",
            json={"reason": "Game got stuck"},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["flagged"] is True

    def test_flag_session_nonexistent_returns_404(self, client):
        """Flagging nonexistent session should return 404."""
        response = client.post(
            "/api/sessions/no-such-session/flag",
            json={"reason": "broken"},
        )
        assert response.status_code == 404

    def test_flag_session_reason_optional(self, client):
        """Flagging without a reason should still work."""
        response = client.post("/api/sessions/existing-session-123/flag", json={})
        assert response.status_code == 200
