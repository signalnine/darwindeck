# MCTS Skill Evaluation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add post-evolution MCTS skill evaluation to measure true skill gap in evolved games.

**Architecture:** Extend the Go simulation runner to support asymmetric AI (different AI types for P1 and P2). Create Python `skill_evaluation.py` module that runs symmetric head-to-head games (MCTS vs Random in both directions). Integrate into CLI with progress reporting.

**Tech Stack:** Go (simulation), Python (orchestration), Flatbuffers (IPC), multiprocessing (parallelization)

---

## Task 1: Extend Flatbuffers Schema for Asymmetric AI

**Files:**
- Modify: `schema/simulation.fbs`

**Step 1: Add per-player AI type fields to schema**

Edit `schema/simulation.fbs` to add new fields to `SimulationRequest`:

```flatbuffers
// Single simulation request
table SimulationRequest {
  genome_bytecode: [ubyte] (required);  // Compiled genome from Python
  num_games: uint32;                     // How many games to simulate
  ai_player_type: ubyte;                 // 0=Random, 1=Greedy, 2=MCTS_Weak, etc (both players)
  mcts_iterations: uint32;               // Only used if ai_player_type >= 2
  random_seed: uint64;                   // For reproducible results
  // NEW: Per-player AI types for asymmetric evaluation
  player0_ai_type: ubyte;                // 0=use ai_player_type, 1+=override for P0
  player1_ai_type: ubyte;                // 0=use ai_player_type, 1+=override for P1
}
```

**Step 2: Regenerate Flatbuffers bindings**

Run:
```bash
cd /home/gabe/cards-playtest
flatc --go -o src/gosim/bindings schema/simulation.fbs
flatc --python -o src/darwindeck/bindings schema/simulation.fbs
```

Expected: New files generated in bindings directories

**Step 3: Commit**

```bash
git add schema/simulation.fbs src/gosim/bindings src/darwindeck/bindings
git commit -m "schema: add per-player AI type fields for asymmetric evaluation"
```

---

## Task 2: Update Go Runner for Asymmetric AI

**Files:**
- Modify: `src/gosim/simulation/runner.go`
- Test: `src/gosim/simulation/runner_test.go`

**Step 1: Write failing test for asymmetric AI**

Add to `src/gosim/simulation/runner_test.go`:

```go
func TestRunBatchAsymmetric(t *testing.T) {
	// Simple War genome for testing
	genome := createWarGenome()

	// P0=MCTS, P1=Random - MCTS should win more
	stats := RunBatchAsymmetric(genome, 20, MCTS500AI, RandomAI, 500, 12345)

	if stats.TotalGames != 20 {
		t.Errorf("Expected 20 games, got %d", stats.TotalGames)
	}

	// MCTS should have some wins (not necessarily dominant in War)
	if stats.Player0Wins == 0 && stats.Player1Wins == 0 {
		t.Error("Expected at least one player to win some games")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd src/gosim && go test ./simulation -run TestRunBatchAsymmetric -v`
Expected: FAIL with "undefined: RunBatchAsymmetric"

**Step 3: Implement RunBatchAsymmetric and RunSingleGameAsymmetric**

Add to `src/gosim/simulation/runner.go`:

