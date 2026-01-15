# Self-Describing Genomes Design (v2)

> **Goal:** Make genomes fully self-describing so they work with any simulator, produce accurate rulebooks, and evolution only creates fully-specified games.

**Problem:** 10/18 seed genomes rely on ~38 implicit mechanics hardcoded in the Go simulator (Hearts scoring, Poker rankings, Blackjack valuation, etc.). A genome alone isn't enough to play the game.

**Solution:** Expand the genome schema to make all mechanics explicit. **No auto-inference** - all seeds migrated to explicit format.

---

## Design Principles (from Multi-Agent Review)

1. **No implicit mechanics** - Everything needed to play is in the genome
2. **No auto-inference** - Defaults are just defaults, not magic behavior detection
3. **Compositional over enumerated** - Define patterns via constraints, not hardcoded lists
4. **Validation at schema level** - Invalid combinations caught at parse time

---

## 1. Scoring System

**Problem:** Simulator hardcodes Hearts-style scoring (1pt/heart, 13pt/QS).

**Solution:** Add explicit `card_scoring` rules:

```python
@dataclass(frozen=True)
class CardScoringRule:
    """Score points when a card meets a condition."""
    condition: CardCondition  # e.g., suit=HEARTS
    points: int               # Points awarded (can be negative)
    trigger: ScoringTrigger   # When scoring happens

class ScoringTrigger(Enum):
    TRICK_WIN = "trick_win"       # Score when winning trick with this card
    CAPTURE = "capture"           # Score when capturing this card
    PLAY = "play"                 # Score when playing this card
    HAND_END = "hand_end"         # Score for cards in hand at end
    SET_COMPLETE = "set_complete" # Score when completing a set (Go Fish)
```

**Example - Hearts:**
```python
card_scoring=(
    CardScoringRule(
        condition=CardCondition(suit=Suit.HEARTS),
        points=1,
        trigger=ScoringTrigger.TRICK_WIN
    ),
    CardScoringRule(
        condition=CardCondition(suit=Suit.SPADES, rank=Rank.QUEEN),
        points=13,
        trigger=ScoringTrigger.TRICK_WIN
    ),
)
```

---

## 2. Hand Evaluation System (REVISED)

**Problem:** Poker and Blackjack rely on hardcoded hand evaluation.

**Original flaw:** `HandType` enum with ROYAL_FLUSH, STRAIGHT_FLUSH, etc. re-introduces implicit mechanics - the simulator must know what "royal flush" means.

**Solution:** Replace enum with compositional `HandPattern` that defines patterns via constraints:

```python
@dataclass(frozen=True)
class HandPattern:
    """A pattern to match in a hand. Fully describes what to look for."""
    name: str                              # "Full House", "Flush", etc.
    rank_priority: int                     # Higher = better hand (100 > 50)

    # Constraints (all must be satisfied)
    required_count: Optional[int] = None   # Exactly N cards (e.g., 5 for poker)
    same_suit_count: Optional[int] = None  # N cards must share suit (5 = flush)
    same_rank_groups: Optional[tuple[int, ...]] = None  # (3, 2) = three + pair
    sequence_length: Optional[int] = None  # N consecutive ranks (5 = straight)
    sequence_wrap: bool = False            # A-2-3 and Q-K-A both valid
    required_ranks: Optional[tuple[Rank, ...]] = None  # Must contain these ranks

@dataclass(frozen=True)
class HandEvaluation:
    """How to evaluate and compare hands."""
    method: HandEvaluationMethod
    patterns: tuple[HandPattern, ...] = ()  # For PATTERN_MATCH
    card_values: tuple[CardValue, ...] = ()  # For POINT_TOTAL
    target_value: Optional[int] = None       # Blackjack: 21
    bust_threshold: Optional[int] = None     # Blackjack: 22

class HandEvaluationMethod(Enum):
    NONE = "none"
    HIGH_CARD = "high_card"          # Compare highest cards
    POINT_TOTAL = "point_total"      # Sum card values (Blackjack)
    PATTERN_MATCH = "pattern_match"  # Match patterns in priority order
    CARD_COUNT = "card_count"        # Most cards wins (War)

@dataclass(frozen=True)
class CardValue:
    """Point value for a card rank."""
    rank: Rank
    value: int
    alternate_value: Optional[int] = None  # Ace: 1 or 11
```

