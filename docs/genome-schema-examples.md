# Enhanced Genome Schema with Known Game Examples

**Date:** 2026-01-10
**Status:** Design validation - Path A (Enhanced Dataclasses)

This document defines the enhanced genome schema and validates it by encoding known card games.

## Core Schema Types

### Enumerations

```python
class Rank(Enum):
    ACE = "A"
    TWO = "2"
    THREE = "3"
    FOUR = "4"
    FIVE = "5"
    SIX = "6"
    SEVEN = "7"
    EIGHT = "8"
    NINE = "9"
    TEN = "10"
    JACK = "J"
    QUEEN = "Q"
    KING = "K"

class Suit(Enum):
    HEARTS = "H"
    DIAMONDS = "D"
    CLUBS = "C"
    SPADES = "S"

class Location(Enum):
    DECK = "deck"
    HAND = "hand"
    DISCARD = "discard"
    TABLEAU = "tableau"

class ConditionType(Enum):
    HAND_SIZE = "hand_size"
    CARD_MATCHES_RANK = "card_matches_rank"
    CARD_MATCHES_SUIT = "card_matches_suit"
    CARD_MATCHES_COLOR = "card_matches_color"
    CARD_IS_RANK = "card_is_rank"
    PLAYER_HAS_CARD = "player_has_card"
    LOCATION_EMPTY = "location_empty"
    LOCATION_SIZE = "location_size"
    SCORE_COMPARE = "score_compare"
    SEQUENCE_ADJACENT = "sequence_adjacent"  # For runs

class Operator(Enum):
    EQ = "=="
    NE = "!="
    LT = "<"
    GT = ">"
    LE = "<="
    GE = ">="

class ActionType(Enum):
    DRAW_CARDS = "draw_cards"
    PLAY_CARD = "play_card"
    DISCARD_CARD = "discard_card"
    SKIP_TURN = "skip_turn"
    REVERSE_ORDER = "reverse_order"
    CHOOSE_SUIT = "choose_suit"
    TRANSFER_CARDS = "transfer_cards"
    ADD_SCORE = "add_score"
    PASS = "pass"
```

### Condition System

```python
@dataclass
class Condition:
    """Composable predicate for game logic."""
    type: ConditionType
    operator: Optional[Operator] = None
    value: Optional[Union[int, Rank, Suit]] = None
    reference: Optional[str] = None  # "top_discard", "last_played", etc.

@dataclass
class CompoundCondition:
    """Combine conditions with AND/OR logic."""
    logic: Literal["AND", "OR"]
    conditions: List[Union[Condition, 'CompoundCondition']]
```

### Action System

```python
@dataclass
class Action:
    """Executable game action."""
    type: ActionType
    source: Optional[Location] = None
    target: Optional[Location] = None
    count: Optional[int] = None
    condition: Optional[Condition] = None
    card_filter: Optional[Condition] = None  # Which cards can be moved
```

### Game Structure

```python
@dataclass
class SetupRules:
    """Initial game configuration."""
    cards_per_player: int
    initial_deck: str = "standard_52"  # or "double", "custom"
    initial_discard_count: int = 0  # Cards to flip to discard pile
    initial_tableau: Optional[TableauConfig] = None
    starting_player: str = "random"  # or "youngest", "dealer_left"

@dataclass
class DrawPhase:
    """Draw cards from a location."""
    source: Location
    count: int
    condition: Optional[Condition] = None  # Draw if condition true
    mandatory: bool = True

@dataclass
class PlayPhase:
    """Play cards from hand."""
    target: Location
    valid_play_condition: Condition  # What makes a play legal
    min_cards: int = 1
    max_cards: int = 1
    mandatory: bool = True  # Must play if able
    pass_if_unable: bool = True

@dataclass
class DiscardPhase:
    """Discard cards from hand."""
    target: Location
    count: int
    mandatory: bool = False

@dataclass
class TurnStructure:
    """Ordered phases within a turn."""
    phases: List[Union[DrawPhase, PlayPhase, DiscardPhase]]

@dataclass
class SpecialEffect:
    """Card-triggered special action."""
    trigger_card: Rank
    trigger_condition: Optional[Condition] = None  # When effect activates
    actions: List[Action]

@dataclass
class WinCondition:
    """How to win the game."""
    type: Literal["empty_hand", "high_score", "first_to_score", "capture_all"]
    threshold: Optional[int] = None  # Score threshold if applicable

@dataclass
class ScoringRule:
    """How points are calculated."""
    condition: Condition  # When points are scored
    points: int  # How many points
    per_card: bool = False  # Multiply by card count

@dataclass
class GameGenome:
    """Complete game specification."""
    schema_version: str = "1.0"
    genome_id: str
    generation: int

    setup: SetupRules
    turn_structure: TurnStructure
    special_effects: List[SpecialEffect]
    win_conditions: List[WinCondition]
    scoring_rules: List[ScoringRule]

    max_turns: int = 100
    player_count: int = 2
```

