# Multi-Agent Consensus Analysis

**Mode:** general-prompt
**Date:** 2026-01-10 15:03:23
**Agents Succeeded:** 3/3
**Chairman:** Claude

---

## Original Question

Design an implementation plan for proper fitness metric instrumentation that balances accuracy with performance. For each metric that needs improvement (decision_density, tension_curve, interaction_frequency, skill_vs_luck):

1. What data should the Go simulator collect during gameplay?
2. How should this data be aggregated and returned to Python?
3. What's the performance impact and how can we minimize it?
4. Should we implement all metrics at once or prioritize?
5. Are there any sampling strategies to reduce overhead?

Provide specific architectural recommendations considering our 256-core parallelization setup and the need to maintain 800K+ games/second throughput.


## Context

# DarwinDeck Fitness Function - Long-term Implementation Plan

## Current State

DarwinDeck is an evolutionary card game system that uses genetic algorithms to evolve novel card games. We recently improved the fitness function from placeholder implementations to better heuristics:

**Before:** 0.7000 ceiling (hardcoded placeholders)
**After:** 0.8403 best fitness (improved heuristics)

## Architecture

- **Python simulation engine:** Interprets game genomes
- **Go simulation core:** High-performance Monte Carlo runner (800K+ games/sec on 256 cores)
- **Fitness metrics (6 components, equal weight ~0.167 each):**
  1. decision_density
  2. comeback_potential
  3. tension_curve
  4. interaction_frequency
  5. rules_complexity
  6. skill_vs_luck

## Current Implementations (Heuristics)

### 1. Decision Density
**Current:** Counts optional phases, conditions, total phases
**Problem:** Doesn't measure actual player choices during gameplay
**Need:** Track real decision points (e.g., "player had 3 valid moves" vs "forced move")

### 2. Comeback Potential
**Current:** Measures win rate balance (1.0 - |win_rate - 0.5| * 2)
**Status:** ✅ Good enough (measures balance well)

### 3. Tension Curve
**Current:** Game length proxy (longer games = more tension)
**Problem:** Doesn't track actual uncertainty over time
**Need:** Win probability variance over game duration

### 4. Interaction Frequency
**Current:** Counts special effects, trick-based play, phases
**Problem:** Doesn't measure actual player interactions
**Need:** Track actions affecting opponents (stealing, blocking, etc.)

### 5. Rules Complexity
**Current:** Counts game elements (phases, effects, rules)
**Status:** ✅ Good enough (simpler = better)

### 6. Skill vs Luck
**Current:** Estimates from game structure (length, balance, complexity)
**Problem:** Doesn't measure actual skill ceiling
**Need:** MCTS win rate vs random baseline

## Technical Constraints

- **Performance critical:** Evaluating 100-500 genomes × 100 simulations each
- **Go simulator:** Currently returns only aggregated stats (win counts, avg turns)
- **Python/Go bridge:** FlatBuffers for serialization
- **Parallelization:** 256 workers on server (critical to maintain)

## Files

- `src/darwindeck/evolution/fitness_full.py` - Fitness evaluator
- `src/darwindeck/evolution/parallel_fitness.py` - Parallel evaluation wrapper
- `src/gosim/simulation/runner.go` - Go Monte Carlo runner
- `schema/simulation.fbs` - FlatBuffers schema for Python/Go interface


---

## Stage 1: Independent Analyses

### Claude Review

# Independent Analysis: Fitness Metric Instrumentation Plan

## Strong Points

### 1. Prioritization Strategy: Incremental Implementation
- **Recommend phased rollout**: Start with `interaction_frequency` → `decision_density` → `tension_curve` → `skill_vs_luck`
- **Rationale**: First three can be collected with minimal overhead (counters during existing game loop). `skill_vs_luck` requires MCTS which is computationally expensive and should be a separate phase.
- This matches the "2 metrics already good enough" observation - focus resources on the 4 that need work.

### 2. Data Collection Architecture for Go Simulator
- **decision_density**: Track `(valid_moves_count, was_forced_move)` per decision point. Aggregate as: `sum(moves > 1) / total_decisions`
- **tension_curve**: Capture win probability at fixed intervals (e.g., every 10% of game progress). Return as fixed-size array [10 floats] per game.
- **interaction_frequency**: Increment counter when action targets opponent's cards/state. Return `interactions / total_actions`
- **Key insight**: All three can use simple counters/arrays that fit in existing per-game structs with ~100 bytes overhead.

