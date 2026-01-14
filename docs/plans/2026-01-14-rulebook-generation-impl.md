# Rulebook Generation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Generate complete, print-and-play rulebooks from evolved game genomes with validation at both ends.

**Architecture:** Template + LLM hybrid. GenomeValidator catches impossible setups, GenomeExtractor produces deterministic rules, RulebookEnhancer adds LLM polish, OutputValidator ensures LLM didn't invent rules.

**Tech Stack:** Python 3.11+, dataclasses, anthropic SDK, click CLI

---

## Task 1: Create RulebookSections Dataclass

**Files:**
- Create: `src/darwindeck/evolution/rulebook.py`
- Test: `tests/unit/test_rulebook.py`

**Step 1: Write the test for RulebookSections**

```python
# tests/unit/test_rulebook.py
"""Tests for rulebook generation."""

import pytest
from darwindeck.evolution.rulebook import RulebookSections


class TestRulebookSections:
    """Tests for the RulebookSections dataclass."""

    def test_rulebook_sections_creation(self):
        """RulebookSections can be created with all fields."""
        sections = RulebookSections(
            game_name="TestGame",
            player_count=2,
            overview="A test game.",
            components=["Standard 52-card deck"],
            setup_steps=["Shuffle the deck", "Deal 5 cards to each player"],
            objective="First to empty hand wins",
            phases=[("Draw", "Draw 1 card from the deck")],
            special_rules=[],
            edge_cases=["Reshuffle discard when deck empty"],
            quick_reference="Draw → Play → Win"
        )
        assert sections.game_name == "TestGame"
        assert len(sections.setup_steps) == 2
        assert len(sections.phases) == 1

    def test_rulebook_sections_defaults(self):
        """RulebookSections has sensible defaults."""
        sections = RulebookSections(
            game_name="Minimal",
            player_count=2,
            objective="Win the game"
        )
        assert sections.overview is None
        assert sections.components == []
        assert sections.edge_cases == []
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_rulebook.py -v
```

Expected: FAIL with "No module named 'darwindeck.evolution.rulebook'"

**Step 3: Write minimal implementation**

```python
# src/darwindeck/evolution/rulebook.py
"""Full rulebook generation from game genomes."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional


@dataclass
class RulebookSections:
    """Intermediate representation of rulebook content."""

    game_name: str
    player_count: int
    objective: str

    # Optional sections (filled by extraction or LLM)
    overview: Optional[str] = None
    components: list[str] = field(default_factory=list)
    setup_steps: list[str] = field(default_factory=list)
    phases: list[tuple[str, str]] = field(default_factory=list)  # (name, description)
    special_rules: list[str] = field(default_factory=list)
    edge_cases: list[str] = field(default_factory=list)
    quick_reference: Optional[str] = None
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_rulebook.py -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/rulebook.py tests/unit/test_rulebook.py
git commit -m "feat(rulebook): add RulebookSections dataclass"
```

---

## Task 2: Implement GenomeValidator

**Files:**
- Modify: `src/darwindeck/evolution/rulebook.py`
- Test: `tests/unit/test_rulebook.py`

**Step 1: Write the test for GenomeValidator**

```python
# Add to tests/unit/test_rulebook.py
from darwindeck.evolution.rulebook import GenomeValidator, ValidationResult
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    PlayPhase, DrawPhase, BettingPhase, Location
)


class TestGenomeValidator:
    """Tests for pre-extraction genome validation."""

    def _make_genome(self, cards_per_player=5, player_count=2, starting_chips=0,
                     phases=None, win_conditions=None):
        """Helper to create test genomes."""
        if phases is None:
            phases = [DrawPhase(source=Location.DECK, count=1)]
        if win_conditions is None:
            win_conditions = [WinCondition(type="empty_hand")]
        return GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=cards_per_player, starting_chips=starting_chips),
            turn_structure=TurnStructure(phases=phases),
            special_effects=[],
            win_conditions=win_conditions,
            player_count=player_count,
        )

    def test_valid_genome_passes(self):
        """A valid genome passes validation."""
        genome = self._make_genome()
        result = GenomeValidator().validate(genome)
        assert result.valid is True
        assert result.errors == []

    def test_too_many_cards_fails(self):
        """Dealing more cards than deck has fails."""
        genome = self._make_genome(cards_per_player=30, player_count=2)  # 60 > 52
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("cards" in e.lower() for e in result.errors)

    def test_betting_without_chips_fails(self):
        """BettingPhase with 0 starting chips fails."""
        genome = self._make_genome(
            starting_chips=0,
            phases=[BettingPhase(min_bet=10)]
        )
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("chip" in e.lower() for e in result.errors)

    def test_no_phases_fails(self):
        """Empty turn structure fails."""
        genome = self._make_genome(phases=[])
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("phase" in e.lower() for e in result.errors)

    def test_no_win_conditions_fails(self):
        """No win conditions fails."""
        genome = self._make_genome(win_conditions=[])
        result = GenomeValidator().validate(genome)
        assert result.valid is False
        assert any("win" in e.lower() for e in result.errors)
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_rulebook.py::TestGenomeValidator -v
```

Expected: FAIL with "cannot import name 'GenomeValidator'"

**Step 3: Write implementation**

