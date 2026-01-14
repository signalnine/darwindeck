"""Example game genomes for testing."""

from typing import List
from darwindeck.genome.schema import (
    GameGenome,
    SetupRules,
    TurnStructure,
    PlayPhase,
    DrawPhase,
    DiscardPhase,
    TrickPhase,
    BettingPhase,
    WinCondition,
    Location,
    Suit,
    Rank,
    SpecialEffect,
    EffectType,
    TargetSelector,
)
from darwindeck.genome.conditions import Condition, ConditionType, Operator, CompoundCondition


def create_war_genome() -> GameGenome:
    """Create War card game genome.

    War is a pure luck game with:
    - Zero meaningful decisions
    - Simple card comparison
    - Winner-takes-all mechanics
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="war-baseline",
        generation=0,
        setup=SetupRules(
            cards_per_player=26,
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
                PlayPhase(
                    target=Location.TABLEAU,
                    min_cards=1,
                    max_cards=1,
                    mandatory=True,
                    pass_if_unable=False
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="capture_all"
            )
        ],
        scoring_rules=[],
        max_turns=1000,
        player_count=2
    )


def create_hearts_genome() -> GameGenome:
    """Create classic 4-player Hearts genome using trick-taking extension.

    Classic 4-player Hearts:
    - 4 players, 13 cards each (full 52-card deck)
    - Must follow suit if able
    - Hearts cannot be led until "broken" (Hearts played when unable to follow suit)
    - Each Heart counts as 1 point (scored automatically)
    - Lowest score at end of tricks wins
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="hearts-classic",
        generation=0,
        setup=SetupRules(
            cards_per_player=13,  # 4 players × 13 = 52 cards (full deck)
            initial_deck="standard_52",
            initial_discard_count=0,  # Full deck distribution
        ),
        turn_structure=TurnStructure(
            phases=[
                TrickPhase(
                    lead_suit_required=True,   # Must follow suit if able
                    trump_suit=None,            # No trump in Hearts
                    high_card_wins=True,        # High card wins
                    breaking_suit=Suit.HEARTS,  # Hearts cannot be led until broken
                )
            ],
            is_trick_based=True,
            tricks_per_hand=13,  # 13 tricks per hand
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="low_score",  # Lowest score wins (avoid hearts)
                threshold=13  # Max possible hearts
            ),
            WinCondition(
                type="all_hands_empty",
                threshold=0
            )
        ],
        scoring_rules=[],  # Simplified: scoring handled by trick-taking logic
        max_turns=200,     # 13 tricks × 4 players
        player_count=4,    # Classic 4-player format
        min_turns=52       # At least one full hand (13 tricks × 4 players)
    )


