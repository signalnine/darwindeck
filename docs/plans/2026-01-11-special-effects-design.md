# Special Effects System Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add card-triggered immediate effects (Skip, Reverse, Draw N) to enable Uno-style games.

**Architecture:** Effects are triggered by card rank, execute immediately when played, and modify turn order or force card movement. Effects are encoded in bytecode and looked up by rank in Go.

**Tech Stack:** Python (schema, bytecode compiler, evolution), Go (execution), FlatBuffers (serialization)

---

## Data Model

### New Types (Python: `src/darwindeck/genome/schema.py`)

```python
class EffectType(Enum):
    """Types of immediate effects a card can trigger."""
    SKIP_NEXT = "skip_next"           # Skip the next player's turn
    REVERSE_DIRECTION = "reverse"      # Reverse play direction
    DRAW_CARDS = "draw_cards"          # Force target to draw N cards
    EXTRA_TURN = "extra_turn"          # Current player goes again
    FORCE_DISCARD = "force_discard"    # Force target to discard N cards


@dataclass(frozen=True)
class SpecialEffect:
    """A card-triggered immediate effect."""

    trigger_rank: Rank              # Which rank triggers this effect
    effect_type: EffectType         # What happens
    target: TargetSelector          # Who is affected (NEXT_PLAYER, ALL_OPPONENTS, etc.)
    value: int = 1                  # Magnitude (cards to draw, turns to skip)
```

### Example Usage

```python
# Uno-style effects
special_effects = [
    SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, value=2),
    SpecialEffect(Rank.JACK, EffectType.SKIP_NEXT, TargetSelector.NEXT_PLAYER),
    SpecialEffect(Rank.QUEEN, EffectType.REVERSE_DIRECTION, TargetSelector.ALL_OPPONENTS),
    SpecialEffect(Rank.KING, EffectType.EXTRA_TURN, TargetSelector.NEXT_PLAYER),
]
```

---

## Game State Changes

### Go State (src/gosim/engine/types.go)

Add two fields to GameState:

```go
type GameState struct {
    // ... existing fields ...

    PlayDirection    int8   // 1 = clockwise, -1 = counter-clockwise
    SkipCount        uint8  // Number of players to skip (usually 0 or 1)
}
```

Initialize in `NewGameState`:
```go
PlayDirection: 1,
SkipCount:     0,
```

### Turn Advancement (src/gosim/engine/movegen.go)

Current logic:
```go
nextPlayer = (currentPlayer + 1) % numPlayers
```

New logic:
```go
func (state *GameState) AdvanceTurn() {
    step := int(state.PlayDirection)  // +1 or -1
    next := int(state.CurrentPlayer)

    // Always advance at least once, plus any skips
    for i := 0; i <= int(state.SkipCount); i++ {
        next = (next + step + int(state.NumPlayers)) % int(state.NumPlayers)
    }

    state.CurrentPlayer = uint8(next)
    state.SkipCount = 0  // Reset after applying
}
```

---

## Effect Execution

### Effect Lookup (src/gosim/engine/effects.go - new file)

