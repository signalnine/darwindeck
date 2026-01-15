# Self-Describing Genomes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make genomes fully self-describing by adding explicit scoring, hand evaluation, and game rules - eliminating all implicit simulator mechanics.

**Architecture:** Add new dataclasses to schema.py (CardScoringRule, HandPattern, HandEvaluation, GameRules), implement GenomeValidator, migrate all 18 seed genomes to explicit format, update completeness tests to require all seeds pass.

**Tech Stack:** Python 3.11+, pytest, dataclasses, frozen immutable types

**Design Document:** `docs/plans/2026-01-15-self-describing-genomes-design.md`

---

## Task 1: Add ScoringTrigger Enum

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Test: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Create new test file:

```python
# tests/unit/test_self_describing_types.py
"""Tests for self-describing genome types."""

import pytest
from darwindeck.genome.schema import ScoringTrigger


class TestScoringTrigger:
    def test_scoring_trigger_enum_values(self):
        """ScoringTrigger enum has expected values."""
        assert ScoringTrigger.TRICK_WIN.value == "trick_win"
        assert ScoringTrigger.CAPTURE.value == "capture"
        assert ScoringTrigger.PLAY.value == "play"
        assert ScoringTrigger.HAND_END.value == "hand_end"
        assert ScoringTrigger.SET_COMPLETE.value == "set_complete"
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestScoringTrigger::test_scoring_trigger_enum_values -v
```

Expected: FAIL with `ImportError: cannot import name 'ScoringTrigger'`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py` after the `BettingAction` enum (~line 90):

```python
class ScoringTrigger(Enum):
    """When scoring happens for a card."""
    TRICK_WIN = "trick_win"       # Score when winning trick with this card
    CAPTURE = "capture"           # Score when capturing this card
    PLAY = "play"                 # Score when playing this card
    HAND_END = "hand_end"         # Score for cards in hand at end
    SET_COMPLETE = "set_complete" # Score when completing a set (Go Fish)
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestScoringTrigger -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add ScoringTrigger enum for explicit card scoring"
```

---

## Task 2: Add CardCondition Dataclass

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import CardCondition, Suit, Rank


class TestCardCondition:
    def test_card_condition_suit_only(self):
        """CardCondition can match by suit."""
        cond = CardCondition(suit=Suit.HEARTS)
        assert cond.suit == Suit.HEARTS
        assert cond.rank is None

    def test_card_condition_rank_only(self):
        """CardCondition can match by rank."""
        cond = CardCondition(rank=Rank.QUEEN)
        assert cond.rank == Rank.QUEEN
        assert cond.suit is None

    def test_card_condition_both(self):
        """CardCondition can match by suit and rank."""
        cond = CardCondition(suit=Suit.SPADES, rank=Rank.QUEEN)
        assert cond.suit == Suit.SPADES
        assert cond.rank == Rank.QUEEN

    def test_card_condition_frozen(self):
        """CardCondition is immutable."""
        cond = CardCondition(suit=Suit.HEARTS)
        with pytest.raises(AttributeError):
            cond.suit = Suit.CLUBS
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestCardCondition -v
```

Expected: FAIL with `ImportError: cannot import name 'CardCondition'`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py` after `ScoringTrigger`:

```python
@dataclass(frozen=True)
class CardCondition:
    """Condition to match a card by suit and/or rank."""
    suit: Optional[Suit] = None
    rank: Optional[Rank] = None
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestCardCondition -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add CardCondition dataclass for card matching"
```

---

## Task 3: Add CardScoringRule Dataclass

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import CardScoringRule, ScoringTrigger


class TestCardScoringRule:
    def test_hearts_scoring_rule(self):
        """CardScoringRule can express Hearts 1-point-per-heart."""
        rule = CardScoringRule(
            condition=CardCondition(suit=Suit.HEARTS),
            points=1,
            trigger=ScoringTrigger.TRICK_WIN
        )
        assert rule.points == 1
        assert rule.trigger == ScoringTrigger.TRICK_WIN
        assert rule.condition.suit == Suit.HEARTS

    def test_queen_of_spades_scoring(self):
        """CardScoringRule can express Queen of Spades 13 points."""
        rule = CardScoringRule(
            condition=CardCondition(suit=Suit.SPADES, rank=Rank.QUEEN),
            points=13,
            trigger=ScoringTrigger.TRICK_WIN
        )
        assert rule.points == 13

    def test_negative_points(self):
        """CardScoringRule can have negative points."""
        rule = CardScoringRule(
            condition=CardCondition(rank=Rank.ACE),
            points=-10,
            trigger=ScoringTrigger.HAND_END
        )
        assert rule.points == -10

    def test_card_scoring_rule_frozen(self):
        """CardScoringRule is immutable."""
        rule = CardScoringRule(
            condition=CardCondition(suit=Suit.HEARTS),
            points=1,
            trigger=ScoringTrigger.TRICK_WIN
        )
        with pytest.raises(AttributeError):
            rule.points = 5
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestCardScoringRule -v
```

Expected: FAIL with `ImportError: cannot import name 'CardScoringRule'`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py` after `CardCondition`:

```python
@dataclass(frozen=True)
class CardScoringRule:
    """Score points when a card meets a condition."""
    condition: CardCondition
    points: int
    trigger: ScoringTrigger
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestCardScoringRule -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add CardScoringRule dataclass for explicit scoring"
```

---

## Task 4: Add HandEvaluationMethod Enum

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import HandEvaluationMethod


class TestHandEvaluationMethod:
    def test_hand_evaluation_method_values(self):
        """HandEvaluationMethod enum has expected values."""
        assert HandEvaluationMethod.NONE.value == "none"
        assert HandEvaluationMethod.HIGH_CARD.value == "high_card"
        assert HandEvaluationMethod.POINT_TOTAL.value == "point_total"
        assert HandEvaluationMethod.PATTERN_MATCH.value == "pattern_match"
        assert HandEvaluationMethod.CARD_COUNT.value == "card_count"
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestHandEvaluationMethod -v
```

