"""Tests for new effect types in movegen.py.

Tests WILD_CARD, BLOCK_NEXT, SWAP_HANDS, STEAL_CARD, and PEEK_HAND effects.
"""

import pytest
from darwindeck.simulation.state import GameState, PlayerState, Card
from darwindeck.simulation.movegen import (
    apply_effect,
    resolve_target,
    EffectType,
    TargetSelector,
)
from darwindeck.genome.schema import Rank, Suit, SpecialEffect


def create_test_state(num_players: int = 2) -> GameState:
    """Create a simple game state for testing."""
    players = tuple(
        PlayerState(
            player_id=i,
            hand=(
                Card(Rank.ACE, Suit.HEARTS),
                Card(Rank.KING, Suit.SPADES),
                Card(Rank.QUEEN, Suit.DIAMONDS),
            ),
            score=0,
        )
        for i in range(num_players)
    )
    deck = (
        Card(Rank.TWO, Suit.CLUBS),
        Card(Rank.THREE, Suit.HEARTS),
        Card(Rank.FOUR, Suit.SPADES),
    )
    return GameState(
        players=players,
        deck=deck,
        discard=(),
        turn=0,
        active_player=0,
    )


class TestResolveTarget:
    """Test target resolution for effects."""

    def test_resolve_next_player(self) -> None:
        """NEXT_PLAYER target resolves to next player in turn order."""
        state = create_test_state(num_players=3)
        target_id = resolve_target(state, TargetSelector.NEXT_PLAYER)
        assert target_id == 1

    def test_resolve_next_player_wraps(self) -> None:
        """NEXT_PLAYER wraps around to player 0."""
        state = create_test_state(num_players=3)
        state = state.copy_with(active_player=2)
        target_id = resolve_target(state, TargetSelector.NEXT_PLAYER)
        assert target_id == 0

    def test_resolve_prev_player(self) -> None:
        """PREV_PLAYER target resolves to previous player."""
        state = create_test_state(num_players=3)
        state = state.copy_with(active_player=1)
        target_id = resolve_target(state, TargetSelector.PREV_PLAYER)
        assert target_id == 0

    def test_resolve_prev_player_wraps(self) -> None:
        """PREV_PLAYER wraps around to last player."""
        state = create_test_state(num_players=3)
        target_id = resolve_target(state, TargetSelector.PREV_PLAYER)
        assert target_id == 2

    def test_resolve_self(self) -> None:
        """SELF target returns active player."""
        state = create_test_state(num_players=3)
        state = state.copy_with(active_player=1)
        target_id = resolve_target(state, TargetSelector.SELF)
        assert target_id == 1


class TestWildCardEffect:
    """Test WILD_CARD effect.

    WILD_CARD marks a card as wild for matching purposes.
    In simulation, this is typically a no-op (wild matching is handled
    in condition evaluation, not as a state mutation).
    """

    def test_wild_card_is_no_op(self) -> None:
        """WILD_CARD effect doesn't change state (handled by condition eval)."""
        state = create_test_state()
        effect = SpecialEffect(
            trigger_rank=Rank.EIGHT,
            effect_type=EffectType.WILD_CARD,
            target=TargetSelector.SELF,
            value=1,
        )
        new_state = apply_effect(state, effect)
        # State should be unchanged
        assert new_state.players == state.players
        assert new_state.deck == state.deck


class TestBlockNextEffect:
    """Test BLOCK_NEXT effect.

    BLOCK_NEXT is similar to SKIP_NEXT - it blocks the next player from acting.
    """

    def test_block_next_sets_skip_count(self) -> None:
        """BLOCK_NEXT increases skip count to skip next player."""
        state = create_test_state(num_players=3)
        effect = SpecialEffect(
            trigger_rank=Rank.JACK,
            effect_type=EffectType.BLOCK_NEXT,
            target=TargetSelector.NEXT_PLAYER,
            value=1,
        )
        new_state = apply_effect(state, effect)
        assert new_state.skip_count == 1

    def test_block_next_capped_at_num_players_minus_one(self) -> None:
        """BLOCK_NEXT skip count capped to prevent infinite turns."""
        state = create_test_state(num_players=3)
        state = state.copy_with(skip_count=2)
        effect = SpecialEffect(
            trigger_rank=Rank.JACK,
            effect_type=EffectType.BLOCK_NEXT,
            target=TargetSelector.NEXT_PLAYER,
            value=5,
        )
        new_state = apply_effect(state, effect)
        # Cap at num_players - 1 = 2
        assert new_state.skip_count == 2


