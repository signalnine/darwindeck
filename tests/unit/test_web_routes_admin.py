# tests/unit/test_web_routes_admin.py
"""Tests for admin API routes."""

import json
import os
import tempfile
from pathlib import Path
from unittest.mock import patch
import pytest
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from fastapi.testclient import TestClient
from fastapi import FastAPI
from darwindeck.web.models import Base, Game
from darwindeck.web.dependencies import get_db, verify_admin_dependency


# Valid minimal genome for testing
VALID_GENOME = {
    "schema_version": "1.0",
    "genome_id": "TestImport",
    "generation": 1,
    "setup": {
        "cards_per_player": 5,
        "initial_deck": "standard_52",
        "initial_discard_count": 0,
        "starting_chips": 0,
    },
    "turn_structure": {
        "phases": [
            {
                "type": "DrawPhase",
                "source": "DECK",
                "count": 1,
                "mandatory": True,
                "condition": None,
            },
            {
                "type": "PlayPhase",
                "target": "DISCARD",
                "valid_play_condition": {
                    "type": "simple",
                    "condition_type": "CARD_MATCHES_RANK",
                    "operator": None,
                    "value": None,
                    "reference": None,
                },
                "min_cards": 1,
                "max_cards": 1,
                "mandatory": True,
            },
        ],
        "is_trick_based": False,
        "tricks_per_hand": None,
    },
    "special_effects": [],
    "win_conditions": [{"type": "empty_hand", "threshold": None}],
    "scoring_rules": [],
    "max_turns": 100,
    "min_turns": 1,
    "player_count": 2,
}


def create_test_app():
    """Create a minimal test app with admin routes."""
    app = FastAPI(title="DarwinDeck Test")

    # Register admin routes
    from darwindeck.web.routes import admin
    app.include_router(admin.router, prefix="/api/admin", tags=["admin"])

    return app


@pytest.fixture
def client():
    """Create test client with mocked admin access."""
    fd, db_path = tempfile.mkstemp(suffix=".db")
    os.close(fd)

    try:
        engine = create_engine(
            f"sqlite:///{db_path}",
            connect_args={"check_same_thread": False},
        )
        Base.metadata.create_all(bind=engine)
        SessionLocal = sessionmaker(bind=engine)

        # Create app and override dependencies
        app = create_test_app()

        def override_get_db():
            session = SessionLocal()
            try:
                yield session
            finally:
                session.close()

        # Mock admin verification to always allow
        async def override_admin():
            return True

        app.dependency_overrides[get_db] = override_get_db
        app.dependency_overrides[verify_admin_dependency] = override_admin

        with TestClient(app) as test_client:
            # Also pass the DB session maker for direct DB checks
            test_client.db_maker = SessionLocal
            yield test_client

    finally:
        if os.path.exists(db_path):
            os.unlink(db_path)


class TestImportEndpoint:
    """Tests for POST /api/admin/import"""

    def test_import_valid_genome_creates_game(self, client):
        """Import a valid genome should create a Game record."""
        response = client.post("/api/admin/import", json=VALID_GENOME)
        assert response.status_code == 200
        data = response.json()
        assert data["success"] is True
        assert data["game_id"] == "TestImport"

        # Verify game exists in DB
        db = client.db_maker()
        game = db.query(Game).filter(Game.id == "TestImport").first()
        assert game is not None
        assert game.status == "active"
        db.close()

    def test_import_updates_existing_game(self, client):
        """Re-importing a genome should update the existing record."""
        # First import
        response = client.post("/api/admin/import", json=VALID_GENOME)
        assert response.status_code == 200

        # Modify genome and re-import
        updated_genome = VALID_GENOME.copy()
        updated_genome["max_turns"] = 200
        response = client.post("/api/admin/import", json=updated_genome)
        assert response.status_code == 200
        data = response.json()
        assert data["success"] is True
        assert data["updated"] is True

        # Verify genome was updated
        db = client.db_maker()
        game = db.query(Game).filter(Game.id == "TestImport").first()
        stored_genome = json.loads(game.genome_json)
        assert stored_genome["max_turns"] == 200
        db.close()

    def test_import_invalid_genome_returns_error(self, client):
        """Importing an invalid genome should return validation errors."""
        invalid_genome = {
            "schema_version": "1.0",
            "genome_id": "InvalidGame",
            "generation": 1,
            "setup": {"cards_per_player": 5},
            # Missing required fields
        }
        response = client.post("/api/admin/import", json=invalid_genome)
        assert response.status_code == 400
        data = response.json()
        assert "detail" in data
        assert "error" in data["detail"].lower() or "invalid" in data["detail"].lower()

    def test_import_extracts_fitness(self, client):
        """Import should extract fitness from genome if present."""
        genome_with_fitness = VALID_GENOME.copy()
        genome_with_fitness["fitness"] = 0.85
        genome_with_fitness["genome_id"] = "FitnessGame"
        response = client.post("/api/admin/import", json=genome_with_fitness)
        assert response.status_code == 200

        db = client.db_maker()
        game = db.query(Game).filter(Game.id == "FitnessGame").first()
        assert game.fitness == 0.85
        db.close()