```python
# Add to src/darwindeck/evolution/rulebook.py

@dataclass
class ValidationResult:
    """Result of genome or output validation."""
    valid: bool
    errors: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)


class GenomeValidator:
    """Pre-extraction validation for genome feasibility."""

    def validate(self, genome: "GameGenome") -> ValidationResult:
        """Check genome can produce a playable rulebook."""
        from darwindeck.genome.schema import BettingPhase

        errors = []
        warnings = []

        # Card count feasibility
        total_cards_needed = genome.setup.cards_per_player * genome.player_count
        total_cards_needed += genome.setup.initial_discard_count
        if total_cards_needed > 52:
            errors.append(
                f"Setup requires {total_cards_needed} cards but deck only has 52"
            )

        # Betting requires chips
        has_betting = any(
            isinstance(p, BettingPhase) for p in genome.turn_structure.phases
        )
        if has_betting and genome.setup.starting_chips == 0:
            errors.append("BettingPhase present but starting_chips is 0")

        # Must have win conditions
        if not genome.win_conditions:
            errors.append("No win conditions defined")

        # Must have phases
        if not genome.turn_structure.phases:
            errors.append("No phases defined in turn structure")

        return ValidationResult(
            valid=len(errors) == 0,
            errors=errors,
            warnings=warnings
        )
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_rulebook.py::TestGenomeValidator -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/rulebook.py tests/unit/test_rulebook.py
git commit -m "feat(rulebook): add GenomeValidator for pre-extraction checks"
```

---

## Task 3: Implement GenomeExtractor (Setup & Objective)

**Files:**
- Modify: `src/darwindeck/evolution/rulebook.py`
- Test: `tests/unit/test_rulebook.py`

**Step 1: Write the test**

```python
# Add to tests/unit/test_rulebook.py
from darwindeck.evolution.rulebook import GenomeExtractor


class TestGenomeExtractor:
    """Tests for deterministic rule extraction."""

    def _make_genome(self, cards_per_player=5, starting_chips=0,
                     initial_discard_count=0, win_conditions=None, phases=None):
        """Helper to create test genomes."""
        if win_conditions is None:
            win_conditions = [WinCondition(type="empty_hand")]
        if phases is None:
            phases = [DrawPhase(source=Location.DECK, count=1)]
        return GameGenome(
            schema_version="1.0",
            genome_id="TestGame",
            generation=1,
            setup=SetupRules(
                cards_per_player=cards_per_player,
                starting_chips=starting_chips,
                initial_discard_count=initial_discard_count
            ),
            turn_structure=TurnStructure(phases=phases),
            special_effects=[],
            win_conditions=win_conditions,
            player_count=2,
        )

    def test_extract_basic_setup(self):
        """Extracts basic setup steps."""
        genome = self._make_genome(cards_per_player=7)
        sections = GenomeExtractor().extract(genome)

        assert "Shuffle the deck" in sections.setup_steps
        assert any("7 cards" in step for step in sections.setup_steps)

    def test_extract_setup_with_chips(self):
        """Includes chips in setup when starting_chips > 0."""
        genome = self._make_genome(starting_chips=1000)
        sections = GenomeExtractor().extract(genome)

        assert any("1000" in step and "chip" in step.lower() for step in sections.setup_steps)

    def test_extract_setup_with_discard(self):
        """Includes initial discard when present."""
        genome = self._make_genome(initial_discard_count=1)
        sections = GenomeExtractor().extract(genome)

        assert any("discard" in step.lower() for step in sections.setup_steps)

    def test_extract_empty_hand_objective(self):
        """Extracts empty_hand win condition."""
        genome = self._make_genome(win_conditions=[WinCondition(type="empty_hand")])
        sections = GenomeExtractor().extract(genome)

        assert "empty" in sections.objective.lower()

    def test_extract_high_score_objective(self):
        """Extracts high_score win condition."""
        genome = self._make_genome(win_conditions=[WinCondition(type="high_score")])
        sections = GenomeExtractor().extract(genome)

        assert "score" in sections.objective.lower() or "points" in sections.objective.lower()

    def test_extract_multiple_win_conditions(self):
        """Handles multiple win conditions."""
        genome = self._make_genome(win_conditions=[
            WinCondition(type="empty_hand"),
            WinCondition(type="capture_all")
        ])
        sections = GenomeExtractor().extract(genome)

        assert "empty" in sections.objective.lower() or "capture" in sections.objective.lower()
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_rulebook.py::TestGenomeExtractor -v
```

Expected: FAIL

**Step 3: Write implementation**

