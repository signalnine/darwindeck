# Phase 1: Foundation & Benchmarking Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Set up Python/Golang hybrid project structure and benchmark performance gains to validate Golang core decision.

**Architecture:** Dual-language project with Python for orchestration and Golang for performance-critical simulation loops. Prototype both versions of a simple game to measure speedup.

**Tech Stack:** Python 3.11+, Golang 1.21+, pytest, go test, protocol buffers or CGo

**Duration:** Week 1-2 (10-12 hours total)

**Success Criteria:**
- Working Python project structure with tests
- Working Golang module with benchmarks
- Measured speedup of 10-50x for simulation loops
- Decision made: CGo vs gRPC interface

---

## Task 1: Python Project Scaffolding

**Files:**
- Create: `pyproject.toml`
- Create: `src/darwindeck/__init__.py`
- Create: `src/darwindeck/genome/__init__.py`
- Create: `tests/__init__.py`
- Create: `.gitignore`

**Step 1: Write project configuration**

Create `pyproject.toml`:

```toml
[tool.poetry]
name = "cards-evolve"
version = "0.1.0"
description = "Evolutionary card game system"
authors = ["Your Name <you@example.com>"]
readme = "README.md"
packages = [{include = "darwindeck", from = "src"}]

[tool.poetry.dependencies]
python = "^3.11"
pydantic = "^2.5"
numpy = "^1.26"
click = "^8.1"
pyyaml = "^6.0"

[tool.poetry.group.dev.dependencies]
pytest = "^7.4"
pytest-cov = "^4.1"
hypothesis = "^6.92"
black = "^23.12"
mypy = "^1.8"

[build-system]
requires = ["poetry-core"]
build-backend = "poetry.core.masonry.api"

[tool.pytest.ini_options]
testpaths = ["tests"]
python_files = "test_*.py"
python_functions = "test_*"

[tool.black]
line-length = 100
target-version = ['py311']

[tool.mypy]
python_version = "3.11"
warn_return_any = true
warn_unused_configs = true
disallow_untyped_defs = true
```

**Step 2: Create package structure**

```bash
mkdir -p src/darwindeck/genome
mkdir -p src/darwindeck/simulation
mkdir -p src/darwindeck/evolution
mkdir -p src/darwindeck/cli
mkdir -p tests/unit
mkdir -p tests/integration
mkdir -p docs/plans
```

**Step 3: Create __init__.py files**

Create `src/darwindeck/__init__.py`:
```python
"""Evolutionary card game system."""

__version__ = "0.1.0"
```

Create `src/darwindeck/genome/__init__.py`:
```python
"""Game genome representation and manipulation."""
```

Create `tests/__init__.py`:
```python
"""Test suite for cards-evolve."""
```

**Step 4: Create .gitignore**

Create `.gitignore`:
```
# Python
__pycache__/
*.py[cod]
*$py.class
*.so
.Python
build/
develop-eggs/
dist/
downloads/
eggs/
.eggs/
lib/
lib64/
parts/
sdist/
var/
wheels/
*.egg-info/
.installed.cfg
*.egg

# Virtual environments
venv/
ENV/
env/
.venv

# Testing
.pytest_cache/
.coverage
htmlcov/
.tox/

# IDEs
.vscode/
.idea/
*.swp
*.swo

# Project specific
experiments/
*.pyc

# Golang
*.exe
*.test
*.out
gosim/vendor/
```

**Step 5: Install dependencies**

Run:
```bash
poetry install
```

Expected: Dependencies installed successfully

**Step 6: Verify Python setup**

Run:
```bash
poetry run python -c "import darwindeck; print(darwindeck.__version__)"
```

Expected: `0.1.0`

**Step 7: Commit**

```bash
git add pyproject.toml src/ tests/ .gitignore
git commit -m "feat: initialize Python project structure

- Add Poetry configuration with dependencies
- Create package structure for genome, simulation, evolution, cli
- Add pytest configuration
- Add development tools (black, mypy)"
```

---

## Task 2: Basic Genome Schema

**Files:**
- Create: `src/darwindeck/genome/schema.py`
- Create: `tests/unit/test_genome_schema.py`

**Step 1: Write failing test for Rank enum**

