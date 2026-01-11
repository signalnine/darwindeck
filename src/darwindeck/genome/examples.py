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
    WinCondition,
    Location,
    Suit,
    Rank,
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
                    # Always play from top of hand
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
            WinCondition(
                type="capture_all"
            )
        ],
        scoring_rules=[],
        max_turns=1000,
        player_count=2
    )


def create_hearts_genome() -> GameGenome:
    """Create simplified Hearts genome using trick-taking extension.

    Simplified version for validation:
    - 4 players, 13 cards each
    - Must follow suit if able
    - Hearts cannot be led until "broken" (Hearts played when unable to follow suit)
    - Each Heart counts as 1 point (scored automatically)
    - Low score wins when someone reaches 100 points
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="hearts-simplified",
        generation=0,
        setup=SetupRules(
            cards_per_player=13,  # 4 players × 13 = 52 cards
            initial_deck="standard_52",
            initial_discard_count=0,
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
                type="low_score",  # Lowest score wins when threshold reached
                threshold=100  # Game ends when someone reaches 100 points
            ),
            WinCondition(
                type="all_hands_empty",  # Also check if all hands empty (single hand game)
                threshold=0
            )
        ],
        scoring_rules=[],  # Simplified: scoring handled by trick-taking logic
        max_turns=500,     # 13 tricks × 4 cards × multiple hands
        player_count=4,
        min_turns=52       # At least one full hand
    )


def create_crazy_eights_genome() -> GameGenome:
    """Create Crazy 8s card game genome.

    Crazy 8s is a shedding game with:
    - Match suit or rank of discard pile top card
    - 8s are wild (can be played on anything, change suit)
    - Draw if unable to play
    - First to empty hand wins
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="crazy-eights",
        generation=0,
        setup=SetupRules(
            cards_per_player=7,
            initial_deck="standard_52",
            initial_discard_count=1  # Start with one card in discard
        ),
        turn_structure=TurnStructure(
            phases=[
                # Draw if unable to play
                DrawPhase(
                    source=Location.DECK,
                    count=1,
                    mandatory=True,
                    condition=Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.EQ,
                        value=0,  # Has 0 playable cards (simplified - assumes no valid plays)
                        reference="valid_plays"
                    )
                ),
                # Try to play a matching card
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
                    max_cards=1,
                    mandatory=True,
                    pass_if_unable=False  # Must draw if can't play
                )
            ]
        ),
        special_effects=[],  # TODO: Add CHOOSE_SUIT action for 8s when SpecialEffect class is implemented
        win_conditions=[
            WinCondition(type="empty_hand")
        ],
        scoring_rules=[],
        max_turns=200,
        player_count=4
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

    Simplified Old Maid features:
    - Draw from opponent's hand (simplified to draw from deck)
    - Discard pairs of matching ranks
    - Avoid being stuck with the odd card
    - Player with last card loses
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
                # Simplified: draw from deck instead of opponent hand
                DrawPhase(
                    source=Location.DECK,
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

    Simplified Go Fish features:
    - Draw from deck (instead of asking opponent)
    - Play "books" (4 of a kind) to discard pile
    - Score 1 point per book
    - First to empty hand wins (or highest score at deck exhaustion)

    Multi-card plays now supported - when you have 4 cards of the same rank,
    you can play them all at once and score a point.
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="go-fish",
        generation=0,
        setup=SetupRules(
            cards_per_player=7,
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
                # Simplified: draw from deck instead of asking opponent
                DrawPhase(
                    source=Location.DECK,
                    count=1,
                    mandatory=True
                ),
                # Play books (sets of 4) to discard pile for scoring
                PlayPhase(
                    target=Location.DISCARD,
                    valid_play_condition=Condition(
                        type=ConditionType.HAS_SET_OF_N,
                        value=4  # Books of 4
                    ),
                    min_cards=4,
                    max_cards=4,
                    mandatory=False
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="empty_hand"),  # First to empty hand wins
            WinCondition(
                type="high_score",  # Or highest score when deck depletes
                threshold=1  # At least 1 book needed to trigger score win
            )
        ],
        scoring_rules=[],
        max_turns=200,
        player_count=2
    )


