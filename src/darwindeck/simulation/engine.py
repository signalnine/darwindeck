"""Game simulation engine."""

from dataclasses import dataclass, field
from typing import List
from darwindeck.genome.schema import GameGenome
from darwindeck.simulation.state import GameState
from darwindeck.simulation.interpreter import GenomeInterpreter
from darwindeck.simulation.players import AIPlayer


@dataclass(frozen=True)
class GameResult:
    """Result of a simulated game."""

    winner: int  # Player ID
    turn_count: int
    history: tuple[GameState, ...]  # State at each turn

    def __init__(
        self,
        winner: int,
        turn_count: int,
        history: List[GameState]
    ) -> None:
        object.__setattr__(self, "winner", winner)
        object.__setattr__(self, "turn_count", turn_count)
        object.__setattr__(self, "history", tuple(history))


class GameEngine:
    """Simulates card games from genomes."""

    def __init__(self) -> None:
        self.interpreter = GenomeInterpreter()

    def simulate_game(
        self,
        genome: GameGenome,
        players: List[AIPlayer],
        seed: int
    ) -> GameResult:
        """Simulate a complete game using genome interpreter."""
        from darwindeck.simulation.movegen import (
            generate_legal_moves,
            apply_move,
            check_win_conditions
        )
        import random

        logic = self.interpreter.to_executable(genome)
        state = logic.create_initial_state(seed)

        # Create RNG for move selection
        rng = random.Random(seed + 1000)  # Offset to avoid correlation with shuffle

        history: List[GameState] = [state]
        winner = None

        # Game loop with genome-based move generation
        while state.turn < genome.max_turns:
            # Check win conditions
            winner = check_win_conditions(state, genome)
            if winner is not None:
                break

            # Generate legal moves
            legal_moves = generate_legal_moves(state, genome)

            # Check for no legal moves
            if not legal_moves:
                # Game is stuck - determine winner by hand size
                hand_sizes = [len(p.hand) for p in state.players]
                max_size = max(hand_sizes)
                winner = hand_sizes.index(max_size)
                break

            # Choose move (for now, random selection)
            # TODO: Use AI players for move selection
            move_idx = rng.randint(0, len(legal_moves) - 1)
            move = legal_moves[move_idx]

            # Apply move
            state = apply_move(state, move, genome)
            history.append(state)

        # If no winner after max turns, determine by hand size or score
        if winner is None:
            hand_sizes = [len(p.hand) for p in state.players]
            max_size = max(hand_sizes)
            winner = hand_sizes.index(max_size)

        return GameResult(
            winner=winner,
            turn_count=len(history) - 1,
            history=history
        )
