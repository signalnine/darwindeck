"""Move generation and application for genome-based games."""

from dataclasses import dataclass
from enum import Enum
from typing import List, Optional
from darwindeck.genome.schema import GameGenome, PlayPhase, Location, BettingPhase
from darwindeck.simulation.state import GameState, Card, PlayerState


class BettingAction(Enum):
    """Betting action types."""
    CHECK = "check"
    BET = "bet"
    CALL = "call"
    RAISE = "raise"
    ALL_IN = "all_in"
    FOLD = "fold"


@dataclass(frozen=True)
class BettingMove:
    """A betting action (separate from card play moves)."""
    action: BettingAction
    phase_index: int


# Rank value mapping for card comparison
RANK_VALUES = {
    "A": 14,  # Ace high
    "K": 13,
    "Q": 12,
    "J": 11,
    "10": 10,
    "9": 9,
    "8": 8,
    "7": 7,
    "6": 6,
    "5": 5,
    "4": 4,
    "3": 3,
    "2": 2,
}


def get_rank_value(card: Card) -> int:
    """Get numeric value for card rank."""
    return RANK_VALUES[card.rank.value]


@dataclass(frozen=True)
class LegalMove:
    """A possible move in the game."""
    phase_index: int
    card_index: int  # -1 if not card-specific
    target_loc: Location


def generate_legal_moves(state: GameState, genome: GameGenome) -> List[LegalMove]:
    """Generate all legal moves for current player."""
    moves: List[LegalMove] = []
    current_player = state.active_player

    for phase_idx, phase in enumerate(genome.turn_structure.phases):
        if isinstance(phase, PlayPhase):
            # PlayPhase: play cards from hand
            target = phase.target
            min_cards = phase.min_cards
            max_cards = phase.max_cards

            # For now, only support single-card plays
            if min_cards <= 1 and max_cards >= 1:
                # Check each card in hand
                for card_idx in range(len(state.players[current_player].hand)):
                    # TODO: Evaluate valid_play_condition
                    # For now, allow all cards
                    moves.append(LegalMove(
                        phase_index=phase_idx,
                        card_index=card_idx,
                        target_loc=target
                    ))

    return moves


def apply_move(state: GameState, move: LegalMove, genome: GameGenome) -> GameState:
    """Apply a move to the state, returning new state."""
    if move.phase_index >= len(genome.turn_structure.phases):
        return state

    phase = genome.turn_structure.phases[move.phase_index]
    current_player = state.active_player

    if isinstance(phase, PlayPhase):
        if move.card_index >= 0:
            # Play card from hand
            state = play_card(state, current_player, move.card_index, move.target_loc)

            # War-specific logic: resolve battle after both players play
            if move.target_loc == Location.TABLEAU and len(state.players) == 2:
                state = resolve_war_battle(state)

    # Advance turn
    next_player = (state.active_player + 1) % len(state.players)
    new_turn = state.turn + 1

    return state.copy_with(
        active_player=next_player,
        turn=new_turn
    )


def play_card(state: GameState, player_id: int, card_index: int, target: Location) -> GameState:
    """Play a card from player's hand to target location."""
    player = state.players[player_id]

    if card_index < 0 or card_index >= len(player.hand):
        return state  # Invalid card index

    card = player.hand[card_index]
    new_hand = player.hand[:card_index] + player.hand[card_index+1:]

    # Update player
    new_player = player.copy_with(hand=new_hand)
    new_players = tuple(
        new_player if i == player_id else p
        for i, p in enumerate(state.players)
    )

    # Add card to target location
    if target == Location.DISCARD:
        new_discard = state.discard + (card,)
        return state.copy_with(players=new_players, discard=new_discard)

    elif target == Location.TABLEAU:
        # Initialize tableau if needed
        if state.tableau is None:
            tableau = ((),)  # Single pile
        else:
            tableau = state.tableau

        # Add card to first pile
        new_pile = tableau[0] + (card,)
        new_tableau = (new_pile,) + tableau[1:] if len(tableau) > 1 else (new_pile,)

        return state.copy_with(players=new_players, tableau=new_tableau)

    elif target == Location.DECK:
        new_deck = state.deck + (card,)
        return state.copy_with(players=new_players, deck=new_deck)

    return state.copy_with(players=new_players)


