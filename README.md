# DarwinDeck 🧬🃏

**Evolutionary card game system using genetic algorithms to create novel, playable card games**

DarwinDeck uses evolutionary computation to automatically design card games playable with a standard 52-card deck. The system optimizes for configurable parameters like complexity, session length, skill/luck ratio, and player count.

## Overview

DarwinDeck combines:
- **Genetic algorithms** for game rule evolution
- **Monte Carlo simulation** for fitness evaluation
- **Multi-core parallelization** for massive performance (256+ cores supported)
- **16 seed games** from Hoyle's Encyclopedia as starting population

## Features

- 🧬 **Evolutionary Design**: Genetic operators (mutation, crossover, selection) evolve game rules
- 🎯 **Fitness Evaluation**: Measures fun proxies (decision density, comeback potential, tension curve)
- 🎲 **Two-Tier Skill Evaluation**: Greedy vs Random + MCTS vs Random measures skill ceiling
- ⚖️ **First-Player Advantage Detection**: Filters out unbalanced games (>30% FPA)
- 🔄 **In-Evolution Skill Penalties**: Penalizes unfit games during breeding, not just at the end
- ⚡ **Parallel Execution**: 360x speedup on 256-core systems
- 🎮 **18 Seed Games**: Including 4 betting/poker variants for evolution
- 🚀 **High Throughput**: 800,000+ games/second on large servers
- 📝 **LLM Descriptions**: Auto-generated game summaries using Claude
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
# Install dependencies (using uv - recommended)
uv sync

# Run evolution locally
uv run python -m darwindeck.cli.evolve \
    --population-size 100 \
    --generations 50 \
    --output-dir output/run1

# Run with fitness style preset
uv run python -m darwindeck.cli.evolve --style strategic

# Adjust skill evaluation during evolution
uv run python -m darwindeck.cli.evolve \
    --skill-eval-frequency 10 \
    --fpa-penalty-threshold 0.3

# Deploy to 256-core server
./scripts/deploy-to-server.sh
ssh your-server "cd darwindeck && ./scripts/run-evolution.sh"
```

## Generating Game Descriptions

After evolution, generate human-readable game rules from saved genomes:

```bash
# Generate description from saved genome JSON
uv run python -m darwindeck.cli.describe output/run1/rank01_GameName.json

# With verbose output showing fitness and skill metrics
uv run python -m darwindeck.cli.describe output/run1/rank01_GameName.json -v

# Override fitness score (if not in file)
uv run python -m darwindeck.cli.describe output/run1/genome.json --fitness 0.85
```

The describe command uses Claude to generate natural language game rules suitable for human playtesting. It extracts:
- Game setup and deal rules
- Turn structure and valid moves
- Win conditions and scoring
- Skill evaluation summary (if available)

**Note:** Requires `ANTHROPIC_API_KEY` environment variable to be set.

## Generating Rulebooks

Generate comprehensive rulebooks in markdown format:

```bash
# Generate rulebook for a single genome
uv run python -m darwindeck.cli.rulebook output/run1/rank01_GameName.json

# Save to specific output directory
uv run python -m darwindeck.cli.rulebook output/run1/rank01_GameName.json -o output/rulebooks/

# Generate rulebooks for all genomes in an evolution output
uv run python -m darwindeck.cli.rulebook output/evolution-run/ -o output/rulebooks/
```

Rulebooks include:
- Complete setup instructions
- Turn structure with all phases
- Special rules and card effects
- Edge case handling (empty deck, no valid plays, ties)
- Win conditions

**Note:** Requires `ANTHROPIC_API_KEY` environment variable to be set.

## Interactive Playtesting

Play evolved games against AI opponents:

```bash
# Play against greedy AI
uv run python -m darwindeck.cli.playtest output/run1/rank01_GameName.json --difficulty greedy

# Play against MCTS AI (stronger)
uv run python -m darwindeck.cli.playtest output/run1/rank01_GameName.json --difficulty mcts

# Use a specific seed for reproducibility
uv run python -m darwindeck.cli.playtest output/run1/rank01_GameName.json --seed 12345