def create_crazy_eights_genome() -> GameGenome:
    """Create Crazy 8s card game genome.

    Crazy 8s is a shedding game with:
    - Match suit or rank of discard pile top card
    - 8s are wild (can be played on anything)
    - Draw if unable to play
    - First to empty hand wins

    Modified for better simulation:
    - 2 players for more direct competition
    - Can play multiple cards of same rank (speeds up game, adds decisions)
    - More cards per player for longer games
    - Tableau used for "attack" plays that affect opponent
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="crazy-eights",
        generation=0,
        setup=SetupRules(
            cards_per_player=10,  # More cards = longer game
            initial_deck="standard_52",
            initial_discard_count=1  # Start with one card in discard
        ),
        turn_structure=TurnStructure(
            phases=[
                # Draw from deck (optional - simpler than conditional draw)
                DrawPhase(
                    source=Location.DECK,
                    count=1,
                    mandatory=False  # Let AI decide when to draw
                ),
                # Play matching card(s) - can play multiple of same rank
                PlayPhase(
                    target=Location.DISCARD,
                    valid_play_condition=CompoundCondition(
                        logic="OR",
                        conditions=[
                            Condition(
                                type=ConditionType.CARD_MATCHES_SUIT,
                                reference="top_discard"
                            ),
                            Condition(
                                type=ConditionType.CARD_MATCHES_RANK,
                                reference="top_discard"
                            ),
                            Condition(
                                type=ConditionType.CARD_IS_RANK,
                                value=Rank.EIGHT  # 8s are wild
                            )
                        ]
                    ),
                    min_cards=1,
                    max_cards=4,  # Can play multiple cards of same rank
                    mandatory=True,
                    pass_if_unable=True
                ),
                # Optional: play to tableau (represents "attack" cards)
                PlayPhase(
                    target=Location.TABLEAU,
                    valid_play_condition=Condition(
                        type=ConditionType.CARD_IS_RANK,
                        value=Rank.TWO  # 2s go to tableau (attack cards)
                    ),
                    min_cards=0,
                    max_cards=1,
                    mandatory=False,
                    pass_if_unable=True
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="empty_hand")
        ],
        scoring_rules=[],
        max_turns=500,  # Increased from 200 - shedding games need more turns
        player_count=2
    )


def create_gin_rummy_genome() -> GameGenome:
    """Create simplified Gin Rummy genome.

    Simplified Gin Rummy features:
    - Draw from deck or discard pile
    - Form sets (3-4 of a kind) and runs (3+ sequential cards same suit)
    - Discard one card each turn
    - Go out when hand organized into valid melds
    - Simplified scoring (just winner gets points)
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="gin-rummy-simplified",
        generation=0,
        setup=SetupRules(
            cards_per_player=10,
            initial_deck="standard_52",
            initial_discard_count=1  # Start discard pile
        ),
        turn_structure=TurnStructure(
            phases=[
                # Draw from deck (simplified - no choice of discard pile)
                DrawPhase(
                    source=Location.DECK,
                    count=1,
                    mandatory=True
                ),
                # Optional: play melds to tableau (simplified - no meld validation)
                PlayPhase(
                    target=Location.TABLEAU,
                    valid_play_condition=CompoundCondition(
                        logic="OR",
                        conditions=[
                            # Simplified: allow playing sets or runs (minimal validation)
                            Condition(
                                type=ConditionType.HAND_SIZE,
                                operator=Operator.GE,
                                value=3  # Must have at least 3 cards to form a meld
                            )
                        ]
                    ),
                    min_cards=0,  # Playing melds is optional
                    max_cards=10,
                    mandatory=False
                ),
                # Discard one card
                DiscardPhase(
                    target=Location.DISCARD,
                    count=1,
                    mandatory=True
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="empty_hand"  # Simplified win condition
            )
        ],
        scoring_rules=[],  # TODO: Add scoring when ScoringRule class is implemented
        max_turns=100,
        player_count=2
    )


