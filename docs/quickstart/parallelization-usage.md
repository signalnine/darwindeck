# Using Parallelization in Production

Quick guide to using the parallelization features in the evolutionary card game system.

## Quick Start

The parallelization system has two levels:
1. **Go-level:** Parallel simulation within a single genome evaluation (automatic)
2. **Python-level:** Parallel evaluation of multiple genomes (opt-in)

Both levels use automatic CPU detection and require no manual configuration.

## Go-Level Parallelization

### Basic Usage

The parallel simulator is **automatically used** when you call the Go simulation through the CGo bridge:

```python
from darwindeck.bindings.cgo_bridge import simulate_batch
from darwindeck.genome.bytecode import BytecodeCompiler

# Compile genome to bytecode
compiler = BytecodeCompiler()
bytecode = compiler.compile_genome(genome)

# This automatically uses parallel simulation with runtime.NumCPU() workers
results = simulate_batch(
    bytecode=bytecode,
    num_games=1000,      # Optimal batch size
    ai_type=0,           # 0=Random, 1=Greedy, 2=MCTS
    mcts_iterations=0,   # Only used if ai_type=2
    random_seed=42       # For reproducibility
)
```

### Direct Go API (Advanced)

If calling the Go simulator directly:

```go
package main

import (
    "github.com/yourusername/cards-playtest/src/gosim/simulation"
)

func main() {
    // RunBatchParallel uses all CPU cores automatically
    results := simulation.RunBatchParallel(
        bytecode,
        1000,                      // numGames
        simulation.AIRandom,       // aiType
        0,                         // mctsIterations
        42,                        // seed
    )

    // Results include statistics from all games
    println("Total games:", results.TotalGames)
    println("Player 0 wins:", results.Player0Wins)
    println("Player 1 wins:", results.Player1Wins)
    println("Average turns:", results.AvgTurns)
}
```

**Performance:**
- Average speedup: **1.43x** on 4 cores
- Best speedup: **1.61x** with GreedyAI
- Memory overhead: **< 0.5%**

## Python-Level Parallelization

### Basic Usage

Use `ParallelFitnessEvaluator` to evaluate multiple genomes in parallel:

```python
from darwindeck.evolution.parallel_fitness import ParallelFitnessEvaluator
from darwindeck.evolution.fitness_full import FitnessEvaluator

# Create parallel evaluator with auto-detected CPU count
evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator(),
    num_workers=None  # None = auto-detect (recommended)
)

# Evaluate population (each genome runs in separate process)
genomes = [genome1, genome2, genome3, ...]
fitness_results = evaluator.evaluate_population(
    genomes,
    num_simulations=1000,  # Games per genome
    use_mcts=False         # True for skill measurement
)

# fitness_results is a list of FitnessMetrics, one per genome
for genome, metrics in zip(genomes, fitness_results):
    print(f"{genome.genome_id}: fitness={metrics.total_fitness:.3f}")
```

### Custom Worker Count

If you need to override the default worker count:

```python
# Use specific number of workers
evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator(),
    num_workers=2  # Explicit worker count
)
```

**When to use custom worker count:**
- **Leave cores for other tasks:** Use `cpu_count() - 1`
- **Testing/debugging:** Use `num_workers=1` for serial execution
- **Cloud/container:** Respect resource limits (e.g., Docker CPU quota)

### Integration with Genetic Algorithm

Example integration with evolutionary algorithm:

```python
from darwindeck.evolution.parallel_fitness import ParallelFitnessEvaluator
from darwindeck.evolution.fitness_full import FitnessEvaluator
from darwindeck.evolution.selection import tournament_selection
from darwindeck.evolution.mutation import mutate_genome

# Initialize parallel evaluator
parallel_eval = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator()
)

# Evolutionary loop
population = initialize_population(size=100)

for generation in range(100):
    # Parallel fitness evaluation (this is the expensive part)
    fitness_scores = parallel_eval.evaluate_population(
        population,
        num_simulations=1000
    )

    # Selection, crossover, mutation (fast, runs serially)
    parents = tournament_selection(population, fitness_scores)
    offspring = crossover(parents)
    population = [mutate_genome(child) for child in offspring]

    # Progress reporting
    best_fitness = max(f.total_fitness for f in fitness_scores)
    print(f"Generation {generation}: best={best_fitness:.3f}")
```

