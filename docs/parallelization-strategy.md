# Multi-Core Parallelization Strategy

**Date:** 2026-01-10
**System:** 4 CPU cores available
**Goal:** Maximize throughput by leveraging all CPU cores

## Overview

This evolutionary card game system has **multiple parallelization opportunities** across all phases. The workload is embarrassingly parallel at multiple levels:

1. **Phase 3 (Go Simulations):** Each simulation is independent
2. **Phase 4 (Fitness Evaluation):** Each genome evaluation is independent
3. **MCTS (Advanced AI):** Tree search can be parallelized

---

## Parallelization Opportunities

### Phase 3: Go Simulation Core (Goroutines)

**Current Design:** Batch API with 100-1000 simulations per CGo call

**Parallelization Strategy:**

```go
// Worker pool pattern in Go
func RunSimulationBatch(bytecode []byte, numSims int) SimulationResults {
    numWorkers := runtime.NumCPU()  // Use all available cores
    runtime.GOMAXPROCS(numWorkers)

    jobs := make(chan int, numSims)
    results := make(chan GameResult, numSims)

    // Start worker goroutines
    var wg sync.WaitGroup
    for w := 0; w < numWorkers; w++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for simID := range jobs {
                result := runSingleSimulation(bytecode, simID)
                results <- result
            }
        }()
    }

    // Queue all simulations
    for i := 0; i < numSims; i++ {
        jobs <- i
    }
    close(jobs)

    // Collect results
    wg.Wait()
    close(results)

    return aggregateResults(results)
}
```

**Benefits:**
- ✅ **4x speedup** on 4-core system (near-linear scaling)
- ✅ **No additional Python complexity** (parallelism hidden in Go)
- ✅ **Memory efficient** (sync.Pool reuses GameState across goroutines)
- ✅ **Lock-free** (each goroutine has own state)

**Expected Performance:**
- Serial: 0.03ms × 100 sims = **3ms per batch**
- Parallel (4 cores): 0.03ms × 100 / 4 = **0.75ms per batch** (4x faster)
- With Go speedup (10x): **0.075ms per batch** (40x faster than Python serial)

**Implementation Effort:** 15 minutes (worker pool is standard Go pattern)

---

### Phase 4: Fitness Evaluation (Python Multiprocessing)

**Current Design:** Sequential genome evaluation in Python

**Parallelization Strategy:**

```python
from multiprocessing import Pool
import os

class ParallelFitnessEvaluator:
    def __init__(self, go_sim_module):
        self.go_sim = go_sim_module
        self.num_workers = os.cpu_count()  # Use all cores

    def evaluate_population(self, genomes: List[GameGenome]) -> List[FitnessMetrics]:
        """Evaluate genomes in parallel using process pool."""
        with Pool(processes=self.num_workers) as pool:
            # Each worker process gets a copy of go_sim module
            # CGo is process-safe (separate memory spaces)
            results = pool.map(self._evaluate_single, genomes)
        return results

    def _evaluate_single(self, genome: GameGenome) -> FitnessMetrics:
        """Worker function: evaluate one genome (called in subprocess)."""
        # Stage 1: 10 simulations
        bytecode = compile_genome(genome)
        results = self.go_sim.run_batch(bytecode, num_sims=10)

        if results.avg_fitness < 0.3:
            return FitnessMetrics(valid=False, total_fitness=0.0)

        # Stage 2: 100 simulations
        results = self.go_sim.run_batch(bytecode, num_sims=100)
        return compute_metrics(genome, results)
```

**Benefits:**
- ✅ **4x speedup** on population evaluation (near-linear)
- ✅ **Process-safe** (CGo calls in separate processes)
- ✅ **Simple implementation** (standard multiprocessing.Pool)
- ✅ **Automatic load balancing** (pool distributes work)

**Expected Performance:**
- Population size: 100 genomes
- Serial: 100 genomes × 0.1s/genome = **10 seconds/generation**
- Parallel (4 cores): 100 / 4 × 0.1s = **2.5 seconds/generation** (4x faster)
- With Go speedup: **0.25 seconds/generation** (40x faster total)

**Bottleneck Analysis:**
- **100 generations** × 0.25s = **25 seconds total evolution time**
- This is fast enough for MVP (< 1 minute)
- Can increase population to 300-500 without performance issues

**Implementation Effort:** 30 minutes (standard Python multiprocessing)

---

### MCTS Parallelization (Phase 4 Advanced)

