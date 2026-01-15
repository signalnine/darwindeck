# Explicit Tableau Mode Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make tableau card interactions explicit in the genome schema, eliminating implicit War/Scopa behaviors in the Go simulator.

**Problem:** The Go simulator applies War-style battles for 2-player TABLEAU games and Scopa captures based on win conditions. This implicit behavior isn't in the genome, so rulebooks can't describe it and evolution produces semantically incoherent games.

**Solution:** Add `tableau_mode` field to `SetupRules` with explicit modes: NONE, WAR, MATCH_RANK, SEQUENCE.

---

## Schema Changes

### New Enums (`src/darwindeck/genome/schema.py`)

```python
class TableauMode(Enum):
    """How cards on the tableau interact."""
    NONE = "none"              # Cards accumulate, no interaction
    WAR = "war"                # Compare cards, winner takes all (2-player only)
    MATCH_RANK = "match_rank"  # Matching rank captures (renamed from SCOPA for clarity)
    SEQUENCE = "sequence"      # Build ascending/descending piles

class SequenceDirection(Enum):
    """Direction for SEQUENCE tableau mode."""
    ASCENDING = "ascending"
    DESCENDING = "descending"
    BOTH = "both"
```

### SetupRules Fields

```python
@dataclass(frozen=True)
class SetupRules:
    # ... existing fields ...

    tableau_mode: TableauMode = TableauMode.NONE
    sequence_direction: SequenceDirection = SequenceDirection.BOTH  # Only used when mode=SEQUENCE
```

**Note:** `sequence_direction` is only meaningful when `tableau_mode == SEQUENCE`. Serialization and mutations should respect this conditionality.

---

## Bytecode Encoding

### Version Header

Add bytecode version to enable future migrations:

| Offset | Size | Field |
|--------|------|-------|
| 0 | 1 | `bytecode_version` (currently: 2, previous implicit: 1) |
| 1-36 | 36 | Existing header fields (shifted by 1) |
| 37 | 1 | `tableau_mode` (0=NONE, 1=WAR, 2=MATCH_RANK, 3=SEQUENCE) |
| 38 | 1 | `sequence_direction` (0=ASC, 1=DESC, 2=BOTH) |

**New header size:** 39 bytes

**Version handling:**
- Version 1 (implicit): Old 36-byte format, no tableau_mode → treat as NONE
- Version 2: New 39-byte format with explicit tableau_mode
- Unknown versions: Return parse error (fail-fast)

**Reserved values:** Enum values 4-255 reserved for future modes. Parser should reject unknown values.

---

## Go Simulator Changes

### types.go

Remove `CaptureMode bool` field. Add:

```go
TableauMode       uint8  // 0=NONE, 1=WAR, 2=MATCH_RANK, 3=SEQUENCE
SequenceDirection uint8  // 0=ASC, 1=DESC, 2=BOTH
```

### movegen.go

Replace implicit behavior in `ApplyMove`:

```go
if move.TargetLoc == LocationTableau {
    switch state.TableauMode {
    case 1: // WAR
        resolveWarBattle(state)
    case 2: // MATCH_RANK
        resolveMatchRankCapture(state, currentPlayer, playedCard)
    case 3: // SEQUENCE
        // Validation done in move generation; card placement already validated
    }
    // case 0 (NONE): do nothing
}
```

For SEQUENCE mode, `GenerateLegalMoves` validates plays are in order.

### runner.go

Read `tableau_mode` from bytecode header instead of inferring from win conditions.

---

## WAR Mode: Temporal Model

**How the sequential engine handles card comparison:**

The current simulator resolves War battles using a **deferred comparison** model:

1. Player 0 plays a card to TABLEAU → card added to `state.Tableau[0]`
2. Player 1 plays a card to TABLEAU → card added to `state.Tableau[0]`
3. After Player 1's play, `resolveWarBattle()` is called
4. Function checks: `if len(state.Tableau[0]) >= 2` → compare last two cards
5. Winner takes all cards from tableau

**Key invariant:** `resolveWarBattle()` only triggers comparison when 2+ cards are present. This makes the sequential turn order work correctly.

### WAR Mode Constraints

**WAR mode is only valid for 2-player games.**

Validation:
```python
if genome.setup.tableau_mode == TableauMode.WAR and genome.player_count != 2:
    raise ValueError("WAR tableau mode requires exactly 2 players")
```

Rationale: N-player War has ambiguous semantics (pairwise? round-robin? highest takes all?). Rather than over-specify, constrain to the well-understood 2-player case.

---

## SEQUENCE Mode: Complete Specification

### Semantics

In SEQUENCE mode, cards on the tableau form ordered piles. Players can only add cards that continue the sequence.

### Starting Conditions

- Any card can start a new pile on an empty tableau
- Each pile has a **base rank** (the first card played to it)
- Multiple piles are supported (one per suit, for games like Fan Tan)

### Direction Rules

| Direction | Valid plays |
|-----------|-------------|
| ASCENDING | Next higher rank (7→8→9→10→J→Q→K) |
| DESCENDING | Next lower rank (7→6→5→4→3→2→A) |
| BOTH | Either direction from base |

### Boundary Behavior

- **Ascending from King:** No valid play (pile is complete)
- **Descending from Ace:** No valid play (pile is complete)
- **Wrapping:** NOT supported (K→A or A→K are invalid)

### Move Generation

```go
// In GenerateLegalMoves for SEQUENCE mode:
for each card in hand:
    if tableau is empty:
        // Any card can start a pile
        add move(card → TABLEAU)
    else:
        for each pile in tableau:
            topCard := pile[len(pile)-1]
            if isValidSequencePlay(card, topCard, direction):
                add move(card → TABLEAU)
```

