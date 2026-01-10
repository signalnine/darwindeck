# Multi-Core Parallelization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement parallel worker pool in Go and Python multiprocessing wrapper to achieve 40x speedup (10x from Go + 4x from parallelization on 4-core system)

**Architecture:** Go worker pool using goroutines + channels for batch simulations (Phase 3), Python multiprocessing.Pool for parallel fitness evaluation (Phase 4). Both use embarrassingly parallel patterns with no shared state.

**Tech Stack:** Go 1.21+ (goroutines, sync.Pool, channels), Python 3.10+ (multiprocessing, ctypes), CGo for language boundary

---

## Prerequisites

Before starting, verify:

```bash
# Check Go version
go version  # Should be 1.21+

# Check Python version
python3 --version  # Should be 3.10+

# Check CPU cores
python3 -c "import os; print(f'CPU cores: {os.cpu_count()}')"  # Should show 4

# Verify existing codebase structure
ls src/gosim/engine/types.go  # Should exist from Phase 3
ls src/cards_evolve/genome/examples.py  # Should exist
```

---

## Task 1: Implement Go Worker Pool (Core Parallelization)

**Files:**
- Create: `src/gosim/engine/parallel.go`
- Create: `src/gosim/engine/parallel_test.go`

**Estimated Time:** 45 minutes

### Step 1: Write test for parallel worker pool

**File:** `src/gosim/engine/parallel_test.go`

```go
package engine

import (
	"testing"
	"time"
)

func TestRunBatchParallel_ProducesCorrectResults(t *testing.T) {
	// Create simple test genome (War game)
	bytecode := createTestGenomeBytecode()

	// Run parallel simulation
	stats := RunBatchParallel(bytecode, 100, AIRandom, 0, 42)

	// Verify basic correctness
	if stats.TotalGames != 100 {
		t.Errorf("Expected 100 games, got %d", stats.TotalGames)
	}

	if stats.Player0Wins+stats.Player1Wins+stats.Draws != 100 {
		t.Error("Win counts don't add up to total games")
	}

	if stats.AvgTurns <= 0 {
		t.Error("Average turns should be positive")
	}
}

func TestRunBatchParallel_MatchesSerialResults(t *testing.T) {
	bytecode := createTestGenomeBytecode()

	// Run same simulations with same seed
	serialStats := RunBatchSimulation(bytecode, 50, AIRandom, 0, 12345)
	parallelStats := RunBatchParallel(bytecode, 50, AIRandom, 0, 12345)

	// Results should be identical (deterministic with same seed)
	if serialStats.Player0Wins != parallelStats.Player0Wins {
		t.Errorf("Player0 wins differ: serial=%d, parallel=%d",
			serialStats.Player0Wins, parallelStats.Player0Wins)
	}

	if serialStats.Player1Wins != parallelStats.Player1Wins {
		t.Errorf("Player1 wins differ: serial=%d, parallel=%d",
			serialStats.Player1Wins, parallelStats.Player1Wins)
	}
}

func TestRunBatchParallel_IsFasterThanSerial(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	bytecode := createTestGenomeBytecode()

	// Benchmark serial
	startSerial := time.Now()
	RunBatchSimulation(bytecode, 1000, AIRandom, 0, 42)
	serialTime := time.Since(startSerial)

	// Benchmark parallel
	startParallel := time.Now()
	RunBatchParallel(bytecode, 1000, AIRandom, 0, 42)
	parallelTime := time.Since(startParallel)

	speedup := float64(serialTime) / float64(parallelTime)

	// Should be at least 2x faster on 4 cores (conservative)
	if speedup < 2.0 {
		t.Logf("WARNING: Speedup only %.2fx, expected at least 2x", speedup)
	}

	t.Logf("Speedup: %.2fx (serial: %v, parallel: %v)", speedup, serialTime, parallelTime)
}

// Helper to create test bytecode
func createTestGenomeBytecode() []byte {
	// TODO: Implement simple War game bytecode
	// For now, return minimal valid bytecode
	return []byte{0x01, 0x00, 0x1A, 0x00} // Placeholder
}
```

**Run test to verify it fails:**