```go
type SpecialEffect struct {
    TriggerRank  uint8
    EffectType   uint8
    Target       uint8
    Value        uint8
}

const (
    EFFECT_SKIP_NEXT = iota
    EFFECT_REVERSE
    EFFECT_DRAW_CARDS
    EFFECT_EXTRA_TURN
    EFFECT_FORCE_DISCARD
)

func ApplyEffect(state *GameState, effect *SpecialEffect) {
    switch effect.EffectType {
    case EFFECT_SKIP_NEXT:
        state.SkipCount += effect.Value

    case EFFECT_REVERSE:
        state.PlayDirection *= -1

    case EFFECT_DRAW_CARDS:
        targetID := resolveTarget(state, effect.Target)
        for i := uint8(0); i < effect.Value && len(state.Deck) > 0; i++ {
            card := state.DrawCard()
            state.Players[targetID].Hand = append(state.Players[targetID].Hand, card)
        }

    case EFFECT_EXTRA_TURN:
        // Skip everyone else = current player goes again
        state.SkipCount = state.NumPlayers - 1

    case EFFECT_FORCE_DISCARD:
        targetID := resolveTarget(state, effect.Target)
        hand := &state.Players[targetID].Hand
        toDiscard := min(int(effect.Value), len(*hand))
        for i := 0; i < toDiscard; i++ {
            card := (*hand)[len(*hand)-1]
            *hand = (*hand)[:len(*hand)-1]
            state.Discard = append(state.Discard, card)
        }
    }
}

func resolveTarget(state *GameState, target uint8) int {
    current := int(state.CurrentPlayer)
    numPlayers := int(state.NumPlayers)
    direction := int(state.PlayDirection)

    switch target {
    case TARGET_NEXT_PLAYER:
        return (current + direction + numPlayers) % numPlayers
    case TARGET_PREV_PLAYER:
        return (current - direction + numPlayers) % numPlayers
    case TARGET_RANDOM_OPPONENT:
        // Pick random opponent
        offset := rand.Intn(numPlayers-1) + 1
        return (current + offset) % numPlayers
    default:
        return (current + 1) % numPlayers
    }
}
```

### Integration with ApplyMove

In `ApplyMove`, after playing a card:

```go
func ApplyMove(state *GameState, move Move, genome *ParsedGenome) {
    // ... existing move application ...

    // Check for special effect
    if move.Type == MOVE_PLAY {
        playedCard := move.Cards[0]
        if effect, ok := genome.Effects[playedCard.Rank]; ok {
            ApplyEffect(state, &effect)
        }
    }

    // Advance turn (now uses PlayDirection and SkipCount)
    state.AdvanceTurn()
}
```

---

## Bytecode Encoding

### New OpCodes (src/darwindeck/genome/bytecode.py)

```python
class OpCode(Enum):
    # ... existing opcodes ...

    # Special Effects (60-69)
    EFFECT_HEADER = 60      # Start of effects section
    EFFECT_ENTRY = 61       # Single effect definition
```

### Effect Encoding (4 bytes per effect)

```
Byte 0: trigger_rank (0-12 for 2-A)
Byte 1: effect_type (0-4 for EffectType enum)
Byte 2: target_selector (0-6 for TargetSelector enum)
Byte 3: value (0-255)
```

### Bytecode Structure

```
[Header: 36 bytes]
[Phases: variable]
[Win conditions: variable]
[Effects section:]
  - EFFECT_HEADER opcode (1 byte)
  - effect_count (1 byte)
  - effect entries (4 bytes each)
```

### Python Compilation

```python
def compile_effects(effects: list[SpecialEffect]) -> bytes:
    result = bytes([OpCode.EFFECT_HEADER.value, len(effects)])
    for effect in effects:
        result += bytes([
            RANK_TO_BYTE[effect.trigger_rank],
            effect.effect_type.value,
            TARGET_TO_BYTE[effect.target],
            effect.value,
        ])
    return result
```

### Go Parsing

```go
type ParsedGenome struct {
    // ... existing fields ...
    Effects map[uint8]SpecialEffect  // rank -> effect lookup
}

func parseEffects(data []byte, offset int) (map[uint8]SpecialEffect, int) {
    effects := make(map[uint8]SpecialEffect)

    if data[offset] != OP_EFFECT_HEADER {
        return effects, offset
    }
    offset++

    count := int(data[offset])
    offset++

    for i := 0; i < count; i++ {
        effect := SpecialEffect{
            TriggerRank: data[offset],
            EffectType:  data[offset+1],
            Target:      data[offset+2],
            Value:       data[offset+3],
        }
        effects[effect.TriggerRank] = effect
        offset += 4
    }

    return effects, offset
}
```

---

## Evolution Operators

### New Mutations (src/darwindeck/evolution/operators.py)