---

## Example 1: Crazy 8s

```python
crazy_eights = GameGenome(
    schema_version="1.0",
    genome_id="crazy-eights-baseline",
    generation=0,

    setup=SetupRules(
        cards_per_player=7,
        initial_discard_count=1  # Flip one card to start discard
    ),

    turn_structure=TurnStructure(phases=[
        DrawPhase(
            source=Location.DECK,
            count=1,
            # Draw only if unable to play
            condition=Condition(
                type=ConditionType.PLAYER_HAS_CARD,
                operator=Operator.EQ,
                value=0,  # Has 0 playable cards
                reference="valid_plays"
            ),
            mandatory=True
        ),
        PlayPhase(
            target=Location.DISCARD,
            valid_play_condition=CompoundCondition(
                logic="OR",
                conditions=[
                    Condition(
                        type=ConditionType.CARD_MATCHES_RANK,
                        reference="top_discard"
                    ),
                    Condition(
                        type=ConditionType.CARD_MATCHES_SUIT,
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
    ]),

    special_effects=[
        SpecialEffect(
            trigger_card=Rank.EIGHT,
            actions=[
                Action(
                    type=ActionType.CHOOSE_SUIT,
                    # Player chooses suit for next play
                )
            ]
        )
    ],

    win_conditions=[
        WinCondition(
            type="empty_hand"
        )
    ],

    scoring_rules=[],  # No scoring in basic Crazy 8s

    max_turns=200,
    player_count=2
)
```

---

## Example 2: War

```python
war = GameGenome(
    schema_version="1.0",
    genome_id="war-baseline",
    generation=0,

    setup=SetupRules(
        cards_per_player=26,  # Split deck evenly
        initial_discard_count=0
    ),

    turn_structure=TurnStructure(phases=[
        PlayPhase(
            target=Location.TABLEAU,
            # Always play from top of hand (face down)
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
    ]),

    special_effects=[
        # No special effects - purely deterministic comparison
    ],

    win_conditions=[
        WinCondition(
            type="capture_all"  # Win by taking all cards
        )
    ],

    scoring_rules=[
        ScoringRule(
            # Highest card wins the battle
            condition=Condition(
                type=ConditionType.CARD_MATCHES_RANK,
                reference="highest_played",
                operator=Operator.EQ,
                value=None  # Determined at runtime
            ),
            points=0,  # Winner takes cards, not points
            per_card=False
        )
    ],

    max_turns=1000,  # Can be very long
    player_count=2
)
```

---

## Example 3: Gin Rummy (Simplified)

