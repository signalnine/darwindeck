# Special Effects System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add card-triggered immediate effects (Skip, Reverse, Draw N) to enable Uno-style games in evolution.

**Architecture:** Effects are stored in genome's `special_effects` list, compiled to bytecode, parsed by Go into a rank->effect map, and executed in `ApplyMove`. Turn advancement uses new `PlayDirection` and `SkipCount` fields.

**Tech Stack:** Python (schema, bytecode, mutations), Go (state, execution), TDD throughout

**Reference:** See `docs/plans/2026-01-11-special-effects-design.md` for full design details.

---

## Task 1: Add Python Schema Types

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Test: `tests/unit/test_schema.py`

**Step 1: Write failing test**

Add to `tests/unit/test_schema.py`:

```python
def test_effect_type_enum():
    """EffectType enum has all expected values."""
    from darwindeck.genome.schema import EffectType

    assert EffectType.SKIP_NEXT.value == "skip_next"
    assert EffectType.REVERSE_DIRECTION.value == "reverse"
    assert EffectType.DRAW_CARDS.value == "draw_cards"
    assert EffectType.EXTRA_TURN.value == "extra_turn"
    assert EffectType.FORCE_DISCARD.value == "force_discard"


def test_special_effect_creation():
    """SpecialEffect dataclass is frozen and has correct fields."""
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector

    effect = SpecialEffect(
        trigger_rank=Rank.TWO,
        effect_type=EffectType.DRAW_CARDS,
        target=TargetSelector.NEXT_PLAYER,
        value=2
    )

    assert effect.trigger_rank == Rank.TWO
    assert effect.effect_type == EffectType.DRAW_CARDS
    assert effect.target == TargetSelector.NEXT_PLAYER
    assert effect.value == 2


def test_special_effect_default_value():
    """SpecialEffect value defaults to 1."""
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector

    effect = SpecialEffect(
        trigger_rank=Rank.JACK,
        effect_type=EffectType.SKIP_NEXT,
        target=TargetSelector.NEXT_PLAYER
    )

    assert effect.value == 1
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_schema.py::test_effect_type_enum -v`
Expected: FAIL with "cannot import name 'EffectType'"

**Step 3: Implement EffectType and SpecialEffect**

Add to `src/darwindeck/genome/schema.py` after `Visibility` enum:

```python
class EffectType(Enum):
    """Types of immediate effects a card can trigger."""
    SKIP_NEXT = "skip_next"
    REVERSE_DIRECTION = "reverse"
    DRAW_CARDS = "draw_cards"
    EXTRA_TURN = "extra_turn"
    FORCE_DISCARD = "force_discard"


@dataclass(frozen=True)
class SpecialEffect:
    """A card-triggered immediate effect."""
    trigger_rank: Rank
    effect_type: EffectType
    target: TargetSelector
    value: int = 1
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_schema.py -v -k effect`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_schema.py
git commit -m "feat(schema): add EffectType enum and SpecialEffect dataclass"
```

---

## Task 2: Add Go State Fields

**Files:**
- Modify: `src/gosim/engine/types.go`
- Modify: `src/gosim/engine/types_test.go`

**Step 1: Write failing test**

Add to `src/gosim/engine/types_test.go`:

```go
func TestGameStateHasEffectFields(t *testing.T) {
	state := GetState()
	defer PutState(state)

	// New fields should exist and have defaults
	if state.PlayDirection != 1 {
		t.Errorf("PlayDirection should default to 1, got %d", state.PlayDirection)
	}
	if state.SkipCount != 0 {
		t.Errorf("SkipCount should default to 0, got %d", state.SkipCount)
	}
}

