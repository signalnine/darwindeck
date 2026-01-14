"""Tests for betting move generation and application."""

import pytest
from darwindeck.simulation.state import PlayerState, Card
from darwindeck.genome.schema import Rank, Suit


class TestPlayerStateBetting:
    """Test PlayerState betting fields."""

    def test_player_state_has_chips(self):
        """PlayerState should have chips field."""
        player = PlayerState(
            player_id=0,
            hand=(),
            score=0,
            chips=500,
        )
        assert player.chips == 500

    def test_player_state_has_betting_flags(self):
        """PlayerState should have current_bet, has_folded, is_all_in."""
        player = PlayerState(
            player_id=0,
            hand=(),
            score=0,
            chips=500,
            current_bet=50,
            has_folded=False,
            is_all_in=False,
        )
        assert player.current_bet == 50
        assert player.has_folded is False
        assert player.is_all_in is False

    def test_player_state_betting_fields_default_to_zero(self):
        """Betting fields should default to 0/False for non-betting games."""
        player = PlayerState(player_id=0, hand=(), score=0)
        assert player.chips == 0
        assert player.current_bet == 0
        assert player.has_folded is False
        assert player.is_all_in is False


class TestGameStateBetting:
    """Test GameState betting fields."""

    def test_game_state_has_pot(self):
        """GameState should have pot field."""
        from darwindeck.simulation.state import GameState

        player = PlayerState(player_id=0, hand=(), score=0)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            pot=150,
        )
        assert state.pot == 150

    def test_game_state_has_betting_fields(self):
        """GameState should have current_bet and raise_count."""
        from darwindeck.simulation.state import GameState

        player = PlayerState(player_id=0, hand=(), score=0)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            pot=150,
            current_bet=50,
            raise_count=1,
        )
        assert state.current_bet == 50
        assert state.raise_count == 1

    def test_game_state_betting_fields_default_to_zero(self):
        """Betting fields should default to 0 for non-betting games."""
        from darwindeck.simulation.state import GameState

        player = PlayerState(player_id=0, hand=(), score=0)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
        )
        assert state.pot == 0
        assert state.current_bet == 0
        assert state.raise_count == 0