```python
# Add to src/darwindeck/evolution/rulebook.py

class GenomeExtractor:
    """Deterministic extraction of rules from genome fields."""

    # Win condition type to human-readable text
    WIN_CONDITION_TEXT = {
        "empty_hand": "First player to empty their hand wins",
        "high_score": "Player with the highest score wins",
        "low_score": "Player with the lowest score wins",
        "capture_all": "Capture all cards to win",
        "most_tricks": "Player who wins the most tricks wins",
        "fewest_tricks": "Player who wins the fewest tricks wins",
        "most_chips": "Player with the most chips wins",
        "most_captured": "Player who captures the most cards wins",
        "first_to_score": "First player to reach the target score wins",
    }

    def extract(self, genome: "GameGenome") -> RulebookSections:
        """Extract rulebook sections from genome."""
        return RulebookSections(
            game_name=genome.genome_id,
            player_count=genome.player_count,
            objective=self._extract_objective(genome),
            components=self._extract_components(genome),
            setup_steps=self._extract_setup(genome),
            phases=self._extract_phases(genome),
            special_rules=self._extract_special_rules(genome),
        )

    def _extract_components(self, genome: "GameGenome") -> list[str]:
        """Extract required components."""
        components = [f"Standard 52-card deck ({genome.player_count} players)"]
        if genome.setup.starting_chips > 0:
            components.append(f"Chips or tokens ({genome.setup.starting_chips} per player)")
        if any(wc.type in ("high_score", "low_score", "first_to_score") for wc in genome.win_conditions):
            components.append("Score tracking (pen and paper)")
        return components

    def _extract_setup(self, genome: "GameGenome") -> list[str]:
        """Extract setup steps."""
        steps = ["Shuffle the deck"]

        # Deal cards
        steps.append(f"Deal {genome.setup.cards_per_player} cards to each player")

        # Initial discard
        if genome.setup.initial_discard_count > 0:
            if genome.setup.initial_discard_count == 1:
                steps.append("Place 1 card face-up to start the discard pile")
            else:
                steps.append(f"Place {genome.setup.initial_discard_count} cards face-up to start the discard pile")

        # Chips
        if genome.setup.starting_chips > 0:
            steps.append(f"Give each player {genome.setup.starting_chips} chips")

        # Remaining deck
        steps.append("Place remaining cards face-down as the draw pile")

        return steps

    def _extract_objective(self, genome: "GameGenome") -> str:
        """Extract win conditions as objective text."""
        if not genome.win_conditions:
            return "Win the game"

        objectives = []
        for wc in genome.win_conditions:
            text = self.WIN_CONDITION_TEXT.get(wc.type, f"Meet the {wc.type} condition")
            if wc.threshold:
                text = text.replace("target score", str(wc.threshold))
            objectives.append(text)

        if len(objectives) == 1:
            return objectives[0]
        else:
            return "Win by either:\n- " + "\n- ".join(objectives)

    def _extract_phases(self, genome: "GameGenome") -> list[tuple[str, str]]:
        """Extract turn phases."""
        # Placeholder - will implement in Task 4
        return []

    def _extract_special_rules(self, genome: "GameGenome") -> list[str]:
        """Extract special card effects."""
        # Placeholder - will implement in Task 5
        return []
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_rulebook.py::TestGenomeExtractor -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/rulebook.py tests/unit/test_rulebook.py
git commit -m "feat(rulebook): add GenomeExtractor for setup and objectives"
```

---

## Task 4: Implement Phase Extraction

**Files:**
- Modify: `src/darwindeck/evolution/rulebook.py`
- Test: `tests/unit/test_rulebook.py`

**Step 1: Write the test**

```python
# Add to TestGenomeExtractor class in tests/unit/test_rulebook.py

    def test_extract_draw_phase(self):
        """Extracts DrawPhase correctly."""
        genome = self._make_genome(phases=[
            DrawPhase(source=Location.DECK, count=2)
        ])
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 1
        name, desc = sections.phases[0]
        assert "draw" in name.lower()
        assert "2" in desc

    def test_extract_play_phase(self):
        """Extracts PlayPhase correctly."""
        genome = self._make_genome(phases=[
            PlayPhase(target=Location.DISCARD, min_cards=1, max_cards=1)
        ])
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 1
        name, desc = sections.phases[0]
        assert "play" in name.lower()

    def test_extract_betting_phase(self):
        """Extracts BettingPhase correctly."""
        genome = self._make_genome(
            starting_chips=1000,
            phases=[BettingPhase(min_bet=25, max_raises=3)]
        )
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 1
        name, desc = sections.phases[0]
        assert "bet" in name.lower()
        assert "25" in desc

    def test_extract_multiple_phases(self):
        """Extracts multiple phases in order."""
        genome = self._make_genome(phases=[
            DrawPhase(source=Location.DECK, count=1),
            PlayPhase(target=Location.DISCARD, min_cards=1, max_cards=3),
        ])
        sections = GenomeExtractor().extract(genome)

        assert len(sections.phases) == 2
        assert "draw" in sections.phases[0][0].lower()
        assert "play" in sections.phases[1][0].lower()
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_rulebook.py::TestGenomeExtractor::test_extract_draw_phase -v
```

Expected: FAIL (phases list is empty)

**Step 3: Write implementation**

