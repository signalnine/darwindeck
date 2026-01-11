# Skill Evaluation System

DarwinDeck uses a two-tier skill evaluation system to measure how much skill (vs luck) influences game outcomes. This helps ensure evolved games reward good play rather than being purely random.

## Overview

The skill evaluation system answers two key questions:

1. **Does basic strategy help?** (Greedy AI vs Random AI)
2. **What's the skill ceiling?** (MCTS AI vs Random AI)

Games where skilled play significantly outperforms random play are considered "skill games." Games where random play performs similarly to skilled play are considered "luck games."

## Two-Tier Evaluation

### Tier 1: Greedy vs Random (Fast)

The Greedy AI uses simple heuristics:
- Play the highest value card when possible
- Prefer moves that reduce hand size (shedding games)
- Avoid taking penalty cards (Hearts-style games)

If Greedy beats Random significantly more than 50% of the time, the game rewards basic strategy.

**Interpretation:**
- Win rate ~50%: No basic strategy advantage (pure luck or very subtle strategy)
- Win rate 60-70%: Moderate skill component
- Win rate 80%+: Strong skill component

### Tier 2: MCTS vs Random (Deeper Analysis)

Monte Carlo Tree Search (MCTS) uses tree search with random rollouts to find strong moves. This measures the "skill ceiling" - how much better can a very strong player perform?

**MCTS Configuration:**
- 100 iterations: Fast evaluation (default during evolution)
- 500 iterations: Moderate depth (default for final evaluation)
- 1000+ iterations: Deep analysis (research/validation)

**Interpretation:**
- Win rate similar to Greedy: Skill ceiling is low (simple tactics suffice)
- Win rate higher than Greedy: Deep strategy matters (higher skill ceiling)

## Bidirectional Testing

Each test runs in **both directions** to eliminate first-player bias:

```
Test 1: Skilled AI as Player 0 vs Random as Player 1
Test 2: Random as Player 0 vs Skilled AI as Player 1
```

The combined win rate averages both directions, giving a fair skill measurement regardless of turn order advantage.

## Key Metrics

### Skill Score (0.0 - 1.0)

Combined metric: `(greedy_win_rate + mcts_win_rate) / 2`

| Score | Interpretation |
|-------|----------------|
| 0.50 | Pure luck (50/50 random vs skilled) |
| 0.60 | Low skill component |
| 0.70 | Moderate skill component |
| 0.80+ | High skill component |

### First Player Advantage (-1.0 to +1.0)

Measures turn order bias: `P0_win_rate - P1_win_rate`

| FPA | Interpretation |
|-----|----------------|
| 0.0 | Perfectly balanced |
| +0.1 to +0.3 | Slight P0 advantage |
| +0.3+ | Strong P0 advantage (problematic) |
| -0.3 | Strong P1 advantage (problematic) |

Games with |FPA| > 0.3 are penalized during evolution.

## Integration with Evolution

### In-Evolution Skill Evaluation

During evolution, skill evaluation runs periodically on top performers:

```bash
uv run python -m darwindeck.cli.evolve \
    --skill-eval-frequency 10 \    # Every 10 generations
    --fpa-penalty-threshold 0.3 \  # Penalize if |FPA| > 30%
    --low-skill-threshold 0.6      # Penalize if skill < 60%
```

**Configuration Options:**

| Flag | Default | Description |
|------|---------|-------------|
| `--skill-eval-frequency` | 10 | Run every N generations (0 = disabled) |
| `--skill-eval-games` | 10 | Games per evaluation (fast) |
| `--fpa-penalty-threshold` | 0.3 | Penalize if |FPA| exceeds this |
| `--fpa-penalty-weight` | 0.3 | Fitness reduction (30%) |
| `--low-skill-threshold` | 0.6 | Penalize if skill_score below this |
| `--low-skill-penalty` | 0.2 | Fitness reduction (20%) |

### Penalty Application

When a genome exceeds thresholds, its fitness is reduced:

```python
# High first-player advantage penalty
if abs(fpa) > 0.3:
    fitness *= 0.7  # 30% penalty

# Low skill penalty (normal styles)
if skill_score < 0.6:
    fitness *= 0.8  # 20% penalty

# For "party" style, logic is inverted:
# Penalize HIGH skill (we want luck-friendly party games)
if skill_score > 0.6:
    fitness *= 0.8
```

### Caching