**Example - Standard Poker (fully explicit):**
```python
hand_evaluation=HandEvaluation(
    method=HandEvaluationMethod.PATTERN_MATCH,
    patterns=(
        # Royal Flush: 5 same suit, sequence, must be 10-J-Q-K-A
        HandPattern(
            name="Royal Flush",
            rank_priority=100,
            required_count=5,
            same_suit_count=5,
            sequence_length=5,
            required_ranks=(Rank.TEN, Rank.JACK, Rank.QUEEN, Rank.KING, Rank.ACE),
        ),
        # Straight Flush: 5 same suit, sequence
        HandPattern(
            name="Straight Flush",
            rank_priority=90,
            required_count=5,
            same_suit_count=5,
            sequence_length=5,
        ),
        # Four of a Kind: 4 same rank
        HandPattern(
            name="Four of a Kind",
            rank_priority=80,
            required_count=5,
            same_rank_groups=(4,),
        ),
        # Full House: 3 same + 2 same
        HandPattern(
            name="Full House",
            rank_priority=70,
            required_count=5,
            same_rank_groups=(3, 2),
        ),
        # Flush: 5 same suit
        HandPattern(
            name="Flush",
            rank_priority=60,
            required_count=5,
            same_suit_count=5,
        ),
        # Straight: 5 consecutive ranks
        HandPattern(
            name="Straight",
            rank_priority=50,
            required_count=5,
            sequence_length=5,
            sequence_wrap=True,  # A-2-3-4-5 is valid
        ),
        # Three of a Kind
        HandPattern(
            name="Three of a Kind",
            rank_priority=40,
            required_count=5,
            same_rank_groups=(3,),
        ),
        # Two Pair
        HandPattern(
            name="Two Pair",
            rank_priority=30,
            required_count=5,
            same_rank_groups=(2, 2),
        ),
        # One Pair
        HandPattern(
            name="One Pair",
            rank_priority=20,
            required_count=5,
            same_rank_groups=(2,),
        ),
        # High Card (fallback - no constraints except count)
        HandPattern(
            name="High Card",
            rank_priority=10,
            required_count=5,
        ),
    ),
)
```

**Why this is better:**
- Simulator just matches constraints, no knowledge of "what is a flush"
- Evolution can create novel hand patterns (e.g., "Rainbow" - 5 different suits)
- Rulebook generator can describe patterns from constraints
- Any simulator can implement the matching logic

**Example - Blackjack (point-based):**
```python
hand_evaluation=HandEvaluation(
    method=HandEvaluationMethod.POINT_TOTAL,
    card_values=(
        CardValue(rank=Rank.ACE, value=11, alternate_value=1),
        CardValue(rank=Rank.KING, value=10),
        CardValue(rank=Rank.QUEEN, value=10),
        CardValue(rank=Rank.JACK, value=10),
        CardValue(rank=Rank.TEN, value=10),
        CardValue(rank=Rank.NINE, value=9),
        CardValue(rank=Rank.EIGHT, value=8),
        CardValue(rank=Rank.SEVEN, value=7),
        CardValue(rank=Rank.SIX, value=6),
        CardValue(rank=Rank.FIVE, value=5),
        CardValue(rank=Rank.FOUR, value=4),
        CardValue(rank=Rank.THREE, value=3),
        CardValue(rank=Rank.TWO, value=2),
    ),
    target_value=21,
    bust_threshold=22,
)
```

---

## 3. Win Condition Modifiers

**Problem:** Win conditions have hidden trigger logic.

**Solution:** Expand `WinCondition` with explicit modifiers:

```python
@dataclass(frozen=True)
class WinCondition:
    type: str
    threshold: int = 0

    # Explicit modifiers (no auto-inference)
    comparison: WinComparison = WinComparison.HIGHEST
    trigger_mode: TriggerMode = TriggerMode.IMMEDIATE
    required_hand_size: Optional[int] = None

class WinComparison(Enum):
    HIGHEST = "highest"
    LOWEST = "lowest"      # Hearts
    FIRST = "first"
    NONE = "none"          # empty_hand, capture_all

class TriggerMode(Enum):
    IMMEDIATE = "immediate"
    THRESHOLD_GATE = "threshold_gate"
    ALL_HANDS_EMPTY = "all_hands_empty"
    DECK_EMPTY = "deck_empty"
```

---

## 4. Phase-Specific Mechanics

**Problem:** Phases have implicit behaviors (claim ranks, breaking suits, showdowns).

**Solution:** Add explicit fields to each phase:

### ClaimPhase
```python
@dataclass(frozen=True)
class ClaimPhase:
    # existing fields...
    rank_mode: ClaimRankMode = ClaimRankMode.SEQUENTIAL
    fixed_rank: Optional[Rank] = None
    starting_rank: Rank = Rank.ACE  # Where sequence starts

class ClaimRankMode(Enum):
    SEQUENTIAL = "sequential"      # A,2,3...K,A,2...
    PLAYER_CHOICE = "player_choice"
    FIXED = "fixed"
```

### TrickPhase
```python
@dataclass(frozen=True)
class TrickPhase:
    # existing fields...
    breaking_suit: Optional[Suit] = None
    breaking_rule: BreakingRule = BreakingRule.NONE

class BreakingRule(Enum):
    NONE = "none"
    CANNOT_LEAD_UNTIL_BROKEN = "cannot_lead_until_broken"
    CANNOT_PLAY_UNTIL_BROKEN = "cannot_play_until_broken"
```

### BettingPhase
```python
@dataclass(frozen=True)
class BettingPhase:
    # existing fields...
    showdown_method: ShowdownMethod = ShowdownMethod.HAND_EVALUATION

class ShowdownMethod(Enum):
    HAND_EVALUATION = "hand_evaluation"
    HIGHEST_CARD = "highest_card"
    FOLD_ONLY = "fold_only"
```

### SetupRules (Sequence Mode)
```python
@dataclass(frozen=True)
class SetupRules:
    # existing fields...
    sequence_wrap: bool = False
    sequence_must_match_suit: bool = True
    sequence_gap_allowed: int = 1
```

---

## 5. Game Rules (Edge Cases)

**Problem:** Implicit edge case handling (pass clears tableau, deck reshuffles, tie-breaking).

**Solution:** Add explicit `game_rules`:

```python
@dataclass(frozen=True)
class GameRules:
    consecutive_pass_action: PassAction = PassAction.NONE
    passes_to_trigger: Optional[int] = None  # None = num_players - 1
    deck_empty_action: DeckEmptyAction = DeckEmptyAction.RESHUFFLE_DISCARD
    keep_top_discard: bool = True
    tie_breaker: TieBreaker = TieBreaker.ACTIVE_PLAYER
    same_player_on_win: bool = False

class PassAction(Enum):
    NONE = "none"
    CLEAR_TABLEAU = "clear_tableau"
    END_ROUND = "end_round"
    SKIP_PLAYER = "skip_player"

class DeckEmptyAction(Enum):
    RESHUFFLE_DISCARD = "reshuffle_discard"
    GAME_ENDS = "game_ends"
    SKIP_DRAW = "skip_draw"

class TieBreaker(Enum):
    ACTIVE_PLAYER = "active_player"
    ALTERNATING = "alternating"
    SPLIT = "split"
    BATTLE = "battle"
```

---

## 6. AI Hints (Optional)

**Problem:** AI strategies have hardcoded thresholds.

**Solution:** Add optional `ai_hints` (not required for completeness):

```python
@dataclass(frozen=True)
class AIHints:
    """Optional hints for AI players. Not required for game validity."""
    stand_threshold: Optional[int] = None
    raise_threshold: float = 0.7
    call_threshold: float = 0.3
    bluff_frequency: float = 0.1
    pair_weight: float = 0.2
    high_card_weight: float = 0.4
    suit_weight: float = 0.1
```

