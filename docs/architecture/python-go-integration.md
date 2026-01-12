# Python-Go Integration Architecture

This document explains how DarwinDeck's Python and Go components interact to achieve high-performance evolutionary simulation.

## Overview

DarwinDeck uses a **hybrid architecture** where:
- **Python** handles high-level concerns: genome representation, evolution algorithms, fitness computation
- **Go** handles performance-critical work: game simulation, MCTS search, parallel batch execution

The two layers communicate via **CGo** (C foreign function interface) with **Flatbuffers** for efficient binary serialization.

## Architecture Layers

```
┌─────────────────────────────────────────────────────────┐
│  Python Layer (Evolution & Fitness)                     │
│  - GameGenome: Dataclass representing game rules        │
│  - BytecodeCompiler: Genome → binary bytecode           │
│  - GoSimulator: Wraps CGo interface                     │
│  - FitnessEvaluator: Computes fitness metrics           │
│  - ParallelFitnessEvaluator: Multiprocessing pool       │
└──────────────────┬──────────────────────────────────────┘
                   │
                   │ Flatbuffers (Binary Serialization)
                   │ + CGo FFI (ctypes)
                   ↓
┌─────────────────────────────────────────────────────────┐
│  Go Simulation Layer (Performance Core)                 │
│  - SimulateBatch: CGo entry point                       │
│  - ParseGenome: Bytecode → executable state machine     │
│  - RunBatchParallel: Multi-worker simulation            │
│  - RunSingleGame: Individual game executor              │
│  - MCTS: Monte Carlo tree search                        │
└─────────────────────────────────────────────────────────┘
```

## Key Files

| Component | Python | Go |
|-----------|--------|-----|
| Bytecode | `src/darwindeck/genome/bytecode.py` | `src/gosim/engine/bytecode.go` |
| CGo Bridge | `src/darwindeck/bindings/cgo_bridge.py` | `src/gosim/cgo/bridge.go` |
| Simulator | `src/darwindeck/simulation/go_simulator.py` | `src/gosim/simulation/runner.go` |
| Parallel | `src/darwindeck/evolution/parallel_fitness.py` | `src/gosim/simulation/parallel.go` |
| Schema | - | `schema/simulation.fbs` |

## Data Flow

### 1. Bytecode Compilation (Python)

The `BytecodeCompiler` converts a `GameGenome` dataclass into a compact binary format:

```python
# src/darwindeck/genome/bytecode.py
compiler = BytecodeCompiler()
bytecode = compiler.compile_genome(genome)  # Returns bytes
```

**Bytecode structure (36-byte header + sections):**

```
[Header: 36 bytes]
├── version: uint32
├── genome_id_hash: uint64
├── player_count: uint32
├── max_turns: uint32
├── setup_offset: int32          (byte offset to setup section)
├── turn_structure_offset: int32 (offset to phases)
├── win_conditions_offset: int32 (offset to win logic)
└── scoring_offset: int32        (offset to scoring rules)

[Setup Section]
├── cards_per_player: int32
└── initial_discard_count: int32

[Turn Structure Section]
├── phase_count: uint32
└── For each phase:
    └── [phase_type: ubyte + phase_data]
        ├── DrawPhase, PlayPhase, DiscardPhase
        ├── TrickPhase, BettingPhase, ClaimPhase

[Win Conditions Section]
└── [win_type + threshold] per condition

[Special Effects Section (optional)]
└── [trigger_rank, effect_type, target, value] per effect
```

### 2. Flatbuffers Serialization (Python)

The `GoSimulator` wraps bytecode in a Flatbuffers request:

```python
# src/darwindeck/simulation/go_simulator.py
builder = flatbuffers.Builder(2048)
genome_offset = builder.CreateByteVector(bytecode)

SimulationRequestStart(builder)
SimulationRequestAddGenomeBytecode(builder, genome_offset)
SimulationRequestAddNumGames(builder, num_games)
SimulationRequestAddAiPlayerType(builder, ai_type)
SimulationRequestAddRandomSeed(builder, seed)
req_offset = SimulationRequestEnd(builder)

# Wrap in batch
BatchRequestStart(builder)
BatchRequestAddBatchId(builder, batch_id)
BatchRequestAddRequests(builder, requests_offset)
builder.Finish(BatchRequestEnd(builder))

request_bytes = bytes(builder.Output())
```