Expected: FAIL with `ImportError: cannot import name 'HandEvaluationMethod'`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py`:

```python
class HandEvaluationMethod(Enum):
    """How to evaluate and compare hands."""
    NONE = "none"
    HIGH_CARD = "high_card"          # Compare highest cards
    POINT_TOTAL = "point_total"      # Sum card values (Blackjack)
    PATTERN_MATCH = "pattern_match"  # Match patterns in priority order
    CARD_COUNT = "card_count"        # Most cards wins (War)
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestHandEvaluationMethod -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add HandEvaluationMethod enum"
```

---

## Task 5: Add CardValue Dataclass

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import CardValue


class TestCardValue:
    def test_card_value_simple(self):
        """CardValue can express simple point value."""
        cv = CardValue(rank=Rank.KING, value=10)
        assert cv.rank == Rank.KING
        assert cv.value == 10
        assert cv.alternate_value is None

    def test_card_value_with_alternate(self):
        """CardValue can express alternate value (Ace in Blackjack)."""
        cv = CardValue(rank=Rank.ACE, value=11, alternate_value=1)
        assert cv.value == 11
        assert cv.alternate_value == 1

    def test_card_value_frozen(self):
        """CardValue is immutable."""
        cv = CardValue(rank=Rank.KING, value=10)
        with pytest.raises(AttributeError):
            cv.value = 20
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestCardValue -v
```

Expected: FAIL with `ImportError: cannot import name 'CardValue'`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py`:

```python
@dataclass(frozen=True)
class CardValue:
    """Point value for a card rank."""
    rank: Rank
    value: int
    alternate_value: Optional[int] = None
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestCardValue -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add CardValue dataclass for point totals"
```

---

## Task 6: Add HandPattern Dataclass

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import HandPattern


class TestHandPattern:
    def test_flush_pattern(self):
        """HandPattern can express a flush (5 same suit)."""
        pattern = HandPattern(
            name="Flush",
            rank_priority=60,
            required_count=5,
            same_suit_count=5,
        )
        assert pattern.name == "Flush"
        assert pattern.rank_priority == 60
        assert pattern.same_suit_count == 5

    def test_full_house_pattern(self):
        """HandPattern can express full house (3+2)."""
        pattern = HandPattern(
            name="Full House",
            rank_priority=70,
            required_count=5,
            same_rank_groups=(3, 2),
        )
        assert pattern.same_rank_groups == (3, 2)

    def test_straight_pattern(self):
        """HandPattern can express straight (5 consecutive)."""
        pattern = HandPattern(
            name="Straight",
            rank_priority=50,
            required_count=5,
            sequence_length=5,
            sequence_wrap=True,
        )
        assert pattern.sequence_length == 5
        assert pattern.sequence_wrap is True

    def test_royal_flush_pattern(self):
        """HandPattern can express royal flush with required ranks."""
        pattern = HandPattern(
            name="Royal Flush",
            rank_priority=100,
            required_count=5,
            same_suit_count=5,
            sequence_length=5,
            required_ranks=(Rank.TEN, Rank.JACK, Rank.QUEEN, Rank.KING, Rank.ACE),
        )
        assert pattern.required_ranks == (Rank.TEN, Rank.JACK, Rank.QUEEN, Rank.KING, Rank.ACE)

    def test_hand_pattern_frozen(self):
        """HandPattern is immutable."""
        pattern = HandPattern(name="Test", rank_priority=10)
        with pytest.raises(AttributeError):
            pattern.name = "Changed"
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestHandPattern -v
```

Expected: FAIL with `ImportError: cannot import name 'HandPattern'`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py`:

```python
@dataclass(frozen=True)
class HandPattern:
    """A pattern to match in a hand. Fully describes what to look for."""
    name: str
    rank_priority: int  # Higher = better hand (100 > 50)

    # Constraints (all must be satisfied)
    required_count: Optional[int] = None   # Exactly N cards
    same_suit_count: Optional[int] = None  # N cards must share suit
    same_rank_groups: Optional[tuple[int, ...]] = None  # (3, 2) = three + pair
    sequence_length: Optional[int] = None  # N consecutive ranks
    sequence_wrap: bool = False            # A-2-3 and Q-K-A both valid
    required_ranks: Optional[tuple[Rank, ...]] = None  # Must contain these ranks
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestHandPattern -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add HandPattern dataclass for compositional hand definitions"
```

---

## Task 7: Add HandEvaluation Dataclass

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import HandEvaluation


class TestHandEvaluation:
    def test_poker_hand_evaluation(self):
        """HandEvaluation can express poker with patterns."""
        eval = HandEvaluation(
            method=HandEvaluationMethod.PATTERN_MATCH,
            patterns=(
                HandPattern(name="Flush", rank_priority=60, same_suit_count=5),
                HandPattern(name="Pair", rank_priority=20, same_rank_groups=(2,)),
            ),
        )
        assert eval.method == HandEvaluationMethod.PATTERN_MATCH
        assert len(eval.patterns) == 2

    def test_blackjack_hand_evaluation(self):
        """HandEvaluation can express blackjack with card values."""
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
        assert eval.bust_threshold == 22

    def test_hand_evaluation_frozen(self):
        """HandEvaluation is immutable."""
        eval = HandEvaluation(method=HandEvaluationMethod.HIGH_CARD)
        with pytest.raises(AttributeError):
            eval.method = HandEvaluationMethod.PATTERN_MATCH
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestHandEvaluation -v
```

Expected: FAIL with `ImportError: cannot import name 'HandEvaluation'`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py`:

```python
@dataclass(frozen=True)
class HandEvaluation:
    """How to evaluate and compare hands."""
    method: HandEvaluationMethod
    patterns: tuple[HandPattern, ...] = ()  # For PATTERN_MATCH
    card_values: tuple[CardValue, ...] = ()  # For POINT_TOTAL
    target_value: Optional[int] = None       # Blackjack: 21
    bust_threshold: Optional[int] = None     # Blackjack: 22
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestHandEvaluation -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add HandEvaluation dataclass"
```

---

## Task 8: Add WinComparison and TriggerMode Enums

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import WinComparison, TriggerMode


class TestWinConditionEnums:
    def test_win_comparison_values(self):
        """WinComparison enum has expected values."""
        assert WinComparison.HIGHEST.value == "highest"
        assert WinComparison.LOWEST.value == "lowest"
        assert WinComparison.FIRST.value == "first"
        assert WinComparison.NONE.value == "none"

    def test_trigger_mode_values(self):
        """TriggerMode enum has expected values."""
        assert TriggerMode.IMMEDIATE.value == "immediate"
        assert TriggerMode.THRESHOLD_GATE.value == "threshold_gate"
        assert TriggerMode.ALL_HANDS_EMPTY.value == "all_hands_empty"
        assert TriggerMode.DECK_EMPTY.value == "deck_empty"
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestWinConditionEnums -v
```

Expected: FAIL with `ImportError: cannot import name 'WinComparison'`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py`:

```python
class WinComparison(Enum):
    """How scores are compared for winning."""
    HIGHEST = "highest"
    LOWEST = "lowest"    # Hearts
    FIRST = "first"
    NONE = "none"        # empty_hand, capture_all


