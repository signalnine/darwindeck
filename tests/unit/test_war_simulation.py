"""Tests for War game simulation."""

import pytest
from darwindeck.simulation.war import WarGame, play_war_game


def test_war_game_initialization() -> None:
    """Test War game initializes with 52 cards split evenly."""
    game = WarGame(seed=42)
    assert len(game.player1_hand) == 26
    assert len(game.player2_hand) == 26


def test_play_single_battle() -> None:
    """Test a single battle resolves correctly."""
    game = WarGame(seed=42)
    initial_p1 = len(game.player1_hand)
    initial_p2 = len(game.player2_hand)

    game.play_battle()

    # One player should have gained cards
    assert len(game.player1_hand) + len(game.player2_hand) == 52


def test_play_full_game() -> None:
    """Test a full game runs to completion."""
    result = play_war_game(seed=42, max_turns=1000)

    assert result["winner"] in [1, 2]
    assert result["turns"] > 0
    assert result["turns"] <= 1000
