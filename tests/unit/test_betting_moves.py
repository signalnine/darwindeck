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

    def test_presenter_shows_betting_options(self):
        """MovePresenter should display betting actions by name."""
        from darwindeck.simulation.state import GameState
        from darwindeck.simulation.movegen import BettingMove, BettingAction
        from darwindeck.genome.schema import GameGenome, SetupRules, TurnStructure, BettingPhase
        from darwindeck.playtest.display import MovePresenter

        player = PlayerState(player_id=0, hand=(), score=0, chips=500)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
        )
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=2, starting_chips=500),
            turn_structure=TurnStructure(phases=(BettingPhase(min_bet=10, max_raises=3),)),
            special_effects=[],
            win_conditions=(),
            scoring_rules=(),
            player_count=1,
        )
        moves = [
            BettingMove(action=BettingAction.CHECK, phase_index=0),
            BettingMove(action=BettingAction.BET, phase_index=0),
        ]
        presenter = MovePresenter()

        output = presenter.present(moves, state, genome)

        assert "Check" in output
        assert "Bet" in output
        assert "[1]" in output
        assert "[2]" in output


class TestAIBettingStrategy:
    """Test AI betting strategy based on hand strength."""

    def test_ai_evaluates_hand_strength(self):
        """AI should calculate hand strength between 0 and 1."""
        from darwindeck.playtest.session import PlaytestSession, SessionConfig
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase
        )
        from darwindeck.simulation.state import Card
        from darwindeck.genome.schema import Rank, Suit

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
        session.state = session._initialize_state()

        # Evaluate for player 1 (AI)
        strength = session._evaluate_hand_strength(player_id=1)

        assert 0.0 <= strength <= 1.0

    def test_ai_raises_with_strong_hand(self):
        """AI with strong hand should prefer RAISE when available."""
        from darwindeck.playtest.session import PlaytestSession, SessionConfig
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase, Rank, Suit
        )
        from darwindeck.simulation.state import Card
        from darwindeck.simulation.movegen import BettingMove, BettingAction

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
        session.state = session._initialize_state()

        # Give AI a strong hand (Aces and Kings)
        ai_player = 1 - session.human_player_idx
        strong_hand = (
            Card(rank=Rank.ACE, suit=Suit.SPADES),
            Card(rank=Rank.KING, suit=Suit.HEARTS),
        )
        new_player = session.state.players[ai_player].copy_with(hand=strong_hand)
        new_players = tuple(
            new_player if i == ai_player else p
            for i, p in enumerate(session.state.players)
        )
        session.state = session.state.copy_with(players=new_players)

        # Provide betting moves including RAISE
        moves = [
            BettingMove(action=BettingAction.CHECK, phase_index=0),
            BettingMove(action=BettingAction.BET, phase_index=0),
            BettingMove(action=BettingAction.RAISE, phase_index=0),
        ]

        selected = session._ai_betting_select(moves)

        # Strong hand should prefer RAISE
        assert selected.action == BettingAction.RAISE

    def test_ai_checks_with_weak_hand(self):
        """AI with weak hand should prefer CHECK when available."""
        from darwindeck.playtest.session import PlaytestSession, SessionConfig
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase, Rank, Suit
        )
        from darwindeck.simulation.state import Card
        from darwindeck.simulation.movegen import BettingMove, BettingAction

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
        session.state = session._initialize_state()

        # Give AI a weak hand (2s and 3s)
        ai_player = 1 - session.human_player_idx
        weak_hand = (
            Card(rank=Rank.TWO, suit=Suit.SPADES),
            Card(rank=Rank.THREE, suit=Suit.HEARTS),
        )
        new_player = session.state.players[ai_player].copy_with(hand=weak_hand)
        new_players = tuple(
            new_player if i == ai_player else p
            for i, p in enumerate(session.state.players)
        )
        session.state = session.state.copy_with(players=new_players)

        # Provide betting moves
        moves = [
            BettingMove(action=BettingAction.CHECK, phase_index=0),
            BettingMove(action=BettingAction.BET, phase_index=0),
        ]

        selected = session._ai_betting_select(moves)

        # Weak hand should prefer CHECK
        assert selected.action == BettingAction.CHECK


