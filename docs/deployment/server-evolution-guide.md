# Running Evolution on 256-Core Server (anarres)

Quick guide to deploy and run the genetic algorithm on your beefy server.

## Server Specs
- **Host:** 192.168.1.15 (anarres)
- **CPUs:** 256 cores
- **RAM:** 1TB
- **OS:** Linux 6.14.11-2-pve

## Expected Performance

With 256-core parallelization:
- **Python-level:** 256 workers evaluating genomes in parallel
- **Go-level:** 1.43x speedup per genome (goroutine worker pool)
- **Combined speedup:** ~360x vs serial execution
- **Throughput:** ~800,000+ games/second
- **Population of 500:** ~1-2 seconds per generation
- **100 generations:** ~2-3 minutes total

## Quick Start

### 1. Deploy from Local Machine

```bash
# From your local machine (/home/gabe/cards-playtest)
./scripts/deploy-to-server.sh
```

This will:
- rsync all code to the server
- Transfer the compiled Go simulator (libcardsim.so)
- Exclude build artifacts and caches

### 2. Setup Server Environment

```bash
# SSH to the server
ssh 192.168.1.15

# Navigate to project directory
cd /home/gabe/cards-playtest

# Run setup script
./scripts/setup-server.sh
```

This will:
- Install uv if needed
- Install Python dependencies
- Verify Go simulator is present

### 3. Run Evolution

```bash
# Basic run with defaults
./scripts/run-evolution.sh

# Custom parameters
POPULATION=1000 GENERATIONS=200 ./scripts/run-evolution.sh

# Or run directly with more control
uv run python -m darwindeck.cli.evolve \
    --population-size 500 \
    --generations 100 \
    --output-dir output/my-run \
    --save-top-n 20 \
    --verbose
```

### Evolution Parameters

- `--population-size` / `-p`: Population size (default: 100, recommended for server: 500-1000)
- `--generations` / `-g`: Max generations (default: 100)
- `--elitism-rate` / `-e`: Top % preserved (default: 0.1)
- `--crossover-rate` / `-c`: Crossover probability (default: 0.7)
- `--tournament-size` / `-t`: Tournament selection size (default: 3)
- `--plateau-threshold`: Generations without improvement before stopping (default: 30)
- `--seed-ratio`: Ratio of known games to mutants (default: 0.7)
- `--output-dir` / `-o`: Output directory (default: output)
- `--save-top-n`: Number of top genomes to save (default: 10)
- `--style` / `-s`: Fitness style preset (default: balanced)
- `--verbose` / `-v`: Enable verbose logging

### Fitness Styles

Different style presets weight fitness metrics differently:

| Style | Description | Best For |
|-------|-------------|----------|
| `balanced` | Default, well-rounded games | General exploration |
| `bluffing` | Hidden info, betting, interaction | Poker-like games |
| `strategic` | Deep thinking, skill emphasis | Chess-like games |
| `party` | Quick, accessible, high luck | Casual/family games |
| `trick-taking` | Trick-based mechanics | Bridge/Hearts-like games |

```bash
# Run evolution with a specific style
STYLE=strategic ./scripts/run-evolution.sh

# Or via CLI
uv run python -m darwindeck.cli.evolve --style bluffing
```

### Environment Variables

- `EVOLUTION_WORKERS`: Override number of parallel workers (default: detect CPU count)
- `POPULATION`: Population size for run-evolution.sh
- `GENERATIONS`: Generations for run-evolution.sh
- `STYLE`: Fitness style preset (default: balanced)
- `OUTPUT_DIR`: Output directory for run-evolution.sh

## Monitoring

### Real-time Progress

The evolution engine logs:
- Generation number
- Best fitness
- Average fitness
- Diversity metrics
- Evaluation time

Example output:
```
🧬 Evolution Configuration:
  Population size: 500
  Generations: 100
  Workers: 256
  Output directory: output/evolution-20260110-143022

Evolution engine initialized with 256 parallel workers
Initializing population of size 500
Population initialized with 500 individuals
Evaluating 500 individuals using 256 workers...
Evaluation complete. Avg fitness: 0.542

Generation 1/100:
  Best fitness: 0.743
  Avg fitness: 0.542
  Diversity: 0.821
  Time: 1.8s
```

### Monitor CPU Usage

```bash
# SSH to server in another terminal
htop

# Or check CPU stats
mpstat -P ALL 1
```

You should see all 256 cores at ~100% utilization during fitness evaluation.

### Monitor Memory

```bash
free -h
```

Expected memory usage: ~5-10GB for population of 500

## Output

Results are saved to the output directory (default: `output/`):

```
output/evolution-TIMESTAMP/
├── genome_rank01_fitness0.8234.txt
├── genome_rank02_fitness0.7891.txt
├── genome_rank03_fitness0.7654.txt
...
└── genome_rank20_fitness0.6123.txt
```

Each file contains:
- Genome ID
- Fitness score
- Generation number
- Full genome specification

## Troubleshooting

### Evolution runs but no progress

Check that FitnessEvaluator is working:
```python
from darwindeck.evolution.fitness import FitnessEvaluator
from darwindeck.genome.examples import create_war_genome

evaluator = FitnessEvaluator()
result = evaluator.evaluate(create_war_genome())
print(f"Fitness: {result.total_fitness}")
```

### Workers not being used

Check environment variable:
```bash
echo $EVOLUTION_WORKERS
```

Should be 256 (or your desired worker count).

### Go simulator not found

```bash
ls -la libcardsim.so
```

If missing, rebuild on server or copy from local machine.

### Out of memory

Reduce population size:
```bash
POPULATION=100 ./scripts/run-evolution.sh
```

## Performance Tips

1. **Start small:** Test with population=100, generations=10 to verify setup
2. **Scale up:** Once confirmed working, increase to population=500-1000
3. **Monitor:** Watch CPU and memory usage for first few generations
4. **Tune:** Adjust population size based on available RAM
5. **Save frequently:** Use `--save-top-n 20` to keep best genomes

## Re-deployment

After making code changes locally:

```bash
# From local machine
./scripts/deploy-to-server.sh

# SSH to server
ssh 192.168.1.15
cd /home/gabe/cards-playtest

# Re-run evolution
./scripts/run-evolution.sh
```

No need to re-run setup unless dependencies changed.

## Advanced Usage

### Custom Fitness Evaluator

```python
from darwindeck.evolution.engine import EvolutionEngine, EvolutionConfig

def my_fitness_evaluator(individual):
    # Custom fitness logic
    return Individual(
        genome=individual.genome,
        fitness=custom_score,
        evaluated=True
    )

config = EvolutionConfig(population_size=500)
engine = EvolutionEngine(
    config,
    fitness_evaluator=my_fitness_evaluator,
    num_workers=256
)
engine.evolve()
```

### Parallel Go Simulator

The Go simulator automatically uses goroutines for parallel game execution within each worker. This is transparent to Python code.

Total parallelization:
- **Level 1 (Python):** 256 workers × 1 genome each
- **Level 2 (Go):** Each genome runs 100 games with 1.43x parallel speedup
- **Combined:** ~360x faster than serial execution

## Next Steps

- Analyze best genomes from output directory
- Playtest top-ranked games with humans
- Iterate on fitness function to optimize for fun
- Scale up to larger populations (1000+) if time allows