### 3. Aggregation Strategy
- Return percentile distributions, not just means: `[p10, p25, p50, p75, p90]` for each metric
- This enables detecting bimodal games (sometimes interactive, sometimes not) which simple averages miss
- FlatBuffers schema extension: Add `MetricDistribution` table with fixed-size arrays

## Moderate Points

### 4. Performance Impact Analysis
- **Counter-based metrics**: ~1-3% overhead (branch prediction-friendly increment operations)
- **Win probability tracking**: ~5-10% overhead if using simple heuristic (card advantage ratio)
- **MCTS for skill_vs_luck**: 10-100x overhead - must be sampled
- At 800K games/sec on 256 cores, a 5% slowdown = 40K fewer games/sec, which is acceptable for better metrics

### 5. Sampling Strategies
- **Stratified sampling for MCTS**: Run on 1% of games (1000 of 100K simulations), extrapolate
- **Adaptive tension sampling**: Only record win probability when it changes by >5% (reduces storage, maintains signal)
- **Batch aggregation**: Accumulate metrics in thread-local buffers, merge at batch boundaries (reduces lock contention)

### 6. Memory Layout Optimization
- Pre-allocate metric arrays per worker (256 workers × 1KB = 256KB total)
- Use ring buffers for tension curve data to avoid allocations
- Pack metrics into cache-aligned structs to minimize false sharing

## Weak Points / Concerns

### 7. Win Probability Estimation is Hard
- For `tension_curve`, we need win probability during gameplay, but actual win probability requires simulation (circular dependency)
- **Workaround**: Use heuristic proxies (card advantage, resource ratio), but these may not correlate with actual outcomes for novel games
- **Risk**: Metric becomes "card_advantage_variance" not "tension_curve" - may optimize for wrong thing

### 8. MCTS for Skill vs Luck is Problematic at Scale
- MCTS requires 100-1000 playouts per decision to be meaningful
- For a 20-decision game at 500 playouts = 10,000 simulated moves per game
- Even at 1% sampling, this could dominate evaluation time
- **Alternative**: Consider using decision tree depth or "perfect information win rate change" as proxy

### 9. Interaction Classification is Genome-Dependent
- What counts as "interaction"? Stealing a card? Playing a card that triggers opponent response?
- Current genomes may not encode action targets explicitly
- **Risk**: May need to modify genome representation to enable this metric, which is a larger change

### 10. 256-Core Scalability Concerns
- Current approach likely uses per-game parallelism
- Adding metric aggregation introduces synchronization points
- Need to verify that lock-free aggregation (atomic increments) scales to 256 cores without contention
- Consider NUMA-aware allocation if running on multi-socket system

### 11. FlatBuffers Schema Evolution
- Extending the schema requires coordinated Python/Go changes
- Should version the schema to allow gradual rollout
- Risk of breaking existing cached results or serialized genomes

## Architectural Recommendation Summary

| Metric | Implementation | Overhead | Priority |
|--------|---------------|----------|----------|
| interaction_frequency | Counter per action | <1% | 1 (easiest) |
| decision_density | Track valid moves per decision | 2-3% | 2 |
| tension_curve | Heuristic win prob sampling | 5-8% | 3 |
| skill_vs_luck | 1% sampled MCTS | +50% on samples | 4 (defer) |

---

### Gemini Review

Here is the independent analysis of the implementation plan for the DarwinDeck fitness metric instrumentation.

## Strong Points

*   **Tiered Evaluation Strategy (The "Funnel" Approach):**
    *   **Recommendation:** Do not run expensive metrics (specifically Skill vs. Luck) on every genome. Use a multi-pass approach.
    *   **Logic:** Run the fast, heuristic-based simulation (800K/sec) for the entire population. Take the top 5-10% of candidates and run the expensive MCTS-based "Skill vs. Luck" analysis only on them. This preserves the evolutionary throughput while getting high-fidelity data where it matters.
    *   **Architecture:** The Python `FitnessEvaluator` needs a pipeline: `Pre-filter (Heuristic) -> Sim (Fast Metrics) -> Elite Sim (MCTS)`.

