# DarwinDeck v2: Pure Go Rewrite Design

**Date:** 2026-04-11
**Status:** Approved
**Goal:** Evolve novel, fun, playable card games using constrained skeleton templates, massive simulation on a 256-core EPYC, and a pure Go architecture.

## Motivation

The v1 system has fundamental problems:

1. **Evolution converges to one degenerate game.** Every top genome is a ClaimPhase bluffing game with `empty_hand` win conditions. Fitness plateaus at ~0.806. No diversity.
2. **Most evolved games are unplayable.** Playtest data shows games getting stuck, infinite loops, players quitting after 1-2 turns. Validation is structural but doesn't catch behavioral incoherence.
3. **The Python/Go split is broken.** Python 3.13 + CGo = hangs. Evolution runs serially. Timeouts ratcheted from 10s → 2s → 500ms. FlatBuffers serialization through ctypes for a synchronous blocking call.
4. **The genome is too expressive.** 468-line schema, 16+ enums, 7 phase types, recursive compound conditions. Most features go unused. The search space is too large for evolution to find good games.

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Pure Go | Eliminates Python/Go boundary. Existing Go sim code works. 256-core EPYC scales with goroutines. |
| Game representation | Constrained skeleton templates | v1 proved unconstrained search doesn't work. Skeletons guarantee playability by construction. |
| Skeletons at launch | Shedding, Trick-taking, Rummy | Best design space, best skill/luck spectrum, no betting complexity. |
| Novelty mechanism | Borrowed mechanics (whitelist) | Cross-skeleton hybridization via hooks. Bounded expressiveness. |
| Seed games | 8 known-good games across 3 skeletons | Real card games as starting points. Evolution mutates freely from there. |
| Validation | Static analysis + tiered simulation | Kill broken genomes before spending compute. |
| Fitness | 5 metrics (decisions, arc, interaction, skill, length) | Fewer than v1's 8, but each is better defined. |
| GPU/CUDA | Deferred | 256-core EPYC first. Add GPU later if bottlenecked. |
| Existing code | Clean slate in same repo | Keep git history and docs. Wipe src/ and rebuild. |

---

## 1. System Architecture

Three layers, single binary:

```
┌─────────────────────────────────────────────┐
│  CLI / Output                               │
│  (evolution runner, playtest, rulebook gen)  │
├─────────────────────────────────────────────┤
│  Evolution Engine                           │
│  (population, selection, mutation,          │
│   crossover, fitness evaluation)            │
├─────────────────────────────────────────────┤
│  Simulation Core                            │
│  (skeleton runners, AI players,             │
│   static analysis, game state)              │
└─────────────────────────────────────────────┘
```

**Single binary.** One `darwindeck` binary: evolve, playtest, generate rulebooks, analyze results. No IPC, no serialization boundaries.

**Parallelism model.** Goroutines with worker pools. On the 256-core EPYC, one goroutine per genome evaluation. Each goroutine runs its simulation batch sequentially. No shared mutable state — genomes are value types, game states are pool-allocated per-goroutine.

**Data flow:**

```
Seed genomes (JSON)
  → Population init
  → [Mutate/Crossover → Static analysis (kill invalid)
     → Quick sim (5 games, kill broken)
     → Full sim (200+ games) → Fitness scoring → Selection] × N generations
  → Top-N output: genome JSON + rulebook + playtest report
```

---

## 2. Genome Representation

Flat struct. ~20-25 evolvable parameters per skeleton. No recursive conditions, no bytecode.

```go
type Genome struct {
    ID         string
    Generation int
    Skeleton   SkeletonType  // SHEDDING, TRICK_TAKING, RUMMY

    // Shared parameters
    Players    int           // 2-6
    HandSize   int           // 3-13
    MaxTurns   int           // computed from skeleton, not evolved

    // Skeleton-specific params (only one is active)
    Shedding    *SheddingParams
    TrickTaking *TrickTakingParams
    Rummy       *RummyParams

    // Borrowed mechanics (from other skeletons)
    Borrowed   []BorrowedMechanic

    // Shared optional mechanics
    SpecialCards []SpecialCard  // skip, reverse, draw-N, wild
    Scoring      ScoringConfig  // point values for cards/events
    TrumpRule    TrumpRule      // none, fixed suit, cut from deck, led suit
}
```

### Skeleton Parameters

```go
type SheddingParams struct {
    MatchRule     MatchRule  // SUIT, RANK, EITHER, BOTH
    DrawPenalty   int        // 1-3 cards on no match
    CanStack      bool       // chain effects (e.g., draw-2 on draw-2)
    PlayMultiple  bool       // play runs/sets at once
}

type TrickTakingParams struct {
    MustFollowSuit  bool
    TrickScoring    TrickScoring  // PER_TRICK, CARD_POINTS, AVOIDANCE
    LeadRestriction LeadRule      // NONE, NO_TRUMP_UNTIL_BROKEN, WINNER_LEADS
    RoundsPerGame   int           // 1-13
}

type RummyParams struct {
    MeldTypes      MeldType    // SETS, RUNS, BOTH
    MinMeldSize    int         // 2-4
    DrawFrom       DrawSource  // DECK, DISCARD, EITHER
    CanLayOff      bool        // extend opponent's melds
    KnockThreshold int         // deadwood points to knock (0 = gin only)
}
```

