# MCTS Skill Evaluation Design

**Date:** 2026-01-10
**Status:** Validated - Ready for Implementation

## Multi-Agent Review Summary

| Priority | Issue | Resolution |
|----------|-------|------------|
| **High** | First-player bias | Run games both directions, average results |
| **Medium** | Timeout ambiguity | 60s timeout per genome (not per game) |
| **Medium** | No progress reporting | Add "Evaluating X/20" output |
| **Medium** | Parallel evaluation | Reuse ParallelFitnessEvaluator pattern |
| **Low** | Tie eval count to --save-top-n | Eval count = save_top_n |
| **Low** | Preserve original ranking | Output includes both fitness and skill rankings |

## Overview

Add post-evolution MCTS skill evaluation to measure true skill gap in evolved games. After evolution completes, the top candidates are re-evaluated with MCTS vs Random AI to determine how much skilled play matters.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| When to run | Post-evolution only | MCTS is expensive; run once at end |
| Skill measurement | Symmetric head-to-head | Run both directions, average to eliminate first-player bias |
| Evaluation intensity | 100 games, MCTS-500 iterations | Balance of confidence vs speed |
| Result integration | Re-rank for strategic style only | Respects user's style choice |

## Flow

```
Evolution completes (N generations)
    ↓
Get top N unique genomes (N = --save-top-n)
    ↓
MCTS Skill Evaluation (with progress: "Evaluating 1/N...")
  - For each genome:
    - 50 games: MCTS as P1, Random as P2
    - 50 games: Random as P1, MCTS as P2
    - Compute mcts_win_rate = total_mcts_wins / 100
  - 60s timeout per genome; skip if exceeded
    ↓
Re-ranking (conditional)
  - If --style strategic: sort by mcts_win_rate descending
  - Otherwise: keep original fitness order, annotate with mcts_win_rate
    ↓
Save results (JSON includes both fitness rank and skill rank)
    ↓
LLM descriptions include skill analysis
```

## Implementation Components

### 1. New Module: `src/darwindeck/evolution/skill_evaluation.py`

```python
from dataclasses import dataclass
from typing import List, Dict, Optional, Callable
from darwindeck.genome.schema import GameGenome
from darwindeck.simulation.go_simulator import GoSimulator

@dataclass
class SkillEvalResult:
    genome_id: str
    mcts_wins_as_p1: int      # MCTS as Player 1
    mcts_wins_as_p2: int      # MCTS as Player 2
    total_mcts_wins: int      # Combined
    total_games: int
    mcts_win_rate: float      # total_mcts_wins / total_games (0.0-1.0)
    timed_out: bool = False   # True if evaluation was cut short

def evaluate_skill(
    genome: GameGenome,
    num_games: int = 100,
    mcts_iterations: int = 500,
    timeout_sec: float = 60.0,
    progress_callback: Optional[Callable[[str], None]] = None
) -> SkillEvalResult:
    """Run symmetric MCTS vs Random evaluation.

    Runs num_games/2 with MCTS as P1, num_games/2 with MCTS as P2.
    """
    pass

def evaluate_batch_skill(
    genomes: List[GameGenome],
    num_games: int = 100,
    mcts_iterations: int = 500,
    timeout_sec: float = 60.0,
    num_workers: Optional[int] = None,
    progress_callback: Optional[Callable[[int, int], None]] = None
) -> List[SkillEvalResult]:
    """Parallel skill evaluation using ParallelFitnessEvaluator pattern."""
    pass
```

### 2. Flatbuffers Schema Update (`schema/simulation.fbs`)

Add asymmetric AI support:

```flatbuffers
table BatchRequest {
    genome: [ubyte];
    num_games: int;
    ai_type: AIType;           // Keep for backward compat (both players)
    player1_ai_type: AIType;   // NEW: Override for player 1
    player2_ai_type: AIType;   // NEW: Override for player 2
    random_seed: long;
}
```

Logic: If `player1_ai_type` is set (non-zero), use asymmetric mode. Otherwise fall back to `ai_type` for both players.

### 3. Go Runner Changes (`src/gosim/simulation/runner.go`)

