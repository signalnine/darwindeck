"""Tests for web UI database models."""

import pytest
from datetime import datetime, timedelta, timezone
from darwindeck.web.db import get_test_db, init_db
from darwindeck.web.models import Game, Rating, GameSession, Session


class TestGameModel:
    def test_create_game(self):
        db = get_test_db()
        init_db(db)

        game = Game(
            id="TestGame",
            genome_json='{"name": "TestGame"}',
            fitness=0.75,
        )
        db.add(game)
        db.commit()

        loaded = db.query(Game).filter(Game.id == "TestGame").first()
        assert loaded is not None
        assert loaded.fitness == 0.75
        assert loaded.status == "active"
        assert loaded.play_count == 0

    def test_game_default_values(self):
        db = get_test_db()
        init_db(db)

        game = Game(id="Test2", genome_json='{}')
        db.add(game)
        db.commit()

        loaded = db.get(Game, "Test2")
        assert loaded.flag_count == 0
        assert loaded.status == "active"


class TestRatingModel:
    def test_create_rating(self):
        db = get_test_db()
        init_db(db)

        game = Game(id="RatedGame", genome_json='{}')
        db.add(game)
        db.commit()

        rating = Rating(
            game_id="RatedGame",
            session_id="session123",
            ip_hash="abc123",
            rating=4,
            comment="Fun game!",
        )
        db.add(rating)
        db.commit()

        loaded = db.query(Rating).filter(Rating.game_id == "RatedGame").first()
        assert loaded.rating == 4
        assert loaded.comment == "Fun game!"


class TestSessionModel:
    def test_session_expiry(self):
        db = get_test_db()
        init_db(db)

        now = datetime.now(timezone.utc)
        session = Session(
            id="sess123",
            ip_hash="hash",
            expires_at=now + timedelta(days=30),
        )
        db.add(session)
        db.commit()

        loaded = db.get(Session, "sess123")
        # Compare without timezone info since SQLite doesn't preserve it
        assert loaded.expires_at > now.replace(tzinfo=None)


class TestGameSessionModel:
    def test_create_game_session(self):
        db = get_test_db()
        init_db(db)

        # Create a game first
        game = Game(id="SessionGame", genome_json='{}')
        db.add(game)
        db.commit()

        game_session = GameSession(
            id="game-session-uuid",
            game_id="SessionGame",
            session_id="browser-session",
            state_json='{"turn": 5}',
        )
        db.add(game_session)
        db.commit()

        loaded = db.get(GameSession, "game-session-uuid")
        assert loaded is not None
        assert loaded.game_id == "SessionGame"
        assert loaded.completed is False
        assert loaded.version == 1

    def test_game_session_relationships(self):
        db = get_test_db()
        init_db(db)

        game = Game(id="RelGame", genome_json='{}')
        db.add(game)
        db.commit()

        game_session = GameSession(
            id="rel-session",
            game_id="RelGame",
            session_id="browser",
        )
        db.add(game_session)
        db.commit()

        # Test relationship navigation
        loaded = db.get(GameSession, "rel-session")
        assert loaded.game.id == "RelGame"

        # Test reverse relationship
        loaded_game = db.get(Game, "RelGame")
        assert len(loaded_game.game_sessions) == 1
        assert loaded_game.game_sessions[0].id == "rel-session"
