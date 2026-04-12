# Diversity Experiments Design

**Date:** 2026-04-11
**Goal:** Determine which diversity mechanism produces the most behaviorally distinct playable card games.

## Experiment Overview

Three configurations tested:

| Config | Algorithm | Key Parameters |
|--------|-----------|---------------|
| A (Baseline) | Current fitness sharing | Skeleton niche, linear division, boost for underrepresented |
| B (MAP-Elites) | Quality-diversity archive | 10x10 grid, AvgTurns x WinEntropy, per skeleton |
| C (Novelty Search) | k-NN behavioral distance bonus | k=15, novelty as 6th fitness metric, 0.70 floor |

**Runs:** 15 seeds per config = 45 total. Each run: 500 pop, 100 gens.

**Pre-flight checks:**
- [x] Validate behavioral axes vary within each skeleton (Play/Draw Ratio failed; switched to WinEntropy)
- [ ] Confirm 0.70 floor is enforced inside each algorithm, not just at output
- [ ] Document all 45 seeds upfront

## MAP-Elites Implementation

Replace the population-based selection loop with an archive-based one. The archive is a 10x10 grid where each cell holds the single best genome for that behavioral niche.

**Behavioral descriptors (computed from BatchResult):**
- X-axis: AvgTurns -- normalized to [0, 1] by mapping range [5, 100]
- Y-axis: WinEntropy -- Shannon entropy of win distribution, normalized by max entropy (log2(numPlayers)), naturally [0, 1]

**Pre-flight validation (2026-04-11):** Play/Draw Ratio was degenerate (constant 1.0 for trick-taking, constant ~0.47 for rummy). WinEntropy has meaningful variance across all skeletons:
- Shedding: 0.90-1.00 (narrow but nonzero)
- Trick-taking: 0.50-0.96 (good spread -- some games are balanced, others favor one player)
- Rummy: 0.58-1.00 (excellent spread)

**Per-skeleton archives:** 3 independent 10x10 grids. A shedding genome only competes with other shedding genomes for cell placement.

**Algorithm per generation:**
1. Generate offspring via mutation/crossover from archive occupants (uniform random parent selection from occupied cells)
2. Evaluate offspring fitness (same Tier 0/1/2 pipeline)
3. If fitness >= 0.70, compute behavior descriptor, map to cell
4. If cell is empty OR offspring fitness > current occupant, replace
5. Track: cells filled, QD-score, best fitness per skeleton

**Key difference from current engine:** No tournament selection, no elitism, no population array. The archive IS the population. Generate offspring by picking random occupied cells and mutating their genomes.

**Output:** The full archive contents (up to 300 games across 3 skeletons), not just top-20.

## Novelty Search Implementation

Keep the existing population-based engine but add a 6th metric -- behavioral novelty -- that rewards genomes for being distant from their neighbors.

**Behavioral descriptor:** Same 2D vector as MAP-Elites: (AvgTurns_normalized, WinEntropy). Ensures fair comparison.

**Novelty score computation:**
1. After evaluating a genome, compute its 2D behavior vector
2. Compute mean Euclidean distance to its k=15 nearest neighbors in the current population
3. Normalize to [0, 1] by dividing by the max observed distance in that generation
4. This becomes the Novelty metric

**Fitness integration:**
```
SharedFitness = (0.5 * TotalFitness) + (0.5 * Novelty)
```
Equal weight -- strong diversity pressure. The 0.70 fitness floor is enforced as a hard gate: genomes below 0.70 raw fitness get SharedFitness = 0 regardless of novelty.

**Novelty archive:** Maintain a secondary archive of behaviorally novel individuals encountered during the run. When a genome's novelty exceeds a threshold, add it to the archive. The archive contributes to k-NN distance calculations, providing a "memory" of where the search has been.

**Key difference from MAP-Elites:** Novelty Search still uses tournament selection and populations. It incentivizes diversity rather than structurally guaranteeing it. The output is the final population + novelty archive contents.

## Metrics and Evaluation

**Primary metrics (used for decision-making):**

| Metric | Definition | What it measures |
|--------|-----------|-----------------|
| Qualified Coverage | Occupied cells with fitness >= 0.70 / total cells | How much of the behavioral space was explored |
| QD-Score | Sum of fitness values in qualified cells | Quality AND diversity combined |
| Mean Pairwise Distance | Average Euclidean distance between all qualified outputs in 2D behavior space | How spread out the games actually are |

**For fair comparison across algorithms:**
- Baseline and Novelty Search: compute the behavior vector for each output genome, place into a virtual 10x10 grid, then measure coverage/QD-score as if it were MAP-Elites
- This gives apples-to-apples comparison even though only MAP-Elites uses the grid during evolution

**Secondary metrics (for interpretation):**
- Median fitness of qualified outputs
- Total unique qualified games produced
- Per-skeleton breakdown of all primary metrics

**Evaluation protocol:**
1. Each run produces all qualified genomes (fitness >= 0.70)
2. Compute behavior vector for each
3. Place into virtual grid, compute coverage and QD-score
4. Compute pairwise distances
5. Report per-skeleton, then aggregate across skeletons

**Statistical comparison:**
- Mann-Whitney U test between configurations (nonparametric, no normality assumption)
- Report median +/- IQR across 15 seeds
- Effect size (rank-biserial correlation)

## Execution Plan

**Phase 0: Pre-flight (1 run, ~2 min)**
- Run current baseline once
- Extract AvgTurns and Play/Draw Ratio histograms per skeleton
- Verify trick-taking has meaningful variance on Play/Draw Ratio (not all clustered at 1.0)
- If degenerate: substitute Special Event Frequency for that skeleton's Y-axis

**Phase 1: Main experiment (45 runs, ~6 min parallel)**
- Seeds: 1-15 for baseline, 16-30 for MAP-Elites, 31-45 for Novelty Search
- All runs: 500 pop equivalent budget, 100 gens
- Write results to output/experiments/diversity-{config}-seed{N}/
- Each output includes: all qualified genomes, their behavior vectors, fitness metrics

**Phase 2: Analysis (automated script)**
- Compute coverage, QD-score, pairwise distance for each run
- Generate comparison table: median +/- IQR per metric per config
- Mann-Whitney U tests: baseline vs MAP-Elites, baseline vs Novelty, MAP-Elites vs Novelty
- Per-skeleton breakdown
- Output: docs/experiments/diversity-results.md

**Phase 3: Decision**
- If one method clearly wins on 2+ primary metrics: adopt it
- If metrics conflict: report tradeoff, pick based on which games look more interesting on manual inspection
- If neither beats baseline: investigate whether axes are the problem (try different descriptors)

**Timeline:** Total wall-clock ~30 min (pre-flight + parallel runs + analysis).
