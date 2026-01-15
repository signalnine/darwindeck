# Tableau Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Make tableau card interactions explicit in the genome schema with four modes: NONE, WAR, MATCH_RANK, SEQUENCE.

**Architecture:** Add `TableauMode` and `SequenceDirection` enums to Python schema, encode in bytecode with version byte, update Go simulator to use explicit mode instead of inference, update rulebook generator and mutations.

**Tech Stack:** Python 3.13, Go 1.21, pytest, go test

**Design Document:** `docs/plans/2026-01-15-tableau-mode-design.md`

---

## Task 1: Add Python Enums

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Test: `tests/unit/test_schema.py`

**Step 1: Write the failing test**

```python
# tests/unit/test_schema.py - add to existing file

def test_tableau_mode_enum_values():
    """TableauMode enum has expected values."""
    from darwindeck.genome.schema import TableauMode

    assert TableauMode.NONE.value == "none"
    assert TableauMode.WAR.value == "war"
    assert TableauMode.MATCH_RANK.value == "match_rank"
    assert TableauMode.SEQUENCE.value == "sequence"


def test_sequence_direction_enum_values():
    """SequenceDirection enum has expected values."""
    from darwindeck.genome.schema import SequenceDirection

    assert SequenceDirection.ASCENDING.value == "ascending"
    assert SequenceDirection.DESCENDING.value == "descending"
    assert SequenceDirection.BOTH.value == "both"
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_schema.py::test_tableau_mode_enum_values -v
```

Expected: FAIL with "cannot import name 'TableauMode'"

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/schema.py` after the `BettingAction` enum:

```python
class TableauMode(Enum):
    """How cards on the tableau interact."""
    NONE = "none"              # Cards accumulate, no interaction
    WAR = "war"                # Compare cards, winner takes all (2-player only)
    MATCH_RANK = "match_rank"  # Matching rank captures
    SEQUENCE = "sequence"      # Build ascending/descending piles


class SequenceDirection(Enum):
    """Direction for SEQUENCE tableau mode."""
    ASCENDING = "ascending"
    DESCENDING = "descending"
    BOTH = "both"
```

**Step 4: Run tests to verify they pass**

```bash
uv run pytest tests/unit/test_schema.py::test_tableau_mode_enum_values tests/unit/test_schema.py::test_sequence_direction_enum_values -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_schema.py
git commit -m "feat(schema): add TableauMode and SequenceDirection enums"
```

---

## Task 2: Add SetupRules Fields

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Test: `tests/unit/test_schema.py`

**Step 1: Write the failing test**

```python
# tests/unit/test_schema.py - add to existing file

def test_setup_rules_tableau_mode_default():
    """SetupRules defaults tableau_mode to NONE."""
    from darwindeck.genome.schema import SetupRules, TableauMode, SequenceDirection

    setup = SetupRules(cards_per_player=7)
    assert setup.tableau_mode == TableauMode.NONE
    assert setup.sequence_direction == SequenceDirection.BOTH


def test_setup_rules_tableau_mode_explicit():
    """SetupRules accepts explicit tableau_mode."""
    from darwindeck.genome.schema import SetupRules, TableauMode, SequenceDirection

    setup = SetupRules(
        cards_per_player=7,
        tableau_mode=TableauMode.WAR,
        sequence_direction=SequenceDirection.ASCENDING
    )
    assert setup.tableau_mode == TableauMode.WAR
    assert setup.sequence_direction == SequenceDirection.ASCENDING
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_schema.py::test_setup_rules_tableau_mode_default -v
```

Expected: FAIL with "unexpected keyword argument 'tableau_mode'"

**Step 3: Write minimal implementation**

Modify `SetupRules` in `src/darwindeck/genome/schema.py`:

```python
@dataclass(frozen=True)
class SetupRules:
    """Initial game configuration."""

    cards_per_player: int
    initial_deck: str = "standard_52"
    initial_discard_count: int = 0
    wild_cards: tuple[Rank, ...] = ()
    hand_visibility: Visibility = Visibility.OWNER_ONLY
    deck_visibility: Visibility = Visibility.FACE_DOWN
    discard_visibility: Visibility = Visibility.FACE_UP
    trump_suit: Optional[Suit] = None
    rotate_trump: bool = False
    random_trump: bool = False
    starting_chips: int = 0
    custom_printed_deck: bool = False
    # NEW: Tableau interaction mode
    tableau_mode: TableauMode = TableauMode.NONE
    sequence_direction: SequenceDirection = SequenceDirection.BOTH

    def __post_init__(self):
        """Convert lists to tuples for immutability."""
        if isinstance(self.wild_cards, list):
            object.__setattr__(self, "wild_cards", tuple(self.wild_cards))
```

**Step 4: Run tests to verify they pass**

```bash
uv run pytest tests/unit/test_schema.py::test_setup_rules_tableau_mode_default tests/unit/test_schema.py::test_setup_rules_tableau_mode_explicit -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_schema.py
git commit -m "feat(schema): add tableau_mode and sequence_direction to SetupRules"
```

---

## Task 3: Update JSON Serialization

**Files:**
- Modify: `src/darwindeck/genome/serialization.py`
- Test: `tests/unit/test_serialization.py`

**Step 1: Write the failing test**

```python
# tests/unit/test_serialization.py - add to existing file

