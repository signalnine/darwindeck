# Analysis: 0.8403 Fitness Ceiling

**Date:** 2026-01-10
**Evolved Games:** output/evolution-20260110-145814/

## Key Finding

All top 20 genomes converged to exactly **0.8403 fitness**, representing a new local optimum after our improved heuristics.

## Common Patterns (All 0.8403 Genomes)

### Structural Invariants
- ✅ **Trick-based:** `is_trick_based=True` (100%)
- ✅ **Phase count:** Exactly 5 phases (100%)
- ✅ **Cards per player:** 13 (100%)
- ✅ **Player count:** 4 players (100%)
- ✅ **Breaking suit:** Hearts (100%)
- ✅ **Turn limits:** max_turns=500, min_turns=52 (100%)
- ✅ **Special effects:** None (0 special effects) (100%)

### Phase Composition (Varied)
- **TrickPhase:** 1-2 per game (lead_suit_required=True, high_card_wins=True)
- **DrawPhase:** 0-2 per game (mostly optional, source varies: deck/discard)
- **DiscardPhase:** 0-3 per game (optional, count=1)
- **PlayPhase:** 0-1 per game (optional with conditions)

### Optional vs Mandatory
- Almost all phases are `mandatory=False` (optional)
- This maximizes `decision_density` score in our fitness function

### Win Conditions (Binary Split)
- **Group 1:** `first_to_score` threshold=100
- **Group 2:** `empty_hand` (no threshold)
- No games with complex scoring rules

## Sample Genomes

### Rank 1 (Simplest, Gen 6)
```
Phases: [DrawPhase(deck), DrawPhase(deck), TrickPhase, TrickPhase, DiscardPhase]
Win: first_to_score(100)
Complexity: Low (5 phases, 0 effects)
```

### Rank 10 (Medium, Gen 7)
```
Phases: [TrickPhase, DiscardPhase, DiscardPhase, TrickPhase, DiscardPhase]
Win: empty_hand
Initial discard: 1 card
Complexity: Medium (5 phases, 3 discards)
```

### Rank 18 (Most Complex, Gen 12)
```
Phases: [DiscardPhase, DrawPhase(deck), DrawPhase(discard), TrickPhase, PlayPhase]
Win: first_to_score(100)
Complexity: Higher (5 phases, mixed sources)
Ancestry: 65 crossovers deep!
```

## Fitness Component Analysis

### Why 0.8403 is the Ceiling

Given our improved heuristics in `fitness_full.py`, the 0.8403 score likely results from:

#### 1. Decision Density (~0.85)
```python
decision_density = min(1.0, (
    min(1.0, phase_count / 6.0) * 0.5 +      # 5 phases = 0.833
    min(1.0, optional_phases / 3.0) * 0.3 +  # 4-5 optional = 1.0
    min(1.0, has_conditions / 3.0) * 0.2     # 0-1 conditions = 0-0.33
))
# Result: ~0.75-0.92
```

#### 2. Comeback Potential (~0.80-0.95)
```python
# Win rate balance close to 50/50
comeback_potential = 1.0 - abs(win_rate_p0 - 0.5) * 2
# Result: 0.80-0.95 (well-balanced games)
```

#### 3. Tension Curve (~0.90-1.00)
```python
turn_score = min(1.0, avg_turns / 100.0)      # Long games
length_bonus = (avg_turns - 20) / 50.0        # 60+ turns
tension_curve = turn_score * 0.6 + length_bonus * 0.4
# Result: ~0.90-1.00 (games run long)
```

#### 4. Interaction Frequency (~0.50-0.70)
```python
special_effects_score = 0 / 3.0 = 0.0          # No effects
trick_based_score = 0.3                        # All are trick-based
multi_phase_score = 5 / 10.0 = 0.5            # Max contribution
interaction_frequency = 0.0*0.4 + 0.3 + 0.5 = 0.80
# Result: ~0.50-0.80
```

#### 5. Rules Complexity (~0.70-0.80)
```python
complexity = (
    5 phases +           # 5
    0 special_effects +  # 0
    0 scoring_rules +    # 0
    1 win_condition      # 1
) = 6
rules_complexity = 1.0 - (6 / 20.0) = 0.70
# Result: ~0.70
```

