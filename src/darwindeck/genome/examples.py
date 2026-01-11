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
    - Ask opponent for cards (simplified to draw from deck)
    - Form sets of 4 matching ranks ("books")
    - Draw if opponent doesn't have requested card
    - Most books wins
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
                # Play books (sets of 4) to tableau
                PlayPhase(
                    target=Location.TABLEAU,
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
            WinCondition(type="empty_hand")
        ],
        scoring_rules=[],
        max_turns=150,
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
    """Create I Doubt It / Cheat card game genome.

    Simplified version without bluffing mechanics:
    - Play cards face down claiming a rank
    - Opponents can challenge (not implemented - simplified)
    - First to empty hand wins
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="cheat",
        generation=0,
        setup=SetupRules(
            cards_per_player=13,
            initial_deck="standard_52",
            initial_discard_count=0
        ),
        turn_structure=TurnStructure(
            phases=[
                # Play cards to discard pile
                PlayPhase(
                    target=Location.DISCARD,
                    valid_play_condition=Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.GT,
                        value=0
                    ),
                    min_cards=1,
                    max_cards=4,  # Can play 1-4 cards
                    mandatory=True,
                    pass_if_unable=False
                )
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(type="empty_hand")
        ],
        scoring_rules=[],
        max_turns=100,
        player_count=4
    )


def create_scopa_genome() -> GameGenome:
    """Create Scopa (Italian capturing game) genome.

    Simplified version without arithmetic sum matching:
    - Capture cards from tableau by matching rank
    - Score points for captured cards
    - Most captures wins
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

    Simplified version without betting:
    - Deal 5 cards
    - Discard and draw to improve hand
    - Best poker hand wins
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
            WinCondition(type="best_hand")  # Poker hand evaluation
        ],
        scoring_rules=[],
        max_turns=20,
        player_count=4
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


def get_seed_genomes() -> List[GameGenome]:
    """Get all seed genomes for initial population in Phase 4.

    Returns a diverse set of 11 games to seed the genetic algorithm:
    - War: Pure luck baseline
    - Hearts: Trick-taking
    - Crazy 8s: Matching with wildcards
    - Gin Rummy: Set collection and melds
    - Old Maid: Pairing and avoidance
    - Go Fish: Set collection
    - Betting War: War variant
    - I Doubt It/Cheat: Bluffing (simplified)
    - Scopa: Capturing game
    - Draw Poker: Hand improvement
    - Scotch Whist: Trick-taking variant
    """
    return [
        create_war_genome(),
        create_hearts_genome(),
        create_crazy_eights_genome(),
        create_gin_rummy_genome(),
        create_old_maid_genome(),
        create_go_fish_genome(),
        create_betting_war_genome(),
        create_cheat_genome(),
        create_scopa_genome(),
        create_draw_poker_genome(),
        create_scotch_whist_genome(),
    ]