def test_genome_serialization_with_tableau_mode():
    """Genome with tableau_mode serializes and deserializes correctly."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition,
        TableauMode, SequenceDirection
    )
    from darwindeck.genome.serialization import genome_to_dict, genome_from_dict

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test_war",
        generation=1,
        setup=SetupRules(
            cards_per_player=26,
            tableau_mode=TableauMode.WAR,
        ),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="capture_all")],
        scoring_rules=[],
    )

    # Round-trip
    d = genome_to_dict(genome)
    restored = genome_from_dict(d)

    assert restored.setup.tableau_mode == TableauMode.WAR
    assert restored.setup.sequence_direction == SequenceDirection.BOTH


def test_genome_deserialization_missing_tableau_mode():
    """Genome without tableau_mode defaults to NONE."""
    from darwindeck.genome.schema import TableauMode, SequenceDirection
    from darwindeck.genome.serialization import genome_from_dict

    # Old-style genome dict without tableau_mode
    d = {
        "schema_version": "1.0",
        "genome_id": "old_game",
        "generation": 1,
        "setup": {"cards_per_player": 7},
        "turn_structure": {"phases": []},
        "special_effects": [],
        "win_conditions": [{"type": "empty_hand"}],
        "scoring_rules": [],
    }

    genome = genome_from_dict(d)
    assert genome.setup.tableau_mode == TableauMode.NONE
    assert genome.setup.sequence_direction == SequenceDirection.BOTH
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_serialization.py::test_genome_serialization_with_tableau_mode -v
```

Expected: FAIL (either key missing in dict or deserialization fails)

**Step 3: Write minimal implementation**

Modify `src/darwindeck/genome/serialization.py`:

In `setup_to_dict()` function, add:
```python
if setup.tableau_mode != TableauMode.NONE:
    d["tableau_mode"] = setup.tableau_mode.value
if setup.sequence_direction != SequenceDirection.BOTH:
    d["sequence_direction"] = setup.sequence_direction.value
```

In `setup_from_dict()` function, add:
```python
tableau_mode = TableauMode(d.get("tableau_mode", "none"))
sequence_direction = SequenceDirection(d.get("sequence_direction", "both"))
```

And include in SetupRules construction:
```python
tableau_mode=tableau_mode,
sequence_direction=sequence_direction,
```

Also add imports at top:
```python
from darwindeck.genome.schema import TableauMode, SequenceDirection
```

**Step 4: Run tests to verify they pass**

```bash
uv run pytest tests/unit/test_serialization.py::test_genome_serialization_with_tableau_mode tests/unit/test_serialization.py::test_genome_deserialization_missing_tableau_mode -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/serialization.py tests/unit/test_serialization.py
git commit -m "feat(serialization): handle tableau_mode in JSON serialization"
```

---

## Task 4: Update Bytecode Compiler - Version Byte

**Files:**
- Modify: `src/darwindeck/genome/bytecode.py`
- Test: `tests/unit/test_bytecode.py`

**Step 1: Write the failing test**

```python
# tests/unit/test_bytecode.py - add to existing file

def test_bytecode_version_header():
    """Bytecode starts with version byte."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, TableauMode
    )
    from darwindeck.genome.bytecode import compile_genome

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.WAR),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    bytecode = compile_genome(genome)

    # First byte should be version 2
    assert bytecode[0] == 2


