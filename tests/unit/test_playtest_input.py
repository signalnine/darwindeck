"""Tests for HumanPlayer input handling."""

import pytest
from unittest.mock import patch
from darwindeck.playtest.input import HumanPlayer, InputResult
from darwindeck.simulation.movegen import LegalMove
from darwindeck.genome.schema import Location


class TestHumanPlayer:
    """Tests for HumanPlayer."""

    def test_parses_valid_number(self):
        """Parses valid move number."""
        player = HumanPlayer()
        moves = [
            LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD),
            LegalMove(phase_index=0, card_index=1, target_loc=Location.DISCARD),
        ]

        with patch("builtins.input", return_value="1"):
            result = player.get_move(moves)

        assert result.move == moves[0]
        assert not result.quit
        assert result.error is None

    def test_parses_quit_command(self):
        """Recognizes quit command."""
        player = HumanPlayer()
        moves = [LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD)]

        with patch("builtins.input", return_value="q"):
            result = player.get_move(moves)

        assert result.quit
        assert result.move is None

    def test_handles_invalid_number(self):
        """Returns error for invalid number."""
        player = HumanPlayer()
        moves = [LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD)]

        with patch("builtins.input", return_value="5"):
            result = player.get_move(moves)

        assert result.error is not None
        assert result.move is None
        assert not result.quit

    def test_handles_non_numeric(self):
        """Returns error for non-numeric input."""
        player = HumanPlayer()
        moves = [LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD)]

        with patch("builtins.input", return_value="xyz"):
            result = player.get_move(moves)

        assert result.error is not None

    def test_handles_empty_moves_pass(self):
        """Returns pass for empty move list."""
        player = HumanPlayer()

        with patch("builtins.input", return_value=""):
            result = player.get_move([])

        assert result.is_pass
        assert not result.quit