def create_old_maid_genome() -> GameGenome:
    """Create Old Maid card game genome.

    Old Maid features:
    - Draw from opponent's hand
    - Discard pairs of matching ranks
    - Avoid being stuck with the odd card (queen)
    - First player to empty hand wins
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="old-maid",
        generation=0,
        setup=SetupRules(
            cards_per_player=13,  # Simplified: use even distribution
            initial_deck="standard_52",
            initial_discard_count=1  # Remove one card to create odd
        ),
        turn_structure=TurnStructure(
            phases=[
                # Draw from opponent's hand (core Old Maid mechanic)
                DrawPhase(
                    source=Location.OPPONENT_HAND,
                    count=1,
                    mandatory=True
                ),
                # Discard matching pairs
                DiscardPhase(
                    target=Location.DISCARD,
                    count=2,
                    mandatory=False  # Only if you have a pair
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="empty_hand")
        ],
        scoring_rules=[],
        max_turns=100,
        player_count=2
    )


def create_go_fish_genome() -> GameGenome:
    """Create Go Fish card game genome.

    Modified Go Fish for better simulation:
    - Draw from deck
    - Can play pairs OR books (2+ cards of same rank)
    - Multiple decision points per turn
    - Play to tableau for "asking" simulation (opponent interaction)
    - More cards for longer games

    The tableau play simulates the "asking" mechanic - playing there
    represents revealing what you're looking for.
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="go-fish",
        generation=0,
        setup=SetupRules(
            cards_per_player=10,  # More cards = longer game
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
                # Draw from deck
                DrawPhase(
                    source=Location.DECK,
                    count=1,
                    mandatory=True
                ),
                # Play pairs or sets to tableau ("asking" simulation)
                PlayPhase(
                    target=Location.TABLEAU,
                    valid_play_condition=Condition(
                        type=ConditionType.HAS_MATCHING_PAIR,
                        value=2  # At least a pair
                    ),
                    min_cards=2,
                    max_cards=4,
                    mandatory=False
                ),
                # Play completed books to discard for scoring
                PlayPhase(
                    target=Location.DISCARD,
                    valid_play_condition=Condition(
                        type=ConditionType.HAS_SET_OF_N,
                        value=4  # Books of 4
                    ),
                    min_cards=4,
                    max_cards=4,
                    mandatory=False
                ),
                # Optional discard to cycle cards
                DiscardPhase(
                    target=Location.DISCARD,
                    count=1,
                    mandatory=False
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            # Primary: highest score (most books) wins
            # This ensures ScoreLeaderDetector is used for tension tracking
            WinCondition(
                type="high_score",
                threshold=1
            ),
            # Fallback: if deck runs out and hands empty, game ends
            WinCondition(type="empty_hand"),
        ],
        scoring_rules=[],
        max_turns=200,
        player_count=2
    )


def create_betting_war_genome() -> GameGenome:
    """Create Betting War card game genome.

    War with betting mechanics:
    - Players bet before revealing cards
    - Compare top cards - higher wins the pot
    - Winner takes opponent's chips over time
    - Starting chips: 500, min bet: 10
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="betting-war",
        generation=0,
        setup=SetupRules(
            cards_per_player=26,
            initial_deck="standard_52",
            initial_discard_count=0,
            starting_chips=500,
        ),
        turn_structure=TurnStructure(
            phases=[
                BettingPhase(min_bet=10, max_raises=2),
                PlayPhase(
                    target=Location.TABLEAU,
                    valid_play_condition=Condition(
                        type=ConditionType.LOCATION_SIZE,
                        reference="hand",
                        operator=Operator.GT,
                        value=0
                    ),
                    min_cards=1,
                    max_cards=1,
                    mandatory=True,
                    pass_if_unable=False
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="capture_all")
        ],
        scoring_rules=[],
        max_turns=1000,
        player_count=2
    )


def create_cheat_genome() -> GameGenome:
    """Create I Doubt It / Cheat / BS card game genome.

    The real Cheat game mechanics:
    - Players play cards face-down to discard pile
    - Players claim what rank they're playing (sequential: A, 2, 3, ..., K, A, ...)
    - Can lie about the rank
    - Opponents can challenge ("Cheat!" / "BS!" / "I Doubt It!")
    - If challenged:
      - Claim was TRUE: challenger takes the discard pile
      - Claim was FALSE: claimer takes the discard pile
    - First player to empty their hand wins
    """
    from darwindeck.genome.schema import ClaimPhase

    return GameGenome(
        schema_version="1.0",
        genome_id="cheat",
        generation=0,
        setup=SetupRules(
            cards_per_player=26,  # Half deck each for 2 players
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
                # Claim phase - play cards face-down with a claimed rank
                ClaimPhase(
                    min_cards=1,
                    max_cards=1,  # Simplified: 1 card at a time
                    sequential_rank=True,  # Must claim A, 2, 3, ..., K, A, ...
                    allow_challenge=True,
                    pile_penalty=True  # Loser of challenge takes pile
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="empty_hand")
        ],
        scoring_rules=[],
        max_turns=2000,  # Games can be long with pile pickups and random challenges
        player_count=2
    )


def create_scopa_genome() -> GameGenome:
    """Create Scopa (Italian capturing game) genome.

    Simplified Scopa features:
    - Play card to tableau to capture matching rank
    - Each capture scores 2 points (both cards)
    - When hands empty, draw 3 new cards
    - Game ends when deck and hands are empty
    - Player with most captured cards wins

    Simplifications:
    - Only captures by exact rank match (not sum matching)
    - No "Scopa" bonus for clearing tableau
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="scopa",
        generation=0,
        setup=SetupRules(
            cards_per_player=3,
            initial_deck="standard_52",
            initial_discard_count=4  # Start with 4 cards on tableau
        ),
        turn_structure=TurnStructure(
            phases=[
                # Play card to capture or add to tableau
                PlayPhase(
                    target=Location.TABLEAU,
                    valid_play_condition=Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.GT,
                        value=0
                    ),
                    min_cards=1,
                    max_cards=1,
                    mandatory=True,
                    pass_if_unable=False
                ),
                # Draw new cards when hand empty
                DrawPhase(
                    source=Location.DECK,
                    count=3,
                    mandatory=True,
                    condition=Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.EQ,
                        value=0
                    )
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="most_captured")
        ],
        scoring_rules=[],
        max_turns=100,
        player_count=2
    )


