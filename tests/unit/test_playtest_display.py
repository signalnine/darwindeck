"""Tests for display components."""

import pytest
from darwindeck.playtest.display import StateRenderer, format_card
from darwindeck.simulation.state import GameState, PlayerState, Card
from darwindeck.genome.schema import (
    Rank, Suit, Location, GameGenome, SetupRules,
    TurnStructure, WinCondition, PlayPhase
)


def make_card(rank: str, suit: str) -> Card:
    """Helper to create cards."""
    return Card(rank=Rank(rank), suit=Suit(suit))


def make_simple_genome() -> GameGenome:
    """Create a simple test genome."""
    return GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(cards_per_player=5),
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


def make_state_with_hand(hand: list[tuple[str, str]]) -> GameState:
    """Create state with specific hand."""
    cards = tuple(make_card(r, s) for r, s in hand)
    players = (
        PlayerState(player_id=0, hand=cards, score=0),
        PlayerState(player_id=1, hand=(make_card("A", "H"),), score=0),
    )
    return GameState(
        players=players,
        deck=(),
        discard=(make_card("Q", "H"),),
        turn=1,
        active_player=0,
    )


class TestStateRenderer:
    """Tests for StateRenderer."""

    def test_renders_hand(self):
        """Renders player's hand."""
        renderer = StateRenderer()
        state = make_state_with_hand([("7", "S"), ("K", "H"), ("3", "D")])
        genome = make_simple_genome()

        output = renderer.render(state, genome, player_idx=0)

        assert "7" in output and "♠" in output
        assert "K" in output and "♥" in output
        assert "3" in output and "♦" in output

    def test_renders_discard(self):
        """Renders discard pile top card."""
        renderer = StateRenderer()
        state = make_state_with_hand([("A", "C")])
        genome = make_simple_genome()

        output = renderer.render(state, genome, player_idx=0)

        assert "Q" in output and "♥" in output

    def test_debug_shows_opponent_hand(self):
        """Debug mode shows opponent's hand."""
        renderer = StateRenderer()
        state = make_state_with_hand([("A", "C")])
        genome = make_simple_genome()

        normal = renderer.render(state, genome, player_idx=0, debug=False)
        debug = renderer.render(state, genome, player_idx=0, debug=True)

        # Debug should be longer and contain opponent info
        assert len(debug) > len(normal)
        assert "opponent" in debug.lower() or "player 1" in debug.lower()

    def test_renders_turn_number(self):
        """Shows current turn number."""
        renderer = StateRenderer()
        state = make_state_with_hand([("A", "C")])
        state = state.copy_with(turn=15)
        genome = make_simple_genome()

        output = renderer.render(state, genome, player_idx=0)

        assert "15" in output