class TestSwapHandsEffect:
    """Test SWAP_HANDS effect.

    SWAP_HANDS exchanges hands between active player and target.
    """

    def test_swap_hands_exchanges_cards(self) -> None:
        """SWAP_HANDS swaps hands between active player and target."""
        # Create state with distinct hands
        players = (
            PlayerState(
                player_id=0,
                hand=(Card(Rank.ACE, Suit.HEARTS), Card(Rank.KING, Suit.HEARTS)),
                score=0,
            ),
            PlayerState(
                player_id=1,
                hand=(Card(Rank.TWO, Suit.CLUBS), Card(Rank.THREE, Suit.CLUBS), Card(Rank.FOUR, Suit.CLUBS)),
                score=0,
            ),
        )
        state = GameState(
            players=players,
            deck=(),
            discard=(),
            turn=0,
            active_player=0,
        )

        effect = SpecialEffect(
            trigger_rank=Rank.QUEEN,
            effect_type=EffectType.SWAP_HANDS,
            target=TargetSelector.NEXT_PLAYER,
            value=1,
        )
        new_state = apply_effect(state, effect)

        # Player 0 should now have player 1's original hand
        assert len(new_state.players[0].hand) == 3
        assert new_state.players[0].hand[0].rank == Rank.TWO

        # Player 1 should now have player 0's original hand
        assert len(new_state.players[1].hand) == 2
        assert new_state.players[1].hand[0].rank == Rank.ACE

    def test_swap_hands_with_self_no_change(self) -> None:
        """SWAP_HANDS with SELF target is no-op."""
        state = create_test_state()
        original_hand = state.players[0].hand

        effect = SpecialEffect(
            trigger_rank=Rank.QUEEN,
            effect_type=EffectType.SWAP_HANDS,
            target=TargetSelector.SELF,
            value=1,
        )
        new_state = apply_effect(state, effect)

        # Hand should be unchanged when swapping with self
        assert new_state.players[0].hand == original_hand


class TestStealCardEffect:
    """Test STEAL_CARD effect.

    STEAL_CARD takes one card from target's hand (deterministically, from end).
    """

    def test_steal_card_takes_from_target(self) -> None:
        """STEAL_CARD transfers one card from target to active player."""
        players = (
            PlayerState(
                player_id=0,
                hand=(Card(Rank.ACE, Suit.HEARTS),),
                score=0,
            ),
            PlayerState(
                player_id=1,
                hand=(Card(Rank.TWO, Suit.CLUBS), Card(Rank.THREE, Suit.DIAMONDS)),
                score=0,
            ),
        )
        state = GameState(
            players=players,
            deck=(),
            discard=(),
            turn=0,
            active_player=0,
        )

        effect = SpecialEffect(
            trigger_rank=Rank.KING,
            effect_type=EffectType.STEAL_CARD,
            target=TargetSelector.NEXT_PLAYER,
            value=1,
        )
        new_state = apply_effect(state, effect)

        # Player 0 should have gained a card (now 2 cards)
        assert len(new_state.players[0].hand) == 2
        # The stolen card should be the last from target's hand (deterministic)
        assert new_state.players[0].hand[-1].rank == Rank.THREE

        # Player 1 should have lost a card (now 1 card)
        assert len(new_state.players[1].hand) == 1
        assert new_state.players[1].hand[0].rank == Rank.TWO

    def test_steal_card_from_empty_hand_no_op(self) -> None:
        """STEAL_CARD from empty hand is a no-op."""
        players = (
            PlayerState(
                player_id=0,
                hand=(Card(Rank.ACE, Suit.HEARTS),),
                score=0,
            ),
            PlayerState(
                player_id=1,
                hand=(),  # Empty hand
                score=0,
            ),
        )
        state = GameState(
            players=players,
            deck=(),
            discard=(),
            turn=0,
            active_player=0,
        )

        effect = SpecialEffect(
            trigger_rank=Rank.KING,
            effect_type=EffectType.STEAL_CARD,
            target=TargetSelector.NEXT_PLAYER,
            value=1,
        )
        new_state = apply_effect(state, effect)

        # No change when target has no cards
        assert len(new_state.players[0].hand) == 1
        assert len(new_state.players[1].hand) == 0

    def test_steal_multiple_cards(self) -> None:
        """STEAL_CARD with value > 1 steals multiple cards."""
        players = (
            PlayerState(
                player_id=0,
                hand=(),
                score=0,
            ),
            PlayerState(
                player_id=1,
                hand=(
                    Card(Rank.TWO, Suit.CLUBS),
                    Card(Rank.THREE, Suit.DIAMONDS),
                    Card(Rank.FOUR, Suit.HEARTS),
                ),
                score=0,
            ),
        )
        state = GameState(
            players=players,
            deck=(),
            discard=(),
            turn=0,
            active_player=0,
        )

        effect = SpecialEffect(
            trigger_rank=Rank.KING,
            effect_type=EffectType.STEAL_CARD,
            target=TargetSelector.NEXT_PLAYER,
            value=2,
        )
        new_state = apply_effect(state, effect)

        # Player 0 should have 2 cards
        assert len(new_state.players[0].hand) == 2
        # Player 1 should have 1 card left
        assert len(new_state.players[1].hand) == 1