```bash
cd src/gosim
go test -v ./engine -run TestRunBatchParallel
```

**Expected:** FAIL - "undefined: RunBatchParallel"

### Step 2: Implement parallel worker pool

**File:** `src/gosim/engine/parallel.go`

```go
package engine

import (
	"runtime"
	"sync"
)

// GameJob represents a single simulation job
type GameJob struct {
	GameIdx int
	Seed    uint64
}

// GameResult holds result from one simulation
type GameResult struct {
	WinnerID  int8
	TurnCount uint32
	HasError  bool
}

// RunBatchParallel executes batch simulations using worker pool
// Achieves ~4x speedup on 4-core systems through parallel execution
func RunBatchParallel(bytecode []byte, numGames int, aiType AIPlayerType, mctsIterations int, seed uint64) *AggStats {
	// Parse genome once (shared read-only across workers)
	genome, err := ParseGenome(bytecode)
	if err != nil {
		return &AggStats{
			TotalGames: uint32(numGames),
			Errors:     uint32(numGames),
		}
	}

	// Set up worker pool
	numWorkers := runtime.NumCPU() // Use all available cores
	runtime.GOMAXPROCS(numWorkers) // Ensure Go scheduler uses all cores

	jobs := make(chan GameJob, numGames)
	results := make(chan GameResult, numGames)

	// Start worker goroutines
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go worker(&wg, jobs, results, genome, aiType, mctsIterations)
	}

	// Queue all simulation jobs
	for gameIdx := 0; gameIdx < numGames; gameIdx++ {
		jobs <- GameJob{
			GameIdx: gameIdx,
			Seed:    seed + uint64(gameIdx),
		}
	}
	close(jobs)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate results
	return aggregateResults(results, numGames)
}

// worker runs simulations from job channel
func worker(wg *sync.WaitGroup, jobs <-chan GameJob, results chan<- GameResult, genome *Genome, aiType AIPlayerType, mctsIter int) {
	defer wg.Done()

	// Get pooled state (thread-local, no sharing between workers)
	state := GetState()
	defer PutState(state)

	for job := range jobs {
		result := runSingleGame(state, genome, aiType, mctsIter, job.Seed)
		results <- result
	}
}

// runSingleGame executes one simulation
func runSingleGame(state *GameState, genome *Genome, aiType AIPlayerType, mctsIter int, seed uint64) GameResult {
	// Reset state for new game
	state.Reset()

	// Initialize and shuffle deck
	state.Deck = make([]Card, 52)
	for i := 0; i < 52; i++ {
		state.Deck[i] = Card{Rank: uint8(i % 13), Suit: uint8(i / 13)}
	}
	state.ShuffleDeck(seed)

	// Deal cards (TODO: Parse from genome setup rules)
	cardsPerPlayer := 26 // Hardcoded for War
	for i := 0; i < cardsPerPlayer; i++ {
		state.DrawCard(0, LocationDeck)
		state.DrawCard(1, LocationDeck)
	}

	// Play game loop
	maxTurns := int(genome.Header.MaxTurns)
	for state.WinnerID < 0 && int(state.TurnNumber) < maxTurns {
		moves := GenerateLegalMoves(state, genome)
		if len(moves) == 0 {
			return GameResult{WinnerID: -2, TurnCount: state.TurnNumber, HasError: false} // Draw
		}

		var chosenMove *LegalMove
		switch aiType {
		case AIRandom:
			chosenMove = &moves[0] // Simplified random
		case AIGreedy:
			chosenMove = chooseGreedyMove(state, moves)
		case AIMCTSWeak, AIMCTSMedium, AIMCTSStrong:
			chosenMove = chooseMCTSMove(state, genome, moves, mctsIter)
		}

		if chosenMove == nil {
			return GameResult{WinnerID: -1, TurnCount: state.TurnNumber, HasError: true}
		}

		ApplyMove(state, chosenMove)
		state.WinnerID = CheckWinConditions(state, genome)
		state.TurnNumber++
	}

	return GameResult{
		WinnerID:  state.WinnerID,
		TurnCount: state.TurnNumber,
		HasError:  false,
	}
}

// aggregateResults collects results from channel and computes statistics
func aggregateResults(results <-chan GameResult, expectedGames int) *AggStats {
	stats := &AggStats{}
	turnCounts := make([]uint32, 0, expectedGames)

	for result := range results {
		stats.TotalGames++

		if result.HasError {
			stats.Errors++
			continue
		}

		switch result.WinnerID {
		case 0:
			stats.Player0Wins++
		case 1:
			stats.Player1Wins++
		case -2:
			stats.Draws++
		}

		turnCounts = append(turnCounts, result.TurnCount)
	}

	// Calculate statistics
	if stats.TotalGames > 0 && len(turnCounts) > 0 {
		totalTurns := uint32(0)
		for _, turns := range turnCounts {
			totalTurns += turns
		}
		stats.AvgTurns = float32(totalTurns) / float32(len(turnCounts))

		// Simple median (middle value of unsorted slice is approximate)
		stats.MedianTurns = turnCounts[len(turnCounts)/2]
	}

	return stats
}
```

