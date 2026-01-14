# Full Rulebook Generation Design

**Date:** 2026-01-14
**Status:** Approved (revised after multi-agent review)
**Goal:** Generate complete, print-and-play rulebooks from evolved game genomes

## Overview

Expand the current 2-3 sentence game descriptions into full rulebooks suitable for physical playtesting with a standard 52-card deck. Uses a template + LLM hybrid approach: deterministic extraction for accuracy, LLM for readability.

## Requirements

- **Use case:** Print & play - complete, unambiguous rules humans can follow
- **Edge cases:** Standard defaults for common situations (deck exhaustion, no valid plays, ties)
- **Output format:** Markdown
- **Integration:** Both standalone CLI and evolution output option

## Architecture

```
src/darwindeck/
├── evolution/
│   ├── describe.py          # Existing 2-3 sentence descriptions
│   └── rulebook.py          # NEW: Full rulebook generation
├── cli/
│   └── rulebook.py          # NEW: Standalone CLI command
```

### Core Components

1. **`GenomeValidator`** - Pre-extraction validation (NEW - addresses multi-agent review)
2. **`GenomeExtractor`** - Deterministic extraction of rules from genome fields
3. **`RulebookGenerator`** - Main class that orchestrates generation
4. **`RulebookEnhancer`** - Optional LLM pass for prose polish and examples
5. **`OutputValidator`** - Post-generation validation of LLM output (NEW - addresses multi-agent review)

### Data Flow

```
GameGenome
    → GenomeValidator.validate()           # NEW: Catch impossible setups
    → GenomeExtractor.extract()
    → RulebookSections (structured data)
    → RulebookGenerator.render_markdown()
    → Basic rulebook (no LLM)
    → RulebookEnhancer.enhance() [optional]
    → OutputValidator.validate()           # NEW: Verify LLM didn't invent rules
    → Final polished rulebook
```

Key benefits:
- Works without API key (basic mode)
- LLM only adds value, never invents rules
- **Validation at both ends ensures correctness**
- Easy to test extraction logic independently

## Rulebook Structure

```markdown
# [Game Name]

## Overview
[1-2 sentences: what kind of game, player count, approximate length]

## Components
- Standard 52-card deck
- [Chips if starting_chips > 0]
- [Score tracking if scoring_rules exist]

## Setup
1. Shuffle the deck
2. Deal [N] cards to each player
3. [Place X cards face-up in tableau if initial_discard_count > 0]
4. [Give each player N chips if betting]

## Objective
[Win conditions in plain English]

## Turn Structure
Each turn consists of [N] phases:

### Phase 1: [Draw/Play/Discard/Bet/Claim]
[What the player does, constraints, options]

## Special Rules
[Card effects, valid play conditions]

## Edge Cases
[Standard rulings for common situations]

## Quick Reference
[One-line summary of each phase]
```

## Pre-Extraction Validation (NEW)

The `GenomeValidator` catches impossible or problematic configurations before extraction:

### Feasibility Checks

```python
class GenomeValidator:
    def validate(self, genome: GameGenome) -> ValidationResult:
        errors = []
        warnings = []

        # Card count feasibility
        total_cards_needed = genome.setup.cards_per_player * genome.player_count
        total_cards_needed += genome.setup.initial_discard_count
        if total_cards_needed > 52:
            errors.append(f"Setup requires {total_cards_needed} cards but deck only has 52")

        # Betting requires chips
        has_betting = any(isinstance(p, BettingPhase) for p in genome.turn_structure.phases)
        if has_betting and genome.setup.starting_chips == 0:
            errors.append("BettingPhase present but starting_chips is 0")

        # Win condition coherence
        if not genome.win_conditions:
            errors.append("No win conditions defined")

        # Phase sequence sanity
        if not genome.turn_structure.phases:
            errors.append("No phases defined in turn structure")

        return ValidationResult(
            valid=len(errors) == 0,
            errors=errors,
            warnings=warnings
        )
```

### Validation Outcomes

| Result | Action |
|--------|--------|
| **Valid** | Proceed to extraction |
| **Warnings** | Proceed but include warnings in rulebook |
| **Errors** | Abort with clear error message |

## Genome-to-Rules Extraction

### Setup Extraction

| Genome Field | Rulebook Output |
|--------------|-----------------|
| `setup.cards_per_player` | "Deal {n} cards to each player" |
| `setup.initial_discard_count` | "Place {n} cards face-up to start the discard pile" |
| `setup.starting_chips` | "Give each player {n} chips" (omit if 0) |

### Phase Extraction