# Choose which player slot you play
uv run python -m darwindeck.cli.playtest output/run1/rank01_GameName.json --human-player 1
```

During playtest:
- See your hand and game state
- Select moves from legal options
- Rate the game after completion (1-5 stars)
- Results saved to `playtest_results.jsonl`

AI difficulty levels:
- `random`: Random legal moves (easiest)
- `greedy`: Heuristic-based play (medium)
- `mcts`: Monte Carlo tree search (hardest)

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

## Fitness Metrics

Games are evaluated on multiple dimensions:

| Metric | Description |
|--------|-------------|
| **Decision Density** | Ratio of meaningful choices to total actions |
| **Comeback Potential** | Can trailing players recover? |
| **Tension Curve** | Uncertainty over game progression |
| **Interaction Frequency** | How much do players affect each other? |
| **Rules Complexity** | Reasonable rule count for learning |
| **Session Length** | Target game duration in turns |
| **Bluffing Depth** | Deception and hidden information |

### Skill Evaluation

Two-tier evaluation measures skill vs luck:

1. **Greedy vs Random**: Does basic strategy help?
2. **MCTS vs Random**: What's the skill ceiling?

Games are penalized for:
- **High First-Player Advantage** (>30%): Unbalanced turn order
- **Low Skill Score** (<0.6): Too luck-dependent

### Fitness Styles

Presets optimize for different game types:

- `balanced`: General-purpose (default)
- `strategic`: Deep decision-making
- `bluffing`: Hidden information and deception
- `party`: Quick, accessible games
- `trick-taking`: Traditional card game mechanics

## Example Games

DarwinDeck includes 18 seed games from Hoyle's Encyclopedia and classic card game collections:

| Game | Category | Players | Key Mechanic |
|------|----------|---------|--------------|
| **War** | Luck | 2 | Pure luck baseline - card comparison |
| **Hearts** | Trick-taking | 4 | Avoid penalty cards (hearts, Queen of Spades) |
| **Spades** | Trick-taking | 4 | Fixed trump suit (spades always trump) |
| **Scotch Whist** | Trick-taking | 2-4 | Trump-based trick capture |
| **Knock-Out Whist** | Trick-taking | 2-4 | Elimination - lose a life each round |
| **Crazy 8s** | Shedding | 2-4 | Match suit/rank, 8s are wild |
| **Old Maid** | Shedding | 2-4 | Pair matching, avoid the odd card |
| **President** | Climbing | 4 | Beat previous play, 2 is highest |
| **Fan Tan** | Sequencing | 2-4 | Build sequences from 7s outward |
| **Gin Rummy** | Set Collection | 2 | Form melds (sets and runs) |
| **Go Fish** | Set Collection | 2-4 | Ask for cards, collect books |
| **Cheat** | Bluffing | 2-4 | Claim cards, challenge lies |
| **Scopa** | Capturing | 2 | Capture cards summing to target |
| **Uno** | Shedding | 2-4 | Special effects on cards (skip, reverse, draw) |

### Betting Games

These games use the betting/wagering system with chip management:

| Game | Starting Chips | Min Bet | Description |
|------|---------------|---------|-------------|
| **Simple Poker** | 1000 | 10 | 5-card showdown with one betting round |
| **Draw Poker** | 1000 | 20 | 5-card draw with betting |
| **Betting War** | 500 | 10 | War variant with wagering |
| **Blackjack** | 500 | 25 | Beat the dealer to 21 |

Betting games support: CHECK, BET, CALL, RAISE, ALL-IN, and FOLD actions with pot management and showdown resolution.

## Documentation

- **`CLAUDE.md`**: Project overview and architecture
- **`docs/plans/`**: Phase-by-phase implementation plans
- **`docs/parallelization-strategy.md`**: Multi-core optimization strategy
- **`docs/deployment/server-evolution-guide.md`**: Running on large servers
- **`docs/benchmarks/`**: Performance analysis and results

## Testing

```bash
# Run all tests
uv run pytest

# Run specific test suites
uv run pytest tests/unit/              # Unit tests
uv run pytest tests/integration/       # Integration tests
uv run pytest tests/property/          # Property-based tests

# Run Go tests
cd src/gosim && go test ./...

# Run Go benchmarks
cd src/gosim/simulation && go test -bench=. -benchmem
```

## Development

```bash
# Format code
uv run black src/ tests/

# Type checking
uv run mypy src/

# Build Go simulator (CGo shared library)
make build-cgo

# Run evolution locally
uv run python -m darwindeck.cli.evolve --verbose
```

## License

MIT

## Citation

If you use DarwinDeck in research, please cite:

```bibtex
@software{darwindeck2026,
  title = {DarwinDeck: Evolutionary Card Game System},
  author = {Gabriel Ortiz},
  year = {2026},
  url = {https://github.com/signalnine/darwindeck}
}
```

## Acknowledgments

- Game examples adapted from **Hoyle's Encyclopedia of Card Games**
- Evolutionary algorithms inspired by genetic programming research
- Parallelization strategy optimized for modern multi-core systems
- Built with Claude Code assistance