Skill evaluation results are cached by genome_id during evolution to avoid redundant computation. The cache persists across generations.

## Final Skill Evaluation

After evolution completes, run comprehensive skill evaluation on winners:

```bash
# Automatic (included in normal evolution run)
# Final evaluation uses 500 MCTS iterations on top 20 genomes

# Manual evaluation of saved genomes
uv run python -m darwindeck.cli.describe genome.json -v
```

The describe command shows skill metrics when available:
```
Skill Evaluation:
  Greedy win rate: 72%
  MCTS win rate: 81%
  First player advantage: +8%
  Overall skill score: 0.76
```

## Fitness Style Interaction

Different fitness styles handle skill differently:

| Style | Skill Preference | FPA Tolerance |
|-------|-----------------|---------------|
| `balanced` | Moderate skill (0.6+) | Low (< 0.3) |
| `strategic` | High skill (0.7+) | Low (< 0.3) |
| `party` | Low skill (luck-friendly) | Moderate (< 0.4) |
| `bluffing` | Moderate skill | Low (< 0.3) |
| `trick-taking` | Moderate skill | Low (< 0.3) |

The `party` style **inverts** the skill penalty: games where skill dominates are penalized, because party games should be accessible to casual players.

## Performance

### Speed

| Evaluation Type | Games | MCTS Iters | Time (per genome) |
|-----------------|-------|------------|-------------------|
| Fast (in-evolution) | 10 | 100 | ~100ms |
| Standard | 100 | 100 | ~1s |
| Thorough | 100 | 500 | ~5s |
| Research | 500 | 1000 | ~30s |

### Parallel Execution

Skill evaluation runs in parallel across genomes:

```python
from darwindeck.evolution.skill_evaluation import evaluate_batch_skill

results = evaluate_batch_skill(
    genomes=genome_list,
    num_games=100,
    mcts_iterations=500,
    num_workers=8  # Parallel processes
)
```

## Example Output

```
Generation 50 skill evaluation:
  Evaluating top 10% (10 genomes)...
  CrimsonDragon: skill=0.78, FPA=+0.12 (OK)
  AzurePhoenix: skill=0.65, FPA=+0.35 (FPA penalty applied)
  GoldenSerpent: skill=0.52, FPA=-0.08 (skill penalty applied)
  ...
  Skill eval complete: 4 penalties applied (2 FPA, 2 low-skill)
```

## Interpreting Results

### Good Skill Profile
```
Greedy win rate: 65%
MCTS win rate: 75%
First player advantage: +5%
```
- Greedy > 50%: Basic strategy helps
- MCTS > Greedy: Deeper thinking helps more
- Low FPA: Balanced turn order

### Problematic: Pure Luck
```
Greedy win rate: 51%
MCTS win rate: 52%
First player advantage: +2%
```
- Both ~50%: No skill advantage
- This is essentially a coin flip with extra steps

### Problematic: High FPA
```
Greedy win rate: 70%
MCTS win rate: 82%
First player advantage: +45%
```
- Good skill component, but...
- P0 wins 45% more than P1 - severely unbalanced

## API Reference

### SkillEvalResult

```python
@dataclass
class SkillEvalResult:
    genome_id: str
    greedy_wins_as_p0: int
    greedy_wins_as_p1: int
    greedy_win_rate: float      # 0.0-1.0
    mcts_wins_as_p0: int
    mcts_wins_as_p1: int
    mcts_win_rate: float        # 0.0-1.0
    total_games: int
    skill_score: float          # Combined metric (0.0-1.0)
    first_player_advantage: float  # -1.0 to +1.0
    timed_out: bool
```

### Functions

```python
# Single genome evaluation
from darwindeck.evolution.skill_evaluation import evaluate_skill

result = evaluate_skill(
    genome=my_genome,
    num_games=100,          # Games per tier (total = 2x)
    mcts_iterations=500,    # MCTS depth
    timeout_sec=60.0        # Max evaluation time
)

# Batch evaluation (parallel)
from darwindeck.evolution.skill_evaluation import evaluate_batch_skill

results = evaluate_batch_skill(
    genomes=[g1, g2, g3],
    num_games=100,
    mcts_iterations=500,
    num_workers=4           # Parallel processes
)
```

## See Also

- **Fitness Metrics**: `docs/fitness-metrics.md` (TODO)
- **MCTS Implementation**: `src/gosim/mcts/`
- **AI Players**: `src/gosim/engine/ai.go`
