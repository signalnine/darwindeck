# Genome Schema Reference

**Date:** 2026-01-11
**Status:** Current implementation

This document describes the actual genome schema as implemented in the codebase.

## Core Schema Types

### Location: src/darwindeck/genome/schema.py

### Enumerations

```python
class Rank(Enum):
    """Playing card ranks."""
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
    """Playing card suits."""
    HEARTS = "H"
    DIAMONDS = "D"
    CLUBS = "C"
    SPADES = "S"

class Location(Enum):
    """Card locations in game."""
    DECK = "deck"
    HAND = "hand"
    DISCARD = "discard"
    TABLEAU = "tableau"
    OPPONENT_HAND = "opponent_hand"
    OPPONENT_DISCARD = "opponent_discard"

class TargetSelector(Enum):
    """Target selection for opponent-directed actions."""
    NEXT_PLAYER = "next_player"
    PREV_PLAYER = "prev_player"
    PLAYER_CHOICE = "player_choice"
    RANDOM_OPPONENT = "random_opponent"
    ALL_OPPONENTS = "all_opponents"
    LEFT_OPPONENT = "left_opponent"
    RIGHT_OPPONENT = "right_opponent"

class Visibility(Enum):
    """Card visibility state."""
    FACE_DOWN = "face_down"
    FACE_UP = "face_up"
    OWNER_ONLY = "owner_only"
    REVEALED = "revealed"
```

### Condition System

Location: `src/darwindeck/genome/conditions.py`

```python
class ConditionType(Enum):
    """Types of conditions that can be evaluated."""
    # Core conditions
    HAND_SIZE = "hand_size"
    CARD_MATCHES_RANK = "card_matches_rank"
    CARD_MATCHES_SUIT = "card_matches_suit"
    CARD_MATCHES_COLOR = "card_matches_color"
    CARD_IS_RANK = "card_is_rank"
    PLAYER_HAS_CARD = "player_has_card"
    LOCATION_EMPTY = "location_empty"
    LOCATION_SIZE = "location_size"
    SCORE_COMPARE = "score_compare"
    SEQUENCE_ADJACENT = "sequence_adjacent"

    # Wildcard matching
    MATCHES_OR_WILD = "matches_or_wild"

    # Pattern matching (set collection)
    HAS_SET_OF_N = "has_set_of_n"
    HAS_RUN_OF_N = "has_run_of_n"
    HAS_MATCHING_PAIR = "has_matching_pair"

    # Trick-taking conditions
    MUST_FOLLOW_SUIT = "must_follow_suit"
    HAS_TRUMP = "has_trump"
    SUIT_BROKEN = "suit_broken"
    IS_TRICK_WINNER = "is_trick_winner"
    TRICK_CONTAINS_CARD = "trick_contains_card"

    # Hand value (Blackjack-style)
    HAND_VALUE = "hand_value"

    # Layout/building (Fan Tan-style)
    CARD_ADJACENT_TO_LAYOUT = "card_adjacent_to_layout"

    # Climbing games (President-style)
    CARD_BEATS_TOP = "card_beats_top"

class Operator(Enum):
    """Comparison operators."""
    EQ = "=="
    NE = "!="
    LT = "<"
    GT = ">"
    LE = "<="
    GE = ">="

@dataclass(frozen=True)
class Condition:
    """Single condition predicate."""
    type: ConditionType
    operator: Optional[Operator] = None
    value: Optional[Union[int, str]] = None
    reference: Optional[str] = None  # "top_discard", "last_played", etc.

@dataclass(frozen=True)
class CompoundCondition:
    """Combine conditions with AND/OR logic."""
    logic: Literal["AND", "OR"]
    conditions: tuple[ConditionOrCompound, ...]
```

### Phase Types

```python
@dataclass(frozen=True)
class DrawPhase:
    """Draw cards from a location."""
    source: Location
    count: int = 1
    mandatory: bool = True
    condition: Optional[ConditionOrCompound] = None

@dataclass(frozen=True)
class PlayPhase:
    """Play cards from hand."""
    target: Location
    valid_play_condition: ConditionOrCompound
    min_cards: int = 1
    max_cards: int = 1
    mandatory: bool = True
    pass_if_unable: bool = True

@dataclass(frozen=True)
class DiscardPhase:
    """Discard cards to a location."""
    target: Location
    count: int = 1
    mandatory: bool = False
    matching_condition: Optional[ConditionOrCompound] = None

@dataclass(frozen=True)
class TrickPhase:
    """Trick-taking phase for games like Hearts, Spades."""
    lead_suit_required: bool = True
    trump_suit: Optional[Suit] = None
    high_card_wins: bool = True
    breaking_suit: Optional[Suit] = None  # Suit cannot be led until "broken"

@dataclass(frozen=True)
class ClaimPhase:
    """Bluffing/claiming phase for games like Cheat/BS."""
    min_cards: int = 1
    max_cards: int = 4
    sequential_rank: bool = True   # Must claim in order (A, 2, 3, ..., K)
    allow_challenge: bool = True
    pile_penalty: bool = True      # Loser takes discard pile
```

