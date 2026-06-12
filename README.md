# DarwinDeck

**Evolutionary card game discovery using genetic algorithms and quality-diversity search**

DarwinDeck evolves novel, playable card games for a standard 52-card deck. Games are built from three constrained skeleton templates (shedding, trick-taking, rummy) and scored on five fitness metrics measuring "fun." The system uses fitness sharing and within-skeleton novelty search to discover diverse, distinct game designs rather than converging on a single optimum.

## Quick Start

```bash
# Build
make build-v2

# Evolve a population of card games (default: hybrid algorithm)
./bin/darwindeck evolve -population 500 -generations 100 -workers 256

# Try a different algorithm
./bin/darwindeck evolve -algorithm baseline   # fitness sharing only
./bin/darwindeck evolve -algorithm hybrid     # novelty + fitness sharing (default)
./bin/darwindeck evolve -algorithm mapelites  # quality-diversity archive

# Play an evolved game against AI
./bin/darwindeck playtest output/<run>/games/rank01_*/genome.json --difficulty greedy

# Show genome details
./bin/darwindeck describe output/<run>/games/rank01_*/genome.json

# Run comparison experiments across algorithms
./bin/darwindeck experiment -configs baseline,hybrid -seeds 10
```

## How It Works

### Skeletons

Games are built from three skeleton templates that guarantee mechanical playability:

- **Shedding** (Crazy Eights, Mau-Mau): Match suit/rank to discard pile, first to empty hand wins
- **Trick-taking** (Whist, Hearts, Spades): One card per player per trick, highest card wins
- **Rummy** (Gin Rummy, Knock Rummy): Draw-meld-discard, lowest deadwood wins

The skeleton handles core game flow. Genomes encode parameters: hand size, player count, trump rules, special cards, scoring, win conditions. Cross-skeleton mechanic borrowing lets a shedding game include rummy-style meld bonuses, etc.

### Fitness Function

Five metrics, each normalized to [0, 1], weighted into a total fitness score:

| Metric | Weight | Measures |
|--------|--------|----------|
| Meaningful Decisions | 0.25 | Plays vs forced draws |
| Game Arc | 0.25 | Win distribution entropy + turn variance |
| Interaction | 0.20 | How much player actions affect each other |
| Skill Gradient | 0.20 | Greedy AI win rate vs random baseline |
| Session Length | 0.10 | Target 15-40 turns |

### Validation Pipeline

- **Tier 0** (free): Static analysis on genome struct
- **Tier 1** (5 games): Random AI smoke test, kill if hangs/degenerate
- **Tier 2** (250 games): Full evaluation with 200 random + 50 greedy games

### Algorithms

Three evolution strategies are available:

- **Baseline**: Fitness sharing by skeleton type. Linear division by niche population, with boost for underrepresented niches.
- **Hybrid (default)**: Within-skeleton novelty search (k-NN behavioral distance) combined with fitness sharing. Best diversity-to-quality ratio.
- **MAP-Elites**: 10x10 behavioral grid per skeleton, axes are AvgTurns x WinEntropy. Maintains best genome per cell.

The hybrid is the default because experimental comparison showed it produces ~2x the behavioral coverage of baseline at minimal fitness cost.

## Performance

Benchmark numbers on a 256-core EPYC:

| Configuration | Wall Time |
|---------------|-----------|
| 500 pop, 100 gens (default) | ~2 minutes |
| 2000 pop, 200 gens | ~15 minutes |
| 45-run experiment (3 algos x 15 seeds) | ~80 minutes |

Each Tier 2 evaluation runs 250 game simulations. With sub-millisecond per-game cost, throughput exceeds 100k games/second per worker pool.

## Diversity Experiments

> **Caveat (2026-06-11):** every number in this section comes from April 2026 runs that predate the April-June 2026 fitness-metric fixes. The experiments will be re-run on fixed code and these tables regenerated (see `docs/plans/2026-06-11-audit-remediation.md`, Phase 7), at which point this caveat is removed.

Experimental comparison of evolution algorithms (10 seeds each, 2000 pop, 200 gens):