func TestGameStateClonePreservesEffectFields(t *testing.T) {
	state := GetState()
	state.PlayDirection = -1
	state.SkipCount = 2

	clone := state.Clone()
	defer PutState(state)
	defer PutState(clone)

	if clone.PlayDirection != -1 {
		t.Errorf("Clone PlayDirection should be -1, got %d", clone.PlayDirection)
	}
	if clone.SkipCount != 2 {
		t.Errorf("Clone SkipCount should be 2, got %d", clone.SkipCount)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd src/gosim && go test ./engine -v -run TestGameStateHasEffectFields`
Expected: FAIL with "state.PlayDirection undefined"

**Step 3: Add fields to GameState**

In `src/gosim/engine/types.go`, add to GameState struct (after `CaptureMode`):

```go
	// Special effects state
	PlayDirection int8  // 1 = clockwise, -1 = counter-clockwise
	SkipCount     uint8 // Number of players to skip (capped at NumPlayers-1)
```

Update `Reset()` method to add:

```go
	s.PlayDirection = 1
	s.SkipCount = 0
```

Update `Clone()` method to add (after `s.CaptureMode`):

```go
	clone.PlayDirection = s.PlayDirection
	clone.SkipCount = s.SkipCount
```

**Step 4: Run tests to verify they pass**

Run: `cd src/gosim && go test ./engine -v -run TestGameState`
Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/types.go src/gosim/engine/types_test.go
git commit -m "feat(gosim): add PlayDirection and SkipCount to GameState"
```

---

## Task 3: Create Go Effects Module

**Files:**
- Create: `src/gosim/engine/effects.go`
- Create: `src/gosim/engine/effects_test.go`

**Step 1: Write failing tests**

Create `src/gosim/engine/effects_test.go`:

```go
package engine

import (
	"testing"
)

func TestApplySkipNext(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 3

	effect := &SpecialEffect{EffectType: EFFECT_SKIP_NEXT, Value: 1}
	ApplyEffect(state, effect, nil)

	if state.SkipCount != 1 {
		t.Errorf("SkipCount should be 1, got %d", state.SkipCount)
	}
}

func TestApplySkipNextCapped(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 3
	state.SkipCount = 2

	effect := &SpecialEffect{EffectType: EFFECT_SKIP_NEXT, Value: 5}
	ApplyEffect(state, effect, nil)

	// Should cap at NumPlayers-1 = 2
	if state.SkipCount != 2 {
		t.Errorf("SkipCount should cap at 2, got %d", state.SkipCount)
	}
}

func TestApplyReverse(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.PlayDirection = 1

	effect := &SpecialEffect{EffectType: EFFECT_REVERSE}
	ApplyEffect(state, effect, nil)

	if state.PlayDirection != -1 {
		t.Errorf("PlayDirection should be -1, got %d", state.PlayDirection)
	}

	// Reverse again
	ApplyEffect(state, effect, nil)
	if state.PlayDirection != 1 {
		t.Errorf("PlayDirection should be 1, got %d", state.PlayDirection)
	}
}

func TestApplyDrawCards(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 2
	state.CurrentPlayer = 0
	state.PlayDirection = 1
	state.Deck = []Card{{Rank: 5, Suit: 0}, {Rank: 7, Suit: 1}, {Rank: 9, Suit: 2}}
	state.Players[1].Hand = []Card{}

	effect := &SpecialEffect{
		EffectType: EFFECT_DRAW_CARDS,
		Target:     TARGET_NEXT_PLAYER,
		Value:      2,
	}
	ApplyEffect(state, effect, nil)

	if len(state.Players[1].Hand) != 2 {
		t.Errorf("Player 1 should have 2 cards, got %d", len(state.Players[1].Hand))
	}
	if len(state.Deck) != 1 {
		t.Errorf("Deck should have 1 card, got %d", len(state.Deck))
	}
}

func TestApplyExtraTurn(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 3

	effect := &SpecialEffect{EffectType: EFFECT_EXTRA_TURN}
	ApplyEffect(state, effect, nil)

	// Should skip all other players (NumPlayers - 1)
	if state.SkipCount != 2 {
		t.Errorf("SkipCount should be 2, got %d", state.SkipCount)
	}
}

func TestApplyForceDiscard(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 2
	state.CurrentPlayer = 0
	state.PlayDirection = 1
	state.Players[1].Hand = []Card{{Rank: 2, Suit: 0}, {Rank: 5, Suit: 1}, {Rank: 8, Suit: 2}}
	state.Discard = []Card{}

	effect := &SpecialEffect{
		EffectType: EFFECT_FORCE_DISCARD,
		Target:     TARGET_NEXT_PLAYER,
		Value:      2,
	}
	ApplyEffect(state, effect, nil)

	if len(state.Players[1].Hand) != 1 {
		t.Errorf("Player 1 should have 1 card, got %d", len(state.Players[1].Hand))
	}
	if len(state.Discard) != 2 {
		t.Errorf("Discard should have 2 cards, got %d", len(state.Discard))
	}
}

func TestResolveTargetNextPlayer(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 4
	state.CurrentPlayer = 1
	state.PlayDirection = 1

	target := resolveTarget(state, TARGET_NEXT_PLAYER)
	if target != 2 {
		t.Errorf("Next player from 1 (direction 1) should be 2, got %d", target)
	}

	state.PlayDirection = -1
	target = resolveTarget(state, TARGET_NEXT_PLAYER)
	if target != 0 {
		t.Errorf("Next player from 1 (direction -1) should be 0, got %d", target)
	}
}

func TestResolveTargetAllOpponents(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 3
	state.CurrentPlayer = 1

	target := resolveTarget(state, TARGET_ALL_OPPONENTS)
	if target != -1 {
		t.Errorf("ALL_OPPONENTS should return -1, got %d", target)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd src/gosim && go test ./engine -v -run TestApply`
Expected: FAIL with "undefined: SpecialEffect"

**Step 3: Implement effects.go**

Create `src/gosim/engine/effects.go`:

```go
package engine

// Effect type constants
const (
	EFFECT_SKIP_NEXT = iota
	EFFECT_REVERSE
	EFFECT_DRAW_CARDS
	EFFECT_EXTRA_TURN
	EFFECT_FORCE_DISCARD
)

// Target constants
const (
	TARGET_NEXT_PLAYER = iota
	TARGET_PREV_PLAYER
	TARGET_PLAYER_CHOICE
	TARGET_RANDOM_OPPONENT
	TARGET_ALL_OPPONENTS
	TARGET_LEFT_OPPONENT
	TARGET_RIGHT_OPPONENT
)

// SpecialEffect represents a card-triggered effect
type SpecialEffect struct {
	TriggerRank uint8
	EffectType  uint8
	Target      uint8
	Value       uint8
}

// ApplyEffect executes a special effect on the game state
func ApplyEffect(state *GameState, effect *SpecialEffect, rng RNG) {
	switch effect.EffectType {
	case EFFECT_SKIP_NEXT:
		state.SkipCount += effect.Value
		// Cap at NumPlayers-1 to prevent degenerate infinite turns
		maxSkip := state.NumPlayers - 1
		if state.SkipCount > maxSkip {
			state.SkipCount = maxSkip
		}

	case EFFECT_REVERSE:
		state.PlayDirection *= -1

	case EFFECT_DRAW_CARDS:
		applyToTargets(state, effect.Target, rng, func(targetID int) {
			for i := uint8(0); i < effect.Value && len(state.Deck) > 0; i++ {
				card := state.Deck[0]
				state.Deck = state.Deck[1:]
				state.Players[targetID].Hand = append(state.Players[targetID].Hand, card)
			}
		})

	case EFFECT_EXTRA_TURN:
		// Skip everyone else = current player goes again
		state.SkipCount = state.NumPlayers - 1

	case EFFECT_FORCE_DISCARD:
		applyToTargets(state, effect.Target, rng, func(targetID int) {
			hand := &state.Players[targetID].Hand
			toDiscard := int(effect.Value)
			if toDiscard > len(*hand) {
				toDiscard = len(*hand)
			}
			for i := 0; i < toDiscard; i++ {
				card := (*hand)[len(*hand)-1]
				*hand = (*hand)[:len(*hand)-1]
				state.Discard = append(state.Discard, card)
			}
		})

	default:
		// Unknown effect type - ignore for forward compatibility
	}
}

// RNG interface for deterministic random (nil = no random effects)
type RNG interface {
	Intn(n int) int
}

// resolveTarget determines which player(s) an effect targets
func resolveTarget(state *GameState, target uint8) int {
	current := int(state.CurrentPlayer)
	numPlayers := int(state.NumPlayers)
	direction := int(state.PlayDirection)

	switch target {
	case TARGET_NEXT_PLAYER:
		return (current + direction + numPlayers) % numPlayers
	case TARGET_PREV_PLAYER:
		return (current - direction + numPlayers) % numPlayers
	case TARGET_ALL_OPPONENTS:
		// Returns -1 to signal caller must loop over all opponents
		return -1
	default:
		return (current + 1) % numPlayers
	}
}

// applyToTargets handles single target or ALL_OPPONENTS
func applyToTargets(state *GameState, target uint8, rng RNG, action func(int)) {
	targetID := resolveTarget(state, target)
	if targetID == -1 {
		// ALL_OPPONENTS: apply to everyone except current player
		for i := 0; i < int(state.NumPlayers); i++ {
			if i != int(state.CurrentPlayer) {
				action(i)
			}
		}
	} else {
		action(targetID)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd src/gosim && go test ./engine -v -run "TestApply|TestResolve"`
Expected: PASS (8 tests)

**Step 5: Commit**

```bash
git add src/gosim/engine/effects.go src/gosim/engine/effects_test.go
git commit -m "feat(gosim): implement special effects execution"
```

---

## Task 4: Update Turn Advancement

**Files:**
- Modify: `src/gosim/engine/movegen.go`
- Test: `src/gosim/engine/effects_test.go`

**Step 1: Write failing test**

Add to `src/gosim/engine/effects_test.go`:

```go
func TestAdvanceTurnWithSkip(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 4
	state.CurrentPlayer = 0
	state.PlayDirection = 1
	state.SkipCount = 1

	AdvanceTurn(state)

	if state.CurrentPlayer != 2 {
		t.Errorf("Should skip to player 2, got %d", state.CurrentPlayer)
	}
	if state.SkipCount != 0 {
		t.Errorf("SkipCount should reset to 0, got %d", state.SkipCount)
	}
}

func TestAdvanceTurnReversed(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 4
	state.CurrentPlayer = 1
	state.PlayDirection = -1
	state.SkipCount = 0

	AdvanceTurn(state)

	if state.CurrentPlayer != 0 {
		t.Errorf("Reversed from 1 should go to 0, got %d", state.CurrentPlayer)
	}
}

func TestAdvanceTurnWraparound(t *testing.T) {
	state := GetState()
	defer PutState(state)
	state.NumPlayers = 3
	state.CurrentPlayer = 2
	state.PlayDirection = 1
	state.SkipCount = 0

	AdvanceTurn(state)

	if state.CurrentPlayer != 0 {
		t.Errorf("Should wrap to 0, got %d", state.CurrentPlayer)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd src/gosim && go test ./engine -v -run TestAdvanceTurn`
Expected: FAIL with "undefined: AdvanceTurn"

**Step 3: Implement AdvanceTurn**

Add to `src/gosim/engine/effects.go`:

```go
// AdvanceTurn moves to the next player, respecting direction and skips
func AdvanceTurn(state *GameState) {
	step := int(state.PlayDirection)
	next := int(state.CurrentPlayer)
	numPlayers := int(state.NumPlayers)

	// Always advance at least once, plus any skips
	for i := 0; i <= int(state.SkipCount); i++ {
		next = (next + step + numPlayers) % numPlayers
	}

	state.CurrentPlayer = uint8(next)
	state.SkipCount = 0 // Reset after applying
}
```

**Step 4: Run tests to verify they pass**

Run: `cd src/gosim && go test ./engine -v -run TestAdvanceTurn`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add src/gosim/engine/effects.go src/gosim/engine/effects_test.go
git commit -m "feat(gosim): add AdvanceTurn with direction and skip support"
```

---

## Task 5: Add Bytecode Opcodes

**Files:**
- Modify: `src/darwindeck/genome/bytecode.py`
- Test: `tests/unit/test_bytecode.py`

**Step 1: Write failing test**

Add to `tests/unit/test_bytecode.py`:

```python
def test_effect_opcodes_exist():
    """Effect-related opcodes are defined."""
    from darwindeck.genome.bytecode import OpCode

    assert OpCode.EFFECT_HEADER.value == 60
    assert OpCode.EFFECT_ENTRY.value == 61


def test_compile_effects():
    """compile_effects produces correct bytecode."""
    from darwindeck.genome.bytecode import compile_effects
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector

    effects = [
        SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        SpecialEffect(Rank.JACK, EffectType.SKIP_NEXT, TargetSelector.NEXT_PLAYER, 1),
    ]

    bytecode = compile_effects(effects)

    # Header: opcode (60), count (2)
    assert bytecode[0] == 60  # EFFECT_HEADER
    assert bytecode[1] == 2   # count

    # Effect 1: TWO (0), DRAW_CARDS (2), NEXT_PLAYER (0), value (2)
    assert bytecode[2] == 0   # Rank.TWO -> 0
    assert bytecode[3] == 2   # EffectType.DRAW_CARDS -> 2
    assert bytecode[4] == 0   # TARGET_NEXT_PLAYER -> 0
    assert bytecode[5] == 2   # value

    # Effect 2: JACK (9), SKIP_NEXT (0), NEXT_PLAYER (0), value (1)
    assert bytecode[6] == 9   # Rank.JACK -> 9
    assert bytecode[7] == 0   # EffectType.SKIP_NEXT -> 0
    assert bytecode[8] == 0   # TARGET_NEXT_PLAYER -> 0
    assert bytecode[9] == 1   # value
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_bytecode.py::test_effect_opcodes_exist -v`
Expected: FAIL with "EFFECT_HEADER not found"

**Step 3: Add opcodes and compile_effects**

In `src/darwindeck/genome/bytecode.py`, add to OpCode enum:

```python
    # Special Effects (60-69)
    EFFECT_HEADER = 60
    EFFECT_ENTRY = 61
```

Add mapping constants (after existing mappings):

```python
# Rank to bytecode mapping
RANK_TO_BYTE = {
    Rank.TWO: 0, Rank.THREE: 1, Rank.FOUR: 2, Rank.FIVE: 3,
    Rank.SIX: 4, Rank.SEVEN: 5, Rank.EIGHT: 6, Rank.NINE: 7,
    Rank.TEN: 8, Rank.JACK: 9, Rank.QUEEN: 10, Rank.KING: 11, Rank.ACE: 12,
}

# EffectType to bytecode mapping
EFFECT_TYPE_TO_BYTE = {
    EffectType.SKIP_NEXT: 0,
    EffectType.REVERSE_DIRECTION: 1,
    EffectType.DRAW_CARDS: 2,
    EffectType.EXTRA_TURN: 3,
    EffectType.FORCE_DISCARD: 4,
}

# TargetSelector to bytecode mapping
TARGET_TO_BYTE = {
    TargetSelector.NEXT_PLAYER: 0,
    TargetSelector.PREV_PLAYER: 1,
    TargetSelector.PLAYER_CHOICE: 2,
    TargetSelector.RANDOM_OPPONENT: 3,
    TargetSelector.ALL_OPPONENTS: 4,
    TargetSelector.LEFT_OPPONENT: 5,
    TargetSelector.RIGHT_OPPONENT: 6,
}
```

Add compile_effects function:

```python
def compile_effects(effects: list) -> bytes:
    """Compile special effects to bytecode.

    Format:
    - EFFECT_HEADER opcode (1 byte)
    - effect_count (1 byte)
    - For each effect (4 bytes):
      - trigger_rank (1 byte)
      - effect_type (1 byte)
      - target (1 byte)
      - value (1 byte)
    """
    if not effects:
        return bytes()

    result = bytes([OpCode.EFFECT_HEADER.value, len(effects)])
    for effect in effects:
        result += bytes([
            RANK_TO_BYTE[effect.trigger_rank],
            EFFECT_TYPE_TO_BYTE[effect.effect_type],
            TARGET_TO_BYTE[effect.target],
            effect.value,
        ])
    return result
```

Add imports at top of bytecode.py:

```python
from darwindeck.genome.schema import EffectType, TargetSelector
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_bytecode.py -v -k effect`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/genome/bytecode.py tests/unit/test_bytecode.py
git commit -m "feat(bytecode): add effect compilation opcodes and function"
```

---

## Task 6: Parse Effects in Go

**Files:**
- Modify: `src/gosim/engine/bytecode.go`
- Modify: `src/gosim/engine/bytecode_test.go`

**Step 1: Write failing test**

Add to `src/gosim/engine/bytecode_test.go`:

```go
func TestParseEffects(t *testing.T) {
	// Bytecode: HEADER(60), count(2), effect1(4 bytes), effect2(4 bytes)
	data := []byte{
		60, 2,          // Header, count=2
		0, 2, 0, 2,     // TWO, DRAW_CARDS, NEXT_PLAYER, value=2
		9, 0, 0, 1,     // JACK, SKIP_NEXT, NEXT_PLAYER, value=1
	}

	effects, offset, err := parseEffects(data, 0)
	if err != nil {
		t.Fatalf("parseEffects failed: %v", err)
	}

	if offset != 10 {
		t.Errorf("offset should be 10, got %d", offset)
	}

	if len(effects) != 2 {
		t.Fatalf("should have 2 effects, got %d", len(effects))
	}

	// Check effect for rank 0 (TWO)
	e1, ok := effects[0]
	if !ok {
		t.Fatal("missing effect for rank 0")
	}
	if e1.EffectType != EFFECT_DRAW_CARDS || e1.Value != 2 {
		t.Errorf("effect 1: expected DRAW_CARDS/2, got %d/%d", e1.EffectType, e1.Value)
	}

	// Check effect for rank 9 (JACK)
	e2, ok := effects[9]
	if !ok {
		t.Fatal("missing effect for rank 9")
	}
	if e2.EffectType != EFFECT_SKIP_NEXT || e2.Value != 1 {
		t.Errorf("effect 2: expected SKIP_NEXT/1, got %d/%d", e2.EffectType, e2.Value)
	}
}

func TestParseEffectsBoundsCheck(t *testing.T) {
	// Truncated bytecode - says 2 effects but only has 1
	data := []byte{
		60, 2,          // Header, count=2
		0, 2, 0, 2,     // Only 1 effect
	}

	_, _, err := parseEffects(data, 0)
	if err == nil {
		t.Error("should fail on truncated data")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd src/gosim && go test ./engine -v -run TestParseEffects`
Expected: FAIL with "undefined: parseEffects"

**Step 3: Implement parseEffects**

Add to `src/gosim/engine/bytecode.go`:

```go
const OP_EFFECT_HEADER = 60

// parseEffects extracts special effects from bytecode
func parseEffects(data []byte, offset int) (map[uint8]SpecialEffect, int, error) {
	effects := make(map[uint8]SpecialEffect)

	// Bounds check: need at least 1 byte
	if offset >= len(data) {
		return effects, offset, nil // No effects section
	}

	if data[offset] != OP_EFFECT_HEADER {
		return effects, offset, nil // No effects section
	}
	offset++

	// Bounds check: need count byte
	if offset >= len(data) {
		return nil, offset, fmt.Errorf("truncated effects section: missing count")
	}

	count := int(data[offset])
	offset++

	// Bounds check: need 4 bytes per effect
	requiredBytes := count * 4
	if offset+requiredBytes > len(data) {
		return nil, offset, fmt.Errorf("truncated effects section: expected %d bytes, have %d",
			requiredBytes, len(data)-offset)
	}

	for i := 0; i < count; i++ {
		effect := SpecialEffect{
			TriggerRank: data[offset],
			EffectType:  data[offset+1],
			Target:      data[offset+2],
			Value:       data[offset+3],
		}
		// Note: Later effects with same rank overwrite earlier ones
		effects[effect.TriggerRank] = effect
		offset += 4
	}

	return effects, offset, nil
}
```

Add import at top if needed:

```go
import "fmt"
```

**Step 4: Run tests to verify they pass**

Run: `cd src/gosim && go test ./engine -v -run TestParseEffects`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add src/gosim/engine/bytecode.go src/gosim/engine/bytecode_test.go
git commit -m "feat(gosim): add bytecode parsing for special effects"
```

---

## Task 7: Integrate Effects into ParsedGenome

**Files:**
- Modify: `src/gosim/engine/bytecode.go`

**Step 1: Add Effects field to ParsedGenome**

In `src/gosim/engine/bytecode.go`, update ParsedGenome struct:

```go
type ParsedGenome struct {
	// ... existing fields ...
	Effects map[uint8]SpecialEffect // rank -> effect lookup
}
```

**Step 2: Update ParseGenome to call parseEffects**

Find where ParseGenome builds the result and add:

```go
	// Parse effects section (at end of bytecode)
	effects, _, err := parseEffects(data, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to parse effects: %w", err)
	}
	genome.Effects = effects
```

**Step 3: Run all bytecode tests**

Run: `cd src/gosim && go test ./engine -v -run "Bytecode|Parse"`
Expected: PASS

**Step 4: Commit**

```bash
git add src/gosim/engine/bytecode.go
git commit -m "feat(gosim): integrate effects into ParsedGenome"
```

---

## Task 8: Hook Effects into Move Execution

**Files:**
- Modify: `src/gosim/engine/movegen.go`
- Test: Integration test

**Step 1: Update ApplyMove to trigger effects**

In `src/gosim/engine/movegen.go`, find the ApplyMove function and add after card is played:

```go
	// Check for special effect after playing a card
	if genome != nil && genome.Effects != nil {
		if len(move.Cards) > 0 {
			playedRank := move.Cards[0].Rank
			if effect, ok := genome.Effects[playedRank]; ok {
				ApplyEffect(state, &effect, nil) // nil RNG for now
			}
		}
	}
```

**Step 2: Run simulation tests**

Run: `cd src/gosim && go test ./... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add src/gosim/engine/movegen.go
git commit -m "feat(gosim): trigger special effects when cards are played"
```

---

## Task 9: Add Python Mutation Operators

**Files:**
- Modify: `src/darwindeck/evolution/operators.py`
- Test: `tests/unit/test_operators.py`

**Step 1: Write failing tests**

Add to `tests/unit/test_operators.py`:

```python
def test_add_effect_mutation():
    """AddEffectMutation adds an effect to the genome."""
    from darwindeck.evolution.operators import AddEffectMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()
    original_count = len(genome.special_effects)

    mutation = AddEffectMutation()
    mutated = mutation.apply(genome)

    assert len(mutated.special_effects) == original_count + 1


def test_remove_effect_mutation():
    """RemoveEffectMutation removes an effect."""
    from darwindeck.evolution.operators import RemoveEffectMutation
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector, GameGenome, SetupRules, TurnStructure, WinCondition

    # Create genome with effects
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[
            SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        ],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = RemoveEffectMutation()
    mutated = mutation.apply(genome)

    assert len(mutated.special_effects) == 0


def test_mutate_effect_mutation():
    """MutateEffectMutation changes one field of an effect."""
    from darwindeck.evolution.operators import MutateEffectMutation
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector, GameGenome, SetupRules, TurnStructure, WinCondition
    import random

    random.seed(42)  # For reproducibility

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[
            SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        ],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = MutateEffectMutation()
    mutated = mutation.apply(genome)

    # Should still have one effect
    assert len(mutated.special_effects) == 1
    # Something should have changed
    original = genome.special_effects[0]
    changed = mutated.special_effects[0]
    assert (original.trigger_rank != changed.trigger_rank or
            original.effect_type != changed.effect_type or
            original.target != changed.target or
            original.value != changed.value)
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_operators.py::test_add_effect_mutation -v`
Expected: FAIL with "cannot import name 'AddEffectMutation'"

**Step 3: Implement mutation operators**

Add to `src/darwindeck/evolution/operators.py`:

```python
from darwindeck.genome.schema import SpecialEffect, EffectType, TargetSelector


@dataclass
class AddEffectMutation(MutationOperator):
    """Add a random special effect to the genome."""

    weight: float = 0.1

    def apply(self, genome: GameGenome) -> GameGenome:
        new_effect = SpecialEffect(
            trigger_rank=random.choice(list(Rank)),
            effect_type=random.choice(list(EffectType)),
            target=random.choice([
                TargetSelector.NEXT_PLAYER,
                TargetSelector.ALL_OPPONENTS,
            ]),
            value=random.randint(1, 3),
        )
        new_effects = list(genome.special_effects) + [new_effect]
        return self._copy_genome_with(genome, special_effects=new_effects)


@dataclass
class RemoveEffectMutation(MutationOperator):
    """Remove a random special effect from the genome."""

    weight: float = 0.1

    def apply(self, genome: GameGenome) -> GameGenome:
        if not genome.special_effects:
            return genome
        idx = random.randrange(len(genome.special_effects))
        new_effects = [e for i, e in enumerate(genome.special_effects) if i != idx]
        return self._copy_genome_with(genome, special_effects=new_effects)


@dataclass
class MutateEffectMutation(MutationOperator):
    """Mutate one field of a random special effect."""

    weight: float = 0.15

    def apply(self, genome: GameGenome) -> GameGenome:
        if not genome.special_effects:
            return genome

        idx = random.randrange(len(genome.special_effects))
        effect = genome.special_effects[idx]

        field = random.choice(['rank', 'type', 'target', 'value'])
        if field == 'rank':
            mutated = SpecialEffect(
                random.choice(list(Rank)),
                effect.effect_type,
                effect.target,
                effect.value,
            )
        elif field == 'type':
            mutated = SpecialEffect(
                effect.trigger_rank,
                random.choice(list(EffectType)),
                effect.target,
                effect.value,
            )
        elif field == 'target':
            mutated = SpecialEffect(
                effect.trigger_rank,
                effect.effect_type,
                random.choice([TargetSelector.NEXT_PLAYER, TargetSelector.ALL_OPPONENTS]),
                effect.value,
            )
        else:  # value
            new_value = max(1, min(4, effect.value + random.randint(-1, 1)))
            mutated = SpecialEffect(
                effect.trigger_rank,
                effect.effect_type,
                effect.target,
                new_value,
            )

        new_effects = list(genome.special_effects)
        new_effects[idx] = mutated
        return self._copy_genome_with(genome, special_effects=new_effects)
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_operators.py -v -k effect`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/operators.py tests/unit/test_operators.py
git commit -m "feat(evolution): add special effect mutation operators"
```

---

## Task 10: Create Uno-Style Seed Game

**Files:**
- Modify: `src/darwindeck/genome/examples.py`
- Test: Integration test

**Step 1: Add create_uno_genome function**

Add to `src/darwindeck/genome/examples.py`:

```python
def create_uno_genome() -> GameGenome:
    """
    Uno-style game with special effects.

    Mechanics:
    - Match rank or suit of top discard
    - 2s force next player to draw 2
    - Jacks skip next player
    - Queens reverse direction
    - Kings give extra turn
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="uno-style",
        generation=0,
        setup=SetupRules(
            cards_per_player=7,
            initial_discard_count=1,  # Start with one card face up
        ),
        turn_structure=TurnStructure(phases=[
            PlayPhase(
                target=Location.DISCARD,
                valid_play_condition=CompoundCondition(
                    logic="OR",
                    conditions=[
                        Condition(type=ConditionType.CARD_MATCHES_RANK, reference="top_discard"),
                        Condition(type=ConditionType.CARD_MATCHES_SUIT, reference="top_discard"),
                    ]
                ),
                min_cards=1,
                max_cards=1,
                mandatory=False,  # Can choose not to play
                pass_if_unable=True,
            ),
            DrawPhase(
                source=Location.DECK,
                count=1,
                mandatory=False,  # Only draw if didn't play
                condition=Condition(
                    type=ConditionType.HAND_SIZE,
                    operator=Operator.EQ,
                    value=0,  # Dummy - controlled by pass_if_unable
                ),
            ),
        ]),
        special_effects=[
            SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
            SpecialEffect(Rank.JACK, EffectType.SKIP_NEXT, TargetSelector.NEXT_PLAYER, 1),
            SpecialEffect(Rank.QUEEN, EffectType.REVERSE_DIRECTION, TargetSelector.ALL_OPPONENTS, 1),
            SpecialEffect(Rank.KING, EffectType.EXTRA_TURN, TargetSelector.NEXT_PLAYER, 1),
        ],
        win_conditions=[
            WinCondition(type="empty_hand"),
        ],
        scoring_rules=[],
        max_turns=200,
        player_count=2,
    )
```

Add imports at top:

```python
from darwindeck.genome.schema import SpecialEffect, EffectType
```

**Step 2: Add to get_seed_genomes**

Update `get_seed_genomes()` to include the new game:

```python
def get_seed_genomes() -> list[GameGenome]:
    return [
        # ... existing games ...
        create_uno_genome(),
    ]
```

**Step 3: Test the genome can be created**

Run: `uv run python -c "from darwindeck.genome.examples import create_uno_genome; g = create_uno_genome(); print(f'Created {g.genome_id} with {len(g.special_effects)} effects')"`
Expected: "Created uno-style with 4 effects"

**Step 4: Commit**

```bash
git add src/darwindeck/genome/examples.py
git commit -m "feat(examples): add Uno-style seed game with special effects"
```

---

## Task 11: End-to-End Integration Test

**Files:**
- Create: `tests/integration/test_special_effects.py`

**Step 1: Write integration test**

Create `tests/integration/test_special_effects.py`:

```python
"""Integration tests for special effects system."""

import pytest
from darwindeck.genome.examples import create_uno_genome
from darwindeck.simulation.go_simulator import GoSimulator


def test_uno_game_runs_without_errors():
    """Uno-style game with effects runs through Go simulator."""
    genome = create_uno_genome()

    simulator = GoSimulator()
    results = simulator.simulate(genome, num_games=50)

    assert results.errors == 0, f"Simulation had {results.errors} errors"
    assert results.games_played == 50
    assert results.avg_turns > 5, "Game should last more than 5 turns"


def test_effect_bytecode_roundtrip():
    """Effects survive Python->bytecode->Go->execution roundtrip."""
    from darwindeck.genome.bytecode import compile_effects
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector

    effects = [
        SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        SpecialEffect(Rank.JACK, EffectType.SKIP_NEXT, TargetSelector.NEXT_PLAYER, 1),
    ]

    bytecode = compile_effects(effects)

    # Should be: header(1) + count(1) + 2*effect(4) = 10 bytes
    assert len(bytecode) == 10

    # Verify structure
    assert bytecode[0] == 60  # EFFECT_HEADER
    assert bytecode[1] == 2   # count


def test_evolution_with_effects():
    """Evolution can add/remove/mutate effects."""
    from darwindeck.evolution.operators import (
        AddEffectMutation, RemoveEffectMutation, MutateEffectMutation
    )
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()
    assert len(genome.special_effects) == 0

    # Add effect
    add_mut = AddEffectMutation()
    genome = add_mut.apply(genome)
    assert len(genome.special_effects) == 1

    # Mutate effect
    mutate_mut = MutateEffectMutation()
    genome = mutate_mut.apply(genome)
    assert len(genome.special_effects) == 1

    # Remove effect
    remove_mut = RemoveEffectMutation()
    genome = remove_mut.apply(genome)
    assert len(genome.special_effects) == 0
```

**Step 2: Run integration tests**

Run: `uv run pytest tests/integration/test_special_effects.py -v`
Expected: PASS (3 tests)

**Step 3: Commit**

```bash
git add tests/integration/test_special_effects.py
git commit -m "test: add integration tests for special effects system"
```

---

## Task 12: Update ROADMAP

**Files:**
- Modify: `ROADMAP.md`

**Step 1: Move Special Effects to Completed**

Update `ROADMAP.md`:

1. Remove from Planned Features:
```markdown
**Special Effects System**
- [ ] `SpecialEffect` - Card-triggered actions
- [ ] `ActionType` enum for effect actions
- [ ] Trigger conditions (rank, suit, location)
```

2. Add to Completed section:
```markdown
### Special Effects System (Complete)
- [x] EffectType enum and SpecialEffect dataclass
- [x] Go execution (ApplyEffect, AdvanceTurn)
- [x] Bytecode encoding and parsing
- [x] Evolution mutation operators
- [x] Uno-style seed game
```

3. Update Current Status to mention effects.

**Step 2: Commit**

```bash
git add ROADMAP.md
git commit -m "docs: mark special effects system as complete"
```

---

## Summary

**Total Tasks:** 12

**Files Created:**
- `src/gosim/engine/effects.go`
- `src/gosim/engine/effects_test.go`
- `tests/integration/test_special_effects.py`

**Files Modified:**
- `src/darwindeck/genome/schema.py` - EffectType, SpecialEffect
- `src/darwindeck/genome/bytecode.py` - Opcodes, compile_effects
- `src/darwindeck/evolution/operators.py` - 3 mutation operators
- `src/darwindeck/genome/examples.py` - Uno genome
- `src/gosim/engine/types.go` - PlayDirection, SkipCount
- `src/gosim/engine/bytecode.go` - parseEffects, ParsedGenome.Effects
- `src/gosim/engine/movegen.go` - Hook effects into ApplyMove
- `ROADMAP.md` - Documentation update

**Test Commands:**
```bash
# Python tests
uv run pytest tests/unit/test_schema.py -v -k effect
uv run pytest tests/unit/test_bytecode.py -v -k effect
uv run pytest tests/unit/test_operators.py -v -k effect
uv run pytest tests/integration/test_special_effects.py -v

# Go tests
cd src/gosim && go test ./engine -v
```
