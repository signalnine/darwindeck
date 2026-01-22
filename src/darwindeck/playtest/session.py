"""Playtest session management."""

from __future__ import annotations

import os
import random
import sys
from collections import deque
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional, Callable

from typing import Union, List

from darwindeck.genome.schema import GameGenome, Rank, Suit, BettingPhase
from darwindeck.simulation.state import GameState, PlayerState, Card
from darwindeck.simulation.movegen import (
    LegalMove, BettingMove, BettingAction,
    generate_legal_moves, apply_move, apply_betting_move, check_win_conditions,
    all_bets_matched, count_active_players, count_acting_players,
)
from darwindeck.playtest.stuck import StuckDetector
from darwindeck.playtest.display import StateRenderer, MovePresenter
from darwindeck.playtest.rules import RuleExplainer
from darwindeck.playtest.input import HumanPlayer, InputResult
from darwindeck.playtest.feedback import FeedbackCollector, PlaytestResult
from darwindeck.playtest.rich_display import RichDisplay, MOVE_LOG_SIZE
from darwindeck.playtest.display_state import DisplayState, MoveOption


@dataclass
class SessionConfig:
    """Configuration for playtest session."""

    difficulty: str = "greedy"  # random, greedy, mcts
    debug: bool = False
    max_turns: int = 200
    seed: Optional[int] = None
    show_rules: bool = True
    results_path: Path = field(default_factory=lambda: Path("playtest_results.jsonl"))

    def __post_init__(self):
        """Generate seed if not provided."""
        if self.seed is None:
            self.seed = random.randint(0, 2**32 - 1)