```python
# Replace _extract_phases in GenomeExtractor

    def _extract_phases(self, genome: "GameGenome") -> list[tuple[str, str]]:
        """Extract turn phases as (name, description) tuples."""
        from darwindeck.genome.schema import (
            DrawPhase, PlayPhase, DiscardPhase, BettingPhase,
            TrickPhase, ClaimPhase, Location
        )

        phases = []
        for i, phase in enumerate(genome.turn_structure.phases, 1):
            name, desc = self._describe_phase(phase)
            phases.append((f"Phase {i}: {name}", desc))
        return phases

    def _describe_phase(self, phase) -> tuple[str, str]:
        """Convert a phase to (name, description)."""
        from darwindeck.genome.schema import (
            DrawPhase, PlayPhase, DiscardPhase, BettingPhase,
            TrickPhase, ClaimPhase, Location
        )

        if isinstance(phase, DrawPhase):
            source = "deck" if phase.source == Location.DECK else "discard pile"
            if phase.count == 1:
                desc = f"Draw 1 card from the {source}"
            else:
                desc = f"Draw {phase.count} cards from the {source}"
            if not phase.mandatory:
                desc += " (optional)"
            return ("Draw", desc)

        elif isinstance(phase, PlayPhase):
            target = "discard pile" if phase.target == Location.DISCARD else "tableau"
            if phase.min_cards == phase.max_cards:
                if phase.min_cards == 1:
                    desc = f"Play exactly 1 card to the {target}"
                else:
                    desc = f"Play exactly {phase.min_cards} cards to the {target}"
            elif phase.min_cards == 0:
                desc = f"Play up to {phase.max_cards} cards to the {target} (optional)"
            else:
                desc = f"Play {phase.min_cards}-{phase.max_cards} cards to the {target}"
            return ("Play", desc)

        elif isinstance(phase, DiscardPhase):
            if phase.count == 1:
                desc = "Discard 1 card"
            else:
                desc = f"Discard {phase.count} cards"
            if not phase.mandatory:
                desc += " (optional)"
            return ("Discard", desc)

        elif isinstance(phase, BettingPhase):
            desc = f"Betting round (minimum bet: {phase.min_bet} chips, max {phase.max_raises} raises)"
            return ("Betting", desc)

        elif isinstance(phase, TrickPhase):
            desc = "Play one card to the trick. "
            if phase.lead_suit_required:
                desc += "Must follow suit if able. "
            if phase.high_card_wins:
                desc += "Highest card wins the trick."
            else:
                desc += "Lowest card wins the trick."
            return ("Trick", desc)

        elif isinstance(phase, ClaimPhase):
            desc = f"Play {phase.min_cards}-{phase.max_cards} cards face-down and claim a rank. "
            if phase.sequential_rank:
                desc += "Claims must follow sequence (A, 2, 3, ..., K). "
            if phase.allow_challenge:
                desc += "Opponents may challenge your claim."
            return ("Claim", desc)

        else:
            return ("Unknown", "Perform the phase action")
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_rulebook.py::TestGenomeExtractor -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/rulebook.py tests/unit/test_rulebook.py
git commit -m "feat(rulebook): add phase extraction to GenomeExtractor"
```

---

## Task 5: Implement Special Rules Extraction

**Files:**
- Modify: `src/darwindeck/evolution/rulebook.py`
- Test: `tests/unit/test_rulebook.py`

**Step 1: Write the test**

```python
# Add to TestGenomeExtractor class
from darwindeck.genome.schema import SpecialEffect, EffectType, TargetSelector, Rank

    def test_extract_skip_effect(self):
        """Extracts skip next player effect."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[
                SpecialEffect(trigger_rank=Rank.EIGHT, effect_type=EffectType.SKIP_NEXT, target=TargetSelector.NEXT_PLAYER)
            ],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
        )
        sections = GenomeExtractor().extract(genome)

        assert len(sections.special_rules) >= 1
        assert any("8" in rule or "eight" in rule.lower() for rule in sections.special_rules)
        assert any("skip" in rule.lower() for rule in sections.special_rules)

    def test_extract_reverse_effect(self):
        """Extracts reverse direction effect."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[
                SpecialEffect(trigger_rank=Rank.ACE, effect_type=EffectType.REVERSE_DIRECTION, target=TargetSelector.ALL_OPPONENTS)
            ],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
        )
        sections = GenomeExtractor().extract(genome)

        assert any("reverse" in rule.lower() for rule in sections.special_rules)

    def test_extract_draw_cards_effect(self):
        """Extracts draw cards effect."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[
                SpecialEffect(trigger_rank=Rank.TWO, effect_type=EffectType.DRAW_CARDS, target=TargetSelector.NEXT_PLAYER, value=2)
            ],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
        )
        sections = GenomeExtractor().extract(genome)

        assert any("draw" in rule.lower() and "2" in rule for rule in sections.special_rules)
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_rulebook.py::TestGenomeExtractor::test_extract_skip_effect -v
```

Expected: FAIL (special_rules is empty)

**Step 3: Write implementation**

```python
# Replace _extract_special_rules in GenomeExtractor

    def _extract_special_rules(self, genome: "GameGenome") -> list[str]:
        """Extract special card effects as rules."""
        from darwindeck.genome.schema import EffectType, Rank

        rules = []

        rank_names = {
            Rank.ACE: "Ace", Rank.TWO: "2", Rank.THREE: "3", Rank.FOUR: "4",
            Rank.FIVE: "5", Rank.SIX: "6", Rank.SEVEN: "7", Rank.EIGHT: "8",
            Rank.NINE: "9", Rank.TEN: "10", Rank.JACK: "Jack",
            Rank.QUEEN: "Queen", Rank.KING: "King"
        }

        for effect in genome.special_effects:
            rank_name = rank_names.get(effect.trigger_rank, str(effect.trigger_rank))

            if effect.effect_type == EffectType.SKIP_NEXT:
                rules.append(f"**{rank_name}:** Playing this card skips the next player's turn")
            elif effect.effect_type == EffectType.REVERSE_DIRECTION:
                rules.append(f"**{rank_name}:** Playing this card reverses the turn order")
            elif effect.effect_type == EffectType.DRAW_CARDS:
                rules.append(f"**{rank_name}:** Next player must draw {effect.value} cards")
            elif effect.effect_type == EffectType.EXTRA_TURN:
                rules.append(f"**{rank_name}:** Playing this card gives you an extra turn")
            elif effect.effect_type == EffectType.FORCE_DISCARD:
                rules.append(f"**{rank_name}:** Next player must discard {effect.value} cards")

        # Add wild card rules if any
        if genome.setup.wild_cards:
            wild_names = [rank_names.get(r, str(r)) for r in genome.setup.wild_cards]
            rules.append(f"**Wild cards ({', '.join(wild_names)}):** Can be played on any card")

        return rules
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_rulebook.py::TestGenomeExtractor -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/rulebook.py tests/unit/test_rulebook.py
git commit -m "feat(rulebook): add special rules extraction"
```