def test_bytecode_tableau_mode_encoding():
    """Bytecode encodes tableau_mode at correct offset."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition,
        TableauMode, SequenceDirection
    )
    from darwindeck.genome.bytecode import compile_genome

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(
            cards_per_player=7,
            tableau_mode=TableauMode.SEQUENCE,
            sequence_direction=SequenceDirection.DESCENDING
        ),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    bytecode = compile_genome(genome)

    # Offset 37: tableau_mode (3=SEQUENCE)
    # Offset 38: sequence_direction (1=DESCENDING)
    assert bytecode[37] == 3  # SEQUENCE
    assert bytecode[38] == 1  # DESCENDING
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_bytecode.py::test_bytecode_version_header -v
```

Expected: FAIL (version byte not present or wrong value)

**Step 3: Write minimal implementation**

Modify `src/darwindeck/genome/bytecode.py`:

Update `compile_genome()` to:
1. Start header with version byte (2)
2. Shift all existing fields by 1 byte
3. Add tableau_mode at offset 37 and sequence_direction at offset 38

```python
BYTECODE_VERSION = 2
HEADER_SIZE = 39  # Was 36, now 39

# TableauMode encoding
TABLEAU_MODE_MAP = {
    TableauMode.NONE: 0,
    TableauMode.WAR: 1,
    TableauMode.MATCH_RANK: 2,
    TableauMode.SEQUENCE: 3,
}

SEQUENCE_DIRECTION_MAP = {
    SequenceDirection.ASCENDING: 0,
    SequenceDirection.DESCENDING: 1,
    SequenceDirection.BOTH: 2,
}
```

In the header compilation section:
```python
# Byte 0: Version
header.append(BYTECODE_VERSION)

# Bytes 1-36: Existing header fields (shifted by 1)
# ... existing code but offsets +1 ...

# Byte 37: tableau_mode
header.append(TABLEAU_MODE_MAP.get(genome.setup.tableau_mode, 0))

# Byte 38: sequence_direction
header.append(SEQUENCE_DIRECTION_MAP.get(genome.setup.sequence_direction, 2))
```

**Step 4: Run tests to verify they pass**

```bash
uv run pytest tests/unit/test_bytecode.py::test_bytecode_version_header tests/unit/test_bytecode.py::test_bytecode_tableau_mode_encoding -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/bytecode.py tests/unit/test_bytecode.py
git commit -m "feat(bytecode): add version byte and tableau_mode encoding"
```

---

## Task 5: Update Go Types

**Files:**
- Modify: `src/gosim/engine/types.go`
- Test: `src/gosim/engine/types_test.go`

**Step 1: Write the failing test**

```go
// src/gosim/engine/types_test.go - add to existing file

func TestGameStateHasTableauMode(t *testing.T) {
    state := NewGameState(2)

    // Default should be 0 (NONE)
    if state.TableauMode != 0 {
        t.Errorf("Expected TableauMode 0, got %d", state.TableauMode)
    }

    // Should be settable
    state.TableauMode = 1 // WAR
    if state.TableauMode != 1 {
        t.Errorf("Expected TableauMode 1, got %d", state.TableauMode)
    }

    state.SequenceDirection = 2 // BOTH
    if state.SequenceDirection != 2 {
        t.Errorf("Expected SequenceDirection 2, got %d", state.SequenceDirection)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd src/gosim && go test ./engine -run TestGameStateHasTableauMode -v
```

Expected: FAIL with "state.TableauMode undefined"

**Step 3: Write minimal implementation**

Modify `src/gosim/engine/types.go`:

Remove the `CaptureMode` field and add:

```go
// In GameState struct:
TableauMode       uint8       // 0=NONE, 1=WAR, 2=MATCH_RANK, 3=SEQUENCE
SequenceDirection uint8       // 0=ASC, 1=DESC, 2=BOTH
```

Update `NewGameState()` to initialize these (defaults to 0 which is NONE).

Update `Reset()` method:
```go
s.TableauMode = 0
s.SequenceDirection = 0
```

Update `Clone()` method:
```go
clone.TableauMode = s.TableauMode
clone.SequenceDirection = s.SequenceDirection
```

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./engine -run TestGameStateHasTableauMode -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/types.go src/gosim/engine/types_test.go
git commit -m "feat(gosim): add TableauMode to GameState, remove CaptureMode"
```

---

## Task 6: Update Go Bytecode Parsing

**Files:**
- Modify: `src/gosim/engine/bytecode.go`
- Test: `src/gosim/engine/bytecode_test.go`

**Step 1: Write the failing test**

```go
// src/gosim/engine/bytecode_test.go - add to existing file

func TestParseGenomeVersion2(t *testing.T) {
    // Minimal v2 bytecode: version + header + tableau fields
    bytecode := make([]byte, 39)
    bytecode[0] = 2  // Version 2
    bytecode[37] = 1 // TableauMode = WAR
    bytecode[38] = 0 // SequenceDirection = ASCENDING

    genome, err := ParseGenome(bytecode)
    if err != nil {
        t.Fatalf("ParseGenome failed: %v", err)
    }

    if genome.Header.TableauMode != 1 {
        t.Errorf("Expected TableauMode 1, got %d", genome.Header.TableauMode)
    }
    if genome.Header.SequenceDirection != 0 {
        t.Errorf("Expected SequenceDirection 0, got %d", genome.Header.SequenceDirection)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd src/gosim && go test ./engine -run TestParseGenomeVersion2 -v
```

Expected: FAIL (field doesn't exist or parse fails)

**Step 3: Write minimal implementation**

Modify `src/gosim/engine/bytecode.go`:

Add to `GenomeHeader` struct:
```go
BytecodeVersion   uint8
TableauMode       uint8
SequenceDirection uint8
```

Update `ParseGenome()`:
```go
func ParseGenome(bytecode []byte) (*Genome, error) {
    if len(bytecode) < 1 {
        return nil, fmt.Errorf("empty bytecode")
    }

    version := bytecode[0]

    if version == 2 {
        return parseV2Genome(bytecode)
    }

    // Assume v1 (old format without version byte)
    return parseV1Genome(bytecode)
}

func parseV2Genome(bytecode []byte) (*Genome, error) {
    if len(bytecode) < 39 {
        return nil, fmt.Errorf("v2 bytecode too short: %d < 39", len(bytecode))
    }

    header := GenomeHeader{
        BytecodeVersion:   bytecode[0],
        // Parse existing fields at offset+1
        // ...
        TableauMode:       bytecode[37],
        SequenceDirection: bytecode[38],
    }

    // ... rest of parsing
}

func parseV1Genome(bytecode []byte) (*Genome, error) {
    // Original parsing logic, defaults TableauMode to 0
    // ...
}
```

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./engine -run TestParseGenomeVersion2 -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/bytecode.go src/gosim/engine/bytecode_test.go
git commit -m "feat(gosim): parse bytecode version and tableau_mode"
```

---

## Task 7: Update Go Runner to Read Tableau Mode

**Files:**
- Modify: `src/gosim/simulation/runner.go`
- Test: `src/gosim/simulation/runner_test.go`

**Step 1: Write the failing test**

```go
// src/gosim/simulation/runner_test.go - add to existing file

func TestRunnerSetsTableauMode(t *testing.T) {
    // Create a v2 bytecode with WAR mode
    bytecode := makeV2Bytecode(1) // TableauMode = WAR

    genome, _ := engine.ParseGenome(bytecode)

    // Run a single game and check state was initialized correctly
    // (This requires exposing state or checking behavior)
    result := RunSingleGame(genome, RandomAI, 12345)

    // If WAR mode is set, games should have captures happening
    // For now just verify no error
    if result.Error != "" {
        t.Errorf("Game errored: %s", result.Error)
    }
}
```

**Step 2: Run test to verify behavior**

```bash
cd src/gosim && go test ./simulation -run TestRunnerSetsTableauMode -v
```

**Step 3: Write minimal implementation**

Modify `src/gosim/simulation/runner.go`:

Remove the CaptureMode inference logic:
```go
// DELETE THIS:
// for _, wc := range genome.WinConditions {
//     if wc.WinType == 7 { // most_captured
//         state.CaptureMode = true
//         break
//     }
// }
```

Replace with explicit tableau mode from header:
```go
// Set tableau mode from genome header
state.TableauMode = genome.Header.TableauMode
state.SequenceDirection = genome.Header.SequenceDirection
```

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./simulation -v
```

Expected: PASS (all existing tests still work)

**Step 5: Commit**

```bash
git add src/gosim/simulation/runner.go src/gosim/simulation/runner_test.go
git commit -m "feat(gosim): read tableau_mode from genome header in runner"
```

---

## Task 8: Update Go movegen to Use Explicit Mode

**Files:**
- Modify: `src/gosim/engine/movegen.go`
- Test: `src/gosim/engine/movegen_test.go`

**Step 1: Write the failing test**

```go
// src/gosim/engine/movegen_test.go - add to existing file

func TestApplyMoveTableauModeNone(t *testing.T) {
    state := NewGameState(2)
    state.TableauMode = 0 // NONE

    // Setup: give player cards
    state.Players[0].Hand = []Card{{Rank: 10, Suit: 0}}
    state.Players[1].Hand = []Card{{Rank: 5, Suit: 0}}

    // Play cards to tableau
    move := &LegalMove{PhaseIndex: 0, CardIndex: 0, TargetLoc: LocationTableau}
    state.CurrentPlayer = 0
    ApplyMove(state, move, minimalPlayPhaseGenome())

    state.CurrentPlayer = 1
    ApplyMove(state, move, minimalPlayPhaseGenome())

    // With NONE mode, cards should stay on tableau (no battle)
    if len(state.Tableau) == 0 || len(state.Tableau[0]) != 2 {
        t.Errorf("Expected 2 cards on tableau, got %v", state.Tableau)
    }
}

func TestApplyMoveTableauModeWar(t *testing.T) {
    state := NewGameState(2)
    state.TableauMode = 1 // WAR

    // Setup: give player cards
    state.Players[0].Hand = []Card{{Rank: 10, Suit: 0}}
    state.Players[1].Hand = []Card{{Rank: 5, Suit: 0}}

    // Play cards to tableau
    move := &LegalMove{PhaseIndex: 0, CardIndex: 0, TargetLoc: LocationTableau}
    state.CurrentPlayer = 0
    ApplyMove(state, move, minimalPlayPhaseGenome())

    state.CurrentPlayer = 1
    ApplyMove(state, move, minimalPlayPhaseGenome())

    // With WAR mode, player 0 should have won both cards
    if len(state.Players[0].Hand) != 2 {
        t.Errorf("Expected player 0 to have 2 cards, got %d", len(state.Players[0].Hand))
    }
    if len(state.Tableau) > 0 && len(state.Tableau[0]) != 0 {
        t.Errorf("Expected empty tableau, got %v", state.Tableau)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd src/gosim && go test ./engine -run TestApplyMoveTableauModeNone -v
```

Expected: FAIL (cards captured even with NONE mode)

**Step 3: Write minimal implementation**

Modify `src/gosim/engine/movegen.go` in `ApplyMove()`:

Replace the current logic:
```go
// OLD:
if move.TargetLoc == LocationTableau {
    if state.CaptureMode {
        resolveScopaCapture(state, currentPlayer, playedCard)
    } else if state.NumPlayers == 2 {
        resolveWarBattle(state)
    }
}
```

With explicit mode check:
```go
// NEW:
if move.TargetLoc == LocationTableau {
    switch state.TableauMode {
    case 1: // WAR
        resolveWarBattle(state)
    case 2: // MATCH_RANK
        resolveScopaCapture(state, currentPlayer, playedCard)
    case 3: // SEQUENCE
        // Validation done in move generation
    }
    // case 0 (NONE): do nothing
}
```

Also rename `resolveScopaCapture` to `resolveMatchRankCapture` for consistency.

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./engine -run "TestApplyMoveTableauMode" -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/movegen.go src/gosim/engine/movegen_test.go
git commit -m "feat(gosim): use explicit TableauMode in ApplyMove"
```

---

## Task 9: Add SEQUENCE Mode Move Generation

**Files:**
- Modify: `src/gosim/engine/movegen.go`
- Test: `src/gosim/engine/movegen_test.go`

**Step 1: Write the failing test**

```go
// src/gosim/engine/movegen_test.go - add to existing file

func TestSequenceModeValidMoves(t *testing.T) {
    state := NewGameState(2)
    state.TableauMode = 3            // SEQUENCE
    state.SequenceDirection = 0      // ASCENDING

    // Empty tableau: any card can be played
    state.Players[0].Hand = []Card{{Rank: 7, Suit: 0}}
    state.CurrentPlayer = 0

    genome := sequencePhaseGenome()
    moves := GenerateLegalMoves(state, genome)

    if len(moves) != 1 {
        t.Errorf("Expected 1 move on empty tableau, got %d", len(moves))
    }

    // Play the 7
    ApplyMove(state, &moves[0], genome)

    // Now only 8 should be valid (ascending from 7)
    state.Players[0].Hand = []Card{
        {Rank: 6, Suit: 0},  // Invalid (descending)
        {Rank: 8, Suit: 0},  // Valid (ascending)
        {Rank: 9, Suit: 0},  // Invalid (not adjacent)
    }

    moves = GenerateLegalMoves(state, genome)

    if len(moves) != 1 {
        t.Errorf("Expected 1 valid move, got %d", len(moves))
    }
    if moves[0].CardIndex != 1 { // Index of rank 8
        t.Errorf("Expected card index 1 (rank 8), got %d", moves[0].CardIndex)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd src/gosim && go test ./engine -run TestSequenceModeValidMoves -v
```

Expected: FAIL (SEQUENCE not implemented)

**Step 3: Write minimal implementation**

Add to `src/gosim/engine/movegen.go`:

```go
// isValidSequencePlay checks if card can be played on pile
func isValidSequencePlay(card Card, topCard Card, direction uint8) bool {
    // Must match suit
    if card.Suit != topCard.Suit {
        return false
    }

    switch direction {
    case 0: // ASCENDING
        return card.Rank == topCard.Rank+1
    case 1: // DESCENDING
        return card.Rank == topCard.Rank-1
    case 2: // BOTH
        return card.Rank == topCard.Rank+1 || card.Rank == topCard.Rank-1
    }
    return false
}
```

In `GenerateLegalMoves()` case 2 (PlayPhase), add SEQUENCE handling:
```go
// If SEQUENCE mode, filter to valid sequence plays
if state.TableauMode == 3 && target == LocationTableau {
    if len(state.Tableau) == 0 || len(state.Tableau[0]) == 0 {
        // Empty tableau: any card valid
        moves = append(moves, LegalMove{...})
    } else {
        // Check each card against tableau piles
        for cardIdx, card := range hand {
            for _, pile := range state.Tableau {
                if len(pile) > 0 {
                    topCard := pile[len(pile)-1]
                    if isValidSequencePlay(card, topCard, state.SequenceDirection) {
                        moves = append(moves, LegalMove{
                            PhaseIndex: phaseIdx,
                            CardIndex:  cardIdx,
                            TargetLoc:  target,
                        })
                    }
                }
            }
        }
    }
    continue // Skip normal play logic
}
```

**Step 4: Run tests to verify they pass**

```bash
cd src/gosim && go test ./engine -run TestSequenceModeValidMoves -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/movegen.go src/gosim/engine/movegen_test.go
git commit -m "feat(gosim): implement SEQUENCE mode move generation"
```

---

## Task 10: Update Rulebook Generator

**Files:**
- Modify: `src/darwindeck/evolution/rulebook.py`
- Test: `tests/unit/test_rulebook.py`

**Step 1: Write the failing test**

```python
# tests/unit/test_rulebook.py - add to existing file

def test_rulebook_describes_war_mode():
    """Rulebook includes WAR mode description."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, PlayPhase,
        WinCondition, Location, TableauMode
    )
    from darwindeck.evolution.rulebook import RulebookGenerator

    genome = GameGenome(
        schema_version="1.0",
        genome_id="WarGame",
        generation=1,
        setup=SetupRules(cards_per_player=26, tableau_mode=TableauMode.WAR),
        turn_structure=TurnStructure(phases=[
            PlayPhase(target=Location.TABLEAU)
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="capture_all")],
        scoring_rules=[],
    )

    generator = RulebookGenerator()
    rulebook = generator.generate(genome, use_llm=False)

    assert "compare ranks" in rulebook.lower()
    assert "winner takes" in rulebook.lower() or "wins both" in rulebook.lower()