### Pile Organization

- Each suit gets its own pile (tracked by `state.Tableau[suitIndex]`)
- Card must match suit of target pile
- This mirrors Fan Tan structure

---

## Rulebook Generator

### Phase Descriptions

Update `_describe_phase()` to include tableau mode context:

| Mode | Description |
|------|-------------|
| NONE | "Cards remain on the tableau." |
| WAR | "When both players have played, compare ranks—higher card wins both cards." |
| MATCH_RANK | "If your card matches a card on the tableau by rank, capture both cards." |
| SEQUENCE (ASC) | "Play cards in ascending order (7→8→9→...) to build on tableau piles." |
| SEQUENCE (DESC) | "Play cards in descending order (7→6→5→...) to build on tableau piles." |
| SEQUENCE (BOTH) | "Play cards in either direction to build on tableau piles." |

### New Section

Add "Tableau Rules" section when mode ≠ NONE with full explanation of interaction rules.

---

## Mutations

### New Operators

1. **`MutateTableauModeMutation`** - Changes tableau_mode randomly (low weight)
   - If changing TO `WAR`, must be 2-player game
   - If changing FROM `SEQUENCE`, clear sequence_direction to default

2. **`MutateSequenceDirectionMutation`** - Changes direction when mode=SEQUENCE (low weight)
   - Only applies when `tableau_mode == SEQUENCE`

### Coherence (Soft Penalty)

| Tableau Mode | Good With | Bad With | Penalty |
|--------------|-----------|----------|---------|
| WAR | `capture_all`, `most_captured` | `empty_hand` | 30% |
| MATCH_RANK | `most_captured`, `high_score` | `capture_all` | 20% |
| SEQUENCE | `empty_hand` | `capture_all` | 30% |
| NONE | Any | - | 0% |

**Rationale for soft penalties:** Evolution may discover valid edge cases we didn't anticipate. Penalties discourage but don't prevent exploration.

---

## Seed Genomes

Update `src/darwindeck/genome/examples.py`:

| Genome | `tableau_mode` | `sequence_direction` |
|--------|---------------|---------------------|
| war-baseline | WAR | - |
| betting-war | WAR | - |
| scopa | MATCH_RANK | - |
| fan-tan | SEQUENCE | BOTH |
| Others | NONE | - |

---

## Migration

### Version Detection

```python
def parse_bytecode(data: bytes) -> Genome:
    if len(data) < 1:
        raise ValueError("Empty bytecode")

    version = data[0]
    if version == 0 or version > 2:
        # Assume old format (no version byte, first byte was part of header)
        return parse_v1_bytecode(data)  # Defaults tableau_mode to NONE
    elif version == 2:
        return parse_v2_bytecode(data)
    else:
        raise ValueError(f"Unknown bytecode version: {version}")
```

### JSON Migration

Missing `tableau_mode` field in JSON defaults to `NONE`.

### Testing Migration

Add tests that:
1. Load old v1 bytecode → tableau_mode defaults to NONE
2. Load old JSON without tableau_mode → defaults to NONE
3. Round-trip new genomes → preserves tableau_mode

---

## Testing

1. **Unit tests for each tableau mode in Go simulator**
   - WAR: 2 cards on tableau → comparison and capture
   - MATCH_RANK: matching rank → capture
   - SEQUENCE: valid/invalid sequence plays
   - NONE: cards accumulate without interaction

2. **Integration test: genome with each mode simulates correctly**
   - War genome with WAR mode → games complete, one player captures all
   - Scopa genome with MATCH_RANK mode → captures occur
   - Fan Tan genome with SEQUENCE mode → shedding works

3. **Rulebook test: each mode generates appropriate description**

4. **Mutation test: tableau mode mutations produce valid genomes**
   - WAR only on 2-player games
   - sequence_direction only when mode=SEQUENCE

5. **Migration tests**
   - Old bytecode loads with NONE default
   - Old JSON loads with NONE default
   - Version detection works correctly

---

## Files to Modify

| File | Changes |
|------|---------|
| `src/darwindeck/genome/schema.py` | Add TableauMode, SequenceDirection enums and SetupRules fields |
| `src/darwindeck/genome/bytecode.py` | Add version byte, encode tableau_mode |
| `src/darwindeck/genome/serialization.py` | Handle new fields in JSON |
| `src/gosim/engine/types.go` | Add TableauMode field, remove CaptureMode |
| `src/gosim/engine/movegen.go` | Use explicit mode; add SEQUENCE validation |
| `src/gosim/engine/bytecode.go` | Parse version + tableau_mode from header |
| `src/gosim/simulation/runner.go` | Read mode from header, not win conditions |
| `src/darwindeck/evolution/rulebook.py` | Generate mode-specific descriptions |
| `src/darwindeck/evolution/mutations.py` | Add tableau mode mutations with constraints |
| `src/darwindeck/evolution/fitness_full.py` | Add coherence penalties |
| `src/darwindeck/genome/examples.py` | Update seed genomes |
| `tests/` | Add all tests listed above |

---

## Appendix: Review Feedback Addressed

| Issue | Resolution |
|-------|------------|
| Bytecode versioning missing | Added version byte at offset 0; defined v1/v2 handling |
| SEQUENCE mode under-specified | Added complete specification: starting, boundaries, pile organization |
| N-player WAR undefined | Constrained WAR to 2-player games with validation |
| WAR temporal model unclear | Documented deferred comparison mechanism |
| SCOPA naming ambiguity | Renamed to MATCH_RANK for clarity |
| Migration tests missing | Added to testing section |
| sequence_direction conditionality | Documented; mutations respect it |