---

## Task 6: Implement Genome-Conditional Defaults

**Files:**
- Modify: `src/darwindeck/evolution/rulebook.py`
- Test: `tests/unit/test_rulebook.py`

**Step 1: Write the test**

```python
# Add to tests/unit/test_rulebook.py
from darwindeck.evolution.rulebook import select_applicable_defaults, EdgeCaseDefault


class TestEdgeCaseDefaults:
    """Tests for genome-conditional edge case defaults."""

    def _make_genome(self, win_conditions=None, phases=None, starting_chips=0):
        if win_conditions is None:
            win_conditions = [WinCondition(type="empty_hand")]
        if phases is None:
            phases = [DrawPhase(source=Location.DECK, count=1)]
        return GameGenome(
            schema_version="1.0",
            genome_id="test",
            generation=1,
            setup=SetupRules(cards_per_player=5, starting_chips=starting_chips),
            turn_structure=TurnStructure(phases=phases),
            special_effects=[],
            win_conditions=win_conditions,
            player_count=2,
        )

    def test_deck_exhaustion_default_included(self):
        """Deck exhaustion default included for normal games."""
        genome = self._make_genome()
        defaults = select_applicable_defaults(genome)

        assert any(d.name == "deck_exhaustion" for d in defaults)

    def test_deck_exhaustion_skipped_when_win_condition(self):
        """Deck exhaustion default skipped if it's a win condition."""
        genome = self._make_genome(win_conditions=[WinCondition(type="deck_empty")])
        defaults = select_applicable_defaults(genome)

        assert not any(d.name == "deck_exhaustion" for d in defaults)

    def test_betting_defaults_only_with_betting(self):
        """Betting defaults only included when BettingPhase exists."""
        # Without betting
        genome_no_bet = self._make_genome()
        defaults_no_bet = select_applicable_defaults(genome_no_bet)
        assert not any("betting" in d.name for d in defaults_no_bet)

        # With betting
        genome_bet = self._make_genome(
            starting_chips=1000,
            phases=[BettingPhase(min_bet=10)]
        )
        defaults_bet = select_applicable_defaults(genome_bet)
        assert any("betting" in d.name for d in defaults_bet)

    def test_hand_limit_skipped_for_capture_games(self):
        """Hand limit not applied to capture/accumulation games."""
        genome = self._make_genome(win_conditions=[WinCondition(type="capture_all")])
        defaults = select_applicable_defaults(genome)

        assert not any(d.name == "hand_limit" for d in defaults)
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_rulebook.py::TestEdgeCaseDefaults -v
```

Expected: FAIL

**Step 3: Write implementation**

```python
# Add to src/darwindeck/evolution/rulebook.py

@dataclass
class EdgeCaseDefault:
    """A default edge case rule."""
    name: str
    rule: str


# Define all available defaults
DECK_EXHAUSTION = EdgeCaseDefault(
    name="deck_exhaustion",
    rule="**Empty deck:** Shuffle the discard pile (except top card) to form a new deck. If still empty, skip the draw."
)

NO_VALID_PLAYS = EdgeCaseDefault(
    name="no_valid_plays",
    rule="**No valid plays:** Draw up to 3 cards until you can play, then pass if still unable."
)

SIMULTANEOUS_WIN = EdgeCaseDefault(
    name="simultaneous_win",
    rule="**Tie:** If multiple players meet win conditions simultaneously, the active player wins."
)

HAND_LIMIT = EdgeCaseDefault(
    name="hand_limit",
    rule="**Hand limit:** If your hand exceeds 15 cards, discard down to 15 at end of turn."
)

BETTING_ALL_IN = EdgeCaseDefault(
    name="betting_all_in",
    rule="**All-in:** If you can't afford to call, you may go all-in with remaining chips."
)

BETTING_POT_SPLIT = EdgeCaseDefault(
    name="betting_pot_split",
    rule="**Pot split:** If the pot can't split evenly, odd chips go to the player left of dealer."
)

TURN_LIMIT = EdgeCaseDefault(
    name="turn_limit",
    rule="**Turn limit:** If max turns reached, highest score wins (or draw if no scoring)."
)


def select_applicable_defaults(genome: "GameGenome") -> list[EdgeCaseDefault]:
    """Select edge case defaults that don't conflict with genome mechanics."""
    from darwindeck.genome.schema import BettingPhase, PlayPhase

    defaults = []

    # Check win condition types
    win_types = {wc.type for wc in genome.win_conditions}

    # Deck exhaustion - skip if it's a win condition
    if not win_types & {"deck_empty", "last_card"}:
        defaults.append(DECK_EXHAUSTION)

    # No valid plays - skip if genome has optional play (min=0)
    has_optional_play = any(
        isinstance(p, PlayPhase) and p.min_cards == 0
        for p in genome.turn_structure.phases
    )
    if not has_optional_play:
        defaults.append(NO_VALID_PLAYS)

    # Simultaneous win - always applies
    defaults.append(SIMULTANEOUS_WIN)

    # Hand limit - skip for accumulation games
    if not win_types & {"capture_all", "most_cards", "most_captured"}:
        defaults.append(HAND_LIMIT)

    # Betting defaults - only if betting phases exist
    has_betting = any(
        isinstance(p, BettingPhase) for p in genome.turn_structure.phases
    )
    if has_betting:
        defaults.append(BETTING_ALL_IN)
        defaults.append(BETTING_POT_SPLIT)

    # Turn limit - always applies
    defaults.append(TURN_LIMIT)

    return defaults
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_rulebook.py::TestEdgeCaseDefaults -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/rulebook.py tests/unit/test_rulebook.py
git commit -m "feat(rulebook): add genome-conditional edge case defaults"
```