*   **In-Engine Aggregation (Go-side Compute):**
    *   **Recommendation:** Never return raw frame-by-frame data to Python.
    *   **Logic:** Transferring per-turn data for 800K games via FlatBuffers will saturate the memory bandwidth and serialization capabilities, crashing performance.
    *   **Implementation:** The Go `SimulationRunner` should calculate `avg_branching_factor`, `total_interactions`, and `lead_changes` locally using accumulators (integers/floats) and return only the final scalar values in the `SimulationResult` struct.

*   **Stateless/Thread-Local Accumulators:**
    *   **Recommendation:** Ensure metric collection is thread-local per simulation instance.
    *   **Logic:** With 256 cores, cache contention is the primary enemy. If workers try to write to shared statistical objects, performance will tank. Each Go routine must maintain its own stack-allocated counters (e.g., `current_game_interactions`, `current_game_decisions`) that are only written to the result buffer at the very end of the game loop.

## Moderate Points

*   **Tension Curve Proxies (Avoiding "Win Probability"):**
    *   **Recommendation:** Use "Score Volatility" or "Lead Changes" as a proxy for Tension, rather than calculating actual Win Probability.
    *   **Logic:** True Tension requires calculating "Win Probability" at every turn, which requires running sub-simulations inside the main simulation ($O(N^2)$ complexity). This is impossible at scale.
    *   **Data:** Track the delta of the game state evaluation function (or simply score) relative to the opponent. If the lead flips often, tension is high. If the lead grows linearly, tension is low.

*   **Decision Density Implementation:**
    *   **Recommendation:** Instrument the `GetValidMoves()` function.
    *   **Data:** Collect `sum_valid_moves` and `turn_count`.
    *   **Aggregation:** Return `avg_moves_per_turn = sum_valid_moves / turn_count`.
    *   **Nuance:** You must differentiate between "Optional Moves" (playing a card) and "Mandatory Moves" (passing because you have no cards). A game with high "pass" frequency has low decision density even if the branching factor looks non-zero.

*   **Metric Prioritization:**
    *   **Order:**
        1.  **Decision Density:** Easiest to implement (just count list length), high impact on identifying "autopilot" games.
        2.  **Interaction Frequency:** Requires tagging game events (e.g., `EventTag.TargetOpponent`). Moderate effort.
        3.  **Tension Curve:** Harder to define generically across evolved games.
        4.  **Skill vs. Luck:** Highest architectural cost (MCTS integration).

## Weak Points / Concerns

*   **MCTS Performance Impact (Skill vs. Luck):**
    *   **Concern:** Even on a subset of games, MCTS is orders of magnitude slower than random playouts.
    *   **Risk:** If the evolutionary loop waits for MCTS to evaluate the top 10% of genomes, the entire pipeline stalls.
    *   **Mitigation:** The "Skill vs. Luck" metric might need to be an asynchronous "Audit" process rather than part of the generation loop, or strict limits must be placed on MCTS depth/iterations.

*   **Generic Interaction Definitions:**
    *   **Concern:** "Interaction" is hard to define in an evolutionary system where rules change.
    *   **Risk:** Is drawing a card an interaction? Is discarding? The system might game the metric by generating meaningless "touch opponent" actions that don't actually impact the win condition.
    *   **Mitigation:** Only count interactions that change the *opponent's* state (score, hand size, board state) negatively.

*   **FlatBuffers Schema Rigidity:**
    *   **Concern:** Changing the `.fbs` schema requires recompiling both Go and Python bindings.
    *   **Risk:** Frequent iterations on which metrics to collect will be painful.
    *   **Mitigation:** Add a generic `float32 metrics[8]` array to the FlatBuffers schema immediately, mapping indices to specific metrics in code (0=decisions, 1=interactions, etc.) to allow logic updates without schema recompilation during the experimentation phase.

---

### Codex Review

