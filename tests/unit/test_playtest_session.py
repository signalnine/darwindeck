"""Tests for PlaytestSession."""

import pytest
from unittest.mock import Mock, patch
from darwindeck.playtest.session import PlaytestSession, SessionConfig
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    PlayPhase, Location
)


def make_simple_genome() -> GameGenome:
    """Create simple test genome."""
    return GameGenome(
        schema_version="1.0",
        genome_id="TestGame",
        generation=1,
        setup=SetupRules(cards_per_player=2),
        turn_structure=TurnStructure(phases=[
            PlayPhase(target=Location.DISCARD)
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        max_turns=100,
        min_turns=1,
        player_count=2,
    )


class TestSessionConfig:
    """Tests for SessionConfig."""

    def test_default_config(self):
        """Default config has sensible values."""
        config = SessionConfig()

        assert config.difficulty == "greedy"
        assert config.debug is False
        assert config.max_turns == 200

    def test_seed_generation(self):
        """Generates seed if not provided."""
        config1 = SessionConfig()
        config2 = SessionConfig()

        # Seeds should be set
        assert config1.seed is not None
        assert config2.seed is not None


class TestPlaytestSession:
    """Tests for PlaytestSession."""

    def test_initialization(self):
        """Session initializes correctly."""
        genome = make_simple_genome()
        config = SessionConfig(seed=12345)

        session = PlaytestSession(genome, config)

        assert session.seed == 12345
        assert session.genome == genome
        assert session.move_history == []

    def test_assigns_human_player(self):
        """Assigns human to player 0 or 1."""
        genome = make_simple_genome()
        config = SessionConfig(seed=12345)

        session = PlaytestSession(genome, config)

        assert session.human_player_idx in (0, 1)

    def test_move_history_tracking(self):
        """Tracks moves in history."""
        genome = make_simple_genome()
        config = SessionConfig(seed=12345)
        session = PlaytestSession(genome, config)

        # Simulate adding moves
        session._record_move(0, "human", {"card": 0})
        session._record_move(1, "ai", {"card": 1})

        assert len(session.move_history) == 2
        assert session.move_history[0]["player"] == "human"
        assert session.move_history[1]["player"] == "ai"
