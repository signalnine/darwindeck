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
- **Validation runs at genome creation and after every mutation**

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

    // Team play state
    TeamScores    []int32  // One entry per team, empty if not team_mode
    PlayerToTeam  []int8   // Precomputed lookup: player_idx -> team_idx (-1 if no teams)
}
```

### Python GameState

```python
@dataclass(frozen=True)
class GameState:
    # ... existing fields ...

    # Team play state
    team_scores: tuple[int, ...] = ()      # One entry per team
    player_to_team: tuple[int, ...] = ()   # Precomputed lookup: player_idx -> team_idx
```

### Precomputed Team Lookup (MODERATE fix)

At game initialization, build `PlayerToTeam` lookup table:

```go
func buildPlayerToTeamLookup(teams [][]int, numPlayers int) []int8 {
    lookup := make([]int8, numPlayers)
    for i := range lookup {
        lookup[i] = -1  // Default: no team
    }
    for teamIdx, team := range teams {
        for _, playerIdx := range team {
            lookup[playerIdx] = int8(teamIdx)
        }
    }
    return lookup
}
```

This avoids O(N) team lookups during scoring - now O(1) via `PlayerToTeam[playerIdx]`.

### Score Flow (STRONG fix - preserve individual scores)

**Both individual AND team scores are tracked:**

1. When a player scores points:
   - **Always** add to `PlayerState.score` (preserves turn-order logic, trick winner identification)
   - **Additionally**, if `team_mode=True`, add to `TeamScores[PlayerToTeam[playerIdx]]`
2. Individual scores remain populated for:
   - Determining who leads next trick (trick winner)
   - AI training signals
   - Display/debugging
3. Win conditions check `TeamScores` when `team_mode=True`

### Initialization

- At game start, if `team_mode=True`:
  - Initialize `TeamScores` with zeros, length = `len(genome.teams)`
  - Build `PlayerToTeam` lookup table
- If `team_mode=False`:
  - `TeamScores` stays empty
  - `PlayerToTeam` stays empty (or all -1)

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

### Winner Identification (STRONG fix - separate fields)

**Use separate fields for player and team winners to avoid ambiguity:**

```go
type GameResult struct {
    Winner         int   // Winning player index (-1 for draw, -1 if team mode)
    WinningTeam    int   // Winning team index (-1 for draw, -1 if not team mode)
    // ... other fields
}
```

In Python:
```python
@dataclass(frozen=True)
class GameResult:
    winner: int = -1          # Winning player index (-1 if draw or team mode)
    winning_team: int = -1    # Winning team index (-1 if draw or not team mode)