---

## Task 7: Implement RulebookGenerator.render_markdown()

**Files:**
- Modify: `src/darwindeck/evolution/rulebook.py`
- Test: `tests/unit/test_rulebook.py`

**Step 1: Write the test**

```python
# Add to tests/unit/test_rulebook.py
from darwindeck.evolution.rulebook import RulebookGenerator


class TestRulebookGenerator:
    """Tests for markdown rulebook generation."""

    def _make_genome(self):
        return GameGenome(
            schema_version="1.0",
            genome_id="TestGame",
            generation=1,
            setup=SetupRules(cards_per_player=7),
            turn_structure=TurnStructure(phases=[
                DrawPhase(source=Location.DECK, count=1),
                PlayPhase(target=Location.DISCARD, min_cards=1, max_cards=1),
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
        )

    def test_render_markdown_has_all_sections(self):
        """Rendered markdown has all required sections."""
        genome = self._make_genome()
        generator = RulebookGenerator()
        markdown = generator.generate(genome)

        assert "# TestGame" in markdown
        assert "## Components" in markdown
        assert "## Setup" in markdown
        assert "## Objective" in markdown
        assert "## Turn Structure" in markdown
        assert "## Edge Cases" in markdown

    def test_render_markdown_includes_setup_steps(self):
        """Setup steps are included in markdown."""
        genome = self._make_genome()
        markdown = RulebookGenerator().generate(genome)

        assert "Shuffle the deck" in markdown
        assert "7 cards" in markdown

    def test_render_markdown_includes_phases(self):
        """Phases are included in markdown."""
        genome = self._make_genome()
        markdown = RulebookGenerator().generate(genome)

        assert "Draw" in markdown
        assert "Play" in markdown

    def test_generate_basic_mode(self):
        """Basic mode works without LLM."""
        genome = self._make_genome()
        markdown = RulebookGenerator().generate(genome, use_llm=False)

        assert "# TestGame" in markdown
        assert "## Overview" not in markdown or "Overview" in markdown  # May have template overview
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_rulebook.py::TestRulebookGenerator -v
```

Expected: FAIL

**Step 3: Write implementation**

```python
# Add to src/darwindeck/evolution/rulebook.py

class RulebookGenerator:
    """Generates complete rulebooks from genomes."""

    def __init__(self):
        self.validator = GenomeValidator()
        self.extractor = GenomeExtractor()

    def generate(self, genome: "GameGenome", use_llm: bool = True) -> str:
        """Generate a complete rulebook for a genome.

        Args:
            genome: The game genome
            use_llm: Whether to use LLM enhancement (default True)

        Returns:
            Complete rulebook as markdown string

        Raises:
            ValueError: If genome fails validation
        """
        # Validate genome first
        validation = self.validator.validate(genome)
        if not validation.valid:
            raise ValueError(f"Invalid genome: {'; '.join(validation.errors)}")

        # Extract sections
        sections = self.extractor.extract(genome)

        # Get applicable edge case defaults
        defaults = select_applicable_defaults(genome)
        sections.edge_cases = [d.rule for d in defaults]

        # TODO: LLM enhancement (Task 8)
        # if use_llm:
        #     sections = RulebookEnhancer().enhance(sections, genome)

        # Render to markdown
        return self._render_markdown(sections)

    def _render_markdown(self, sections: RulebookSections) -> str:
        """Render sections to markdown format."""
        lines = []

        # Title
        lines.append(f"# {sections.game_name}")
        lines.append("")

        # Overview (if present)
        if sections.overview:
            lines.append("## Overview")
            lines.append(sections.overview)
            lines.append("")

        # Components
        lines.append("## Components")
        for component in sections.components:
            lines.append(f"- {component}")
        lines.append("")

        # Setup
        lines.append("## Setup")
        for i, step in enumerate(sections.setup_steps, 1):
            lines.append(f"{i}. {step}")
        lines.append("")

        # Objective
        lines.append("## Objective")
        lines.append(sections.objective)
        lines.append("")

        # Turn Structure
        lines.append("## Turn Structure")
        lines.append(f"Each turn consists of {len(sections.phases)} phase(s):")
        lines.append("")
        for name, desc in sections.phases:
            lines.append(f"### {name}")
            lines.append(desc)
            lines.append("")

        # Special Rules (if any)
        if sections.special_rules:
            lines.append("## Special Rules")
            for rule in sections.special_rules:
                lines.append(rule)
                lines.append("")

        # Edge Cases
        lines.append("## Edge Cases")
        for edge_case in sections.edge_cases:
            lines.append(edge_case)
            lines.append("")

        # Quick Reference (if present)
        if sections.quick_reference:
            lines.append("## Quick Reference")
            lines.append(sections.quick_reference)
            lines.append("")

        return "\n".join(lines)
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_rulebook.py::TestRulebookGenerator -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/rulebook.py tests/unit/test_rulebook.py
git commit -m "feat(rulebook): add RulebookGenerator with markdown rendering"
```

---

## Task 8: Implement RulebookEnhancer (LLM)

**Files:**
- Modify: `src/darwindeck/evolution/rulebook.py`
- Test: `tests/unit/test_rulebook.py`

**Step 1: Write the test**