def test_rulebook_describes_sequence_mode():
    """Rulebook includes SEQUENCE mode description."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, PlayPhase,
        WinCondition, Location, TableauMode, SequenceDirection
    )
    from darwindeck.evolution.rulebook import RulebookGenerator

    genome = GameGenome(
        schema_version="1.0",
        genome_id="SequenceGame",
        generation=1,
        setup=SetupRules(
            cards_per_player=7,
            tableau_mode=TableauMode.SEQUENCE,
            sequence_direction=SequenceDirection.ASCENDING
        ),
        turn_structure=TurnStructure(phases=[
            PlayPhase(target=Location.TABLEAU)
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    generator = RulebookGenerator()
    rulebook = generator.generate(genome, use_llm=False)

    assert "ascending" in rulebook.lower()
    assert "sequence" in rulebook.lower() or "order" in rulebook.lower()
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_rulebook.py::test_rulebook_describes_war_mode -v
```

Expected: FAIL (no tableau mode description in output)

**Step 3: Write minimal implementation**

Modify `src/darwindeck/evolution/rulebook.py`:

Add method to `GenomeExtractor`:
```python
def _get_tableau_mode_description(self, genome: "GameGenome") -> str:
    """Get description for tableau mode."""
    from darwindeck.genome.schema import TableauMode, SequenceDirection

    mode = genome.setup.tableau_mode

    if mode == TableauMode.NONE:
        return ""
    elif mode == TableauMode.WAR:
        return "When both players have played, compare ranks—the higher card wins both cards."
    elif mode == TableauMode.MATCH_RANK:
        return "If your card matches a card on the tableau by rank, capture both cards."
    elif mode == TableauMode.SEQUENCE:
        direction = genome.setup.sequence_direction
        if direction == SequenceDirection.ASCENDING:
            return "Play cards in ascending order to build on tableau piles."
        elif direction == SequenceDirection.DESCENDING:
            return "Play cards in descending order to build on tableau piles."
        else:
            return "Play cards in sequence (ascending or descending) to build on tableau piles."
    return ""
```

Update `_describe_phase()` to include tableau context when target is TABLEAU:
```python
elif isinstance(phase, PlayPhase):
    target = "discard pile" if phase.target == Location.DISCARD else "tableau"
    # ... existing description logic ...

    # Add tableau mode context if playing to tableau
    if phase.target == Location.TABLEAU and genome is not None:
        tableau_desc = self._get_tableau_mode_description(genome)
        if tableau_desc:
            desc = f"{desc} {tableau_desc}"

    return ("Play", desc)
```

**Step 4: Run tests to verify they pass**

```bash
uv run pytest tests/unit/test_rulebook.py::test_rulebook_describes_war_mode tests/unit/test_rulebook.py::test_rulebook_describes_sequence_mode -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/rulebook.py tests/unit/test_rulebook.py
git commit -m "feat(rulebook): add tableau mode descriptions"
```

---

## Task 11: Add Tableau Mode Mutations

**Files:**
- Modify: `src/darwindeck/evolution/mutations.py`
- Test: `tests/unit/test_mutations.py`

**Step 1: Write the failing test**

```python
# tests/unit/test_mutations.py - add to existing file

def test_mutate_tableau_mode():
    """MutateTableauModeMutation changes tableau mode."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, TableauMode
    )
    from darwindeck.evolution.mutations import MutateTableauModeMutation
    import random

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.NONE),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=2,
    )

    mutation = MutateTableauModeMutation()
    rng = random.Random(42)

    mutated = mutation.apply(genome, rng)

    # Mode should have changed
    assert mutated.setup.tableau_mode != TableauMode.NONE