Create `tests/unit/test_genome_schema.py`:

```python
"""Tests for genome schema types."""

import pytest
from darwindeck.genome.schema import Rank, Suit, GameGenome


def test_rank_enum_has_all_ranks() -> None:
    """Test that Rank enum contains all standard playing card ranks."""
    assert Rank.ACE.value == "A"
    assert Rank.TWO.value == "2"
    assert Rank.KING.value == "K"
    assert len(Rank) == 13


def test_suit_enum_has_four_suits() -> None:
    """Test that Suit enum contains four standard suits."""
    assert Suit.HEARTS.value == "H"
    assert Suit.DIAMONDS.value == "D"
    assert Suit.CLUBS.value == "C"
    assert Suit.SPADES.value == "S"
    assert len(Suit) == 4
```

**Step 2: Run test to verify it fails**

Run:
```bash
poetry run pytest tests/unit/test_genome_schema.py -v
```

Expected: ImportError - module 'darwindeck.genome.schema' not found

**Step 3: Write minimal implementation**

Create `src/darwindeck/genome/schema.py`:

```python
"""Core genome schema types and enumerations."""

from enum import Enum
from typing import List, Optional, Union
from dataclasses import dataclass, field


class Rank(Enum):
    """Playing card ranks."""

    ACE = "A"
    TWO = "2"
    THREE = "3"
    FOUR = "4"
    FIVE = "5"
    SIX = "6"
    SEVEN = "7"
    EIGHT = "8"
    NINE = "9"
    TEN = "10"
    JACK = "J"
    QUEEN = "Q"
    KING = "K"


class Suit(Enum):
    """Playing card suits."""

    HEARTS = "H"
    DIAMONDS = "D"
    CLUBS = "C"
    SPADES = "S"


class Location(Enum):
    """Card locations in game."""

    DECK = "deck"
    HAND = "hand"
    DISCARD = "discard"
    TABLEAU = "tableau"


@dataclass
class GameGenome:
    """Placeholder for complete genome structure."""

    schema_version: str = "1.0"
    genome_id: str = ""
    generation: int = 0
```

**Step 4: Run test to verify it passes**

Run:
```bash
poetry run pytest tests/unit/test_genome_schema.py -v
```

Expected: 2 passed

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_genome_schema.py
git commit -m "feat: add basic genome schema enums

- Add Rank enum with 13 card ranks
- Add Suit enum with 4 suits
- Add Location enum for card positions
- Add GameGenome placeholder dataclass
- Include tests for enum completeness"
```

---

## Task 3: Golang Module Setup

**Files:**
- Create: `go.mod`
- Create: `src/gosim/main.go`
- Create: `src/gosim/game/card.go`
- Create: `src/gosim/game/card_test.go`

**Step 1: Initialize Go module**

Run:
```bash
cd src/gosim
go mod init github.com/youruser/cards-evolve/gosim
cd ../..
```

**Step 2: Write card representation test**

Create `src/gosim/game/card_test.go`:

```go
package game

import "testing"