def create_betting_war_genome() -> GameGenome:
    """Create Betting War card game genome.

    Simplified version of War with betting (betting mechanics not implemented):
    - Similar to regular War
    - Players compare top cards
    - Higher card wins
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="betting-war",
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

    Simplified Draw Poker features:
    - Deal 5 cards to each player
    - Discard and draw to improve hand
    - Best poker hand wins (compared after drawing phase)

    Hand rankings (high to low):
    - Royal Flush, Straight Flush, Four of a Kind, Full House, Flush
    - Straight, Three of a Kind, Two Pair, One Pair, High Card

    Simplifications:
    - No betting rounds
    - Single draw phase
    - Winner determined immediately after draw
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="draw-poker",
        generation=0,
        setup=SetupRules(
            cards_per_player=5,
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
                # Discard unwanted cards
                DiscardPhase(
                    target=Location.DISCARD,
                    count=3,  # Can discard up to 3
                    mandatory=False
                ),
                # Draw replacement cards
                DrawPhase(
                    source=Location.DECK,
                    count=3,  # Draw same number as discarded
                    mandatory=False,
                    condition=Condition(
                        type=ConditionType.LOCATION_SIZE,
                        reference="discard",
                        operator=Operator.GT,
                        value=0
                    )
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="best_hand")  # Compare poker hands after drawing
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
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="scotch-whist",
        generation=0,
        setup=SetupRules(
            cards_per_player=9,  # Simplified: fewer cards
            initial_deck="standard_52",
            initial_discard_count=0,
            trump_suit=Suit.HEARTS  # Fixed trump for simplicity
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
            tricks_per_hand=9
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="high_score",  # Highest score wins when threshold reached
                threshold=41  # Traditional scoring threshold
            ),
            WinCondition(
                type="all_hands_empty",  # Also check if all hands empty
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
    - Players start with 7 cards
    - Must follow suit if able
    - Trump suit rotates each round
    - Player who wins most tricks in a round stays in
    - Players who win no tricks are eliminated
    - Last player standing wins

    Simplified version:
    - Fixed trump (Hearts)
    - Single round for simulation
    - Most tricks wins
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="knockout-whist",
        generation=0,
        setup=SetupRules(
            cards_per_player=7,
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
        player_count=2
    )


def create_blackjack_genome() -> GameGenome:
    """Create Blackjack/21 card game genome.

    Classic hand-value game:
    - Goal: Get hand value as close to 21 as possible without going over
    - Card values: Number cards = face value, Face cards = 10, Ace = 1 or 11
    - Players choose to "hit" (draw) or "stand" (stop)
    - Going over 21 = "bust" = lose
    - Highest hand value under 21 wins

    Simplified version:
    - 2 players (no dealer distinction)
    - Draw until satisfied or bust
    - Compare final hand values
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="blackjack",
        generation=0,
        setup=SetupRules(
            cards_per_player=2,  # Start with 2 cards each
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
                # Optional draw (hit)
                DrawPhase(
                    source=Location.DECK,
                    count=1,
                    mandatory=False,  # Player chooses to hit or stand
                    condition=Condition(
                        type=ConditionType.HAND_VALUE,
                        operator=Operator.LT,
                        value=21  # Can only hit if not at 21
                    )
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="closest_to_target",  # Closest to 21 without busting
                threshold=21
            )
        ],
        scoring_rules=[],
        max_turns=50,  # Short game - just drawing cards
        player_count=2
    )


def create_fan_tan_genome() -> GameGenome:
    """Create Fan Tan / Sevens card game genome.

    Sequential building layout game:
    - Start by playing 7s to the center
    - Build up (8, 9, 10, J, Q, K, A) and down (6, 5, 4, 3, 2) from 7s
    - Must play if able, otherwise pass
    - First to empty hand wins

    Strategic element: holding key cards (6s, 7s, 8s) blocks opponents.

    Simplified version:
    - Play cards adjacent to existing layout
    - 7s start each suit
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="fan-tan",
        generation=0,
        setup=SetupRules(
            cards_per_player=13,  # Full deck split (4 players)
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
                PlayPhase(
                    target=Location.TABLEAU,
                    valid_play_condition=CompoundCondition(
                        logic="OR",
                        conditions=[
                            # Can play a 7 (starts a suit row)
                            Condition(
                                type=ConditionType.CARD_IS_RANK,
                                value=Rank.SEVEN
                            ),
                            # Can play card adjacent to existing layout
                            Condition(
                                type=ConditionType.CARD_ADJACENT_TO_LAYOUT,
                                reference="tableau"
                            )
                        ]
                    ),
                    min_cards=1,
                    max_cards=1,
                    mandatory=True,
                    pass_if_unable=True  # Pass if no valid play
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="empty_hand")
        ],
        scoring_rules=[],
        max_turns=200,
        player_count=4
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

    Simplified version:
    - Singles only for now
    - 2 is highest, 3 is lowest
    - First to empty hand wins
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="president",
        generation=0,
        setup=SetupRules(
            cards_per_player=13,  # Half deck each for 2 players
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
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
        max_turns=200,
        player_count=2
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


def get_seed_genomes() -> List[GameGenome]:
    """Get all seed genomes for initial population in Phase 4.

    Returns a diverse set of 16 games to seed the genetic algorithm:

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

    Set Collection:
    - Gin Rummy: Set collection and melds
    - Go Fish: Book collection

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
        # Set Collection
        create_gin_rummy_genome(),
        create_go_fish_genome(),
        # Other Mechanics
        create_cheat_genome(),
        create_scopa_genome(),
        create_draw_poker_genome(),
        create_blackjack_genome(),
    ]