**Current Design:** Serial MCTS tree search in Go

**Parallelization Options:**

#### Option 1: Root Parallelization (Simplest)
Run multiple independent MCTS searches and average results.

```go
func ParallelMCTS(state *GameState, iterations int, numThreads int) Action {
    iterPerThread := iterations / numThreads
    results := make(chan Action, numThreads)

    var wg sync.WaitGroup
    for t := 0; t < numThreads; t++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            action := runMCTS(state.Clone(), iterPerThread)
            results <- action
        }()
    }

    wg.Wait()
    close(results)

    return selectBestAction(results)  // Vote or average
}
```

**Benefits:**
- ✅ No synchronization (independent searches)
- ✅ Simple implementation
- ✅ Linear speedup up to ~4 cores

**Drawback:**
- ⚠️ Less efficient than tree parallelization (duplicated work)

#### Option 2: Tree Parallelization (More Complex)
Share single tree across goroutines with fine-grained locks.

```go
type MCTSNode struct {
    mu       sync.RWMutex  // Protect this node
    visits   int32         // Use atomic operations
    value    float64
    children []*MCTSNode
}

func ParallelTreeMCTS(root *MCTSNode, iterations int, numThreads int) {
    var wg sync.WaitGroup
    for t := 0; t < numThreads; t++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for i := 0; i < iterations/numThreads; i++ {
                // Selection: readers lock
                root.mu.RLock()
                node := selectBestChild(root)
                root.mu.RUnlock()

                // Expansion + Simulation: no lock needed
                value := simulate(node)

                // Backpropagation: writers lock each node
                backpropagate(node, value)
            }
        }()
    }
    wg.Wait()
}
```

**Benefits:**
- ✅ More efficient (shared tree knowledge)
- ✅ Better scaling (up to 8-16 cores)

**Drawbacks:**
- ⚠️ Lock contention at root (limits scaling)
- ⚠️ More complex implementation

**Recommendation:** Start with root parallelization (Option 1), upgrade to tree parallelization if MCTS becomes bottleneck.

**Expected Performance:**
- Serial MCTS: 1000 iterations = 50ms
- Parallel (4 cores): 1000 / 4 = **12.5ms** (4x faster)

**Implementation Effort:**
- Root parallelization: 20 minutes
- Tree parallelization: 1-2 hours

---

## Implementation Priority

### High Priority (Phase 3 - Must Have)

**1. Go Worker Pool for Simulations** (15 min)
- Immediate 4x speedup
- No Python changes required
- Standard Go pattern
- **Impact:** 40x total speedup (10x Go + 4x parallelism)

### Medium Priority (Phase 4 - Should Have)

**2. Python Multiprocessing for Fitness Evaluation** (30 min)
- 4x speedup on population evaluation
- Critical for larger populations (300-500 genomes)
- Simple implementation with multiprocessing.Pool
- **Impact:** Evolution time: 10s → 2.5s per generation

### Low Priority (Phase 4 Advanced - Nice to Have)

**3. MCTS Root Parallelization** (20 min)
- Only needed if skill measurement becomes bottleneck
- Deferred until top 20% MCTS evaluation in Phase 4
- **Impact:** MCTS time: 50ms → 12.5ms

---

## Scalability Analysis

### Current System (4 cores):

| Component | Serial Time | Parallel Time | Speedup |
|-----------|-------------|---------------|---------|
| **Go simulations** (100 batch) | 3ms | 0.75ms | 4x |
| **With Go optimization** | 0.3ms | 0.075ms | 40x vs Python |
| **Fitness eval** (100 genomes) | 10s | 2.5s | 4x |
| **MCTS** (1000 iterations) | 50ms | 12.5ms | 4x |

### Full Generation (100 genomes × 100 sims each):

**Serial Pipeline:**
- 100 genomes × 100 sims × 0.3ms (Go serial) = **3 seconds**

**Parallel Pipeline (Go workers + Python multiprocessing):**
- 100 genomes / 4 cores × 25 sims / 4 workers × 0.3ms = **0.19 seconds**
- **~15x total speedup**

### Larger System (16 cores):

Could scale population to 400-500 genomes without performance regression:
- 400 genomes / 16 cores × 100 sims / 4 workers = **~0.75 seconds/generation**
- **100 generations in ~75 seconds** (< 2 minutes)

---

## Implementation Checklist

### Phase 3 (Golang Core):