| Phase Type | Rulebook Output |
|------------|-----------------|
| `DrawPhase(source=DECK, count=1)` | "Draw 1 card from the deck" |
| `DrawPhase(source=DISCARD, count=2)` | "Draw 2 cards from the discard pile" |
| `PlayPhase(min=1, max=1, target=DISCARD)` | "Play exactly 1 card to the discard pile" |
| `PlayPhase(min=0, max=3, target=TABLEAU)` | "Play up to 3 cards to the tableau" |
| `DiscardPhase(count=1)` | "Discard 1 card" |
| `BettingPhase(min_bet=10)` | "Betting round (minimum bet: 10 chips)" |
| `ClaimPhase(rank_order=SEQUENTIAL)` | "Claim cards by declaring rank (must follow sequence)" |

### Win Condition Extraction

| Win Type | Rulebook Output |
|----------|-----------------|
| `empty_hand` | "First player to empty their hand wins" |
| `high_score` | "Player with the highest score wins" |
| `capture_all` | "Capture all cards to win" |
| `most_tricks` | "Player who wins the most tricks wins" |
| `most_chips` | "Player with the most chips wins" |

### Special Effects Extraction

| Effect | Rulebook Output |
|--------|-----------------|
| `SkipNext` | "Playing this card skips the next player" |
| `Reverse` | "Playing this card reverses turn order" |
| `DrawN(2)` | "Next player must draw 2 cards" |
| `Wild` | "This card can be played on anything" |

## LLM Enhancement

Three targeted LLM calls (~300 tokens total per rulebook):

### 1. Overview Generation (~50 tokens)

**Input:** Game name, player count, phase types, win conditions
**Output:** 1-2 engaging sentences summarizing the game's feel

### 2. Example Turn (~150 tokens)

**Input:** Phase sequence, valid play rules, special effects
**Output:** A concrete example showing one complete turn

### 3. Edge Case Reasoning (~100 tokens)

**Input:** Win conditions, phase types, game mechanics
**Output:** Rulings for 3-5 common edge cases

**Constraint:** Prompt explicitly states: "Do not invent rules. Only clarify how existing rules handle edge cases."

### LLM Output Validation (NEW)

The `OutputValidator` verifies LLM didn't invent rules not present in the genome:

```python
class OutputValidator:
    def validate(self, genome: GameGenome, llm_output: str, section: str) -> ValidationResult:
        """Check that LLM output doesn't contain invented rules."""
        errors = []

        # Extract key terms that MUST come from genome
        genome_phases = {type(p).__name__ for p in genome.turn_structure.phases}
        genome_win_types = {wc.type for wc in genome.win_conditions}
        genome_effects = {e.effect_type for e in genome.special_effects}

        # Check for invented phases
        phase_keywords = ["draw", "play", "discard", "bet", "claim", "trick"]
        for keyword in phase_keywords:
            if keyword in llm_output.lower():
                # Verify this phase type exists in genome
                if not any(keyword in p.lower() for p in genome_phases):
                    errors.append(f"LLM mentioned '{keyword}' but genome has no such phase")

        # Check for invented win conditions
        if "win" in llm_output.lower() and section == "example":
            # Example turns shouldn't declare winners
            errors.append("Example turn should not determine a winner")

        return ValidationResult(
            valid=len(errors) == 0,
            errors=errors,
            fallback_to_basic=True  # Use template version if validation fails
        )
```

**On validation failure:** Fall back to basic (non-LLM) version of that section, log warning.

### Basic Mode (No LLM)

- Overview: Generic template based on game type
- Example: Omitted
- Edge cases: Standard defaults only

## Genome-Conditional Edge Case Defaults (REVISED)

Defaults are now **conditional on genome mechanics** to avoid conflicts with evolved rules.

### Default Selection Logic

```python
def select_applicable_defaults(genome: GameGenome) -> list[EdgeCaseDefault]:
    """Only apply defaults that don't conflict with genome mechanics."""
    defaults = []

    # Deck exhaustion - SKIP if deck exhaustion is a win condition
    deck_exhaustion_wins = any(
        wc.type in ("deck_empty", "last_card") for wc in genome.win_conditions
    )
    if not deck_exhaustion_wins:
        defaults.append(DECK_EXHAUSTION_RESHUFFLE)

    # No valid plays - SKIP if genome has explicit pass/skip mechanics
    has_pass_phase = any(
        isinstance(p, PlayPhase) and p.min_cards == 0
        for p in genome.turn_structure.phases
    )
    if not has_pass_phase:
        defaults.append(NO_VALID_PLAYS_DRAW_OR_PASS)

    # Hand limit - SKIP for accumulation games (capture_all, most_cards)
    accumulation_game = any(
        wc.type in ("capture_all", "most_cards") for wc in genome.win_conditions
    )
    if not accumulation_game:
        defaults.append(HAND_LIMIT_15)

    # Betting defaults - ONLY if betting phases exist
    has_betting = any(isinstance(p, BettingPhase) for p in genome.turn_structure.phases)
    if has_betting:
        defaults.append(BETTING_ALL_IN)
        defaults.append(BETTING_POT_SPLIT)

    return defaults
```