def create_draw_poker_genome() -> GameGenome:
    """Create Draw Poker card game genome.

    Classic 5-card draw poker with betting:
    - Deal 5 cards to each player
    - Betting round before the draw
    - Players can discard and draw new cards
    - Final betting round
    - Best poker hand wins
    - Starting chips: 1000, min bet: 20
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="draw-poker",
        generation=0,
        setup=SetupRules(
            cards_per_player=5,
            initial_deck="standard_52",
            initial_discard_count=0,
            starting_chips=1000,
        ),
        turn_structure=TurnStructure(
            phases=[
                # Pre-draw betting round
                BettingPhase(min_bet=20, max_raises=3),
                # Discard up to 3 cards to improve hand
                DiscardPhase(
                    target=Location.DISCARD,
                    count=3,
                    mandatory=False
                ),
                # Draw replacements
                DrawPhase(
                    source=Location.DECK,
                    count=3,
                    mandatory=False,
                    condition=Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.LT,
                        value=5
                    )
                ),
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="best_hand"),
        ],
        scoring_rules=[],
        max_turns=20,
        player_count=2
    )


def create_scotch_whist_genome() -> GameGenome:
    """Create Scotch Whist (Catch the Ten) card game genome.

    Simplified trick-taking game:
    - Must follow suit if able
    - Trump suit determined
    - High card wins trick
    - Points for capturing certain cards

    Balance fix: Use 13 cards per player (full half-deck) and most_tricks
    win condition. With more cards, the distribution variance is higher,
    reducing first-player advantage. Also using SPADES as trump (more cards
    in that suit reduces trump scarcity advantage).
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="scotch-whist",
        generation=0,
        setup=SetupRules(
            cards_per_player=13,  # Half deck each - more cards = more variance
            initial_deck="standard_52",
            initial_discard_count=0,
            trump_suit=Suit.SPADES  # Spades has good distribution
        ),
        turn_structure=TurnStructure(
            phases=[
                TrickPhase(
                    lead_suit_required=True,
                    trump_suit=Suit.SPADES,
                    high_card_wins=True
                )
            ],
            is_trick_based=True,
            tricks_per_hand=13
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="most_tricks",  # Most tricks wins - simpler and more balanced
                threshold=0
            ),
            WinCondition(
                type="all_hands_empty",
                threshold=0
            )
        ],
        scoring_rules=[],
        max_turns=200,
        player_count=2
    )