## Strong Points
- Emphasize doing the heavy lifting in the Go core: compute simple counters/aggregates per game and only ship summaries back via FlatBuffers, avoiding per-move Python callbacks.
- Identify minimal data per metric: number of legal moves per turn for decision_density, counts of opponent-targeting actions for interaction_frequency, per-turn score/lead changes for tension_curve, and occasional MCTS‑vs‑random outcomes for skill_vs_luck.
- Recommend per‑worker/local accumulators to avoid contention and amortize atomic operations, then reduce at batch end before returning to Python.
- Suggest sampling instrumentation (e.g. instrument 1–5 % of games or a subset of turns) to keep the inner loop hot and maintain throughput.
- Advocate for incremental rollout: prioritize the least intrusive metrics first and measure impact, rather than flipping everything at once.
- Note the need to extend the FlatBuffers schema to carry additional aggregated fields while keeping the payload small.

## Moderate Points
- Using coarse proxies (e.g. lead changes or variance instead of full win probability curves) for tension_curve can reduce complexity but may oversimplify.
- Running MCTS vs random games for skill_vs_luck on a subset of genomes/games could provide signal without overwhelming the scheduler, but requires careful budgeting and possibly separate worker pools.
- Deciding thresholds for what counts as “interaction” or “decision” might need tuning based on game semantics.
- The bridge between Python and Go could be a bottleneck if too many metrics are returned per game; batching over many simulations mitigates this.

## Weak Points / Concerns
- Full per-turn instrumentation in all games will almost certainly impact the 800 K+/s target; any plan must explicitly quantify and cap overhead.
- Estimating “win probability” in tension_curve accurately in a stochastic, evolving game might require expensive forward simulations that aren’t feasible in the main loop.
- Skill_vs_luck via MCTS introduces a different search cost model; integrating it into the existing 256‑core Monte Carlo setup without starving throughput is nontrivial.
- Implementing all metric changes simultaneously could make performance regression debugging difficult; lacking a staged plan with benchmarks would be risky.

---

## Stage 2: Chairman Consensus (by Claude)

# Chairman Synthesis: Fitness Metric Instrumentation Plan

## Areas of Agreement

**All three analyses strongly converge on these points:**

1. **Go-side aggregation is mandatory** - Never return per-turn data to Python. Compute counters/aggregates in Go, return only scalars or small fixed-size arrays via FlatBuffers.

2. **Thread-local/per-worker accumulators** - All reviewers emphasize avoiding shared state. Each goroutine maintains its own counters, merged only at batch boundaries to prevent cache contention on 256 cores.

3. **Incremental rollout is essential** - Do not implement all metrics simultaneously. All agree on roughly this priority order:
   - **P1:** `interaction_frequency` / `decision_density` (counter-based, <3% overhead)
   - **P2:** `tension_curve` (heuristic proxy, 5-8% overhead)
   - **P3:** `skill_vs_luck` (MCTS-based, must be heavily sampled or deferred)

4. **MCTS for skill_vs_luck is the hard problem** - Unanimous concern that MCTS is 10-100x more expensive and cannot run on all games. Must be sampled (1-10%) or run asynchronously.

5. **Sampling strategies are necessary** - All support instrumenting a subset of games or turns to preserve throughput, especially for expensive metrics.

6. **FlatBuffers schema needs extension** - All note this requires coordinated Go/Python changes.

## Areas of Disagreement

### **Conflict 1: Tension Curve Implementation**

| Reviewer | Approach | Risk Identified |
|----------|----------|-----------------|
| Claude | Heuristic win probability at fixed intervals (10 floats per game) | "May become card_advantage_variance, not tension" |
| Gemini | Lead changes / score volatility as proxy | Easier but "harder to define generically across evolved games" |
| Codex | Coarse proxies (variance, lead changes) | "May oversimplify" |

**Explicit conflict:** Claude wants to attempt win probability estimation; Gemini and Codex prefer simpler lead-change proxies. No consensus on whether the complexity of win probability is worth the accuracy gain.

### **Conflict 2: Skill vs. Luck Architecture**

| Reviewer | Approach |
|----------|----------|
| Claude | 1% sampled MCTS within main evaluation loop |
| Gemini | Async "audit" process, separate from generation loop; or strict MCTS limits |
| Codex | Separate worker pools, careful budgeting |

**Explicit conflict:** Should MCTS sampling be synchronous (blocking elite evaluation) or asynchronous (decoupled audit)? Gemini explicitly warns synchronous approach "will stall the entire pipeline."

