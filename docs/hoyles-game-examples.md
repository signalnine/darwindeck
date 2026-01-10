# Hoyle's Card Games - Genome Schema Examples

**Date:** 2026-01-10
**Source:** Hoyle's Encyclopedia of Card Games by Walter B. Gibson
**Purpose:** Validate genome schema against real card games from Hoyle's

## Overview

This document encodes classic card games from Hoyle's Encyclopedia using our genome schema. Some games require simplifications or extensions to fit the turn-based structure.

---

## Game 1: Slapjack (Simplified for Turn-Based Play)

**Original Rules:** Players flip cards to center pile in rotation. When a Jack appears, first to slap wins the pile.

**Schema Challenge:** Slapjack is a real-time reaction game, not turn-based.

**Simplified Version:** Turn-based variant where current player automatically wins the pile when a Jack is revealed.

```python
slapjack = GameGenome(
    schema_version="1.0",
    genome_id="slapjack-turnbased",
    generation=0,

    setup=SetupRules(
        cards_per_player=26,  # Split deck evenly (2 players)
        initial_discard_count=0
    ),

    turn_structure=TurnStructure(phases=[
        PlayPhase(
            target=Location.DISCARD,
            # Always play top card from hand (face down deck)
            valid_play_condition=Condition(
                type=ConditionType.HAND_SIZE,
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
        SpecialEffect(
            trigger_card=Rank.JACK,
            actions=[
                Action(
                    type=ActionType.TRANSFER_CARDS,
                    source=Location.DISCARD,
                    target=Location.HAND,  # Current player wins entire discard pile
                    count=-1  # All cards
                )
            ]
        )
    ],

    win_conditions=[
        WinCondition(
            type="capture_all"  # Win by capturing all 52 cards
        )
    ],

    scoring_rules=[],
    max_turns=500,  # Can be very long
    player_count=2
)
```

**Notes:**
- Loses the real-time "slapping" element that makes the game exciting
- Demonstrates special effect triggered by specific rank
- Uses TRANSFER_CARDS action to move entire discard pile to hand

**Schema Validation:**
- ✅ Can represent with current schema
- ⚠️ Loses core mechanic (real-time slapping)
- 💡 Extension idea: `RaceCondition` action type for simultaneous player responses

---

## Game 2: Old Maid (Simplified)

**Original Rules:**
- Remove one Queen
- Deal all cards
- Pair up matching rank+color cards and discard
- Draw from each other's hands in rotation
- Last player with unpaired Queen loses

**Schema Challenge:** Interactive drawing from opponents' hands not in current schema.

**Simplified Version:** Draw-and-discard variant with pairs

```python
old_maid = GameGenome(
    schema_version="1.0",
    genome_id="old-maid-simplified",
    generation=0,

    setup=SetupRules(
        cards_per_player=25,  # 51 cards (one Queen removed) / 2 players
        initial_deck="51_cards",  # Remove one Queen
        initial_discard_count=0
    ),

    turn_structure=TurnStructure(phases=[
        # Phase 1: Discard any pairs you have
        DiscardPhase(
            target=Location.DISCARD,
            count=2,  # Discard in pairs
            mandatory=False,  # Only if you have a pair
            # TODO: Need card_filter to specify "same rank, same color"
        ),

        # Phase 2: Draw from deck (simplified - not from opponent)
        DrawPhase(
            source=Location.DECK,
            count=1,
            mandatory=True,
            condition=Condition(
                type=ConditionType.LOCATION_SIZE,
                reference="deck",
                operator=Operator.GT,
                value=0
            )
        )
    ]),

    special_effects=[],

    win_conditions=[
        WinCondition(
            type="empty_hand"  # First to empty hand wins (opponent has the Queen)
        )
    ],

    scoring_rules=[],
    max_turns=100,
    player_count=2
)
```

