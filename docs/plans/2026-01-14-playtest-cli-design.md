# Human Playtesting CLI Design

**Date:** 2026-01-14
**Status:** Draft (Updated after multi-agent review)
**Goal:** Interactive terminal game for humans to playtest evolved card games against AI opponents

## Overview

A CLI tool that lets humans play evolved card games in the terminal against AI opponents of varying difficulty. Collects feedback to correlate fitness metrics with actual human enjoyment.

## Multi-Agent Review Findings (Addressed)

**STRONG issues fixed:**
1. ✅ Game abstraction layer for arbitrary evolved games (Section: Game Abstraction)
2. ✅ Improved stuck detection with state hashing (Section: Error Handling)
3. ✅ Reproducibility via seed and move history (Section: Reproducibility)

## Architecture

```
src/darwindeck/
├── cli/
│   └── playtest.py      # CLI entry point
├── playtest/            # Playtest module
│   ├── __init__.py
│   ├── session.py       # Game session management
│   ├── display.py       # StateRenderer, MovePresenter
│   ├── input.py         # Human input handling
│   ├── feedback.py      # Post-game feedback collection
│   ├── rules.py         # RuleExplainer
│   └── stuck.py         # StuckDetector
```

### Core Components

1. **CLI (`playtest.py`)** — Parses args, loads genome (from file or picker), starts session
2. **Session (`session.py`)** — Orchestrates game loop: display state → get move → apply → check win
3. **Display (`display.py`)** — `StateRenderer` and `MovePresenter` for terminal output
4. **Input (`input.py`)** — Prompts human for moves, validates input, handles quit
5. **Feedback (`feedback.py`)** — Collects rating, saves to JSON alongside genome
6. **Rules (`rules.py`)** — `RuleExplainer` generates human-readable rules from genome
7. **Stuck (`stuck.py`)** — `StuckDetector` with state hashing and multiple detection strategies

### Key Classes

- `PlaytestSession` — Main game loop, tracks seed, move history, human player index
- `StateRenderer` — Renders visible game state based on genome structure
- `MovePresenter` — Presents legal moves in human-readable format per phase type
- `RuleExplainer` — Generates rule summary from genome
- `StuckDetector` — Detects stuck games via state hashing and progress tracking
- `HumanPlayer` — Implements same interface as AI players but prompts for input

### Dependencies

- Reuses `GameState`, `generate_legal_moves`, `apply_move` from simulation
- Reuses `genome_from_dict` for loading genomes
- No new external dependencies (just stdlib + existing project deps)

## Game Abstraction Layer

Evolved games have arbitrary phases and move types. The abstraction layer handles this generically.

### MovePresenter

Converts `LegalMove` objects into human-readable prompts:

```python
class MovePresenter:
    """Presents moves to human based on phase type."""

    def present_moves(self, moves: list[LegalMove], state: GameState, genome: GameGenome) -> str:
        """Returns formatted move options for current phase."""
        phase = genome.turn_structure.phases[state.current_phase]

        if phase.phase_type == PhaseType.PLAY_CARD:
            return self._present_card_play(moves, state)
        elif phase.phase_type == PhaseType.DRAW_CARD:
            return self._present_draw(moves, state)
        elif phase.phase_type == PhaseType.BETTING:
            return self._present_betting(moves, state)
        elif phase.phase_type == PhaseType.PASS:
            return "[1] Pass turn"
        else:
            return self._present_generic(moves)

    def _present_card_play(self, moves: list[LegalMove], state: GameState) -> str:
        """Show hand cards with numbers."""
        hand = state.hands[state.active_player]
        lines = ["Your hand:"]
        for i, card in enumerate(hand):
            lines.append(f"  [{i+1}] {card}")
        return "\n".join(lines)

    def _present_betting(self, moves: list[LegalMove], state: GameState) -> str:
        """Show betting actions."""
        lines = [f"Your chips: {state.players[state.active_player].chips}"]
        lines.append(f"Current bet: {state.current_bet}")
        for i, move in enumerate(moves):
            action = self._betting_action_name(move)
            lines.append(f"  [{i+1}] {action}")
        return "\n".join(lines)

    def _present_generic(self, moves: list[LegalMove]) -> str:
        """Fallback for unknown phase types."""
        lines = ["Available moves:"]
        for i, move in enumerate(moves):
            lines.append(f"  [{i+1}] {move}")
        return "\n".join(lines)
```

### StateRenderer

Renders game state based on what locations exist:

```python
class StateRenderer:
    """Renders visible game state to terminal."""

    def render(self, state: GameState, genome: GameGenome,
               player_idx: int, debug: bool = False) -> str:
        """Render state from player's perspective."""
        sections = []

        # Always show player's hand
        hand = state.hands[player_idx]
        sections.append(self._render_hand(hand))

        # Show discard pile if genome uses it
        if self._has_discard(genome):
            sections.append(f"Discard: {self._top_card(state.discard)}")

        # Show tableau if genome uses it
        if self._has_tableau(genome):
            sections.append(self._render_tableau(state))

        # Show chips if betting game
        if genome.setup.starting_chips > 0:
            sections.append(f"Chips: {state.players[player_idx].chips}")
            sections.append(f"Pot: {state.pot}")

        # Debug mode shows everything
        if debug:
            sections.append(self._render_debug(state, player_idx))

        return "\n".join(sections)

    def _has_discard(self, genome: GameGenome) -> bool:
        """Check if genome uses discard pile."""
        for phase in genome.turn_structure.phases:
            if phase.target_location == Location.DISCARD:
                return True
        return False
```

