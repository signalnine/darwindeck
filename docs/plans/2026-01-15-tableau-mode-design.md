# Explicit Tableau Mode Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make tableau card interactions explicit in the genome schema, eliminating implicit War/Scopa behaviors in the Go simulator.

**Problem:** The Go simulator applies War-style battles for 2-player TABLEAU games and Scopa captures based on win conditions. This implicit behavior isn't in the genome, so rulebooks can't describe it and evolution produces semantically incoherent games.

**Solution:** Add `tableau_mode` field to `SetupRules` with explicit modes: NONE, WAR, SCOPA, SEQUENCE.

---

## Schema Changes

### New Enums (`src/darwindeck/genome/schema.py`)

```python
class TableauMode(Enum):
    """How cards on the tableau interact."""
    NONE = "none"          # Cards accumulate, no interaction
    WAR = "war"            # Compare cards, winner takes all
    SCOPA = "scopa"        # Matching rank captures
    SEQUENCE = "sequence"  # Build ascending/descending piles

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
    sequence_direction: SequenceDirection = SequenceDirection.BOTH
```

---

## Bytecode Encoding

Extend genome header from 36 to 38 bytes:

| Offset | Size | Field |
|--------|------|-------|
| 36 | 1 | `tableau_mode` (0=NONE, 1=WAR, 2=SCOPA, 3=SEQUENCE) |
| 37 | 1 | `sequence_direction` (0=ASC, 1=DESC, 2=BOTH) |

---

## Go Simulator Changes

### types.go

Remove `CaptureMode bool` field. Add:

```go
TableauMode       uint8  // 0=NONE, 1=WAR, 2=SCOPA, 3=SEQUENCE
SequenceDirection uint8  // 0=ASC, 1=DESC, 2=BOTH
```

### movegen.go

Replace implicit behavior in `ApplyMove`:

```go
if move.TargetLoc == LocationTableau {
    switch state.TableauMode {
    case 1: // WAR
        resolveWarBattle(state)
    case 2: // SCOPA
        resolveScopaCapture(state, currentPlayer, playedCard)
    case 3: // SEQUENCE
        // Validation done in move generation
    }
    // case 0 (NONE): do nothing
}
```

For SEQUENCE mode, `GenerateLegalMoves` validates plays are in order.

### runner.go

Read `tableau_mode` from bytecode header instead of inferring from win conditions.

---

## Rulebook Generator

### Phase Descriptions

Update `_describe_phase()` to include tableau mode context:

| Mode | Description |
|------|-------------|
| NONE | "Cards remain on the tableau." |
| WAR | "When both players have played, compare ranks—higher card wins both cards." |
| SCOPA | "If your card matches a card on the tableau by rank, capture both cards." |
| SEQUENCE | "Cards must be played in [direction] order from the base card." |

### New Section

Add "Tableau Rules" section when mode ≠ NONE.

---

## Mutations

### New Operators

1. **`MutateTableauModeMutation`** - Changes tableau_mode randomly (low weight)
2. **`MutateSequenceDirectionMutation`** - Changes direction when mode=SEQUENCE (low weight)

### Coherence (Soft Penalty)

| Tableau Mode | Good With | Bad With |
|--------------|-----------|----------|
| WAR | `capture_all`, `most_captured` | `empty_hand` |
| SCOPA | `most_captured`, `high_score` | `capture_all` |
| SEQUENCE | `empty_hand` | `capture_all` |
| NONE | Any | - |

Apply fitness penalty for incoherent combinations rather than hard rejection.

---

## Seed Genomes

Update `src/darwindeck/genome/examples.py`:

| Genome | `tableau_mode` |
|--------|---------------|
| war-baseline | WAR |
| betting-war | WAR |
| scopa | SCOPA |
| fan-tan | SEQUENCE |
| Others | NONE |

---

## Migration

Missing `tableau_mode` field defaults to `NONE`. Old evolved genomes may break but were semantically confused anyway.

---

## Testing

1. Unit tests for each tableau mode in Go simulator
2. Integration test: genome with each mode simulates correctly
3. Rulebook test: each mode generates appropriate description
4. Mutation test: tableau mode mutations produce valid genomes

---

## Files to Modify

| File | Changes |
|------|---------|
| `src/darwindeck/genome/schema.py` | Add TableauMode, SequenceDirection enums and SetupRules fields |
| `src/darwindeck/genome/bytecode.py` | Encode tableau_mode in header |
| `src/darwindeck/genome/serialization.py` | Handle new fields in JSON |
| `src/gosim/engine/types.go` | Add TableauMode field, remove CaptureMode |
| `src/gosim/engine/movegen.go` | Use explicit mode instead of inference |
| `src/gosim/engine/bytecode.go` | Parse tableau_mode from header |
| `src/gosim/simulation/runner.go` | Read mode from header, not win conditions |
| `src/darwindeck/evolution/rulebook.py` | Generate mode-specific descriptions |
| `src/darwindeck/evolution/mutations.py` | Add tableau mode mutations |
| `src/darwindeck/genome/examples.py` | Update seed genomes |
| `tests/` | Add tests for all changes |
