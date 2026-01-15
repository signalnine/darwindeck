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

## 6. Complete Schema

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
```

---

## 7. Validation Rules

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

## 8. Migration Strategy

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

## 9. New Types Summary

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
| `GenomeValidator` | Schema validation at parse time |

---

## 10. Testing Plan

### Unit Tests: Type Definitions

**File:** `tests/unit/test_self_describing_types.py`

```python
class TestCardScoringRule:
    def test_create_hearts_scoring(self):
        """CardScoringRule can express Hearts scoring."""
        rule = CardScoringRule(
            condition=CardCondition(suit=Suit.HEARTS),
            points=1,
            trigger=ScoringTrigger.TRICK_WIN
        )
        assert rule.points == 1
        assert rule.trigger == ScoringTrigger.TRICK_WIN

    def test_negative_points(self):
        """Points can be negative (e.g., penalty cards)."""
        rule = CardScoringRule(
            condition=CardCondition(rank=Rank.ACE),
            points=-10,
            trigger=ScoringTrigger.HAND_END
        )
        assert rule.points == -10

class TestHandPattern:
    def test_flush_pattern(self):
        """HandPattern can express a flush."""
        pattern = HandPattern(
            name="Flush",
            rank_priority=60,
            required_count=5,
            same_suit_count=5,
        )
        assert pattern.same_suit_count == 5

    def test_full_house_pattern(self):
        """HandPattern can express a full house."""
        pattern = HandPattern(
            name="Full House",
            rank_priority=70,
            required_count=5,
            same_rank_groups=(3, 2),
        )
        assert pattern.same_rank_groups == (3, 2)

    def test_straight_with_wrap(self):
        """HandPattern can express wrap-around straight (A-2-3-4-5)."""
        pattern = HandPattern(
            name="Straight",
            rank_priority=50,
            sequence_length=5,
            sequence_wrap=True,
        )
        assert pattern.sequence_wrap is True

class TestHandEvaluation:
    def test_blackjack_card_values(self):
        """HandEvaluation can express Blackjack card values."""
        eval = HandEvaluation(
            method=HandEvaluationMethod.POINT_TOTAL,
            card_values=(
                CardValue(rank=Rank.ACE, value=11, alternate_value=1),
                CardValue(rank=Rank.KING, value=10),
            ),
            target_value=21,
            bust_threshold=22,
        )
        assert eval.target_value == 21
        assert eval.card_values[0].alternate_value == 1
```

### Unit Tests: Validation

**File:** `tests/unit/test_genome_validator.py`

```python
class TestGenomeValidator:
    def test_valid_war_genome(self):
        """War genome passes validation."""
        genome = create_war_genome()
        errors = GenomeValidator.validate(genome)
        assert errors == []

    def test_score_win_requires_scoring(self):
        """Score-based win without scoring rules fails validation."""
        genome = GameGenome(
            ...,
            win_conditions=(WinCondition(type="high_score"),),
            card_scoring=(),
            scoring_rules=(),
        )
        errors = GenomeValidator.validate(genome)
        assert "Score-based win condition requires" in errors[0]

    def test_best_hand_requires_pattern_match(self):
        """best_hand win without PATTERN_MATCH fails validation."""
        genome = GameGenome(
            ...,
            win_conditions=(WinCondition(type="best_hand"),),
            hand_evaluation=None,
        )
        errors = GenomeValidator.validate(genome)
        assert "best_hand win condition requires" in errors[0]

    def test_betting_requires_chips(self):
        """BettingPhase without starting_chips fails validation."""
        genome = GameGenome(
            ...,
            setup=SetupRules(cards_per_player=5, starting_chips=0),
            turn_structure=TurnStructure(phases=(BettingPhase(min_bet=10),)),
        )
        errors = GenomeValidator.validate(genome)
        assert "starting_chips > 0" in errors[0]

    def test_hand_pattern_constraint_consistency(self):
        """HandPattern with invalid constraints fails validation."""
        genome = GameGenome(
            ...,
            hand_evaluation=HandEvaluation(
                method=HandEvaluationMethod.PATTERN_MATCH,
                patterns=(
                    HandPattern(
                        name="Invalid",
                        rank_priority=50,
                        required_count=5,
                        same_rank_groups=(4, 3),  # Sum=7 > 5
                    ),
                ),
            ),
        )
        errors = GenomeValidator.validate(genome)
        assert "same_rank_groups sum" in errors[0]

    def test_point_total_requires_all_ranks(self):
        """POINT_TOTAL with missing card values fails validation."""
        genome = GameGenome(
            ...,
            hand_evaluation=HandEvaluation(
                method=HandEvaluationMethod.POINT_TOTAL,
                card_values=(CardValue(rank=Rank.ACE, value=11),),  # Missing others
            ),
        )
        errors = GenomeValidator.validate(genome)
        assert "missing" in errors[0]
