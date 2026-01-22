# Playtest Rich Display Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace plain text playtest display with Rich library for colored cards, panels, live updates, and move history.

**Architecture:** Add `RichDisplay` class using Rich's Live feature for in-place updates, with a `DisplayState` dataclass to decouple rendering from game internals. Falls back to plain text for piped input.

**Tech Stack:** Python, Rich library

---

## Overview

The current playtest display uses plain text with unicode card symbols. This overhaul adds:

- Colored suits (red hearts/diamonds, adaptive color for clubs/spades)
- Boxed panels for game state, hand, moves, history
- Live display that redraws in place (no scrolling during AI turns)
- Compact move log showing last 5 actions
- Robust terminal state recovery on errors

## Architecture

```
PlaytestSession
    │
    ├── DisplayState (dataclass - decouples game state from display)
    │
    ├── RichDisplay (TTY mode)
    │   ├── render(DisplayState) -> Layout
    │   ├── show_and_prompt(DisplayState) -> str  # Handles Live lifecycle per turn
    │   ├── show_message(text)
    │   └── cleanup()  # Guaranteed terminal restoration
    │
    └── StateRenderer + MovePresenter (pipe mode, unchanged)
```

**Key insight:** Instead of keeping Live running continuously and pausing for input, we render → stop Live → get input → repeat. This is acceptable for a turn-based game and avoids the flicker/state issues.

**New files:**
- `src/darwindeck/playtest/rich_display.py` - RichDisplay class
- `src/darwindeck/playtest/display_state.py` - DisplayState dataclass

**TTY detection:** `sys.stdout.isatty()` and `FORCE_PLAIN_DISPLAY` env var for override.

## DisplayState Dataclass

Decouples rendering from game internals. Session builds this, display renders it.

```python
@dataclass
class DisplayState:
    """Intermediate representation for display rendering."""
    game_name: str
    turn: int
    phase_name: str
    player_id: int

    # Hand info
    hand_cards: list[tuple[str, str]]  # [(rank, suit), ...]

    # Opponent info
    opponent_card_count: int
    opponent_chips: int
    opponent_bet: int

    # Player resources
    player_chips: int
    player_bet: int
    pot: int
    current_bet: int

    # Table state
    discard_top: tuple[str, str] | None  # (rank, suit) or None

    # Available moves
    moves: list[MoveOption]  # Simplified move representation

    # History
    move_log: list[tuple[str, str]]  # [(direction, description), ...]

    # Terminal info
    terminal_width: int

@dataclass
class MoveOption:
    """Simplified move for display."""
    index: int
    label: str  # "A♥", "Pass", "Fold", etc.
    move_type: str  # "card", "betting", "pass"
```

## Visual Layout

```
╭─────────────────── LittleHeart ───────────────────╮
│  Turn: 7  │  Phase: Play  │  You: P0             │
│  Your chips: 450  │  Pot: 100  │  Bet: 20        │
╰───────────────────────────────────────────────────╯

╭─ Opponent ────────────────────────────────────────╮
│  Cards: 8  │  Chips: 380  │  Bet: 20             │
╰───────────────────────────────────────────────────╯

╭─ Your Hand ───────────────────────────────────────╮
│                                                   │
│   [1] A♥   [2] K♠   [3] 9♦   [4] 7♣   [5] 3♥    │
│   [6] J♦   [7] 5♠   [8] 2♣                       │
│                                                   │
╰───────────────────────────────────────────────────╯

╭─ Discard Pile ────────────────────────────────────╮
│  Top: Q♥                                          │
╰───────────────────────────────────────────────────╯

╭─ Actions ─────────────────────────────────────────╮
│  [1] A♥   [2] K♠   [3] 9♦   [4] 7♣   [5] 3♥     │
│  [6] J♦   [7] 5♠   [8] 2♣   [9] Pass            │
╰───────────────────────────────────────────────────╯

  ← AI discarded J♣
  ← AI played 10♦
  → You discarded 4♠

Enter choice [1-9] or (q)uit: _
```

**Compact layout** (terminal width < 60):

```
═══ LittleHeart ═══
Turn 7 | Play | P0
Chips: 450 | Pot: 100

Opponent: 8 cards, 380 chips

Hand: A♥ K♠ 9♦ 7♣ 3♥ J♦ 5♠ 2♣
Discard: Q♥

[1-8] Play card  [9] Pass

← AI: J♣  ← AI: 10♦  → You: 4♠

Choice [1-9] or (q)uit: _
```

