"""AI player implementations."""

import random
from abc import ABC, abstractmethod
from typing import List, Optional
from darwindeck.simulation.state import GameState


class AIPlayer(ABC):
    """Base class for AI players."""

    @abstractmethod
    def choose_action(
        self,
        state: GameState,
        legal_actions: List[int]  # Simplified: indices of legal moves
    ) -> int:
        """Choose an action from legal actions."""
        pass


class RandomPlayer(AIPlayer):
    """Player that chooses randomly from legal moves."""

    def __init__(self, seed: Optional[int] = None) -> None:
        self.rng = random.Random(seed)

    def choose_action(
        self,
        state: GameState,
        legal_actions: List[int]
    ) -> int:
        """Choose uniformly from legal actions."""
        if not legal_actions:
            raise ValueError("No legal actions available")
        return self.rng.choice(legal_actions)