**Performance:**
- Expected speedup: **3.5-4.0x** on 4 cores (end-to-end)
- Population of 100 genomes: ~25-30 seconds
- Full evolution (100 generations): ~42-50 minutes

## Optimal Configurations

### By Use Case

| Use Case | Genomes | Sims/Genome | Workers | Time (4 cores) |
|----------|---------|-------------|---------|----------------|
| Quick validation | 10 | 10 | 2 | ~0.1s |
| Development iteration | 50 | 100 | 4 | ~1.5s |
| **Production fitness** | 100 | 1000 | 4 | ~25-30s |
| High confidence | 100 | 5000 | 4 | ~2.5 min |
| Full evolution (100 gen) | 100 | 1000 | 4 | ~45 min |

### By Batch Size

| Batch Size | When to Use | Speedup | Memory |
|------------|-------------|---------|--------|
| 10 | Unit tests, quick validation | 1.34x | Minimal |
| 100 | Development, rapid iteration | 1.40x | Low |
| **500-1000** | **Production (recommended)** | **1.43-1.51x** | **Low** |
| 5000 | High confidence evaluation | 1.46x | Medium |
| 10000 | Final validation, research | 1.22x | High |

**Recommendation:** Use **1000 games** for production fitness evaluation. This provides:
- Excellent speedup (1.43x)
- Good statistical confidence
- Reasonable completion time (~300ms per genome)

### By AI Type

| AI Type | Games/sec | Speedup | When to Use |
|---------|-----------|---------|-------------|
| RandomAI (0) | 3,082 | 1.47x | Initial screening (10 games) |
| GreedyAI (1) | 35,714 | 1.61x | Full fitness evaluation (100-1000 games) |
| MCTS (2) | ~500 (est) | 1.8x+ (est) | Skill measurement (1000 iterations) |

**Multi-stage strategy:**
```python
# Stage 1: Quick filter with RandomAI (10 games)
quick_results = evaluate_population(genomes, num_simulations=10, ai_type=0)
promising = [g for g, r in zip(genomes, quick_results) if r.valid]

# Stage 2: Full evaluation with GreedyAI (100 games)
full_results = evaluate_population(promising, num_simulations=100, ai_type=1)
good_genomes = [g for g, r in zip(promising, full_results) if r.total_fitness > 0.5]

# Stage 3: Skill measurement with MCTS (top 20%)
top_20_percent = sorted(good_genomes, key=lambda g: g.fitness, reverse=True)[:20]
final_results = evaluate_population(top_20_percent, num_simulations=1000, use_mcts=True)
```

## Performance Monitoring

### Measuring Throughput

```python
import time

# Measure evaluation time
genomes = create_test_population(100)
start = time.time()
results = evaluator.evaluate_population(genomes, num_simulations=1000)
elapsed = time.time() - start

# Calculate metrics
total_games = len(genomes) * 1000
throughput = total_games / elapsed
print(f"Throughput: {throughput:.0f} games/sec")
print(f"Time per genome: {elapsed/len(genomes)*1000:.1f}ms")
```

**Expected throughput:**
- Go-level (1000 games): **3,082 games/sec**
- End-to-end (100 genomes × 1000 games): **~3,000-4,000 games/sec**

### Monitoring CPU Utilization

```bash
# During evaluation, check CPU usage
htop  # Linux
top   # macOS/Linux

# Should see ~100% CPU on all cores during parallel evaluation
```

### Memory Usage