def resolve_war_battle(state: GameState) -> GameState:
    """Handle War game card comparison.

    After both players play to tableau, compare cards and winner takes both.
    """
    if state.tableau is None or len(state.tableau) == 0:
        return state

    tableau = state.tableau[0]

    # Check if both players have played (2 cards in tableau)
    if len(tableau) < 2:
        return state

    # Get the last two cards played
    card1 = tableau[-2]  # Second-to-last (player 0's card)
    card2 = tableau[-1]  # Last (player 1's card)

    # Compare ranks
    rank1 = get_rank_value(card1)
    rank2 = get_rank_value(card2)

    if rank1 > rank2:
        winner = 0
    elif rank2 > rank1:
        winner = 1
    else:
        # Tie - simplified: alternate winners
        winner = state.active_player

    # Winner takes all cards from tableau
    winner_player = state.players[winner]
    new_hand = winner_player.hand + tableau
    new_winner = winner_player.copy_with(hand=new_hand)

    new_players = tuple(
        new_winner if i == winner else p
        for i, p in enumerate(state.players)
    )

    # Clear tableau
    new_tableau = ((),) + state.tableau[1:] if len(state.tableau) > 1 else ((),)

    return state.copy_with(
        players=new_players,
        tableau=new_tableau
    )


def check_win_conditions(state: GameState, genome: GameGenome) -> Optional[int]:
    """Check if any player has won. Returns winner ID or None."""
    for wc in genome.win_conditions:
        if wc.type == "empty_hand":
            for player_id, player in enumerate(state.players):
                if len(player.hand) == 0:
                    return player_id

        elif wc.type == "capture_all":
            for player_id, player in enumerate(state.players):
                if len(player.hand) == 52:
                    return player_id

        elif wc.type == "first_to_score":
            if wc.threshold is not None:
                for player_id, player in enumerate(state.players):
                    if player.score >= wc.threshold:
                        return player_id

        elif wc.type == "high_score":
            # TODO: Only check at end of game
            pass

    return None


# Pattern matching functions for set collection games

def has_set_of_n(hand: tuple[Card, ...], n: int) -> bool:
    """Check if hand contains N cards of the same rank.

    Example: has_set_of_n(hand, 4) checks for 4-of-a-kind (Go Fish book)
    Complexity: O(n) where n is hand size
    """
    rank_counts: dict[str, int] = {}

    for card in hand:
        rank_value = card.rank.value
        rank_counts[rank_value] = rank_counts.get(rank_value, 0) + 1

        if rank_counts[rank_value] >= n:
            return True

    return False


def has_run_of_n(hand: tuple[Card, ...], n: int) -> bool:
    """Check if hand contains N cards in sequential rank order.

    Example: has_run_of_n(hand, 3) checks for runs like 5-6-7
    Complexity: O(n log n) due to sorting
    """
    if len(hand) < n:
        return False

    # Sort hand by rank value
    sorted_hand = sorted(hand, key=lambda c: RANK_VALUES[c.rank.value])

    # Find sequential run
    run_length = 1
    for i in range(1, len(sorted_hand)):
        curr_rank = RANK_VALUES[sorted_hand[i].rank.value]
        prev_rank = RANK_VALUES[sorted_hand[i-1].rank.value]

        if curr_rank == prev_rank + 1:
            run_length += 1
            if run_length >= n:
                return True
        elif curr_rank != prev_rank:
            # Different rank, not sequential - reset counter
            run_length = 1
        # Same rank = continue current run length

    return False


def has_matching_pair(hand: tuple[Card, ...]) -> bool:
    """Check if hand contains two cards with matching rank and color.

    Used for Old Maid style games where pairs are same rank + same color.
    Complexity: O(n²) where n is hand size
    """
    for i in range(len(hand)):
        for j in range(i + 1, len(hand)):
            # Check if same rank
            if hand[i].rank == hand[j].rank:
                # Check if same color (Hearts/Diamonds=red, Clubs/Spades=black)
                color1 = 0 if hand[i].suit.value in ['H', 'D'] else 1
                color2 = 0 if hand[j].suit.value in ['H', 'D'] else 1

                if color1 == color2:
                    return True

    return False


