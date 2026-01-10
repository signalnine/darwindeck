"""Example game genomes for testing."""

from cards_evolve.genome.schema import (
    GameGenome,
    SetupRules,
    TurnStructure,
    PlayPhase,
    WinCondition,
    Location,
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