func TestCard_String(t *testing.T) {
	tests := []struct {
		card Card
		want string
	}{
		{Card{Rank: Ace, Suit: Hearts}, "AH"},
		{Card{Rank: King, Suit: Spades}, "KS"},
		{Card{Rank: Two, Suit: Diamonds}, "2D"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.card.String(); got != tt.want {
				t.Errorf("Card.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewDeck(t *testing.T) {
	deck := NewDeck()

	if len(deck) != 52 {
		t.Errorf("NewDeck() returned %d cards, want 52", len(deck))
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, card := range deck {
		key := card.String()
		if seen[key] {
			t.Errorf("Duplicate card found: %s", key)
		}
		seen[key] = true
	}
}
```

**Step 3: Run test to verify it fails**

Run:
```bash
cd src/gosim
go test ./game
```

Expected: Compilation error - undefined types

**Step 4: Write minimal implementation**

Create `src/gosim/game/card.go`:

```go
package game

import "fmt"

// Rank represents a card rank
type Rank int

const (
	Ace Rank = iota + 1
	Two
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
)

// String returns the rank as a string
func (r Rank) String() string {
	ranks := []string{"", "A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}
	return ranks[r]
}

// Suit represents a card suit
type Suit int

const (
	Hearts Suit = iota + 1
	Diamonds
	Clubs
	Spades
)

// String returns the suit as a string
func (s Suit) String() string {
	suits := []string{"", "H", "D", "C", "S"}
	return suits[s]
}

// Card represents a playing card
type Card struct {
	Rank Rank
	Suit Suit
}

// String returns the card as a string (e.g., "AH")
func (c Card) String() string {
	return fmt.Sprintf("%s%s", c.Rank.String(), c.Suit.String())
}

// NewDeck creates a standard 52-card deck
func NewDeck() []Card {
	deck := make([]Card, 0, 52)
	for suit := Hearts; suit <= Spades; suit++ {
		for rank := Ace; rank <= King; rank++ {
			deck = append(deck, Card{Rank: rank, Suit: suit})
		}
	}
	return deck
}
```

**Step 5: Run test to verify it passes**

Run:
```bash
cd src/gosim
go test ./game -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add go.mod src/gosim/
git commit -m "feat: initialize Golang module with card types

- Add go.mod for gosim module
- Implement Rank and Suit types with String methods
- Implement Card struct
- Add NewDeck function for standard 52-card deck
- Include comprehensive tests"
```

---

## Task 4: Python War Game Implementation

**Files:**
- Create: `src/darwindeck/simulation/war.py`
- Create: `tests/unit/test_war_simulation.py`

**Step 1: Write failing test for War game**

Create `tests/unit/test_war_simulation.py`:

```python
"""Tests for War game simulation."""

import pytest
from darwindeck.simulation.war import WarGame, play_war_game


def test_war_game_initialization() -> None:
    """Test War game initializes with 52 cards split evenly."""
    game = WarGame(seed=42)
    assert len(game.player1_hand) == 26
    assert len(game.player2_hand) == 26


def test_play_single_battle() -> None:
    """Test a single battle resolves correctly."""
    game = WarGame(seed=42)
    initial_p1 = len(game.player1_hand)
    initial_p2 = len(game.player2_hand)

    game.play_battle()

    # One player should have gained cards
    assert len(game.player1_hand) + len(game.player2_hand) == 52


def test_play_full_game() -> None:
    """Test a full game runs to completion."""
    result = play_war_game(seed=42, max_turns=1000)

    assert result["winner"] in [1, 2]
    assert result["turns"] > 0
    assert result["turns"] <= 1000
```

**Step 2: Run test to verify it fails**

Run:
```bash
poetry run pytest tests/unit/test_war_simulation.py -v
```

Expected: ImportError

**Step 3: Write minimal implementation**

Create `src/darwindeck/simulation/war.py`:

```python
"""War game simulation (Python baseline)."""

import random
from typing import Dict, List, Tuple


class WarGame:
    """Simple War card game implementation."""

    def __init__(self, seed: int = 42) -> None:
        """Initialize War game with shuffled deck."""
        self.rng = random.Random(seed)

        # Create deck (rank only matters for War)
        deck = list(range(1, 14)) * 4  # 1-13, four suits
        self.rng.shuffle(deck)

        # Split evenly
        self.player1_hand = deck[:26]
        self.player2_hand = deck[26:]
        self.turns = 0

    def play_battle(self) -> None:
        """Play one battle."""
        if not self.player1_hand or not self.player2_hand:
            return

        p1_card = self.player1_hand.pop(0)
        p2_card = self.player2_hand.pop(0)

        if p1_card > p2_card:
            self.player1_hand.extend([p1_card, p2_card])
        elif p2_card > p1_card:
            self.player2_hand.extend([p2_card, p1_card])
        else:
            # War! Each player plays 3 face down + 1 face up
            if len(self.player1_hand) >= 4 and len(self.player2_hand) >= 4:
                war_pile = [p1_card, p2_card]
                war_pile.extend(self.player1_hand[:4])
                war_pile.extend(self.player2_hand[:4])
                self.player1_hand = self.player1_hand[4:]
                self.player2_hand = self.player2_hand[4:]

                # Winner takes all
                if war_pile[-4] > war_pile[-1]:  # p1 wins
                    self.player1_hand.extend(war_pile)
                else:  # p2 wins
                    self.player2_hand.extend(war_pile)
            else:
                # Not enough cards for war, return cards
                self.player1_hand.append(p1_card)
                self.player2_hand.append(p2_card)

        self.turns += 1

    def is_game_over(self) -> bool:
        """Check if game has ended."""
        return len(self.player1_hand) == 0 or len(self.player2_hand) == 0

    def get_winner(self) -> int:
        """Get winner (1 or 2)."""
        if len(self.player1_hand) > len(self.player2_hand):
            return 1
        return 2


def play_war_game(seed: int = 42, max_turns: int = 1000) -> Dict[str, int]:
    """Play a complete War game and return results."""
    game = WarGame(seed=seed)

    while not game.is_game_over() and game.turns < max_turns:
        game.play_battle()

    return {
        "winner": game.get_winner(),
        "turns": game.turns
    }
```

**Step 4: Run test to verify it passes**

Run:
```bash
poetry run pytest tests/unit/test_war_simulation.py -v
```

Expected: 3 passed

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/war.py tests/unit/test_war_simulation.py
git commit -m "feat: implement War game in Python

- Add WarGame class with battle mechanics
- Handle war (tie) scenarios
- Add play_war_game function for full games
- Include tests for initialization, battles, and completion"
```

---

## Task 5: Golang War Game Implementation

**Files:**
- Create: `src/gosim/game/war.go`
- Create: `src/gosim/game/war_test.go`

**Step 1: Write failing benchmark test**

Create `src/gosim/game/war_test.go`:

```go
package game

import (
	"testing"
)

func TestWarGame_PlayBattle(t *testing.T) {
	game := NewWarGame(42)

	if len(game.Player1Hand) != 26 {
		t.Errorf("Player1 has %d cards, want 26", len(game.Player1Hand))
	}
	if len(game.Player2Hand) != 26 {
		t.Errorf("Player2 has %d cards, want 26", len(game.Player2Hand))
	}

	game.PlayBattle()

	total := len(game.Player1Hand) + len(game.Player2Hand)
	if total != 52 {
		t.Errorf("Total cards = %d, want 52", total)
	}
}

func TestPlayWarGame(t *testing.T) {
	result := PlayWarGame(42, 1000)

	if result.Winner != 1 && result.Winner != 2 {
		t.Errorf("Winner = %d, want 1 or 2", result.Winner)
	}
	if result.Turns < 1 || result.Turns > 1000 {
		t.Errorf("Turns = %d, want 1-1000", result.Turns)
	}
}

func BenchmarkPlayWarGame(b *testing.B) {
	for i := 0; i < b.N; i++ {
		PlayWarGame(int64(i), 1000)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd src/gosim
go test ./game -run TestWarGame
```

Expected: Compilation error - undefined types

**Step 3: Write implementation**

Create `src/gosim/game/war.go`:

```go
package game

import (
	"math/rand"
)

// WarGame represents the game state for War
type WarGame struct {
	Player1Hand []int
	Player2Hand []int
	Turns       int
	rng         *rand.Rand
}

// WarResult contains game outcome
type WarResult struct {
	Winner int
	Turns  int
}

// NewWarGame creates a new War game
func NewWarGame(seed int64) *WarGame {
	rng := rand.New(rand.NewSource(seed))

	// Create deck (ranks 1-13, four of each)
	deck := make([]int, 52)
	idx := 0
	for suit := 0; suit < 4; suit++ {
		for rank := 1; rank <= 13; rank++ {
			deck[idx] = rank
			idx++
		}
	}

	// Shuffle
	rng.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	return &WarGame{
		Player1Hand: deck[:26],
		Player2Hand: deck[26:],
		Turns:       0,
		rng:         rng,
	}
}

// PlayBattle plays one battle
func (g *WarGame) PlayBattle() {
	if len(g.Player1Hand) == 0 || len(g.Player2Hand) == 0 {
		return
	}

	p1Card := g.Player1Hand[0]
	p2Card := g.Player2Hand[0]
	g.Player1Hand = g.Player1Hand[1:]
	g.Player2Hand = g.Player2Hand[1:]

	if p1Card > p2Card {
		g.Player1Hand = append(g.Player1Hand, p1Card, p2Card)
	} else if p2Card > p1Card {
		g.Player2Hand = append(g.Player2Hand, p2Card, p1Card)
	} else {
		// War!
		if len(g.Player1Hand) >= 4 && len(g.Player2Hand) >= 4 {
			warPile := []int{p1Card, p2Card}
			warPile = append(warPile, g.Player1Hand[:4]...)
			warPile = append(warPile, g.Player2Hand[:4]...)
			g.Player1Hand = g.Player1Hand[4:]
			g.Player2Hand = g.Player2Hand[4:]

			// Winner takes all
			if warPile[len(warPile)-4] > warPile[len(warPile)-1] {
				g.Player1Hand = append(g.Player1Hand, warPile...)
			} else {
				g.Player2Hand = append(g.Player2Hand, warPile...)
			}
		} else {
			// Not enough cards, return them
			g.Player1Hand = append(g.Player1Hand, p1Card)
			g.Player2Hand = append(g.Player2Hand, p2Card)
		}
	}

	g.Turns++
}

// IsGameOver checks if game has ended
func (g *WarGame) IsGameOver() bool {
	return len(g.Player1Hand) == 0 || len(g.Player2Hand) == 0
}

// GetWinner returns winner (1 or 2)
func (g *WarGame) GetWinner() int {
	if len(g.Player1Hand) > len(g.Player2Hand) {
		return 1
	}
	return 2
}

// PlayWarGame plays a complete game
func PlayWarGame(seed int64, maxTurns int) WarResult {
	game := NewWarGame(seed)

	for !game.IsGameOver() && game.Turns < maxTurns {
		game.PlayBattle()
	}

	return WarResult{
		Winner: game.GetWinner(),
		Turns:  game.Turns,
	}
}
```

**Step 4: Run tests and benchmark**

Run:
```bash
cd src/gosim
go test ./game -v
go test ./game -bench=BenchmarkPlayWarGame -benchtime=10s
```

Expected: Tests pass, benchmark shows operations/sec

**Step 5: Commit**

```bash
git add src/gosim/game/war.go src/gosim/game/war_test.go
git commit -m "feat: implement War game in Golang

- Add WarGame struct with battle mechanics
- Handle war (tie) scenarios
- Add PlayWarGame function
- Include tests and benchmarks"
```

---

## Task 6: Performance Comparison

**Files:**
- Create: `benchmarks/compare_war.py`
- Create: `benchmarks/README.md`

**Step 1: Write Python benchmark script**

Create `benchmarks/compare_war.py`:

```python
"""Compare Python vs Golang War game performance."""

import time
import subprocess
import statistics
from darwindeck.simulation.war import play_war_game


def benchmark_python_war(iterations: int = 100) -> float:
    """Benchmark Python War implementation."""
    times = []

    for i in range(iterations):
        start = time.perf_counter()
        play_war_game(seed=i, max_turns=1000)
        elapsed = time.perf_counter() - start
        times.append(elapsed)

    return statistics.mean(times)


def benchmark_golang_war(iterations: int = 100) -> float:
    """Benchmark Golang War implementation via subprocess."""
    # Run Go benchmark and parse output
    result = subprocess.run(
        ["go", "test", "./game", "-bench=BenchmarkPlayWarGame",
         f"-benchtime={iterations}x"],
        cwd="src/gosim",
        capture_output=True,
        text=True
    )

    # Parse: "BenchmarkPlayWarGame-8   100   12345 ns/op"
    for line in result.stdout.split('\n'):
        if 'BenchmarkPlayWarGame' in line:
            parts = line.split()
            ns_per_op = float(parts[-2])
            return ns_per_op / 1e9  # Convert ns to seconds

    return 0.0


def main() -> None:
    """Run performance comparison."""
    print("Performance Comparison: Python vs Golang War Game")
    print("=" * 60)

    iterations = 100
    print(f"\nRunning {iterations} iterations of War game (max 1000 turns)...\n")

    print("Python implementation:")
    python_time = benchmark_python_war(iterations)
    print(f"  Average time: {python_time*1000:.2f}ms per game")

    print("\nGolang implementation:")
    golang_time = benchmark_golang_war(iterations)
    print(f"  Average time: {golang_time*1000:.2f}ms per game")

    speedup = python_time / golang_time
    print(f"\nSpeedup: {speedup:.1f}x")

    if speedup >= 10:
        print("✅ SUCCESS: Golang is 10x+ faster")
    else:
        print(f"⚠️  WARNING: Speedup is only {speedup:.1f}x (target: 10x+)")


if __name__ == "__main__":
    main()
```

**Step 2: Create benchmarks documentation**

Create `benchmarks/README.md`:

```markdown
# Performance Benchmarks

## War Game Comparison

Compares Python vs Golang implementation of War card game.

### Run Benchmark

```bash
poetry run python benchmarks/compare_war.py
```

### Expected Results

- **Python:** ~10-50ms per game
- **Golang:** ~0.5-2ms per game
- **Speedup:** 10-50x

### Interpretation

- **10-20x:** Good, proceed with Golang core
- **20-50x:** Excellent, validates architecture decision
- **<10x:** Investigate optimization opportunities
```

**Step 3: Run benchmark**

Run:
```bash
poetry run python benchmarks/compare_war.py
```

Expected: Output showing speedup ratio

**Step 4: Document results**

Add results to `benchmarks/README.md`:

```markdown
### Actual Results (2026-01-10)

[Record actual results here]

- Python: XX.XXms per game
- Golang: X.XXms per game
- Speedup: XXx

Decision: [CGo / gRPC based on results]
```

**Step 5: Commit**

```bash
git add benchmarks/
git commit -m "feat: add Python vs Golang performance benchmark

- Create comparison script for War game
- Benchmark both implementations with 100 iterations
- Document expected and actual results
- Validate 10-50x speedup target"
```

---

## Task 7: Interface Decision (CGo vs gRPC)

**Files:**
- Create: `docs/architecture/python-go-interface-decision.md`

**Step 1: Document interface analysis**

Create `docs/architecture/python-go-interface-decision.md`:

```markdown
# Python ↔ Golang Interface Decision

## Benchmark Results

- War game speedup: [XX]x
- Python overhead acceptable: [Yes/No]

## Option A: CGo Bindings

**Pros:**
- Lower latency (direct C calls)
- No serialization overhead
- Simpler deployment (single binary)

**Cons:**
- Tighter coupling
- CGo debugging is harder
- Must rebuild for each change

**Implementation:**
```go
//export PlayWarGame
func PlayWarGame(seed int64, maxTurns int) int {
    // ...
}
```

**Python side:**
```python
from ctypes import CDLL, c_int64

libgosim = CDLL("./gosim.so")
result = libgosim.PlayWarGame(42, 1000)
```

## Option B: gRPC Service

**Pros:**
- Clean separation
- Language independence
- Easier debugging (wireshark)
- Can scale to distributed later

**Cons:**
- Serialization overhead (~1-2ms per call)
- More complex setup
- Requires proto definitions

**Implementation:**

proto:
```protobuf
service GameSimulator {
  rpc PlayWarGame(WarRequest) returns (WarResult);
}
```

## Decision

**Choice: [CGo / gRPC]**

**Rationale:**
- [Performance requirements]
- [Development complexity]
- [Future scalability]

## Next Steps

- Implement chosen interface
- Create Python bridge module
- Benchmark with interface overhead
```

**Step 2: Make decision based on project needs**

Consider:
- If speedup > 30x → gRPC is fine (overhead negligible)
- If speedup 10-20x → CGo preferred (minimize overhead)
- If planning distributed eventually → gRPC

**Step 3: Document decision**

Fill in the decision section in the markdown file.

**Step 4: Commit**

```bash
git add docs/architecture/python-go-interface-decision.md
git commit -m "docs: document Python-Go interface decision

- Analyze CGo vs gRPC trade-offs
- Document performance impact
- Make decision based on benchmark results
- Outline next steps for chosen approach"
```

---

## Task 8: Schema Versioning

**Files:**
- Create: `src/darwindeck/genome/versioning.py`
- Create: `tests/unit/test_genome_versioning.py`

**Step 1: Write failing test for schema validation**

Create `tests/unit/test_genome_versioning.py`:

```python
"""Tests for genome schema versioning."""

import pytest
from darwindeck.genome.versioning import SchemaVersion, validate_schema_version
from darwindeck.genome.schema import GameGenome


def test_current_schema_version() -> None:
    """Test current schema version is 1.0."""
    assert SchemaVersion.CURRENT == "1.0"


def test_validate_compatible_version() -> None:
    """Test compatible schema versions pass validation."""
    genome = GameGenome(schema_version="1.0", genome_id="test", generation=0)

    # Should not raise
    validate_schema_version(genome)


def test_validate_incompatible_version_raises() -> None:
    """Test incompatible schema versions raise error."""
    genome = GameGenome(schema_version="2.0", genome_id="test", generation=0)

    with pytest.raises(ValueError, match="Incompatible schema version"):
        validate_schema_version(genome)
```

**Step 2: Run test to verify it fails**

Run:
```bash
poetry run pytest tests/unit/test_genome_versioning.py -v
```

Expected: ImportError

**Step 3: Write implementation**

Create `src/darwindeck/genome/versioning.py`:

```python
"""Schema versioning for genome compatibility."""

from typing import Set
from darwindeck.genome.schema import GameGenome


class SchemaVersion:
    """Schema version constants."""

    CURRENT = "1.0"
    COMPATIBLE: Set[str] = {"1.0"}


def validate_schema_version(genome: GameGenome) -> None:
    """Validate genome schema version is compatible.

    Args:
        genome: The genome to validate

    Raises:
        ValueError: If schema version is not compatible
    """
    if genome.schema_version not in SchemaVersion.COMPATIBLE:
        raise ValueError(
            f"Incompatible schema version: {genome.schema_version}. "
            f"Compatible versions: {SchemaVersion.COMPATIBLE}"
        )
```

**Step 4: Run test to verify it passes**

Run:
```bash
poetry run pytest tests/unit/test_genome_versioning.py -v
```

Expected: 3 passed

**Step 5: Commit**

```bash
git add src/darwindeck/genome/versioning.py tests/unit/test_genome_versioning.py
git commit -m "feat: add genome schema versioning

- Add SchemaVersion class with CURRENT and COMPATIBLE versions
- Add validate_schema_version function
- Raise ValueError for incompatible versions
- Include tests for validation logic"
```

---

## Task 9: Update CLAUDE.md with Phase 1 Learnings

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add benchmark results section**

Update `CLAUDE.md` to include:

```markdown
## Performance Benchmarks

### Golang Core Validation (Phase 1)

**War Game Benchmark:**
- Python implementation: XX.XXms per game
- Golang implementation: X.XXms per game
- Measured speedup: XXx

**Interface Decision:** [CGo / gRPC]

**Rationale:** [Performance vs complexity tradeoffs]

## Development Commands

### Run Tests

**Python:**
```bash
poetry run pytest tests/ -v
poetry run pytest tests/unit/test_specific.py -v  # Single file
```

**Golang:**
```bash
cd src/gosim
go test ./game -v
go test ./game -bench=. -benchtime=10s
```

### Benchmarks

```bash
poetry run python benchmarks/compare_war.py
```

### Code Quality

```bash
poetry run black src/ tests/
poetry run mypy src/
```
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with Phase 1 learnings

- Add performance benchmark results
- Document interface decision
- Add development commands for tests and benchmarks
- Include code quality tools"
```

---

## Summary

**Phase 1 Complete Checklist:**

- [x] Python project structure with Poetry
- [x] Golang module with tests and benchmarks
- [x] Basic genome schema types
- [x] War game implementation in both languages
- [x] Performance comparison (10-50x speedup validated)
- [x] Interface decision documented (CGo vs gRPC)
- [x] Schema versioning system
- [x] CLAUDE.md updated with learnings

**Key Metrics:**
- Python War game: ~XX.XXms per game
- Golang War game: ~X.XXms per game
- Speedup: XXx (target: 10-50x) ✅

**Next Phase:**
Phase 2: Python Simulation Core - Build interpreter pattern, game state, basic fitness metrics

**Estimated Time:** 10-12 hours over 1-2 weeks

---