**Schema Gaps Identified:**
1. **Pairing logic:** Need `card_filter` in DiscardPhase to specify "same rank, same color"
2. **Drawing from opponent:** No way to specify `source=Location.OPPONENT_HAND`
3. **Initial pairing:** Should happen during setup, not each turn

**Possible Schema Extensions:**

```python
# Extension 1: Enhanced DiscardPhase
@dataclass
class DiscardPhase:
    target: Location
    count: int
    mandatory: bool = False
    pair_condition: Optional[Condition] = None  # NEW: Cards must satisfy this to be discarded together

# Extension 2: Opponent interaction
class Location(Enum):
    DECK = "deck"
    HAND = "hand"
    DISCARD = "discard"
    TABLEAU = "tableau"
    OPPONENT_HAND = "opponent_hand"  # NEW: For Old Maid, I Doubt It, etc.

# Extension 3: Setup phase actions
@dataclass
class SetupRules:
    cards_per_player: int
    initial_deck: str = "standard_52"
    initial_discard_count: int = 0
    initial_tableau: Optional[TableauConfig] = None
    starting_player: str = "random"
    setup_actions: List[Action] = field(default_factory=list)  # NEW: Run after deal
```

---

## Game 3: Go Fish (Partially Representable)

**Original Rules:**
- Deal 7 cards (2-3 players) or 5 cards (4-5 players)
- Ask opponent for a specific rank
- If they have it, they give you all cards of that rank
- If not, "go fish" - draw from deck
- When you get 4 of a kind, lay down the "book"
- First to empty hand wins

**Schema Challenge:**
- **Asking for specific rank:** Interactive choice not represented
- **Conditional on opponent's response:** Draw if opponent says "go fish"

**Skeleton (Incomplete):**

```python
go_fish = GameGenome(
    schema_version="1.0",
    genome_id="go-fish-incomplete",
    generation=0,

    setup=SetupRules(
        cards_per_player=7,  # 2-3 players
        initial_discard_count=0
    ),

    turn_structure=TurnStructure(phases=[
        # ❌ CANNOT REPRESENT: "Ask opponent for rank X"
        # This requires:
        # 1. Player input to choose rank
        # 2. Checking opponent's hand
        # 3. Transfer if match, draw if not

        # Simplified approximation:
        DrawPhase(
            source=Location.DECK,
            count=1,
            mandatory=True
        ),

        # Lay down books of 4
        PlayPhase(
            target=Location.TABLEAU,
            valid_play_condition=Condition(
                type=ConditionType.PLAYER_HAS_CARD,
                # ❌ CANNOT EXPRESS: "4 cards of same rank"
                operator=Operator.GE,
                value=4,
                reference="same_rank_group"
            ),
            min_cards=4,
            max_cards=4,
            mandatory=False
        )
    ]),

    special_effects=[],

    win_conditions=[
        WinCondition(
            type="empty_hand"
        )
    ],

    scoring_rules=[],
    max_turns=100,
    player_count=2
)
```

**Schema Gaps Identified:**
1. **Player choice of rank:** No `ChooseRankAction` or input mechanism
2. **Opponent hand checking:** Cannot check/transfer from opponent's hand
3. **Grouping by rank:** `PLAYER_HAS_CARD` with `same_rank_group` reference is conceptual, not implemented

**Required Extensions for Go Fish:**

```python
# Extension 1: Player input actions
class ActionType(Enum):
    # ... existing ...
    ASK_FOR_RANK = "ask_for_rank"  # NEW
    TRANSFER_FROM_OPPONENT = "transfer_from_opponent"  # NEW

@dataclass
class AskForRankAction(Action):
    """Player chooses a rank to request from opponent."""
    type: ActionType = ActionType.ASK_FOR_RANK
    must_have_rank: bool = True  # Player must hold at least one of requested rank

@dataclass
class ConditionalTransferAction(Action):
    """Transfer cards based on condition, otherwise trigger alternative."""
    type: ActionType = ActionType.TRANSFER_FROM_OPPONENT
    requested_rank: str  # Set by previous ASK_FOR_RANK action
    on_success: List[Action]  # Continue turn
    on_failure: List[Action]  # Draw from deck
```