### **Conflict 3: Interaction Definition**

| Reviewer | Concern |
|----------|---------|
| Claude | May need genome representation changes to encode action targets |
| Gemini | Risk of gaming metric with "meaningless touch opponent actions" |
| Codex | Thresholds need tuning based on game semantics |

**No clear resolution** on how to define "interaction" in evolved games with novel rules. This is a semantic problem, not just an implementation one.

### **Conflict 4: FlatBuffers Evolution Strategy**

| Reviewer | Approach |
|----------|----------|
| Claude | Version the schema for gradual rollout |
| Gemini | Add generic `float32 metrics[8]` array to avoid recompilation during experimentation |
| Codex | Extend schema but keep payload small |

**Minor conflict:** Gemini's generic array approach trades type safety for iteration speed. Claude prefers explicit versioning.

## Confidence Level

**Medium-High** on architecture fundamentals (Go-side aggregation, thread-local state, incremental rollout).

**Medium** on tension_curve approach (legitimate disagreement on proxy vs. probability).

**Low** on skill_vs_luck integration (all reviewers flag this as architecturally risky, no proven solution).

## Synthesized Recommendation

### Phase 1: Low-Risk Counter Metrics (Week 1)
```
Target: <3% overhead, maintain 780K+ games/sec
```

| Metric | Data Collection (Go) | Aggregation | Return Format |
|--------|---------------------|-------------|---------------|
| `decision_density` | Increment `valid_moves_sum` at each decision; track `decision_count` | `avg = sum / count`; flag forced moves separately | 2 floats |
| `interaction_frequency` | Increment counter when action modifies opponent state (score, hand, board) | `ratio = interactions / total_actions` | 1 float |

**Implementation notes:**
- Use thread-local accumulators per worker
- Merge at batch boundaries with lock-free atomics
- Extend FlatBuffers with explicit fields (not generic array yet—defer Gemini's approach until schema churn is proven problematic)

### Phase 2: Tension Proxy (Week 2-3)
```
Target: <8% additional overhead
```

**Adopt hybrid approach to resolve disagreement:**
1. Track **lead changes** (Gemini/Codex approach) as primary signal—count sign flips in score differential
2. Track **score variance** across game progression
3. **Do NOT attempt win probability estimation initially**—Claude's concern about circular dependency and correlation risk is valid

Return: `[lead_changes: int, score_variance: float, final_margin: float]`

**Explicitly defer** full win probability curves until we validate simpler proxies correlate with perceived tension.

### Phase 3: Skill vs. Luck (Week 4+, Experimental)
```
Target: Run on ≤5% of elite genomes, accept 50% slowdown on that subset
```

**Resolve architectural conflict with tiered approach:**

1. **Main loop**: Fast heuristic evaluation for all genomes (800K/sec)
2. **Elite filter**: Top 10% of population identified
3. **MCTS audit** (Gemini's async model): 
   - Separate worker pool (suggest 32 of 256 cores dedicated)
   - Run 100-game MCTS evaluation on elite genomes only
   - Results feed into *next* generation's selection, not current (async)

**Alternative (Claude's proxy):** If MCTS proves too expensive even on elites, fall back to "perfect-information win rate comparison" or "decision tree depth" as suggested.

### FlatBuffers Strategy
- **Week 1:** Add explicit fields for Phase 1 metrics
- **If schema churn becomes painful:** Migrate to Gemini's `float32 metrics[N]` array with code-level mapping
- Version all schema changes to maintain backward compatibility with cached results

### Performance Validation Gates
Before each phase ships:
1. Benchmark on 256-core setup
2. Reject if throughput drops below 750K games/sec (after Phase 1) or 700K (after Phase 2)
3. Profile for cache contention and NUMA effects (Claude's concern)

### Open Questions Requiring Resolution
1. **Interaction definition:** Convene to define precisely what "modifies opponent state" means for current genome structure. May require small genome changes.
2. **Tension metric validation:** After Phase 2, compare lead-change proxy against human-perceived tension in sample games before trusting it for fitness.
3. **MCTS budget:** Empirically determine max playouts/decision that keeps elite evaluation under 10 seconds per genome.