class TriggerMode(Enum):
    """When win condition is checked."""
    IMMEDIATE = "immediate"
    THRESHOLD_GATE = "threshold_gate"
    ALL_HANDS_EMPTY = "all_hands_empty"
    DECK_EMPTY = "deck_empty"
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestWinConditionEnums -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add WinComparison and TriggerMode enums"
```

---

## Task 9: Add GameRules Enums (PassAction, DeckEmptyAction, TieBreaker)

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import PassAction, DeckEmptyAction, TieBreaker


class TestGameRulesEnums:
    def test_pass_action_values(self):
        """PassAction enum has expected values."""
        assert PassAction.NONE.value == "none"
        assert PassAction.CLEAR_TABLEAU.value == "clear_tableau"
        assert PassAction.END_ROUND.value == "end_round"
        assert PassAction.SKIP_PLAYER.value == "skip_player"

    def test_deck_empty_action_values(self):
        """DeckEmptyAction enum has expected values."""
        assert DeckEmptyAction.RESHUFFLE_DISCARD.value == "reshuffle_discard"
        assert DeckEmptyAction.GAME_ENDS.value == "game_ends"
        assert DeckEmptyAction.SKIP_DRAW.value == "skip_draw"

    def test_tie_breaker_values(self):
        """TieBreaker enum has expected values."""
        assert TieBreaker.ACTIVE_PLAYER.value == "active_player"
        assert TieBreaker.ALTERNATING.value == "alternating"
        assert TieBreaker.SPLIT.value == "split"
        assert TieBreaker.BATTLE.value == "battle"
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestGameRulesEnums -v
```

Expected: FAIL with `ImportError`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py`:

```python
class PassAction(Enum):
    """What happens when players pass consecutively."""
    NONE = "none"
    CLEAR_TABLEAU = "clear_tableau"
    END_ROUND = "end_round"
    SKIP_PLAYER = "skip_player"


class DeckEmptyAction(Enum):
    """What happens when deck is empty."""
    RESHUFFLE_DISCARD = "reshuffle_discard"
    GAME_ENDS = "game_ends"
    SKIP_DRAW = "skip_draw"


class TieBreaker(Enum):
    """How ties are resolved."""
    ACTIVE_PLAYER = "active_player"
    ALTERNATING = "alternating"
    SPLIT = "split"
    BATTLE = "battle"
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestGameRulesEnums -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add PassAction, DeckEmptyAction, TieBreaker enums"
```

---

## Task 10: Add GameRules Dataclass

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import GameRules


class TestGameRules:
    def test_game_rules_defaults(self):
        """GameRules has sensible defaults."""
        rules = GameRules()
        assert rules.consecutive_pass_action == PassAction.NONE
        assert rules.deck_empty_action == DeckEmptyAction.RESHUFFLE_DISCARD
        assert rules.tie_breaker == TieBreaker.ACTIVE_PLAYER

    def test_game_rules_custom(self):
        """GameRules can be customized."""
        rules = GameRules(
            consecutive_pass_action=PassAction.CLEAR_TABLEAU,
            passes_to_trigger=3,
            deck_empty_action=DeckEmptyAction.GAME_ENDS,
        )
        assert rules.consecutive_pass_action == PassAction.CLEAR_TABLEAU
        assert rules.passes_to_trigger == 3

    def test_game_rules_frozen(self):
        """GameRules is immutable."""
        rules = GameRules()
        with pytest.raises(AttributeError):
            rules.tie_breaker = TieBreaker.SPLIT
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestGameRules -v
```

Expected: FAIL with `ImportError: cannot import name 'GameRules'`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py`:

```python
@dataclass(frozen=True)
class GameRules:
    """Explicit rules for edge cases."""
    consecutive_pass_action: PassAction = PassAction.NONE
    passes_to_trigger: Optional[int] = None  # None = num_players - 1
    deck_empty_action: DeckEmptyAction = DeckEmptyAction.RESHUFFLE_DISCARD
    keep_top_discard: bool = True
    tie_breaker: TieBreaker = TieBreaker.ACTIVE_PLAYER
    same_player_on_win: bool = False
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestGameRules -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add GameRules dataclass for edge case handling"
```

---

## Task 11: Add ClaimRankMode and BreakingRule Enums

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import ClaimRankMode, BreakingRule


class TestPhaseEnums:
    def test_claim_rank_mode_values(self):
        """ClaimRankMode enum has expected values."""
        assert ClaimRankMode.SEQUENTIAL.value == "sequential"
        assert ClaimRankMode.PLAYER_CHOICE.value == "player_choice"
        assert ClaimRankMode.FIXED.value == "fixed"

    def test_breaking_rule_values(self):
        """BreakingRule enum has expected values."""
        assert BreakingRule.NONE.value == "none"
        assert BreakingRule.CANNOT_LEAD_UNTIL_BROKEN.value == "cannot_lead_until_broken"
        assert BreakingRule.CANNOT_PLAY_UNTIL_BROKEN.value == "cannot_play_until_broken"
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestPhaseEnums -v
```

Expected: FAIL with `ImportError`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py`:

```python
class ClaimRankMode(Enum):
    """How claim rank is determined."""
    SEQUENTIAL = "sequential"      # A,2,3...K,A,2...
    PLAYER_CHOICE = "player_choice"
    FIXED = "fixed"


class BreakingRule(Enum):
    """Rule for breaking suits (Hearts)."""
    NONE = "none"
    CANNOT_LEAD_UNTIL_BROKEN = "cannot_lead_until_broken"
    CANNOT_PLAY_UNTIL_BROKEN = "cannot_play_until_broken"
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestPhaseEnums -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add ClaimRankMode and BreakingRule enums"
```

---

## Task 12: Add ShowdownMethod Enum

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import ShowdownMethod


class TestShowdownMethod:
    def test_showdown_method_values(self):
        """ShowdownMethod enum has expected values."""
        assert ShowdownMethod.HAND_EVALUATION.value == "hand_evaluation"
        assert ShowdownMethod.HIGHEST_CARD.value == "highest_card"
        assert ShowdownMethod.FOLD_ONLY.value == "fold_only"
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestShowdownMethod -v
```