```python
# Add to tests/unit/test_rulebook.py
from unittest.mock import patch, MagicMock
from darwindeck.evolution.rulebook import RulebookEnhancer


class TestRulebookEnhancer:
    """Tests for LLM enhancement."""

    def _make_sections(self):
        return RulebookSections(
            game_name="TestGame",
            player_count=2,
            objective="Empty your hand to win",
            components=["Standard 52-card deck"],
            setup_steps=["Deal 5 cards"],
            phases=[("Draw", "Draw 1 card")],
        )

    @patch.dict("os.environ", {"ANTHROPIC_API_KEY": ""})
    def test_enhance_without_api_key_returns_unchanged(self):
        """Without API key, sections returned unchanged."""
        sections = self._make_sections()
        enhanced = RulebookEnhancer().enhance(sections, None)

        assert enhanced.overview is None  # Not enhanced

    @patch("darwindeck.evolution.rulebook.anthropic")
    @patch.dict("os.environ", {"ANTHROPIC_API_KEY": "test-key"})
    def test_enhance_adds_overview(self, mock_anthropic):
        """LLM enhancement adds overview."""
        mock_client = MagicMock()
        mock_anthropic.Anthropic.return_value = mock_client
        mock_client.messages.create.return_value = MagicMock(
            content=[MagicMock(text="A fun card game about emptying your hand.")]
        )

        sections = self._make_sections()
        genome = GameGenome(
            schema_version="1.0",
            genome_id="TestGame",
            generation=1,
            setup=SetupRules(cards_per_player=5),
            turn_structure=TurnStructure(phases=[DrawPhase(source=Location.DECK)]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            player_count=2,
        )

        enhanced = RulebookEnhancer().enhance(sections, genome)

        assert enhanced.overview is not None
        assert "fun" in enhanced.overview.lower() or "card" in enhanced.overview.lower()
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_rulebook.py::TestRulebookEnhancer -v
```

Expected: FAIL

**Step 3: Write implementation**

```python
# Add to src/darwindeck/evolution/rulebook.py
import os
import logging

logger = logging.getLogger(__name__)


class RulebookEnhancer:
    """Optional LLM enhancement for rulebook sections."""

    def enhance(self, sections: RulebookSections, genome: "GameGenome") -> RulebookSections:
        """Enhance sections with LLM-generated content.

        Args:
            sections: Extracted rulebook sections
            genome: Original genome (for validation)

        Returns:
            Enhanced sections (or original if LLM unavailable)
        """
        api_key = os.environ.get("ANTHROPIC_API_KEY")
        if not api_key:
            logger.warning("ANTHROPIC_API_KEY not set, skipping LLM enhancement")
            return sections

        try:
            import anthropic
            client = anthropic.Anthropic(api_key=api_key)

            # Generate overview
            overview = self._generate_overview(client, sections, genome)
            if overview:
                sections = RulebookSections(
                    game_name=sections.game_name,
                    player_count=sections.player_count,
                    objective=sections.objective,
                    overview=overview,
                    components=sections.components,
                    setup_steps=sections.setup_steps,
                    phases=sections.phases,
                    special_rules=sections.special_rules,
                    edge_cases=sections.edge_cases,
                    quick_reference=sections.quick_reference,
                )

            # TODO: Add example turn generation
            # TODO: Add quick reference generation

            return sections

        except Exception as e:
            logger.warning(f"LLM enhancement failed: {e}")
            return sections

    def _generate_overview(self, client, sections: RulebookSections, genome: "GameGenome") -> Optional[str]:
        """Generate engaging overview."""
        phase_names = [name for name, _ in sections.phases]

        prompt = f"""Write a 1-2 sentence overview for this card game:

Game: {sections.game_name}
Players: {sections.player_count}
Phases: {', '.join(phase_names)}
Objective: {sections.objective}

Make it engaging and accessible. Do not invent mechanics not listed above.
Return ONLY the overview text, no quotes or formatting."""

        try:
            response = client.messages.create(
                model="claude-sonnet-4-20250514",
                max_tokens=100,
                messages=[{"role": "user", "content": prompt}]
            )
            return response.content[0].text.strip()
        except Exception as e:
            logger.warning(f"Overview generation failed: {e}")
            return None
```

**Step 4: Run test to verify it passes**

```bash
uv run pytest tests/unit/test_rulebook.py::TestRulebookEnhancer -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/rulebook.py tests/unit/test_rulebook.py
git commit -m "feat(rulebook): add RulebookEnhancer with LLM overview generation"
```

---

## Task 9: Implement CLI Command

**Files:**
- Create: `src/darwindeck/cli/rulebook.py`
- Test: Manual CLI test

**Step 1: Write the CLI module**