def _update_player_tuple(players: tuple[PlayerState, ...], idx: int, new_player: PlayerState) -> tuple[PlayerState, ...]:
    """Return new players tuple with updated player at idx."""
    return tuple(
        new_player if i == idx else p
        for i, p in enumerate(players)
    )


def apply_betting_move(state: GameState, move: BettingMove, phase: BettingPhase) -> GameState:
    """Apply a betting move to the state, returning new state.

    Mirrors Go's ApplyBettingAction in betting.go.
    """
    player = state.players[state.active_player]

    if move.action == BettingAction.CHECK:
        return state  # No change

    elif move.action == BettingAction.BET:
        new_player = player.copy_with(
            chips=player.chips - phase.min_bet,
            current_bet=phase.min_bet,
        )
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        return state.copy_with(
            players=new_players,
            pot=state.pot + phase.min_bet,
            current_bet=phase.min_bet,
        )

    elif move.action == BettingAction.CALL:
        to_call = state.current_bet - player.current_bet
        new_player = player.copy_with(
            chips=player.chips - to_call,
            current_bet=state.current_bet,
        )
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        return state.copy_with(
            players=new_players,
            pot=state.pot + to_call,
        )

    elif move.action == BettingAction.RAISE:
        to_call = state.current_bet - player.current_bet
        raise_amount = to_call + phase.min_bet
        new_current_bet = state.current_bet + phase.min_bet
        new_player = player.copy_with(
            chips=player.chips - raise_amount,
            current_bet=new_current_bet,
        )
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        return state.copy_with(
            players=new_players,
            pot=state.pot + raise_amount,
            current_bet=new_current_bet,
            raise_count=state.raise_count + 1,
        )

    elif move.action == BettingAction.ALL_IN:
        amount = player.chips
        new_current_bet = player.current_bet + amount
        new_player = player.copy_with(
            chips=0,
            current_bet=new_current_bet,
            is_all_in=True,
        )
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        game_current_bet = max(state.current_bet, new_current_bet)
        return state.copy_with(
            players=new_players,
            pot=state.pot + amount,
            current_bet=game_current_bet,
        )

    elif move.action == BettingAction.FOLD:
        new_player = player.copy_with(has_folded=True)
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        return state.copy_with(players=new_players)

    return state


def generate_betting_moves(state: GameState, phase: BettingPhase, player_id: int) -> list[BettingMove]:
    """Generate all legal betting moves for a player.

    Mirrors Go's GenerateBettingMoves in betting.go.
    """
    player = state.players[player_id]
    moves: list[BettingMove] = []

    # Can't act if folded, all-in, or no chips
    if player.has_folded or player.is_all_in or player.chips <= 0:
        return moves

    to_call = state.current_bet - player.current_bet
    phase_index = 0  # Will be set by caller if needed

    if to_call == 0:
        # No bet to match
        moves.append(BettingMove(action=BettingAction.CHECK, phase_index=phase_index))
        if player.chips >= phase.min_bet:
            moves.append(BettingMove(action=BettingAction.BET, phase_index=phase_index))
        elif player.chips > 0:
            # Can't afford min bet, but can go all-in
            moves.append(BettingMove(action=BettingAction.ALL_IN, phase_index=phase_index))
    else:
        # Must match, raise, all-in, or fold
        if player.chips >= to_call:
            moves.append(BettingMove(action=BettingAction.CALL, phase_index=phase_index))
            if player.chips >= to_call + phase.min_bet and state.raise_count < phase.max_raises:
                moves.append(BettingMove(action=BettingAction.RAISE, phase_index=phase_index))
        if player.chips > 0 and player.chips < to_call:
            # Can't afford call, but can go all-in
            moves.append(BettingMove(action=BettingAction.ALL_IN, phase_index=phase_index))
        moves.append(BettingMove(action=BettingAction.FOLD, phase_index=phase_index))

    return moves
