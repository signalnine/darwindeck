# Full Rulebook Generation Design

**Date:** 2026-01-14
**Status:** Approved
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

1. **`RulebookGenerator`** - Main class that orchestrates generation
2. **`GenomeExtractor`** - Deterministic extraction of rules from genome fields
3. **`RulebookEnhancer`** - Optional LLM pass for prose polish and examples

### Data Flow

```
GameGenome
    → GenomeExtractor.extract()
    → RulebookSections (structured data)
    → RulebookGenerator.render_markdown()
    → Basic rulebook (no LLM)
    → RulebookEnhancer.enhance() [optional]
    → Final polished rulebook
```

Key benefits:
- Works without API key (basic mode)
- LLM only adds value, never invents rules
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

### Basic Mode (No LLM)

- Overview: Generic template based on game type
- Example: Omitted
- Edge cases: Standard defaults only

## Standard Edge Case Defaults

### Deck Exhaustion
```
If deck is empty and a draw is required:
→ Shuffle the discard pile (except top card) to form new deck
→ If still empty: skip the draw phase
```

### No Valid Plays
```
If no legal card can be played:
→ Draw games: Draw until playable card found (max 3 draws, then pass)
→ Non-draw games: Pass turn
```

### Simultaneous Win
```
If multiple players meet win conditions on same turn:
→ Points-based: Highest score wins
→ Empty-hand: Active player wins
→ Capture: Player with more captured cards wins
→ Still tied: Draw
```

### Hand Limit
```
If hand exceeds 15 cards:
→ Discard down to 15 at end of turn
```

### Betting Edge Cases
```
If chips reach 0: Player is eliminated (or all-in if mid-round)
If pot can't split evenly: Remainder to player closest to dealer
```

### Turn Limit
```
If max_turns reached:
→ Score-based: Highest score wins
→ Other: Draw
```

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

1. Create `RulebookSections` dataclass
2. Implement `GenomeExtractor` with phase/win condition mapping
3. Implement `RulebookGenerator.render_markdown()`
4. Implement `RulebookEnhancer` with three LLM calls
5. Create CLI command `darwindeck.cli.rulebook`
6. Add `--rulebooks` flag to evolution CLI
7. Write tests for extraction logic
8. Test with evolved genomes from recent runs