class TestSyncEndpoint:
    """Tests for POST /api/admin/sync"""

    def test_sync_imports_json_files(self, client):
        """Sync should import all valid JSON genome files from directory."""
        with tempfile.TemporaryDirectory() as tmpdir:
            tmpdir_path = Path(tmpdir).resolve()
            # Create subdirectory for genomes
            subdir = tmpdir_path / "genomes"
            subdir.mkdir()

            # Create valid genome files
            genome1 = VALID_GENOME.copy()
            genome1["genome_id"] = "SyncGame1"
            with open(subdir / "game1.json", "w") as f:
                json.dump(genome1, f)

            genome2 = VALID_GENOME.copy()
            genome2["genome_id"] = "SyncGame2"
            with open(subdir / "game2.json", "w") as f:
                json.dump(genome2, f)

            # Patch ALLOWED_IMPORT_DIR to the temp directory
            with patch("darwindeck.web.routes.admin.ALLOWED_IMPORT_DIR", tmpdir_path):
                response = client.post("/api/admin/sync", json={"directory": "genomes"})
            assert response.status_code == 200
            data = response.json()
            assert data["imported"] == 2
            assert data["failed"] == 0

            # Verify games exist
            db = client.db_maker()
            assert db.query(Game).filter(Game.id == "SyncGame1").first() is not None
            assert db.query(Game).filter(Game.id == "SyncGame2").first() is not None
            db.close()

    def test_sync_skips_invalid_files(self, client):
        """Sync should skip invalid genome files and report failures."""
        with tempfile.TemporaryDirectory() as tmpdir:
            tmpdir_path = Path(tmpdir).resolve()
            # Create subdirectory for genomes
            subdir = tmpdir_path / "genomes"
            subdir.mkdir()

            # Create valid genome
            genome1 = VALID_GENOME.copy()
            genome1["genome_id"] = "ValidGame"
            with open(subdir / "valid.json", "w") as f:
                json.dump(genome1, f)

            # Create invalid genome (missing required fields)
            with open(subdir / "invalid.json", "w") as f:
                json.dump({"genome_id": "Broken", "invalid": True}, f)

            # Create non-JSON file (should be ignored)
            with open(subdir / "readme.txt", "w") as f:
                f.write("Not a genome file")

            # Patch ALLOWED_IMPORT_DIR to the temp directory
            with patch("darwindeck.web.routes.admin.ALLOWED_IMPORT_DIR", tmpdir_path):
                response = client.post("/api/admin/sync", json={"directory": "genomes"})
            assert response.status_code == 200
            data = response.json()
            assert data["imported"] == 1
            assert data["failed"] == 1
            assert "errors" in data
            assert len(data["errors"]) == 1

    def test_sync_nonexistent_directory_returns_error(self, client):
        """Sync with nonexistent directory should return error."""
        with tempfile.TemporaryDirectory() as tmpdir:
            tmpdir_path = Path(tmpdir).resolve()
            # Patch ALLOWED_IMPORT_DIR to the temp directory
            with patch("darwindeck.web.routes.admin.ALLOWED_IMPORT_DIR", tmpdir_path):
                response = client.post("/api/admin/sync", json={"directory": "nonexistent"})
            assert response.status_code == 400
            data = response.json()
            assert "detail" in data

    def test_sync_returns_game_ids(self, client):
        """Sync should return list of imported game IDs."""
        with tempfile.TemporaryDirectory() as tmpdir:
            tmpdir_path = Path(tmpdir).resolve()
            # Create subdirectory for genomes
            subdir = tmpdir_path / "genomes"
            subdir.mkdir()

            genome = VALID_GENOME.copy()
            genome["genome_id"] = "ListedGame"
            with open(subdir / "game.json", "w") as f:
                json.dump(genome, f)

            # Patch ALLOWED_IMPORT_DIR to the temp directory
            with patch("darwindeck.web.routes.admin.ALLOWED_IMPORT_DIR", tmpdir_path):
                response = client.post("/api/admin/sync", json={"directory": "genomes"})
            assert response.status_code == 200
            data = response.json()
            assert "game_ids" in data
            assert "ListedGame" in data["game_ids"]

    def test_sync_rejects_path_traversal(self, client):
        """Sync should reject path traversal attempts."""
        with tempfile.TemporaryDirectory() as tmpdir:
            tmpdir_path = Path(tmpdir).resolve()
            # Patch ALLOWED_IMPORT_DIR to the temp directory
            with patch("darwindeck.web.routes.admin.ALLOWED_IMPORT_DIR", tmpdir_path):
                # Try to escape the allowed directory
                response = client.post("/api/admin/sync", json={"directory": "../../../etc"})
            assert response.status_code == 400
            data = response.json()
            assert "allowed import path" in data["detail"]

    def test_sync_rejects_too_many_files(self, client):
        """Sync should reject directories with too many files."""
        with tempfile.TemporaryDirectory() as tmpdir:
            tmpdir_path = Path(tmpdir).resolve()
            # Create subdirectory for genomes
            subdir = tmpdir_path / "genomes"
            subdir.mkdir()

            # Create many files (more than MAX_SYNC_FILES)
            from darwindeck.web.routes.admin import MAX_SYNC_FILES
            for i in range(MAX_SYNC_FILES + 10):
                (subdir / f"genome_{i}.json").touch()

            # Patch ALLOWED_IMPORT_DIR to the temp directory
            with patch("darwindeck.web.routes.admin.ALLOWED_IMPORT_DIR", tmpdir_path):
                response = client.post("/api/admin/sync", json={"directory": "genomes"})
            assert response.status_code == 400
            data = response.json()
            assert "Too many files" in data["detail"]


class TestAdminAuth:
    """Tests that admin routes require authentication."""

    def test_import_without_auth_denied(self):
        """Import without admin auth should be denied."""
        # Create app WITHOUT mocked admin - use real verification
        fd, db_path = tempfile.mkstemp(suffix=".db")
        os.close(fd)

        try:
            engine = create_engine(
                f"sqlite:///{db_path}",
                connect_args={"check_same_thread": False},
            )
            Base.metadata.create_all(bind=engine)
            SessionLocal = sessionmaker(bind=engine)

            app = create_test_app()

            def override_get_db():
                session = SessionLocal()
                try:
                    yield session
                finally:
                    session.close()

            app.dependency_overrides[get_db] = override_get_db
            # NOTE: Not overriding verify_admin_dependency - use real auth

            with TestClient(app) as test_client:
                # Simulate request from remote IP (not localhost)
                response = test_client.post(
                    "/api/admin/import",
                    json=VALID_GENOME,
                    headers={"X-Forwarded-For": "203.0.113.50"},
                )
                assert response.status_code == 403

        finally:
            if os.path.exists(db_path):
                os.unlink(db_path)