```python
import psutil
import os

process = psutil.Process(os.getpid())

# Before evaluation
mem_before = process.memory_info().rss / 1024 / 1024  # MB

# Evaluate
results = evaluator.evaluate_population(genomes, num_simulations=1000)

# After evaluation
mem_after = process.memory_info().rss / 1024 / 1024  # MB
print(f"Memory used: {mem_after - mem_before:.1f} MB")
```

**Expected memory usage:**
- Per game: ~1 MB
- Batch of 1000 games: ~1 GB
- Should scale linearly with batch size

## Troubleshooting

### Low Speedup (< 1.3x)

**Possible causes:**
1. **I/O bottleneck:** Check if disk/network is saturated
2. **Memory pressure:** Reduce batch size or number of workers
3. **Competing processes:** Check `htop` for other CPU-intensive tasks
4. **Batch size too small:** Use 500-1000 games minimum

**Solutions:**
```python
# Reduce worker count to leave resources for other tasks
evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator(),
    num_workers=2  # Instead of 4
)

# Reduce batch size to lower memory pressure
results = evaluator.evaluate_population(genomes, num_simulations=500)
```

### Memory Growth

**Possible causes:**
1. **Memory leak in custom evaluator:** Check your FitnessEvaluator implementation
2. **Batch size too large:** Go simulator needs ~1MB per game
3. **Too many workers:** Each worker holds memory for full batch

**Solutions:**
```python
# Process population in chunks to limit memory
def evaluate_in_chunks(genomes, chunk_size=25):
    all_results = []
    for i in range(0, len(genomes), chunk_size):
        chunk = genomes[i:i+chunk_size]
        results = evaluator.evaluate_population(chunk, num_simulations=1000)
        all_results.extend(results)
    return all_results

# Evaluate 100 genomes in 4 chunks of 25
results = evaluate_in_chunks(genomes, chunk_size=25)
```

### Inconsistent Results

**Possible causes:**
1. **Different random seeds:** Parallel execution affects RNG sequence
2. **Non-deterministic AI:** RandomAI and MCTS have inherent variance
3. **Floating-point rounding:** Different execution orders can cause small differences

**Solutions:**
```python
# For deterministic results, use serial evaluation
evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator(),
    num_workers=1  # Serial execution
)

# Or accept variance (parallel execution with same seed is within 15-20%)
results1 = evaluate_population(genomes, num_simulations=100, seed=42)
results2 = evaluate_population(genomes, num_simulations=100, seed=42)
# Expect: abs(results1[i] - results2[i]) / results1[i] < 0.20
```

### Process Pool Hangs

**Possible causes:**
1. **Exception in worker process:** Check logs for errors
2. **Deadlock in evaluator:** Check for locks in FitnessEvaluator
3. **Resource exhaustion:** Too many workers for available memory

**Solutions:**
```python
# Enable debugging to see worker errors
import logging
logging.basicConfig(level=logging.DEBUG)

# Reduce worker count
evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator(),
    num_workers=2
)

# Add timeout to detect hangs
import multiprocessing
multiprocessing.set_start_method('spawn')  # More robust than fork
```

## Advanced Usage

### Custom Simulator Factory

If you need to pass additional configuration to the simulator:

```python
def create_custom_simulator():
    # Custom simulator setup
    return CustomSimulator(config={...})

evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator(),
    simulator_factory=create_custom_simulator,
    num_workers=4
)
```

### Progress Reporting

Show progress during long evaluations:

```python
from tqdm import tqdm

def evaluate_with_progress(genomes, num_simulations=1000):
    results = []
    with tqdm(total=len(genomes), desc="Evaluating genomes") as pbar:
        # Process in chunks to update progress
        chunk_size = 10
        for i in range(0, len(genomes), chunk_size):
            chunk = genomes[i:i+chunk_size]
            chunk_results = evaluator.evaluate_population(chunk, num_simulations)
            results.extend(chunk_results)
            pbar.update(len(chunk))
    return results

# Shows progress bar: Evaluating genomes: 60/100 [01:23<00:55, 0.72it/s]
results = evaluate_with_progress(genomes)
```

