"""Move generation and application for genome-based games."""

from dataclasses import dataclass
from enum import Enum
from typing import List, Optional, Union
from darwindeck.genome.schema import (
    GameGenome, PlayPhase, Location, BettingPhase, DiscardPhase,
    TrickPhase, ClaimPhase, DrawPhase, SpecialEffect, EffectType, TargetSelector
)
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
    """Generate all legal moves for current player in the current phase."""
    moves: List[Union[LegalMove, BettingMove]] = []
    current_player = state.active_player

    # Only generate moves for the current phase
    phase_idx = state.current_phase
    if phase_idx >= len(genome.turn_structure.phases):
        return moves

    phase = genome.turn_structure.phases[phase_idx]

    # Process just this phase (removed for loop - single phase only)
    if True:  # Keep indentation structure for minimal diff
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
                return moves  # No cards to play

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
                state = resolve_trick(state, phase, genome)
                # resolve_trick handles phase advancement
                return state

            # Trick not complete - advance to next player but stay in same phase
            next_player = (state.active_player + 1) % len(state.players)
            return state.copy_with(
                active_player=next_player,
                turn=state.turn + 1,
            )

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

    # Advance phase and player
    num_phases = len(genome.turn_structure.phases)
    next_phase = state.current_phase + 1

    # If all phases completed for this turn cycle, reset to phase 0 and advance turn
    if next_phase >= num_phases:
        next_phase = 0
        next_player = (state.active_player + 1) % len(state.players)
        new_turn = state.turn + 1
    else:
        # Stay on same player for next phase in their turn
        next_player = state.active_player
        new_turn = state.turn

    return state.copy_with(
        active_player=next_player,
        turn=new_turn,
        current_phase=next_phase
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


def resolve_trick(state: GameState, phase: TrickPhase, genome: GameGenome) -> GameState:
    """Resolve a completed trick.

    The winner is determined by:
    1. Highest trump card (if trump suit is set and trump was played)
    2. Highest card of lead suit

    Winner takes the trick cards and leads next phase.
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

    # Advance to next phase, winner leads
    num_phases = len(genome.turn_structure.phases)
    next_phase = state.current_phase + 1

    # If all phases completed, reset to phase 0
    if next_phase >= num_phases:
        next_phase = 0

    return state.copy_with(
        players=new_players,
        current_trick=(),
        active_player=winner_player_id,
        turn=state.turn + 1,
        current_phase=next_phase,
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
    num_players = len(state.players)

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
            # Highest score wins when anyone reaches threshold
            threshold = wc.threshold if wc.threshold else 0
            triggered = any(p.score >= threshold for p in state.players)
            if triggered:
                max_score = -1
                winner = None
                for player_id, player in enumerate(state.players):
                    if player.score > max_score:
                        max_score = player.score
                        winner = player_id
                if winner is not None:
                    return winner

        elif wc.type == "low_score":
            # Lowest score wins when anyone reaches threshold (Hearts-style)
            threshold = wc.threshold if wc.threshold else 0
            triggered = any(p.score >= threshold for p in state.players)
            if triggered:
                min_score = 999999
                winner = None
                for player_id, player in enumerate(state.players):
                    if player.score < min_score:
                        min_score = player.score
                        winner = player_id
                if winner is not None:
                    return winner

        elif wc.type == "all_hands_empty":
            # Trick-taking: hand ends when all hands empty, lowest score wins
            all_empty = all(len(p.hand) == 0 for p in state.players)
            if all_empty:
                min_score = 999999
                winner = None
                for player_id, player in enumerate(state.players):
                    if player.score < min_score:
                        min_score = player.score
                        winner = player_id
                if winner is not None:
                    return winner

        elif wc.type == "best_hand":
            # Poker: best poker hand wins
            # Only trigger after some turns (draw phase complete)
            all_have_five = all(len(p.hand) == 5 for p in state.players)
            if all_have_five and state.turn >= num_players * 2:
                return find_best_poker_winner(state)

        elif wc.type == "most_captured":
            # Scopa-style: deck empty + hands empty, highest score wins
            deck_empty = len(state.deck) == 0
            hands_empty = all(len(p.hand) == 0 for p in state.players)
            if deck_empty and hands_empty:
                max_score = -1
                winner = None
                for player_id, player in enumerate(state.players):
                    if player.score > max_score:
                        max_score = player.score
                        winner = player_id
                if winner is not None:
                    return winner

    return None


# Poker hand evaluation for best_hand win condition

class PokerHandRank:
    """Poker hand rankings (higher is better)."""
    HIGH_CARD = 0
    ONE_PAIR = 1
    TWO_PAIR = 2
    THREE_OF_A_KIND = 3
    STRAIGHT = 4
    FLUSH = 5
    FULL_HOUSE = 6
    FOUR_OF_A_KIND = 7
    STRAIGHT_FLUSH = 8


def evaluate_poker_hand(cards: tuple[Card, ...]) -> tuple[int, list[int]]:
    """Evaluate a 5-card poker hand.

    Returns (rank, kickers) where rank is PokerHandRank and kickers
    are values for tiebreaking (highest first).
    """
    if len(cards) != 5:
        return (PokerHandRank.HIGH_CARD, [])

    # Sort by rank descending
    sorted_cards = sorted(cards, key=lambda c: RANK_VALUES[c.rank.value], reverse=True)
    ranks = [RANK_VALUES[c.rank.value] for c in sorted_cards]
    suits = [c.suit.value for c in sorted_cards]

    # Check flush
    is_flush = len(set(suits)) == 1

    # Check straight
    is_straight = False
    if ranks == [ranks[0] - i for i in range(5)]:
        is_straight = True
    # Wheel straight: A-2-3-4-5
    elif ranks == [14, 5, 4, 3, 2]:
        is_straight = True
        ranks = [5, 4, 3, 2, 1]  # Treat ace as 1 for wheel

    # Count ranks
    rank_counts: dict[int, int] = {}
    for r in ranks:
        rank_counts[r] = rank_counts.get(r, 0) + 1

    counts = sorted(rank_counts.values(), reverse=True)
    # Sort by count then by rank value
    count_rank_pairs = sorted(
        [(count, rank) for rank, count in rank_counts.items()],
        key=lambda x: (x[0], x[1]),
        reverse=True
    )
    kickers = [rank for _, rank in count_rank_pairs]

    # Determine hand rank
    if is_straight and is_flush:
        return (PokerHandRank.STRAIGHT_FLUSH, [max(ranks)])
    if counts == [4, 1]:
        return (PokerHandRank.FOUR_OF_A_KIND, kickers)
    if counts == [3, 2]:
        return (PokerHandRank.FULL_HOUSE, kickers)
    if is_flush:
        return (PokerHandRank.FLUSH, ranks)
    if is_straight:
        return (PokerHandRank.STRAIGHT, [max(ranks)])
    if counts == [3, 1, 1]:
        return (PokerHandRank.THREE_OF_A_KIND, kickers)
    if counts == [2, 2, 1]:
        return (PokerHandRank.TWO_PAIR, kickers)
    if counts == [2, 1, 1, 1]:
        return (PokerHandRank.ONE_PAIR, kickers)

    return (PokerHandRank.HIGH_CARD, ranks)


def compare_poker_hands(hand1: tuple[int, list[int]], hand2: tuple[int, list[int]]) -> int:
    """Compare two poker hands. Returns 1 if hand1 wins, -1 if hand2 wins, 0 for tie."""
    rank1, kickers1 = hand1
    rank2, kickers2 = hand2

    if rank1 > rank2:
        return 1
    if rank1 < rank2:
        return -1

    # Same rank, compare kickers
    for k1, k2 in zip(kickers1, kickers2):
        if k1 > k2:
            return 1
        if k1 < k2:
            return -1

    return 0


def find_best_poker_winner(state: GameState) -> Optional[int]:
    """Find player with best poker hand. Returns player ID or None."""
    best_player = None
    best_hand: Optional[tuple[int, list[int]]] = None

    for player_id, player in enumerate(state.players):
        if len(player.hand) != 5:
            continue

        poker_hand = evaluate_poker_hand(player.hand)

        if best_player is None:
            best_player = player_id
            best_hand = poker_hand
        else:
            cmp = compare_poker_hands(poker_hand, best_hand)  # type: ignore
            if cmp > 0:
                best_player = player_id
                best_hand = poker_hand

    return best_player


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
        new_chips = player.chips - phase.min_bet
        new_player = player.copy_with(
            chips=new_chips,
            current_bet=phase.min_bet,
            is_all_in=new_chips <= 0,
        )
        new_players = _update_player_tuple(state.players, state.active_player, new_player)
        return state.copy_with(
            players=new_players,
            pot=state.pot + phase.min_bet,
            current_bet=phase.min_bet,
        )

    elif move.action == BettingAction.CALL:
        to_call = state.current_bet - player.current_bet
        # Guard against negative to_call (defensive)
        to_call = max(0, to_call)
        new_chips = player.chips - to_call
        new_player = player.copy_with(
            chips=new_chips,
            current_bet=state.current_bet,
            is_all_in=new_chips <= 0,
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
        new_chips = player.chips - raise_amount
        new_player = player.copy_with(
            chips=new_chips,
            current_bet=new_current_bet,
            is_all_in=new_chips <= 0,
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


def count_acting_players(state: GameState) -> int:
    """Count players who can still take betting actions.

    A player can act if they:
    - Have not folded
    - Are not all-in
    - Have chips remaining
    """
    count = 0
    for player in state.players:
        if not player.has_folded and not player.is_all_in and player.chips > 0:
            count += 1
    return count


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


# =============================================================================
# Special Effect Handling
# =============================================================================


def resolve_target(state: GameState, target: TargetSelector) -> int:
    """Resolve which player ID an effect targets.

    Args:
        state: Current game state
        target: Target selector from the effect

    Returns:
        Player ID of the target, or -1 for ALL_OPPONENTS
    """
    current = state.active_player
    num_players = len(state.players)
    direction = state.play_direction

    if target == TargetSelector.SELF:
        return current
    elif target == TargetSelector.NEXT_PLAYER:
        return (current + direction + num_players) % num_players
    elif target == TargetSelector.PREV_PLAYER:
        return (current - direction + num_players) % num_players
    elif target == TargetSelector.ALL_OPPONENTS:
        return -1  # Signals caller must loop over all opponents
    elif target == TargetSelector.LEFT_OPPONENT:
        return (current + 1 + num_players) % num_players
    elif target == TargetSelector.RIGHT_OPPONENT:
        return (current - 1 + num_players) % num_players
    else:
        # Default to next player for unknown targets
        return (current + 1) % num_players


def apply_effect(state: GameState, effect: SpecialEffect) -> GameState:
    """Apply a special effect to the game state.

    Handles all effect types including:
    - SKIP_NEXT: Skip next player(s)
    - REVERSE_DIRECTION: Reverse play direction
    - DRAW_CARDS: Target draws cards
    - EXTRA_TURN: Current player gets extra turn
    - FORCE_DISCARD: Target discards cards
    - WILD_CARD: No-op (handled by condition evaluation)
    - BLOCK_NEXT: Block next player (similar to skip)
    - SWAP_HANDS: Exchange hands with target
    - STEAL_CARD: Take card(s) from target
    - PEEK_HAND: No-op (UI-only effect)

    Args:
        state: Current game state
        effect: The special effect to apply

    Returns:
        New game state with effect applied
    """
    effect_type = effect.effect_type
    value = effect.value
    target = effect.target

    if effect_type == EffectType.SKIP_NEXT:
        return _apply_skip_next(state, value)

    elif effect_type == EffectType.BLOCK_NEXT:
        # BLOCK_NEXT is similar to SKIP_NEXT
        return _apply_skip_next(state, value)

    elif effect_type == EffectType.REVERSE_DIRECTION:
        return state.copy_with(play_direction=state.play_direction * -1)

    elif effect_type == EffectType.DRAW_CARDS:
        return _apply_draw_cards(state, target, value)

    elif effect_type == EffectType.EXTRA_TURN:
        # Skip everyone else = current player goes again
        num_players = len(state.players)
        return state.copy_with(skip_count=num_players - 1)

    elif effect_type == EffectType.FORCE_DISCARD:
        return _apply_force_discard(state, target, value)

    elif effect_type == EffectType.WILD_CARD:
        # WILD_CARD is handled by condition evaluation, not as state mutation
        return state

    elif effect_type == EffectType.SWAP_HANDS:
        return _apply_swap_hands(state, target)

    elif effect_type == EffectType.STEAL_CARD:
        return _apply_steal_card(state, target, value)

    elif effect_type == EffectType.PEEK_HAND:
        # PEEK_HAND is UI-only, no mechanical effect
        return state

    else:
        # Unknown effect type - ignore for forward compatibility
        return state


def _apply_skip_next(state: GameState, value: int) -> GameState:
    """Apply skip effect, capping at num_players - 1."""
    new_skip = state.skip_count + value
    max_skip = len(state.players) - 1
    if new_skip > max_skip:
        new_skip = max_skip
    return state.copy_with(skip_count=new_skip)


def _apply_draw_cards(state: GameState, target: TargetSelector, count: int) -> GameState:
    """Make target player(s) draw cards from deck."""
    target_id = resolve_target(state, target)

    if target_id == -1:
        # ALL_OPPONENTS: apply to everyone except current player
        for i in range(len(state.players)):
            if i != state.active_player:
                state = _draw_cards_for_player(state, i, count)
    else:
        state = _draw_cards_for_player(state, target_id, count)

    return state


def _draw_cards_for_player(state: GameState, player_id: int, count: int) -> GameState:
    """Draw cards from deck into player's hand."""
    player = state.players[player_id]
    deck = state.deck
    hand = player.hand

    drawn = 0
    new_deck = list(deck)
    new_hand = list(hand)

    for _ in range(count):
        if len(new_deck) == 0:
            break
        card = new_deck.pop(0)
        new_hand.append(card)
        drawn += 1

    new_player = player.copy_with(hand=tuple(new_hand))
    new_players = _update_player_tuple(state.players, player_id, new_player)
    return state.copy_with(players=new_players, deck=tuple(new_deck))


def _apply_force_discard(state: GameState, target: TargetSelector, count: int) -> GameState:
    """Force target player(s) to discard cards."""
    target_id = resolve_target(state, target)

    if target_id == -1:
        # ALL_OPPONENTS: apply to everyone except current player
        for i in range(len(state.players)):
            if i != state.active_player:
                state = _force_discard_for_player(state, i, count)
    else:
        state = _force_discard_for_player(state, target_id, count)

    return state


def _force_discard_for_player(state: GameState, player_id: int, count: int) -> GameState:
    """Force a specific player to discard cards (from end of hand)."""
    player = state.players[player_id]
    hand = list(player.hand)
    discard = list(state.discard)

    to_discard = min(count, len(hand))
    for _ in range(to_discard):
        if hand:
            card = hand.pop()  # Remove from end (deterministic)
            discard.append(card)

    new_player = player.copy_with(hand=tuple(hand))
    new_players = _update_player_tuple(state.players, player_id, new_player)
    return state.copy_with(players=new_players, discard=tuple(discard))


def _apply_swap_hands(state: GameState, target: TargetSelector) -> GameState:
    """Swap hands between active player and target."""
    target_id = resolve_target(state, target)

    # Can't swap with ALL_OPPONENTS or with self
    if target_id == -1:
        return state  # ALL_OPPONENTS not supported for swap
    if target_id == state.active_player:
        return state  # Swapping with self is a no-op

    active_player = state.players[state.active_player]
    target_player = state.players[target_id]

    # Swap hands
    new_active = active_player.copy_with(hand=target_player.hand)
    new_target = target_player.copy_with(hand=active_player.hand)

    # Update players tuple
    new_players = tuple(
        new_active if i == state.active_player
        else (new_target if i == target_id else p)
        for i, p in enumerate(state.players)
    )

    return state.copy_with(players=new_players)


def _apply_steal_card(state: GameState, target: TargetSelector, count: int) -> GameState:
    """Steal card(s) from target's hand (deterministically from end)."""
    target_id = resolve_target(state, target)

    # Can't steal from ALL_OPPONENTS or from self
    if target_id == -1:
        return state
    if target_id == state.active_player:
        return state

    active_player = state.players[state.active_player]
    target_player = state.players[target_id]

    target_hand = list(target_player.hand)
    active_hand = list(active_player.hand)

    # Steal cards from the end of target's hand (deterministic)
    to_steal = min(count, len(target_hand))
    for _ in range(to_steal):
        if target_hand:
            card = target_hand.pop()  # Remove from end
            active_hand.append(card)

    new_active = active_player.copy_with(hand=tuple(active_hand))
    new_target = target_player.copy_with(hand=tuple(target_hand))

    # Update players tuple
    new_players = tuple(
        new_active if i == state.active_player
        else (new_target if i == target_id else p)
        for i, p in enumerate(state.players)
    )

    return state.copy_with(players=new_players)
