# Team Play Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add team/partnership support to enable games like Spades (2v2) where players on the same team share scores and win together.

**Architecture:** Flexible team assignments via genome config, shared score pools per team, mode flag that makes existing win conditions evaluate team aggregates instead of individual scores.

**Tech Stack:** Go (simulation), Python (schema, evolution), FlatBuffers (serialization)

---

## Problem Statement

Current system only supports individual play - each player competes alone. Many classic card games (Spades, Bridge, Euchre) feature partnerships where two players form a team, share scores, and win or lose together. Without team support, these games cannot be evolved or simulated.

**Use cases:**
- Partnership trick-taking (Spades, Bridge) - combined trick counts
- Team scoring (Rummy variants) - shared score pool
- Asymmetric teams (3v1) - interesting evolutionary possibilities

---

## Schema Changes

### New Fields on GameGenome

```python
@dataclass(frozen=True)
class GameGenome:
    # ... existing fields ...

    # Team play configuration
    team_mode: bool = False  # When True, win conditions evaluate team aggregates
    teams: tuple[tuple[int, ...], ...] = ()  # e.g., ((0, 2), (1, 3)) for 2v2
```

### Validation Rules

- If `team_mode=True`, `teams` must be non-empty
- All player indices in `teams` must be < `player_count`
- Each player appears in exactly one team
- At least 2 teams required when `team_mode=True`

### Example Configurations

| Config | teams | Description |
|--------|-------|-------------|
| 2v2 partnership | `((0, 2), (1, 3))` | Players across from each other |
| 3v1 | `((0, 1, 2), (3,))` | Three vs one |
| 2v2v2 (6-player) | `((0, 3), (1, 4), (2, 5))` | Three teams of two |
| Free-for-all | `()` with `team_mode=False` | Default individual play |

---

## State Tracking

### Go GameState

```go
type GameState struct {
    // ... existing fields ...
    TeamScores []int32  // One entry per team, empty if not team_mode
}
```

### Python GameState

```python
@dataclass(frozen=True)
class GameState:
    # ... existing fields ...
    team_scores: tuple[int, ...] = ()  # One entry per team
```

### Score Flow

1. When a player scores points and `team_mode=True`:
   - Lookup player's team via `get_team_for_player(player_idx, teams)`
   - Add points to `TeamScores[team_idx]` instead of `PlayerState.score`
2. Individual `PlayerState.score` stays at 0 in team mode
3. For trick-taking: trick winner's team gets trick counted toward team total

### Initialization

- At game start, if `team_mode=True`, initialize `TeamScores` with zeros
- Length matches `len(genome.teams)`

---

## Win Condition Evaluation

### Team-Aware Evaluation

When `team_mode=True`, win conditions evaluate team aggregates:

| Win Type | Individual Mode | Team Mode |
|----------|-----------------|-----------|
| `high_score` | Player with highest score | Team with highest team score |
| `first_to_score` | First player to reach threshold | First team to reach threshold |
| `empty_hand` | First player to empty hand | First team where ANY member empties hand |
| `capture_all` | Player captures all cards | Team collectively captures all |
| `best_hand` | Player with best poker hand | Best hand among all team members |

### Winner Identification

- In team mode, `winner` field returns winning *team index* (0, 1, ...), not player index
- Fitness evaluation handles team winners appropriately
- Ties between teams return -1 (draw)

### Implementation

```go
func CheckWinConditions(state *GameState, genome *Genome) int {
    if genome.TeamMode {
        return checkTeamWinConditions(state, genome)
    }
    return checkIndividualWinConditions(state, genome)  // existing logic
}
```

---

## Evolution and Mutation

### New Mutation Operators

1. **EnableTeamModeMutation**
   - Converts individual game to team game
   - Sets `team_mode=True`
   - Generates valid team assignment based on `player_count`
   - For 4 players: randomly choose `((0,2),(1,3))`, `((0,1),(2,3))`, or `((0,3),(1,2))`

2. **DisableTeamModeMutation**
   - Converts team game back to individual
   - Sets `team_mode=False`, clears `teams`

3. **MutateTeamAssignmentMutation**
   - Shuffles team membership (only when `team_mode=True`)
   - Maintains valid team count and sizes

### Crossover Handling

- If one parent has `team_mode=True` and other doesn't, randomly inherit from one
- If both have teams, can swap team configurations

### Constraints

- `team_mode=True` requires `player_count >= 2`
- Minimum 2 teams when team mode enabled
- Low mutation rates (team mode is structural change)

---

## Implementation Scope

### Files to Modify

| Layer | Files | Changes |
|-------|-------|---------|
| Schema | `genome/schema.py` | Add `team_mode`, `teams` fields |
| Bytecode | `genome/bytecode.py` | Encode team config in header |
| Go Types | `gosim/engine/types.go` | Add `TeamScores`, team helpers |
| Go Win Check | `gosim/engine/conditions.go` | Team-aware win evaluation |
| Go Scoring | `gosim/simulation/runner.go` | Route scores to team totals |
| FlatBuffers | `schema/simulation.fbs` | Add team fields to results |
| Python State | `simulation/state.py` | Add `team_scores` field |
| Mutations | `evolution/mutation.py` | Add team mutation operators |
| Seed Games | `genome/examples.py` | Add Spades partnership variant |

### Out of Scope

- Partner hand visibility (future enhancement)
- Complex turn order (partner leads after trick win)
- Team bidding/contracts (separate roadmap item)
- Team-based AI strategies (teams play independently)

---

## Testing Strategy

### Unit Tests

| Test | Validates |
|------|-----------|
| `test_team_assignment_validation` | Invalid team configs rejected |
| `test_get_team_for_player` | Helper returns correct team index |
| `test_team_score_accumulation` | Points go to team, not player |
| `test_team_win_high_score` | Highest team score wins |
| `test_team_win_first_to_score` | First team to threshold wins |
| `test_team_win_empty_hand` | Any member emptying hand wins for team |
| `test_individual_mode_unchanged` | Existing games work without teams |

### Integration Tests

| Test | Validates |
|------|-----------|
| `test_partnership_spades_simulation` | Full game with team scoring |
| `test_team_mode_evolution` | Mutations produce valid team configs |
| `test_team_fitness_evaluation` | Fitness works with team winners |

### Seed Game for Testing

Partnership Spades:
- 4 players
- `teams=((0, 2), (1, 3))`
- Trick-taking with team scoring
- First team to 500 points wins

---

## Backward Compatibility

- Default `team_mode=False` and `teams=()` means existing genomes unchanged
- No migration needed
- Individual games continue working exactly as before
- FlatBuffers fields default to empty/false