### Game Structure

```python
@dataclass(frozen=True)
class SetupRules:
    """Initial game configuration."""
    cards_per_player: int
    initial_deck: str = "standard_52"
    initial_discard_count: int = 0
    wild_cards: tuple[Rank, ...] = ()
    hand_visibility: Visibility = Visibility.OWNER_ONLY
    deck_visibility: Visibility = Visibility.FACE_DOWN
    discard_visibility: Visibility = Visibility.FACE_UP
    trump_suit: Optional[Suit] = None
    rotate_trump: bool = False
    random_trump: bool = False

@dataclass(frozen=True)
class TurnStructure:
    """Ordered phases within a turn."""
    phases: tuple[Phase, ...]
    is_trick_based: bool = False
    tricks_per_hand: Optional[int] = None

@dataclass(frozen=True)
class WinCondition:
    """How to win the game."""
    type: str  # "empty_hand", "high_score", "capture_all", "most_tricks", etc.
    threshold: Optional[int] = None

@dataclass(frozen=True)
class GameGenome:
    """Complete game specification."""
    schema_version: str
    genome_id: str
    generation: int
    setup: SetupRules
    turn_structure: TurnStructure
    special_effects: list       # Reserved for future use
    win_conditions: list[WinCondition]
    scoring_rules: list         # Reserved for future use
    max_turns: int = 100
    player_count: int = 2
    min_turns: int = 10
```

---

## Seed Game Examples

The following examples are the actual seed genomes from `src/darwindeck/genome/examples.py`.

### War (Pure Luck Baseline)

```python
GameGenome(
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
    win_conditions=[WinCondition(type="capture_all")],
    scoring_rules=[],
    max_turns=1000,
    player_count=2
)
```

### Crazy 8s (Matching with Wildcards)

```python
GameGenome(
    schema_version="1.0",
    genome_id="crazy-eights",
    generation=0,
    setup=SetupRules(
        cards_per_player=10,
        initial_deck="standard_52",
        initial_discard_count=1
    ),
    turn_structure=TurnStructure(
        phases=[
            DrawPhase(
                source=Location.DECK,
                count=1,
                mandatory=True,
                condition=Condition(
                    type=ConditionType.HAND_SIZE,
                    operator=Operator.EQ,
                    value=0,
                    reference="valid_plays"
                )
            ),
            PlayPhase(
                target=Location.DISCARD,
                valid_play_condition=CompoundCondition(
                    logic="OR",
                    conditions=[
                        Condition(type=ConditionType.CARD_MATCHES_SUIT, reference="top_discard"),
                        Condition(type=ConditionType.CARD_MATCHES_RANK, reference="top_discard"),
                        Condition(type=ConditionType.CARD_IS_RANK, value=Rank.EIGHT)
                    ]
                ),
                min_cards=1,
                max_cards=4,
                mandatory=True,
                pass_if_unable=True
            )
        ]
    ),
    special_effects=[],
    win_conditions=[WinCondition(type="empty_hand")],
    scoring_rules=[],
    max_turns=200,
    player_count=2
)
```

### Hearts (4-Player Trick-Taking)

```python
GameGenome(
    schema_version="1.0",
    genome_id="hearts-classic",
    generation=0,
    setup=SetupRules(
        cards_per_player=13,
        initial_deck="standard_52",
        initial_discard_count=0,
    ),
    turn_structure=TurnStructure(
        phases=[
            TrickPhase(
                lead_suit_required=True,
                trump_suit=None,
                high_card_wins=True,
                breaking_suit=Suit.HEARTS,
            )
        ],
        is_trick_based=True,
        tricks_per_hand=13,
    ),
    special_effects=[],
    win_conditions=[
        WinCondition(type="low_score", threshold=13),
        WinCondition(type="all_hands_empty", threshold=0)
    ],
    scoring_rules=[],
    max_turns=200,
    player_count=4,
    min_turns=52
)
```

### Cheat / I Doubt It (Bluffing)

```python
GameGenome(
    schema_version="1.0",
    genome_id="cheat",
    generation=0,
    setup=SetupRules(
        cards_per_player=26,
        initial_deck="standard_52",
        initial_discard_count=0
    ),
    turn_structure=TurnStructure(
        phases=[
            ClaimPhase(
                min_cards=1,
                max_cards=1,
                sequential_rank=True,
                allow_challenge=True,
                pile_penalty=True
            )
        ]
    ),
    special_effects=[],
    win_conditions=[WinCondition(type="empty_hand")],
    scoring_rules=[],
    max_turns=2000,
    player_count=2
)
```