def create_knockout_whist_genome() -> GameGenome:
    """Create Knock-Out Whist card game genome.

    Simple elimination trick-taking game:
    - Players start with 7 cards (4 players × 7 = 28 cards dealt)
    - Must follow suit if able
    - Trump suit rotates each round
    - Player who wins most tricks in a round stays in
    - Players who win no tricks are eliminated
    - Last player standing wins

    Simplified version:
    - Fixed trump (Hearts)
    - Single round for simulation
    - Most tricks wins
    - 4 players with 7 cards each (24 cards remaining in deck)
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="knockout-whist",
        generation=0,
        setup=SetupRules(
            cards_per_player=7,  # 4 players × 7 = 28 cards dealt
            initial_deck="standard_52",
            initial_discard_count=0,
            trump_suit=Suit.HEARTS
        ),
        turn_structure=TurnStructure(
            phases=[
                TrickPhase(
                    lead_suit_required=True,
                    trump_suit=Suit.HEARTS,
                    high_card_wins=True
                )
            ],
            is_trick_based=True,
            tricks_per_hand=7
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="most_tricks",  # Most tricks wins
                threshold=0
            ),
            WinCondition(
                type="all_hands_empty",
                threshold=0
            )
        ],
        scoring_rules=[],
        max_turns=100,
        player_count=4
    )


def create_blackjack_genome() -> GameGenome:
    """Create Blackjack/21 card game genome.

    Casino-style blackjack with betting:
    - Players bet before seeing cards
    - Draw cards to get close to 21 without going over
    - Higher hand wins the pot
    - Starting chips: 500, min bet: 25
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="blackjack",
        generation=0,
        setup=SetupRules(
            cards_per_player=2,  # Initial deal
            initial_deck="standard_52",
            initial_discard_count=0,
            starting_chips=500,
        ),
        turn_structure=TurnStructure(
            phases=[
                # Bet before seeing full hand
                BettingPhase(min_bet=25, max_raises=1),
                # Hit - draw a card
                DrawPhase(
                    source=Location.DECK,
                    count=1,
                    mandatory=False,
                    condition=Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.LT,
                        value=5  # Max 5 cards (5-card charlie)
                    )
                ),
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="high_score",
                threshold=21
            ),
        ],
        scoring_rules=[],
        max_turns=20,
        player_count=2
    )