Expected: FAIL with `ImportError`

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py`:

```python
class ShowdownMethod(Enum):
    """How betting showdown is resolved."""
    HAND_EVALUATION = "hand_evaluation"
    HIGHEST_CARD = "highest_card"
    FOLD_ONLY = "fold_only"
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestShowdownMethod -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add ShowdownMethod enum"
```

---

## Task 13: Update WinCondition with New Fields

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import WinCondition


class TestWinConditionExtended:
    def test_win_condition_new_fields(self):
        """WinCondition has new explicit fields."""
        wc = WinCondition(
            type="low_score",
            threshold=100,
            comparison=WinComparison.LOWEST,
            trigger_mode=TriggerMode.THRESHOLD_GATE,
        )
        assert wc.comparison == WinComparison.LOWEST
        assert wc.trigger_mode == TriggerMode.THRESHOLD_GATE

    def test_win_condition_defaults(self):
        """WinCondition new fields have sensible defaults."""
        wc = WinCondition(type="empty_hand")
        assert wc.comparison == WinComparison.NONE
        assert wc.trigger_mode == TriggerMode.IMMEDIATE
        assert wc.required_hand_size is None

    def test_win_condition_best_hand(self):
        """WinCondition for best_hand with required_hand_size."""
        wc = WinCondition(
            type="best_hand",
            comparison=WinComparison.HIGHEST,
            required_hand_size=5,
        )
        assert wc.required_hand_size == 5
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestWinConditionExtended -v
```

Expected: FAIL with `TypeError: WinCondition.__init__() got an unexpected keyword argument 'comparison'`

**Step 3: Write minimal implementation**

Modify `WinCondition` in `src/darwindeck/genome/schema.py` (~line 242):

```python
@dataclass(frozen=True)
class WinCondition:
    """How to win the game."""

    type: str  # "empty_hand", "high_score", "first_to_score", "capture_all", "best_hand"
    threshold: Optional[int] = None

    # NEW: Explicit modifiers
    comparison: WinComparison = WinComparison.NONE
    trigger_mode: TriggerMode = TriggerMode.IMMEDIATE
    required_hand_size: Optional[int] = None
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestWinConditionExtended -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add comparison, trigger_mode, required_hand_size to WinCondition"
```

---

## Task 14: Update GameGenome with New Fields

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
class TestGameGenomeExtended:
    def test_genome_has_card_scoring(self):
        """GameGenome has card_scoring field."""
        from darwindeck.genome.examples import create_war_genome

        genome = create_war_genome()
        assert hasattr(genome, 'card_scoring')
        assert genome.card_scoring == ()

    def test_genome_has_hand_evaluation(self):
        """GameGenome has hand_evaluation field."""
        from darwindeck.genome.examples import create_war_genome

        genome = create_war_genome()
        assert hasattr(genome, 'hand_evaluation')
        assert genome.hand_evaluation is None

    def test_genome_has_game_rules(self):
        """GameGenome has game_rules field."""
        from darwindeck.genome.examples import create_war_genome

        genome = create_war_genome()
        assert hasattr(genome, 'game_rules')
        assert isinstance(genome.game_rules, GameRules)
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestGameGenomeExtended -v
```

Expected: FAIL with `AttributeError: 'GameGenome' object has no attribute 'card_scoring'`

**Step 3: Write minimal implementation**

Modify `GameGenome` in `src/darwindeck/genome/schema.py` (~line 250):

```python
@dataclass(frozen=True)
class GameGenome:
    """Complete game specification."""

    schema_version: str
    genome_id: str
    generation: int
    setup: SetupRules
    turn_structure: TurnStructure
    special_effects: list  # type: ignore
    win_conditions: list[WinCondition]
    scoring_rules: list  # type: ignore
    max_turns: int = 100
    player_count: int = 2
    min_turns: int = 10

    # NEW: Self-describing fields
    card_scoring: tuple[CardScoringRule, ...] = ()
    hand_evaluation: Optional[HandEvaluation] = None
    game_rules: GameRules = field(default_factory=GameRules)
```

Note: Need to import `field` from dataclasses at top of file.

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestGameGenomeExtended -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add card_scoring, hand_evaluation, game_rules to GameGenome"
```

---

## Task 15: Add TrickPhase Breaking Rule Field

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
class TestTrickPhaseExtended:
    def test_trick_phase_has_breaking_rule(self):
        """TrickPhase has breaking_rule field."""
        from darwindeck.genome.schema import TrickPhase

        phase = TrickPhase(
            lead_suit_required=True,
            breaking_suit=Suit.HEARTS,
            breaking_rule=BreakingRule.CANNOT_LEAD_UNTIL_BROKEN,
        )
        assert phase.breaking_rule == BreakingRule.CANNOT_LEAD_UNTIL_BROKEN

    def test_trick_phase_breaking_rule_default(self):
        """TrickPhase breaking_rule defaults to NONE."""
        from darwindeck.genome.schema import TrickPhase

        phase = TrickPhase(lead_suit_required=True)
        assert phase.breaking_rule == BreakingRule.NONE
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestTrickPhaseExtended -v
```

Expected: FAIL with `TypeError: TrickPhase.__init__() got an unexpected keyword argument 'breaking_rule'`

**Step 3: Write minimal implementation**

Modify `TrickPhase` in `src/darwindeck/genome/schema.py`:

```python
@dataclass(frozen=True)
class TrickPhase:
    """Trick-taking phase for games like Hearts, Spades, Bridge."""
    lead_suit_required: bool = True
    trump_suit: Optional[Suit] = None
    high_card_wins: bool = True
    breaking_suit: Optional[Suit] = None

    # NEW: Explicit breaking rule
    breaking_rule: BreakingRule = BreakingRule.NONE

    def __post_init__(self):
        pass
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestTrickPhaseExtended -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add breaking_rule field to TrickPhase"
```

---

## Task 16: Add ClaimPhase Rank Mode Fields

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
from darwindeck.genome.schema import ClaimPhase


class TestClaimPhaseExtended:
    def test_claim_phase_has_rank_mode(self):
        """ClaimPhase has rank_mode field."""
        phase = ClaimPhase(
            min_cards=1,
            max_cards=4,
            rank_mode=ClaimRankMode.SEQUENTIAL,
            starting_rank=Rank.ACE,
        )
        assert phase.rank_mode == ClaimRankMode.SEQUENTIAL
        assert phase.starting_rank == Rank.ACE

    def test_claim_phase_fixed_rank(self):
        """ClaimPhase can have fixed rank."""
        phase = ClaimPhase(
            min_cards=1,
            max_cards=4,
            rank_mode=ClaimRankMode.FIXED,
            fixed_rank=Rank.QUEEN,
        )
        assert phase.rank_mode == ClaimRankMode.FIXED
        assert phase.fixed_rank == Rank.QUEEN

    def test_claim_phase_defaults(self):
        """ClaimPhase has sensible defaults."""
        phase = ClaimPhase(min_cards=1, max_cards=4)
        assert phase.rank_mode == ClaimRankMode.SEQUENTIAL
        assert phase.starting_rank == Rank.ACE
        assert phase.fixed_rank is None
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestClaimPhaseExtended -v
```