```

### Unit Tests: Completeness

**File:** `tests/unit/test_genome_completeness.py` (existing, updated)

```python
class TestAllSeedGenomesComplete:
    """After migration, ALL seed genomes must be complete."""

    @pytest.mark.parametrize("genome", get_seed_genomes())
    def test_seed_genome_is_complete(self, genome: GameGenome):
        """Every seed genome is self-describing."""
        result = check_genome_completeness(genome)
        assert result.complete, f"{genome.genome_id}: {result}"

    @pytest.mark.parametrize("genome", get_seed_genomes())
    def test_seed_genome_passes_validation(self, genome: GameGenome):
        """Every seed genome passes validation."""
        errors = GenomeValidator.validate(genome)
        assert errors == [], f"{genome.genome_id}: {errors}"
```

### Integration Tests: Pattern Matching

**File:** `tests/integration/test_hand_pattern_matching.py`

```python
class TestHandPatternMatching:
    """Test that HandPattern matching works correctly in simulation."""

    def test_flush_detected(self):
        """Five cards of same suit matches flush pattern."""
        hand = [
            Card(Suit.HEARTS, Rank.TWO),
            Card(Suit.HEARTS, Rank.FIVE),
            Card(Suit.HEARTS, Rank.SEVEN),
            Card(Suit.HEARTS, Rank.JACK),
            Card(Suit.HEARTS, Rank.KING),
        ]
        pattern = HandPattern(name="Flush", rank_priority=60, same_suit_count=5)
        assert matches_pattern(hand, pattern)

    def test_straight_detected(self):
        """Five consecutive ranks matches straight pattern."""
        hand = [
            Card(Suit.HEARTS, Rank.FIVE),
            Card(Suit.CLUBS, Rank.SIX),
            Card(Suit.DIAMONDS, Rank.SEVEN),
            Card(Suit.SPADES, Rank.EIGHT),
            Card(Suit.HEARTS, Rank.NINE),
        ]
        pattern = HandPattern(name="Straight", rank_priority=50, sequence_length=5)
        assert matches_pattern(hand, pattern)

    def test_wrap_around_straight(self):
        """A-2-3-4-5 matches straight with wrap enabled."""
        hand = [
            Card(Suit.HEARTS, Rank.ACE),
            Card(Suit.CLUBS, Rank.TWO),
            Card(Suit.DIAMONDS, Rank.THREE),
            Card(Suit.SPADES, Rank.FOUR),
            Card(Suit.HEARTS, Rank.FIVE),
        ]
        pattern = HandPattern(name="Straight", rank_priority=50, sequence_length=5, sequence_wrap=True)
        assert matches_pattern(hand, pattern)

    def test_full_house_detected(self):
        """Three of a kind + pair matches full house pattern."""
        hand = [
            Card(Suit.HEARTS, Rank.KING),
            Card(Suit.CLUBS, Rank.KING),
            Card(Suit.DIAMONDS, Rank.KING),
            Card(Suit.SPADES, Rank.TWO),
            Card(Suit.HEARTS, Rank.TWO),
        ]
        pattern = HandPattern(name="Full House", rank_priority=70, same_rank_groups=(3, 2))
        assert matches_pattern(hand, pattern)

    def test_pattern_priority_ordering(self):
        """Higher priority pattern wins when multiple match."""
        # This hand is both a flush AND a straight flush
        hand = [
            Card(Suit.HEARTS, Rank.FIVE),
            Card(Suit.HEARTS, Rank.SIX),
            Card(Suit.HEARTS, Rank.SEVEN),
            Card(Suit.HEARTS, Rank.EIGHT),
            Card(Suit.HEARTS, Rank.NINE),
        ]
        patterns = (
            HandPattern(name="Straight Flush", rank_priority=90, same_suit_count=5, sequence_length=5),
            HandPattern(name="Flush", rank_priority=60, same_suit_count=5),
            HandPattern(name="Straight", rank_priority=50, sequence_length=5),
        )
        best = find_best_matching_pattern(hand, patterns)
        assert best.name == "Straight Flush"
```

### Integration Tests: Scoring

**File:** `tests/integration/test_card_scoring.py`

```python
class TestCardScoring:
    """Test that CardScoringRule works in simulation."""

    def test_hearts_scoring_on_trick_win(self):
        """Hearts are scored when winning tricks."""
        genome = create_hearts_genome()  # With explicit card_scoring
        result = simulate_game(genome, num_games=100)

        # Player who wins tricks with hearts should have positive score
        assert result.games_played == 100
        assert result.avg_winner_score > 0

    def test_queen_of_spades_13_points(self):
        """Queen of Spades scores 13 points."""
        rule = CardScoringRule(
            condition=CardCondition(suit=Suit.SPADES, rank=Rank.QUEEN),
            points=13,
            trigger=ScoringTrigger.TRICK_WIN
        )
        card = Card(Suit.SPADES, Rank.QUEEN)
        assert card_matches_rule(card, rule)
        assert rule.points == 13