```go
// RunBatchAsymmetric simulates games with different AI types for each player
func RunBatchAsymmetric(genome *engine.Genome, numGames int, p0AIType AIPlayerType, p1AIType AIPlayerType, mctsIterations int, seed uint64) AggregatedStats {
	results := make([]GameResult, numGames)
	rng := rand.New(rand.NewSource(int64(seed)))

	for i := 0; i < numGames; i++ {
		gameSeed := rng.Uint64()
		results[i] = RunSingleGameAsymmetric(genome, p0AIType, p1AIType, mctsIterations, gameSeed)
	}

	return aggregateResults(results)
}

// RunSingleGameAsymmetric plays one game with different AI for each player
func RunSingleGameAsymmetric(genome *engine.Genome, p0AIType AIPlayerType, p1AIType AIPlayerType, mctsIterations int, seed uint64) GameResult {
	start := time.Now()
	var metrics GameMetrics

	state := engine.GetState()
	defer engine.PutState(state)

	setupDeck(state, seed)

	cardsPerPlayer := 26
	if genome.Header.SetupOffset > 0 && genome.Header.SetupOffset+8 <= int32(len(genome.Bytecode)) {
		setupOffset := genome.Header.SetupOffset
		cardsPerPlayer = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset : setupOffset+4])))
	}

	numPlayers := int(genome.Header.PlayerCount)
	if numPlayers == 0 || numPlayers > 4 {
		numPlayers = 2
	}

	state.NumPlayers = uint8(numPlayers)
	state.CardsPerPlayer = cardsPerPlayer

	for i := 0; i < cardsPerPlayer; i++ {
		for p := 0; p < numPlayers; p++ {
			state.DrawCard(uint8(p), engine.LocationDeck)
		}
	}

	maxTurns := genome.Header.MaxTurns
	for state.TurnNumber < maxTurns {
		winner := engine.CheckWinConditions(state, genome)
		if winner >= 0 {
			return GameResult{
				WinnerID:   winner,
				TurnCount:  state.TurnNumber,
				DurationNs: uint64(time.Since(start).Nanoseconds()),
				Metrics:    metrics,
			}
		}

		moves := engine.GenerateLegalMoves(state, genome)
		if len(moves) == 0 {
			return GameResult{
				WinnerID:   -1,
				TurnCount:  state.TurnNumber,
				DurationNs: uint64(time.Since(start).Nanoseconds()),
				Error:      "no legal moves",
				Metrics:    metrics,
			}
		}

		metrics.TotalDecisions++
		metrics.TotalValidMoves += uint64(len(moves))
		if len(moves) == 1 {
			metrics.ForcedDecisions++
		}

		// Select AI based on current player
		var aiType AIPlayerType
		if state.CurrentPlayer == 0 {
			aiType = p0AIType
		} else {
			aiType = p1AIType
		}

		var move *engine.LegalMove
		switch aiType {
		case RandomAI:
			move = &moves[rand.Intn(len(moves))]
		case GreedyAI:
			move = selectGreedyMove(state, genome, moves)
		case MCTS100AI:
			move = mcts.Search(state, genome, 100, mcts.DefaultExplorationParam)
		case MCTS500AI:
			move = mcts.Search(state, genome, 500, mcts.DefaultExplorationParam)
		case MCTS1000AI:
			move = mcts.Search(state, genome, 1000, mcts.DefaultExplorationParam)
		case MCTS2000AI:
			move = mcts.Search(state, genome, 2000, mcts.DefaultExplorationParam)
		default:
			move = &moves[0]
		}

		if move == nil {
			return GameResult{
				WinnerID:   -1,
				TurnCount:  state.TurnNumber,
				DurationNs: uint64(time.Since(start).Nanoseconds()),
				Error:      "AI returned nil move",
				Metrics:    metrics,
			}
		}

		metrics.TotalActions++
		if isInteraction(state, move, genome) {
			metrics.TotalInteractions++
		}

		engine.ApplyMove(state, move, genome)
	}

	return GameResult{
		WinnerID:   -1,
		TurnCount:  state.TurnNumber,
		DurationNs: uint64(time.Since(start).Nanoseconds()),
		Metrics:    metrics,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd src/gosim && go test ./simulation -run TestRunBatchAsymmetric -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src/gosim/simulation/runner.go src/gosim/simulation/runner_test.go
git commit -m "feat(go): add asymmetric AI support for skill evaluation"
```

---

## Task 3: Update CGo Bridge for Asymmetric Simulation

**Files:**
- Modify: `src/gosim/cgo/bridge.go`
- Modify: `src/darwindeck/bindings/cgo_bridge.py`

**Step 1: Update Go CGo bridge to handle per-player AI types**

In `src/gosim/cgo/bridge.go`, update the simulation handling to check for per-player AI types:

```go
// In the batch processing loop, after parsing request:
p0AI := AIPlayerType(req.AiPlayerType())
p1AI := AIPlayerType(req.AiPlayerType())

// Override if per-player types specified (non-zero means override)
if req.Player0AiType() > 0 {
	p0AI = AIPlayerType(req.Player0AiType() - 1) // -1 because 0 means "use default"
}
if req.Player1AiType() > 0 {
	p1AI = AIPlayerType(req.Player1AiType() - 1)
}

var stats AggregatedStats
if p0AI == p1AI {
	stats = RunBatch(genome, numGames, p0AI, mctsIter, seed)
} else {
	stats = RunBatchAsymmetric(genome, numGames, p0AI, p1AI, mctsIter, seed)
}
```

**Step 2: Update Python CGo bridge**

In `src/darwindeck/bindings/cgo_bridge.py`, update `simulate_batch` to accept per-player AI types.

**Step 3: Rebuild CGo library**

Run: `make build-cgo`
Expected: `libcardsim.so` rebuilt successfully

**Step 4: Commit**

```bash
git add src/gosim/cgo/bridge.go src/darwindeck/bindings/cgo_bridge.py
git commit -m "feat(cgo): pass per-player AI types through bridge"
```

---

## Task 4: Extend GoSimulator Python Class

**Files:**
- Modify: `src/darwindeck/simulation/go_simulator.py`
- Test: `tests/unit/test_go_simulator.py`

**Step 1: Write failing test**

Create or add to `tests/unit/test_go_simulator.py`:

```python
def test_simulate_asymmetric():
    """Test asymmetric AI simulation (MCTS vs Random)."""
    from darwindeck.simulation.go_simulator import GoSimulator
    from darwindeck.genome.examples import WAR_GENOME

    sim = GoSimulator(seed=42)

    # MCTS as P0, Random as P1
    result = sim.simulate_asymmetric(
        genome=WAR_GENOME,
        num_games=10,
        p0_ai_type="mcts",
        p1_ai_type="random",
        mcts_iterations=100
    )

    assert result.total_games == 10
    assert result.player0_wins + result.player1_wins + result.draws == 10
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_go_simulator.py::test_simulate_asymmetric -v`
Expected: FAIL with "no attribute 'simulate_asymmetric'"

**Step 3: Implement simulate_asymmetric method**

Add to `src/darwindeck/simulation/go_simulator.py`:

```python
# AI type mapping
AI_TYPE_MAP = {
    "random": 1,    # +1 offset because 0 means "use default"
    "greedy": 2,
    "mcts": 3,      # MCTS100
    "mcts100": 3,
    "mcts500": 4,
    "mcts1000": 5,
    "mcts2000": 6,
}

def simulate_asymmetric(
    self,
    genome: GameGenome,
    num_games: int = 100,
    p0_ai_type: str = "random",
    p1_ai_type: str = "random",
    mcts_iterations: int = 500
) -> SimulationResults:
    """Simulate games with different AI types for each player.

    Args:
        genome: Game genome to simulate
        num_games: Number of games to run
        p0_ai_type: AI for player 0 ("random", "greedy", "mcts", "mcts500", etc)
        p1_ai_type: AI for player 1
        mcts_iterations: MCTS iterations (used if either player is MCTS)

    Returns:
        SimulationResults with game statistics
    """
    try:
        bytecode = self.compiler.compile_genome(genome)
    except Exception as e:
        return SimulationResults(
            total_games=num_games,
            player0_wins=0,
            player1_wins=0,
            draws=0,
            avg_turns=0.0,
            errors=num_games,
        )

    builder = flatbuffers.Builder(2048)
    genome_offset = builder.CreateByteVector(bytecode)

    # Map AI type strings to enum values
    p0_type = AI_TYPE_MAP.get(p0_ai_type.lower(), 1)
    p1_type = AI_TYPE_MAP.get(p1_ai_type.lower(), 1)

    SimulationRequestStart(builder)
    SimulationRequestAddGenomeBytecode(builder, genome_offset)
    SimulationRequestAddNumGames(builder, num_games)
    SimulationRequestAddAiPlayerType(builder, 0)  # Not used when per-player set
    SimulationRequestAddMctsIterations(builder, mcts_iterations)
    SimulationRequestAddRandomSeed(builder, self.seed + self._batch_id)
    SimulationRequestAddPlayer0AiType(builder, p0_type)
    SimulationRequestAddPlayer1AiType(builder, p1_type)
    req_offset = SimulationRequestEnd(builder)

    # ... rest of batch building same as simulate() ...
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_go_simulator.py::test_simulate_asymmetric -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/simulation/go_simulator.py tests/unit/test_go_simulator.py
git commit -m "feat(python): add simulate_asymmetric method to GoSimulator"
```

