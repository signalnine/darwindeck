# Human Playtesting CLI Design

**Date:** 2026-01-14
**Status:** Draft
**Goal:** Interactive terminal game for humans to playtest evolved card games against AI opponents

## Overview

A CLI tool that lets humans play evolved card games in the terminal against AI opponents of varying difficulty. Collects feedback to correlate fitness metrics with actual human enjoyment.

## Architecture

```
src/darwindeck/
├── cli/
│   └── playtest.py      # CLI entry point
├── playtest/            # Playtest module
│   ├── __init__.py
│   ├── session.py       # Game session management
│   ├── display.py       # Terminal output formatting
│   ├── input.py         # Human input handling
│   └── feedback.py      # Post-game feedback collection
```

### Core Components

1. **CLI (`playtest.py`)** — Parses args, loads genome (from file or picker), starts session
2. **Session (`session.py`)** — Orchestrates game loop: display state → get move → apply → check win
3. **Display (`display.py`)** — Formats hand, game state, prompts for terminal output
4. **Input (`input.py`)** — Prompts human for moves, validates input, handles quit
5. **Feedback (`feedback.py`)** — Collects rating, saves to JSON alongside genome

### Key Classes

- `PlaytestSession` — Main game loop, tracks human player index, AI type
- `HumanPlayer` — Implements same interface as AI players but prompts for input
- `GameDisplay` — Renders state to terminal (hand, discard, turn info)

### Dependencies

- Reuses `GameState`, `generate_legal_moves`, `apply_move` from simulation
- Reuses `genome_from_dict` for loading genomes
- No new external dependencies (just stdlib + existing project deps)

## Game Flow

### Startup Sequence

1. Parse CLI args: `--genome PATH`, `--debug`, `--difficulty [random|greedy|mcts]`
2. If no genome specified, show interactive picker (list recent evolution runs)
3. Load genome, validate it's playable via `GenomeValidator`
4. Ask difficulty if not specified: "Choose AI difficulty: [1] Random [2] Greedy [3] MCTS"
5. Initialize game state (shuffle, deal)
6. Randomly assign human to player 0 or 1

### Main Game Loop

```python
while not game_over:
    display_game_state()

    if current_player == human:
        move = prompt_human_for_move()
        if move == QUIT:
            ask_if_game_broken()
            break
    else:
        move = ai.select_move()
        display_ai_move()

    state = apply_move(state, move)
    check_win_conditions()

    # Stuck detection: no progress for 50 turns
    if turns_without_progress > 50:
        end_game("Game appears stuck")
        break
```

### Turn Display

```
=== Turn 12 ===
Your hand: [1] 7♠  [2] K♥  [3] 3♦  [4] 9♣
Discard pile: Q♥
Phase: Play 1 card to discard

Play card [1-4] or [q]uit:
```

### Debug Mode

With `--debug` flag, also show:
- AI's hand
- Full deck contents
- Internal game state

## Feedback Collection

### Post-Game Sequence

```
=== Game Over ===
Winner: You! (emptied hand in 23 turns)

Rate this game:
[1] Not fun  [2] Meh  [3] Okay  [4] Good  [5] Great

Your rating: 4

Any comments? (press Enter to skip): Interesting bluffing mechanic

Thanks! Play again? [y/n]:
```

### Data Format

Saved to `playtest_results.jsonl` (append-only):

```json
{
  "timestamp": "2026-01-14T10:30:00",
  "genome_id": "InnerBout",
  "genome_path": "output/evolution-.../rank01_InnerBout.json",
  "difficulty": "greedy",
  "winner": "human",
  "turns": 23,
  "rating": 4,
  "comment": "Interesting bluffing mechanic",
  "quit_early": false,
  "felt_broken": false
}
```

### Early Quit Feedback

```
You quit the game.
Did the game feel broken or unplayable? [y/n]: y
What went wrong? (optional): No valid moves for 10 turns

(Saved feedback - thanks!)
```

## Interactive Genome Picker

When no genome file specified:

```
$ ./scripts/playtest.sh

No genome specified. Recent evolution runs:

[1] 2026-01-13_21-56-51 (bluffing style)
    Top: InnerBout (0.816), DimArrow (0.768), CloudWall (0.725)

[2] 2026-01-12_16-59-12 (strategic style)
    Top: SwiftEdge (0.792), BrightLock (0.761)

[3] Enter path manually

Select run [1-3]: 1

Games from 2026-01-13_21-56-51:
[1] InnerBout    (fitness: 0.816, skill: 0.49)
[2] DimArrow     (fitness: 0.768, skill: 0.77)
[3] CloudWall    (fitness: 0.725, skill: 1.00)

Select game [1-5]: 3

Loading CloudWall...
```

**Implementation:**
- Scan `output/evolution-*/` for recent runs (sort by date)
- Parse genome files for names and fitness
- Show top 5 runs, top 5 games per run
- Fallback: prompt for manual path if no runs found

## Error Handling

### Stuck Detection

- Track "turns without hand size change"
- After 50 such turns: "Game appears stuck (no progress for 50 turns). Ending game."
- Mark as `stuck: true` in feedback data

### No Legal Moves

- If `generate_legal_moves()` returns empty: "No legal moves available. Passing turn."
- 5 consecutive passes triggers stuck detection

### Invalid Input

```
Play card [1-4] or [q]uit: 7
Invalid choice. Enter 1-4 or q: 2
```

### Ctrl+C Handling

- Catch `KeyboardInterrupt`, treat as quit
- Still prompt for "felt broken?" feedback before exit

### Genome Validation

- Run `GenomeValidator` before starting
- If invalid, show error and exit

### AI Timeout

- MCTS: 5-second timeout per move
- Fallback to greedy if timeout, warn user

## CLI Interface

```
Usage: playtest [OPTIONS] [GENOME_PATH]

  Play an evolved card game against an AI opponent.

Arguments:
  GENOME_PATH          Path to genome JSON file (optional - shows picker if omitted)

Options:
  -d, --difficulty [random|greedy|mcts]
                       AI difficulty (prompts if not specified)
  --debug              Show AI's hand and full game state
  --results PATH       Where to save playtest results (default: playtest_results.jsonl)
  -v, --verbose        Verbose logging
  --help               Show this message and exit
```

### Shell Script

```bash
#!/usr/bin/env bash
# scripts/playtest.sh
set -euo pipefail
cd "$(dirname "$0")/.."
uv run python -m darwindeck.cli.playtest "$@"
```

### Example Usage

```bash
# Quick start - interactive picker
./scripts/playtest.sh

# Specific game, easy mode
./scripts/playtest.sh output/.../rank01_InnerBout.json -d random

# Debug mode to understand game mechanics
./scripts/playtest.sh genome.json --debug -d greedy
```

## Implementation Tasks

1. Create `playtest/` module structure
2. Implement `GameDisplay` for terminal output
3. Implement `HumanPlayer` input handling
4. Implement `PlaytestSession` game loop
5. Implement feedback collection and saving
6. Implement interactive genome picker
7. Create CLI entry point
8. Add stuck detection and error handling
9. Create shell script wrapper
10. Manual testing with real evolved games