### Borrowed Mechanics

Cross-skeleton hybridization via a hardcoded whitelist (~8-10 valid borrows):

```go
type BorrowedMechanic struct {
    Source    SkeletonType
    Mechanic MechanicType  // e.g., TRICK_SCORING, MELD_BONUS, DRAW_PENALTY
    Config   MechanicConfig
}
```

Borrowed mechanics register hooks (before-turn, after-play, end-of-round, scoring) that skeleton runners call at defined points. The runner controls flow; the borrowed mechanic adds behavior.

Static analysis rejects incoherent borrows.

---

## 3. Skeleton Runners

Each skeleton is a hardcoded Go game loop implementing a common interface:

```go
type SkeletonRunner interface {
    Setup(genome *Genome, rng *rand.Rand) *GameState
    GenerateMoves(state *GameState) []Move
    ApplyMove(state *GameState, move Move) []Event
    CheckEnd(state *GameState) int
}
```

**Why hardcoded runners instead of an interpreter:**
- A shedding runner *always* generates valid shedding moves — "draw if you can't play" is built into the loop
- Trick-taking *always* completes a round of tricks — can't get stuck mid-trick
- Parameters control *what* happens, not *whether* the game works
- No bytecode, no condition trees, no possibility of unsatisfiable states

**Game state:**

```go
type GameState struct {
    Deck      []Card
    Hands     [][]Card
    Discard   []Card
    Tableau   [][]Card   // melds, tricks won, etc.
    Scores    []int
    Turn      int
    Active    int
    Phase     PhaseType

    // Hook results from borrowed mechanics
    TrickWinner    int
    MeldsThisRound []Meld
}
```

**AI players** (all share the SkeletonRunner interface):
- **Random:** uniform selection from legal moves
- **Greedy:** heuristic scoring per skeleton
- **MCTS:** tree search, configurable iterations

---

## 4. Validation Pipeline

### Tier 0: Static Analysis (free, instant)

Pure function on the genome struct:

- HandSize × Players ≤ 52
- All parameters within skeleton's valid ranges
- Borrowed mechanics on the whitelist and don't conflict
- Scoring config reachable (points-based win → something awards points)
- Trump rule coherent with skeleton

Any violation = zero fitness, no simulation.

### Tier 1: Quick Simulation (5 games, ~0.1ms)

Run 5 games with random AI. Kill genome if ANY:

- Game hits MaxTurns (infinite loop)
- Game errors (panic, illegal state)
- Same player wins all 5 (degenerate)
- All 5 are draws (no winner possible)
- Average turns < 3 (instant end)

Expected to kill 30-50% of mutated genomes.

### Tier 2: Full Simulation (200+ games)

Only surviving genomes reach this tier:

| AI Type | Games | Cost | Purpose |
|---------|-------|------|---------|
| Random | 200 | cheap | Completion rate, turn distribution, winner balance |
| Greedy | 50 | ~5x | Greedy vs random win rate |
| MCTS | 20 | ~50x | Skill gradient measurement |

Pyramid shape: concentrate expensive compute where it matters.

**Estimated throughput on EPYC:** Tier 0+1 filters are essentially free. Tier 2 for a surviving genome: 10-50ms. Population of 500 where half survive: under 1 second per generation.

---

## 5. Fitness Function

Five metrics, each 0.0-1.0, weighted sum:

### Meaningful Decisions (weight: 0.25)

Not "did the player have >1 legal move" but "did the choice matter." At sampled decision points, compare: what happens if the player picks their best move vs. a random move? Track how often the final winner changes. Higher divergence = more meaningful decisions.

Sample ~10 decision points per game across the 200 random-AI games.

### Game Arc (weight: 0.25)

At each turn, estimate "who's winning" (score lead, cards remaining, tricks won). Compute entropy of the lead over time. Good games:
- High entropy early (anyone could win)
- Decreasing entropy mid-game (strategies emerge)
- Resolution at end (clear winner)

Penalize flat (boring), monotone (foregone conclusion), oscillating-without-resolution.

### Interaction (weight: 0.20)

After each player's move, measure how many other players' legal move sets changed. Cheap: snapshot legal moves before/after each play, count diffs.

### Skill Gradient (weight: 0.20)

```
random_wr  = random AI win rate (should be ~1/N)
greedy_wr  = greedy AI win rate vs random
mcts_wr    = MCTS AI win rate vs random

skill = (greedy_wr - random_wr) * 0.4 + (mcts_wr - greedy_wr) * 0.6
```

Good games: random < greedy < MCTS. Flat = no skill. Greedy ≈ MCTS = too shallow.

### Session Length (weight: 0.10)

Target: 15-40 turns. Linear dropoff outside range. Below 5 or above 100 = zero.

---

## 6. Evolution Engine

- **Population:** 500 genomes
- **Selection:** Tournament, size 5
- **Elitism:** Top 10 carry forward unchanged
- **Diversity:** Hash-based clone detection in top 50

