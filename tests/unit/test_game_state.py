"""Tests for immutable game state."""

import pytest
from darwindeck.simulation.state import GameState, PlayerState, Card
from darwindeck.genome.schema import Rank, Suit


def test_card_immutability() -> None:
    """Test Card is immutable."""
    card = Card(rank=Rank.ACE, suit=Suit.HEARTS)
    assert card.rank == Rank.ACE

    with pytest.raises(AttributeError):
        card.rank = Rank.KING  # type: ignore


def test_player_state_immutability() -> None:
    """Test PlayerState is immutable."""
    player = PlayerState(
        player_id=0,
        hand=(Card(Rank.ACE, Suit.HEARTS),),
        score=0
    )
    assert len(player.hand) == 1

    with pytest.raises(AttributeError):
        player.score = 10  # type: ignore


def test_game_state_immutability() -> None:
    """Test GameState is immutable."""
    state = GameState(
        players=(
            PlayerState(0, (), 0),
            PlayerState(1, (), 0),
        ),
        deck=(),
        discard=(),
        turn=0,
        active_player=0
    )

    with pytest.raises(AttributeError):
        state.turn = 1  # type: ignore


def test_game_state_nested_tuples() -> None:
    """Test GameState uses tuples (not lists) for nested structures."""
    state = GameState(
        players=(PlayerState(0, (), 0),),
        deck=(Card(Rank.ACE, Suit.HEARTS),),
        discard=(),
        turn=0,
        active_player=0
    )

    assert isinstance(state.deck, tuple)
    assert isinstance(state.players, tuple)