---

## Task 5: Create Skill Evaluation Module

**Files:**
- Create: `src/darwindeck/evolution/skill_evaluation.py`
- Test: `tests/unit/test_skill_evaluation.py`

**Step 1: Write failing test**

Create `tests/unit/test_skill_evaluation.py`:

```python
import pytest
from darwindeck.evolution.skill_evaluation import SkillEvalResult, evaluate_skill

def test_skill_eval_result_creation():
    """Test SkillEvalResult dataclass."""
    result = SkillEvalResult(
        genome_id="test_genome",
        mcts_wins_as_p1=45,
        mcts_wins_as_p2=38,
        total_mcts_wins=83,
        total_games=100,
        mcts_win_rate=0.83,
        timed_out=False
    )

    assert result.genome_id == "test_genome"
    assert result.mcts_win_rate == 0.83
    assert result.total_mcts_wins == 83
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_skill_evaluation.py::test_skill_eval_result_creation -v`
Expected: FAIL with "cannot import name 'SkillEvalResult'"

**Step 3: Create skill_evaluation.py with SkillEvalResult**

Create `src/darwindeck/evolution/skill_evaluation.py`:

```python
"""MCTS skill evaluation for evolved games."""

from dataclasses import dataclass
from typing import List, Optional, Callable
import logging
import time

from darwindeck.genome.schema import GameGenome
from darwindeck.simulation.go_simulator import GoSimulator

logger = logging.getLogger(__name__)


@dataclass
class SkillEvalResult:
    """Result of MCTS skill evaluation for a single genome."""
    genome_id: str
    mcts_wins_as_p1: int      # MCTS wins when playing as Player 1
    mcts_wins_as_p2: int      # MCTS wins when playing as Player 2
    total_mcts_wins: int      # Combined wins
    total_games: int          # Total games played
    mcts_win_rate: float      # total_mcts_wins / total_games (0.0-1.0)
    timed_out: bool = False   # True if evaluation was cut short
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_skill_evaluation.py::test_skill_eval_result_creation -v`
Expected: PASS

**Step 5: Write failing test for evaluate_skill function**

Add to `tests/unit/test_skill_evaluation.py`:

```python
def test_evaluate_skill_basic():
    """Test basic skill evaluation."""
    from darwindeck.genome.examples import WAR_GENOME

    result = evaluate_skill(
        genome=WAR_GENOME,
        num_games=20,  # Small for testing
        mcts_iterations=100,
        timeout_sec=30.0
    )

    assert result.genome_id == WAR_GENOME.genome_id
    assert result.total_games == 20
    assert 0.0 <= result.mcts_win_rate <= 1.0
    assert result.mcts_wins_as_p1 + result.mcts_wins_as_p2 == result.total_mcts_wins
```

**Step 6: Implement evaluate_skill function**

Add to `src/darwindeck/evolution/skill_evaluation.py`:

```python
def evaluate_skill(
    genome: GameGenome,
    num_games: int = 100,
    mcts_iterations: int = 500,
    timeout_sec: float = 60.0,
    progress_callback: Optional[Callable[[str], None]] = None
) -> SkillEvalResult:
    """Run symmetric MCTS vs Random evaluation.

    Runs num_games/2 with MCTS as P1, num_games/2 with MCTS as P2.
    This eliminates first-player advantage bias.

    Args:
        genome: Game genome to evaluate
        num_games: Total games to play (split evenly between directions)
        mcts_iterations: MCTS search iterations per move
        timeout_sec: Maximum time for entire evaluation
        progress_callback: Optional callback for progress updates

    Returns:
        SkillEvalResult with win rates and timing info
    """
    start_time = time.time()
    simulator = GoSimulator()

    games_per_direction = num_games // 2

    # Direction 1: MCTS as P1, Random as P2
    if progress_callback:
        progress_callback(f"Running MCTS as P1...")

    result_p1 = simulator.simulate_asymmetric(
        genome=genome,
        num_games=games_per_direction,
        p0_ai_type="mcts500" if mcts_iterations >= 500 else "mcts",
        p1_ai_type="random",
        mcts_iterations=mcts_iterations
    )

    # Check timeout
    elapsed = time.time() - start_time
    if elapsed > timeout_sec:
        return SkillEvalResult(
            genome_id=genome.genome_id,
            mcts_wins_as_p1=result_p1.player0_wins,
            mcts_wins_as_p2=0,
            total_mcts_wins=result_p1.player0_wins,
            total_games=games_per_direction,
            mcts_win_rate=result_p1.player0_wins / games_per_direction if games_per_direction > 0 else 0.5,
            timed_out=True
        )

    # Direction 2: Random as P1, MCTS as P2
    if progress_callback:
        progress_callback(f"Running MCTS as P2...")

    result_p2 = simulator.simulate_asymmetric(
        genome=genome,
        num_games=games_per_direction,
        p0_ai_type="random",
        p1_ai_type="mcts500" if mcts_iterations >= 500 else "mcts",
        mcts_iterations=mcts_iterations
    )

    # Combine results
    mcts_wins_p1 = result_p1.player0_wins  # MCTS was P0
    mcts_wins_p2 = result_p2.player1_wins  # MCTS was P1
    total_wins = mcts_wins_p1 + mcts_wins_p2
    total_games_played = games_per_direction * 2

    return SkillEvalResult(
        genome_id=genome.genome_id,
        mcts_wins_as_p1=mcts_wins_p1,
        mcts_wins_as_p2=mcts_wins_p2,
        total_mcts_wins=total_wins,
        total_games=total_games_played,
        mcts_win_rate=total_wins / total_games_played if total_games_played > 0 else 0.5,
        timed_out=False
    )
```

**Step 7: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_skill_evaluation.py::test_evaluate_skill_basic -v`
Expected: PASS

**Step 8: Commit**

```bash
git add src/darwindeck/evolution/skill_evaluation.py tests/unit/test_skill_evaluation.py
git commit -m "feat: add skill_evaluation module with evaluate_skill function"
```

---

## Task 6: Add Batch Skill Evaluation with Parallelization

**Files:**
- Modify: `src/darwindeck/evolution/skill_evaluation.py`
- Test: `tests/unit/test_skill_evaluation.py`

**Step 1: Write failing test**

Add to `tests/unit/test_skill_evaluation.py`:

```python
def test_evaluate_batch_skill():
    """Test batch skill evaluation."""
    from darwindeck.genome.examples import WAR_GENOME
    from darwindeck.evolution.skill_evaluation import evaluate_batch_skill

    # Create slight variations for testing
    genomes = [WAR_GENOME, WAR_GENOME]  # Same genome twice for simplicity

    results = evaluate_batch_skill(
        genomes=genomes,
        num_games=10,
        mcts_iterations=100,
        num_workers=2
    )

    assert len(results) == 2
    for result in results:
        assert result.total_games == 10
        assert 0.0 <= result.mcts_win_rate <= 1.0
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_skill_evaluation.py::test_evaluate_batch_skill -v`
Expected: FAIL with "cannot import name 'evaluate_batch_skill'"

**Step 3: Implement evaluate_batch_skill**

Add to `src/darwindeck/evolution/skill_evaluation.py`:

```python
import multiprocessing as mp
import os

# Use 'spawn' context for CGo compatibility
_mp_context = mp.get_context('spawn')