### Mutation Operators

| Operator | Probability | Description |
|----------|------------|-------------|
| Tweak parameter | 0.40 | Nudge one numeric param ±1-2 within valid range |
| Flip boolean | 0.15 | Toggle one bool |
| Change enum | 0.15 | Swap one enum value |
| Add special card | 0.08 | Add skip/reverse/draw-N/wild |
| Remove special card | 0.07 | Remove one special card |
| Add borrowed mechanic | 0.05 | Pull valid mechanic from another skeleton |
| Remove borrowed mechanic | 0.05 | Drop one borrowed mechanic |
| Change skeleton | 0.02 | Swap primary skeleton, reinit params from seed |
| Mutate scoring | 0.03 | Modify card point values or scoring triggers |

Multiple mutations can fire independently per genome per generation.

### Crossover

Uniform crossover between same-skeleton genomes. Each parameter from parent A or B with 50/50 probability. No cross-skeleton crossover.

---

## 7. Seed Games

8 seed games across 3 skeletons:

| Skeleton | Seed | Key Characteristics |
|----------|------|---------------------|
| Shedding | Crazy Eights | Match suit or rank, draw on miss |
| Shedding | Mau-Mau | Crazy Eights + special card effects |
| Trick-taking | Whist | Simplest trick-taker, follow suit, trump |
| Trick-taking | Hearts | Trick avoidance, point cards |
| Trick-taking | Spades | Always-trump, partnership-lite |
| Trick-taking | Oh Hell | Exact-bid scoring, precision over power |
| Rummy | Gin Rummy | Sets and runs, deadwood, knock/gin |
| Rummy | Knock Rummy | Simpler scoring, everyone pays deadwood |

Seeds are starting positions only — evolution can mutate freely in any direction with no gravity toward the original game.

---

## 8. Output & Reporting

Top-N genomes (20) per run get a full output package:

```
output/YYYY-MM-DD_HH-MM-SS/
├── summary.json              # Run metadata, best fitness, generation stats
├── fitness_curve.csv         # Best/avg/worst fitness per generation
├── games/
│   ├── rank01_NovelGame/
│   │   ├── genome.json       # Machine-readable genome
│   │   ├── rulebook.md       # Human-readable rules
│   │   ├── report.md         # Playtest analysis report
│   │   └── transcripts/      # 3-5 sample game transcripts
│   ├── rank02_AnotherGame/
│   │   └── ...
│   └── ...
└── population_final.json     # Full population for resume
```

**Rulebook generator:** Template-driven per skeleton. Borrowed mechanics appended as "Additional rules." Card values get a scoring table.

**Playtest report:** Stats (game length, skill gradient, decision density, interaction), plus a heuristic "what makes it interesting" section based on which metrics are highest and which borrowed mechanics are present. Template-driven, not LLM-generated.

**Resume:** `population_final.json` allows restarting from the final population with different parameters or more generations.

---

## 9. Project Structure

```
darwindeck/
├── cmd/
│   └── darwindeck/        # Single binary entry point
│       └── main.go
├── pkg/
│   ├── genome/            # Genome struct, serialization, validation
│   ├── skeleton/
│   │   ├── shedding/      # Shedding runner + greedy AI
│   │   ├── tricktaking/   # Trick-taking runner + greedy AI
│   │   └── rummy/         # Rummy runner + greedy AI
│   ├── mechanic/          # Borrowed mechanics, hooks, whitelist
│   ├── sim/               # Game state, AI interface, MCTS, batch runner
│   ├── evolution/         # Population, selection, mutation, crossover
│   ├── fitness/           # 5 metrics + tiered validation pipeline
│   ├── output/            # Rulebook gen, report gen, transcript writer
│   └── seeds/             # 8 seed game definitions
├── seeds/                 # Seed genome JSON files
├── output/                # Evolution run results
├── docs/
│   └── plans/
└── go.mod
```

---

## 10. Implementation Order

1. **Genome + seeds** — structs, 8 seed games, serialization, static validation
2. **Skeleton runners** — shedding first (simplest), then trick-taking, then rummy
3. **Random AI + batch sim** — run games, collect stats, verify all 8 seeds work
4. **Tiered validation** — Tier 0 static analysis, Tier 1 quick sim filter
5. **Fitness function** — all 5 metrics, verified against seed games
6. **Evolution engine** — mutation, crossover, selection, population, goroutine parallelism
7. **Greedy + MCTS AI** — enables skill gradient metric
8. **Output pipeline** — rulebook gen, report gen, transcripts, resume support
9. **Borrowed mechanics** — cross-skeleton hooks, whitelist, mutation operators
10. **CLI polish** — playtest mode, flags, progress output

Steps 1-6 produce a working evolution system on shedding alone. Steps 7-9 add depth. Step 10 is polish.

---

## Hardware Targets

- **Primary:** 256-core AMD EPYC — goroutine parallelism, 500-genome populations
- **Development:** 14th gen i7 — smaller populations, faster iteration
- **GPU (deferred):** NVIDIA 5090 — add CUDA simulation kernel if EPYC throughput bottlenecks