| Metric | Baseline | Hybrid |
|--------|----------|--------|
| Coverage (qualified cells filled) | 0.117 | **0.247** |
| QD-Score (sum of fitness in cells) | 29.6 | **60.6** |
| Pairwise Behavioral Distance | 0.275 | **0.524** |
| Median Fitness | 0.832 | 0.771 |
| Games Produced | 1747 | 2107 |

Hybrid doubles coverage and pairwise distance with a small (-7%) median fitness tradeoff.

Per-skeleton coverage shows hybrid lifts all skeleton types:
- Shedding: 0.090 -> 0.220 (+144%)
- Trick-taking: 0.110 -> 0.300 (+173%)
- Rummy: 0.150 -> 0.230 (+53%)

MAP-Elites was compared in a separate 15-seed run at a smaller evaluation budget (`results/pre-fix-experiments/full/results.json`), so its absolute numbers are not comparable to the table above; it is shown against its own in-run baseline (mean over 15 seeds):

| Metric | Baseline | MAP-Elites |
|--------|----------|------------|
| Coverage (qualified cells filled) | 0.090 | 0.087 |
| QD-Score (sum of fitness in cells) | 22.1 | 22.2 |
| Pairwise Behavioral Distance | 0.220 | **0.348** |
| Median Fitness | 0.800 | **0.862** |
| Games Produced | 436 | 79 |

MAP-Elites matched baseline coverage and QD-score with ~5x fewer output games and the highest median fitness of any algorithm in either run (0.862) -- it concentrates quality into a small, diverse archive rather than a large output pool.

See `docs/plans/2026-04-11-diversity-experiments-design.md` for the full experimental design.

## Architecture

```
cmd/darwindeck/         CLI entry point (evolve, experiment, playtest, describe)
pkg/
├── genome/             Genome struct, skeleton params, static validation
├── skeleton/
│   ├── shedding/       Shedding runner (match suit/rank, special cards)
│   ├── tricktaking/    Trick-taking runner (suit following, trump, tricks)
│   └── rummy/          Rummy runner (draw-meld-discard, knock/gin)
├── sim/                Card types, GameState, AI players, batch runner
├── mechanic/           Borrowed mechanics hook system
├── evolution/          Mutation, crossover, selection, fitness sharing
│   ├── engine.go       Baseline engine with fitness sharing
│   ├── novelty.go      Hybrid: within-skeleton novelty + fitness sharing
│   ├── mapelites.go    Quality-diversity archive engine
│   └── behavior.go     Behavior descriptor (AvgTurns x WinEntropy)
├── fitness/            5 fitness metrics, tiered evaluation pipeline
├── output/             Rulebook generation, JSON output
├── playtest/           Interactive playtest session
└── seeds/              8 seed game definitions across 3 skeletons
```

## Seed Games

| Skeleton | Seeds |
|----------|-------|
| Shedding | Crazy Eights, Mau-Mau |
| Trick-taking | Whist, Hearts, Spades, Oh Hell |
| Rummy | Gin Rummy, Knock Rummy |

Seeds initialize the population. Mutation, crossover, and cross-skeleton mechanic borrowing produce the search space.

## Development

```bash
# Build
make build-v2

# Run tests
go test ./pkg/... -v

# Single test
go test ./pkg/evolution/ -run TestSmallEvolution -v

# Quick evolution (for iteration)
./bin/darwindeck evolve -population 30 -generations 5 -verbose
```

## Project Layout

- `cmd/darwindeck/` — CLI entry point
- `pkg/` — Pure Go library code (no external dependencies beyond stdlib + math/rand/v2)
- `docs/plans/` — Design documents for major features
- `output/` — Evolution run outputs (gitignored)

## License

MIT

## Citation

```bibtex
@software{darwindeck2026,
  title = {DarwinDeck: Evolutionary Card Game Discovery via Quality-Diversity Search},
  author = {Gabriel Ortiz},
  year = {2026},
  url = {https://github.com/signalnine/darwindeck}
}
```