class TestPeekHandEffect:
    """Test PEEK_HAND effect.

    PEEK_HAND is purely informational (UI-only) and has no mechanical effect
    on the game state in simulation.
    """

    def test_peek_hand_is_no_op(self) -> None:
        """PEEK_HAND doesn't change state (UI-only effect)."""
        state = create_test_state()
        effect = SpecialEffect(
            trigger_rank=Rank.TEN,
            effect_type=EffectType.PEEK_HAND,
            target=TargetSelector.NEXT_PLAYER,
            value=1,
        )
        new_state = apply_effect(state, effect)
        # State should be completely unchanged
        assert new_state.players == state.players
        assert new_state.deck == state.deck
        assert new_state.discard == state.discard


class TestExistingEffects:
    """Test that existing effect types still work after adding new ones."""

    def test_skip_next_effect(self) -> None:
        """SKIP_NEXT increases skip count."""
        state = create_test_state(num_players=3)
        effect = SpecialEffect(
            trigger_rank=Rank.JACK,
            effect_type=EffectType.SKIP_NEXT,
            target=TargetSelector.NEXT_PLAYER,
            value=1,
        )
        new_state = apply_effect(state, effect)
        assert new_state.skip_count == 1

    def test_reverse_direction_effect(self) -> None:
        """REVERSE_DIRECTION flips play direction."""
        state = create_test_state(num_players=3)
        effect = SpecialEffect(
            trigger_rank=Rank.QUEEN,
            effect_type=EffectType.REVERSE_DIRECTION,
            target=TargetSelector.NEXT_PLAYER,
            value=1,
        )
        new_state = apply_effect(state, effect)
        assert new_state.play_direction == -1

        # Reverse again should go back to 1
        new_state = apply_effect(new_state, effect)
        assert new_state.play_direction == 1

    def test_draw_cards_effect(self) -> None:
        """DRAW_CARDS makes target draw cards."""
        state = create_test_state()
        initial_hand_size = len(state.players[1].hand)
        initial_deck_size = len(state.deck)

        effect = SpecialEffect(
            trigger_rank=Rank.TWO,
            effect_type=EffectType.DRAW_CARDS,
            target=TargetSelector.NEXT_PLAYER,
            value=2,
        )
        new_state = apply_effect(state, effect)

        assert len(new_state.players[1].hand) == initial_hand_size + 2
        assert len(new_state.deck) == initial_deck_size - 2

    def test_extra_turn_effect(self) -> None:
        """EXTRA_TURN sets skip to skip all other players."""
        state = create_test_state(num_players=4)
        effect = SpecialEffect(
            trigger_rank=Rank.KING,
            effect_type=EffectType.EXTRA_TURN,
            target=TargetSelector.SELF,
            value=1,
        )
        new_state = apply_effect(state, effect)
        # Should skip all other players (num_players - 1)
        assert new_state.skip_count == 3

    def test_force_discard_effect(self) -> None:
        """FORCE_DISCARD makes target discard cards."""
        state = create_test_state()
        initial_hand_size = len(state.players[1].hand)

        effect = SpecialEffect(
            trigger_rank=Rank.SIX,
            effect_type=EffectType.FORCE_DISCARD,
            target=TargetSelector.NEXT_PLAYER,
            value=2,
        )
        new_state = apply_effect(state, effect)

        assert len(new_state.players[1].hand) == initial_hand_size - 2
        assert len(new_state.discard) == 2