**Color scheme:**
- Hearts/Diamonds: `red`
- Clubs/Spades: `default` (adapts to terminal theme)
- Panel borders: `dim cyan`
- Pot/bet when active: `yellow`
- Move numbers: `bold green`
- Errors: `bold red`

## Implementation Details

### Card Formatting

```python
from rich.text import Text

# Constants
SUIT_SYMBOLS = {"H": "♥", "D": "♦", "C": "♣", "S": "♠"}
SUIT_COLORS = {"H": "red", "D": "red", "C": "default", "S": "default"}
MIN_WIDE_WIDTH = 60
MOVE_LOG_SIZE = 5

def format_card_rich(rank: str, suit: str) -> Text:
    """Format card with colored suit symbol."""
    symbol = SUIT_SYMBOLS.get(suit, suit)
    color = SUIT_COLORS.get(suit, "default")

    text = Text()
    text.append(rank, style="bold")
    text.append(symbol, style=color)
    return text
```

### RichDisplay Class

```python
from collections import deque
from rich.console import Console
from rich.live import Live
from rich.panel import Panel
from rich.layout import Layout
import os

class RichDisplay:
    """Rich-based terminal display with Live updates."""

    def __init__(self):
        self.console = Console()
        self._live: Live | None = None

    def render(self, state: DisplayState) -> Layout:
        """Build complete display layout from DisplayState."""
        if state.terminal_width < MIN_WIDE_WIDTH:
            return self._render_compact(state)
        return self._render_wide(state)

    def show_and_prompt(self, state: DisplayState, prompt: str = "Enter choice: ") -> str:
        """Render state, then prompt for input. Handles Live lifecycle safely."""
        layout = self.render(state)

        # Brief Live display to show state, then stop for input
        try:
            with Live(layout, console=self.console, refresh_per_second=4, transient=True):
                pass  # Just render once
        except Exception:
            # Fallback: print without Live if something goes wrong
            self.console.print(layout)

        # Now get input (outside Live context)
        try:
            return self.console.input(f"[green]{prompt}[/]").strip()
        except (EOFError, KeyboardInterrupt):
            return "q"

    def show_ai_turn(self, state: DisplayState, duration: float = 0.5) -> None:
        """Show AI turn briefly with Live display."""
        layout = self.render(state)
        try:
            with Live(layout, console=self.console, transient=True) as live:
                import time
                time.sleep(duration)
        except Exception:
            self.console.print(layout)

    def show_message(self, message: str, style: str = "bold") -> None:
        """Show a standalone message (win/lose, errors)."""
        self.console.print(f"[{style}]{message}[/]")

    def show_error(self, message: str) -> None:
        """Show error message."""
        self.console.print(f"[bold red]{message}[/]")

    def get_terminal_width(self) -> int:
        """Get current terminal width."""
        return self.console.width

    def _render_wide(self, state: DisplayState) -> Panel:
        """Render full-width layout with panels."""
        # ... implementation ...

    def _render_compact(self, state: DisplayState) -> Panel:
        """Render compact layout for narrow terminals."""
        # ... implementation ...
```

### Session Integration

```python
# In session.py

def run(self, output_fn: Callable[[str], None] = print) -> PlaytestResult:
    """Run the playtest session."""
    self.state = self._initialize_state()

    # Detect display mode
    use_rich = (
        sys.stdout.isatty()
        and not os.environ.get("FORCE_PLAIN_DISPLAY")
    )

    if use_rich:
        return self._run_rich()
    else:
        return self._run_plain(output_fn)

def _run_rich(self) -> PlaytestResult:
    """Run with Rich display."""
    display = RichDisplay()
    move_log: deque[tuple[str, str]] = deque(maxlen=MOVE_LOG_SIZE)

    try:
        while True:
            # Check stuck/win conditions...

            # Build display state
            ds = self._build_display_state(move_log, display.get_terminal_width())

            if self.state.active_player == self.human_player_idx:
                # Human turn: show and prompt
                raw = display.show_and_prompt(ds)
                result = self._parse_input(raw, moves)

                if result.quit:
                    break
                if result.error:
                    display.show_error(result.error)
                    continue
                # ... apply move, log it ...
                move_log.append(("→", f"You played {card_desc}"))
            else:
                # AI turn: show briefly
                display.show_ai_turn(ds, duration=0.3)
                move = self._ai_select_move(moves)
                # ... apply move, log it ...
                move_log.append(("←", f"AI played {card_desc}"))

        # Show result
        display.show_message(f"\n=== {result_text} ===", "bold green")

    except Exception as e:
        # Ensure terminal is clean even on error
        display.show_error(f"Error: {e}")
        raise

    return self._collect_feedback(display)

def _build_display_state(self, move_log: deque, terminal_width: int) -> DisplayState:
    """Convert GameState to DisplayState."""
    # ... extract and simplify game state for display ...
```