@dataclass
class _SkillEvalTask:
    """Task for parallel skill evaluation."""
    genome: GameGenome
    num_games: int
    mcts_iterations: int
    timeout_sec: float


def _evaluate_skill_task(task: _SkillEvalTask) -> SkillEvalResult:
    """Worker function for parallel evaluation."""
    return evaluate_skill(
        genome=task.genome,
        num_games=task.num_games,
        mcts_iterations=task.mcts_iterations,
        timeout_sec=task.timeout_sec
    )


def evaluate_batch_skill(
    genomes: List[GameGenome],
    num_games: int = 100,
    mcts_iterations: int = 500,
    timeout_sec: float = 60.0,
    num_workers: Optional[int] = None,
    progress_callback: Optional[Callable[[int, int], None]] = None
) -> List[SkillEvalResult]:
    """Evaluate skill gap for multiple genomes in parallel.

    Args:
        genomes: List of genomes to evaluate
        num_games: Games per genome (split between directions)
        mcts_iterations: MCTS search iterations
        timeout_sec: Timeout per genome
        num_workers: Worker processes (default: CPU count)
        progress_callback: Called with (completed, total) for progress

    Returns:
        List of SkillEvalResult, one per genome (same order)
    """
    if not genomes:
        return []

    num_workers = num_workers or int(os.environ.get('EVOLUTION_WORKERS', os.cpu_count() or 4))

    tasks = [
        _SkillEvalTask(genome, num_games, mcts_iterations, timeout_sec)
        for genome in genomes
    ]

    results: List[SkillEvalResult] = []

    with _mp_context.Pool(processes=num_workers) as pool:
        for i, result in enumerate(pool.imap(_evaluate_skill_task, tasks)):
            results.append(result)
            if progress_callback:
                progress_callback(i + 1, len(genomes))

    return results
```

**Step 4: Run test to verify it passes**

Run: `uv run pytest tests/unit/test_skill_evaluation.py::test_evaluate_batch_skill -v`
Expected: PASS

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/skill_evaluation.py tests/unit/test_skill_evaluation.py
git commit -m "feat: add parallel batch skill evaluation"
```

---

## Task 7: Integrate Skill Evaluation into Evolution Engine

**Files:**
- Modify: `src/darwindeck/evolution/engine.py`
- Test: `tests/unit/test_evolution_engine.py`

**Step 1: Add evaluate_skill_gaps method to EvolutionEngine**

Add to `src/darwindeck/evolution/engine.py`:

```python
from darwindeck.evolution.skill_evaluation import evaluate_batch_skill, SkillEvalResult

# In EvolutionEngine class:
def evaluate_skill_gaps(
    self,
    top_n: int = 20,
    num_games: int = 100,
    mcts_iterations: int = 500,
    timeout_sec: float = 60.0
) -> Dict[str, SkillEvalResult]:
    """Run MCTS skill evaluation on top genomes.

    Args:
        top_n: Number of top genomes to evaluate
        num_games: Games per genome for evaluation
        mcts_iterations: MCTS search iterations
        timeout_sec: Timeout per genome

    Returns:
        Dict mapping genome_id to SkillEvalResult
    """
    best_genomes = self.get_best_genomes(n=top_n)
    genomes = [ind.genome for ind in best_genomes]

    def progress(completed: int, total: int):
        logger.info(f"  Evaluating genome {completed}/{total}...")

    results = evaluate_batch_skill(
        genomes=genomes,
        num_games=num_games,
        mcts_iterations=mcts_iterations,
        timeout_sec=timeout_sec,
        num_workers=self.num_workers,
        progress_callback=progress
    )

    return {result.genome_id: result for result in results}
```

**Step 2: Run existing engine tests to ensure no regression**

Run: `uv run pytest tests/unit/test_evolution_engine.py -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add src/darwindeck/evolution/engine.py
git commit -m "feat: add evaluate_skill_gaps method to EvolutionEngine"
```

---

## Task 8: Add CLI Flags and Integration

