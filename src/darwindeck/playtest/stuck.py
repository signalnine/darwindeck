"""Stuck detection for playtest games."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from darwindeck.simulation.state import GameState


@dataclass
class StuckDetector:
    """Detects stuck games using multiple strategies.

    Strategies:
    1. Absolute turn limit
    2. State repetition via hashing
    3. Consecutive passes
    """

    max_turns: int = 200
    repeat_threshold: int = 3
    pass_threshold: int = 10

    # Internal state
    _state_hashes: dict[int, int] = field(default_factory=dict)
    _consecutive_passes: int = 0

    def check(self, state: GameState) -> Optional[str]:
        """Check if game is stuck. Returns reason or None."""
        # Strategy 1: Turn limit
        if state.turn >= self.max_turns:
            return f"Turn limit reached ({self.max_turns})"

        # Strategy 2: State repetition
        state_hash = self._hash_state(state)
        self._state_hashes[state_hash] = self._state_hashes.get(state_hash, 0) + 1

        if self._state_hashes[state_hash] >= self.repeat_threshold:
            return f"Same state repeated {self.repeat_threshold} times"

        return None

    def record_pass(self) -> Optional[str]:
        """Record a pass action. Returns reason if stuck."""
        self._consecutive_passes += 1

        if self._consecutive_passes >= self.pass_threshold:
            return f"{self.pass_threshold} consecutive passes"

        return None

    def record_action(self) -> None:
        """Record a non-pass action (resets pass counter)."""
        self._consecutive_passes = 0

    def reset(self) -> None:
        """Reset all detection state."""
        self._state_hashes.clear()
        self._consecutive_passes = 0

    def _hash_state(self, state: GameState) -> int:
        """Hash relevant state for comparison."""
        key = (
            tuple(len(p.hand) for p in state.players),
            len(state.deck),
            state.discard[-1] if state.discard else None,
            state.active_player,
            # Include betting state to avoid false positives during betting rounds
            state.pot,
            state.current_bet,
            state.raise_count,
            tuple(p.chips for p in state.players),
            tuple(p.current_bet for p in state.players),
            tuple(p.has_folded for p in state.players),
            tuple(p.is_all_in for p in state.players),
        )
        return hash(key)
