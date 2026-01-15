"""Playtest session management."""

from __future__ import annotations

import random
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional, Callable

from typing import Union, List

from darwindeck.genome.schema import GameGenome, Rank, Suit, BettingPhase
from darwindeck.simulation.state import GameState, PlayerState, Card
from darwindeck.simulation.movegen import (
    LegalMove, BettingMove, BettingAction,
    generate_legal_moves, apply_move, apply_betting_move, check_win_conditions,
)
from darwindeck.playtest.stuck import StuckDetector
from darwindeck.playtest.display import StateRenderer, MovePresenter
from darwindeck.playtest.rules import RuleExplainer
from darwindeck.playtest.input import HumanPlayer, InputResult
from darwindeck.playtest.feedback import FeedbackCollector, PlaytestResult


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
                        self._advance_turn()
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
                        phase = self.genome.turn_structure.phases[move.phase_index]
                        if isinstance(phase, BettingPhase):
                            self.state = apply_betting_move(self.state, move, phase)
                        self._advance_turn()
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

    def _ai_select_move(self, moves: List[Union[LegalMove, BettingMove]]) -> Optional[Union[LegalMove, BettingMove]]:
        """Select move using AI strategy."""
        if not moves:
            return None

        # Check if these are betting moves
        if isinstance(moves[0], BettingMove):
            if self.config.difficulty == "random":
                return self.rng.choice(moves)
            return self._ai_betting_select(moves)  # type: ignore

        if self.config.difficulty == "random":
            return self.rng.choice(moves)

        # Greedy/MCTS: use hand-reducing heuristic for card plays
        return self._greedy_select(moves)  # type: ignore

    def _ai_betting_select(self, moves: list[BettingMove]) -> BettingMove:
        """AI betting strategy: simple heuristic until Task 13."""
        # For now: prefer check > call > bet > raise > fold
        priority = {
            BettingAction.CHECK: 5,
            BettingAction.CALL: 4,
            BettingAction.BET: 3,
            BettingAction.RAISE: 2,
            BettingAction.ALL_IN: 1,
            BettingAction.FOLD: 0,
        }
        return max(moves, key=lambda m: priority.get(m.action, -1))

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