```python
gin_rummy = GameGenome(
    schema_version="1.0",
    genome_id="gin-rummy-baseline",
    generation=0,

    setup=SetupRules(
        cards_per_player=10,
        initial_discard_count=1
    ),

    turn_structure=TurnStructure(phases=[
        DrawPhase(
            source=Location.DECK,  # Can also draw from discard
            count=1,
            mandatory=True
        ),
        PlayPhase(
            target=Location.TABLEAU,
            # Can lay down melds (sets or runs)
            valid_play_condition=CompoundCondition(
                logic="OR",
                conditions=[
                    # Set: 3+ cards of same rank
                    Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.GE,
                        value=3,
                        reference="same_rank_group"
                    ),
                    # Run: 3+ cards of same suit in sequence
                    Condition(
                        type=ConditionType.SEQUENCE_ADJACENT,
                        operator=Operator.GE,
                        value=3,
                        reference="same_suit_sequence"
                    )
                ]
            ),
            min_cards=0,  # Playing melds is optional
            max_cards=10,
            mandatory=False
        ),
        DiscardPhase(
            target=Location.DISCARD,
            count=1,
            mandatory=True
        )
    ]),

    special_effects=[],

    win_conditions=[
        WinCondition(
            type="first_to_score",
            threshold=100  # First to 100 points wins
        )
    ],

    scoring_rules=[
        ScoringRule(
            # Deadwood points (unmelded cards)
            condition=Condition(
                type=ConditionType.HAND_SIZE,
                operator=Operator.GT,
                value=0,
                reference="unmelded_cards"
            ),
            points=-1,  # Negative points per unmelded card
            per_card=True
        ),
        ScoringRule(
            # Gin bonus (no deadwood)
            condition=Condition(
                type=ConditionType.HAND_SIZE,
                operator=Operator.EQ,
                value=0,
                reference="unmelded_cards"
            ),
            points=25,
            per_card=False
        )
    ],

    max_turns=50,
    player_count=2
)
```

---

## Schema Validation Findings

### ✅ Can Represent:
- **Crazy 8s:** Matching conditions, wild cards, suit selection
- **War:** Deterministic comparison, card capture
- **Gin Rummy:** Set/run formation, optional plays, scoring systems

### ⚠️ Edge Cases Identified:

1. **War's "Battle" Mechanic:**
   - When cards tie, need to play multiple cards face down, then compare
   - Current schema needs a `ConditionalAction` or `TriggerEffect` for ties
   - **Solution:** Add `trigger_condition` to actions (already in SpecialEffect)

2. **Gin Rummy's "Knocking":**
   - Player can end round early if deadwood below threshold
   - Need way to express "optional end-turn action"
   - **Solution:** Add `EndRoundAction` to action types

3. **Multi-Card Plays:**
   - Gin Rummy melds involve playing multiple cards as a group
   - Current `PlayPhase.max_cards` handles count but not "must be valid set/run"
   - **Solution:** `card_filter` in `PlayPhase` checks group validity

### 📋 Schema Enhancements Needed:

```python
# Add to ActionType enum:
class ActionType(Enum):
    # ... existing ...
    END_ROUND = "end_round"
    KNOCK = "knock"  # Gin Rummy specific
    DECLARE_WAR = "declare_war"  # War tie-breaker

# Add trigger system:
@dataclass
class TriggerEffect:
    """Action triggered by game state, not card."""
    trigger_condition: Condition
    actions: List[Action]
    priority: int = 0  # Resolve order if multiple triggers

# Enhance PlayPhase:
@dataclass
class PlayPhase:
    # ... existing fields ...
    group_validation: Optional[Condition] = None  # For melds, sets, runs
```

---

## Recommendations

1. **Core Schema is Sufficient** ✅
   - Can represent the three test games with minor additions
   - Structured approach works for shedding games, trick-taking variants

2. **Add to Schema:**
   - `TriggerEffect` for state-based actions (War ties)
   - `EndRoundAction` for early termination (Gin Rummy knock)
   - `group_validation` to `PlayPhase` for multi-card plays

3. **Test Coverage:**
   - Create Python implementations of these three games
   - Use as integration test fixtures
   - Benchmark Golang performance on these known games

4. **Next Steps:**
   - Implement enhanced schema in Python
   - Create JSON serialization examples
   - Build GenomeInterpreter for these test cases
   - Validate that genetic operators (mutation, crossover) work on real genomes

---

**Conclusion:** Path A (Enhanced Dataclasses) is validated. The schema can express known card games with minimal extensions.
