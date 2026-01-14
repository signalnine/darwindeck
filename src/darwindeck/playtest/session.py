"""Playtest session management."""

from __future__ import annotations

import random
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional, Callable

from darwindeck.genome.schema import GameGenome, Rank, Suit
from darwindeck.simulation.state import GameState, PlayerState, Card
from darwindeck.simulation.movegen import LegalMove, generate_legal_moves, apply_move, check_win_conditions
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

        # Create player states
        players = tuple(
            PlayerState(player_id=i, hand=hand, score=0)
            for i, hand in enumerate(hands)
        )

        return GameState(
            players=players,
            deck=tuple(deck),
            discard=(),
            turn=1,
            active_player=0,
        )
