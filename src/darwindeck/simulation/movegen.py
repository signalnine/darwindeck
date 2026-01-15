"""Move generation and application for genome-based games."""

from dataclasses import dataclass
from enum import Enum
from typing import List, Optional, Union
from darwindeck.genome.schema import GameGenome, PlayPhase, Location, BettingPhase, DiscardPhase, TrickPhase, ClaimPhase, DrawPhase
from darwindeck.simulation.state import GameState, Card, PlayerState, TrickCard, Claim

# Special CardIndex values for ClaimPhase
MOVE_CHALLENGE = -1  # Challenge the current claim
MOVE_CLAIM_PASS = -2  # Accept the claim without challenging

# Special CardIndex values for DrawPhase
MOVE_DRAW = -1  # Draw a card
MOVE_DRAW_PASS = -3  # Skip drawing (stand)


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


def generate_legal_moves(state: GameState, genome: GameGenome) -> List[Union[LegalMove, BettingMove]]:
    """Generate all legal moves for current player."""
    moves: List[Union[LegalMove, BettingMove]] = []
    current_player = state.active_player

    for phase_idx, phase in enumerate(genome.turn_structure.phases):
        if isinstance(phase, BettingPhase):
            # Generate betting moves
            betting_moves = generate_betting_moves(state, phase, current_player)
            # Set correct phase_index
            for bm in betting_moves:
                moves.append(BettingMove(action=bm.action, phase_index=phase_idx))

        elif isinstance(phase, PlayPhase):
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

        elif isinstance(phase, DiscardPhase):
            # DiscardPhase: discard cards from hand
            target = phase.target
            hand = state.players[current_player].hand

            if len(hand) > 0:
                # Generate moves to discard each card
                for card_idx in range(len(hand)):
                    # TODO: Evaluate matching_condition
                    moves.append(LegalMove(
                        phase_index=phase_idx,
                        card_index=card_idx,
                        target_loc=target
                    ))

            # If not mandatory and hand is empty (or we want to skip), allow pass
            if not phase.mandatory:
                # Pass move represented by card_index=-1
                moves.append(LegalMove(
                    phase_index=phase_idx,
                    card_index=-1,
                    target_loc=target
                ))

        elif isinstance(phase, TrickPhase):
            # TrickPhase: play cards following suit rules
            hand = state.players[current_player].hand
            if len(hand) == 0:
                continue

            # Determine if we're leading or following
            is_leading = len(state.current_trick) == 0

            if is_leading:
                # Leading: can play any card, except breaking suit until broken
                for card_idx, card in enumerate(hand):
                    # If breaking suit (e.g., Hearts) and not broken yet
                    if phase.breaking_suit is not None and card.suit == phase.breaking_suit and not state.hearts_broken:
                        # Check if player has any non-breaking suit cards
                        has_other = any(c.suit != phase.breaking_suit for c in hand)
                        if has_other:
                            continue  # Can't lead breaking suit
                        # If only breaking suit cards, can lead them

                    moves.append(LegalMove(
                        phase_index=phase_idx,
                        card_index=card_idx,
                        target_loc=Location.TABLEAU,
                    ))
            else:
                # Following: must follow suit if required and able
                lead_suit = state.current_trick[0].card.suit

                if phase.lead_suit_required:
                    # Check if we have cards of lead suit
                    has_lead_suit = any(card.suit == lead_suit for card in hand)

                    if has_lead_suit:
                        # Must follow suit
                        for card_idx, card in enumerate(hand):
                            if card.suit == lead_suit:
                                moves.append(LegalMove(
                                    phase_index=phase_idx,
                                    card_index=card_idx,
                                    target_loc=Location.TABLEAU,
                                ))
                    else:
                        # Can't follow suit - can play any card
                        for card_idx in range(len(hand)):
                            moves.append(LegalMove(
                                phase_index=phase_idx,
                                card_index=card_idx,
                                target_loc=Location.TABLEAU,
                            ))
                else:
                    # No suit following required - can play any card
                    for card_idx in range(len(hand)):
                        moves.append(LegalMove(
                            phase_index=phase_idx,
                            card_index=card_idx,
                            target_loc=Location.TABLEAU,
                        ))

        elif isinstance(phase, ClaimPhase):
            # ClaimPhase: bluffing/Cheat mechanics
            if state.current_claim is None:
                # No active claim - current player makes a claim
                hand = state.players[current_player].hand
                if len(hand) > 0:
                    for card_idx in range(len(hand)):
                        moves.append(LegalMove(
                            phase_index=phase_idx,
                            card_index=card_idx,
                            target_loc=Location.DISCARD,
                        ))
            else:
                # Active claim exists - opponent responds
                if current_player != state.current_claim.claimer_id:
                    # Can challenge or pass
                    moves.append(LegalMove(
                        phase_index=phase_idx,
                        card_index=MOVE_CHALLENGE,
                        target_loc=Location.DISCARD,
                    ))
                    moves.append(LegalMove(
                        phase_index=phase_idx,
                        card_index=MOVE_CLAIM_PASS,
                        target_loc=Location.DISCARD,
                    ))

        elif isinstance(phase, DrawPhase):
            # DrawPhase: draw cards from deck/discard/opponent
            source = phase.source

            # Check if can draw from source
            can_draw = False
            if source == Location.DECK:
                can_draw = len(state.deck) > 0
            elif source == Location.DISCARD:
                can_draw = len(state.discard) > 0
            elif source == Location.OPPONENT_HAND:
                opponent_id = (current_player + 1) % len(state.players)
                can_draw = len(state.players[opponent_id].hand) > 0

            if can_draw:
                moves.append(LegalMove(
                    phase_index=phase_idx,
                    card_index=MOVE_DRAW,
                    target_loc=source,
                ))

            # Add pass/stand option when drawing is not mandatory
            if not phase.mandatory:
                moves.append(LegalMove(
                    phase_index=phase_idx,
                    card_index=MOVE_DRAW_PASS,
                    target_loc=source,
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

    elif isinstance(phase, DiscardPhase):
        if move.card_index >= 0:
            # Discard card from hand to target location
            state = play_card(state, current_player, move.card_index, move.target_loc)
        # card_index == -1 means pass (no discard)

    elif isinstance(phase, TrickPhase):
        if move.card_index >= 0 and move.card_index < len(state.players[current_player].hand):
            card = state.players[current_player].hand[move.card_index]

            # Remove card from hand
            player = state.players[current_player]
            new_hand = player.hand[:move.card_index] + player.hand[move.card_index+1:]
            new_player = player.copy_with(hand=new_hand)
            new_players = tuple(
                new_player if i == current_player else p
                for i, p in enumerate(state.players)
            )

            # Add to current trick
            new_trick = state.current_trick + (TrickCard(player_id=current_player, card=card),)

            # Check if this card breaks hearts
            hearts_broken = state.hearts_broken
            if phase.breaking_suit is not None and card.suit == phase.breaking_suit:
                hearts_broken = True

            state = state.copy_with(
                players=new_players,
                current_trick=new_trick,
                hearts_broken=hearts_broken,
            )

            # Check if trick is complete
            num_players = len(state.players)
            if len(state.current_trick) >= num_players:
                # Resolve trick - winner takes the cards
                state = resolve_trick(state, phase)
                # Don't advance turn normally - resolve_trick sets next player
                return state

    elif isinstance(phase, ClaimPhase):
        if move.card_index >= 0:
            # Making a claim - play card and create claim
            hand = state.players[current_player].hand
            if move.card_index < len(hand):
                card = hand[move.card_index]

                # Remove card from hand
                new_hand = hand[:move.card_index] + hand[move.card_index+1:]
                new_player = state.players[current_player].copy_with(hand=new_hand)
                new_players = tuple(
                    new_player if i == current_player else p
                    for i, p in enumerate(state.players)
                )

                # Add to discard pile (face-down conceptually)
                new_discard = state.discard + (card,)

                # Create claim - claimed rank is sequential based on turn number
                # Rank maps: 0=A, 1=2, 2=3, ..., 12=K
                claimed_rank = state.turn % 13

                new_claim = Claim(
                    claimer_id=current_player,
                    claimed_rank=claimed_rank,
                    claimed_count=1,
                    cards_played=(card,),
                )

                state = state.copy_with(
                    players=new_players,
                    discard=new_discard,
                    current_claim=new_claim,
                )

        elif move.card_index == MOVE_CHALLENGE:
            # Challenge the claim
            if state.current_claim is not None:
                state = resolve_challenge(state, current_player)
                # After challenge resolves, current player makes the next claim
                return state.copy_with(turn=state.turn + 1)

        elif move.card_index == MOVE_CLAIM_PASS:
            # Accept claim - clear it, cards stay in discard
            state = state.copy_with(current_claim=None)
            # After pass, current player makes the next claim
            return state.copy_with(turn=state.turn + 1)

    elif isinstance(phase, DrawPhase):
        if move.card_index == MOVE_DRAW:
            # Draw card(s) from source
            source = phase.source
            count = phase.count

            for _ in range(count):
                state = draw_card(state, current_player, source)

        # MOVE_DRAW_PASS means skip drawing - no state change needed

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


def draw_card(state: GameState, player_id: int, source: Location) -> GameState:
    """Draw a card from source location to player's hand."""
    player = state.players[player_id]

    if source == Location.DECK:
        if len(state.deck) == 0:
            return state
        card = state.deck[0]
        new_deck = state.deck[1:]
        new_hand = player.hand + (card,)
        new_player = player.copy_with(hand=new_hand)
        new_players = tuple(
            new_player if i == player_id else p
            for i, p in enumerate(state.players)
        )
        return state.copy_with(players=new_players, deck=new_deck)

    elif source == Location.DISCARD:
        if len(state.discard) == 0:
            return state
        card = state.discard[-1]  # Draw from top of discard
        new_discard = state.discard[:-1]
        new_hand = player.hand + (card,)
        new_player = player.copy_with(hand=new_hand)
        new_players = tuple(
            new_player if i == player_id else p
            for i, p in enumerate(state.players)
        )
        return state.copy_with(players=new_players, discard=new_discard)

    elif source == Location.OPPONENT_HAND:
        # Draw random card from next player's hand
        opponent_id = (player_id + 1) % len(state.players)
        opponent = state.players[opponent_id]
        if len(opponent.hand) == 0:
            return state
        # Draw from end (random in actual play, but deterministic here)
        card = opponent.hand[-1]
        new_opponent_hand = opponent.hand[:-1]
        new_opponent = opponent.copy_with(hand=new_opponent_hand)
        new_hand = player.hand + (card,)
        new_player = player.copy_with(hand=new_hand)
        new_players = tuple(
            new_player if i == player_id else (new_opponent if i == opponent_id else p)
            for i, p in enumerate(state.players)
        )
        return state.copy_with(players=new_players)

    return state


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


def resolve_trick(state: GameState, phase: TrickPhase) -> GameState:
    """Resolve a completed trick.

    The winner is determined by:
    1. Highest trump card (if trump suit is set and trump was played)
    2. Highest card of lead suit

    Winner takes the trick cards and leads next.
    """
    if len(state.current_trick) == 0:
        return state

    lead_suit = state.current_trick[0].card.suit
    trump_suit = phase.trump_suit
    high_card_wins = phase.high_card_wins

    # Find winner
    winner_idx = 0
    winner_card = state.current_trick[0].card
    winner_value = get_rank_value(winner_card)
    winner_is_trump = trump_suit is not None and winner_card.suit == trump_suit

    for i, trick_card in enumerate(state.current_trick[1:], 1):
        card = trick_card.card
        card_value = get_rank_value(card)
        card_is_trump = trump_suit is not None and card.suit == trump_suit

        # Determine if this card beats current winner
        beats_winner = False

        if card_is_trump and not winner_is_trump:
            # Trump beats non-trump
            beats_winner = True
        elif card_is_trump and winner_is_trump:
            # Both trump - compare values
            if high_card_wins:
                beats_winner = card_value > winner_value
            else:
                beats_winner = card_value < winner_value
        elif not card_is_trump and not winner_is_trump:
            # Neither is trump - must follow lead suit to win
            if card.suit == lead_suit and winner_card.suit == lead_suit:
                if high_card_wins:
                    beats_winner = card_value > winner_value
                else:
                    beats_winner = card_value < winner_value
            elif card.suit == lead_suit and winner_card.suit != lead_suit:
                # This card follows lead, winner doesn't - this card wins
                beats_winner = True
            # If neither follows lead, first one wins (winner stays)

        if beats_winner:
            winner_idx = i
            winner_card = card
            winner_value = card_value
            winner_is_trump = card_is_trump

    # Winner's player ID
    winner_player_id = state.current_trick[winner_idx].player_id

    # Award trick to winner (add cards to their score or captured pile)
    # For Hearts-style games, taking tricks adds to score (bad)
    # For now, just track who won (scoring handled elsewhere)
    trick_cards = tuple(tc.card for tc in state.current_trick)

    # Update winner's score based on cards taken
    # Simple scoring: each card = 1 point (customize for specific games)
    winner_player = state.players[winner_player_id]
    new_score = winner_player.score + len(trick_cards)
    new_winner = winner_player.copy_with(score=new_score)

    new_players = tuple(
        new_winner if i == winner_player_id else p
        for i, p in enumerate(state.players)
    )

    # Clear current trick and set winner as next player
    return state.copy_with(
        players=new_players,
        current_trick=(),
        active_player=winner_player_id,
        turn=state.turn + 1,
    )


def resolve_challenge(state: GameState, challenger_id: int) -> GameState:
    """Resolve a challenge in ClaimPhase.

    If claim was TRUE (cards match claimed rank), challenger takes pile.
    If claim was FALSE (cards don't match), claimer takes pile.
    """
    if state.current_claim is None:
        return state

    claim = state.current_claim
    claimer_id = claim.claimer_id

    # Rank value mapping for comparison
    rank_to_index = {
        "A": 0, "2": 1, "3": 2, "4": 3, "5": 4, "6": 5, "7": 6,
        "8": 7, "9": 8, "10": 9, "J": 10, "Q": 11, "K": 12
    }

    # Check if the claim was truthful
    truthful = True
    for card in claim.cards_played:
        card_rank_idx = rank_to_index.get(card.rank.value, -1)
        if card_rank_idx != claim.claimed_rank:
            truthful = False
            break

    # Determine loser
    if truthful:
        # Claim was true - challenger was wrong, takes the pile
        loser_id = challenger_id
    else:
        # Claim was false - claimer was lying, takes the pile
        loser_id = claimer_id

    # Loser takes entire discard pile
    loser_player = state.players[loser_id]
    new_hand = loser_player.hand + state.discard
    new_loser = loser_player.copy_with(hand=new_hand)

    new_players = tuple(
        new_loser if i == loser_id else p
        for i, p in enumerate(state.players)
    )

    # Clear the claim and discard
    return state.copy_with(
        players=new_players,
        discard=(),
        current_claim=None,
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


def count_active_players(state: GameState) -> int:
    """Count players who haven't folded."""
    return sum(1 for p in state.players if not p.has_folded)


def all_bets_matched(state: GameState) -> bool:
    """Check if all active players have matched the current bet."""
    for p in state.players:
        if not p.has_folded and not p.is_all_in:
            if p.current_bet != state.current_bet:
                return False
    return True


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