### Input Handling

Input parsing remains in `input.py` but is called differently:

```python
# In session.py

def _parse_input(self, raw: str, moves: list) -> InputResult:
    """Parse raw input string into InputResult."""
    if raw.lower() in ("q", "quit", "exit"):
        return InputResult(quit=True)

    if not raw:
        return InputResult(is_pass=True)

    try:
        choice = int(raw)
    except ValueError:
        return InputResult(error=f"Invalid input '{raw}'. Enter a number or 'q'.")

    if choice < 1 or choice > len(moves):
        return InputResult(error=f"Invalid choice {choice}. Enter 1-{len(moves)}.")

    return InputResult(move=moves[choice - 1])
```

### Move Log

```python
def _render_log(self, move_log: list[tuple[str, str]]) -> Text:
    """Render move history."""
    if not move_log:
        return Text("No moves yet", style="dim")

    text = Text()
    for i, (direction, desc) in enumerate(move_log):
        if i > 0:
            text.append("  ")
        style = "green" if direction == "→" else "cyan"
        text.append(direction, style=style)
        text.append(f" {desc}")

    return text
```

## File Changes

**Create:**
- `src/darwindeck/playtest/rich_display.py` - RichDisplay class
- `src/darwindeck/playtest/display_state.py` - DisplayState and MoveOption dataclasses

**Modify:**
- `pyproject.toml` - Add `rich` dependency
- `src/darwindeck/playtest/session.py` - Add `_run_rich()`, `_build_display_state()`, TTY detection

**Unchanged:**
- `display.py` - Keep for plain text mode (pipe fallback)
- `input.py` - Keep InputResult dataclass, parsing logic moves to session
- `feedback.py`, `stuck.py`, `rules.py`, `picker.py` - Unaffected

## Testing Strategy

1. **Unit tests (`tests/unit/test_rich_display.py`):**
   - `test_format_card_rich_red_suits()` - Hearts/diamonds are red
   - `test_format_card_rich_default_suits()` - Clubs/spades use default color
   - `test_move_log_maxlen()` - Deque keeps only 5 items
   - `test_display_state_construction()` - DisplayState builds correctly
   - `test_render_wide_vs_compact()` - Layout switches at width threshold

2. **Automated output tests (`tests/unit/test_rich_display.py`):**
   ```python
   def test_render_contains_expected_elements():
       console = Console(record=True, width=80)
       display = RichDisplay()
       display.console = console

       state = make_test_display_state()
       layout = display.render(state)
       console.print(layout)

       output = console.export_text()
       assert "Turn:" in output
       assert "Your Hand" in output
       assert "A♥" in output
   ```

3. **Integration test for fallback:**
   ```bash
   echo "1\n1\nq" | FORCE_PLAIN_DISPLAY=1 uv run python -m darwindeck.cli.playtest ...
   ```

4. **Manual verification checklist:**
   - [ ] Wide terminal (120+ cols) shows full panels
   - [ ] Narrow terminal (<60 cols) shows compact layout
   - [ ] Light terminal theme: clubs/spades visible
   - [ ] Dark terminal theme: all suits visible
   - [ ] Ctrl+C during input exits cleanly
   - [ ] AI turns show briefly then prompt appears
   - [ ] Move log shows last 5 moves correctly

## Future Enhancements

- **Textual migration:** If we need more interactive features (scrollable history, mouse support), consider migrating to Textual TUI framework.
- **Color themes:** Allow user to customize color scheme via config.
- **Animation:** Smooth card dealing animation for visual flair.