def create_fan_tan_genome() -> GameGenome:
    """Create Fan Tan / Sevens card game genome.

    Simplified shedding game inspired by Fan-Tan:
    - Play cards to tableau to shed them
    - 7s are valuable (can always be played)
    - 6s and 8s can be played (adjacent to 7)
    - Draw if you can't play
    - First to empty hand wins

    Note: Full sequential building requires CARD_ADJACENT_TO_LAYOUT which
    may not be fully implemented. This simplified version captures the
    spirit while ensuring games complete properly.
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="fan-tan",
        generation=0,
        setup=SetupRules(
            cards_per_player=10,  # 2 players × 10 = 20, leaving 32 in deck
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
                # Play a card - 6s, 7s, 8s are key cards
                PlayPhase(
                    target=Location.TABLEAU,
                    valid_play_condition=CompoundCondition(
                        logic="OR",
                        conditions=[
                            Condition(type=ConditionType.CARD_IS_RANK, value=Rank.SIX),
                            Condition(type=ConditionType.CARD_IS_RANK, value=Rank.SEVEN),
                            Condition(type=ConditionType.CARD_IS_RANK, value=Rank.EIGHT),
                        ]
                    ),
                    min_cards=1,
                    max_cards=1,
                    mandatory=True,
                    pass_if_unable=True
                ),
                # Play any other card to discard
                PlayPhase(
                    target=Location.DISCARD,
                    valid_play_condition=Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.GT,
                        value=0
                    ),
                    min_cards=1,
                    max_cards=1,
                    mandatory=False,
                    pass_if_unable=True
                ),
                # Draw if you passed both phases
                DrawPhase(
                    source=Location.DECK,
                    count=1,
                    mandatory=False,
                    condition=Condition(
                        type=ConditionType.LOCATION_SIZE,
                        reference="deck",
                        operator=Operator.GT,
                        value=0
                    )
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="empty_hand")
        ],
        scoring_rules=[],
        max_turns=150,
        player_count=2
    )


def create_president_genome() -> GameGenome:
    """Create President / Daifugō card game genome.

    Climbing/shedding card game:
    - Cards ranked: 3 (low) to 2 (high), with 2 being the highest
    - Play cards that beat the previous play (higher rank)
    - Can play singles, pairs, triples, etc. but must match count
    - Pass if you can't or don't want to beat
    - When all pass, last player to play starts fresh
    - First to empty hand wins (becomes President)

    4-player format (classic):
    - 4 players × 13 cards = 52 cards (full deck dealt)
    - No draw pile in traditional President
    - Rank hierarchy creates interesting dynamics with 4 players
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="president",
        generation=0,
        setup=SetupRules(
            cards_per_player=13,  # 4 players × 13 = 52 cards (full deck)
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
                # Play to beat top card or start new round
                PlayPhase(
                    target=Location.TABLEAU,
                    valid_play_condition=CompoundCondition(
                        logic="OR",
                        conditions=[
                            # Tableau empty - can play anything
                            Condition(
                                type=ConditionType.LOCATION_SIZE,
                                reference="tableau",
                                operator=Operator.EQ,
                                value=0
                            ),
                            # Must beat top card (with 2 high ranking)
                            Condition(
                                type=ConditionType.CARD_BEATS_TOP,
                                reference="tableau",
                                value="two_high"  # Special ranking: 2 is highest
                            )
                        ]
                    ),
                    min_cards=1,
                    max_cards=1,
                    mandatory=True,
                    pass_if_unable=True  # Pass starts new round
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="empty_hand")
        ],
        scoring_rules=[],
        max_turns=300,  # Longer for 4 players
        player_count=4
    )


def create_spades_genome() -> GameGenome:
    """Create Spades card game genome.

    Classic trick-taking with bidding:
    - Spades are always trump
    - Must follow suit if able
    - Spades cannot be led until "broken"
    - Players bid number of tricks they'll win
    - Score points for meeting/exceeding bid
    - Penalty for "bags" (overtricks)

    Simplified version:
    - No bidding (just play tricks)
    - Spades always trump
    - Breaking spades rule included
    - Most tricks wins
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="spades",
        generation=0,
        setup=SetupRules(
            cards_per_player=13,
            initial_deck="standard_52",
            initial_discard_count=0,
            trump_suit=Suit.SPADES
        ),
        turn_structure=TurnStructure(
            phases=[
                TrickPhase(
                    lead_suit_required=True,
                    trump_suit=Suit.SPADES,
                    high_card_wins=True,
                    breaking_suit=Suit.SPADES  # Can't lead spades until broken
                )
            ],
            is_trick_based=True,
            tricks_per_hand=13
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="most_tricks",
                threshold=0
            ),
            WinCondition(
                type="all_hands_empty",
                threshold=0
            )
        ],
        scoring_rules=[],
        max_turns=200,
        player_count=4
    )


def create_uno_genome() -> GameGenome:
    """
    Uno-style game with special effects.

    Mechanics:
    - Match rank or suit of top discard
    - 2s force next player to draw 2
    - Jacks skip next player
    - Queens reverse direction
    - Kings give extra turn
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="uno-style",
        generation=0,
        setup=SetupRules(
            cards_per_player=7,
            initial_discard_count=1,  # Start with one card face up
            custom_printed_deck=True,  # Effects are printed on the cards (no memorization)
        ),
        turn_structure=TurnStructure(phases=[
            PlayPhase(
                target=Location.DISCARD,
                valid_play_condition=CompoundCondition(
                    logic="OR",
                    conditions=[
                        Condition(type=ConditionType.CARD_MATCHES_RANK, reference="top_discard"),
                        Condition(type=ConditionType.CARD_MATCHES_SUIT, reference="top_discard"),
                    ]
                ),
                min_cards=1,
                max_cards=1,
                mandatory=False,  # Can choose not to play
                pass_if_unable=True,
            ),
            DrawPhase(
                source=Location.DECK,
                count=1,
                mandatory=False,  # Can choose not to draw
            ),
        ]),
        special_effects=[
            SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
            SpecialEffect(Rank.JACK, EffectType.SKIP_NEXT, TargetSelector.NEXT_PLAYER, 1),
            SpecialEffect(Rank.QUEEN, EffectType.REVERSE_DIRECTION, TargetSelector.ALL_OPPONENTS, 1),
            SpecialEffect(Rank.KING, EffectType.EXTRA_TURN, TargetSelector.NEXT_PLAYER, 1),
        ],
        win_conditions=[
            WinCondition(type="empty_hand"),
        ],
        scoring_rules=[],
        max_turns=500,  # Increased from 200 - shedding games need more turns
        player_count=2,
    )