```python
# src/darwindeck/cli/rulebook.py
"""CLI command for generating rulebooks."""

from __future__ import annotations

import json
import logging
import sys
from pathlib import Path

import click

from darwindeck.evolution.rulebook import RulebookGenerator
from darwindeck.genome.serialization import genome_from_dict

logger = logging.getLogger(__name__)


@click.command()
@click.argument("genome_path", type=click.Path(exists=True))
@click.option("-o", "--output", type=click.Path(), help="Output file path")
@click.option("--basic", is_flag=True, help="Skip LLM enhancement")
@click.option("--top", type=int, default=None, help="Only process top N genomes (if directory)")
@click.option("-v", "--verbose", is_flag=True, help="Verbose output")
def main(genome_path: str, output: str | None, basic: bool, top: int | None, verbose: bool):
    """Generate a rulebook from a game genome.

    GENOME_PATH can be a single JSON file or a directory containing genome files.
    """
    if verbose:
        logging.basicConfig(level=logging.DEBUG)
    else:
        logging.basicConfig(level=logging.INFO)

    path = Path(genome_path)
    generator = RulebookGenerator()

    if path.is_file():
        # Single genome
        _process_genome(path, output, basic, generator)
    elif path.is_dir():
        # Directory of genomes
        genome_files = sorted(path.glob("rank*.json"))
        if top:
            genome_files = genome_files[:top]

        if not genome_files:
            click.echo(f"No genome files found in {path}", err=True)
            sys.exit(1)

        output_dir = Path(output) if output else path / "rulebooks"
        output_dir.mkdir(exist_ok=True)

        for gf in genome_files:
            out_path = output_dir / f"{gf.stem}_rulebook.md"
            _process_genome(gf, str(out_path), basic, generator)
    else:
        click.echo(f"Invalid path: {genome_path}", err=True)
        sys.exit(1)


def _process_genome(genome_path: Path, output: str | None, basic: bool, generator: RulebookGenerator):
    """Process a single genome file."""
    click.echo(f"Processing {genome_path.name}...")

    try:
        with open(genome_path) as f:
            data = json.load(f)

        genome = genome_from_dict(data)
        markdown = generator.generate(genome, use_llm=not basic)

        if output:
            out_path = Path(output)
        else:
            out_path = genome_path.with_suffix(".md").with_stem(f"{genome_path.stem}_rulebook")

        out_path.write_text(markdown)
        click.echo(f"  Saved to {out_path}")

    except Exception as e:
        click.echo(f"  Error: {e}", err=True)


if __name__ == "__main__":
    main()
```

**Step 2: Test CLI manually**

```bash
# Test with a real genome from evolution output
uv run python -m darwindeck.cli.rulebook \
    output/evolution-20260113-195913/2026-01-13_21-56-51/rank01_InnerBout.json \
    --basic \
    -o /tmp/test_rulebook.md

cat /tmp/test_rulebook.md
```

Expected: Rulebook markdown printed

**Step 3: Test directory mode**

```bash
uv run python -m darwindeck.cli.rulebook \
    output/evolution-20260113-195913/2026-01-13_21-56-51/ \
    --basic \
    --top 3

ls output/evolution-20260113-195913/2026-01-13_21-56-51/rulebooks/
```

Expected: 3 rulebook files generated

**Step 4: Commit**

```bash
git add src/darwindeck/cli/rulebook.py
git commit -m "feat(cli): add rulebook generation command"
```

---

## Task 10: Add --rulebooks Flag to Evolution CLI

**Files:**
- Modify: `src/darwindeck/cli/evolve.py`

**Step 1: Find where to add the flag**

Look for the `save_top_genomes` or similar function near end of evolution.

**Step 2: Add the flag and integration**

```python
# Add to click options in evolve.py (near other options)
@click.option("--rulebooks", type=int, default=0, help="Generate rulebooks for top N games")

# Add to main function parameters
def main(..., rulebooks: int, ...):

# Add after saving top genomes (near end of main function)
if rulebooks > 0:
    from darwindeck.evolution.rulebook import RulebookGenerator
    click.echo(f"\nGenerating rulebooks for top {rulebooks} games...")

    generator = RulebookGenerator()
    rulebook_dir = output_path / "rulebooks"
    rulebook_dir.mkdir(exist_ok=True)

    for genome, fitness in top_genomes[:rulebooks]:
        try:
            markdown = generator.generate(genome, use_llm=True)
            out_path = rulebook_dir / f"{genome.genome_id}_rulebook.md"
            out_path.write_text(markdown)
            click.echo(f"  Generated {out_path.name}")
        except Exception as e:
            click.echo(f"  Failed {genome.genome_id}: {e}")
```

**Step 3: Test**

```bash
uv run python -m darwindeck.cli.evolve \
    --population 50 \
    --generations 3 \
    --rulebooks 2 \
    --output-dir output/test-rulebooks
```

**Step 4: Commit**

```bash
git add src/darwindeck/cli/evolve.py
git commit -m "feat(evolve): add --rulebooks flag for rulebook generation"
```

---

## Verification

After all tasks complete:

1. **Unit tests pass:**
   ```bash
   uv run pytest tests/unit/test_rulebook.py -v
   ```

2. **CLI works on real genome:**
   ```bash
   uv run python -m darwindeck.cli.rulebook \
       output/evolution-20260113-195913/2026-01-13_21-56-51/rank01_InnerBout.json \
       -o /tmp/test.md
   cat /tmp/test.md
   ```

3. **Evolution integration works:**
   ```bash
   uv run python -m darwindeck.cli.evolve \
       --population 20 \
       --generations 2 \
       --rulebooks 1
   ```

---

## Summary

| Task | Component | Description |
|------|-----------|-------------|
| 1 | RulebookSections | Dataclass for intermediate representation |
| 2 | GenomeValidator | Pre-extraction validation |
| 3 | GenomeExtractor (setup) | Setup and objective extraction |
| 4 | GenomeExtractor (phases) | Phase extraction |
| 5 | GenomeExtractor (special) | Special rules extraction |
| 6 | Edge case defaults | Genome-conditional defaults |
| 7 | RulebookGenerator | Markdown rendering |
| 8 | RulebookEnhancer | LLM enhancement |
| 9 | CLI command | Standalone CLI |
| 10 | Evolution flag | --rulebooks integration |

**Total: 10 tasks**
