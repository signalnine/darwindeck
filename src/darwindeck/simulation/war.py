"""War game simulation (Python baseline)."""

import random
from typing import Dict, List, Tuple


class WarGame:
    """Simple War card game implementation."""

    def __init__(self, seed: int = 42) -> None:
        """Initialize War game with shuffled deck."""
        self.rng = random.Random(seed)

        # Create deck (rank only matters for War)
        deck = list(range(1, 14)) * 4  # 1-13, four suits
        self.rng.shuffle(deck)

        # Split evenly
        self.player1_hand = deck[:26]
        self.player2_hand = deck[26:]
        self.turns = 0

    def play_battle(self) -> None:
        """Play one battle."""
        if not self.player1_hand or not self.player2_hand:
            return

        p1_card = self.player1_hand.pop(0)
        p2_card = self.player2_hand.pop(0)

        if p1_card > p2_card:
            self.player1_hand.extend([p1_card, p2_card])
        elif p2_card > p1_card:
            self.player2_hand.extend([p2_card, p1_card])
        else:
            # War! Each player plays 3 face down + 1 face up
            if len(self.player1_hand) >= 4 and len(self.player2_hand) >= 4:
                war_pile = [p1_card, p2_card]
                war_pile.extend(self.player1_hand[:4])
                war_pile.extend(self.player2_hand[:4])
                self.player1_hand = self.player1_hand[4:]
                self.player2_hand = self.player2_hand[4:]

                # Winner takes all
                if war_pile[-4] > war_pile[-1]:  # p1 wins
                    self.player1_hand.extend(war_pile)
                else:  # p2 wins
                    self.player2_hand.extend(war_pile)
            else:
                # Not enough cards for war, return cards
                self.player1_hand.append(p1_card)
                self.player2_hand.append(p2_card)

        self.turns += 1

    def is_game_over(self) -> bool:
        """Check if game has ended."""
        return len(self.player1_hand) == 0 or len(self.player2_hand) == 0

    def get_winner(self) -> int:
        """Get winner (1 or 2)."""
        if len(self.player1_hand) > len(self.player2_hand):
            return 1
        return 2


def play_war_game(seed: int = 42, max_turns: int = 1000) -> Dict[str, int]:
    """Play a complete War game and return results."""
    game = WarGame(seed=seed)

    while not game.is_game_over() and game.turns < max_turns:
        game.play_battle()

    return {
        "winner": game.get_winner(),
        "turns": game.turns
    }