**Schema definition** (`schema/simulation.fbs`):

```flatbuffers
table SimulationRequest {
    genome_bytecode: [ubyte];     // Compiled game rules
    num_games: uint32;            // Games to simulate
    ai_player_type: ubyte;        // 0=Random, 1=Greedy, 2+=MCTS
    mcts_iterations: uint32;      // Tree search depth
    random_seed: uint64;          // Reproducibility
    ai_types: [ubyte];            // Per-player AI overrides
    player_count: ubyte;          // 2-4 players
}

table BatchRequest {
    batch_id: uint64;
    requests: [SimulationRequest];
}
```

### 3. CGo Bridge (Python → Go)

Python uses `ctypes` to call the shared library:

```python
# src/darwindeck/bindings/cgo_bridge.py
_lib = ctypes.CDLL("libcardsim.so")

_lib.SimulateBatch.argtypes = [
    ctypes.c_void_p,              # Request buffer
    ctypes.c_int,                 # Request length
    ctypes.POINTER(ctypes.c_int)  # Output length (return param)
]
_lib.SimulateBatch.restype = ctypes.c_void_p

def simulate_batch(request_bytes: bytes) -> BatchResponse:
    buf = (ctypes.c_char * len(request_bytes)).from_buffer_copy(request_bytes)
    response_len = ctypes.c_int()

    result_ptr = _lib.SimulateBatch(
        ctypes.cast(buf, ctypes.c_void_p),
        len(request_bytes),
        ctypes.byref(response_len)
    )

    result_bytes = bytes(ctypes.string_at(result_ptr, response_len.value))
    return BatchResponse.GetRootAsBatchResponse(result_bytes, 0)
```

### 4. Go Request Handling

The Go bridge receives the request and orchestrates simulation:

```go
// src/gosim/cgo/bridge.go
//export SimulateBatch
func SimulateBatch(requestPtr unsafe.Pointer, requestLen C.int,
                   responseLen *C.int) unsafe.Pointer {
    // Parse Flatbuffers request
    requestBytes := C.GoBytes(requestPtr, requestLen)
    batchRequest := cardsim.GetRootAsBatchRequest(requestBytes, 0)

    for i := 0; i < batchRequest.RequestsLength(); i++ {
        req := batchRequest.Requests(i)

        // Parse bytecode into executable genome
        genomeBytecode := req.GenomeBytecodeBytes()
        genome, _ := engine.ParseGenome(genomeBytecode)

        // Run parallel simulation
        stats := simulation.RunBatchParallel(
            genome, numGames, aiType, mctsIter, seed)

        resultOffsets[i] = serializeStats(builder, &stats)
    }

    // Return C-allocated response buffer
    responseBytes := builder.FinishedBytes()
    cBytes := C.malloc(C.size_t(len(responseBytes)))
    C.memcpy(cBytes, unsafe.Pointer(&responseBytes[0]), ...)
    return cBytes
}
```

### 5. Bytecode Interpretation (Go)

Go parses and executes the bytecode:

```go
// src/gosim/engine/bytecode.go
func ParseGenome(bytecode []byte) (*Genome, error) {
    header, _ := ParseHeader(bytecode)

    // Jump to turn structure offset, parse phases
    // Jump to win conditions, parse each
    // Jump to effects section, parse special effects

    return &Genome{Header: header, Phases: phases, WinConditions: wins}, nil
}

// Condition evaluation using OpCodes
func EvaluateCondition(state *GameState, cond []byte) bool {
    opcode := cond[0]
    switch opcode {
    case OpCheckHandSize:
        // Check hand size against threshold
    case OpCheckCardRank:
        // Check if card matches rank
    case OpAnd:
        // Recursively evaluate all sub-conditions
    }
}

// Move generation from current game state
func GenerateLegalMoves(state *GameState, genome *Genome) []Move {
    phase := genome.Phases[state.CurrentPhase]
    // Generate moves based on phase type and conditions
}
```

### 6. Parallel Simulation (Go)

Go uses worker goroutines for batch simulation:

```go
// src/gosim/simulation/parallel.go
func RunBatchParallel(genome *Genome, numGames int, ...) AggregatedStats {
    numWorkers := runtime.NumCPU()
    jobs := make(chan GameJob, numGames)
    results := make(chan GameResult, numGames)

    // Start worker pool
    for w := 0; w < numWorkers; w++ {
        go worker(jobs, results, genome, aiType)
    }

    // Generate deterministic seeds from master seed
    rng := rand.New(rand.NewSource(int64(seed)))
    for i := 0; i < numGames; i++ {
        jobs <- GameJob{SimID: i, Seed: rng.Uint64()}
    }
    close(jobs)

    return aggregateParallelResults(results, numGames)
}
```

### 7. Single Game Execution (Go)

Each game uses memory-pooled state:

```go
// src/gosim/simulation/runner.go
func RunSingleGame(genome *Genome, aiType AIPlayerType, seed uint64) GameResult {
    // Get state from sync.Pool (zero-allocation reuse)
    state := engine.GetState()
    defer engine.PutState(state)

    setupDeck(state, seed)

    for state.TurnNumber < genome.Header.MaxTurns {
        // Check win conditions
        winner := engine.CheckWinConditions(state, genome)
        if winner >= 0 {
            return GameResult{WinnerID: winner, ...}
        }

        // Generate legal moves
        moves := engine.GenerateLegalMoves(state, genome)

        // AI selects move
        var move Move
        switch aiType {
        case RandomAI:
            move = moves[rand.Intn(len(moves))]
        case GreedyAI:
            move = selectBestMove(state, moves, genome)
        case MCTS100AI:
            move = mcts.Search(state, moves, 100, seed)
        }

        // Apply move (mutates state in-place)
        engine.ApplyMove(state, move)

        // Track metrics
        metrics.TotalDecisions++
        metrics.TotalValidMoves += len(moves)
    }

    return GameResult{...}
}
```

### 8. Response Serialization (Go)

Results are packaged back into Flatbuffers:

```go
func serializeStats(builder *flatbuffers.Builder, stats *AggregatedStats) flatbuffers.UOffsetT {
    // Create wins array
    winsOffset := builder.CreateUint32Vector(stats.Wins)

    cardsim.AggregatedStatsStart(builder)
    cardsim.AggregatedStatsAddTotalGames(builder, stats.TotalGames)
    cardsim.AggregatedStatsAddWins(builder, winsOffset)
    cardsim.AggregatedStatsAddDraws(builder, stats.Draws)
    cardsim.AggregatedStatsAddAvgTurns(builder, stats.AvgTurns)

    // Phase 1 instrumentation
    cardsim.AggregatedStatsAddTotalDecisions(builder, stats.TotalDecisions)
    cardsim.AggregatedStatsAddTotalValidMoves(builder, stats.TotalValidMoves)
    cardsim.AggregatedStatsAddForcedDecisions(builder, stats.ForcedDecisions)
    cardsim.AggregatedStatsAddTotalInteractions(builder, stats.TotalInteractions)

    return cardsim.AggregatedStatsEnd(builder)
}
```

### 9. Response Parsing (Python)

Python parses the response and extracts metrics:

```python
# src/darwindeck/simulation/go_simulator.py
response = simulate_batch(request_bytes)
result = response.Results(0)

# Extract all metrics
wins = tuple(result.Wins(i) for i in range(result.WinsLength()))
total_games = result.TotalGames()
total_decisions = result.TotalDecisions()
total_valid_moves = result.TotalValidMoves()

return SimulationResults(
    total_games=total_games,
    wins=wins,
    total_decisions=total_decisions,
    total_valid_moves=total_valid_moves,
    forced_decisions=result.ForcedDecisions(),
    total_interactions=result.TotalInteractions(),
    ...
)
```

### 10. Fitness Computation (Python)

Finally, fitness metrics are computed from simulation results:

```python
# src/darwindeck/evolution/fitness_full.py
def evaluate(self, genome, results):
    decision_density = (
        results.total_valid_moves / results.total_decisions
        if results.total_decisions > 0 else 0.0
    )

    skill_vs_luck = results.player0_wins / results.total_games

    interaction_freq = (
        results.total_interactions / results.total_actions
        if results.total_actions > 0 else 0.0
    )

    total_fitness = sum(
        metric * self.weights[name]
        for name, metric in [
            ('decision_density', decision_density),
            ('skill_vs_luck', skill_vs_luck),
            ('interaction_frequency', interaction_freq),
            ...
        ]
    )

    return FitnessMetrics(total_fitness=total_fitness, ...)
```