class TestMixedMovePresentation:
    """Test presentation of mixed LegalMove and BettingMove lists."""

    def test_presenter_handles_mixed_move_types(self):
        """MovePresenter should handle genomes with both PlayPhase and BettingPhase."""
        from darwindeck.simulation.state import GameState
        from darwindeck.simulation.movegen import LegalMove, BettingMove, BettingAction
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, BettingPhase, PlayPhase, Location
        )
        from darwindeck.playtest.display import MovePresenter
        from darwindeck.simulation.state import Card
        from darwindeck.genome.schema import Rank, Suit

        hand = (
            Card(rank=Rank.ACE, suit=Suit.SPADES),
            Card(rank=Rank.KING, suit=Suit.HEARTS),
        )
        player = PlayerState(player_id=0, hand=hand, score=0, chips=500)
        state = GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
        )
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=2, starting_chips=500),
            turn_structure=TurnStructure(phases=(
                PlayPhase(min_cards=1, max_cards=1, target=Location.TABLEAU),
                BettingPhase(min_bet=10, max_raises=3),
            )),
            special_effects=[],
            win_conditions=(),
            scoring_rules=(),
            player_count=1,
        )

        # Mix of card and betting moves
        moves = [
            LegalMove(phase_index=0, card_index=0, target_loc=Location.TABLEAU),
            LegalMove(phase_index=0, card_index=1, target_loc=Location.TABLEAU),
            BettingMove(action=BettingAction.CHECK, phase_index=1),
            BettingMove(action=BettingAction.BET, phase_index=1),
        ]

        presenter = MovePresenter()
        output = presenter.present(moves, state, genome)

        # Should show both card plays and betting options
        assert "Play:" in output
        assert "Bet:" in output
        # Card play indices
        assert "[1]" in output  # First card
        assert "[2]" in output  # Second card
        # Betting indices (offset by 2)
        assert "[3]" in output  # Check
        assert "[4]" in output  # Bet