#### 6. Skill vs Luck (~0.75-0.85)
```python
length_factor = min(1.0, avg_turns / 80.0)     # ~0.75-1.0
balance_factor = comeback_potential            # ~0.80-0.95
complexity_factor = (5 + 0 + 1) / 8.0 = 0.75  # Phases + effects + trick
skill_vs_luck = 0.75*0.4 + 0.85*0.3 + 0.75*0.3 = ~0.78
# Result: ~0.75-0.85
```

### Weighted Total (Equal Weights)
```
0.85 * 0.167 = 0.142  (decision_density)
0.90 * 0.167 = 0.150  (comeback_potential)
0.95 * 0.167 = 0.159  (tension_curve)
0.65 * 0.167 = 0.109  (interaction_frequency)
0.70 * 0.167 = 0.117  (rules_complexity)
0.78 * 0.167 = 0.130  (skill_vs_luck)
──────────────────────
Total:         0.807

With variance: 0.80-0.85 range
Observed:      0.8403
```

## Why Evolution Stopped Here

### Bottleneck 1: Interaction Frequency
- **Current cap:** ~0.65-0.80 (limited by lack of special effects)
- **Problem:** Evolution eliminated special effects (they increase rules_complexity penalty)
- **Trade-off:** `interaction_frequency` ↑ but `rules_complexity` ↓
- **Net effect:** Special effects hurt total fitness

### Bottleneck 2: Rules Complexity Penalty
- **Current:** Simpler = better (inverse scoring)
- **Problem:** Adding special effects, scoring rules hurts this metric
- **Result:** Evolution converges to minimal games (5 phases, 0 effects, 1 win condition)

### Bottleneck 3: Skill vs Luck Estimator
- **Current:** Structural heuristic (length, balance, complexity)
- **Problem:** All 5-phase trick games look similar structurally
- **Result:** All score ~0.75-0.85, can't differentiate truly skillful games

### Bottleneck 4: Decision Density Cap
- **Current:** Capped at 5-6 phases
- **Problem:** Evolution found optimal phase count (~5), can't improve further
- **Result:** Saturated at ~0.85-0.90

## Breakthrough Strategies

To exceed 0.8403, we need to address the fitness function bottlenecks:

### Strategy 1: Reweight Metrics
Current equal weights (0.167 each) may not reflect game quality. Consider:
- **Decrease `rules_complexity` weight** (0.05-0.10) - currently punishing interesting mechanics
- **Increase `skill_vs_luck` weight** (0.25-0.30) - most important for game quality
- **Increase `interaction_frequency` weight** (0.20-0.25) - enables interesting gameplay

### Strategy 2: Fix Interaction Frequency
- **Problem:** Special effects hurt complexity but help interaction
- **Solution:** Separate "mechanical complexity" from "rules complexity"
  - Mechanical: Phase count, mandatory steps (simpler = better)
  - Rules: Special effects, conditions, interactions (more = better)

### Strategy 3: Real Instrumentation (Phase 1 Plan)
Implement actual measurement from gameplay:
- `decision_density`: Count real choices (not structural proxies)
- `interaction_frequency`: Track opponent-affecting actions
- `skill_vs_luck`: MCTS win rate differential

### Strategy 4: Add Diversity Pressure
Current elitism (10%) preserves converged optima. Consider:
- Novelty search component (reward structural differences)
- Speciation (maintain multiple fitness peaks)
- Explicit anti-convergence pressure

## Conclusion

**0.8403 represents a well-balanced, simple trick-taking game:**
- 5 phases (mostly optional)
- Trick-based with Hearts breaking
- 13 cards, 4 players
- No special effects (too costly in rules_complexity)
- Either race-to-100 or empty-hand victory

**To exceed this ceiling, we must:**
1. Reweight metrics (favor skill, interaction over simplicity)
2. Implement Phase 1 real instrumentation
3. Add diversity pressure to prevent convergence
4. Separate mechanical vs rules complexity

The improved heuristics successfully raised the ceiling from **0.7000 → 0.8403** (+20%), validating the approach. Further gains require actual gameplay measurement (Phase 1 consensus plan).