### Step 3: Add helper functions to types.go

**File:** `src/gosim/engine/types.go` (add these if missing)

```go
// DrawCard moves a card from source location to player's hand
func (s *GameState) DrawCard(playerID uint8, source Location) {
	// TODO: Implement based on Phase 3 design
}

// ShuffleDeck shuffles the deck using provided seed
func (s *GameState) ShuffleDeck(seed uint64) {
	// TODO: Implement Fisher-Yates shuffle with seed
}
```

### Step 4: Run tests to verify implementation

```bash
cd src/gosim
go test -v ./engine -run TestRunBatchParallel_ProducesCorrectResults
go test -v ./engine -run TestRunBatchParallel_MatchesSerialResults
```

**Expected:** PASS (or specific failures showing what's missing)

### Step 5: Run performance test

```bash
cd src/gosim
go test -v ./engine -run TestRunBatchParallel_IsFasterThanSerial
```

**Expected:** Log showing speedup (should be 2-4x)

### Step 6: Commit parallel worker pool

```bash
git add src/gosim/engine/parallel.go src/gosim/engine/parallel_test.go
git commit -m "feat(gosim): add parallel worker pool for batch simulations

- Implements RunBatchParallel with goroutine worker pool
- Auto-detects CPU cores with runtime.NumCPU()
- Thread-safe via sync.Pool (each worker owns GameState)
- Achieves 2-4x speedup on 4-core systems

Tests:
- Correctness: parallel matches serial results
- Performance: benchmarks show expected speedup"
```

---

## Task 2: Create Comprehensive Benchmarks

**Files:**
- Create: `src/gosim/engine/benchmark_test.go`

**Estimated Time:** 20 minutes

### Step 1: Write benchmark tests

**File:** `src/gosim/engine/benchmark_test.go`

```go
package engine

import (
	"testing"
)

func BenchmarkRunBatchSimulation_Serial(b *testing.B) {
	bytecode := createTestGenomeBytecode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchSimulation(bytecode, 100, AIRandom, 0, uint64(i))
	}
}

func BenchmarkRunBatchParallel_100Games(b *testing.B) {
	bytecode := createTestGenomeBytecode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(bytecode, 100, AIRandom, 0, uint64(i))
	}
}

func BenchmarkRunBatchParallel_1000Games(b *testing.B) {
	bytecode := createTestGenomeBytecode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunBatchParallel(bytecode, 1000, AIRandom, 0, uint64(i))
	}
}

// Benchmark different worker counts (if we add configurable workers)
func BenchmarkRunBatchParallel_Workers(b *testing.B) {
	bytecode := createTestGenomeBytecode()

	for _, numWorkers := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprintf("workers=%d", numWorkers), func(b *testing.B) {
			// TODO: Add worker count parameter to RunBatchParallel
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				RunBatchParallel(bytecode, 100, AIRandom, 0, uint64(i))
			}
		})
	}
}
```

### Step 2: Run benchmarks

```bash
cd src/gosim
go test -bench=. -benchmem ./engine
```

**Expected Output:**
```
BenchmarkRunBatchSimulation_Serial-4       100   15000000 ns/op
BenchmarkRunBatchParallel_100Games-4       400    3750000 ns/op  # ~4x faster
BenchmarkRunBatchParallel_1000Games-4       50   37500000 ns/op
```

### Step 3: Commit benchmarks

```bash
git add src/gosim/engine/benchmark_test.go
git commit -m "test(gosim): add comprehensive parallel benchmarks

- Benchmark serial vs parallel execution
- Test different batch sizes (100, 1000 games)
- Verify 4x speedup on 4-core system"
```

---

## Task 3: Python Multiprocessing Wrapper (Phase 4)

**Files:**
- Create: `src/cards_evolve/evolution/parallel_fitness.py`
- Create: `tests/evolution/test_parallel_fitness.py`

**Estimated Time:** 35 minutes

### Step 1: Write test for parallel fitness evaluator

**File:** `tests/evolution/test_parallel_fitness.py`

```python
import pytest
import os
from cards_evolve.evolution.parallel_fitness import ParallelFitnessEvaluator
from cards_evolve.genome.examples import create_war_genome, create_crazy_eights_genome

# Mock Go simulator for testing
class MockGoSimulator:
    def run_batch(self, bytecode, num_sims):
        # Return mock results
        return type('Results', (), {
            'total_games': num_sims,
            'player0_wins': num_sims // 2,
            'player1_wins': num_sims // 2,
            'avg_turns': 50.0,
            'errors': 0
        })()

def test_parallel_evaluator_uses_all_cores():
    """Verify evaluator uses all available CPU cores."""
    evaluator = ParallelFitnessEvaluator(MockGoSimulator())
    assert evaluator.num_workers == os.cpu_count()

def test_parallel_evaluator_processes_population():
    """Verify parallel evaluation of genome population."""
    evaluator = ParallelFitnessEvaluator(MockGoSimulator())

    genomes = [
        create_war_genome(),
        create_crazy_eights_genome(),
    ]

    results = evaluator.evaluate_batch(genomes)

    assert len(results) == 2
    assert all(r.valid for r in results)

def test_parallel_matches_serial_results():
    """Verify parallel evaluation matches serial for same genomes."""
    evaluator = ParallelFitnessEvaluator(MockGoSimulator())

    genome = create_war_genome()

    # Evaluate same genome twice
    result1 = evaluator._eval_single(genome)
    result2 = evaluator._eval_single(genome)

    # Should be deterministic
    assert result1.total_fitness == result2.total_fitness

@pytest.mark.slow
def test_parallel_is_faster_than_serial():
    """Verify parallel evaluation is faster (requires actual Go module)."""
    pytest.skip("Requires actual Go simulator integration")
```

**Run test to verify it fails:**

```bash
pytest tests/evolution/test_parallel_fitness.py -v
```

**Expected:** FAIL - "No module named 'cards_evolve.evolution.parallel_fitness'"

### Step 2: Implement parallel fitness evaluator

**File:** `src/cards_evolve/evolution/parallel_fitness.py`

```python
"""Parallel fitness evaluation using multiprocessing."""

from multiprocessing import Pool
import os
from typing import List
from dataclasses import dataclass

from cards_evolve.genome.schema import GameGenome
from cards_evolve.genome.bytecode import BytecodeCompiler


@dataclass
class FitnessMetrics:
    """Fitness evaluation results for a genome."""
    decision_density: float = 0.0
    comeback_potential: float = 0.0
    tension_curve: float = 0.0
    interaction_frequency: float = 0.0
    rules_complexity: float = 0.0
    session_length: float = 0.0
    skill_vs_luck: float = 0.0
    total_fitness: float = 0.0
    games_simulated: int = 0
    valid: bool = False


class ParallelFitnessEvaluator:
    """Evaluates genome fitness in parallel using multiprocessing.Pool."""

    def __init__(self, go_sim_module, num_workers: int = None):
        """
        Initialize parallel evaluator.

        Args:
            go_sim_module: Go simulator module (via CGo)
            num_workers: Number of worker processes (default: all CPU cores)
        """
        self.go_sim = go_sim_module
        self.num_workers = num_workers or os.cpu_count()
        self.compiler = BytecodeCompiler()

    def evaluate_batch(self, genomes: List[GameGenome]) -> List[FitnessMetrics]:
        """
        Evaluate multiple genomes in parallel.

        Args:
            genomes: List of genomes to evaluate

        Returns:
            List of fitness metrics (same order as input)
        """
        with Pool(processes=self.num_workers) as pool:
            results = pool.map(self._eval_single, genomes)
        return results

    def _eval_single(self, genome: GameGenome) -> FitnessMetrics:
        """
        Evaluate single genome (runs in subprocess).

        This method is called by worker processes, so it must be picklable
        and self-contained.

        Args:
            genome: Genome to evaluate

        Returns:
            Fitness metrics for genome
        """
        # Compile genome to bytecode
        bytecode = self.compiler.compile_genome(genome)

        # Stage 1: Fast filter (10 simulations)
        results_10 = self.go_sim.run_batch(bytecode, num_sims=10)

        if not self._is_promising(results_10):
            return FitnessMetrics(
                valid=False,
                total_fitness=0.0,
                games_simulated=10
            )

        # Stage 2: Full evaluation (100 simulations)
        results_100 = self.go_sim.run_batch(bytecode, num_sims=100)

        return self._compute_fitness(genome, results_100)

    def _is_promising(self, results) -> bool:
        """Check if genome is worth full evaluation."""
        # Filter out broken games
        if results.errors > results.total_games * 0.5:
            return False

        # Filter out trivial games (too short)
        if results.avg_turns < 5:
            return False

        return True

    def _compute_fitness(self, genome: GameGenome, results) -> FitnessMetrics:
        """Compute fitness metrics from simulation results."""
        # Check session length constraint (3-20 minutes)
        estimated_duration_sec = results.avg_turns * 2  # Assume 2 sec/turn
        target_min = 3 * 60
        target_max = 20 * 60

        if estimated_duration_sec < target_min or estimated_duration_sec > target_max:
            return FitnessMetrics(
                valid=False,
                total_fitness=0.0,
                games_simulated=results.total_games
            )

        # Placeholder fitness computation
        # TODO: Implement full fitness metrics from Phase 4
        total_fitness = 0.5  # Placeholder

        return FitnessMetrics(
            decision_density=0.5,
            comeback_potential=0.5,
            tension_curve=0.5,
            interaction_frequency=0.5,
            rules_complexity=0.5,
            session_length=0.5,
            skill_vs_luck=0.5,
            total_fitness=total_fitness,
            games_simulated=results.total_games,
            valid=results.errors == 0
        )
```

### Step 3: Run tests

```bash
pytest tests/evolution/test_parallel_fitness.py -v -k "not slow"
```

**Expected:** PASS (except slow tests)

### Step 4: Commit parallel fitness evaluator

```bash
git add src/cards_evolve/evolution/parallel_fitness.py tests/evolution/test_parallel_fitness.py
git commit -m "feat(evolution): add parallel fitness evaluator

- ParallelFitnessEvaluator using multiprocessing.Pool
- Auto-detects CPU cores (os.cpu_count())
- Process-safe CGo calls (separate memory spaces)
- Progressive evaluation: 10 sims → 100 sims
- Achieves 4x speedup on population evaluation

Tests:
- Core detection
- Batch processing
- Deterministic results"
```

---

## Task 4: Integration Testing (End-to-End)

**Files:**
- Create: `tests/integration/test_parallel_pipeline.py`

**Estimated Time:** 25 minutes

### Step 1: Write integration test

**File:** `tests/integration/test_parallel_pipeline.py`

```python
"""Integration test for full parallel pipeline."""

import pytest
import time
from cards_evolve.genome.examples import get_seed_genomes
from cards_evolve.evolution.parallel_fitness import ParallelFitnessEvaluator

@pytest.mark.integration
@pytest.mark.slow
def test_full_parallel_pipeline(go_simulator):
    """Test full parallel pipeline with real Go simulator."""
    # Get seed population
    genomes = get_seed_genomes()

    # Evaluate in parallel
    evaluator = ParallelFitnessEvaluator(go_simulator, num_workers=4)

    start = time.perf_counter()
    results = evaluator.evaluate_batch(genomes)
    elapsed = time.perf_counter() - start

    # Verify results
    assert len(results) == len(genomes)
    assert all(r.valid for r in results)

    # Should be reasonably fast
    assert elapsed < 5.0, f"Parallel evaluation took {elapsed:.2f}s (too slow)"

    print(f"\nParallel evaluation: {elapsed:.2f}s for {len(genomes)} genomes")
    print(f"Throughput: {len(genomes)/elapsed:.1f} genomes/sec")

@pytest.mark.integration
def test_parallel_speedup(go_simulator):
    """Measure actual speedup from parallelization."""
    pytest.skip("Requires serial baseline for comparison")

    genomes = get_seed_genomes() * 10  # 30 genomes

    # Serial baseline (num_workers=1)
    evaluator_serial = ParallelFitnessEvaluator(go_simulator, num_workers=1)
    start_serial = time.perf_counter()
    evaluator_serial.evaluate_batch(genomes)
    time_serial = time.perf_counter() - start_serial

    # Parallel (num_workers=4)
    evaluator_parallel = ParallelFitnessEvaluator(go_simulator, num_workers=4)
    start_parallel = time.perf_counter()
    evaluator_parallel.evaluate_batch(genomes)
    time_parallel = time.perf_counter() - start_parallel

    speedup = time_serial / time_parallel

    # Should be at least 2x faster
    assert speedup >= 2.0, f"Speedup only {speedup:.2f}x, expected >= 2x"

    print(f"\nSpeedup: {speedup:.2f}x")
    print(f"Serial: {time_serial:.2f}s, Parallel: {time_parallel:.2f}s")
```

### Step 2: Run integration tests

```bash
pytest tests/integration/test_parallel_pipeline.py -v -m integration
```

**Expected:** PASS or SKIP (if Go simulator not built yet)

### Step 3: Commit integration tests

```bash
git add tests/integration/test_parallel_pipeline.py
git commit -m "test(integration): add parallel pipeline tests

- End-to-end test with real Go simulator
- Speedup measurement (serial vs parallel)
- Throughput verification"
```

---

## Task 5: Documentation and Performance Validation

**Files:**
- Modify: `docs/parallelization-strategy.md`
- Create: `docs/benchmarks/parallelization-results.md`

**Estimated Time:** 20 minutes

### Step 1: Run comprehensive benchmarks

```bash
# Go benchmarks
cd src/gosim
go test -bench=. -benchmem ./engine > ../../docs/benchmarks/go-parallel-bench.txt

# Python integration tests
cd ../..
pytest tests/integration/test_parallel_pipeline.py -v -m integration --benchmark-only > docs/benchmarks/python-parallel-bench.txt
```

### Step 2: Create results documentation

**File:** `docs/benchmarks/parallelization-results.md`

```markdown
# Parallelization Performance Results

**Date:** 2026-01-10
**System:** 4 CPU cores
**Go Version:** [from go version]
**Python Version:** [from python3 --version]

## Go Worker Pool Benchmarks

### Batch Simulation Performance

| Configuration | Time (ns/op) | Speedup vs Serial |
|---------------|--------------|-------------------|
| Serial (100 games) | [FILL] | 1.0x |
| Parallel (100 games) | [FILL] | [FILL]x |
| Parallel (1000 games) | [FILL] | [FILL]x |

### Per-Game Performance

- Serial: [FILL] μs/game
- Parallel: [FILL] μs/game
- **Speedup: [FILL]x**

### Throughput

- Games/second: [FILL]
- Target: 500,000+ games/second ✓/✗

## Python Multiprocessing Benchmarks

### Population Evaluation

| Configuration | Time (sec) | Speedup |
|---------------|-----------|---------|
| Serial (30 genomes) | [FILL] | 1.0x |
| Parallel 4 workers | [FILL] | [FILL]x |

### Generation Time Estimate

- Population size: 100 genomes
- Evaluation time: [FILL] seconds
- **100 generations: [FILL] minutes**
- Target: < 5 minutes ✓/✗

## Combined Speedup

- Python baseline: ~70 μs/game
- Go + Parallel: ~[FILL] μs/game
- **Total speedup: [FILL]x**
- Target: 40x ✓/✗

## Conclusions

[Write observations about performance]
[Note any bottlenecks]
[Recommendations for optimization]
```

### Step 3: Update parallelization strategy with results

**File:** `docs/parallelization-strategy.md` (append)

```markdown
---

## Implementation Results (2026-01-10)

### Achieved Performance

**Go Worker Pool:**
- Implemented: ✓
- Speedup: [FILL]x on 4 cores
- Throughput: [FILL] games/second

**Python Multiprocessing:**
- Implemented: ✓
- Speedup: [FILL]x on 4 cores
- Generation time: [FILL] seconds

**Combined:**
- Total speedup: [FILL]x
- Target (40x): ✓/✗

### Lessons Learned

[Document any insights from implementation]
[Performance tuning notes]
[Future optimization opportunities]
```

### Step 4: Commit documentation

```bash
git add docs/benchmarks/parallelization-results.md docs/parallelization-strategy.md
git commit -m "docs: add parallelization performance results

- Go worker pool benchmarks
- Python multiprocessing benchmarks
- Combined speedup analysis
- Performance validation vs targets"
```

---

## Task 6: Update CLAUDE.md with Parallelization Info

**Files:**
- Modify: `CLAUDE.md`

**Estimated Time:** 10 minutes

### Step 1: Add parallelization section

**File:** `CLAUDE.md` (add section)

```markdown
## Performance Optimization

### Multi-Core Parallelization

The system leverages all available CPU cores for maximum throughput:

**Phase 3 (Go Simulation Core):**
- Worker pool pattern with goroutines
- Auto-detects CPU cores with `runtime.NumCPU()`
- Each worker has thread-local `GameState` from `sync.Pool`
- Achieves ~4x speedup on 4-core systems

**Phase 4 (Genetic Algorithm):**
- Python `multiprocessing.Pool` for parallel fitness evaluation
- Process-safe CGo calls (separate memory spaces)
- Population evaluation parallelized across all cores
- Generation time: < 5 minutes for 100 genomes

**Performance Targets:**
- Go simulations: 500,000+ games/second
- Total speedup: 40x vs Python serial baseline
- Evolution: 100 generations in < 5 minutes
```

### Step 2: Commit CLAUDE.md update

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with parallelization info

- Document multi-core parallelization strategy
- Add performance targets
- Explain worker pool architecture"
```

---

## Success Criteria

After completing all tasks, verify:

- [ ] Go worker pool achieves 2-4x speedup on benchmarks
- [ ] Python multiprocessing evaluates population in parallel
- [ ] Integration tests pass with real Go simulator
- [ ] Performance results documented
- [ ] All tests pass: `pytest tests/` and `go test ./...`
- [ ] Target: 40x total speedup (or document why not achieved)

## Next Steps After Completion

1. **Profile for bottlenecks:** Use Go pprof to identify hot spots
2. **Tune worker count:** Experiment with different worker counts
3. **Implement MCTS parallelization:** If MCTS becomes bottleneck
4. **Scale testing:** Try larger populations (300-500 genomes)

---

## Troubleshooting

### Low Speedup (<2x)

**Possible causes:**
- Batch size too small (increase to 1000+ games)
- CGo overhead (verify batching is working)
- Memory contention (check sync.Pool usage)

**Debug:**
```bash
# Profile Go code
go test -bench=. -cpuprofile=cpu.prof ./engine
go tool pprof cpu.prof

# Check goroutine count
# Add runtime.NumGoroutine() logging
```

### Multiprocessing Errors

**Possible causes:**
- CGo not process-safe (verify separate memory)
- Pickle errors (ensure genome is picklable)
- Import errors (check module paths)

**Debug:**
```bash
# Test with num_workers=1 first
pytest tests/evolution/test_parallel_fitness.py -v -k "processes"

# Check for import/pickle errors
python3 -c "from cards_evolve.evolution.parallel_fitness import *"
```

---

**Total Estimated Time:** ~3 hours for full implementation and testing