## Two-Level Parallelization

### Go-Level (per genome)

- Worker pool in `src/gosim/simulation/parallel.go`
- ~1.43x speedup on 4-core systems
- Best for batch sizes of 500-1000 games

### Python-Level (across genomes)

- Process pool in `src/darwindeck/evolution/parallel_fitness.py`
- ~4x speedup on 4-core systems
- Uses `spawn` context (not `fork`) to avoid corrupting Go runtime

**Combined speedup: 3.3-4.0x end-to-end** on 4-core systems.

## Performance Characteristics

| Metric | Python (genome-based) | Go (genome-based) | Speedup |
|--------|----------------------|-------------------|---------|
| Per game | 15.94ms | 0.40ms | **39.4x** |
| Throughput | 63 games/sec | 2,472 games/sec | - |
| 1M games | ~4.4 hours | ~7 minutes | - |

**Key optimizations:**
1. **Bytecode compilation**: Eliminates Python DSL overhead
2. **Memory pooling**: `sync.Pool` reuses `GameState` allocations
3. **Mutable state**: In-place mutations vs Python's immutable copies
4. **Native code**: Compiled Go vs interpreted Python
5. **Batching**: Amortizes CGo overhead (~1ms per call)

## Comparison with Cython

An alternative approach would be to use **Cython** to accelerate the Python simulation code. This section compares the two approaches.

### What Cython Would Look Like

Cython compiles Python-like code to C, allowing gradual optimization:

```cython
# hypothetical: simulation_core.pyx
cimport cython
from libc.stdlib cimport malloc, free

@cython.boundscheck(False)
@cython.wraparound(False)
cdef class GameState:
    cdef int[52] deck
    cdef int deck_size
    cdef int[4][13] hands  # 4 players, max 13 cards
    cdef int[4] hand_sizes
    cdef int turn_number
    cdef int active_player

    cdef inline void draw_card(self, int player) nogil:
        if self.deck_size > 0:
            self.deck_size -= 1
            card = self.deck[self.deck_size]
            self.hands[player][self.hand_sizes[player]] = card
            self.hand_sizes[player] += 1

cpdef dict simulate_batch(genome_bytes, int num_games, int seed):
    cdef GameState state
    cdef int i, winner
    cdef int[4] wins = [0, 0, 0, 0]

    for i in range(num_games):
        state = GameState()
        setup_game(&state, genome_bytes, seed + i)
        winner = run_game(&state, genome_bytes)
        wins[winner] += 1

    return {"wins": wins, "total": num_games}
```

### Trade-off Analysis

| Aspect | CGo + Flatbuffers (Current) | Cython |
|--------|----------------------------|--------|
| **Language** | Go (separate codebase) | Python-like (same repo) |
| **Learning curve** | Go + CGo + Flatbuffers | Cython syntax + C types |
| **Debugging** | Separate Go debugger | Python debugger (mostly) |
| **Refactoring** | Two codebases to update | Single codebase |
| **Build complexity** | `go build -buildmode=c-shared` + flatc | `cython` + C compiler |
| **Cross-platform** | Compile per platform | Compile per platform |

### Performance Comparison

| Factor | CGo + Go | Cython |
|--------|----------|--------|
| **Raw speed** | Faster (compiled, GC-tuned) | Fast (C speed for typed code) |
| **GIL handling** | Releases GIL entirely | `nogil` blocks release GIL |
| **Parallelism** | Native goroutines | Requires `nogil` + threading |
| **Memory management** | Go GC + sync.Pool | Manual or Python GC |
| **Call overhead** | ~1ms per CGo batch | ~100ns per call (after warmup) |
| **Startup** | Load .so once | Import compiled .so |

**Expected speedups:**

| Approach | vs Pure Python | Notes |
|----------|---------------|-------|
| **Cython (typed)** | 10-50x | Depends on how much is typed |
| **Cython (untyped)** | 1.5-3x | Just compilation benefit |
| **CGo + Go** | 39x (measured) | Full rewrite in Go |

