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


class TestApplyBettingMove:
    """Test betting move application."""

    def _make_player(self, chips: int, current_bet: int = 0) -> PlayerState:
        return PlayerState(
            player_id=0,
            hand=(),
            score=0,
            chips=chips,
            current_bet=current_bet,
        )

    def _make_state(self, player: PlayerState, current_bet: int = 0, pot: int = 0) -> "GameState":
        from darwindeck.simulation.state import GameState
        return GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            pot=pot,
            current_bet=current_bet,
        )

    def test_apply_check_no_change(self):
        """CHECK should not change state."""
        from darwindeck.simulation.movegen import apply_betting_move, BettingAction, BettingMove
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500)
        state = self._make_state(player, pot=100)
        phase = BettingPhase(min_bet=10, max_raises=3)
        move = BettingMove(action=BettingAction.CHECK, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].chips == 500
        assert new_state.pot == 100

    def test_apply_bet_updates_chips_and_pot(self):
        """BET should decrease chips, increase pot, set current_bet."""
        from darwindeck.simulation.movegen import apply_betting_move, BettingAction, BettingMove
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500)
        state = self._make_state(player, pot=0, current_bet=0)
        phase = BettingPhase(min_bet=50, max_raises=3)
        move = BettingMove(action=BettingAction.BET, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].chips == 450  # 500 - 50
        assert new_state.players[0].current_bet == 50
        assert new_state.pot == 50
        assert new_state.current_bet == 50

    def test_apply_call_matches_bet(self):
        """CALL should match the current bet."""
        from darwindeck.simulation.movegen import apply_betting_move, BettingAction, BettingMove
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, pot=50, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)
        move = BettingMove(action=BettingAction.CALL, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].chips == 450  # 500 - 50
        assert new_state.players[0].current_bet == 50
        assert new_state.pot == 100  # 50 + 50

    def test_apply_raise_increases_bet(self):
        """RAISE should call and add min_bet."""
        from darwindeck.simulation.movegen import apply_betting_move, BettingAction, BettingMove
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, pot=50, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)
        move = BettingMove(action=BettingAction.RAISE, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].chips == 440  # 500 - 50 - 10
        assert new_state.players[0].current_bet == 60  # 50 + 10
        assert new_state.pot == 110  # 50 + 60
        assert new_state.current_bet == 60
        assert new_state.raise_count == 1

    def test_apply_all_in_bets_all_chips(self):
        """ALL_IN should bet all remaining chips."""
        from darwindeck.simulation.movegen import apply_betting_move, BettingAction, BettingMove
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=30, current_bet=0)
        state = self._make_state(player, pot=50, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)
        move = BettingMove(action=BettingAction.ALL_IN, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].chips == 0
        assert new_state.players[0].current_bet == 30
        assert new_state.players[0].is_all_in is True
        assert new_state.pot == 80  # 50 + 30

    def test_apply_fold_sets_flag(self):
        """FOLD should set has_folded flag."""
        from darwindeck.simulation.movegen import apply_betting_move, BettingAction, BettingMove
        from darwindeck.genome.schema import BettingPhase

        player = self._make_player(chips=500, current_bet=0)
        state = self._make_state(player, pot=50, current_bet=50)
        phase = BettingPhase(min_bet=10, max_raises=3)
        move = BettingMove(action=BettingAction.FOLD, phase_index=0)

        new_state = apply_betting_move(state, move, phase)

        assert new_state.players[0].has_folded is True
        assert new_state.players[0].chips == 500  # Unchanged
        assert new_state.pot == 50  # Unchanged


class TestBettingHelpers:
    """Test betting round helper functions."""

    def test_count_active_players_excludes_folded(self):
        """count_active_players should not count folded players."""
        from darwindeck.simulation.movegen import count_active_players
        from darwindeck.simulation.state import GameState

        p0 = PlayerState(player_id=0, hand=(), score=0, has_folded=False)
        p1 = PlayerState(player_id=1, hand=(), score=0, has_folded=True)
        state = GameState(
            players=(p0, p1),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
        )

        assert count_active_players(state) == 1

    def test_all_bets_matched_when_equal(self):
        """all_bets_matched should return True when all players match."""
        from darwindeck.simulation.movegen import all_bets_matched
        from darwindeck.simulation.state import GameState

        p0 = PlayerState(player_id=0, hand=(), score=0, current_bet=50)
        p1 = PlayerState(player_id=1, hand=(), score=0, current_bet=50)
        state = GameState(
            players=(p0, p1),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            current_bet=50,
        )

        assert all_bets_matched(state) is True

    def test_all_bets_matched_ignores_folded(self):
        """all_bets_matched should ignore folded players."""
        from darwindeck.simulation.movegen import all_bets_matched
        from darwindeck.simulation.state import GameState

        p0 = PlayerState(player_id=0, hand=(), score=0, current_bet=50)
        p1 = PlayerState(player_id=1, hand=(), score=0, current_bet=0, has_folded=True)
        state = GameState(
            players=(p0, p1),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            current_bet=50,
        )

        assert all_bets_matched(state) is True

    def test_all_bets_matched_ignores_all_in(self):
        """all_bets_matched should ignore all-in players."""
        from darwindeck.simulation.movegen import all_bets_matched
        from darwindeck.simulation.state import GameState

        p0 = PlayerState(player_id=0, hand=(), score=0, current_bet=50)
        p1 = PlayerState(player_id=1, hand=(), score=0, current_bet=30, is_all_in=True)
        state = GameState(
            players=(p0, p1),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            current_bet=50,
        )

        assert all_bets_matched(state) is True


class TestSessionBettingInit:
    """Test session initializes betting state."""

    def test_session_initializes_chips(self):
        """Session should initialize player chips from genome."""
        from darwindeck.playtest.session import PlaytestSession, SessionConfig
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase
        )

        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_betting",
            generation=0,
            setup=SetupRules(cards_per_player=2, starting_chips=500),
            turn_structure=TurnStructure(
                phases=(BettingPhase(min_bet=10, max_raises=3),),
            ),
            special_effects=[],
            win_conditions=[WinCondition(type="high_score")],
            scoring_rules=[],
            player_count=2,
        )
        config = SessionConfig(seed=12345)
        session = PlaytestSession(genome, config)

        # Initialize state
        state = session._initialize_state()

        assert state.players[0].chips == 500
        assert state.players[1].chips == 500
        assert state.pot == 0


class TestDisplayBetting:
    """Test display shows betting info."""

    def test_render_shows_chips_when_nonzero(self):
        """StateRenderer should show chips when player has them."""
        from darwindeck.simulation.state import GameState
        from darwindeck.genome.schema import GameGenome, SetupRules, TurnStructure
        from darwindeck.playtest.display import StateRenderer

        player = PlayerState(player_id=0, hand=(), score=0, chips=500)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
            pot=150,
        )
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=2, starting_chips=500),
            turn_structure=TurnStructure(phases=()),
            special_effects=[],
            win_conditions=(),
            scoring_rules=(),
            player_count=1,
        )
        renderer = StateRenderer()

        output = renderer.render(state, genome, player_idx=0, debug=False)

        assert "500" in output  # Chips shown
        assert "150" in output  # Pot shown