def test_mutate_tableau_mode_war_requires_2_players():
    """WAR mode mutation only applies to 2-player games."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, TableauMode
    )
    from darwindeck.evolution.mutations import MutateTableauModeMutation
    import random

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.NONE),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=4,  # 4 players
    )

    mutation = MutateTableauModeMutation()
    rng = random.Random(42)

    # Try multiple times - should never get WAR
    for _ in range(20):
        mutated = mutation.apply(genome, rng)
        assert mutated.setup.tableau_mode != TableauMode.WAR
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_mutations.py::test_mutate_tableau_mode -v
```

Expected: FAIL with "cannot import name 'MutateTableauModeMutation'"

**Step 3: Write minimal implementation**

Add to `src/darwindeck/evolution/mutations.py`:

```python
from darwindeck.genome.schema import TableauMode, SequenceDirection

class MutateTableauModeMutation(Mutation):
    """Change the tableau interaction mode."""

    weight = 0.5  # Low weight - significant change

    def apply(self, genome: GameGenome, rng: random.Random) -> GameGenome:
        # Get valid modes for this player count
        valid_modes = [TableauMode.NONE, TableauMode.MATCH_RANK, TableauMode.SEQUENCE]
        if genome.player_count == 2:
            valid_modes.append(TableauMode.WAR)

        # Remove current mode
        valid_modes = [m for m in valid_modes if m != genome.setup.tableau_mode]

        if not valid_modes:
            return genome

        new_mode = rng.choice(valid_modes)

        new_setup = SetupRules(
            cards_per_player=genome.setup.cards_per_player,
            initial_deck=genome.setup.initial_deck,
            initial_discard_count=genome.setup.initial_discard_count,
            wild_cards=genome.setup.wild_cards,
            hand_visibility=genome.setup.hand_visibility,
            deck_visibility=genome.setup.deck_visibility,
            discard_visibility=genome.setup.discard_visibility,
            trump_suit=genome.setup.trump_suit,
            rotate_trump=genome.setup.rotate_trump,
            random_trump=genome.setup.random_trump,
            starting_chips=genome.setup.starting_chips,
            custom_printed_deck=genome.setup.custom_printed_deck,
            tableau_mode=new_mode,
            sequence_direction=genome.setup.sequence_direction,
        )

        return GameGenome(
            schema_version=genome.schema_version,
            genome_id=genome.genome_id,
            generation=genome.generation,
            setup=new_setup,
            turn_structure=genome.turn_structure,
            special_effects=genome.special_effects,
            win_conditions=genome.win_conditions,
            scoring_rules=genome.scoring_rules,
            max_turns=genome.max_turns,
            player_count=genome.player_count,
            min_turns=genome.min_turns,
        )


class MutateSequenceDirectionMutation(Mutation):
    """Change the sequence direction (only when mode is SEQUENCE)."""

    weight = 0.3  # Low weight

    def apply(self, genome: GameGenome, rng: random.Random) -> GameGenome:
        if genome.setup.tableau_mode != TableauMode.SEQUENCE:
            return genome  # No-op if not sequence mode

        directions = [SequenceDirection.ASCENDING, SequenceDirection.DESCENDING, SequenceDirection.BOTH]
        directions = [d for d in directions if d != genome.setup.sequence_direction]

        new_direction = rng.choice(directions)

        new_setup = SetupRules(
            # ... copy all fields ...
            tableau_mode=genome.setup.tableau_mode,
            sequence_direction=new_direction,
        )

        return GameGenome(
            # ... copy all fields with new_setup ...
        )
```

Also add to `ALL_MUTATIONS` list.

**Step 4: Run tests to verify they pass**

```bash
uv run pytest tests/unit/test_mutations.py::test_mutate_tableau_mode tests/unit/test_mutations.py::test_mutate_tableau_mode_war_requires_2_players -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/mutations.py tests/unit/test_mutations.py
git commit -m "feat(mutations): add tableau mode mutation operators"
```

---

## Task 12: Add Coherence Penalties to Fitness

**Files:**
- Modify: `src/darwindeck/evolution/fitness_full.py`
- Test: `tests/unit/test_fitness.py`

**Step 1: Write the failing test**

```python
# tests/unit/test_fitness.py - add to existing file

def test_war_mode_with_empty_hand_gets_penalty():
    """WAR mode + empty_hand win condition gets coherence penalty."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, PlayPhase,
        WinCondition, Location, TableauMode
    )
    from darwindeck.evolution.fitness_full import calculate_coherence_penalty

    genome = GameGenome(
        schema_version="1.0",
        genome_id="incoherent",
        generation=1,
        setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.WAR),
        turn_structure=TurnStructure(phases=[
            PlayPhase(target=Location.TABLEAU)
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],  # Conflict!
        scoring_rules=[],
    )

    penalty = calculate_coherence_penalty(genome)

    assert penalty > 0  # Should have penalty
    assert penalty >= 0.3  # At least 30%
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_fitness.py::test_war_mode_with_empty_hand_gets_penalty -v
```

Expected: FAIL with "cannot import name 'calculate_coherence_penalty'"

**Step 3: Write minimal implementation**

Add to `src/darwindeck/evolution/fitness_full.py`:

```python
from darwindeck.genome.schema import TableauMode