**Note:** AI hints are optimizer suggestions, not game rules. Genomes are "complete" without them.

---

## 7. Complete Schema

```python
@dataclass(frozen=True)
class GameGenome:
    # Existing fields
    schema_version: str
    genome_id: str
    generation: int
    setup: SetupRules
    turn_structure: TurnStructure
    win_conditions: tuple[WinCondition, ...]
    special_effects: tuple[SpecialEffect, ...]
    scoring_rules: tuple[ScoringRule, ...]
    max_turns: int
    player_count: int

    # NEW: Explicit mechanics
    card_scoring: tuple[CardScoringRule, ...] = ()
    hand_evaluation: Optional[HandEvaluation] = None
    game_rules: GameRules = GameRules()  # Explicit defaults, no inference
    ai_hints: Optional[AIHints] = None
```

---

## 8. Validation Rules (NEW)

**Problem:** Original design had no validation - invalid field combinations would fail silently at runtime.

**Solution:** Define explicit validation rules checked at genome load time:

```python
class GenomeValidator:
    """Validates genome consistency at parse time."""

    @staticmethod
    def validate(genome: GameGenome) -> list[str]:
        """Return list of validation errors (empty = valid)."""
        errors = []

        # 1. Score-based wins require scoring rules
        score_wins = {"high_score", "low_score", "first_to_score"}
        has_score_win = any(wc.type in score_wins for wc in genome.win_conditions)
        has_scoring = bool(genome.card_scoring) or bool(genome.scoring_rules)

        if has_score_win and not has_scoring:
            errors.append(
                "Score-based win condition requires card_scoring or scoring_rules"
            )

        # 2. best_hand win requires hand_evaluation with PATTERN_MATCH
        has_best_hand = any(wc.type == "best_hand" for wc in genome.win_conditions)
        has_pattern_eval = (
            genome.hand_evaluation is not None
            and genome.hand_evaluation.method == HandEvaluationMethod.PATTERN_MATCH
        )

        if has_best_hand and not has_pattern_eval:
            errors.append(
                "best_hand win condition requires hand_evaluation with PATTERN_MATCH"
            )

        # 3. Betting phase requires starting_chips > 0
        has_betting = any(
            isinstance(p, BettingPhase)
            for p in genome.turn_structure.phases
        )

        if has_betting and genome.setup.starting_chips <= 0:
            errors.append(
                "BettingPhase requires setup.starting_chips > 0"
            )

        # 4. Betting showdown=HAND_EVALUATION requires hand_evaluation
        for phase in genome.turn_structure.phases:
            if isinstance(phase, BettingPhase):
                if phase.showdown_method == ShowdownMethod.HAND_EVALUATION:
                    if genome.hand_evaluation is None:
                        errors.append(
                            "BettingPhase with HAND_EVALUATION showdown requires hand_evaluation"
                        )

        # 5. Capture wins require capture mechanic
        capture_wins = {"capture_all", "most_captured"}
        has_capture_win = any(wc.type in capture_wins for wc in genome.win_conditions)
        has_capture = genome.setup.tableau_mode in {TableauMode.WAR, TableauMode.MATCH_RANK}

        if has_capture_win and not has_capture:
            errors.append(
                "Capture win condition requires tableau_mode WAR or MATCH_RANK"
            )

        # 6. HandPattern constraints must be internally consistent
        if genome.hand_evaluation and genome.hand_evaluation.patterns:
            for pattern in genome.hand_evaluation.patterns:
                # same_rank_groups sum can't exceed required_count
                if pattern.same_rank_groups and pattern.required_count:
                    group_sum = sum(pattern.same_rank_groups)
                    if group_sum > pattern.required_count:
                        errors.append(
                            f"HandPattern '{pattern.name}': same_rank_groups sum "
                            f"({group_sum}) exceeds required_count ({pattern.required_count})"
                        )

        # 7. Card values must cover all ranks if method is POINT_TOTAL
        if genome.hand_evaluation:
            if genome.hand_evaluation.method == HandEvaluationMethod.POINT_TOTAL:
                defined_ranks = {cv.rank for cv in genome.hand_evaluation.card_values}
                all_ranks = set(Rank)
                missing = all_ranks - defined_ranks
                if missing:
                    errors.append(
                        f"POINT_TOTAL requires card_values for all ranks, "
                        f"missing: {[r.value for r in missing]}"
                    )

        return errors
```