### Available Defaults

| Default | Condition to Apply | Rule |
|---------|-------------------|------|
| **Deck Exhaustion** | No "deck_empty" win condition | Reshuffle discard (except top card) |
| **No Valid Plays** | No optional (min=0) play phase | Draw up to 3, then pass |
| **Simultaneous Win** | Always applies | Active player wins ties |
| **Hand Limit** | Not a capture/accumulation game | Discard to 15 at end of turn |
| **Betting: All-In** | Has BettingPhase | Player with 0 chips goes all-in |
| **Betting: Pot Split** | Has BettingPhase | Odd chips go to dealer-left |
| **Turn Limit** | Always applies | Use max_turns from genome, then highest score or draw |

### Explicit Genome Override

If a genome explicitly specifies behavior for an edge case (future enhancement), that takes precedence over any default.

## CLI Interface

### Standalone Command

```bash
# Generate rulebook for a single genome
uv run python -m darwindeck.cli.rulebook genome.json

# Output to specific file
uv run python -m darwindeck.cli.rulebook genome.json -o rulebook.md

# Skip LLM enhancement (fast, offline)
uv run python -m darwindeck.cli.rulebook genome.json --basic

# Generate for all genomes in a directory
uv run python -m darwindeck.cli.rulebook output/evolution-*/2026-01-13/
```

### Options

```
Usage: python -m darwindeck.cli.rulebook [OPTIONS] GENOME_PATH

Arguments:
  GENOME_PATH    Path to genome JSON file or directory

Options:
  -o, --output PATH     Output file (default: <genome_id>_rulebook.md)
  --basic               Skip LLM enhancement, use templates only
  --top N               If directory, only process top N by fitness
  -v, --verbose         Show extraction details
```

### Evolution Integration

```bash
uv run python -m darwindeck.cli.evolve \
  --population 1000 \
  --generations 100 \
  --rulebooks 5        # Generate rulebooks for top 5
```

Saves to: `output/evolution-*/TIMESTAMP/rulebooks/`

## Sample Output

```markdown
# CloudWall

## Overview
A tense bluffing game where players claim cards while opponents
challenge their honesty. Empty your hand or catch enough liars to win.

## Components
- Standard 52-card deck (2-4 players)

## Setup
1. Shuffle the deck
2. Deal 20 cards to each player
3. Place remaining cards face-down as the draw pile

## Objective
Win by either:
- Emptying your hand completely, OR
- Capturing all cards (from successful challenges)

## Turn Structure

### Phase 1: Claim
Declare a card rank and place 1 card face-down on the pile.
You may bluff - the card doesn't need to match your claim.

### Phase 2: Challenge (Opponents)
Any opponent may say "Challenge!" before the next player's turn.
- If challenged: Reveal the card
  - Honest claim: Challenger takes the entire pile
  - Bluff caught: You take the entire pile

## Special Rules
None.

## Edge Cases
- **Empty deck:** Reshuffle the pile (except top card)
- **Ties:** Active player wins if both empty hands simultaneously
- **All cards captured:** Game ends immediately

## Quick Reference
→ Claim a rank, play face-down → Opponents may challenge → Liar takes pile
```

## Implementation Tasks

1. **`GenomeValidator`** - Pre-extraction validation (feasibility checks)
2. **`RulebookSections`** dataclass - Intermediate representation
3. **`GenomeExtractor`** - Deterministic phase/win condition mapping
4. **`select_applicable_defaults()`** - Genome-conditional edge case selection
5. **`RulebookGenerator.render_markdown()`** - Markdown output
6. **`RulebookEnhancer`** - Three targeted LLM calls
7. **`OutputValidator`** - Post-LLM validation (fallback on failure)
8. **CLI command** `darwindeck.cli.rulebook`
9. **Evolution integration** `--rulebooks` flag
10. **Tests** - Validation logic, extraction, golden file tests for LLM

## Multi-Agent Review Summary

This design was reviewed by Claude, Gemini, and Codex. Three STRONG issues were identified and addressed:

| Issue | Resolution |
|-------|------------|
| LLM constraint unenforceable | Added `OutputValidator` with fallback to basic mode |
| No validation layer | Added `GenomeValidator` pre-extraction checks |
| Defaults conflict with mechanics | Made defaults genome-conditional |

See `/tmp/consensus-*.md` for full review details.