def calculate_coherence_penalty(genome: "GameGenome") -> float:
    """Calculate fitness penalty for incoherent tableau mode + win condition combos."""
    penalty = 0.0

    win_types = {wc.type for wc in genome.win_conditions}
    mode = genome.setup.tableau_mode

    # WAR (accumulation) conflicts with empty_hand (shedding)
    if mode == TableauMode.WAR and "empty_hand" in win_types:
        penalty += 0.30

    # MATCH_RANK (partial capture) conflicts with capture_all (total capture)
    if mode == TableauMode.MATCH_RANK and "capture_all" in win_types:
        penalty += 0.20

    # SEQUENCE (shedding) conflicts with capture_all (accumulation)
    if mode == TableauMode.SEQUENCE and "capture_all" in win_types:
        penalty += 0.30

    return min(penalty, 0.50)  # Cap at 50%
```

In `_compute_metrics()`, add to quality gates section:
```python
# COHERENCE PENALTY: Penalize incoherent tableau mode + win condition combos
coherence_penalty = calculate_coherence_penalty(genome)
quality_multiplier *= (1.0 - coherence_penalty)
```

**Step 4: Run tests to verify they pass**

```bash
uv run pytest tests/unit/test_fitness.py::test_war_mode_with_empty_hand_gets_penalty -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/fitness_full.py tests/unit/test_fitness.py
git commit -m "feat(fitness): add coherence penalty for tableau mode conflicts"
```

---

## Task 13: Update Seed Genomes

**Files:**
- Modify: `src/darwindeck/genome/examples.py`
- Test: `tests/unit/test_examples.py`

**Step 1: Write the failing test**

```python
# tests/unit/test_examples.py - add or create file