```go
func (r *Runner) RunBatch(req *BatchRequest) *BatchResponse {
    p1AI := req.AIType
    p2AI := req.AIType

    // Override if asymmetric mode requested
    if req.Player1AIType != AITypeNone {
        p1AI = req.Player1AIType
    }
    if req.Player2AIType != AITypeNone {
        p2AI = req.Player2AIType
    }

    for i := 0; i < req.NumGames; i++ {
        result := r.RunSingleGame(genome, p1AI, p2AI, seed+i)
        // aggregate results...
    }
}
```

### 4. CLI Flags (`src/darwindeck/cli/evolve.py`)

```python
parser.add_argument(
    '--mcts-games', type=int, default=100,
    help='Games per genome for MCTS skill evaluation'
)
parser.add_argument(
    '--mcts-iterations', type=int, default=500,
    choices=[100, 500, 1000, 2000],
    help='MCTS search iterations per move'
)
parser.add_argument(
    '--skip-skill-eval', action='store_true',
    help='Skip post-evolution MCTS skill evaluation'
)
```

### 5. Engine Integration (`src/darwindeck/evolution/engine.py`)

Add method:

```python
def evaluate_skill_gaps(
    self,
    top_n: int = 20,
    num_games: int = 100,
    mcts_iterations: int = 500
) -> Dict[str, SkillEvalResult]:
    """Run MCTS skill evaluation on top genomes."""
    pass
```

### 6. LLM Description Update (`src/darwindeck/evolution/describe.py`)

```python
def describe_game(
    genome: GameGenome,
    fitness: float,
    mcts_win_rate: Optional[float] = None
) -> str:
    skill_info = ""
    if mcts_win_rate is not None:
        if mcts_win_rate > 0.8:
            skill_info = f"Highly skill-based (skilled players win {mcts_win_rate*100:.0f}% against beginners)."
        elif mcts_win_rate > 0.6:
            skill_info = f"Moderate skill element (skilled players win {mcts_win_rate*100:.0f}%)."
        else:
            skill_info = f"Luck-heavy game (skilled players win only {mcts_win_rate*100:.0f}%)."
    # Include in prompt...
```

## Console Output

```
MCTS Skill Evaluation (100 games, 500 iterations)
  Evaluating genome 1/20: genome_abc...
  Evaluating genome 2/20: genome_def...
  ...
  Evaluating genome 20/20: genome_xyz...

Results:
  1. genome_abc (fitness=0.72, mcts_win_rate=0.83)
  2. genome_def (fitness=0.68, mcts_win_rate=0.76)
  ...

Re-ranking by skill... (--style strategic)
Final Rankings:
  1. genome_abc (mcts_win_rate=0.83, original_rank=1)
  2. genome_ghi (mcts_win_rate=0.79, original_rank=5)
  ...
```

## JSON Output

Add skill evaluation fields to saved genome files:

```json
{
  "genome_id": "abc123",
  "fitness": 0.72,
  "fitness_rank": 1,
  "mcts_win_rate": 0.83,
  "skill_rank": 1,
  "mcts_wins_as_p1": 45,
  "mcts_wins_as_p2": 38,
  "skill_eval_timed_out": false,
  ...
}
```

## Edge Cases

| Case | Handling |
|------|----------|
| MCTS evaluation fails | Log warning, set `mcts_win_rate = None`, continue |
| All games draw | `mcts_win_rate = 0.5`, note potential balance issues |
| Genome takes too long | 60s timeout per genome, mark `timed_out = True`, use partial results |
| User interrupts (Ctrl+C) | Save partial results collected so far |
| First-player advantage | Symmetric evaluation (50 games each direction) eliminates bias |

## Performance Estimates

- MCTS-500: ~30-60 seconds per genome
- Top 20 genomes: 10-20 minutes total
- Acceptable for end-of-run analysis

## Files to Modify

1. `schema/simulation.fbs` - Add asymmetric AI fields
2. `src/gosim/simulation/runner.go` - Support per-player AI types
3. `src/darwindeck/bindings/cgo_bridge.py` - Expose new parameters
4. `src/darwindeck/evolution/skill_evaluation.py` - NEW: Skill evaluation module
5. `src/darwindeck/evolution/engine.py` - Add `evaluate_skill_gaps()` method
6. `src/darwindeck/cli/evolve.py` - Add CLI flags, call skill evaluation
7. `src/darwindeck/evolution/describe.py` - Include skill gap in prompts

## Testing Strategy

1. Unit tests for `SkillEvalResult` computation
2. Integration test: Run skill eval on known genome (War = low skill gap)
3. End-to-end test: Full evolution run with `--style strategic`