**Files:**
- Modify: `src/darwindeck/cli/evolve.py`

**Step 1: Add CLI arguments**

Add to argument parser in `src/darwindeck/cli/evolve.py`:

```python
parser.add_argument(
    '--mcts-games', type=int, default=100,
    help='Games per genome for MCTS skill evaluation (default: 100)'
)
parser.add_argument(
    '--mcts-iterations', type=int, default=500,
    choices=[100, 500, 1000, 2000],
    help='MCTS search iterations per move (default: 500)'
)
parser.add_argument(
    '--skip-skill-eval', action='store_true',
    help='Skip post-evolution MCTS skill evaluation'
)
```

**Step 2: Add skill evaluation after evolution completes**

Add after `engine.evolve()` returns, before saving results:

```python
# Skill evaluation (unless skipped)
skill_results: Dict[str, SkillEvalResult] = {}
if not args.skip_skill_eval:
    logging.info(f"\nMCTS Skill Evaluation ({args.mcts_games} games, {args.mcts_iterations} iterations)")
    skill_results = engine.evaluate_skill_gaps(
        top_n=args.save_top_n,
        num_games=args.mcts_games,
        mcts_iterations=args.mcts_iterations
    )
    logging.info(f"  Completed skill evaluation for {len(skill_results)} genomes")
```

**Step 3: Add re-ranking for strategic style**

```python
# Re-rank if strategic style
if args.style == 'strategic' and skill_results:
    logging.info("\nRe-ranking by skill (--style strategic)...")
    best_genomes = sorted(
        best_genomes,
        key=lambda ind: skill_results.get(ind.genome.genome_id, SkillEvalResult(
            genome_id=ind.genome.genome_id,
            mcts_wins_as_p1=0, mcts_wins_as_p2=0,
            total_mcts_wins=0, total_games=0,
            mcts_win_rate=0.0
        )).mcts_win_rate,
        reverse=True
    )
```

**Step 4: Update JSON output to include skill data**

When saving genomes, include skill evaluation data:

```python
# In the save loop:
for i, individual in enumerate(best_genomes, 1):
    skill = skill_results.get(individual.genome.genome_id)

    # Create extended data dict
    genome_data = json.loads(genome_to_json(individual.genome))
    genome_data['fitness'] = individual.fitness
    genome_data['fitness_rank'] = i
    if skill:
        genome_data['mcts_win_rate'] = skill.mcts_win_rate
        genome_data['skill_rank'] = i  # After re-ranking
        genome_data['mcts_wins_as_p1'] = skill.mcts_wins_as_p1
        genome_data['mcts_wins_as_p2'] = skill.mcts_wins_as_p2
        genome_data['skill_eval_timed_out'] = skill.timed_out

    json_file = run_output_dir / f"rank{i:02d}_{individual.genome.genome_id}.json"
    with open(json_file, 'w') as f:
        json.dump(genome_data, f, indent=2)
```

**Step 5: Update console output**

```python
logging.info(f"\nSaving top {len(best_genomes)} genomes to {run_output_dir}")
for i, individual in enumerate(best_genomes, 1):
    skill = skill_results.get(individual.genome.genome_id)
    skill_str = f", mcts_win_rate={skill.mcts_win_rate:.2f}" if skill else ""
    logging.info(f"  {i}. {individual.genome.genome_id} (fitness={individual.fitness:.4f}{skill_str})")
```

**Step 6: Run full CLI test**

Run: `uv run python -m darwindeck.cli.evolve --population-size 10 --generations 2 --style strategic --mcts-games 20 --verbose`
Expected: Evolution runs, MCTS skill evaluation runs, results saved with skill data

**Step 7: Commit**

```bash
git add src/darwindeck/cli/evolve.py
git commit -m "feat(cli): add MCTS skill evaluation flags and integration"
```

---

## Task 9: Update LLM Descriptions with Skill Info

**Files:**
- Modify: `src/darwindeck/evolution/describe.py`

**Step 1: Update describe_game function signature**

