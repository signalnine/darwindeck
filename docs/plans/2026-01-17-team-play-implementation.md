# Team Play Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add team/partnership support to enable games like Spades (2v2) where players on the same team share scores and win together.

**Architecture:** Flexible team assignments via genome config (`team_mode`, `teams` fields), shared score pools per team (`TeamScores`), precomputed `PlayerToTeam` lookup, separate `winner` and `winning_team` result fields, dual scoring (both individual AND team scores tracked).

**Tech Stack:** Python (schema, evolution, validation), Go (simulation engine), FlatBuffers (serialization)

---

## Task 1: Add team_mode and teams Fields to GameGenome

**Files:**
- Modify: `src/darwindeck/genome/schema.py`
- Test: `tests/unit/test_genome_schema.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_genome_schema.py`:

```python
def test_game_genome_team_fields_default():
    """Test that GameGenome has team_mode and teams fields with correct defaults."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )
    assert genome.team_mode is False
    assert genome.teams == ()


def test_game_genome_team_fields_set():
    """Test that GameGenome accepts team configuration."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=4,
        team_mode=True,
        teams=((0, 2), (1, 3)),
    )
    assert genome.team_mode is True
    assert genome.teams == ((0, 2), (1, 3))
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_genome_schema.py::test_game_genome_team_fields_default -v`
Expected: FAIL with "unexpected keyword argument 'team_mode'"

**Step 3: Write minimal implementation**

Add to `GameGenome` dataclass in `src/darwindeck/genome/schema.py` (after `game_rules` field, around line 425):

```python
    # Team play configuration
    team_mode: bool = False  # When True, win conditions evaluate team aggregates
    teams: tuple[tuple[int, ...], ...] = ()  # e.g., ((0, 2), (1, 3)) for 2v2
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_genome_schema.py::test_game_genome_team_fields_default tests/unit/test_genome_schema.py::test_game_genome_team_fields_set -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/schema.py tests/unit/test_genome_schema.py
git commit -m "feat(schema): add team_mode and teams fields to GameGenome"
```

---

## Task 2: Add Team Validation to GenomeValidator

**Files:**
- Modify: `src/darwindeck/genome/validator.py`
- Test: `tests/unit/test_genome_validator.py`

**Step 1: Write the failing tests**

Add to `tests/unit/test_genome_validator.py`:

```python
def test_team_validation_empty_teams_with_team_mode():
    """Team mode requires non-empty teams."""
    genome = create_minimal_genome(player_count=4, team_mode=True, teams=())
    errors = GenomeValidator.validate(genome)
    assert any("teams required" in e.lower() or "teams must be" in e.lower() for e in errors)


def test_team_validation_invalid_player_index():
    """Teams with invalid player indices are rejected."""
    genome = create_minimal_genome(player_count=4, team_mode=True, teams=((0, 5), (1, 2)))
    errors = GenomeValidator.validate(genome)
    assert any("player" in e.lower() and "index" in e.lower() for e in errors)


def test_team_validation_duplicate_player():
    """Teams with duplicate players are rejected."""
    genome = create_minimal_genome(player_count=4, team_mode=True, teams=((0, 1), (1, 2)))
    errors = GenomeValidator.validate(genome)
    assert any("duplicate" in e.lower() for e in errors)


def test_team_validation_missing_player():
    """All players must be assigned to a team."""
    genome = create_minimal_genome(player_count=4, team_mode=True, teams=((0, 1), (2,)))
    errors = GenomeValidator.validate(genome)
    assert any("not all players" in e.lower() or "missing" in e.lower() for e in errors)


def test_team_validation_single_team():
    """At least 2 teams required."""
    genome = create_minimal_genome(player_count=4, team_mode=True, teams=((0, 1, 2, 3),))
    errors = GenomeValidator.validate(genome)
    assert any("at least 2" in e.lower() or "minimum" in e.lower() for e in errors)


def test_team_validation_valid_config():
    """Valid 2v2 team config passes validation."""
    genome = create_minimal_genome(player_count=4, team_mode=True, teams=((0, 2), (1, 3)))
    errors = GenomeValidator.validate(genome)
    # Should have no team-related errors
    team_errors = [e for e in errors if "team" in e.lower()]
    assert len(team_errors) == 0


def test_team_validation_no_team_mode():
    """Non-team games don't trigger team validation."""
    genome = create_minimal_genome(player_count=4, team_mode=False, teams=())
    errors = GenomeValidator.validate(genome)
    team_errors = [e for e in errors if "team" in e.lower()]
    assert len(team_errors) == 0
```

Also add helper function at top of test file:

```python
def create_minimal_genome(player_count: int = 2, team_mode: bool = False, teams: tuple = ()) -> GameGenome:
    """Create a minimal valid genome for testing."""
    return GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=player_count,
        team_mode=team_mode,
        teams=teams,
    )
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_genome_validator.py::test_team_validation_empty_teams_with_team_mode -v`
Expected: FAIL (no team validation exists yet)

**Step 3: Write minimal implementation**

Add to `GenomeValidator.validate()` in `src/darwindeck/genome/validator.py` (at end of method, before `return errors`):