Expected: FAIL with `TypeError`

**Step 3: Write minimal implementation**

Modify `ClaimPhase` in `src/darwindeck/genome/schema.py`:

```python
@dataclass(frozen=True)
class ClaimPhase:
    """Bluffing/claiming phase for games like Cheat/BS/I Doubt It."""
    min_cards: int = 1
    max_cards: int = 4
    sequential_rank: bool = True
    allow_challenge: bool = True
    pile_penalty: bool = True

    # NEW: Explicit rank mode
    rank_mode: ClaimRankMode = ClaimRankMode.SEQUENTIAL
    starting_rank: Rank = Rank.ACE
    fixed_rank: Optional[Rank] = None
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestClaimPhaseExtended -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add rank_mode, starting_rank, fixed_rank to ClaimPhase"
```

---

## Task 17: Add BettingPhase Showdown Method

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Modify: `tests/unit/test_self_describing_types.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_self_describing_types.py`:

```python
class TestBettingPhaseExtended:
    def test_betting_phase_has_showdown_method(self):
        """BettingPhase has showdown_method field."""
        phase = BettingPhase(
            min_bet=10,
            max_raises=3,
            showdown_method=ShowdownMethod.HAND_EVALUATION,
        )
        assert phase.showdown_method == ShowdownMethod.HAND_EVALUATION

    def test_betting_phase_showdown_default(self):
        """BettingPhase showdown_method defaults to HAND_EVALUATION."""
        phase = BettingPhase(min_bet=10)
        assert phase.showdown_method == ShowdownMethod.HAND_EVALUATION
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestBettingPhaseExtended -v
```

Expected: FAIL with `TypeError`

**Step 3: Write minimal implementation**

Modify `BettingPhase` in `src/darwindeck/genome/schema.py`:

```python
@dataclass(frozen=True)
class BettingPhase:
    """A betting round within the turn structure."""
    min_bet: int = 10
    max_raises: int = 3

    # NEW: Explicit showdown resolution
    showdown_method: ShowdownMethod = ShowdownMethod.HAND_EVALUATION
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_self_describing_types.py::TestBettingPhaseExtended -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_self_describing_types.py
git commit -m "feat(schema): add showdown_method to BettingPhase"
```

---

## Task 18: Create GenomeValidator

**Files:**
- Create: `src/darwindeck/genome/validator.py`
- Create: `tests/unit/test_genome_validator.py`

**Step 1: Write the failing test**

Create `tests/unit/test_genome_validator.py`:

```python
"""Tests for GenomeValidator."""

import pytest
from darwindeck.genome.validator import GenomeValidator
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    BettingPhase, PlayPhase, Location, HandEvaluation,
    HandEvaluationMethod, HandPattern, CardValue, Rank,
    WinComparison, TriggerMode, ShowdownMethod, GameRules,
)
from darwindeck.genome.examples import create_war_genome


class TestGenomeValidator:
    def test_war_genome_valid(self):
        """War genome passes validation."""
        genome = create_war_genome()
        errors = GenomeValidator.validate(genome)
        assert errors == []

    def test_score_win_requires_scoring(self):
        """Score-based win without scoring fails validation."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[]),
            special_effects=[],
            win_conditions=[WinCondition(type="high_score", threshold=100)],
            scoring_rules=[],
            card_scoring=(),  # No scoring!
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) == 1
        assert "Score-based win condition requires" in errors[0]

    def test_best_hand_requires_pattern_match(self):
        """best_hand win without PATTERN_MATCH fails validation."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[]),
            special_effects=[],
            win_conditions=[WinCondition(type="best_hand")],
            scoring_rules=[],
            hand_evaluation=None,  # No hand evaluation!
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) == 1
        assert "best_hand win condition requires" in errors[0]

    def test_betting_requires_chips(self):
        """BettingPhase without starting_chips fails validation."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=0,
            setup=SetupRules(cards_per_player=5, starting_chips=0),
            turn_structure=TurnStructure(phases=[BettingPhase(min_bet=10)]),
            special_effects=[],
            win_conditions=[WinCondition(type="most_chips")],
            scoring_rules=[],
        )
        errors = GenomeValidator.validate(genome)
        assert len(errors) >= 1
        assert any("starting_chips" in e for e in errors)
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_genome_validator.py -v
```

Expected: FAIL with `ModuleNotFoundError: No module named 'darwindeck.genome.validator'`

**Step 3: Write minimal implementation**

Create `src/darwindeck/genome/validator.py`:

```python
"""Genome validation to catch invalid field combinations."""

from typing import List
from darwindeck.genome.schema import (
    GameGenome, BettingPhase, HandEvaluationMethod, TableauMode,
    ShowdownMethod,
)


class GenomeValidator:
    """Validates genome consistency at parse time."""

    @staticmethod
    def validate(genome: GameGenome) -> List[str]:
        """Return list of validation errors (empty = valid)."""
        errors: List[str] = []

        # Get win condition types
        win_types = {wc.type for wc in genome.win_conditions}

        # Check 1: Score-based wins require scoring rules
        score_wins = {"high_score", "low_score", "first_to_score"}
        if win_types & score_wins:
            has_scoring = bool(genome.card_scoring) or bool(genome.scoring_rules)
            if not has_scoring:
                errors.append(
                    "Score-based win condition requires card_scoring or scoring_rules"
                )

        # Check 2: best_hand win requires hand_evaluation with PATTERN_MATCH
        if "best_hand" in win_types:
            has_pattern_eval = (
                genome.hand_evaluation is not None
                and genome.hand_evaluation.method == HandEvaluationMethod.PATTERN_MATCH
            )
            if not has_pattern_eval:
                errors.append(
                    "best_hand win condition requires hand_evaluation with PATTERN_MATCH"
                )

        # Check 3: Betting phase requires starting_chips > 0
        has_betting = any(
            isinstance(p, BettingPhase)
            for p in genome.turn_structure.phases
        )
        if has_betting and genome.setup.starting_chips <= 0:
            errors.append(
                "BettingPhase requires setup.starting_chips > 0"
            )

        # Check 4: Betting showdown=HAND_EVALUATION requires hand_evaluation
        for phase in genome.turn_structure.phases:
            if isinstance(phase, BettingPhase):
                if phase.showdown_method == ShowdownMethod.HAND_EVALUATION:
                    if genome.hand_evaluation is None:
                        errors.append(
                            "BettingPhase with HAND_EVALUATION showdown requires hand_evaluation"
                        )

        # Check 5: Capture wins require capture mechanic
        capture_wins = {"capture_all", "most_captured"}
        if win_types & capture_wins:
            has_capture = genome.setup.tableau_mode in {TableauMode.WAR, TableauMode.MATCH_RANK}
            if not has_capture:
                errors.append(
                    "Capture win condition requires tableau_mode WAR or MATCH_RANK"
                )

        # Check 6: HandPattern constraints must be internally consistent
        if genome.hand_evaluation and genome.hand_evaluation.patterns:
            for pattern in genome.hand_evaluation.patterns:
                if pattern.same_rank_groups and pattern.required_count:
                    group_sum = sum(pattern.same_rank_groups)
                    if group_sum > pattern.required_count:
                        errors.append(
                            f"HandPattern '{pattern.name}': same_rank_groups sum "
                            f"({group_sum}) exceeds required_count ({pattern.required_count})"
                        )

        return errors
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_genome_validator.py -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/validator.py tests/unit/test_genome_validator.py
git commit -m "feat(validator): add GenomeValidator for schema consistency"
```

