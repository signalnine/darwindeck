# Can We Represent Poker?

> **Legacy v1 (2026-09-28):** historical doc for the v1 Python/CGo system (`src/`), not the current Go v2 system (`cmd/` + `pkg/`); see `CLAUDE.md` / `README.md` for current.
> v2 now ships a dedicated vying (poker) skeleton in `pkg/skeleton/vying/`.

**Date:** 2026-01-10
**Question:** Does our enhanced dataclass schema support poker-like games?

## Poker Complexity Analysis

### Core Poker Mechanics

1. **Betting System**
   - Players have chip stacks (resources)
   - Actions: bet, call, raise, fold, check, all-in
   - Pot accumulation and side pots
   - Betting rounds with multiple players

2. **Hidden Information**
   - Each player's hole cards are private
   - Community cards are shared and visible
   - Opponents' cards unknown until showdown

3. **Hand Rankings**
   - Complex hierarchy: High card < Pair < Two Pair < Three of a Kind < Straight < Flush < Full House < Four of a Kind < Straight Flush
   - Requires evaluating 5-card combinations from 7 cards (Texas Hold'em)

4. **Multi-Round Structure**
   - Pre-flop (hole cards dealt, betting)
   - Flop (3 community cards, betting)
   - Turn (1 community card, betting)
   - River (1 community card, betting)
   - Showdown (hand comparison)

5. **Dynamic Player Count**
   - Players can fold and leave the hand
   - Game continues with remaining players

---

## Schema Gap Analysis

### ✅ What We Can Already Represent

**Hidden Information:**
```python
@dataclass
class CardVisibility:
    """NEW: Who can see which cards."""
    visible_to: List[int]  # Player IDs, or [-1] for all
    location: Location
```

**Community Cards:**
```python
setup=SetupRules(
    cards_per_player=2,  # Hole cards
    initial_tableau=TableauConfig(
        size=5,  # 3 flop + turn + river
        visibility="public"
    )
)
```

**Multi-Round Structure:**
```python
turn_structure=TurnStructure(phases=[
    # Pre-flop
    BettingPhase(round_name="pre_flop"),

    # Flop
    DealPhase(target=Location.TABLEAU, count=3, visible_to_all=True),
    BettingPhase(round_name="flop"),

    # Turn
    DealPhase(target=Location.TABLEAU, count=1, visible_to_all=True),
    BettingPhase(round_name="turn"),

    # River
    DealPhase(target=Location.TABLEAU, count=1, visible_to_all=True),
    BettingPhase(round_name="river"),
])
```

---

### ❌ What's Missing from Current Schema

#### 1. **Betting Phase** (Critical Gap)

Current schema only has `DrawPhase`, `PlayPhase`, `DiscardPhase`. Need:

```python
@dataclass
class BettingPhase:
    """Betting round with chips."""
    min_bet: int
    max_bet: Optional[int] = None  # No limit if None
    allow_check: bool = True  # Can pass if no bet to call
    allow_raise: bool = True
    allow_fold: bool = True

@dataclass
class BettingAction(Action):
    """Player betting action."""
    type: Literal["bet", "call", "raise", "fold", "check", "all_in"]
    amount: Optional[int] = None
```

#### 2. **Resource Management** (Critical Gap)

Need to track chips/money, not just scores:

```python
@dataclass
class ResourceRules:
    """Chip/money tracking."""
    starting_chips: int
    min_buy_in: int
    max_buy_in: Optional[int] = None

@dataclass
class GameState:
    # Existing fields...
    player_chips: Dict[int, int]  # Player ID -> chip count
    pot: int  # Main pot
    side_pots: List[SidePot] = field(default_factory=list)
```

#### 3. **Hand Evaluation** (Critical Gap)

Complex logic to compare poker hands:

```python
@dataclass
class HandRankingRule:
    """Define poker hand hierarchy."""
    rank_name: str  # "flush", "straight", "pair", etc.
    priority: int  # Higher = better hand
    detection_logic: Condition  # How to detect this hand

# Example:
HandRankingRule(
    rank_name="flush",
    priority=5,
    detection_logic=Condition(
        type=ConditionType.CARDS_SAME_SUIT,
        value=5
    )
)
```

But this gets complex fast. Straight detection requires:
- Check if ranks form consecutive sequence
- Handle Ace-high vs Ace-low straights
- Combine with suit matching for straight flush

**Problem:** Our Condition system isn't expressive enough for arbitrary card combinations.

#### 4. **Showdown Logic** (Moderate Gap)

Need to compare hands across multiple players:

```python
@dataclass
class ShowdownPhase:
    """Compare hands and award pot."""
    hand_evaluation: List[HandRankingRule]
    tiebreaker: TiebreakerRule  # How to split pot
```

#### 5. **Dynamic Player Elimination**

Players who fold are out of the current hand but can play next hand:

```python
@dataclass
class GameState:
    # ...
    active_players: Set[int]  # Players still in this hand
    folded_players: Set[int]  # Folded this hand, rejoin next hand
```

---

## Expressiveness Ceiling Reached?

### The Hand Evaluation Problem

Poker hand rankings require logic like:

```
Is Flush?
  → Count cards by suit
  → If any suit has 5+ cards: True

Is Straight?
  → Sort cards by rank
  → Check for consecutive sequence of 5
  → Handle wrap-around (A-2-3-4-5 and 10-J-Q-K-A)

Is Full House?
  → Group by rank
  → If one rank has 3 cards AND another rank has 2 cards: True
```

**Current Condition system:**
```python
Condition(
    type=ConditionType.CARDS_SAME_SUIT,
    operator=Operator.GE,
    value=5
)
```

This works for simple checks, but:
- Can't express "group by rank, find if any group has 3"
- Can't express "sort by rank, check for sequence"
- Can't express "compare best 5-card combo from 7 cards"

### Two Options:

#### Option A: Extend Conditions with Aggregators

```python
@dataclass
class AggregateCondition(Condition):
    """Apply function to cards, then check condition."""
    aggregate_func: AggregateFunc  # GROUP_BY_RANK, GROUP_BY_SUIT, SORT, etc.
    inner_condition: Condition  # Check result of aggregation

# Example: Full House
AggregateCondition(
    aggregate_func=AggregateFunc.GROUP_BY_RANK,
    inner_condition=CompoundCondition(
        logic="AND",
        conditions=[
            Condition(type=ConditionType.GROUP_SIZE, operator=Operator.EQ, value=3),
            Condition(type=ConditionType.GROUP_SIZE, operator=Operator.EQ, value=2)
        ]
    )
)
```

**Pros:** Stays within structured data
**Cons:** Getting very close to a programming language

#### Option B: Predefined Hand Evaluator

```python
@dataclass
class PokerHandEvaluator:
    """Built-in poker hand logic."""
    variant: Literal["texas_holdem", "five_card_draw", "omaha"]
    # Uses hardcoded poker rules, not evolved
```

**Pros:** Simple, correct poker logic
**Cons:** Can't evolve novel hand rankings (defeats purpose of evolution)

---

## Can We Represent Poker? Answer:

### **Sort of, but not fully evolvable.**

### What We CAN Do:

✅ **Fixed-rules poker** with built-in hand evaluator
- Define betting phases
- Track chips and pots
- Handle hidden information
- Use `PokerHandEvaluator` for showdown

✅ **Simplified poker variants**
- Single betting round
- Simpler hand rankings (e.g., only pairs, high card)
- No bluffing/psychology (AI players can't bluff anyway in current design)

### What We CAN'T Do (without major schema changes):

❌ **Evolve novel hand rankings**
- Structured Conditions aren't expressive enough for arbitrary card combinations
- Would need Option A (aggregate conditions) or Option B (hardcoded evaluator)

❌ **Full Texas Hold'em with evolvable rules**
- Betting, chips, hand evaluation all together = very complex genome
- Evolution would likely produce degenerate games (everyone folds, or everyone all-ins)

---

## Recommendations

### 1. **For This Project: Don't Prioritize Poker**

Poker is an **outlier** in card game space:
- Requires betting/chips (resource management)
- Requires complex hand evaluation
- Requires psychological elements (bluffing)
- Already has optimal variants (Texas Hold'em is well-balanced)

Evolution is better suited for:
- **Shedding games** (Uno, Crazy 8s variants)
- **Matching games** (simple rules, many possible variations)
- **Trick-taking** (Hearts, Spades variants)
- **Set collection** (Rummy variants)

### 2. **If You Want Poker-Like Games:**

**Option A: Add to Schema (Major Extension)**

Add these types to support betting games:
```python
- BettingPhase
- ResourceRules (chips)
- HandRankingRule with AggregateCondition
- ShowdownPhase
```

**Complexity Cost:** ~30% more schema complexity
**Evolutionary Cost:** Much larger search space, likely more broken games

**Option B: Hardcoded Poker as Test Case**

Use poker with built-in hand evaluator as:
- Validation that schema can handle complex games
- Performance benchmark for Golang core
- Not evolved, just simulated

### 3. **Schema Expressiveness Threshold**

We're approaching the limit of "structured data vs code":
- Simple games: ✅ Structured data is perfect
- Medium complexity (Gin Rummy): ✅ Works with extensions
- High complexity (Poker): ⚠️ Needs aggregators or hardcoded logic
- Arbitrary computation: ❌ Would need actual DSL or AST-as-JSON (Path B)

**Recommendation:** Stick with Path A for now. If poker becomes a requirement, revisit Path B (AST-as-JSON with opcodes).

---

## Conclusion

**Can we represent poker?** Technically yes, with significant schema extensions (BettingPhase, ResourceRules, HandRankingRule).

**Should we?** Probably not for initial implementation:
1. Adds 30%+ complexity to schema
2. Evolution will struggle with betting mechanics
3. Better to focus on shedding/matching games where evolution shines
4. Can add poker support later if needed

**Alternative:** Use simplified poker-like games:
- Single betting round
- Simple hand rankings (pairs, high card only)
- Fixed chip counts
- Tests betting mechanics without full complexity

**Decision Point:** Do you want to support betting/resource games, or focus on simpler card games for the first version?