def create_simple_poker_genome() -> GameGenome:
    """Create Simple Poker card game genome with betting.

    Simplified 5-card draw poker with betting:
    - Deal 5 cards to each player
    - One betting round
    - Best poker hand wins (or last player standing if others fold)
    - Uses starting chips for betting

    This is the first seed genome to use the betting system.
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="simple-poker",
        generation=0,
        setup=SetupRules(
            cards_per_player=5,
            initial_deck="standard_52",
            initial_discard_count=0,
            starting_chips=1000,  # Enable betting
        ),
        turn_structure=TurnStructure(
            phases=[
                BettingPhase(
                    min_bet=10,
                    max_raises=3,
                ),
            ],
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="best_hand"),  # Best poker hand wins at showdown
        ],
        scoring_rules=[],
        max_turns=10,  # Poker hands are quick
        player_count=2,
    )


def get_seed_genomes() -> List[GameGenome]:
    """Get all seed genomes for initial population in Phase 4.

    Returns a diverse set of 18 games to seed the genetic algorithm:

    Luck-based:
    - War: Pure luck baseline
    - Betting War: War variant

    Trick-taking:
    - Hearts: Trick-taking with penalty cards
    - Scotch Whist: Trump-based trick-taking
    - Knock-Out Whist: Elimination trick-taking
    - Spades: Trick-taking with fixed trump

    Shedding/Matching:
    - Crazy 8s: Matching with wildcards
    - Old Maid: Pairing and avoidance
    - President/Daifugō: Climbing game (2 is high)
    - Fan Tan/Sevens: Sequential building
    - Uno-style: Matching with special effects

    Set Collection:
    - Gin Rummy: Set collection and melds
    - Go Fish: Book collection

    Betting:
    - Simple Poker: First betting game with BettingPhase

    Other Mechanics:
    - Cheat/I Doubt It: Bluffing
    - Scopa: Capturing game
    - Draw Poker: Hand improvement
    - Blackjack: Hand value targeting
    """
    return [
        # Luck-based
        create_war_genome(),
        create_betting_war_genome(),
        # Trick-taking
        create_hearts_genome(),
        create_scotch_whist_genome(),
        create_knockout_whist_genome(),
        create_spades_genome(),
        # Shedding/Matching
        create_crazy_eights_genome(),
        create_old_maid_genome(),
        create_president_genome(),
        create_fan_tan_genome(),
        create_uno_genome(),
        # Set Collection
        create_gin_rummy_genome(),
        create_go_fish_genome(),
        # Betting
        create_simple_poker_genome(),
        # Other Mechanics
        create_cheat_genome(),
        create_scopa_genome(),
        create_draw_poker_genome(),
        create_blackjack_genome(),
    ]