### President (4-Player Climbing Game)

```python
GameGenome(
    schema_version="1.0",
    genome_id="president",
    generation=0,
    setup=SetupRules(
        cards_per_player=13,
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
                        Condition(
                            type=ConditionType.LOCATION_SIZE,
                            reference="tableau",
                            operator=Operator.EQ,
                            value=0
                        ),
                        Condition(
                            type=ConditionType.CARD_BEATS_TOP,
                            reference="tableau",
                            value="two_high"
                        )
                    ]
                ),
                min_cards=1,
                max_cards=1,
                mandatory=True,
                pass_if_unable=True
            )
        ]
    ),
    special_effects=[],
    win_conditions=[WinCondition(type="empty_hand")],
    scoring_rules=[],
    max_turns=300,
    player_count=4
)
```

---

## Complete Seed Game List

The following 16 games are used to seed the genetic algorithm:

| Game | Category | Players | Key Mechanics |
|------|----------|---------|---------------|
| **War** | Luck | 2 | Card comparison, capture |
| **Betting War** | Luck | 2 | Card comparison (variant) |
| **Hearts** | Trick-taking | 4 | Penalty cards, suit breaking |
| **Scotch Whist** | Trick-taking | 2 | Trump suit, trick collection |
| **Knock-Out Whist** | Trick-taking | 4 | Trump suit, elimination |
| **Spades** | Trick-taking | 4 | Fixed trump, suit breaking |
| **Crazy 8s** | Shedding | 2 | Matching, wildcards |
| **Old Maid** | Shedding | 2 | Pairing, avoidance |
| **President** | Climbing | 4 | Beat-or-pass, special rankings |
| **Fan Tan** | Building | 2 | Sequential play |
| **Gin Rummy** | Collection | 2 | Sets and runs |
| **Go Fish** | Collection | 2 | Book building |
| **Cheat** | Bluffing | 2 | Claims and challenges |
| **Scopa** | Capturing | 2 | Rank matching |
| **Draw Poker** | Improvement | 2 | Hand building |
| **Blackjack** | Targeting | 2 | Hand value |

---

## Schema Coverage

### Implemented Mechanics

| Mechanic | Phase Type | Example Games |
|----------|-----------|---------------|
| Draw cards | DrawPhase | All games |
| Play to location | PlayPhase | All games |
| Discard cards | DiscardPhase | Gin Rummy, Poker |
| Trick-taking | TrickPhase | Hearts, Spades, Whist variants |
| Bluffing/claims | ClaimPhase | Cheat/I Doubt It |

### Condition Types

| Category | Conditions | Use Cases |
|----------|-----------|-----------|
| Hand/Location | HAND_SIZE, LOCATION_SIZE, LOCATION_EMPTY | Basic flow control |
| Card matching | CARD_MATCHES_RANK, CARD_MATCHES_SUIT, CARD_IS_RANK | Crazy 8s, matching games |
| Pattern detection | HAS_SET_OF_N, HAS_RUN_OF_N, HAS_MATCHING_PAIR | Go Fish, Gin Rummy |
| Trick-taking | MUST_FOLLOW_SUIT, HAS_TRUMP, SUIT_BROKEN | Hearts, Spades |
| Climbing | CARD_BEATS_TOP | President |

### Not Yet Implemented

The following features are reserved for future development:

- **Betting mechanics**: ResourceRules, BettingPhase, chip tracking
- **Team play**: Partnership scoring, shared captures
- **Bidding**: Contract declarations
- **Complex scoring**: ScoringRule dataclass, end-game evaluation
- **Special effects**: Card-triggered actions beyond basic play

---

## Schema Constraints

### Immutability

All schema types are frozen dataclasses. Lists are converted to tuples automatically:

```python
@dataclass(frozen=True)
class TurnStructure:
    phases: tuple[Phase, ...]  # Immutable tuple

    def __init__(self, phases: list, ...) -> None:
        object.__setattr__(self, "phases", tuple(phases))
```

### Serialization

Genomes can be serialized to/from JSON using `src/darwindeck/genome/serialization.py`:

```python
from darwindeck.genome.serialization import genome_to_json, genome_from_json

# Serialize
json_str = genome_to_json(genome)

# Deserialize
genome = genome_from_json(json_str)
```

### Validation

The bytecode compiler validates genomes during compilation. Invalid genomes (missing phases, invalid conditions) will raise errors.
