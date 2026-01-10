# DarwinDeck 🧬🃏

**Evolutionary card game system using genetic algorithms to create novel, playable card games**

DarwinDeck uses evolutionary computation to automatically design card games playable with a standard 52-card deck. The system optimizes for configurable parameters like complexity, session length, skill/luck ratio, and player count.

## Overview

DarwinDeck combines:
- **Genetic algorithms** for game rule evolution
- **Monte Carlo simulation** for fitness evaluation
- **Multi-core parallelization** for massive performance (256+ cores supported)
- **11 seed games** from Hoyle's Encyclopedia as starting population

## Features

- 🧬 **Evolutionary Design**: Genetic operators (mutation, crossover, selection) evolve game rules
- 🎯 **Fitness Evaluation**: Measures fun proxies (decision density, comeback potential, tension curve)
- ⚡ **Parallel Execution**: 360x speedup on 256-core systems
- 🎮 **11 Seed Games**: War, Hearts, Crazy 8s, Gin Rummy, Old Maid, Go Fish, and more
- 🚀 **High Throughput**: 800,000+ games/second on large servers
- 📊 **Comprehensive Testing**: 100+ unit/integration tests

## Performance

### Single Machine (4 cores)
- **Throughput:** 3,000-4,000 games/second
- **Population (100 genomes):** ~25-30 seconds per generation
- **Full evolution (100 gen):** ~42-50 minutes

### Server (256 cores)
- **Throughput:** 800,000+ games/second
- **Population (500 genomes):** ~1-2 seconds per generation
- **Full evolution (100 gen):** ~2-3 minutes ⚡

## Quick Start

```bash
# Install dependencies
poetry install

# Run evolution locally
poetry run python -m darwindeck.cli.evolve \
    --population-size 100 \
    --generations 50 \
    --output-dir output/run1

# Deploy to 256-core server
./scripts/deploy-to-server.sh
ssh your-server "cd darwindeck && ./scripts/run-evolution.sh"
```

## Architecture

```
darwindeck/
├── genome/          # Game rule representation (DSL)
├── simulation/      # Python simulation engine
├── evolution/       # Genetic algorithm engine
├── cli/             # Command-line interface
└── bindings/        # Python/Go bridge (FlatBuffers)

gosim/               # High-performance Go simulator
├── simulation/      # Monte Carlo runner (parallel)
├── mcts/            # MCTS AI player
└── cgo/             # C bridge to Python
```

## Parallelization

DarwinDeck implements two-level parallelization:

1. **Python-level:** Evaluates multiple genomes in parallel (256x on large servers)
2. **Go-level:** Each genome's simulations run in parallel goroutines (1.43x)
3. **Combined:** ~360x speedup vs serial execution

See `docs/parallelization-strategy.md` for details.

## Example Games

DarwinDeck includes 11 seed games from Hoyle's Encyclopedia:

- **War**: Pure luck baseline
- **Hearts**: Trick-taking with suit breaking
- **Crazy 8s**: Shedding with wildcards
- **Gin Rummy**: Set collection and melds
- **Old Maid**: Pairing and avoidance
- **Go Fish**: Book collection
- **Scotch Whist**: Trump-based trick-taking
- **Draw Poker**: Hand improvement
- **Scopa**: Italian capturing game
- **I Doubt It**: Bluffing (simplified)
- **Betting War**: War variant

## Documentation

- **`CLAUDE.md`**: Project overview and architecture
- **`docs/plans/`**: Phase-by-phase implementation plans
- **`docs/parallelization-strategy.md`**: Multi-core optimization strategy
- **`docs/deployment/server-evolution-guide.md`**: Running on large servers
- **`docs/benchmarks/`**: Performance analysis and results

## Testing

```bash
# Run all tests
poetry run pytest

# Run specific test suites
poetry run pytest tests/unit/              # Unit tests
poetry run pytest tests/integration/       # Integration tests
poetry run pytest tests/property/          # Property-based tests

# Run Go tests
cd src/gosim && go test ./...

# Run Go benchmarks
cd src/gosim/simulation && go test -bench=. -benchmem
```

## Development

```bash
# Format code
poetry run black src/ tests/

# Type checking
poetry run mypy src/

# Build Go simulator
cd src/gosim && make

# Run evolution locally
poetry run python -m darwindeck.cli.evolve --verbose
```

## License

MIT

## Citation

If you use DarwinDeck in research, please cite:

```bibtex
@software{darwindeck2026,
  title = {DarwinDeck: Evolutionary Card Game System},
  author = {Gabe},
  year = {2026},
  url = {https://github.com/signalnine/darwindeck}
}
```

## Acknowledgments

- Game examples adapted from **Hoyle's Encyclopedia of Card Games**
- Evolutionary algorithms inspired by genetic programming research
- Parallelization strategy optimized for modern multi-core systems
- Built with Claude Code assistance