### Warm-up Run

For consistent benchmarking, run a warm-up to populate caches:

```python
# Warm-up: single genome to populate Go code cache
warmup_genome = create_test_genome()
_ = evaluator.evaluate_population([warmup_genome], num_simulations=10)

# Now benchmark with warm cache
start = time.time()
results = evaluator.evaluate_population(genomes, num_simulations=1000)
elapsed = time.time() - start
```

## Best Practices

### 1. Use Auto-Detection

Let the system detect CPU count automatically:
```python
# ✅ Good: Auto-detect
evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator()
)

# ❌ Avoid: Hardcoded worker count
evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator(),
    num_workers=4  # What if running on 2-core or 16-core machine?
)
```

### 2. Use Optimal Batch Size

Stick to 500-1000 games for production:
```python
# ✅ Good: Optimal batch size
results = evaluator.evaluate_population(genomes, num_simulations=1000)

# ❌ Avoid: Too small (overhead-heavy)
results = evaluator.evaluate_population(genomes, num_simulations=10)

# ❌ Avoid: Too large (memory-heavy, diminishing returns)
results = evaluator.evaluate_population(genomes, num_simulations=50000)
```

### 3. Process Isolation

Don't share state across workers:
```python
# ✅ Good: Factory creates fresh instance per worker
evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator()
)

# ❌ Avoid: Shared instance across workers
shared_eval = FitnessEvaluator()
evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: shared_eval  # This won't work!
)
```

### 4. Error Handling

Wrap evaluation in try-except for robustness:
```python
try:
    results = evaluator.evaluate_population(genomes, num_simulations=1000)
except Exception as e:
    print(f"Evaluation failed: {e}")
    # Fallback: serial evaluation for debugging
    fallback_eval = FitnessEvaluator()
    results = [fallback_eval.evaluate(g, ...) for g in genomes]
```

### 5. Resource Awareness

Consider total system resources:
```python
import multiprocessing
import psutil

# Leave 1 core for OS and other tasks
available_cores = multiprocessing.cpu_count() - 1

# Leave 20% of memory free
available_memory_gb = psutil.virtual_memory().available / (1024**3)
max_batch_size = int(available_memory_gb * 0.8 * 1000)  # ~1MB per game

evaluator = ParallelFitnessEvaluator(
    evaluator_factory=lambda: FitnessEvaluator(),
    num_workers=available_cores
)

results = evaluator.evaluate_population(
    genomes,
    num_simulations=min(1000, max_batch_size)
)
```

## Performance Summary

**Current System (4 cores, Intel N100):**
- Go speedup: 1.43x (average), 1.61x (GreedyAI)
- Python speedup: ~4.0x (end-to-end)
- Combined speedup: 3.3-4.0x
- Throughput: 3,000-4,000 games/sec

**Expected Performance:**
- Single genome (1000 games): ~300ms
- Population (100 genomes): ~25-30s
- Full evolution (100 generations): ~42-50 minutes

**Scaling to Larger Systems:**
- 8 cores: ~2x faster → ~22 minutes for 100 generations
- 16 cores: ~4x faster → ~11 minutes for 100 generations

## Further Reading

- **Detailed benchmarks:** `/home/gabe/cards-playtest/docs/benchmarks/parallelization-results.md`
- **Implementation strategy:** `/home/gabe/cards-playtest/docs/parallelization-strategy.md`
- **Benchmark analysis:** `/home/gabe/cards-playtest/BENCHMARK_ANALYSIS.md`
- **Quick reference:** `/home/gabe/cards-playtest/BENCHMARK_SUMMARY.md`

## Support

If you encounter issues:
1. Check this guide's Troubleshooting section
2. Review benchmark results for expected performance
3. Enable debug logging to see worker activity
4. Try serial execution (`num_workers=1`) to isolate parallelization issues