class PlaytestSession:
    """Manages a human playtest session."""

    def __init__(self, genome: GameGenome, config: SessionConfig):
        """Initialize session."""
        self.genome = genome
        self.config = config
        self.seed = config.seed
        self.rng = random.Random(self.seed)

        # Components
        self.stuck_detector = StuckDetector(max_turns=config.max_turns)
        self.renderer = StateRenderer()
        self.presenter = MovePresenter()
        self.explainer = RuleExplainer()
        self.human_input = HumanPlayer()

        # Session state
        self.move_history: list[dict] = []
        self.human_player_idx = self.rng.randint(0, 1)
        self.state: Optional[GameState] = None

        # Betting round state
        self._needs_to_act: list[bool] = []

    def _record_move(self, turn: int, player: str, move_data: dict) -> None:
        """Record move in history."""
        self.move_history.append({
            "turn": turn,
            "player": player,
            "move": move_data,
        })

    def _initialize_state(self) -> GameState:
        """Initialize game state from genome."""
        # Create standard 52-card deck
        deck: list[Card] = []
        for suit in Suit:
            for rank in Rank:
                deck.append(Card(rank=rank, suit=suit))

        # Shuffle with session seed
        self.rng.shuffle(deck)

        # Deal to players
        cards_per_player = self.genome.setup.cards_per_player
        hands: list[tuple[Card, ...]] = []

        for i in range(self.genome.player_count):
            hand = tuple(deck[:cards_per_player])
            deck = deck[cards_per_player:]
            hands.append(hand)

        # Get starting chips (0 for non-betting games)
        starting_chips = self.genome.setup.starting_chips

        # Create player states
        players = tuple(
            PlayerState(
                player_id=i,
                hand=hand,
                score=0,
                chips=starting_chips,
            )
            for i, hand in enumerate(hands)
        )

        return GameState(
            players=players,
            deck=tuple(deck),
            discard=(),
            turn=1,
            active_player=0,
        )

    def run(self, output_fn: Callable[[str], None] = print) -> PlaytestResult:
        """Run the playtest session.

        Args:
            output_fn: Function to output text (default: print)

        Returns:
            PlaytestResult with game outcome and feedback
        """
        # Initialize state
        self.state = self._initialize_state()

        # Detect display mode
        use_rich = (
            sys.stdout.isatty()
            and not os.environ.get("FORCE_PLAIN_DISPLAY")
        )

        if use_rich:
            try:
                return self._run_rich()
            except ImportError:
                # Fall back if rich not available
                pass

        return self._run_plain(output_fn)

    def _run_plain(self, output_fn: Callable[[str], None] = print) -> PlaytestResult:
        """Run the playtest session in plain text mode.

        Args:
            output_fn: Function to output text (default: print)

        Returns:
            PlaytestResult with game outcome and feedback
        """
        # Show rules if configured
        if self.config.show_rules:
            output_fn(self.explainer.explain_rules(self.genome))
            output_fn("")
            output_fn(f"You are Player {self.human_player_idx}")
            output_fn(f"Seed: {self.seed} (use --seed {self.seed} to replay)")
            output_fn("")

        # Main game loop
        winner: Optional[str] = None
        quit_early = False
        felt_broken = False
        stuck_reason: Optional[str] = None

        while True:
            # Check for stuck
            stuck_reason = self.stuck_detector.check(self.state)
            if stuck_reason:
                output_fn(f"\nGame stuck: {stuck_reason}")
                winner = "stuck"
                break

            # Check win conditions
            win_id = check_win_conditions(self.state, self.genome)
            if win_id is not None:
                if win_id == self.human_player_idx:
                    winner = "human"
                    output_fn("\n=== You Win! ===")
                else:
                    winner = "ai"
                    output_fn("\n=== AI Wins ===")
                break

            # Display state
            output_fn("")
            output_fn(self.renderer.render(
                self.state, self.genome, self.human_player_idx, self.config.debug
            ))

            # Generate legal moves
            moves = generate_legal_moves(self.state, self.genome)

            # Check if betting phase should auto-complete (no one can act)
            if not moves and self._should_auto_complete_betting():
                self._advance_phase()
                self._reset_betting_state()
                continue

            # Get move based on current player
            if self.state.active_player == self.human_player_idx:
                # Human turn
                output_fn("")
                output_fn(self.presenter.present(moves, self.state, self.genome))

                result = self.human_input.get_move(moves)

                if result.quit:
                    quit_early = True
                    fb = self.human_input.get_yes_no("Did the game feel broken? [y/n]: ")
                    felt_broken = fb if fb is not None else False
                    winner = "quit"
                    break

                if result.error:
                    output_fn(result.error)
                    continue

                if result.is_pass:
                    self.stuck_detector.record_pass()
                    self._advance_turn()
                    continue

                if result.move:
                    if isinstance(result.move, BettingMove):
                        self._record_move(self.state.turn, "human", {"action": result.move.action.value})
                        phase = self.genome.turn_structure.phases[result.move.phase_index]
                        if isinstance(phase, BettingPhase):
                            self.state = apply_betting_move(self.state, result.move, phase)
                        self._handle_betting_completion(result.move, self.human_player_idx)
                    else:
                        self._record_move(self.state.turn, "human", {"card_index": result.move.card_index})
                        self.state = apply_move(self.state, result.move, self.genome)
                    self.stuck_detector.record_action()
            else:
                # AI turn
                move = self._ai_select_move(moves)
                if move:
                    if isinstance(move, BettingMove):
                        output_fn(f"AI: {move.action.value}")
                        self._record_move(self.state.turn, "ai", {"action": move.action.value})
                        ai_player_idx = self.state.active_player  # Capture BEFORE state changes
                        phase = self.genome.turn_structure.phases[move.phase_index]
                        if isinstance(phase, BettingPhase):
                            self.state = apply_betting_move(self.state, move, phase)
                        self._handle_betting_completion(move, ai_player_idx)
                    else:
                        output_fn(f"AI plays: card {move.card_index + 1}")
                        self._record_move(self.state.turn, "ai", {"card_index": move.card_index})
                        self.state = apply_move(self.state, move, self.genome)
                    self.stuck_detector.record_action()
                else:
                    output_fn("AI passes")
                    self.stuck_detector.record_pass()
                    self._advance_turn()

        # Collect feedback
        output_fn("")
        rating = self.human_input.get_rating()
        comment = self.human_input.get_comment()

        return PlaytestResult(
            genome_id=self.genome.genome_id,
            genome_path="",  # Set by caller
            difficulty=self.config.difficulty,
            seed=self.seed,
            winner=winner or "unknown",
            turns=self.state.turn if self.state else 0,
            rating=rating,
            comment=comment,
            quit_early=quit_early,
            felt_broken=felt_broken,
            stuck_reason=stuck_reason,
        )

    def _advance_turn(self) -> None:
        """Advance to next turn without applying a move."""
        if self.state:
            next_player = (self.state.active_player + 1) % len(self.state.players)
            self.state = self.state.copy_with(
                active_player=next_player,
                turn=self.state.turn + 1,
            )

    def _should_auto_complete_betting(self) -> bool:
        """Check if current betting phase should auto-complete.

        Returns True if:
        - Current phase is a BettingPhase
        - No one can act (all folded, all-in, or no chips)
        - Bets are matched among active players
        """
        if not self.state:
            return False

        phase = self.genome.turn_structure.phases[self.state.current_phase]
        if not isinstance(phase, BettingPhase):
            return False

        # Check if no one can act and bets are matched
        if count_acting_players(self.state) == 0 and all_bets_matched(self.state):
            return True

        # Also complete if only one player remains
        if count_active_players(self.state) <= 1:
            return True

        return False

    def _init_betting_round(self) -> None:
        """Initialize needs_to_act for a new betting round."""
        if not self.state:
            return
        self._needs_to_act = []
        for player in self.state.players:
            # Player needs to act if they can act
            can_act = not player.has_folded and not player.is_all_in and player.chips > 0
            self._needs_to_act.append(can_act)

    def _reset_needs_to_act_for_bet_increase(self, acting_player: int) -> None:
        """Reset needs_to_act when bet increases (BET/RAISE/ALL_IN with chips)."""
        if not self.state:
            return
        for i, player in enumerate(self.state.players):
            if i == acting_player:
                self._needs_to_act[i] = False  # Acting player just acted
            elif not player.has_folded and not player.is_all_in and player.chips > 0:
                self._needs_to_act[i] = True  # Everyone else must respond

    def _advance_to_next_acting_player(self) -> None:
        """Advance to the next player who needs to act."""
        if not self.state:
            return

        num_players = len(self.state.players)
        next_player = (self.state.active_player + 1) % num_players

        # Find next player who needs to act
        for _ in range(num_players):
            if self._needs_to_act[next_player]:
                break
            next_player = (next_player + 1) % num_players

        self.state = self.state.copy_with(
            active_player=next_player,
            turn=self.state.turn + 1,
        )

    def _handle_betting_completion(self, move: BettingMove, player_idx: int) -> None:
        """Check if betting round is complete and advance phase if so."""
        if not self.state:
            return

        # Initialize needs_to_act if not set
        if not self._needs_to_act:
            self._init_betting_round()

        # Mark current player as having acted
        self._needs_to_act[player_idx] = False

        # If bet increased, everyone else needs to act again
        if move.action in (BettingAction.BET, BettingAction.RAISE):
            self._reset_needs_to_act_for_bet_increase(player_idx)
        elif move.action == BettingAction.ALL_IN:
            # ALL_IN may increase bet - reset others to act
            self._reset_needs_to_act_for_bet_increase(player_idx)

        # Check termination: only one player remains
        if count_active_players(self.state) <= 1:
            self._advance_phase()
            self._reset_betting_state()
            return

        # Check termination: all remaining players are all-in (no one can act)
        if count_acting_players(self.state) == 0:
            self._advance_phase()
            self._reset_betting_state()
            return

        # Check termination: everyone has acted and bets matched
        if not any(self._needs_to_act) and all_bets_matched(self.state):
            self._advance_phase()
            self._reset_betting_state()
            return

        # Otherwise advance to next player who can act
        self._advance_to_next_acting_player()

    def _advance_phase(self) -> None:
        """Advance to the next phase in the turn structure."""
        if not self.state:
            return

        num_phases = len(self.genome.turn_structure.phases)
        next_phase = self.state.current_phase + 1

        if next_phase >= num_phases:
            # Wrap to first phase and advance turn
            next_phase = 0
            next_player = (self.state.active_player + 1) % len(self.state.players)
            self.state = self.state.copy_with(
                current_phase=next_phase,
                active_player=next_player,
                turn=self.state.turn + 1,
            )
        else:
            # Just advance phase, keep same player
            self.state = self.state.copy_with(
                current_phase=next_phase,
                turn=self.state.turn + 1,
            )

    def _reset_betting_state(self) -> None:
        """Reset betting state for new round."""
        if not self.state:
            return

        # Reset player betting states
        new_players = tuple(
            p.copy_with(current_bet=0)
            for p in self.state.players
        )
        self.state = self.state.copy_with(
            players=new_players,
            current_bet=0,
            raise_count=0,
        )
        # Clear needs_to_act for next betting round
        self._needs_to_act = []

    def _ai_select_move(self, moves: List[Union[LegalMove, BettingMove]]) -> Optional[Union[LegalMove, BettingMove]]:
        """Select move using AI strategy."""
        if not moves:
            return None

        # Separate moves by type (genome may have multiple phase types)
        card_moves = [m for m in moves if isinstance(m, LegalMove)]
        betting_moves = [m for m in moves if isinstance(m, BettingMove)]

        # Prioritize card plays over betting (card plays are core game actions)
        if card_moves:
            if self.config.difficulty == "random":
                return self.rng.choice(card_moves)
            return self._greedy_select(card_moves)

        # If no card moves, use betting
        if betting_moves:
            if self.config.difficulty == "random":
                return self.rng.choice(betting_moves)
            return self._ai_betting_select(betting_moves)

        return None

    def _ai_betting_select(self, moves: list[BettingMove]) -> BettingMove:
        """AI betting strategy based on hand strength.

        Hand strength determines action priority:
        - Strong hand (>0.7): RAISE > BET > CALL > CHECK > FOLD
        - Medium hand (0.4-0.7): CHECK > CALL > BET > FOLD > RAISE
        - Weak hand (<0.4): CHECK > FOLD > CALL > BET > RAISE
        """
        if not self.state:
            return moves[0]

        # Calculate hand strength
        ai_player = 1 - self.human_player_idx
        hand_strength = self._evaluate_hand_strength(ai_player)

        # Strong hand: aggressive play
        if hand_strength > 0.7:
            priority = {
                BettingAction.RAISE: 6,
                BettingAction.BET: 5,
                BettingAction.ALL_IN: 4,
                BettingAction.CALL: 3,
                BettingAction.CHECK: 2,
                BettingAction.FOLD: 0,
            }
        # Medium hand: cautious play
        elif hand_strength > 0.4:
            priority = {
                BettingAction.CHECK: 5,
                BettingAction.CALL: 4,
                BettingAction.BET: 3,
                BettingAction.FOLD: 2,
                BettingAction.RAISE: 1,
                BettingAction.ALL_IN: 0,
            }
        # Weak hand: defensive play
        else:
            priority = {
                BettingAction.CHECK: 5,
                BettingAction.FOLD: 4,
                BettingAction.CALL: 2,
                BettingAction.BET: 1,
                BettingAction.RAISE: 0,
                BettingAction.ALL_IN: 0,
            }

        return max(moves, key=lambda m: priority.get(m.action, -1))

    def _evaluate_hand_strength(self, player_id: int) -> float:
        """Evaluate hand strength as value between 0.0 and 1.0.

        Uses average rank value normalized to [0,1].
        """
        if not self.state:
            return 0.5

        hand = self.state.players[player_id].hand
        if not hand:
            return 0.0

        rank_values = {
            "2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7,
            "8": 8, "9": 9, "10": 10, "J": 11, "Q": 12, "K": 13, "A": 14
        }

        total = sum(rank_values.get(card.rank.value, 7) for card in hand)
        avg = total / len(hand)

        # Normalize: 2 -> 0.0, 14 -> 1.0
        return (avg - 2) / 12.0

    def _greedy_select(self, moves: list[LegalMove]) -> LegalMove:
        """Greedy heuristic: prefer moves that play cards (reduce hand size)."""
        if not self.state:
            return moves[0]

        # Score each move: prefer card plays, higher cards for captures
        def score_move(move: LegalMove) -> tuple[int, int]:
            # Prefer moves that play cards (card_index >= 0 means a card play)
            plays_card = 1 if move.card_index >= 0 else 0

            # For card plays, prefer higher-rank cards (for captures)
            card_value = 0
            if move.card_index >= 0:
                ai_player = 1 - self.human_player_idx
                hand = self.state.players[ai_player].hand
                if move.card_index < len(hand):
                    card = hand[move.card_index]
                    # Rank value: 2=2, ..., A=14
                    rank_values = {
                        "2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7,
                        "8": 8, "9": 9, "10": 10, "J": 11, "Q": 12, "K": 13, "A": 14
                    }
                    card_value = rank_values.get(card.rank.value, 0)

            return (plays_card, card_value)

        # Sort moves by score (descending), pick best
        scored = sorted(moves, key=score_move, reverse=True)
        best_score = score_move(scored[0])

        # Randomly pick among ties
        ties = [m for m in scored if score_move(m) == best_score]
        return self.rng.choice(ties)

    def _run_rich(self) -> PlaytestResult:
        """Run the playtest session using Rich display.

        Returns:
            PlaytestResult with game outcome and feedback
        """
        display = RichDisplay()
        move_log: deque[tuple[str, str]] = deque(maxlen=MOVE_LOG_SIZE)

        # Show rules if configured
        if self.config.show_rules:
            display.show_message(self.explainer.explain_rules(self.genome))
            display.show_message("")
            display.show_message(f"You are Player {self.human_player_idx}")
            display.show_message(f"Seed: {self.seed} (use --seed {self.seed} to replay)")
            display.show_message("")

        # Main game loop
        winner: Optional[str] = None
        quit_early = False
        felt_broken = False
        stuck_reason: Optional[str] = None

        while True:
            # Check for stuck
            stuck_reason = self.stuck_detector.check(self.state)
            if stuck_reason:
                display.show_error(f"Game stuck: {stuck_reason}")
                winner = "stuck"
                break

            # Check win conditions
            win_id = check_win_conditions(self.state, self.genome)
            if win_id is not None:
                if win_id == self.human_player_idx:
                    winner = "human"
                    display.show_message("=== You Win! ===", style="bold green")
                else:
                    winner = "ai"
                    display.show_message("=== AI Wins ===", style="bold red")
                break

            # Generate legal moves
            moves = generate_legal_moves(self.state, self.genome)

            # Check if betting phase should auto-complete (no one can act)
            if not moves and self._should_auto_complete_betting():
                self._advance_phase()
                self._reset_betting_state()
                continue

            # Get move based on current player
            if self.state.active_player == self.human_player_idx:
                # Human turn - build display state and prompt
                terminal_width = display.get_terminal_width()
                display_state = self._build_display_state(moves, move_log, terminal_width)

                raw_input = display.show_and_prompt(display_state)
                result = self._parse_input(raw_input, moves)

                if result.quit:
                    quit_early = True
                    display.show_message("Did the game feel broken? [y/n]: ")
                    fb = self.human_input.get_yes_no("")
                    felt_broken = fb if fb is not None else False
                    winner = "quit"
                    break

                if result.error:
                    display.show_error(result.error)
                    continue

                if result.is_pass:
                    self.stuck_detector.record_pass()
                    move_log.append(("player", "passed"))
                    self._advance_turn()
                    continue

                if result.move:
                    if isinstance(result.move, BettingMove):
                        self._record_move(self.state.turn, "human", {"action": result.move.action.value})
                        move_log.append(("player", result.move.action.value.lower()))
                        phase = self.genome.turn_structure.phases[result.move.phase_index]
                        if isinstance(phase, BettingPhase):
                            self.state = apply_betting_move(self.state, result.move, phase)
                        self._handle_betting_completion(result.move, self.human_player_idx)
                    else:
                        self._record_move(self.state.turn, "human", {"card_index": result.move.card_index})
                        # Log the card played
                        player = self.state.players[self.human_player_idx]
                        if 0 <= result.move.card_index < len(player.hand):
                            card = player.hand[result.move.card_index]
                            move_log.append(("player", f"played {card.rank.value}{card.suit.value}"))
                        else:
                            move_log.append(("player", "played card"))
                        self.state = apply_move(self.state, result.move, self.genome)
                    self.stuck_detector.record_action()
            else:
                # AI turn
                move = self._ai_select_move(moves)

                # Build display state for AI turn display
                terminal_width = display.get_terminal_width()
                display_state = self._build_display_state(moves, move_log, terminal_width)

                if move:
                    if isinstance(move, BettingMove):
                        move_log.append(("opponent", move.action.value.lower()))
                        self._record_move(self.state.turn, "ai", {"action": move.action.value})
                        ai_player_idx = self.state.active_player  # Capture BEFORE state changes
                        phase = self.genome.turn_structure.phases[move.phase_index]
                        if isinstance(phase, BettingPhase):
                            self.state = apply_betting_move(self.state, move, phase)
                        self._handle_betting_completion(move, ai_player_idx)
                    else:
                        # Log the card played
                        ai_player = self.state.players[self.state.active_player]
                        if 0 <= move.card_index < len(ai_player.hand):
                            card = ai_player.hand[move.card_index]
                            move_log.append(("opponent", f"played {card.rank.value}{card.suit.value}"))
                        else:
                            move_log.append(("opponent", "played card"))
                        self._record_move(self.state.turn, "ai", {"card_index": move.card_index})
                        self.state = apply_move(self.state, move, self.genome)
                    self.stuck_detector.record_action()
                else:
                    move_log.append(("opponent", "passed"))
                    self.stuck_detector.record_pass()
                    self._advance_turn()

                # Show AI turn briefly
                display_state = self._build_display_state([], move_log, terminal_width)
                display.show_ai_turn(display_state, duration=0.3)

        # Collect feedback
        display.show_message("")
        rating = self.human_input.get_rating()
        comment = self.human_input.get_comment()

        return PlaytestResult(
            genome_id=self.genome.genome_id,
            genome_path="",  # Set by caller
            difficulty=self.config.difficulty,
            seed=self.seed,
            winner=winner or "unknown",
            turns=self.state.turn if self.state else 0,
            rating=rating,
            comment=comment,
            quit_early=quit_early,
            felt_broken=felt_broken,
            stuck_reason=stuck_reason,
        )

    def _build_display_state(
        self, moves: list, move_log: deque, terminal_width: int
    ) -> DisplayState:
        """Convert GameState to DisplayState for rendering.

        Args:
            moves: List of legal moves
            move_log: Recent move history
            terminal_width: Terminal width in characters

        Returns:
            DisplayState for rendering
        """
        # Get phase name from genome
        phase_idx = self.state.current_phase
        phase = self.genome.turn_structure.phases[phase_idx]
        phase_name = phase.__class__.__name__.replace("Phase", "")

        # Build hand_cards as list of (rank, suit) tuples
        player = self.state.players[self.human_player_idx]
        hand_cards = [(c.rank.value, c.suit.value) for c in player.hand]

        # Build opponent info
        opponent_idx = 1 - self.human_player_idx
        opponent = self.state.players[opponent_idx]

        # Build discard_top
        discard_top = None
        if self.state.discard:
            top = self.state.discard[-1]
            discard_top = (top.rank.value, top.suit.value)

        # Build move options
        move_options = self._build_move_options(moves)

        return DisplayState(
            game_name=self.genome.genome_id,
            turn=self.state.turn,
            phase_name=phase_name,
            player_id=self.human_player_idx,
            hand_cards=hand_cards,
            opponent_card_count=len(opponent.hand),
            opponent_chips=opponent.chips,
            opponent_bet=opponent.current_bet,
            player_chips=player.chips,
            player_bet=player.current_bet,
            pot=self.state.pot,
            current_bet=self.state.current_bet,
            discard_top=discard_top,
            moves=move_options,
            move_log=list(move_log),
            terminal_width=terminal_width,
        )

    def _build_move_options(self, moves: list) -> list[MoveOption]:
        """Convert legal moves to MoveOption for display.

        Args:
            moves: List of legal moves (LegalMove or BettingMove)

        Returns:
            List of MoveOption for display
        """
        options = []
        player = self.state.players[self.human_player_idx]

        for i, move in enumerate(moves):
            if isinstance(move, BettingMove):
                label = move.action.value.title()  # "Check", "Fold", etc.
                move_type = "betting"
            elif move.card_index == -1:
                label = "Pass"
                move_type = "pass"
            elif 0 <= move.card_index < len(player.hand):
                card = player.hand[move.card_index]
                # Format with unicode symbol
                symbols = {"H": "\u2665", "D": "\u2666", "C": "\u2663", "S": "\u2660"}
                symbol = symbols.get(card.suit.value, card.suit.value)
                label = f"{card.rank.value}{symbol}"
                move_type = "card"
            else:
                label = f"Move {i+1}"
                move_type = "other"

            options.append(MoveOption(index=i+1, label=label, move_type=move_type))

        return options

    def _parse_input(self, raw: str, moves: list) -> InputResult:
        """Parse raw input string into InputResult.

        Args:
            raw: Raw input string from user
            moves: List of legal moves

        Returns:
            InputResult with move, quit flag, pass flag, or error
        """
        raw = raw.strip().lower()

        if raw in ("q", "quit", "exit"):
            return InputResult(quit=True)

        if not raw:
            return InputResult(is_pass=True)

        try:
            choice = int(raw)
        except ValueError:
            return InputResult(error=f"Invalid input '{raw}'. Enter a number or 'q'.")

        if choice < 1 or choice > len(moves):
            return InputResult(error=f"Invalid choice {choice}. Enter 1-{len(moves)}.")

        return InputResult(move=moves[choice - 1])