class TestDiscardPhaseMovegen:
    """Test DiscardPhase move generation."""

    def _make_state(self, hand: tuple) -> "GameState":
        from darwindeck.simulation.state import GameState
        player = PlayerState(player_id=0, hand=hand, score=0)
        return GameState(
            players=(player,),
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
        )

    def test_generates_discard_moves_for_each_card(self):
        """DiscardPhase should generate a move for each card in hand."""
        from darwindeck.simulation.movegen import generate_legal_moves, LegalMove
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, DiscardPhase, Location
        )
        from darwindeck.simulation.state import Card

        hand = (
            Card(rank=Rank.ACE, suit=Suit.SPADES),
            Card(rank=Rank.KING, suit=Suit.HEARTS),
            Card(rank=Rank.QUEEN, suit=Suit.DIAMONDS),
        )
        state = self._make_state(hand)
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_discard",
            generation=0,
            setup=SetupRules(cards_per_player=3),
            turn_structure=TurnStructure(phases=[
                DiscardPhase(target=Location.DISCARD, mandatory=True)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=1,
        )

        moves = generate_legal_moves(state, genome)

        # Should have one move per card
        assert len(moves) == 3
        card_indices = [m.card_index for m in moves]
        assert card_indices == [0, 1, 2]

    def test_generates_pass_move_when_not_mandatory(self):
        """Non-mandatory DiscardPhase should include pass option."""
        from darwindeck.simulation.movegen import generate_legal_moves, LegalMove
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, DiscardPhase, Location
        )
        from darwindeck.simulation.state import Card

        hand = (Card(rank=Rank.ACE, suit=Suit.SPADES),)
        state = self._make_state(hand)
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_discard",
            generation=0,
            setup=SetupRules(cards_per_player=1),
            turn_structure=TurnStructure(phases=[
                DiscardPhase(target=Location.DISCARD, mandatory=False)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=1,
        )

        moves = generate_legal_moves(state, genome)

        # Should have discard move + pass move
        assert len(moves) == 2
        card_indices = [m.card_index for m in moves]
        assert 0 in card_indices  # Discard first card
        assert -1 in card_indices  # Pass

    def test_no_pass_when_mandatory(self):
        """Mandatory DiscardPhase should not include pass option."""
        from darwindeck.simulation.movegen import generate_legal_moves, LegalMove
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, DiscardPhase, Location
        )
        from darwindeck.simulation.state import Card

        hand = (Card(rank=Rank.ACE, suit=Suit.SPADES),)
        state = self._make_state(hand)
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_discard",
            generation=0,
            setup=SetupRules(cards_per_player=1),
            turn_structure=TurnStructure(phases=[
                DiscardPhase(target=Location.DISCARD, mandatory=True)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=1,
        )

        moves = generate_legal_moves(state, genome)

        # Should only have discard move, no pass
        assert len(moves) == 1
        assert moves[0].card_index == 0

    def test_apply_discard_move_removes_card(self):
        """Applying discard move should remove card from hand."""
        from darwindeck.simulation.movegen import generate_legal_moves, apply_move, LegalMove
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, DiscardPhase, Location
        )
        from darwindeck.simulation.state import Card

        hand = (
            Card(rank=Rank.ACE, suit=Suit.SPADES),
            Card(rank=Rank.KING, suit=Suit.HEARTS),
        )
        state = self._make_state(hand)
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_discard",
            generation=0,
            setup=SetupRules(cards_per_player=2),
            turn_structure=TurnStructure(phases=[
                DiscardPhase(target=Location.DISCARD, mandatory=True)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=1,
        )

        move = LegalMove(phase_index=0, card_index=0, target_loc=Location.DISCARD)
        new_state = apply_move(state, move, genome)

        # Hand should have one less card
        assert len(new_state.players[0].hand) == 1
        # The Ace should be gone, King remains
        assert new_state.players[0].hand[0].rank == Rank.KING
        # Discard pile should have the Ace
        assert len(new_state.discard) == 1
        assert new_state.discard[0].rank == Rank.ACE

    def test_apply_pass_move_no_change(self):
        """Applying pass move should not change hand."""
        from darwindeck.simulation.movegen import apply_move, LegalMove
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, DiscardPhase, Location
        )
        from darwindeck.simulation.state import Card

        hand = (Card(rank=Rank.ACE, suit=Suit.SPADES),)
        state = self._make_state(hand)
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_discard",
            generation=0,
            setup=SetupRules(cards_per_player=1),
            turn_structure=TurnStructure(phases=[
                DiscardPhase(target=Location.DISCARD, mandatory=False)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=1,
        )

        # Pass move
        move = LegalMove(phase_index=0, card_index=-1, target_loc=Location.DISCARD)
        new_state = apply_move(state, move, genome)

        # Hand should be unchanged
        assert len(new_state.players[0].hand) == 1
        assert new_state.players[0].hand[0].rank == Rank.ACE
        # Discard pile should be empty
        assert len(new_state.discard) == 0


class TestTrickPhaseMovegen:
    """Test TrickPhase move generation."""

    def _make_state(self, hands: list[tuple]) -> "GameState":
        from darwindeck.simulation.state import GameState
        from darwindeck.simulation.state import Card

        players = tuple(
            PlayerState(player_id=i, hand=tuple(Card(rank=Rank(r), suit=Suit(s)) for r, s in hand), score=0)
            for i, hand in enumerate(hands)
        )
        return GameState(
            players=players,
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
        )

    def test_leading_can_play_any_card(self):
        """When leading, player can play any card."""
        from darwindeck.simulation.movegen import generate_legal_moves
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, TrickPhase
        )

        state = self._make_state([[("A", "S"), ("K", "H"), ("Q", "D")], [("J", "C")]])
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_trick",
            generation=0,
            setup=SetupRules(cards_per_player=3),
            turn_structure=TurnStructure(phases=[TrickPhase()]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
        )

        moves = generate_legal_moves(state, genome)

        # Should have 3 moves (one per card)
        assert len(moves) == 3

    def test_following_must_follow_suit(self):
        """When following, must play card of lead suit if able."""
        from darwindeck.simulation.movegen import generate_legal_moves
        from darwindeck.simulation.state import TrickCard
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, TrickPhase
        )
        from darwindeck.simulation.state import Card

        # Player 1's turn, player 0 led with Ace of Spades
        state = self._make_state([[("A", "S")], [("K", "S"), ("Q", "H"), ("J", "D")]])
        state = state.copy_with(
            active_player=1,
            current_trick=(TrickCard(player_id=0, card=Card(rank=Rank.ACE, suit=Suit.SPADES)),),
        )
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_trick",
            generation=0,
            setup=SetupRules(cards_per_player=3),
            turn_structure=TurnStructure(phases=[TrickPhase(lead_suit_required=True)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
        )

        moves = generate_legal_moves(state, genome)

        # Should only have 1 move (King of Spades) - must follow suit
        assert len(moves) == 1
        assert moves[0].card_index == 0  # K♠

    def test_following_can_play_any_when_void(self):
        """When void in lead suit, can play any card."""
        from darwindeck.simulation.movegen import generate_legal_moves
        from darwindeck.simulation.state import TrickCard
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, TrickPhase
        )
        from darwindeck.simulation.state import Card

        # Player 1's turn, no spades in hand
        state = self._make_state([[("A", "S")], [("K", "H"), ("Q", "D"), ("J", "C")]])
        state = state.copy_with(
            active_player=1,
            current_trick=(TrickCard(player_id=0, card=Card(rank=Rank.ACE, suit=Suit.SPADES)),),
        )
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_trick",
            generation=0,
            setup=SetupRules(cards_per_player=3),
            turn_structure=TurnStructure(phases=[TrickPhase(lead_suit_required=True)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
        )

        moves = generate_legal_moves(state, genome)

        # Can play any of the 3 cards (void in spades)
        assert len(moves) == 3


class TestDrawPhaseMovegen:
    """Test DrawPhase move generation."""

    def _make_state(self) -> "GameState":
        from darwindeck.simulation.state import GameState, Card

        players = (
            PlayerState(player_id=0, hand=(), score=0),
            PlayerState(player_id=1, hand=(Card(rank=Rank.ACE, suit=Suit.SPADES),), score=0),
        )
        deck = (Card(rank=Rank.KING, suit=Suit.HEARTS),)
        return GameState(
            players=players,
            deck=deck,
            discard=(),
            turn=1,
            active_player=0,
        )

    def test_generates_draw_move_when_deck_has_cards(self):
        """DrawPhase generates draw move when deck is not empty."""
        from darwindeck.simulation.movegen import generate_legal_moves, MOVE_DRAW
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, DrawPhase, Location
        )

        state = self._make_state()
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_draw",
            generation=0,
            setup=SetupRules(cards_per_player=0),
            turn_structure=TurnStructure(phases=[
                DrawPhase(source=Location.DECK, mandatory=True)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
        )

        moves = generate_legal_moves(state, genome)

        # Should have 1 move (draw)
        assert len(moves) == 1
        assert moves[0].card_index == MOVE_DRAW

    def test_generates_pass_when_not_mandatory(self):
        """Non-mandatory DrawPhase includes pass/stand option."""
        from darwindeck.simulation.movegen import generate_legal_moves, MOVE_DRAW, MOVE_DRAW_PASS
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, DrawPhase, Location
        )

        state = self._make_state()
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_draw",
            generation=0,
            setup=SetupRules(cards_per_player=0),
            turn_structure=TurnStructure(phases=[
                DrawPhase(source=Location.DECK, mandatory=False)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
        )

        moves = generate_legal_moves(state, genome)

        # Should have 2 moves (draw + stand)
        assert len(moves) == 2
        card_indices = [m.card_index for m in moves]
        assert MOVE_DRAW in card_indices
        assert MOVE_DRAW_PASS in card_indices

    def test_apply_draw_adds_card_to_hand(self):
        """Applying draw move adds card to player's hand."""
        from darwindeck.simulation.movegen import apply_move, LegalMove, MOVE_DRAW
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, DrawPhase, Location
        )

        state = self._make_state()
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_draw",
            generation=0,
            setup=SetupRules(cards_per_player=0),
            turn_structure=TurnStructure(phases=[
                DrawPhase(source=Location.DECK, mandatory=True)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
        )

        move = LegalMove(phase_index=0, card_index=MOVE_DRAW, target_loc=Location.DECK)
        new_state = apply_move(state, move, genome)

        # Player should now have 1 card
        assert len(new_state.players[0].hand) == 1
        assert new_state.players[0].hand[0].rank == Rank.KING
        # Deck should be empty
        assert len(new_state.deck) == 0


class TestClaimPhaseMovegen:
    """Test ClaimPhase move generation."""

    def _make_state(self, hands: list[tuple]) -> "GameState":
        from darwindeck.simulation.state import GameState, Card

        players = tuple(
            PlayerState(player_id=i, hand=tuple(Card(rank=Rank(r), suit=Suit(s)) for r, s in hand), score=0)
            for i, hand in enumerate(hands)
        )
        return GameState(
            players=players,
            deck=(),
            discard=(),
            turn=1,
            active_player=0,
        )

    def test_no_claim_generates_card_moves(self):
        """When no active claim, can play cards to make claim."""
        from darwindeck.simulation.movegen import generate_legal_moves
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, ClaimPhase
        )

        state = self._make_state([[("A", "S"), ("K", "H")], [("Q", "D")]])
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_claim",
            generation=0,
            setup=SetupRules(cards_per_player=2),
            turn_structure=TurnStructure(phases=[ClaimPhase()]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
        )

        moves = generate_legal_moves(state, genome)

        # Should have 2 moves (one per card)
        assert len(moves) == 2

    def test_active_claim_generates_challenge_and_pass(self):
        """When active claim exists, opponent can challenge or accept."""
        from darwindeck.simulation.movegen import generate_legal_moves, MOVE_CHALLENGE, MOVE_CLAIM_PASS
        from darwindeck.simulation.state import Claim, Card
        from darwindeck.genome.schema import (
            GameGenome, SetupRules, TurnStructure, WinCondition, ClaimPhase
        )

        state = self._make_state([[("A", "S")], [("K", "H")]])
        # Player 0 made a claim, now player 1's turn
        state = state.copy_with(
            active_player=1,
            current_claim=Claim(
                claimer_id=0,
                claimed_rank=0,  # Ace
                claimed_count=1,
                cards_played=(Card(rank=Rank.ACE, suit=Suit.SPADES),),
            ),
        )
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test_claim",
            generation=0,
            setup=SetupRules(cards_per_player=1),
            turn_structure=TurnStructure(phases=[ClaimPhase()]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
        )

        moves = generate_legal_moves(state, genome)

        # Should have 2 moves (challenge + accept)
        assert len(moves) == 2
        card_indices = [m.card_index for m in moves]
        assert MOVE_CHALLENGE in card_indices
        assert MOVE_CLAIM_PASS in card_indices