- [ ] Add `runtime.GOMAXPROCS(runtime.NumCPU())` to init
- [ ] Implement worker pool pattern for batch simulations
- [ ] Benchmark serial vs parallel (expect ~4x speedup)
- [ ] Verify memory pooling works with goroutines (sync.Pool is goroutine-safe)

### Phase 4 (Genetic Algorithm):

- [ ] Wrap fitness evaluator with multiprocessing.Pool
- [ ] Test process-safety of CGo calls (should work, separate memory)
- [ ] Add worker count configuration (default: os.cpu_count())
- [ ] Monitor CPU utilization (should see ~100% on all cores)
- [ ] Add progress bar for parallel evaluation (tqdm with chunksize)

### Phase 4 Advanced (MCTS):

- [ ] Implement root parallelization for MCTS
- [ ] Benchmark parallel MCTS vs serial
- [ ] Consider tree parallelization if bottleneck persists

---

## Performance Targets (Updated with Parallelism)

### Phase 3 Targets:

| Metric | Serial Target | Parallel Target (4 cores) |
|--------|---------------|---------------------------|
| Simulations/sec | 33,000 | **133,000** |
| Batch time (100 sims) | 3ms | **0.75ms** |
| Speedup vs Python | 10x | **40x** |

### Phase 4 Targets:

| Metric | Serial Target | Parallel Target (4 cores) |
|--------|---------------|---------------------------|
| Population eval time | 10s | **2.5s** |
| Generation time | 10s | **2.5s** |
| 100 generations | 16 min | **4 minutes** |

**Conclusion:** With parallelization, **100 generations completes in < 5 minutes** instead of 16 minutes. This enables rapid iteration and larger population sizes (300-500 genomes feasible).

---

## Code Examples

### Phase 3: Go Worker Pool

```go
// File: src/gosim/engine/parallel.go
package engine

import (
    "runtime"
    "sync"
)

func RunBatchParallel(bytecode []byte, numSims int) *SimulationResults {
    numWorkers := runtime.NumCPU()
    runtime.GOMAXPROCS(numWorkers)

    jobs := make(chan int, numSims)
    results := make(chan *GameResult, numSims)

    var wg sync.WaitGroup
    for w := 0; w < numWorkers; w++ {
        wg.Add(1)
        go worker(&wg, jobs, results, bytecode)
    }

    // Queue jobs
    for i := 0; i < numSims; i++ {
        jobs <- i
    }
    close(jobs)

    // Wait for completion
    wg.Wait()
    close(results)

    // Aggregate
    return aggregateResults(results, numSims)
}

func worker(wg *sync.WaitGroup, jobs <-chan int, results chan<- *GameResult, bytecode []byte) {
    defer wg.Done()

    // Get pooled state (thread-local)
    state := statePool.Get().(*GameState)
    defer statePool.Put(state)

    for simID := range jobs {
        state.Reset()
        result := runSimulation(state, bytecode, simID)
        results <- result
    }
}
```

### Phase 4: Python Multiprocessing

```python
# File: src/cards_evolve/evolution/fitness.py
from multiprocessing import Pool
import os

class ParallelFitnessEvaluator:
    def __init__(self, go_sim_module, num_workers=None):
        self.go_sim = go_sim_module
        self.num_workers = num_workers or os.cpu_count()

    def evaluate_batch(self, genomes: List[GameGenome]) -> List[FitnessMetrics]:
        """Evaluate genomes in parallel."""
        with Pool(processes=self.num_workers) as pool:
            results = pool.map(self._eval_single, genomes)
        return results

    def _eval_single(self, genome: GameGenome) -> FitnessMetrics:
        """Evaluate single genome (runs in subprocess)."""
        bytecode = compile_genome(genome)

        # Stage 1: Fast filter (10 sims)
        results_10 = self.go_sim.run_batch(bytecode, 10)
        if not is_promising(results_10):
            return FitnessMetrics(valid=False, total_fitness=0.0)

        # Stage 2: Full evaluation (100 sims)
        results_100 = self.go_sim.run_batch(bytecode, 100)
        return compute_fitness(genome, results_100)
```

---

## References

- **Go Concurrency Patterns:** https://go.dev/blog/pipelines
- **Python Multiprocessing:** https://docs.python.org/3/library/multiprocessing.html
- **MCTS Parallelization:** Chaslot et al. (2008) "Parallel Monte-Carlo Tree Search"
- **sync.Pool Documentation:** https://pkg.go.dev/sync#Pool
