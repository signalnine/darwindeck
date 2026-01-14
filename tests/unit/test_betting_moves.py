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


class TestBettingTypes:
    """Test BettingAction and BettingMove types."""

    def test_betting_action_enum_values(self):
        """BettingAction should have all poker actions."""
        from darwindeck.simulation.movegen import BettingAction

        assert BettingAction.CHECK.value == "check"
        assert BettingAction.BET.value == "bet"
        assert BettingAction.CALL.value == "call"
        assert BettingAction.RAISE.value == "raise"
        assert BettingAction.ALL_IN.value == "all_in"
        assert BettingAction.FOLD.value == "fold"

    def test_betting_move_dataclass(self):
        """BettingMove should hold action and phase_index."""
        from darwindeck.simulation.movegen import BettingAction, BettingMove

        move = BettingMove(action=BettingAction.BET, phase_index=0)
        assert move.action == BettingAction.BET
        assert move.phase_index == 0


class TestGenerateBettingMoves:
    """Test betting move generation."""

    def _make_player(self, chips: int, current_bet: int = 0, has_folded: bool = False, is_all_in: bool = False) -> PlayerState:
        return PlayerState(
            player_id=0,
            hand=(),
            score=0,
            chips=chips,
            current_bet=current_bet,
            has_folded=has_folded,
            is_all_in=is_all_in,
        )

    def _make_state(self, player: PlayerState, current_bet: int = 0, pot: int = 0, raise_count: int = 0) -> "GameState":
        from darwindeck.simulation.state import GameState
        return GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            pot=pot,
            current_bet=current_bet,
            raise_count=raise_count,
        )

    def test_check_available_when_no_bet(self):
        """CHECK should be available when there's no current bet."""
        from darwindeck.simulation.movegen import generate_betting_moves, BettingAction
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=0)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.CHECK in actions

    def test_bet_available_when_can_afford(self):
        """BET should be available when player can afford min_bet."""
        from darwindeck.simulation.movegen import generate_betting_moves, BettingAction
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=0)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.BET in actions

    def test_bet_not_available_when_cannot_afford(self):
        """BET should not be available when player can't afford min_bet."""
        from darwindeck.simulation.movegen import generate_betting_moves, BettingAction
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=5, current_bet=0)  # Less than min_bet
        state = self._make_state(player, current_bet=0)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.BET not in actions
        assert BettingAction.ALL_IN in actions  # Can still go all-in

    def test_call_available_when_facing_bet(self):
        """CALL should be available when there's a bet to match."""
        from darwindeck.simulation.movegen import generate_betting_moves, BettingAction
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.CALL in actions

    def test_raise_available_when_can_afford(self):
        """RAISE should be available when player can afford call + min_bet."""
        from darwindeck.simulation.movegen import generate_betting_moves, BettingAction
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=50, raise_count=0)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.RAISE in actions

    def test_raise_not_available_at_max_raises(self):
        """RAISE should not be available when max_raises reached."""
        from darwindeck.simulation.movegen import generate_betting_moves, BettingAction
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=50, raise_count=3)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.RAISE not in actions

    def test_fold_available_when_facing_bet(self):
        """FOLD should be available when there's a bet to match."""
        from darwindeck.simulation.movegen import generate_betting_moves, BettingAction
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.FOLD in actions

    def test_all_in_when_short_stacked(self):
        """ALL_IN should be available when can't afford call."""
        from darwindeck.simulation.movegen import generate_betting_moves, BettingAction
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=30, current_bet=0)  # Can't afford 50 call
        state = self._make_state(player, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)

        moves = generate_betting_moves(state, phase, player_id=0)
        actions = [m.action for m in moves]

        assert BettingAction.ALL_IN in actions
        assert BettingAction.CALL not in actions  # Can't afford