### Why We Chose CGo + Go

1. **Goroutines for parallelism**: Go's goroutines are simpler than managing `nogil` blocks and threading in Cython. The worker pool pattern in `parallel.go` is 20 lines; equivalent Cython would need careful GIL management.

2. **sync.Pool for memory**: Go's `sync.Pool` provides zero-allocation game state reuse. Cython would require manual memory management or accepting Python GC overhead.

3. **MCTS implementation**: Tree search benefits from Go's garbage collector handling node allocation/deallocation. Cython MCTS would need manual memory management for comparable performance.

4. **Ecosystem**: Go has excellent profiling (`pprof`), testing, and benchmarking built-in. Cython debugging is harder when issues occur in compiled code.

5. **Clean separation**: The bytecode boundary enforces a clean API. Python handles genomes and fitness; Go handles simulation. This separation prevents creeping complexity.

### When Cython Would Be Better

Cython would be preferable if:

- **Incremental optimization**: You want to speed up specific functions without rewriting
- **Tight Python integration**: Heavy back-and-forth between Python and fast code
- **NumPy-heavy workloads**: Cython has excellent NumPy integration with typed memoryviews
- **Simpler build**: No need for Flatbuffers schema or cross-language serialization
- **Single codebase**: Team primarily knows Python, not Go

### Hybrid Possibility

A hybrid approach could use both:

```
Python (evolution, fitness)
    ↓
Cython (bytecode interpreter, move generation)  ← Hot path
    ↓
Go via CGo (MCTS only)  ← Tree search benefits most from Go
```

This would reduce CGo crossing frequency while keeping Go for the most complex component (MCTS). However, the current architecture's 39x speedup and clean separation made this unnecessary.

### Code Complexity Comparison

**Cython parallel simulation** (hypothetical):

```cython
from cython.parallel import prange
from libc.stdlib cimport rand

cdef int[1000] run_batch_parallel(bytes genome, int num_games) nogil:
    cdef int[1000] results
    cdef int i

    # Must release GIL for parallel loop
    for i in prange(num_games, nogil=True):
        results[i] = run_single_game(genome, rand())

    return results
```

**Go parallel simulation** (actual):

```go
func RunBatchParallel(genome *Genome, numGames int, seed uint64) AggregatedStats {
    jobs := make(chan GameJob, numGames)
    results := make(chan GameResult, numGames)

    for w := 0; w < runtime.NumCPU(); w++ {
        go worker(jobs, results, genome)
    }

    for i := 0; i < numGames; i++ {
        jobs <- GameJob{SimID: i, Seed: deriveSeed(seed, i)}
    }
    close(jobs)

    return aggregate(results, numGames)
}
```

The Go version is more explicit about worker management but handles edge cases (seed derivation, aggregation) more naturally.

### Summary

| Criterion | Winner | Reason |
|-----------|--------|--------|
| Raw performance | **Go** | Native compilation + goroutines |
| Development speed | **Cython** | Stays in Python ecosystem |
| Parallelism | **Go** | Goroutines vs GIL management |
| Memory control | **Go** | sync.Pool vs manual/GC |
| Debugging | **Cython** | Python tooling works (mostly) |
| Build simplicity | **Cython** | No Flatbuffers, single language |
| Long-term maintenance | **Tie** | Depends on team expertise |

For DarwinDeck's use case (millions of simulations with MCTS), Go's advantages in parallelism and memory management justified the additional complexity of the CGo bridge.

## Building and Running

### Build CGo Library

```bash
make build-cgo  # Produces libcardsim.so
```

### Run Tests

```bash
# Python
uv run pytest tests/ -v

# Go
cd src/gosim && go test ./... -v
```

### Run Benchmarks

```bash
# Compare Python vs Go genome implementations
uv run python benchmarks/compare_genome_implementations.py
```

## Determinism Guarantees

The system ensures reproducibility:

1. **Same genome → Same bytecode**: Deterministic compilation
2. **Same bytecode + seed → Same game**: Deterministic interpreter
3. **Batch seeds derived from master seed**: Parallel runs are reproducible

This enables:
- Debugging specific game sequences
- Comparing fitness across runs
- Verifying Python↔Go equivalence via golden tests (`tests/golden/war_genome.bin`)