```python
@dataclass
class AddEffectMutation(Mutation):
    """Add a random special effect."""

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
        return genome.copy_with(
            special_effects=list(genome.special_effects) + [new_effect]
        )


@dataclass
class RemoveEffectMutation(Mutation):
    """Remove a random special effect."""

    def apply(self, genome: GameGenome) -> GameGenome:
        if not genome.special_effects:
            return genome
        idx = random.randrange(len(genome.special_effects))
        new_effects = [e for i, e in enumerate(genome.special_effects) if i != idx]
        return genome.copy_with(special_effects=new_effects)


@dataclass
class MutateEffectMutation(Mutation):
    """Mutate one field of a random special effect."""

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
            mutated = SpecialEffect(
                effect.trigger_rank,
                effect.effect_type,
                effect.target,
                max(1, effect.value + random.randint(-1, 1)),
            )

        new_effects = list(genome.special_effects)
        new_effects[idx] = mutated
        return genome.copy_with(special_effects=new_effects)
```

### Crossover

Combine effects from both parents:

```python
def crossover_effects(parent1: GameGenome, parent2: GameGenome) -> list[SpecialEffect]:
    # Union of effects, deduplicated by trigger rank
    effects_by_rank = {}
    for effect in parent1.special_effects:
        effects_by_rank[effect.trigger_rank] = effect
    for effect in parent2.special_effects:
        if random.random() < 0.5:  # 50% chance to override
            effects_by_rank[effect.trigger_rank] = effect
    return list(effects_by_rank.values())
```

---

## Testing Strategy

### Unit Tests (Go)

```go
func TestSkipNextEffect(t *testing.T) {
    state := NewGameState(3)
    state.CurrentPlayer = 0
    state.PlayDirection = 1

    effect := SpecialEffect{EffectType: EFFECT_SKIP_NEXT, Value: 1}
    ApplyEffect(state, &effect)
    state.AdvanceTurn()

    assert.Equal(t, uint8(2), state.CurrentPlayer)  // Skipped player 1
}

func TestReverseDirection(t *testing.T) {
    state := NewGameState(4)
    state.PlayDirection = 1

    effect := SpecialEffect{EffectType: EFFECT_REVERSE}
    ApplyEffect(state, &effect)

    assert.Equal(t, int8(-1), state.PlayDirection)
}

func TestReverseWithAdvance(t *testing.T) {
    state := NewGameState(4)
    state.CurrentPlayer = 1
    state.PlayDirection = 1

    effect := SpecialEffect{EffectType: EFFECT_REVERSE}
    ApplyEffect(state, &effect)
    state.AdvanceTurn()

    assert.Equal(t, uint8(0), state.CurrentPlayer)  // Went backward
}

func TestDrawCardsEffect(t *testing.T) {
    state := NewGameState(2)
    state.Deck = []Card{{Rank: 5}, {Rank: 7}, {Rank: 9}}
    initialHandSize := len(state.Players[1].Hand)

    effect := SpecialEffect{
        EffectType: EFFECT_DRAW_CARDS,
        Target:     TARGET_NEXT_PLAYER,
        Value:      2,
    }
    ApplyEffect(state, &effect)

    assert.Equal(t, initialHandSize+2, len(state.Players[1].Hand))
    assert.Equal(t, 1, len(state.Deck))
}

func TestExtraTurn(t *testing.T) {
    state := NewGameState(3)
    state.CurrentPlayer = 1

    effect := SpecialEffect{EffectType: EFFECT_EXTRA_TURN}
    ApplyEffect(state, &effect)
    state.AdvanceTurn()

    assert.Equal(t, uint8(1), state.CurrentPlayer)  // Same player
}

func TestForceDiscard(t *testing.T) {
    state := NewGameState(2)
    state.Players[1].Hand = []Card{{Rank: 2}, {Rank: 5}, {Rank: 8}}

    effect := SpecialEffect{
        EffectType: EFFECT_FORCE_DISCARD,
        Target:     TARGET_NEXT_PLAYER,
        Value:      2,
    }
    ApplyEffect(state, &effect)

    assert.Equal(t, 1, len(state.Players[1].Hand))
    assert.Equal(t, 2, len(state.Discard))
}
```