**When validation runs:**
- `GameGenome.from_dict()` - Parse time
- `create_*_genome()` factory functions - Creation time
- `mutate()` - After mutation (reject invalid mutations)
- `crossover()` - After crossover (reject invalid offspring)

---

## 9. Migration Strategy (REVISED)

**Original flaw:** Auto-inference creates hidden coupling and makes debugging impossible.

**New approach:** Migrate all 18 seed genomes to explicit format, then remove all inference code.

### Migration Steps

1. **Update each seed genome factory** to include explicit fields:
   - Hearts: Add `card_scoring` with TRICK_WIN rules
   - Poker: Add `hand_evaluation` with full `HandPattern` list
   - Blackjack: Add `hand_evaluation` with `card_values`
   - Go Fish: Add `card_scoring` with SET_COMPLETE trigger
   - etc.

2. **Run completeness tests** - All 18 must pass

3. **Remove inference code** from simulator - Genomes must be self-describing

4. **Update mutation operators** to only produce valid genomes

### Migration Table

| Seed Genome | Status | Migration Needed |
|-------------|--------|------------------|
| War | Complete | None |
| Scopa | Complete | None |
| UNO | Complete | None |
| Crazy Eights | Complete | None |
| Old Maid | Complete | None |
| Go Fish | Complete | None |
| Rummy | Complete | None |
| Solitaire | Complete | None |
| Hearts | Incomplete | Add card_scoring |
| Spades | Incomplete | Add card_scoring |
| Simple Poker | Incomplete | Add hand_evaluation with patterns |
| Draw Poker | Incomplete | Add hand_evaluation with patterns |
| Blackjack | Incomplete | Add hand_evaluation with card_values |
| Betting War | Incomplete | Add showdown rules |
| Bridge | Incomplete | Add card_scoring, partner rules |
| Pinochle | Incomplete | Add card_scoring, meld patterns |
| Euchre | Incomplete | Add trump rules, card_scoring |
| Canasta | Incomplete | Add meld patterns, card_scoring |

---

## 10. New Types Summary

| Type | Purpose |
|------|---------|
| `CardScoringRule` | Explicit point scoring |
| `ScoringTrigger` | When scoring happens |
| `HandEvaluation` | Hand comparison configuration |
| `HandPattern` | Compositional pattern definition (replaces HandType enum) |
| `CardValue` | Point values per rank |
| `WinComparison` | Highest/lowest/first wins |
| `TriggerMode` | When win condition activates |
| `ClaimRankMode` | How claim rank is determined |
| `BreakingRule` | Hearts-style suit restrictions |
| `ShowdownMethod` | Betting resolution |
| `GameRules` | Edge case handling |
| `PassAction` | Consecutive pass behavior |
| `DeckEmptyAction` | Deck exhaustion behavior |
| `TieBreaker` | Tie resolution |
| `AIHints` | Optimizer suggestions (optional) |
| `GenomeValidator` | Schema validation at parse time |

---

## Success Criteria

After implementation:
1. All 18 seed genomes pass completeness tests
2. All 18 seed genomes pass validation tests
3. Rulebook generator produces accurate, playable rules
4. Genome JSON is sufficient to implement game in any language
5. Evolution produces only fully-specified, valid games
6. **No auto-inference code exists** - Simulator only reads explicit fields

---

## Implementation Order

1. **Add new types** to `schema.py` (CardScoringRule, HandPattern, etc.)
2. **Add GenomeValidator** with all validation rules
3. **Migrate seed genomes** one by one, running tests after each
4. **Update completeness tests** to use validator
5. **Remove inference code** from Go simulator
6. **Update mutation operators** to produce valid genomes
7. **Update rulebook generator** to use new explicit fields