def test_war_genome_has_war_mode():
    """War seed genome has explicit WAR tableau mode."""
    from darwindeck.genome.examples import create_war_genome
    from darwindeck.genome.schema import TableauMode

    genome = create_war_genome()
    assert genome.setup.tableau_mode == TableauMode.WAR


def test_scopa_genome_has_match_rank_mode():
    """Scopa seed genome has MATCH_RANK tableau mode."""
    from darwindeck.genome.examples import create_scopa_genome
    from darwindeck.genome.schema import TableauMode

    genome = create_scopa_genome()
    assert genome.setup.tableau_mode == TableauMode.MATCH_RANK


def test_fantan_genome_has_sequence_mode():
    """Fan Tan seed genome has SEQUENCE tableau mode."""
    from darwindeck.genome.examples import create_fan_tan_genome
    from darwindeck.genome.schema import TableauMode, SequenceDirection

    genome = create_fan_tan_genome()
    assert genome.setup.tableau_mode == TableauMode.SEQUENCE
    assert genome.setup.sequence_direction == SequenceDirection.BOTH
```

**Step 2: Run test to verify it fails**

```bash
uv run pytest tests/unit/test_examples.py::test_war_genome_has_war_mode -v
```

Expected: FAIL (tableau_mode not set or wrong value)

**Step 3: Write minimal implementation**

Update each seed genome in `src/darwindeck/genome/examples.py`:

For `create_war_genome()`:
```python
setup=SetupRules(
    cards_per_player=26,
    tableau_mode=TableauMode.WAR,  # ADD THIS
),
```

For `create_scopa_genome()`:
```python
setup=SetupRules(
    cards_per_player=3,
    tableau_mode=TableauMode.MATCH_RANK,  # ADD THIS
),
```

For `create_fan_tan_genome()`:
```python
setup=SetupRules(
    cards_per_player=13,
    tableau_mode=TableauMode.SEQUENCE,  # ADD THIS
    sequence_direction=SequenceDirection.BOTH,
),
```

Add imports at top:
```python
from darwindeck.genome.schema import TableauMode, SequenceDirection
```

**Step 4: Run tests to verify they pass**

```bash
uv run pytest tests/unit/test_examples.py -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/examples.py tests/unit/test_examples.py
git commit -m "feat(examples): add explicit tableau_mode to seed genomes"
```

---

## Task 14: Integration Test - Full Simulation

**Files:**
- Test: `tests/integration/test_tableau_modes.py`

**Step 1: Write the integration test**

```python
# tests/integration/test_tableau_modes.py - new file