---

## Task 19: Migrate Hearts Genome (Add Card Scoring)

**Files:**
- Modify: `src/darwindeck/genome/examples.py`
- Modify: `tests/unit/test_genome_completeness.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_genome_completeness.py`:

```python
class TestMigratedGenomesComplete:
    """Test that migrated genomes are now complete."""

    def test_hearts_genome_is_complete(self):
        """Hearts genome with card_scoring is complete."""
        from darwindeck.genome.examples import create_hearts_genome
        genome = create_hearts_genome()
        result = check_genome_completeness(genome)
        assert result.complete, f"Hearts genome incomplete: {result}"

        # Verify card_scoring is populated
        assert len(genome.card_scoring) >= 2  # Hearts + QS
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_genome_completeness.py::TestMigratedGenomesComplete::test_hearts_genome_is_complete -v
```

Expected: FAIL (Hearts currently has no card_scoring)

**Step 3: Write minimal implementation**

Modify `create_hearts_genome()` in `src/darwindeck/genome/examples.py`:

```python
def create_hearts_genome() -> GameGenome:
    """Create classic 4-player Hearts genome with explicit scoring."""
    from darwindeck.genome.schema import CardScoringRule, CardCondition, ScoringTrigger

    return GameGenome(
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
                    breaking_rule=BreakingRule.CANNOT_LEAD_UNTIL_BROKEN,
                )
            ],
            is_trick_based=True,
            tricks_per_hand=13,
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="low_score",
                threshold=100,
                comparison=WinComparison.LOWEST,
                trigger_mode=TriggerMode.THRESHOLD_GATE,
            ),
            WinCondition(type="all_hands_empty"),
        ],
        scoring_rules=[],
        # NEW: Explicit card scoring
        card_scoring=(
            CardScoringRule(
                condition=CardCondition(suit=Suit.HEARTS),
                points=1,
                trigger=ScoringTrigger.TRICK_WIN,
            ),
            CardScoringRule(
                condition=CardCondition(suit=Suit.SPADES, rank=Rank.QUEEN),
                points=13,
                trigger=ScoringTrigger.TRICK_WIN,
            ),
        ),
        max_turns=200,
        player_count=4,
        min_turns=52,
    )
```

Add imports at top of examples.py:
```python
from darwindeck.genome.schema import (
    # ... existing imports ...
    CardScoringRule, CardCondition, ScoringTrigger,
    WinComparison, TriggerMode, BreakingRule,
)
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_genome_completeness.py::TestMigratedGenomesComplete::test_hearts_genome_is_complete -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/examples.py tests/unit/test_genome_completeness.py
git commit -m "feat(examples): migrate Hearts genome to explicit card_scoring"
```

---

## Task 20: Migrate Simple Poker Genome (Add Hand Evaluation)

**Files:**
- Modify: `src/darwindeck/genome/examples.py`
- Modify: `tests/unit/test_genome_completeness.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_genome_completeness.py`:

```python
def test_simple_poker_genome_is_complete(self):
    """Simple Poker genome with hand_evaluation is complete."""
    from darwindeck.genome.examples import create_simple_poker_genome
    genome = create_simple_poker_genome()
    result = check_genome_completeness(genome)
    assert result.complete, f"Simple Poker genome incomplete: {result}"

    # Verify hand_evaluation is populated
    assert genome.hand_evaluation is not None
    assert len(genome.hand_evaluation.patterns) >= 10  # All poker hands
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_genome_completeness.py::TestMigratedGenomesComplete::test_simple_poker_genome_is_complete -v
```

Expected: FAIL

**Step 3: Write minimal implementation**

Modify `create_simple_poker_genome()` in `src/darwindeck/genome/examples.py`:

```python
def create_simple_poker_genome() -> GameGenome:
    """Create Simple Poker with explicit hand patterns."""
    from darwindeck.genome.schema import (
        HandEvaluation, HandEvaluationMethod, HandPattern,
    )

    # Standard poker hand patterns
    poker_patterns = (
        HandPattern(
            name="Royal Flush",
            rank_priority=100,
            required_count=5,
            same_suit_count=5,
            sequence_length=5,
            required_ranks=(Rank.TEN, Rank.JACK, Rank.QUEEN, Rank.KING, Rank.ACE),
        ),
        HandPattern(
            name="Straight Flush",
            rank_priority=90,
            required_count=5,
            same_suit_count=5,
            sequence_length=5,
        ),
        HandPattern(
            name="Four of a Kind",
            rank_priority=80,
            required_count=5,
            same_rank_groups=(4,),
        ),
        HandPattern(
            name="Full House",
            rank_priority=70,
            required_count=5,
            same_rank_groups=(3, 2),
        ),
        HandPattern(
            name="Flush",
            rank_priority=60,
            required_count=5,
            same_suit_count=5,
        ),
        HandPattern(
            name="Straight",
            rank_priority=50,
            required_count=5,
            sequence_length=5,
            sequence_wrap=True,
        ),
        HandPattern(
            name="Three of a Kind",
            rank_priority=40,
            required_count=5,
            same_rank_groups=(3,),
        ),
        HandPattern(
            name="Two Pair",
            rank_priority=30,
            required_count=5,
            same_rank_groups=(2, 2),
        ),
        HandPattern(
            name="One Pair",
            rank_priority=20,
            required_count=5,
            same_rank_groups=(2,),
        ),
        HandPattern(
            name="High Card",
            rank_priority=10,
            required_count=5,
        ),
    )

    return GameGenome(
        schema_version="1.0",
        genome_id="simple-poker",
        generation=0,
        setup=SetupRules(
            cards_per_player=5,
            initial_deck="standard_52",
            initial_discard_count=0,
            starting_chips=1000,
        ),
        turn_structure=TurnStructure(
            phases=[
                BettingPhase(
                    min_bet=10,
                    max_raises=3,
                    showdown_method=ShowdownMethod.HAND_EVALUATION,
                ),
            ],
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="best_hand",
                comparison=WinComparison.HIGHEST,
                required_hand_size=5,
            ),
        ],
        scoring_rules=[],
        # NEW: Explicit hand evaluation
        hand_evaluation=HandEvaluation(
            method=HandEvaluationMethod.PATTERN_MATCH,
            patterns=poker_patterns,
        ),
        max_turns=10,
        player_count=2,
    )
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_genome_completeness.py::TestMigratedGenomesComplete::test_simple_poker_genome_is_complete -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/examples.py tests/unit/test_genome_completeness.py
git commit -m "feat(examples): migrate Simple Poker genome to explicit hand_evaluation"
```