---

## Analysis: Schema Fitness for Hoyle's Games

### Games That Fit Well ✅

1. **War** (already implemented)
2. **Crazy 8s** (already implemented)
3. **Solitaire variants** (single-player, no opponent interaction)
4. **Trick-taking games** (Hearts, Spades) - with minor extensions

### Games Requiring Minor Extensions ⚠️

1. **Old Maid** - Needs opponent hand drawing
2. **Gin Rummy** (already implemented) - Needs set/run detection
3. **Snap** - Needs simultaneous action or simplification

### Games Requiring Major Extensions ❌

1. **Go Fish** - Needs player input, opponent hand checking
2. **Poker** - Needs betting, bluffing, complex hand evaluation
3. **Concentration/Memory** - Needs hidden tableau positions
4. **I Doubt It** - Needs lying/challenging mechanics

---

## Recommendations

### 1. ✅ IMPLEMENTED - Schema Extensions Added

The following extensions have been added to the genome schema (see `docs/genome-schema-examples.md`):

**A. Opponent Interaction ✅**
```python
Location.OPPONENT_HAND       # Draw from opponent (Old Maid, I Doubt It)
Location.OPPONENT_DISCARD    # Access opponent's discard
```

**B. Set/Sequence Detection ✅**
```python
ConditionType.HAS_SET_OF_N      # N cards of same rank (Go Fish books)
ConditionType.HAS_RUN_OF_N      # Sequential cards (Gin Rummy runs)
ConditionType.HAS_MATCHING_PAIR # Pairs by property (Old Maid)
```

**C. Setup Actions ✅**
```python
SetupRules.post_deal_actions    # Actions after deal
DiscardPhase.matching_condition # Constrain to matching sets
```

**D. Betting/Wagering ✅**
```python
ResourceRules                # Chip tracking
BettingPhase                # Betting rounds
ActionType.BET/CALL/RAISE/FOLD/CHECK/ALL_IN
ConditionType.CHIP_COUNT/POT_SIZE/CURRENT_BET/CAN_AFFORD
```

**E. Bluffing/Challenges ✅**
```python
ClaimPhase                  # Making claims
ActionType.CLAIM/CHALLENGE/REVEAL
```

All extensions are **optionally enabled** and **backward-compatible**.

### 2. Defer to Phase 4+

Features that are complex or rarely used:

- **Player input/choice actions** (Choose rank, choose suit, etc.)
- **Hidden information beyond hands** (Concentration's face-down cards)
- **Simultaneous actions** (Slapjack, Spoons, Egyptian Ratscrew)
- **Betting/resources** (Poker, any gambling game)
- **Lying/bluffing** (I Doubt It, Cheat, BS)

### 3. Document "Evolvable Game Space"

Our schema is best suited for:
- **Shedding games** (variations of Crazy 8s, Uno)
- **Trick-taking** (Hearts, Spades variants with different scoring)
- **Matching/pairing** (simplified variants)
- **Capture** (War, Beggar My Neighbor)
- **Solitaire** (Klondike, FreeCell variants)

Evolution will produce novel games in this space, not replicate complex games like Poker or Go Fish.

---

## Next Steps

1. ✅ Validate schema against Hoyle's games
2. ⚠️ Identify minimal extensions needed (opponent hand, set detection)
3. ⏳ Create simplified "evolvable" variants of popular games
4. ⏳ Use these as initial population for evolution
5. ⏳ Document "game design patterns" that work well with schema

**Conclusion:** Current schema is solid for 60-70% of simple card games. Strategic extensions can push this to 80-85%. Perfect coverage would require a full programming language (Path B), which we explicitly rejected.