### Integration Tests (Python)

```python
def test_uno_style_game_runs():
    """Uno-style game with effects completes without errors."""
    genome = create_uno_genome()
    results = simulator.simulate(genome, num_games=100)

    assert results.errors == 0
    assert results.avg_turns > 10

def test_effect_bytecode_roundtrip():
    """Effects survive bytecode compilation and parsing."""
    effects = [
        SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        SpecialEffect(Rank.JACK, EffectType.SKIP_NEXT, TargetSelector.NEXT_PLAYER, 1),
    ]
    genome = create_test_genome(special_effects=effects)

    bytecode = compile_genome(genome)
    # Go parses and executes - if no errors, roundtrip works
    results = simulator.simulate_from_bytecode(bytecode, num_games=10)
    assert results.errors == 0

def test_effect_evolution():
    """Effects can be added/removed/mutated during evolution."""
    genome = create_test_genome(special_effects=[])

    # Add effect
    add_mutation = AddEffectMutation()
    mutated = add_mutation.apply(genome)
    assert len(mutated.special_effects) == 1

    # Mutate effect
    mutate_mutation = MutateEffectMutation()
    mutated2 = mutate_mutation.apply(mutated)
    assert len(mutated2.special_effects) == 1

    # Remove effect
    remove_mutation = RemoveEffectMutation()
    mutated3 = remove_mutation.apply(mutated2)
    assert len(mutated3.special_effects) == 0
```

### Property-Based Tests

```python
@given(
    num_players=st.integers(2, 4),
    direction=st.sampled_from([1, -1]),
    skip_count=st.integers(0, 3),
)
def test_turn_advancement_always_valid(num_players, direction, skip_count):
    """Turn always advances to valid player index."""
    state = create_test_state(num_players=num_players)
    state.play_direction = direction
    state.skip_count = min(skip_count, num_players - 1)

    state.advance_turn()

    assert 0 <= state.current_player < num_players

@given(
    deck_size=st.integers(0, 52),
    draw_count=st.integers(1, 10),
)
def test_draw_effect_never_overdraws(deck_size, draw_count):
    """Draw effect never tries to draw more than deck contains."""
    state = create_test_state()
    state.deck = [Card(i % 13, i // 13) for i in range(deck_size)]
    initial_hand = len(state.players[1].hand)

    effect = SpecialEffect(type=DRAW_CARDS, target=NEXT_PLAYER, value=draw_count)
    apply_effect(state, effect)

    drawn = len(state.players[1].hand) - initial_hand
    assert drawn <= deck_size
    assert drawn == min(draw_count, deck_size)
```

---

## Implementation Order

1. **Python schema** - Add EffectType, SpecialEffect to schema.py
2. **Go state** - Add PlayDirection, SkipCount to GameState
3. **Go effects** - Create effects.go with ApplyEffect and resolveTarget
4. **Go turn advancement** - Update AdvanceTurn to use direction and skips
5. **Go integration** - Hook effects into ApplyMove
6. **Bytecode** - Add opcodes and compile/parse logic
7. **Python mutations** - Add effect mutation operators
8. **Tests** - Unit tests, integration tests, property tests
9. **Example genome** - Create Uno-style seed game

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Trigger mechanism | By rank only | Simple, evolvable, matches real games |
| Effect timing | Immediate | No stacking complexity |
| State storage | PlayDirection + SkipCount | Minimal state, handles all turn effects |
| Bytecode format | 4 bytes per effect | Compact, fixed-size entries |
| Effect lookup | Map by rank | O(1) lookup during play |

---

## Out of Scope

- **Stacking effects** - Can add later as optional flag
- **Conditional effects** - Effect only triggers if condition met
- **Multi-card triggers** - Effect requires specific card (rank + suit)
- **Reactive effects** - Real-time reactions (not turn-based)
- **Chain effects** - Effect triggers another effect