---

## Task 21: Migrate Blackjack Genome (Add Card Values)

**Files:**
- Modify: `src/darwindeck/genome/examples.py`
- Modify: `tests/unit/test_genome_completeness.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_genome_completeness.py`:

```python
def test_blackjack_genome_is_complete(self):
    """Blackjack genome with card values is complete."""
    from darwindeck.genome.examples import create_blackjack_genome
    genome = create_blackjack_genome()
    result = check_genome_completeness(genome)
    assert result.complete, f"Blackjack genome incomplete: {result}"

    # Verify hand_evaluation with card_values
    assert genome.hand_evaluation is not None
    assert len(genome.hand_evaluation.card_values) == 13  # All ranks
    assert genome.hand_evaluation.target_value == 21
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_genome_completeness.py::TestMigratedGenomesComplete::test_blackjack_genome_is_complete -v
```

Expected: FAIL

**Step 3: Write minimal implementation**

Modify `create_blackjack_genome()` in `src/darwindeck/genome/examples.py`:

```python
def create_blackjack_genome() -> GameGenome:
    """Create Blackjack/21 with explicit card values."""
    from darwindeck.genome.schema import (
        HandEvaluation, HandEvaluationMethod, CardValue,
    )

    # Standard Blackjack card values
    blackjack_values = (
        CardValue(rank=Rank.ACE, value=11, alternate_value=1),
        CardValue(rank=Rank.TWO, value=2),
        CardValue(rank=Rank.THREE, value=3),
        CardValue(rank=Rank.FOUR, value=4),
        CardValue(rank=Rank.FIVE, value=5),
        CardValue(rank=Rank.SIX, value=6),
        CardValue(rank=Rank.SEVEN, value=7),
        CardValue(rank=Rank.EIGHT, value=8),
        CardValue(rank=Rank.NINE, value=9),
        CardValue(rank=Rank.TEN, value=10),
        CardValue(rank=Rank.JACK, value=10),
        CardValue(rank=Rank.QUEEN, value=10),
        CardValue(rank=Rank.KING, value=10),
    )

    return GameGenome(
        schema_version="1.0",
        genome_id="blackjack",
        generation=0,
        setup=SetupRules(
            cards_per_player=2,
            initial_deck="standard_52",
            initial_discard_count=0,
            starting_chips=500,
        ),
        turn_structure=TurnStructure(
            phases=[
                BettingPhase(min_bet=25, max_raises=1),
                DrawPhase(
                    source=Location.DECK,
                    count=1,
                    mandatory=False,
                    condition=Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.LT,
                        value=5
                    )
                ),
            ]
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="high_score",
                threshold=21,
                comparison=WinComparison.HIGHEST,
            ),
        ],
        scoring_rules=[],
        # NEW: Explicit hand evaluation with card values
        hand_evaluation=HandEvaluation(
            method=HandEvaluationMethod.POINT_TOTAL,
            card_values=blackjack_values,
            target_value=21,
            bust_threshold=22,
        ),
        max_turns=20,
        player_count=2,
    )
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_genome_completeness.py::TestMigratedGenomesComplete::test_blackjack_genome_is_complete -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/examples.py tests/unit/test_genome_completeness.py
git commit -m "feat(examples): migrate Blackjack genome to explicit card_values"
```

---

## Task 22: Update check_genome_completeness for New Fields

**Files:**
- Modify: `tests/unit/test_genome_completeness.py`

**Step 1: Update completeness checker to recognize new fields**

Modify `check_genome_completeness()` in `tests/unit/test_genome_completeness.py`:

```python
def check_genome_completeness(genome: GameGenome) -> CompletenessResult:
    """Check if a genome is self-describing without implicit mechanics."""
    dependencies = []
    warnings = []

    win_types = {wc.type for wc in genome.win_conditions}

    has_trick_phase = any(isinstance(p, TrickPhase) for p in genome.turn_structure.phases)
    has_betting_phase = any(isinstance(p, BettingPhase) for p in genome.turn_structure.phases)
    has_claim_phase = any(isinstance(p, ClaimPhase) for p in genome.turn_structure.phases)
    has_play_to_tableau = any(
        isinstance(p, PlayPhase) and p.target == Location.TABLEAU
        for p in genome.turn_structure.phases
    )

    # Check 1: Score-based wins require explicit scoring
    score_win_types = {"high_score", "low_score", "first_to_score"}
    if win_types & score_win_types:
        # NEW: Check for card_scoring
        has_explicit_scoring = bool(genome.card_scoring) or bool(genome.scoring_rules)

        # NEW: Check for point_total hand evaluation (Blackjack)
        has_point_total = (
            genome.hand_evaluation is not None
            and genome.hand_evaluation.method == HandEvaluationMethod.POINT_TOTAL
        )

        if not has_explicit_scoring and not has_point_total:
            if has_trick_phase:
                dependencies.append(IncompleteDependency.HEARTS_SCORING)
                dependencies.append(IncompleteDependency.TRICK_WINNER_SCORING)
            else:
                dependencies.append(IncompleteDependency.SCORE_WIN_NO_SCORING)

    # Check 2: Threshold=21 only incomplete if no explicit card_values
    for wc in genome.win_conditions:
        if wc.type == "high_score" and wc.threshold == 21:
            has_explicit_values = (
                genome.hand_evaluation is not None
                and genome.hand_evaluation.card_values
            )
            if not has_explicit_values:
                dependencies.append(IncompleteDependency.THRESHOLD_21_BLACKJACK)
                dependencies.append(IncompleteDependency.BLACKJACK_VALUATION)

    # Check 3: best_hand only incomplete if no explicit patterns
    if "best_hand" in win_types:
        has_explicit_patterns = (
            genome.hand_evaluation is not None
            and genome.hand_evaluation.method == HandEvaluationMethod.PATTERN_MATCH
            and genome.hand_evaluation.patterns
        )
        if not has_explicit_patterns:
            dependencies.append(IncompleteDependency.POKER_HAND_RANKING)

    # Check 4: Capture wins require capture mechanic (unchanged)
    capture_win_types = {"capture_all", "most_captured"}
    if win_types & capture_win_types:
        if genome.setup.tableau_mode == TableauMode.NONE:
            if not has_play_to_tableau:
                dependencies.append(IncompleteDependency.CAPTURE_WIN_NO_CAPTURE)

    # Check 5: Betting without showdown (unchanged but improved)
    if has_betting_phase:
        has_showdown = (
            "best_hand" in win_types
            or genome.hand_evaluation is not None
            or genome.scoring_rules
        )
        if not has_showdown:
            dependencies.append(IncompleteDependency.BETTING_NO_SHOWDOWN)

    # Check 6: Claim phase - check for explicit rank_mode
    if has_claim_phase:
        claim_phase = next(p for p in genome.turn_structure.phases if isinstance(p, ClaimPhase))
        # NEW: Check if rank_mode is explicitly set (not relying on turn-number derivation)
        if not hasattr(claim_phase, 'rank_mode'):
            dependencies.append(IncompleteDependency.CLAIM_RANK_IMPLICIT)

    # Check 7: all_hands_empty with tricks
    if "all_hands_empty" in win_types and has_trick_phase:
        if not genome.card_scoring and not genome.scoring_rules:
            dependencies.append(IncompleteDependency.HEARTS_SCORING)

    return CompletenessResult(
        complete=len(dependencies) == 0,
        dependencies=dependencies,
        warnings=warnings,
    )
```

