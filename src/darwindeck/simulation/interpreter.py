"""Genome interpreter - converts structured data to executable logic."""

import random
from typing import List
from darwindeck.genome.schema import GameGenome, Rank, Suit, Location
from darwindeck.simulation.state import GameState, PlayerState, Card


class GameLogic:
    """Executable game logic from genome.

    This is the interpreter pattern: genome is data, this executes it.
    NO code generation, NO exec().
    """

    def __init__(self, genome: GameGenome) -> None:
        self.genome = genome

    def create_initial_state(self, seed: int) -> GameState:
        """Create initial game state with shuffled deck."""
        rng = random.Random(seed)

        # Create standard 52-card deck
        deck: List[Card] = []
        for suit in Suit:
            for rank in Rank:
                deck.append(Card(rank=rank, suit=suit))

        # Shuffle
        rng.shuffle(deck)

        # Deal to players
        cards_per_player = self.genome.setup.cards_per_player
        player_count = self.genome.player_count

        players = []
        for i in range(player_count):
            start_idx = i * cards_per_player
            end_idx = start_idx + cards_per_player
            hand = tuple(deck[start_idx:end_idx])
            players.append(PlayerState(player_id=i, hand=hand, score=0))

        # Remaining cards go to deck
        remaining_deck = tuple(deck[player_count * cards_per_player:])

        # Check if game uses tableau (check if any phase targets tableau)
        uses_tableau = any(
            hasattr(phase, 'target') and phase.target == Location.TABLEAU
            for phase in self.genome.turn_structure.phases
        )

        # Initialize tableau if needed
        tableau = ((),) if uses_tableau else None

        return GameState(
            players=tuple(players),
            deck=remaining_deck,
            discard=(),
            turn=0,
            active_player=0,
            tableau=tableau
        )


class GenomeInterpreter:
    """Converts genome to executable GameLogic."""

    def to_executable(self, genome: GameGenome) -> GameLogic:
        """Convert structured genome to executable logic object.

        Uses interpreter pattern - instantiates logic based on data.
        Safe for pickling, no code generation.
        """
        return GameLogic(genome)
