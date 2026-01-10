"""Example game genomes for testing."""

from typing import List
from cards_evolve.genome.schema import (
    GameGenome,
    SetupRules,
    TurnStructure,
    PlayPhase,
    DrawPhase,
    DiscardPhase,
    TrickPhase,
    WinCondition,
    ScoringRule,
    SpecialEffect,
    Action,
    ActionType,
    Location,
    Suit,
    Rank,
)
from cards_evolve.genome.conditions import Condition, ConditionType, Operator


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
                type="first_to_score",
                threshold=100  # Game ends when someone reaches 100 points
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
                # Try to play a matching card
                PlayPhase(
                    target=Location.DISCARD,
                    valid_play_condition=Condition(
                        type=ConditionType.CARD_MATCHES_TOP,
                        operator=Operator.OR,
                        sub_conditions=[
                            Condition(type=ConditionType.CARD_MATCHES_SUIT),
                            Condition(type=ConditionType.CARD_MATCHES_RANK),
                            Condition(type=ConditionType.CARD_IS_RANK, value=Rank.EIGHT),
                        ]
                    ),
                    min_cards=1,
                    max_cards=1,
                    mandatory=False,
                    pass_if_unable=True
                ),
                # If unable to play, draw from deck
                DrawPhase(
                    source=Location.DECK,
                    count=1,
                    mandatory=True,
                    condition=Condition(
                        type=ConditionType.NO_VALID_PLAY
                    )
                )
            ]
        ),
        special_effects=[
            # 8s allow suit selection (wild card)
            SpecialEffect(
                trigger_card=Rank.EIGHT,
                actions=[
                    Action(
                        type=ActionType.CHOOSE_SUIT,
                        description="Player chooses new suit"
                    )
                ]
            )
        ],
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
                # Draw from deck or discard pile
                DrawPhase(
                    source=Location.DECK,  # Default: deck
                    count=1,
                    mandatory=True,
                    allow_source_choice=True,  # Can choose discard instead
                    alternate_source=Location.DISCARD
                ),
                # Optional: play melds to tableau
                PlayPhase(
                    target=Location.TABLEAU,
                    valid_play_condition=Condition(
                        type=ConditionType.HAS_VALID_MELD,
                        operator=Operator.OR,
                        sub_conditions=[
                            Condition(type=ConditionType.HAS_SET_OF_N, value=3),
                            Condition(type=ConditionType.HAS_RUN_OF_N, value=3),
                        ]
                    ),
                    min_cards=3,
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
                type="all_cards_melded",
                description="All cards in valid sets/runs"
            )
        ],
        scoring_rules=[
            ScoringRule(
                description="Winner scores opponent's deadwood",
                condition=Condition(type=ConditionType.HAND_WINNER),
                points=10  # Simplified: fixed points
            )
        ],
        max_turns=100,
        player_count=2
    )


def get_seed_genomes() -> List[GameGenome]:
    """Get all seed genomes for initial population in Phase 4.

    Returns a diverse set of simple games to seed the genetic algorithm:
    - War: Pure luck baseline
    - Crazy 8s: Matching with special effects
    - Gin Rummy: Set collection and melds
    - Hearts: Trick-taking (after Phase 3.5 TrickPhase implementation)
    """
    return [
        create_war_genome(),
        create_crazy_eights_genome(),
        create_gin_rummy_genome(),
        # create_hearts_genome(),  # Uncomment after Phase 3.5 TrickPhase
    ]
