"""Game simulation engine."""

from dataclasses import dataclass, field
from typing import List
from cards_evolve.genome.schema import GameGenome, Rank
from cards_evolve.simulation.state import GameState, Card
from cards_evolve.simulation.interpreter import GenomeInterpreter
from cards_evolve.simulation.players import AIPlayer


# Rank value mapping for card comparison
RANK_VALUES = {
    Rank.TWO: 2,
    Rank.THREE: 3,
    Rank.FOUR: 4,
    Rank.FIVE: 5,
    Rank.SIX: 6,
    Rank.SEVEN: 7,
    Rank.EIGHT: 8,
    Rank.NINE: 9,
    Rank.TEN: 10,
    Rank.JACK: 11,
    Rank.QUEEN: 12,
    Rank.KING: 13,
    Rank.ACE: 14,  # Ace high in War
}


def get_rank_value(card: Card) -> int:
    """Get numeric value for card rank."""
    return RANK_VALUES[card.rank]


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
        """Simulate a complete game.

        For War (simplified): just play until someone runs out.
        """
        logic = self.interpreter.to_executable(genome)
        state = logic.create_initial_state(seed)

        history: List[GameState] = [state]

        # Simplified War simulation (proper logic comes later)
        while state.turn < genome.max_turns:
            # War: compare top cards
            if len(state.players[0].hand) == 0:
                winner = 1
                break
            if len(state.players[1].hand) == 0:
                winner = 0
                break

            p0_card = state.players[0].hand[0]
            p1_card = state.players[1].hand[0]

            # Compare ranks using numeric values (FIX for string comparison bug)
            p0_hand = state.players[0].hand[1:]
            p1_hand = state.players[1].hand[1:]

            if get_rank_value(p0_card) > get_rank_value(p1_card):
                # Player 0 wins
                p0_hand = p0_hand + (p0_card, p1_card)
            elif get_rank_value(p1_card) > get_rank_value(p0_card):
                # Player 1 wins
                p1_hand = p1_hand + (p1_card, p0_card)
            else:
                # Tie - simplified: return cards to bottom
                p0_hand = p0_hand + (p0_card,)
                p1_hand = p1_hand + (p1_card,)

            # Create next state
            from cards_evolve.simulation.state import PlayerState
            new_players = (
                PlayerState(player_id=0, hand=p0_hand, score=0),
                PlayerState(player_id=1, hand=p1_hand, score=0),
            )
            state = state.copy_with(
                players=new_players,
                turn=state.turn + 1
            )
            history.append(state)

        else:
            # Max turns reached
            winner = 0 if len(state.players[0].hand) > len(state.players[1].hand) else 1

        return GameResult(
            winner=winner,
            turn_count=len(history) - 1,
            history=history
        )