```python
        # Check 9: Team validation
        if genome.team_mode:
            if not genome.teams:
                errors.append("team_mode=True requires teams to be defined")
            else:
                all_players: set[int] = set()
                for team in genome.teams:
                    for p in team:
                        if p >= genome.player_count:
                            errors.append(
                                f"Team contains invalid player index {p} (player_count={genome.player_count})"
                            )
                        if p in all_players:
                            errors.append(f"Duplicate player {p} in teams")
                        all_players.add(p)
                if len(all_players) != genome.player_count:
                    missing = set(range(genome.player_count)) - all_players
                    errors.append(
                        f"Not all players assigned to teams (missing: {missing})"
                    )
                if len(genome.teams) < 2:
                    errors.append("At least 2 teams required when team_mode=True")
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_genome_validator.py -v -k team`
Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/validator.py tests/unit/test_genome_validator.py
git commit -m "feat(validation): add team configuration validation"
```

---

## Task 3: Add Team Fields to Bytecode Header

**Files:**
- Modify: `src/darwindeck/genome/bytecode.py`
- Test: `tests/unit/test_bytecode.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_bytecode.py`:

```python
def test_bytecode_header_team_fields():
    """Test that bytecode header includes team configuration."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="team-test",
        generation=0,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=4,
        team_mode=True,
        teams=((0, 2), (1, 3)),
    )

    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    # Header should contain team_mode flag and team data offset
    header = BytecodeHeader.from_bytes(bytecode[:BytecodeHeader.HEADER_SIZE])
    assert header.team_mode == 1  # True
    assert header.team_count == 2
    assert header.team_data_offset > 0


def test_bytecode_header_no_teams():
    """Test that bytecode header handles non-team games."""
    genome = create_war_genome()  # No teams

    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    header = BytecodeHeader.from_bytes(bytecode[:BytecodeHeader.HEADER_SIZE])
    assert header.team_mode == 0  # False
    assert header.team_count == 0
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_bytecode.py::test_bytecode_header_team_fields -v`
Expected: FAIL with "has no attribute 'team_mode'"

**Step 3: Write minimal implementation**

Update `BytecodeHeader` dataclass in `src/darwindeck/genome/bytecode.py`:

1. Add fields to dataclass (after `hand_evaluation_offset`):
```python
    team_mode: int = 0  # 1 byte (0=individual, 1=team mode)
    team_count: int = 0  # 1 byte (number of teams)
    team_data_offset: int = 0  # 4 bytes (offset to team assignments)
```

2. Update `HEADER_SIZE` to 53 (was 47, +6 for new fields)

3. Update `to_bytes()` to encode team fields:
```python
        # Bytes 47-52: team_mode, team_count, team_data_offset
        result += bytes([self.team_mode, self.team_count])
        result += struct.pack("!i", self.team_data_offset)
        return result
```

4. Update `from_bytes()` to decode team fields:
```python
        # Parse bytes 47-52: team fields
        team_mode = data[47] if len(data) > 47 else 0
        team_count = data[48] if len(data) > 48 else 0
        team_data_offset = struct.unpack("!i", data[49:53])[0] if len(data) > 52 else 0
        return cls(
            *unpacked,
            tableau_mode=tableau_mode,
            sequence_direction=sequence_direction,
            card_scoring_offset=card_scoring_offset,
            hand_evaluation_offset=hand_evaluation_offset,
            team_mode=team_mode,
            team_count=team_count,
            team_data_offset=team_data_offset,
        )
```

5. Add `compile_teams()` function:
```python
def compile_teams(teams: tuple) -> bytes:
    """Compile team assignments to bytecode.

    Format:
    - team_count (1 byte)
    - For each team:
      - player_count (1 byte)
      - player_indices (player_count bytes)
    """
    if not teams:
        return bytes([0])

    result = bytes([len(teams)])
    for team in teams:
        result += bytes([len(team)])
        result += bytes(team)
    return result
```

6. Update `compile_genome()` to include team data:
```python
        # Compile team data section
        team_data_offset = self.offset
        team_bytes = compile_teams(genome.teams if genome.team_mode else ())
        self.offset += len(team_bytes)

        # Update header with team fields
        header = BytecodeHeader(
            # ... existing fields ...
            team_mode=1 if genome.team_mode else 0,
            team_count=len(genome.teams) if genome.team_mode else 0,
            team_data_offset=team_data_offset if genome.team_mode else 0,
        )

        # Add team_bytes to return
        return (header.to_bytes() + setup_bytes + turn_bytes + win_bytes +
                effects_bytes + score_bytes + card_scoring_bytes + hand_eval_bytes +
                team_bytes)
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_bytecode.py -v -k team`
Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/genome/bytecode.py tests/unit/test_bytecode.py
git commit -m "feat(bytecode): add team configuration to header and compile teams"
```

---

## Task 4: Add Team Fields to Go GameState

**Files:**
- Modify: `src/gosim/engine/types.go`
- Test: `src/gosim/engine/types_test.go`

**Step 1: Write the failing test**

Add to `src/gosim/engine/types_test.go`:

```go
func TestGameStateTeamFields(t *testing.T) {
    state := GetState()
    defer PutState(state)

    // Default values
    if state.TeamScores != nil {
        t.Error("TeamScores should be nil by default")
    }
    if state.PlayerToTeam != nil {
        t.Error("PlayerToTeam should be nil by default")
    }
    if state.WinningTeam != -1 {
        t.Error("WinningTeam should be -1 by default")
    }
}

func TestBuildPlayerToTeamLookup(t *testing.T) {
    teams := [][]int{{0, 2}, {1, 3}}
    lookup := BuildPlayerToTeamLookup(teams, 4)

    expected := []int8{0, 1, 0, 1}
    for i, v := range expected {
        if lookup[i] != v {
            t.Errorf("PlayerToTeam[%d] = %d, want %d", i, lookup[i], v)
        }
    }
}

func TestInitializeTeams(t *testing.T) {
    state := GetState()
    defer PutState(state)

    teams := [][]int{{0, 2}, {1, 3}}
    state.InitializeTeams(teams)

    if len(state.TeamScores) != 2 {
        t.Errorf("TeamScores length = %d, want 2", len(state.TeamScores))
    }
    if len(state.PlayerToTeam) != 4 {
        t.Errorf("PlayerToTeam length = %d, want 4", len(state.PlayerToTeam))
    }
    // Check team assignments
    if state.PlayerToTeam[0] != 0 || state.PlayerToTeam[2] != 0 {
        t.Error("Players 0 and 2 should be on team 0")
    }
    if state.PlayerToTeam[1] != 1 || state.PlayerToTeam[3] != 1 {
        t.Error("Players 1 and 3 should be on team 1")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `cd src/gosim && go test ./engine -v -run TestGameStateTeamFields`
Expected: FAIL with "state.TeamScores undefined"

**Step 3: Write minimal implementation**

Add to `GameState` struct in `src/gosim/engine/types.go`:

```go
    // Team play state
    TeamScores    []int32  // One entry per team, nil if not team_mode
    PlayerToTeam  []int8   // Precomputed lookup: player_idx -> team_idx (-1 if no teams)
    WinningTeam   int8     // Winning team index (-1 for draw, -1 if not team mode)
```

Add helper function:

```go
// BuildPlayerToTeamLookup creates a lookup table mapping player indices to team indices.
func BuildPlayerToTeamLookup(teams [][]int, numPlayers int) []int8 {
    lookup := make([]int8, numPlayers)
    for i := range lookup {
        lookup[i] = -1 // Default: no team
    }
    for teamIdx, team := range teams {
        for _, playerIdx := range team {
            if playerIdx < numPlayers {
                lookup[playerIdx] = int8(teamIdx)
            }
        }
    }
    return lookup
}

// InitializeTeams sets up team state for a team game.
func (s *GameState) InitializeTeams(teams [][]int) {
    numTeams := len(teams)
    if numTeams == 0 {
        s.TeamScores = nil
        s.PlayerToTeam = nil
        return
    }

    s.TeamScores = make([]int32, numTeams)
    s.PlayerToTeam = BuildPlayerToTeamLookup(teams, int(s.NumPlayers))
}
```

Update `Reset()` to clear team state:

```go
    // Team play state
    s.TeamScores = nil
    s.PlayerToTeam = nil
    s.WinningTeam = -1
```

Update `Clone()` to copy team state:

```go
    // Clone team state
    if s.TeamScores != nil {
        clone.TeamScores = make([]int32, len(s.TeamScores))
        copy(clone.TeamScores, s.TeamScores)
    }
    if s.PlayerToTeam != nil {
        clone.PlayerToTeam = make([]int8, len(s.PlayerToTeam))
        copy(clone.PlayerToTeam, s.PlayerToTeam)
    }
    clone.WinningTeam = s.WinningTeam
```

**Step 4: Run tests to verify they pass**

Run: `cd src/gosim && go test ./engine -v -run Team`
Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/types.go src/gosim/engine/types_test.go
git commit -m "feat(gosim): add team fields to GameState with lookup helpers"
```

---

## Task 5: Add Team Parsing to Go Genome Header

**Files:**
- Modify: `src/gosim/engine/bytecode.go`
- Test: `src/gosim/engine/bytecode_test.go`

**Step 1: Write the failing test**

Add to `src/gosim/engine/bytecode_test.go`:

```go
func TestParseGenomeTeamFields(t *testing.T) {
    // Create bytecode with team configuration
    // This will be validated once Python compiles a team genome
    header := GenomeHeader{
        Version:           2,
        PlayerCount:       4,
        MaxTurns:          100,
        TeamMode:          1,
        TeamCount:         2,
        TeamDataOffset:    53, // After header
    }

    bytecode := header.ToBytes()
    // Add team data: 2 teams, team 0 = [0,2], team 1 = [1,3]
    teamData := []byte{2, 2, 0, 2, 2, 1, 3}  // count, (size, players...), (size, players...)
    bytecode = append(bytecode, teamData...)

    genome, err := ParseGenome(bytecode)
    if err != nil {
        t.Fatalf("ParseGenome failed: %v", err)
    }

    if genome.Header.TeamMode != 1 {
        t.Errorf("TeamMode = %d, want 1", genome.Header.TeamMode)
    }
    if genome.Header.TeamCount != 2 {
        t.Errorf("TeamCount = %d, want 2", genome.Header.TeamCount)
    }
    if len(genome.Teams) != 2 {
        t.Fatalf("Teams length = %d, want 2", len(genome.Teams))
    }
    if len(genome.Teams[0]) != 2 || genome.Teams[0][0] != 0 || genome.Teams[0][1] != 2 {
        t.Errorf("Team 0 = %v, want [0,2]", genome.Teams[0])
    }
    if len(genome.Teams[1]) != 2 || genome.Teams[1][0] != 1 || genome.Teams[1][1] != 3 {
        t.Errorf("Team 1 = %v, want [1,3]", genome.Teams[1])
    }
}
```

**Step 2: Run test to verify it fails**

Run: `cd src/gosim && go test ./engine -v -run TestParseGenomeTeamFields`
Expected: FAIL with "genome.Header.TeamMode undefined"

**Step 3: Write minimal implementation**

Update `GenomeHeader` struct in `src/gosim/engine/bytecode.go`:

```go
type GenomeHeader struct {
    // ... existing fields ...
    TeamMode          uint8   // 0=individual, 1=team mode
    TeamCount         uint8   // Number of teams
    TeamDataOffset    int32   // Offset to team assignments
}
```

Update `Genome` struct:

```go
type Genome struct {
    Header    GenomeHeader
    Bytecode  []byte
    Teams     [][]int  // Parsed team assignments
}
```

Update header size constant and parsing:

```go
const HEADER_SIZE = 53  // Was 47, +6 for team fields
```

Update `ParseGenome` to parse team fields and team data:

```go
    // Parse team fields (bytes 47-52)
    if len(bytecode) >= 53 {
        header.TeamMode = bytecode[47]
        header.TeamCount = bytecode[48]
        header.TeamDataOffset = int32(binary.BigEndian.Uint32(bytecode[49:53]))
    }

    genome := &Genome{
        Header:   header,
        Bytecode: bytecode,
    }

    // Parse team data if present
    if header.TeamMode == 1 && header.TeamDataOffset > 0 && int(header.TeamDataOffset) < len(bytecode) {
        genome.Teams = parseTeamData(bytecode, header.TeamDataOffset, int(header.TeamCount))
    }

    return genome, nil
```

Add helper function:

```go
func parseTeamData(bytecode []byte, offset int32, teamCount int) [][]int {
    if int(offset) >= len(bytecode) {
        return nil
    }

    teams := make([][]int, 0, teamCount)
    pos := int(offset)

    // Skip team_count byte (already have it from header)
    if pos < len(bytecode) {
        pos++
    }

    for i := 0; i < teamCount && pos < len(bytecode); i++ {
        playerCount := int(bytecode[pos])
        pos++

        team := make([]int, playerCount)
        for j := 0; j < playerCount && pos < len(bytecode); j++ {
            team[j] = int(bytecode[pos])
            pos++
        }
        teams = append(teams, team)
    }

    return teams
}
```

**Step 4: Run tests to verify they pass**

Run: `cd src/gosim && go test ./engine -v -run Team`
Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/bytecode.go src/gosim/engine/bytecode_test.go
git commit -m "feat(gosim): parse team configuration from bytecode header"
```

---

## Task 6: Update Go Win Condition Evaluation for Teams

**Files:**
- Modify: `src/gosim/engine/movegen.go`
- Test: `src/gosim/engine/movegen_test.go`

**Step 1: Write the failing test**

Add to `src/gosim/engine/movegen_test.go`:

```go
func TestCheckWinConditionsTeamHighScore(t *testing.T) {
    state := GetState()
    defer PutState(state)

    state.NumPlayers = 4
    // Team 0: players 0, 2 with scores 50, 30 = 80 total
    // Team 1: players 1, 3 with scores 40, 35 = 75 total
    state.Players[0].Score = 50
    state.Players[1].Score = 40
    state.Players[2].Score = 30
    state.Players[3].Score = 35

    // Set up team state
    state.TeamScores = []int32{80, 75}  // Pre-calculated team scores
    state.PlayerToTeam = []int8{0, 1, 0, 1}

    // Create genome with team mode and high_score win condition
    genome := &Genome{
        Header: GenomeHeader{
            TeamMode:   1,
            TeamCount:  2,
            PlayerCount: 4,
        },
        Teams: [][]int{{0, 2}, {1, 3}},
    }
    // Set win condition: high_score with threshold 70
    // (Need to add win condition bytecode)

    playerWinner, teamWinner := CheckWinConditionsWithTeam(state, genome)

    if playerWinner != -1 {
        t.Errorf("playerWinner = %d, want -1 (team mode)", playerWinner)
    }
    if teamWinner != 0 {
        t.Errorf("teamWinner = %d, want 0 (team 0 has higher score)", teamWinner)
    }
}

func TestCheckWinConditionsIndividualMode(t *testing.T) {
    state := GetState()
    defer PutState(state)

    state.NumPlayers = 2
    state.Players[0].Score = 50
    state.Players[1].Score = 40

    genome := &Genome{
        Header: GenomeHeader{
            TeamMode:    0,
            PlayerCount: 2,
        },
    }

    playerWinner, teamWinner := CheckWinConditionsWithTeam(state, genome)

    if teamWinner != -1 {
        t.Errorf("teamWinner = %d, want -1 (individual mode)", teamWinner)
    }
    // playerWinner depends on actual win condition evaluation
}
```

**Step 2: Run test to verify it fails**

Run: `cd src/gosim && go test ./engine -v -run TestCheckWinConditionsTeam`
Expected: FAIL with "CheckWinConditionsWithTeam undefined"

**Step 3: Write minimal implementation**

Add to `src/gosim/engine/movegen.go`:

```go
// CheckWinConditionsWithTeam returns both player winner and team winner.
// In team mode: playerWinner=-1, teamWinner=winning team index (or -1 for draw)
// In individual mode: teamWinner=-1, playerWinner=winning player index (or -1)
func CheckWinConditionsWithTeam(state *GameState, genome *Genome) (playerWinner int8, teamWinner int8) {
    if genome.Header.TeamMode == 1 {
        return -1, checkTeamWinConditions(state, genome)
    }
    return CheckWinConditions(state, genome), -1
}

// checkTeamWinConditions evaluates win conditions using team aggregates.
func checkTeamWinConditions(state *GameState, genome *Genome) int8 {
    if len(state.TeamScores) == 0 {
        return -1
    }

    // Parse win conditions from bytecode
    winOffset := genome.Header.WinConditionsOffset
    if winOffset <= 0 || int(winOffset) >= len(genome.Bytecode) {
        return -1
    }

    // For now, implement high_score team evaluation
    // TODO: Parse actual win condition type from bytecode

    // Find team with highest score
    maxScore := int32(-1)
    winningTeam := int8(-1)
    tied := false

    for teamIdx, score := range state.TeamScores {
        if score > maxScore {
            maxScore = score
            winningTeam = int8(teamIdx)
            tied = false
        } else if score == maxScore {
            tied = true
        }
    }

    if tied {
        return -1  // Draw
    }
    return winningTeam
}
```

**Step 4: Run tests to verify they pass**

Run: `cd src/gosim && go test ./engine -v -run TestCheckWinConditions`
Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/movegen.go src/gosim/engine/movegen_test.go
git commit -m "feat(gosim): add team-aware win condition evaluation"
```

---

## Task 7: Add Dual Scoring to Go Simulation

**Files:**
- Modify: `src/gosim/engine/movegen.go`
- Test: `src/gosim/engine/movegen_test.go`

**Step 1: Write the failing test**

Add to `src/gosim/engine/movegen_test.go`:

```go
func TestAddScoreDualTracking(t *testing.T) {
    state := GetState()
    defer PutState(state)

    state.NumPlayers = 4
    state.TeamScores = []int32{0, 0}
    state.PlayerToTeam = []int8{0, 1, 0, 1}

    // Player 0 (team 0) scores 10 points
    AddScore(state, 0, 10)

    // Individual score updated
    if state.Players[0].Score != 10 {
        t.Errorf("Player 0 score = %d, want 10", state.Players[0].Score)
    }

    // Team score also updated
    if state.TeamScores[0] != 10 {
        t.Errorf("Team 0 score = %d, want 10", state.TeamScores[0])
    }

    // Player 2 (also team 0) scores 5 points
    AddScore(state, 2, 5)

    if state.Players[2].Score != 5 {
        t.Errorf("Player 2 score = %d, want 5", state.Players[2].Score)
    }
    if state.TeamScores[0] != 15 {
        t.Errorf("Team 0 score = %d, want 15", state.TeamScores[0])
    }

    // Team 1 should still be 0
    if state.TeamScores[1] != 0 {
        t.Errorf("Team 1 score = %d, want 0", state.TeamScores[1])
    }
}

func TestAddScoreNoTeams(t *testing.T) {
    state := GetState()
    defer PutState(state)

    state.NumPlayers = 2
    // No team state

    AddScore(state, 0, 10)

    if state.Players[0].Score != 10 {
        t.Errorf("Player 0 score = %d, want 10", state.Players[0].Score)
    }
    // Should not panic without team state
}
```

**Step 2: Run test to verify it fails**

Run: `cd src/gosim && go test ./engine -v -run TestAddScore`
Expected: FAIL with "AddScore undefined"

**Step 3: Write minimal implementation**

Add to `src/gosim/engine/movegen.go`:

```go
// AddScore adds points to a player's score and their team's score (if team mode).
// This implements dual scoring: both individual AND team scores are tracked.
func AddScore(state *GameState, playerIdx uint8, points int32) {
    // Always update individual score
    state.Players[playerIdx].Score += points

    // Additionally update team score if in team mode
    if state.TeamScores != nil && state.PlayerToTeam != nil {
        teamIdx := state.PlayerToTeam[playerIdx]
        if teamIdx >= 0 && int(teamIdx) < len(state.TeamScores) {
            state.TeamScores[teamIdx] += points
        }
    }
}
```

Update existing score modifications in `ApplyMove` to use `AddScore()` instead of direct assignment.

**Step 4: Run tests to verify they pass**

Run: `cd src/gosim && go test ./engine -v -run TestAddScore`
Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/engine/movegen.go src/gosim/engine/movegen_test.go
git commit -m "feat(gosim): add dual scoring for individual and team scores"
```

---

## Task 8: Update FlatBuffers Schema for Teams

**Files:**
- Modify: `schema/simulation.fbs`
- Run: `flatc` to regenerate bindings

**Step 1: Update schema**

Edit `schema/simulation.fbs`:

```fbs
// Add after existing tables (around line 28):

// Team assignment for team games
table TeamAssignment {
    player_indices: [ubyte];  // Players on this team
}

// Update SimulationRequest table to add:
table SimulationRequest {
    // ... existing fields ...

    // Team configuration
    team_mode: bool = false;
    teams: [TeamAssignment];  // Team definitions
}

// Update AggregatedStats table to add:
table AggregatedStats {
    // ... existing fields ...

    // Team play results
    team_wins: [uint32];       // Win count per team (empty if not team mode)
    winning_team: int8 = -1;   // Winning team index (-1 if draw or not team mode)
}
```

**Step 2: Regenerate bindings**

```bash
cd schema
flatc --go -o ../src/gosim/bindings/ simulation.fbs
flatc --python -o ../src/darwindeck/bindings/ simulation.fbs
```

**Step 3: Verify bindings compile**

```bash
cd src/gosim && go build ./...
uv run python -c "from darwindeck.bindings.cardsim import SimulationRequest"
```

**Step 4: Commit**

```bash
git add schema/simulation.fbs src/gosim/bindings/ src/darwindeck/bindings/
git commit -m "feat(flatbuffers): add team configuration to schema"
```

---

## Task 9: Update Go Simulation Runner for Teams

**Files:**
- Modify: `src/gosim/simulation/runner.go`
- Test: `src/gosim/simulation/runner_test.go`

**Step 1: Write the failing test**

Add to `src/gosim/simulation/runner_test.go`:

```go
func TestRunSingleGameTeamMode(t *testing.T) {
    // Create a team game genome
    // This requires a properly compiled bytecode with team configuration
    // For now, test that team initialization doesn't break the runner

    genome := createTestGenomeWithTeams()
    if genome == nil {
        t.Skip("Test genome with teams not yet implemented")
    }

    result := RunSingleGame(genome, RandomAI, 0, 12345)

    // Game should complete without error
    if result.Error != "" && result.Error != "no legal moves" {
        t.Errorf("Unexpected error: %s", result.Error)
    }
}
```

**Step 2: Update runner to initialize teams**

In `RunSingleGame()` in `src/gosim/simulation/runner.go`, add after chip initialization (around line 191):

```go
    // Initialize teams if this genome uses team mode
    if genome.Header.TeamMode == 1 && len(genome.Teams) > 0 {
        state.InitializeTeams(genome.Teams)
    }
```

Update game result handling to include team winner (around line 200-212):

```go
        playerWinner, teamWinner := engine.CheckWinConditionsWithTeam(state, genome)
        if playerWinner >= 0 || teamWinner >= 0 {
            state.WinningTeam = teamWinner
            tensionMetrics.Finalize(int(playerWinner))
            // ... rest of return
            return GameResult{
                WinnerID:    playerWinner,
                WinningTeam: teamWinner,
                // ... other fields
            }
        }
```

**Step 3: Update GameResult struct**

Add to `GameResult` in `src/gosim/simulation/runner.go`:

```go
type GameResult struct {
    WinnerID       int8
    WinningTeam    int8   // Winning team index (-1 if individual mode or draw)
    TurnCount      uint32
    // ... rest of fields
}
```

**Step 4: Update AggregatedStats**

Add to `AggregatedStats`:

```go
    // Team play results
    TeamWins    []uint32  // Wins per team (empty if not team mode)
```

Update `aggregateResults()` to aggregate team wins.

**Step 5: Run tests**

Run: `cd src/gosim && go test ./simulation -v -run Team`

**Step 6: Commit**

```bash
git add src/gosim/simulation/runner.go src/gosim/simulation/runner_test.go
git commit -m "feat(gosim): initialize and track team state in simulation runner"
```

---

## Task 10: Add Team Mutation Operators

**Files:**
- Modify: `src/darwindeck/evolution/operators.py`
- Test: `tests/unit/test_operators.py`

**Step 1: Write the failing tests**

Add to `tests/unit/test_operators.py`:

```python
def test_enable_team_mode_mutation():
    """EnableTeamModeMutation converts individual game to team game."""
    genome = create_war_genome()  # Individual mode
    assert genome.team_mode is False

    mutation = EnableTeamModeMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should be valid 4-player team game with player_count adjustment
    assert mutated.team_mode is True
    assert len(mutated.teams) >= 2
    assert mutated.player_count == 4  # Team mode requires 4 players for 2v2


def test_disable_team_mode_mutation():
    """DisableTeamModeMutation converts team game to individual."""
    genome = create_team_genome()  # Team mode
    assert genome.team_mode is True

    mutation = DisableTeamModeMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    assert mutated.team_mode is False
    assert mutated.teams == ()


def test_mutate_team_assignment_mutation():
    """MutateTeamAssignmentMutation shuffles team membership."""
    genome = create_team_genome()
    original_teams = genome.teams

    mutation = MutateTeamAssignmentMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Teams should be valid
    assert mutated.team_mode is True
    assert len(mutated.teams) == 2
    # All players still assigned
    all_players = set()
    for team in mutated.teams:
        for p in team:
            all_players.add(p)
    assert len(all_players) == mutated.player_count


def test_player_count_mutation_with_teams_repairs():
    """Player count mutation repairs or disables teams."""
    genome = create_team_genome()  # 4 players, teams=((0,2),(1,3))

    mutation = TweakParameterMutation(probability=1.0)
    # Force a mutation that changes player count
    # This should repair or disable teams

    # Simulate changing to 2 players - can't have 2v2 with 2 players
    from dataclasses import replace
    mutated = replace(genome, player_count=2, team_mode=True, teams=((0,), (1,)))

    # After validation repair, should either:
    # 1. Disable team mode
    # 2. Or adjust teams to be valid
    # Implementation will need to handle this
```

Also add helper:

```python
def create_team_genome() -> GameGenome:
    """Create a minimal team game genome for testing."""
    return GameGenome(
        schema_version="1.0",
        genome_id="team-test",
        generation=0,
        setup=SetupRules(cards_per_player=13),
        turn_structure=TurnStructure(phases=[PlayPhase(target=Location.DISCARD)]),
        special_effects=[],
        win_conditions=[WinCondition(type="high_score", threshold=100)],
        scoring_rules=[],
        player_count=4,
        team_mode=True,
        teams=((0, 2), (1, 3)),
    )
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/unit/test_operators.py::test_enable_team_mode_mutation -v`
Expected: FAIL with "EnableTeamModeMutation is not defined"

**Step 3: Write minimal implementation**

Add to `src/darwindeck/evolution/operators.py`:

```python
class EnableTeamModeMutation(MutationOperator):
    """Convert individual game to team game."""

    def __init__(self, probability: float = 0.03):
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        if genome.team_mode:
            return genome  # Already team mode

        # Team mode requires at least 4 players for 2v2
        new_player_count = max(4, genome.player_count)

        # Generate valid team assignment for 4 players
        team_configs = [
            ((0, 2), (1, 3)),  # Across from each other
            ((0, 1), (2, 3)),  # Side by side
            ((0, 3), (1, 2)),  # Diagonal
        ]
        teams = random.choice(team_configs)

        # Adjust cards_per_player if needed
        max_cards = 52 // new_player_count
        new_cards = min(genome.setup.cards_per_player, max_cards)
        new_setup = replace(genome.setup, cards_per_player=new_cards)

        return replace(
            genome,
            setup=new_setup,
            player_count=new_player_count,
            team_mode=True,
            teams=teams,
            generation=genome.generation + 1,
        )


class DisableTeamModeMutation(MutationOperator):
    """Convert team game to individual game."""

    def __init__(self, probability: float = 0.03):
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        if not genome.team_mode:
            return genome  # Already individual mode

        return replace(
            genome,
            team_mode=False,
            teams=(),
            generation=genome.generation + 1,
        )


class MutateTeamAssignmentMutation(MutationOperator):
    """Shuffle team membership while maintaining valid teams."""

    def __init__(self, probability: float = 0.05):
        super().__init__(probability)

    def mutate(self, genome: GameGenome) -> GameGenome:
        if not genome.team_mode or not genome.teams:
            return genome

        # Shuffle players and reassign to teams
        players = list(range(genome.player_count))
        random.shuffle(players)

        # Maintain same team sizes
        team_sizes = [len(team) for team in genome.teams]
        new_teams = []
        idx = 0
        for size in team_sizes:
            new_teams.append(tuple(players[idx:idx + size]))
            idx += size

        return replace(
            genome,
            teams=tuple(new_teams),
            generation=genome.generation + 1,
        )
```

**Step 4: Update the default pipeline to include team mutations**

Add to `create_default_pipeline()`:

```python
        # Team mutations (low weight - structural changes)
        EnableTeamModeMutation(probability=min(0.03 * mult, 0.06)),
        DisableTeamModeMutation(probability=min(0.03 * mult, 0.06)),
        MutateTeamAssignmentMutation(probability=min(0.05 * mult, 0.10)),
```

**Step 5: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_operators.py -v -k team`
Expected: PASS

**Step 6: Commit**

```bash
git add src/darwindeck/evolution/operators.py tests/unit/test_operators.py
git commit -m "feat(evolution): add team mutation operators"
```

---

## Task 11: Add Partnership Spades Seed Game

**Files:**
- Modify: `src/darwindeck/genome/examples.py`
- Test: `tests/unit/test_examples.py`

**Step 1: Write the failing test**

Add to `tests/unit/test_examples.py`:

```python
def test_create_partnership_spades_genome():
    """Test Partnership Spades genome with team configuration."""
    genome = create_partnership_spades_genome()

    assert genome.player_count == 4
    assert genome.team_mode is True
    assert genome.teams == ((0, 2), (1, 3))  # Partners across from each other
    assert len(genome.win_conditions) > 0
    # Should have trick phase
    trick_phases = [p for p in genome.turn_structure.phases if isinstance(p, TrickPhase)]
    assert len(trick_phases) > 0
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_examples.py::test_create_partnership_spades_genome -v`
Expected: FAIL with "create_partnership_spades_genome is not defined"

**Step 3: Write minimal implementation**

Add to `src/darwindeck/genome/examples.py`:

```python
def create_partnership_spades_genome() -> GameGenome:
    """Create Partnership Spades genome - 2v2 trick-taking with teams.

    Partnership Spades:
    - 4 players in 2 teams (players 0&2 vs 1&3)
    - 13 cards each (full 52-card deck)
    - Spades are always trump
    - Must follow suit if able
    - First team to 500 points wins
    - Team scores are combined
    """
    return GameGenome(
        schema_version="1.0",
        genome_id="partnership-spades",
        generation=0,
        setup=SetupRules(
            cards_per_player=13,
            initial_deck="standard_52",
            initial_discard_count=0,
            trump_suit=Suit.SPADES,  # Spades always trump
        ),
        turn_structure=TurnStructure(
            phases=[
                TrickPhase(
                    lead_suit_required=True,
                    trump_suit=Suit.SPADES,
                    high_card_wins=True,
                ),
            ],
            is_trick_based=True,
            tricks_per_hand=13,
        ),
        special_effects=[],
        win_conditions=[
            WinCondition(
                type="first_to_score",
                threshold=500,
                comparison=WinComparison.FIRST,
                trigger_mode=TriggerMode.THRESHOLD_GATE,
            )
        ],
        scoring_rules=[],
        card_scoring=(
            # Each trick won = 10 points
            CardScoringRule(
                condition=CardCondition(),  # Any card
                points=10,
                trigger=ScoringTrigger.TRICK_WIN,
            ),
        ),
        max_turns=1000,
        player_count=4,
        team_mode=True,
        teams=((0, 2), (1, 3)),  # Partners sit across from each other
    )
```

**Step 4: Export the function**

Add to imports/exports in `examples.py`:

```python
__all__ = [
    # ... existing exports ...
    "create_partnership_spades_genome",
]
```

**Step 5: Run tests to verify they pass**

Run: `uv run pytest tests/unit/test_examples.py::test_create_partnership_spades_genome -v`
Expected: PASS

**Step 6: Commit**

```bash
git add src/darwindeck/genome/examples.py tests/unit/test_examples.py
git commit -m "feat(examples): add Partnership Spades seed game with team configuration"
```

---

## Task 12: Integration Test - Full Team Game Simulation

**Files:**
- Create: `tests/integration/test_team_play.py`

**Step 1: Write the integration test**

Create `tests/integration/test_team_play.py`:

```python
"""Integration tests for team play functionality."""

import pytest
from darwindeck.genome.examples import create_partnership_spades_genome
from darwindeck.genome.bytecode import BytecodeCompiler
from darwindeck.genome.validator import GenomeValidator


class TestTeamPlayIntegration:
    """Integration tests for team play."""

    def test_partnership_spades_validates(self):
        """Partnership Spades genome passes validation."""
        genome = create_partnership_spades_genome()
        errors = GenomeValidator.validate(genome)
        assert len(errors) == 0, f"Validation errors: {errors}"

    def test_partnership_spades_compiles(self):
        """Partnership Spades compiles to valid bytecode."""
        genome = create_partnership_spades_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        assert len(bytecode) > 0
        # Verify team data is in bytecode
        assert bytecode[47] == 1  # team_mode = True
        assert bytecode[48] == 2  # team_count = 2

    def test_team_config_roundtrip(self):
        """Team configuration survives bytecode roundtrip."""
        from darwindeck.genome.bytecode import BytecodeHeader

        genome = create_partnership_spades_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        header = BytecodeHeader.from_bytes(bytecode[:BytecodeHeader.HEADER_SIZE])

        assert header.team_mode == 1
        assert header.team_count == 2
        assert header.team_data_offset > 0

    def test_team_mutation_produces_valid_genome(self):
        """Team mutations produce valid team configurations."""
        from darwindeck.evolution.operators import (
            EnableTeamModeMutation,
            MutateTeamAssignmentMutation,
        )
        from darwindeck.genome.examples import create_war_genome

        # Start with individual game
        genome = create_war_genome()
        assert genome.team_mode is False

        # Enable team mode
        mutation = EnableTeamModeMutation(probability=1.0)
        team_genome = mutation.mutate(genome)

        # Validate result
        errors = GenomeValidator.validate(team_genome)
        team_errors = [e for e in errors if "team" in e.lower()]
        assert len(team_errors) == 0, f"Team validation errors: {team_errors}"

        # Shuffle teams
        shuffle_mutation = MutateTeamAssignmentMutation(probability=1.0)
        shuffled = shuffle_mutation.mutate(team_genome)

        # Still valid
        errors = GenomeValidator.validate(shuffled)
        team_errors = [e for e in errors if "team" in e.lower()]
        assert len(team_errors) == 0
```

**Step 2: Run tests**

Run: `uv run pytest tests/integration/test_team_play.py -v`
Expected: PASS (after all previous tasks complete)

**Step 3: Commit**

```bash
git add tests/integration/test_team_play.py
git commit -m "test: add integration tests for team play"
```

---

## Task 13: Rebuild CGo Library and Verify

**Files:**
- Run build commands
- Verify integration

**Step 1: Rebuild Go library**

```bash
make build-cgo
```

**Step 2: Run Go tests**

```bash
cd src/gosim && go test ./... -v
```

**Step 3: Run Python tests**

```bash
uv run pytest tests/ -v --ignore=tests/property/
```

**Step 4: Commit any final fixes**

```bash
git add -A
git commit -m "fix: address build issues from team play implementation"
```

---

## Task 14: Update ROADMAP.md

**Files:**
- Modify: `ROADMAP.md`

**Step 1: Update roadmap**

Mark Team Play as complete in `ROADMAP.md`:

```markdown
### Schema Extensions

**Team Play** ✅ Complete
- [x] Partnership tracking
- [x] Shared scoring between teammates
- [x] Team-based win conditions
```

Add to version history:

```markdown
| 2026-01-17 | 0.4.0 | Team play support - partnership games like Spades now evolvable |
```

**Step 2: Commit**

```bash
git add ROADMAP.md
git commit -m "docs: mark Team Play as complete in roadmap"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add team_mode and teams fields to GameGenome | schema.py, test_genome_schema.py |
| 2 | Add team validation to GenomeValidator | validator.py, test_genome_validator.py |
| 3 | Add team fields to bytecode header | bytecode.py, test_bytecode.py |
| 4 | Add team fields to Go GameState | types.go, types_test.go |
| 5 | Add team parsing to Go genome header | bytecode.go, bytecode_test.go |
| 6 | Update Go win condition evaluation for teams | movegen.go, movegen_test.go |
| 7 | Add dual scoring to Go simulation | movegen.go, movegen_test.go |
| 8 | Update FlatBuffers schema for teams | simulation.fbs |
| 9 | Update Go simulation runner for teams | runner.go, runner_test.go |
| 10 | Add team mutation operators | operators.py, test_operators.py |
| 11 | Add Partnership Spades seed game | examples.py, test_examples.py |
| 12 | Integration test - full team game simulation | test_team_play.py |
| 13 | Rebuild CGo library and verify | (build) |
| 14 | Update ROADMAP.md | ROADMAP.md |

**Total: 14 tasks**