Add required import:
```python
from darwindeck.genome.schema import HandEvaluationMethod
```

**Step 2: Run all completeness tests**

```bash
uv run pytest tests/unit/test_genome_completeness.py -v
```

Expected: Hearts, Simple Poker, Blackjack now pass

**Step 3: Commit**

```bash
git add tests/unit/test_genome_completeness.py
git commit -m "fix(tests): update completeness checker to recognize new explicit fields"
```

---

## Task 23: Migrate Remaining Incomplete Genomes

**Files:**
- Modify: `src/darwindeck/genome/examples.py`
- Modify: `tests/unit/test_genome_completeness.py`

**Step 1: Add test for all seed genomes being complete**

Add to `tests/unit/test_genome_completeness.py`:

```python
class TestAllSeedGenomesComplete:
    """After migration, ALL seed genomes must be complete."""

    @pytest.mark.parametrize("genome", get_seed_genomes())
    def test_seed_genome_is_complete(self, genome: GameGenome):
        """Every seed genome is self-describing."""
        result = check_genome_completeness(genome)
        assert result.complete, f"{genome.genome_id}: {result}"
```

**Step 2: Run test to identify remaining incomplete genomes**

```bash
uv run pytest tests/unit/test_genome_completeness.py::TestAllSeedGenomesComplete -v
```

Expected: Several genomes will fail

**Step 3: Migrate each failing genome**

For each failing genome, add the appropriate explicit fields:

- **Spades**: Add `card_scoring` (similar to Hearts)
- **Draw Poker**: Add `hand_evaluation` with patterns (copy from Simple Poker)
- **Betting War**: Add `hand_evaluation` with method=CARD_COUNT
- **Go Fish**: Update to use explicit `card_scoring` with SET_COMPLETE trigger
- **Cheat**: Add `rank_mode=ClaimRankMode.SEQUENTIAL` to ClaimPhase

**Step 4: Run tests until all pass**

```bash
uv run pytest tests/unit/test_genome_completeness.py::TestAllSeedGenomesComplete -v
```

Expected: All 18 seed genomes pass

**Step 5: Commit**

```bash
git add src/darwindeck/genome/examples.py tests/unit/test_genome_completeness.py
git commit -m "feat(examples): migrate all seed genomes to explicit format"
```

---

## Task 24: Run Full Test Suite

**Files:**
- None (verification only)

**Step 1: Run all tests**

```bash
uv run pytest tests/ -v
```

Expected: All tests pass

**Step 2: Fix any regressions**

If any tests fail, fix them before proceeding.

**Step 3: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve test regressions from schema changes"
```

---

## Task 25: Final Verification and Summary

**Step 1: Verify completeness tests**

```bash
uv run pytest tests/unit/test_genome_completeness.py -v
```

Expected: All 18 seed genomes complete

**Step 2: Verify validator tests**

```bash
uv run pytest tests/unit/test_genome_validator.py -v
```

Expected: All validation rules working

**Step 3: Verify type tests**

```bash
uv run pytest tests/unit/test_self_describing_types.py -v
```

Expected: All new types working

**Step 4: Run evolution smoke test**

```bash
uv run python -m darwindeck.cli.evolve --generations 3 --population 10
```

Expected: Evolution completes without errors

---

## Summary

| Task | Component | Files |
|------|-----------|-------|
| 1 | ScoringTrigger enum | schema.py, test_self_describing_types.py |
| 2 | CardCondition dataclass | schema.py |
| 3 | CardScoringRule dataclass | schema.py |
| 4 | HandEvaluationMethod enum | schema.py |
| 5 | CardValue dataclass | schema.py |
| 6 | HandPattern dataclass | schema.py |
| 7 | HandEvaluation dataclass | schema.py |
| 8 | WinComparison, TriggerMode enums | schema.py |
| 9 | PassAction, DeckEmptyAction, TieBreaker enums | schema.py |
| 10 | GameRules dataclass | schema.py |
| 11 | ClaimRankMode, BreakingRule enums | schema.py |
| 12 | ShowdownMethod enum | schema.py |
| 13 | WinCondition new fields | schema.py |
| 14 | GameGenome new fields | schema.py |
| 15 | TrickPhase breaking_rule | schema.py |
| 16 | ClaimPhase rank_mode | schema.py |
| 17 | BettingPhase showdown_method | schema.py |
| 18 | GenomeValidator | validator.py, test_genome_validator.py |
| 19 | Migrate Hearts | examples.py |
| 20 | Migrate Simple Poker | examples.py |
| 21 | Migrate Blackjack | examples.py |
| 22 | Update completeness checker | test_genome_completeness.py |
| 23 | Migrate remaining genomes | examples.py |
| 24 | Full test suite | verification |
| 25 | Final verification | verification |

**Total: 25 tasks**