```

**Semantics:**
- Individual mode: `winner` is set, `winning_team` is -1
- Team mode: `winning_team` is set, `winner` is -1
- Draw: both are -1

### Implementation

```go
func CheckWinConditions(state *GameState, genome *Genome) (playerWinner int, teamWinner int) {
    if genome.TeamMode {
        return -1, checkTeamWinConditions(state, genome)
    }
    return checkIndividualWinConditions(state, genome), -1
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

### Mutation Constraints (STRONG fix - team/player sync)

**Player count changes must reconcile with teams:**

```python
def mutate_player_count(genome: GameGenome, new_count: int) -> GameGenome:
    if not genome.team_mode:
        # No teams, just change count
        return genome.copy_with(player_count=new_count)

    # Team mode: must repair or reject
    if not can_repair_teams(genome.teams, new_count):
        # Cannot create valid teams - disable team mode
        return genome.copy_with(
            player_count=new_count,
            team_mode=False,
            teams=()
        )

    # Repair teams for new player count
    new_teams = repair_teams(genome.teams, new_count)
    return genome.copy_with(player_count=new_count, teams=new_teams)
```

**Validation after every mutation:**
```python
def validate_genome(genome: GameGenome) -> bool:
    if genome.team_mode:
        if not genome.teams:
            return False  # teams required
        all_players = set()
        for team in genome.teams:
            for p in team:
                if p >= genome.player_count:
                    return False  # invalid player index
                if p in all_players:
                    return False  # duplicate player
                all_players.add(p)
        if len(all_players) != genome.player_count:
            return False  # not all players assigned
        if len(genome.teams) < 2:
            return False  # need at least 2 teams
    return True
```

### Crossover Handling

- If one parent has `team_mode=True` and other doesn't, randomly inherit from one
- If both have teams, can swap team configurations
- **Always validate result; if invalid, fall back to one parent's config**

### Constraints

- `team_mode=True` requires `player_count >= 2`
- Minimum 2 teams when team mode enabled
- Low mutation rates (team mode is structural change)

---

## Agent Team Awareness (MODERATE fix)

**Agents need to know their team membership for meaningful partnership play:**

### Go Agent Input

```go
type AgentContext struct {
    PlayerIdx    int
    TeamIdx      int   // -1 if not team mode
    PartnerIdxs  []int // Indices of teammates (empty if not team mode)
    // ... existing fields
}
```

### Usage

- AI can use `TeamIdx` to identify allies vs opponents
- For Spades: agent knows not to trump partner's winning card
- For Bridge: agent can consider partner's likely holdings

**Note:** This provides team *identity*, not partner hand visibility. Agents still cannot see partner's cards.

---

## FlatBuffers Schema (MODERATE fix)

### Explicit Schema Definition

```fbs
// In schema/simulation.fbs

table GameResult {
    winner: int32 = -1;           // Winning player (-1 if draw or team mode)
    winning_team: int32 = -1;     // Winning team (-1 if draw or not team mode)
    // ... existing fields
}

table AggregatedStats {
    // ... existing fields ...

    // Team play results
    team_wins: [uint32];          // Win count per team (empty if not team mode)
}

table SimulationRequest {
    // ... existing fields ...

    // Team configuration
    team_mode: bool = false;
    teams: [TeamAssignment];      // Team definitions
}

table TeamAssignment {
    player_indices: [uint8];      // Players on this team
}
```

---

## Implementation Scope

### Files to Modify

| Layer | Files | Changes |
|-------|-------|---------|
| Schema | `genome/schema.py` | Add `team_mode`, `teams` fields |
| Validation | `genome/validation.py` | Team validation logic |
| Bytecode | `genome/bytecode.py` | Encode team config in header |
| Go Types | `gosim/engine/types.go` | Add `TeamScores`, `PlayerToTeam`, team helpers |
| Go Win Check | `gosim/engine/conditions.go` | Team-aware win evaluation, separate winner fields |
| Go Scoring | `gosim/simulation/runner.go` | Dual scoring (individual + team) |
| Go Agent | `gosim/mcts/` | Add `TeamIdx` to agent context |
| FlatBuffers | `schema/simulation.fbs` | Add team fields per schema above |
| Python State | `simulation/state.py` | Add `team_scores`, `player_to_team` |
| Mutations | `evolution/mutation.py` | Add team mutations with validation |
| Seed Games | `genome/examples.py` | Add Spades partnership variant |

### Out of Scope

- Partner hand visibility (future enhancement)
- Complex turn order (partner leads after trick win)
- Team bidding/contracts (separate roadmap item)

---

## Testing Strategy (MODERATE fix - expanded coverage)

### Unit Tests

| Test | Validates |
|------|-----------|
| `test_team_assignment_validation` | Invalid team configs rejected |
| `test_team_validation_player_count_mismatch` | Rejects teams with wrong player indices |
| `test_team_validation_duplicate_player` | Rejects teams with same player twice |
| `test_player_to_team_lookup` | Precomputed lookup is correct |
| `test_dual_score_tracking` | Both individual and team scores updated |
| `test_team_win_high_score` | Highest team score wins |
| `test_team_win_first_to_score` | First team to threshold wins |
| `test_team_win_empty_hand` | Any member emptying hand wins for team |
| `test_individual_mode_unchanged` | Existing games work without teams |
| `test_winner_field_semantics` | `winner` vs `winning_team` set correctly |

### Integration Tests

| Test | Validates |
|------|-----------|
| `test_partnership_spades_simulation` | Full game with team scoring |
| `test_team_mode_evolution` | Mutations produce valid team configs |
| `test_team_fitness_evaluation` | Fitness works with team winners |
| `test_player_count_mutation_with_teams` | Player count change repairs/disables teams |
| `test_flatbuffers_team_roundtrip` | Serialization/deserialization preserves teams |
| `test_team_ties` | Draw handling when teams tie |
| `test_unequal_team_sizes` | 3v1 configuration works |

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
- FlatBuffers fields default to empty/false/-1
- `winner` field semantics unchanged for individual games
- New `winning_team` field is -1 for all existing games

---

## Summary of Fixes from Multi-Agent Review

| Issue | Severity | Fix |
|-------|----------|-----|
| Winner field ambiguity | STRONG | Separate `winner` and `winning_team` fields |
| Team/player count sync | STRONG | Validation after mutations, auto-repair or disable |
| Individual score zeroing | STRONG | Dual scoring - both individual AND team tracked |
| O(N) team lookup | MODERATE | Precomputed `PlayerToTeam` lookup table |
| FlatBuffers underspecified | MODERATE | Explicit schema with `TeamAssignment` table |
| Testing gaps | MODERATE | Added 7 new test cases |
| Agent team awareness | MODERATE | Added `TeamIdx` and `PartnerIdxs` to agent context |