### RuleExplainer

Generates human-readable rules from genome:

```python
class RuleExplainer:
    """Explains game rules from genome."""

    def explain_rules(self, genome: GameGenome) -> str:
        """Generate condensed rule summary."""
        lines = [f"=== {genome.name} ==="]
        lines.append(f"Goal: {self._explain_win_condition(genome)}")
        lines.append(f"Turn: {self._explain_turn_structure(genome)}")
        if genome.setup.starting_chips > 0:
            lines.append(f"Betting: Min bet {self._find_min_bet(genome)}")
        return "\n".join(lines)

    def explain_phase(self, phase_idx: int, genome: GameGenome) -> str:
        """Explain current phase to player."""
        phase = genome.turn_structure.phases[phase_idx]
        return f"Phase: {self._phase_description(phase)}"
```

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
  "seed": 12345,
  "winner": "human",
  "turns": 23,
  "rating": 4,
  "comment": "Interesting bluffing mechanic",
  "quit_early": false,
  "felt_broken": false,
  "stuck_reason": null,
  "replay_path": "replays/2026-01-14_103000.json"
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

## Reproducibility

For debugging broken games and sharing interesting sessions.

### Seed Capture

```python
class PlaytestSession:
    def __init__(self, genome: GameGenome, seed: int | None = None):
        self.seed = seed if seed is not None else random.randint(0, 2**32)
        self.rng = random.Random(self.seed)
        self.move_history: list[MoveRecord] = []

@dataclass
class MoveRecord:
    turn: int
    player: int  # 0=human, 1=AI
    move: LegalMove
    timestamp: float
```

### CLI Options

```
--seed INT           Use specific random seed (for replay)
--save-replay PATH   Save move history to file
--load-replay PATH   Replay a saved game
```

### Replay Format

```json
{
  "genome_path": "output/.../game.json",
  "seed": 12345,
  "difficulty": "greedy",
  "moves": [
    {"turn": 1, "player": 0, "move": {"phase": 0, "card": 3}},
    {"turn": 2, "player": 1, "move": {"phase": 0, "card": 7}}
  ]
}
```

### Display Seed

At game start and in feedback:
```
Starting game with seed: 12345 (use --seed 12345 to replay)
```

## Error Handling

### Stuck Detection (State Hashing)

Multiple detection strategies to catch stuck games:

```python
class StuckDetector:
    """Detects stuck games using multiple strategies."""

    def __init__(self, max_turns: int = 200, repeat_threshold: int = 3):
        self.max_turns = max_turns
        self.repeat_threshold = repeat_threshold
        self.state_hashes: dict[int, int] = {}  # hash -> count
        self.no_progress_turns = 0
        self.consecutive_passes = 0

    def check(self, state: GameState, move: LegalMove) -> str | None:
        """Returns reason if stuck, None otherwise."""
        # Strategy 1: Absolute turn limit
        if state.turn >= self.max_turns:
            return f"Turn limit reached ({self.max_turns})"

        # Strategy 2: State repetition (same state seen N times)
        state_hash = self._hash_state(state)
        self.state_hashes[state_hash] = self.state_hashes.get(state_hash, 0) + 1
        if self.state_hashes[state_hash] >= self.repeat_threshold:
            return f"Same state repeated {self.repeat_threshold} times"

        # Strategy 3: No progress (hand sizes unchanged)
        if self._no_progress(state, move):
            self.no_progress_turns += 1
            if self.no_progress_turns >= 50:
                return "No progress for 50 turns"
        else:
            self.no_progress_turns = 0

        # Strategy 4: Consecutive passes
        if self._is_pass(move):
            self.consecutive_passes += 1
            if self.consecutive_passes >= 10:
                return "10 consecutive passes"
        else:
            self.consecutive_passes = 0

        return None

    def _hash_state(self, state: GameState) -> int:
        """Hash relevant state for comparison."""
        # Hash: hand sizes, deck size, discard top, active player
        key = (
            tuple(len(h) for h in state.hands),
            len(state.deck),
            state.discard[-1] if state.discard else None,
            state.active_player,
            state.current_phase,
        )
        return hash(key)
```

### Configurable Thresholds

```
--max-turns INT      Maximum turns before forced end (default: 200)
--repeat-limit INT   State repetition limit (default: 3)
```

### No Legal Moves

- If `generate_legal_moves()` returns empty: "No legal moves available. Passing turn."
- Tracked by StuckDetector's consecutive passes counter

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
  --seed INT           Random seed for reproducibility
  --save-replay PATH   Save move history for replay
  --load-replay PATH   Replay a saved game
  --max-turns INT      Turn limit before forced end (default: 200)
  --show-rules         Display rule summary before starting
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

1. Create `playtest/` module structure with `__init__.py`
2. Implement `StuckDetector` with state hashing (`stuck.py`)
3. Implement `StateRenderer` for terminal output (`display.py`)
4. Implement `MovePresenter` for move formatting (`display.py`)
5. Implement `RuleExplainer` for rule summaries (`rules.py`)
6. Implement `HumanPlayer` input handling (`input.py`)
7. Implement `PlaytestSession` game loop with seed/replay (`session.py`)
8. Implement feedback collection and saving (`feedback.py`)
9. Implement interactive genome picker (`session.py`)
10. Create CLI entry point with all options (`cli/playtest.py`)
11. Create shell script wrapper (`scripts/playtest.sh`)
12. Add unit tests for all components
13. Manual testing with real evolved games