```python
def describe_game(
    genome: GameGenome,
    fitness: float,
    mcts_win_rate: Optional[float] = None
) -> Optional[str]:
```

**Step 2: Add skill context to prompt**

```python
skill_context = ""
if mcts_win_rate is not None:
    if mcts_win_rate > 0.8:
        skill_context = f"\n\nSkill Analysis: Highly skill-based game. Skilled players win {mcts_win_rate*100:.0f}% against beginners."
    elif mcts_win_rate > 0.6:
        skill_context = f"\n\nSkill Analysis: Moderate skill element. Skilled players win {mcts_win_rate*100:.0f}% of the time."
    else:
        skill_context = f"\n\nSkill Analysis: Luck-heavy game. Skill advantage is only {mcts_win_rate*100:.0f}%."

prompt = f"""..existing prompt...{skill_context}"""
```

**Step 3: Update describe_top_games call in evolve.py**

```python
# When calling describe_top_games, pass skill info
for ind in best_genomes[:5]:
    skill = skill_results.get(ind.genome.genome_id)
    mcts_rate = skill.mcts_win_rate if skill else None
    description = describe_game(ind.genome, ind.fitness, mcts_rate)
```

**Step 4: Commit**

```bash
git add src/darwindeck/evolution/describe.py src/darwindeck/cli/evolve.py
git commit -m "feat: include MCTS skill info in LLM descriptions"
```

---

## Task 10: Final Integration Test

**Files:**
- Test: `tests/integration/test_skill_evaluation_e2e.py`

**Step 1: Create end-to-end test**

Create `tests/integration/test_skill_evaluation_e2e.py`:

```python
"""End-to-end test for MCTS skill evaluation."""
import subprocess
import json
from pathlib import Path
import tempfile

def test_evolution_with_skill_eval():
    """Test full evolution run with skill evaluation."""
    with tempfile.TemporaryDirectory() as tmpdir:
        output_dir = Path(tmpdir) / "output"

        result = subprocess.run([
            "uv", "run", "python", "-m", "darwindeck.cli.evolve",
            "--population-size", "10",
            "--generations", "2",
            "--style", "strategic",
            "--mcts-games", "10",
            "--mcts-iterations", "100",
            "--save-top-n", "5",
            "--no-describe",  # Skip LLM for faster test
            "--output-dir", str(output_dir)
        ], capture_output=True, text=True, timeout=300)

        assert result.returncode == 0, f"Evolution failed: {result.stderr}"

        # Check output files exist
        run_dirs = list(output_dir.iterdir())
        assert len(run_dirs) == 1, "Expected one run directory"

        json_files = list(run_dirs[0].glob("rank*.json"))
        assert len(json_files) == 5, f"Expected 5 output files, got {len(json_files)}"

        # Check skill data in output
        with open(json_files[0]) as f:
            data = json.load(f)

        assert 'mcts_win_rate' in data, "Missing mcts_win_rate in output"
        assert 'fitness_rank' in data, "Missing fitness_rank in output"
        assert 0.0 <= data['mcts_win_rate'] <= 1.0
```

**Step 2: Run integration test**

Run: `uv run pytest tests/integration/test_skill_evaluation_e2e.py -v`
Expected: PASS

**Step 3: Commit**

```bash
git add tests/integration/test_skill_evaluation_e2e.py
git commit -m "test: add end-to-end test for MCTS skill evaluation"
```

---

## Summary

| Task | Description | Estimated Commits |
|------|-------------|-------------------|
| 1 | Extend Flatbuffers schema | 1 |
| 2 | Update Go runner for asymmetric AI | 1 |
| 3 | Update CGo bridge | 1 |
| 4 | Extend GoSimulator Python class | 1 |
| 5 | Create skill_evaluation module | 1 |
| 6 | Add batch evaluation with parallelization | 1 |
| 7 | Integrate into EvolutionEngine | 1 |
| 8 | Add CLI flags and integration | 1 |
| 9 | Update LLM descriptions | 1 |
| 10 | Final integration test | 1 |

**Total: 10 tasks, 10 commits**