"""Integration tests for tableau mode simulation."""

import pytest
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, PlayPhase,
    WinCondition, Location, TableauMode, SequenceDirection
)
from darwindeck.bindings.cgo_bridge import simulate_batch


class TestTableauModeSimulation:
    """Test that each tableau mode simulates correctly."""

    def test_war_mode_games_complete(self):
        """WAR mode games run without errors."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="war_test",
            generation=1,
            setup=SetupRules(
                cards_per_player=26,
                tableau_mode=TableauMode.WAR,
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU, mandatory=True)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="capture_all")],
            scoring_rules=[],
            player_count=2,
        )

        result = simulate_batch(genome, num_games=50)

        assert result.errors == 0
        assert result.games_played == 50
        # War games should have a winner (not all draws)
        assert result.draw_rate < 0.5

    def test_none_mode_no_captures(self):
        """NONE mode games don't have captures."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="none_test",
            generation=1,
            setup=SetupRules(
                cards_per_player=7,
                tableau_mode=TableauMode.NONE,
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU, mandatory=True)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
            max_turns=50,
        )

        result = simulate_batch(genome, num_games=50)

        # Should complete without errors
        assert result.errors == 0

    def test_sequence_mode_shedding(self):
        """SEQUENCE mode allows shedding games."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="seq_test",
            generation=1,
            setup=SetupRules(
                cards_per_player=7,
                tableau_mode=TableauMode.SEQUENCE,
                sequence_direction=SequenceDirection.BOTH,
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU, mandatory=False, pass_if_unable=True)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
            max_turns=200,
        )

        result = simulate_batch(genome, num_games=50)

        # Should complete without errors
        assert result.errors == 0
```

**Step 2: Run tests**

```bash
uv run pytest tests/integration/test_tableau_modes.py -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add tests/integration/test_tableau_modes.py
git commit -m "test: add integration tests for tableau mode simulation"
```

---

## Task 15: Run Full Test Suite

**Step 1: Run all tests**

```bash
uv run pytest tests/ -v --tb=short
```

**Step 2: Run Go tests**

```bash
cd src/gosim && go test ./... -v
```

**Step 3: Fix any failures**

Address any test failures from existing tests that may have broken due to:
- CaptureMode removal
- Bytecode header size change
- Default behavior changes

**Step 4: Commit fixes**

```bash
git add -A
git commit -m "fix: update tests for tableau mode changes"
```

---

## Summary

| Task | Component | Description |
|------|-----------|-------------|
| 1 | Python schema | Add TableauMode and SequenceDirection enums |
| 2 | Python schema | Add fields to SetupRules |
| 3 | Python serialization | Handle JSON serialization |
| 4 | Python bytecode | Add version byte and tableau encoding |
| 5 | Go types | Add TableauMode to GameState |
| 6 | Go bytecode | Parse version and tableau_mode |
| 7 | Go runner | Read mode from header |
| 8 | Go movegen | Use explicit mode in ApplyMove |
| 9 | Go movegen | Implement SEQUENCE move generation |
| 10 | Rulebook | Add mode descriptions |
| 11 | Mutations | Add tableau mode mutation operators |
| 12 | Fitness | Add coherence penalties |
| 13 | Examples | Update seed genomes |
| 14 | Integration | Full simulation tests |
| 15 | Verification | Run full test suite |

**Total: 15 tasks**