```

### Integration Tests: Rulebook Generation

**File:** `tests/integration/test_rulebook_generation.py`

```python
class TestRulebookGeneration:
    """Test that rulebooks are generated from explicit fields."""

    def test_hearts_rulebook_includes_scoring(self):
        """Hearts rulebook describes explicit scoring rules."""
        genome = create_hearts_genome()
        rulebook = generate_rulebook(genome)

        assert "1 point" in rulebook.lower()
        assert "heart" in rulebook.lower()
        assert "queen of spades" in rulebook.lower()
        assert "13 point" in rulebook.lower()

    def test_poker_rulebook_describes_hands(self):
        """Poker rulebook describes hand patterns from constraints."""
        genome = create_simple_poker_genome()
        rulebook = generate_rulebook(genome)

        # Should describe hands from HandPattern constraints
        assert "flush" in rulebook.lower()
        assert "same suit" in rulebook.lower()  # From same_suit_count=5
        assert "straight" in rulebook.lower()
        assert "consecutive" in rulebook.lower()  # From sequence_length=5

    def test_rulebook_no_implicit_assumptions(self):
        """Rulebook doesn't reference anything not in genome."""
        genome = create_war_genome()
        rulebook = generate_rulebook(genome)

        # War genome doesn't have scoring, so rulebook shouldn't mention it
        assert "point" not in rulebook.lower() or "capture" in rulebook.lower()
```

### Property-Based Tests

**File:** `tests/property/test_genome_properties.py`

```python
from hypothesis import given, strategies as st

class TestGenomeProperties:
    """Property-based tests for genome consistency."""

    @given(st.integers(min_value=1, max_value=100))
    def test_hand_pattern_priority_is_total_order(self, seed):
        """Patterns with different priorities produce consistent ordering."""
        patterns = generate_random_patterns(seed)
        for p1 in patterns:
            for p2 in patterns:
                if p1.rank_priority > p2.rank_priority:
                    # p1 should always beat p2
                    assert compare_patterns(p1, p2) > 0

    @given(st.integers(min_value=0, max_value=10000))
    def test_validated_genomes_simulate_without_error(self, seed):
        """Any genome that passes validation can be simulated."""
        genome = generate_random_genome(seed)
        errors = GenomeValidator.validate(genome)

        if not errors:
            # Should not crash
            result = simulate_game(genome, num_games=10)
            assert result.games_played == 10

    @given(st.integers(min_value=0, max_value=10000))
    def test_mutation_preserves_validity(self, seed):
        """Mutating a valid genome produces another valid genome."""
        genome = create_war_genome()
        assert GenomeValidator.validate(genome) == []

        mutated = mutate(genome, seed=seed)
        errors = GenomeValidator.validate(mutated)
        assert errors == [], f"Mutation produced invalid genome: {errors}"
```

### End-to-End Tests

**File:** `tests/e2e/test_evolution_produces_valid_games.py`

```python
class TestEvolutionProducesValidGames:
    """End-to-end test that evolution only produces valid, complete games."""

    def test_evolution_10_generations(self):
        """10 generations of evolution produces only valid genomes."""
        results = run_evolution(generations=10, population=20)

        for genome in results.final_population:
            # Every genome must pass validation
            errors = GenomeValidator.validate(genome)
            assert errors == [], f"{genome.genome_id}: {errors}"

            # Every genome must be complete (no implicit dependencies)
            completeness = check_genome_completeness(genome)
            assert completeness.complete, f"{genome.genome_id}: {completeness}"

    def test_evolved_genomes_produce_accurate_rulebooks(self):
        """Evolved genomes produce rulebooks that match their mechanics."""
        results = run_evolution(generations=5, population=10)
        best = results.best_genome

        rulebook = generate_rulebook(best)

        # If genome has card_scoring, rulebook mentions it
        if best.card_scoring:
            for rule in best.card_scoring:
                assert str(rule.points) in rulebook

        # If genome has hand_evaluation, rulebook describes patterns
        if best.hand_evaluation and best.hand_evaluation.patterns:
            for pattern in best.hand_evaluation.patterns:
                assert pattern.name.lower() in rulebook.lower()
```

---

## 11. Success Criteria

After implementation:
1. All 18 seed genomes pass completeness tests
2. All 18 seed genomes pass validation tests
3. All unit tests pass (types, validation, completeness)
4. All integration tests pass (pattern matching, scoring, rulebook)
5. Property-based tests pass (mutation preserves validity)
6. End-to-end tests pass (evolution produces only valid games)
7. Rulebook generator produces accurate, playable rules from explicit fields
8. Genome JSON is sufficient to implement game in any language
9. **No auto-inference code exists** - Simulator only reads explicit fields

---

## 12. Implementation Order

1. **Add new types** to `schema.py` (CardScoringRule, HandPattern, etc.)
2. **Add GenomeValidator** with all validation rules
3. **Write unit tests** for new types and validation
4. **Implement pattern matching** logic (matches_pattern, find_best_matching_pattern)
5. **Write integration tests** for pattern matching
6. **Migrate seed genomes** one by one, running tests after each
7. **Update completeness tests** to require all seeds pass
8. **Remove inference code** from Go simulator
9. **Update mutation operators** to produce valid genomes
10. **Write property-based tests** for mutation validity
11. **Update rulebook generator** to use explicit fields
12. **Write end-to-end tests** for evolution validity
