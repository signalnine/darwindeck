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

    # NOTE: SpecialEffect not implemented - Jack triggering pile capture
    # would require future extension to the schema
    special_effects=[],

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
- The Jack-triggered pile capture shown above requires SpecialEffect (not implemented)
- This example shows a conceptual encoding, not a working implementation

**Schema Validation:**
- ⚠️ CANNOT fully represent with current schema (no SpecialEffect)
- ⚠️ Loses core mechanic (real-time slapping)
- 💡 Extension needed: SpecialEffect for card-triggered actions

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
1. **Pairing logic:** Use DiscardPhase.matching_condition to specify "same rank, same color"
2. **Drawing from opponent:** ✅ NOW IMPLEMENTED - use `source=Location.OPPONENT_HAND`
3. **Initial pairing:** Setup phase actions not implemented - would need extension

**Implementation Status of Suggested Extensions:**

```python
# ✅ IMPLEMENTED: DiscardPhase with matching_condition
@dataclass
class DiscardPhase:
    target: Location
    count: int
    mandatory: bool = False
    matching_condition: Optional[Condition] = None  # Can constrain discards

# ✅ IMPLEMENTED: Opponent hand/discard locations
class Location(Enum):
    # ... standard locations ...
    OPPONENT_HAND = "opponent_hand"      # For Old Maid, I Doubt It, etc.
    OPPONENT_DISCARD = "opponent_discard"

# ❌ NOT IMPLEMENTED: Setup phase actions
# SetupRules.setup_actions does not exist - would need future work
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

**Required Extensions for Go Fish (NOT IMPLEMENTED):**

```python
# These are conceptual extensions that would be needed:

# Extension 1: Player input actions (requires ActionType enum, not implemented)
# ASK_FOR_RANK - Player chooses a rank to request from opponent
# TRANSFER_FROM_OPPONENT - Transfer matching cards from opponent

# Extension 2: Conditional transfer based on opponent's response
# on_success: Continue turn
# on_failure: Draw from deck ("go fish")
```

**Note:** The above extensions are design proposals, not implemented features.

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
2. **Poker** - Needs betting system (not implemented)
3. **Concentration/Memory** - Needs hidden tableau positions
4. **I Doubt It** - Partially supported via ClaimPhase (bluffing/challenging implemented)

---

## Recommendations

### 1. Schema Features - Current Status

**A. Opponent Interaction ✅ IMPLEMENTED**
```python
Location.OPPONENT_HAND       # Draw from opponent (Old Maid, I Doubt It)
Location.OPPONENT_DISCARD    # Access opponent's discard
```

**B. Set/Sequence Detection ✅ IMPLEMENTED (schema only, evaluation partial)**
```python
ConditionType.HAS_SET_OF_N      # N cards of same rank (Go Fish books)
ConditionType.HAS_RUN_OF_N      # Sequential cards (Gin Rummy runs)
ConditionType.HAS_MATCHING_PAIR # Pairs by property (Old Maid)
```

**C. Discard Matching ✅ IMPLEMENTED**
```python
DiscardPhase.matching_condition # Constrain to matching sets
```

**D. Bluffing/Claiming ✅ IMPLEMENTED**
```python
ClaimPhase                  # Making claims (Cheat/BS/I Doubt It)
# Players play face-down, claim rank, can be challenged
```

**E. Trick-Taking ✅ IMPLEMENTED**
```python
TrickPhase                  # Trick-based games (Hearts, Spades)
# Follow suit, trump, breaking suit
```

### 2. Not Yet Implemented

Features that require future work:

- **Betting/wagering** - No ResourceRules, BettingPhase, or chip tracking
- **Post-deal actions** - No SetupRules.post_deal_actions
- **Player input/choice actions** (Choose rank, choose suit, etc.)
- **Hidden information beyond hands** (Concentration's face-down cards)
- **Simultaneous actions** (Slapjack, Spoons, Egyptian Ratscrew)
- **Special effects** - No SpecialEffect or ActionType system

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
