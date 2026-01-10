# Evolutionary Card Game System - Design Document

**Date:** 2026-01-10
**Status:** Approved
**Target:** Production-ready system for evolving novel card games

## Project Goals

Build a production-ready system that uses genetic algorithms and Monte Carlo simulations to evolve novel card games playable with a standard 52-card deck. The system will:

- Optimize for configurable parameters (complexity, session length, skill/luck ratio, player count)
- Support medium-scale simulations (thousands of games per generation, multi-core optimization)
- Provide CLI interface for evolution runs and analysis
- Track full experiment history for reproducibility and analysis
- Generate human-readable rules via LLM for playtesting

## Technology Stack

- **Language:** Python 3.11+
- **Key Libraries:** numpy, pydantic, click, anthropic/openai, pytest, hypothesis
- **Architecture:** Layered with plugin system
- **Scale:** Medium (multi-core, hours per run, thousands of games per generation)

---

## High-Level Architecture

The system is organized into four primary layers:

### 1. Genome Layer
- `GameGenome`: Structured data representation of a card game (setup, turn structure, win conditions, scoring)
- `GenomeGenerator`: Creates initial random genomes with configurable constraints
- `GenomeInterpreter`: Converts genome data structures into executable Python game logic

### 2. Simulation Layer
- `GameState`: Immutable representation of game state (deck, hands, tableau, scores)
- `GameEngine`: Executes games given a genome and AI players
- `AIPlayer` interface with implementations: `RandomPlayer`, `GreedyPlayer`, `MCTSPlayer` (with variants: weak/medium/strong)

### 3. Evolution Layer
- `GeneticOperators`: Mutation, crossover, selection strategies (pluggable)
- `FitnessEvaluator`: Computes fitness metrics from simulation results
- `FitnessProfile`: Predefined weight configurations ("quick-party", "strategic-depth", "balanced")
- `EvolutionEngine`: Orchestrates the evolutionary loop with population management

### 4. CLI/Output Layer
- `CommandInterface`: Argument parsing, run configuration
- `ExperimentTracker`: Saves generations, metrics, best games to disk
- `RuleGenerator`: Uses LLM API to convert genomes to human-readable rules

Each layer has clear interfaces, making components swappable. For example, you can add new AI players without touching the genome or evolution logic.

---

## Genome Structure

The `GameGenome` dataclass defines game rules through structured fields, generates Python code at runtime, and remains data for genetic operations.

### Core Genome Fields

```python
@dataclass
class GameGenome:
    # Setup Phase
    setup: SetupRules          # cards per player, tableau config

    # Turn Structure
    turn_phases: List[Phase]   # ordered: draw, play, discard phases

    # Valid Moves
    move_rules: MoveRules      # what cards can be played where

    # Special Cards
    special_effects: Dict[Rank, Effect]  # 8=wild, 2=draw2, etc

    # Win/Lose Conditions
    win_conditions: List[WinCondition]

    # Scoring
    scoring: ScoringRules

    # Constraints
    max_turns: int            # force termination

    # Metadata
    genome_id: str
    generation: int
```

### Field Structure Examples

Each field is itself structured:

- `SetupRules` has `cards_per_player: int`, `initial_tableau: TableauConfig`, `deck_config: DeckConfig`
- `Phase` can be `DrawPhase(source, count)`, `PlayPhase(target, rules)`, `PassPhase(conditions)`
- `Effect` can be `SkipNext`, `DrawCards(n)`, `ReverseOrder`, `ChooseSuit`, etc.

### Genetic Operations

**Mutations** change field values:
- Increment `cards_per_player` by 1
- Add a random special card effect
- Change win condition from "first empty hand" to "highest score at turn limit"
- Modify move rules (suit match → rank match)

**Crossover** swaps entire sub-structures between parents (e.g., setup from parent A, win conditions from parent B).

### Python Generation

`GenomeInterpreter.to_executable()` takes the genome and produces a `GameLogic` object with methods like `is_valid_move()`, `apply_move()`, `check_win()`.

---

## Simulation Engine

The simulation layer executes games and collects metrics. Key design principle: a random seed makes games fully deterministic, enabling reproducibility.

### GameEngine Flow

```python
class GameEngine:
    def simulate_game(
        self,
        genome: GameGenome,
        players: List[AIPlayer],
        seed: int
    ) -> GameResult:
        # 1. Initialize from genome
        logic = GenomeInterpreter.to_executable(genome)
        state = logic.create_initial_state(seed)

        # 2. Run turn loop
        history = []
        while not logic.is_terminal(state):
            player = players[state.current_player]
            action = player.choose_action(state, logic)
            state = logic.apply_action(state, action)
            history.append((state.copy(), action))

            if len(history) >= genome.max_turns:
                break

        # 3. Collect metrics
        return GameResult(
            winner=logic.get_winner(state),
            turn_count=len(history),
            decision_points=self._count_decisions(history),
            comeback_events=self._analyze_comebacks(history),
            tension_curve=self._compute_tension(history),
            ...
        )
```

### Key Design Decisions

**Immutable State:** `GameState` is immutable (frozen dataclass), making history tracking and MCTS tree search clean. Each state contains: deck, player hands (with visibility rules), tableau, current player, scores, game-specific state.

**Parallelization:** Run multiple games in parallel using `multiprocessing.Pool`. Each worker gets genome + seed, returns results. This scales well to multi-core for medium-scale targets.

**Metric Collection:** The engine calculates all fitness proxies during simulation:
- Decision density (actions with >1 valid move)
- Comeback potential (trailing player win rate)
- Tension curve (score variance over time)
- Interaction frequency (moves affecting opponents)

---

## AI Player System

The AI player interface is simple but powerful. Multiple skill levels let you measure the skill gradient of evolved games.

### Player Interface

```python
class AIPlayer(ABC):
    @abstractmethod
    def choose_action(
        self,
        state: GameState,
        logic: GameLogic
    ) -> Action:
        pass
```

### Player Implementations

**RandomPlayer** - Baseline validation
- Chooses uniformly from valid actions
- Proves game is mechanically playable
- Used to detect broken/unplayable genomes

**GreedyPlayer** - Heuristic-based
- Uses simple scoring heuristics (play high cards, keep low cards, match suits when possible)
- Configurable heuristics per game type
- Fast and deterministic

**MCTSPlayer Family** - Tree search variants
- `MCTSWeak` (100 iterations): Slightly better than random
- `MCTSMedium` (1000 iterations): Moderate lookahead
- `MCTSStrong` (10000 iterations): Deep search, approximates expert play

All MCTS variants use UCB1 selection and random rollouts. The iteration budget is the only difference, creating a natural skill gradient.

### Skill Measurement

Run tournaments between all player types (Random vs Random, Random vs Weak, Weak vs Medium, Medium vs Strong, etc.). Win rate differentials quantify the skill gap:
- Games with flat win rates (Random ≈ Strong) are pure luck
- Games with steep gradients reward skill

### Performance Considerations

- Greedy is fast enough for thousands of games
- MCTS is expensive - use strategically (maybe 100 MCTS games per genome evaluation, 10,000 Random/Greedy games)

---

## Evolution Engine & Genetic Operators

The evolution engine orchestrates the generational loop with pluggable operators and fitness profiles.

### EvolutionEngine

```python
class EvolutionEngine:
    def __init__(
        self,
        population_size: int,
        operators: GeneticOperators,
        evaluator: FitnessEvaluator,
        profile: FitnessProfile
    ):
        ...

    def evolve(
        self,
        generations: int,
        tracker: ExperimentTracker
    ) -> Population:
        # Initialize random population
        population = self._generate_initial_population()

        for gen in range(generations):
            # Evaluate fitness
            fitnesses = self.evaluator.evaluate_batch(
                population,
                self.profile
            )

            # Selection
            parents = self.operators.select(population, fitnesses)

            # Crossover + Mutation
            offspring = self.operators.breed(parents)
            offspring = self.operators.mutate(offspring)

            # Elitism + replacement
            population = self.operators.next_generation(
                population, offspring, fitnesses
            )

            # Track progress
            tracker.save_generation(gen, population, fitnesses)
```

### GeneticOperators (Pluggable Strategies)

- `select()`: Tournament selection, roulette wheel, or rank-based
- `crossover()`: Single-point (swap setup vs gameplay), uniform (mix fields), or semantic (preserve game coherence)
- `mutate()`: Point mutations (change a value), insertion (add special effect), deletion (remove rule), structural (reorder phases)

### Validation

After each genetic operation, validate the genome is still playable (no contradictions, has at least one win condition, etc.). Discard invalid offspring.

---

## Fitness Evaluation & Profiles

The fitness evaluator runs simulations and computes metrics. Profiles weight these metrics differently for different game styles.

### FitnessEvaluator

```python
class FitnessEvaluator:
    def evaluate(
        self,
        genome: GameGenome,
        profile: FitnessProfile
    ) -> FitnessScore:
        # Run simulation suite
        results = self._run_simulation_suite(genome)

        # Compute raw metrics
        metrics = {
            'decision_density': self._calc_decision_density(results),
            'comeback_potential': self._calc_comeback_rate(results),
            'tension_curve': self._calc_tension_score(results),
            'interaction_freq': self._calc_interaction_score(results),
            'avg_game_length': mean([r.turn_count for r in results]),
            'skill_gradient': self._calc_skill_gradient(results),
            'rules_complexity': self._calc_complexity(genome),
            'termination_rate': self._calc_completion_rate(results)
        }

        # Apply profile weights
        weighted_score = profile.compute_fitness(metrics)

        return FitnessScore(
            total=weighted_score,
            metrics=metrics,
            raw_results=results
        )
```

### Simulation Suite

For each genome, run:
- 1000 games: Random vs Random (baseline playability)
- 1000 games: Greedy vs Greedy (heuristic competition)
- 100 games: Medium MCTS vs Medium MCTS (skilled play)
- 200 games: Mixed (Random vs Greedy, Greedy vs Medium, etc.) for skill gradient

### FitnessProfile Examples

```python
PROFILES = {
    'quick-party': {
        'avg_game_length': -0.3,      # shorter is better
        'decision_density': 0.2,       # some choices
        'rules_complexity': -0.2,      # simpler rules
        'skill_gradient': 0.1,         # slight skill
        'interaction_freq': 0.2,       # social interaction
    },
    'strategic-depth': {
        'skill_gradient': 0.4,         # high skill ceiling
        'decision_density': 0.3,       # many meaningful choices
        'comeback_potential': 0.15,    # avoid runaway leaders
        'avg_game_length': 0.1,        # longer games ok
        'rules_complexity': 0.05,      # complexity acceptable
    },
    # ... more profiles
}
```

Negative weights penalize (shorter games, simpler rules), positive weights reward.

---

## Experiment Tracking & Persistence

The system provides full experiment tracking for analysis and reproducibility.

### ExperimentTracker

```python
class ExperimentTracker:
    def __init__(self, experiment_name: str, output_dir: Path):
        self.base_path = output_dir / experiment_name / timestamp()
        self.base_path.mkdir(parents=True)

    def save_generation(
        self,
        generation: int,
        population: List[GameGenome],
        fitnesses: List[FitnessScore]
    ):
        gen_dir = self.base_path / f"gen_{generation:04d}"

        # Save population genomes
        with open(gen_dir / "population.json", "w") as f:
            json.dump([g.to_dict() for g in population], f)

        # Save fitness scores and metrics
        with open(gen_dir / "fitness.json", "w") as f:
            json.dump([f.to_dict() for f in fitnesses], f)

        # Save best genome separately
        best_idx = max(range(len(fitnesses)),
                      key=lambda i: fitnesses[i].total)
        with open(gen_dir / "best_genome.json", "w") as f:
            json.dump(population[best_idx].to_dict(), f)

        # Update metrics history (for plotting evolution curves)
        self._append_metrics_history(generation, fitnesses)
```

### Directory Structure

```
experiments/
  my-experiment-2026-01-10-14-30/
    config.yaml              # Run configuration
    gen_0000/
      population.json        # All genomes
      fitness.json          # All fitness scores
      best_genome.json      # Top performer
    gen_0001/
      ...
    metrics_history.csv      # Time series of fitness stats
    final_best/
      genome.json
      rules.txt             # LLM-generated human rules
      sample_games.json     # Example game playthroughs
```

### Resumability

Save config + random state every N generations. Can reload and continue from any checkpoint.

### Analysis Tools

Provide utility scripts to load experiment data, plot fitness curves over generations, compare runs, export genomes for external analysis.

---

## CLI Interface

The command-line tool provides intuitive control over evolution runs with sensible defaults.

### Main Commands

```bash
# Start new evolution run
cards-evolve run \
  --profile strategic-depth \
  --population 100 \
  --generations 50 \
  --name "strategic-games-v1"

# Resume from checkpoint
cards-evolve resume experiments/my-run-2026-01-10/

# Generate human-readable rules for a genome
cards-evolve explain \
  --genome experiments/my-run/gen_0049/best_genome.json \
  --output rules.txt

# Compare multiple runs
cards-evolve compare \
  experiments/run-1/ \
  experiments/run-2/ \
  --metric skill_gradient

# Export final best games
cards-evolve export \
  experiments/my-run/ \
  --top 10 \
  --format json

# Replay specific games for visualization
cards-evolve replay \
  --genome experiments/my-run/final_best/genome.json \
  --games 5 \
  --players mcts-medium mcts-medium
```

### Configuration File Support

Support YAML config for complex runs:

```yaml
# evolution_config.yaml
profile: strategic-depth
population_size: 100
generations: 50
experiment_name: "strategic-test"

operators:
  mutation_rate: 0.15
  crossover_rate: 0.7
  elitism_count: 10
  selection: tournament

simulation:
  games_per_evaluation: 2300
  parallel_workers: 8

constraints:
  min_turns: 5
  max_turns: 100
  min_decision_density: 0.2
```

Run with: `cards-evolve run --config evolution_config.yaml`

### Progress Display

Show live progress bar, current generation, best fitness, ETA. Log to file for headless runs.

---

## Natural Language Rule Generator

The rule generator uses a two-pass system to convert evolved genomes into clear, human-readable instructions for playtesting.

### Two-Pass Generation Process

**Pass 1: Content Generation** - Focus on completeness and accuracy
**Pass 2: Copyediting with Elements of Style** - Refine for clarity and conciseness

### RuleGenerator

```python
class RuleGenerator:
    def __init__(self, llm_client: LLMClient):
        self.client = llm_client

    def generate_rules(
        self,
        genome: GameGenome,
        sample_games: List[GameResult] = None
    ) -> str:
        # Pass 1: Generate initial rules
        draft_rules = self._generate_draft(genome, sample_games)

        # Pass 2: Copyedit with Elements of Style
        final_rules = self._copyedit_with_strunk(draft_rules)

        return final_rules

    def _generate_draft(self, genome: GameGenome, samples):
        prompt = self._build_generation_prompt(genome, samples)

        draft = self.client.generate(
            prompt,
            system="You are a card game rules writer. Generate complete, accurate rules for human players. Focus on covering all game mechanics."
        )

        return draft

    def _copyedit_with_strunk(self, draft_rules: str) -> str:
        """Apply Elements of Style principles to improve clarity."""
        copyedit_prompt = f"""
Copyedit these card game rules following William Strunk Jr.'s Elements of Style principles:

RULES TO EDIT:
{draft_rules}

Apply these principles:
1. Use active voice (not "cards are dealt" but "deal cards")
2. Put statements in positive form (not "do not play" but "discard")
3. Use definite, specific, concrete language
4. Omit needless words - be concise
5. Keep related words together
6. Place emphatic words at end of sentence

Focus on:
- Converting passive constructions to active
- Removing wordy phrases ("the fact that", "in order to")
- Making instructions direct and imperative
- Ensuring clarity without repetition

Return only the edited rules, maintaining all sections and game information.
"""

        edited = self.client.generate(
            copyedit_prompt,
            system="You are a copyeditor applying The Elements of Style to technical writing. Make the text clear, direct, and concise."
        )

        return edited

    def _build_generation_prompt(self, genome: GameGenome, samples):
        return f"""
Convert this game genome into human-readable rules:

SETUP:
{genome.setup.to_description()}

GAMEPLAY:
{genome.turn_phases.to_description()}

SPECIAL CARDS:
{genome.special_effects.to_description()}

WIN CONDITION:
{genome.win_conditions.to_description()}

SCORING:
{genome.scoring.to_description()}

{self._format_sample_games(samples) if samples else ""}

Generate a rulebook with sections: Setup, How to Play, Special Rules, Winning.
Include examples where helpful.
"""
```

### LLM Client Abstraction

Support multiple providers (OpenAI, Anthropic) via unified interface. Allow API key configuration via environment variables.

### Pass 1: Content Generation

The first pass focuses on completeness:
- Convert all genome fields to natural language
- Include sample game walkthroughs for concrete examples
- Explain edge cases and unusual rule interactions
- No optimization for style - just accuracy

### Pass 2: Elements of Style Copyediting

The second pass refines for clarity:
- **Active voice**: "Deal 7 cards" not "7 cards are dealt"
- **Positive form**: "Keep cards hidden" not "Don't show cards"
- **Omit needless words**: "If you cannot play" not "In the case that you are unable to play"
- **Concrete language**: "Draw 2 cards" not "Perform the draw action twice"
- **Direct imperatives**: "Play a card" not "The player may choose to play a card"

### Caching

Cache both draft and final rules per genome_id to avoid redundant API calls during analysis. Store separately to enable regenerating just pass 2 if editing principles change.

### Fallback

If LLM unavailable, provide template-based fallback that's less polished but functional. Skip pass 2 if copyediting fails, returning pass 1 output.

---

## Error Handling & Validation

Evolved games can have invalid or nonsensical rules. The system needs robust validation at multiple stages.

### Genome Validation Pipeline

```python
class GenomeValidator:
    def validate(self, genome: GameGenome) -> ValidationResult:
        errors = []
        warnings = []

        # Structural validation
        if genome.setup.cards_per_player * len(players) > 52:
            errors.append("Not enough cards in deck for setup")

        if not genome.win_conditions:
            errors.append("Must have at least one win condition")

        # Playability validation
        if not self._has_valid_moves(genome):
            errors.append("No valid moves possible from initial state")

        # Termination validation
        if genome.max_turns < 1:
            errors.append("Max turns must be positive")

        if genome.max_turns > 1000:
            warnings.append("Very long game limit may cause slow simulations")

        # Logical consistency
        if self._has_contradictory_rules(genome):
            errors.append("Contradictory rules detected")

        return ValidationResult(
            valid=len(errors) == 0,
            errors=errors,
            warnings=warnings
        )
```

### Validation Stages

1. **Post-generation**: Validate new random genomes before adding to initial population
2. **Post-mutation**: Validate offspring after genetic operations, discard invalid
3. **Pre-simulation**: Quick sanity check before expensive simulation
4. **Runtime**: Catch infinite loops, deadlocks during game execution (timeout after max_turns)

### Simulation Safety

```python
class SafeGameEngine(GameEngine):
    def simulate_game(self, genome, players, seed):
        try:
            result = super().simulate_game(genome, players, seed)
            return result
        except InfiniteLoopError:
            return GameResult.failed("infinite_loop")
        except NoValidMovesError:
            return GameResult.failed("deadlock")
        except Exception as e:
            logger.error(f"Simulation failed: {e}", genome_id=genome.id)
            return GameResult.failed("unknown_error")
```

Failed games get zero fitness, causing natural selection to eliminate broken genomes.

---

## Testing Strategy

A production system needs comprehensive tests at multiple levels to ensure reliability.

### Test Pyramid

**Unit Tests** - Fast, isolated component tests
- Genome validation logic (valid/invalid rule combinations)
- Genetic operators (mutation produces valid offspring, crossover preserves constraints)
- Fitness metric calculations (decision density, skill gradient formulas)
- AI player logic (RandomPlayer truly random, GreedyPlayer heuristics)
- GenomeInterpreter output (structured genome → correct Python logic)

**Integration Tests** - Component interactions
- Full game simulation (genome → GameEngine → GameResult)
- Evolution pipeline (generation → evaluation → selection → breeding)
- ExperimentTracker persistence (save/load round-trip)
- CLI commands (run, resume, explain, export)
- Two-pass rule generation (draft → copyedit → final output)
- Rule caching (same genome returns cached result)

**End-to-End Tests** - Full system validation
- Small evolution runs (5 generations, 10 population) complete successfully
- Known game genomes (manually crafted "Crazy 8s" genome) simulate correctly
- Fitness profiles produce expected selection pressure
- Generated rules are readable and accurate
- Copyedited rules use active voice and omit needless words (manual review)
- Pass 2 fallback works when copyediting fails

**Property-Based Tests** - Generative testing with hypothesis
- Any valid genome should simulate without errors
- Mutations of valid genomes should be valid or rejected cleanly
- Selection should prefer higher fitness
- Population diversity should not collapse to zero

**Smoke Tests** - Quick validation
- Can initialize all components
- Sample genome runs single game
- All CLI commands show help text

**Performance Tests** - Benchmarking
- Single game simulation < 100ms (random players)
- 1000 game batch with 8 workers completes in reasonable time
- Memory usage stays bounded during long runs

---

## Project Structure & Dependencies

### Project Structure

```
cards-evolve/
  pyproject.toml              # Poetry/pip config
  README.md
  CLAUDE.md

  src/
    cards_evolve/
      __init__.py

      # Genome layer
      genome/
        __init__.py
        genome.py             # GameGenome dataclass
        generator.py          # Random genome generation
        interpreter.py        # Genome → executable Python
        validator.py          # Validation logic

      # Simulation layer
      simulation/
        __init__.py
        engine.py             # GameEngine
        state.py              # GameState, GameLogic
        ai_players.py         # RandomPlayer, GreedyPlayer, MCTSPlayer

      # Evolution layer
      evolution/
        __init__.py
        engine.py             # EvolutionEngine
        operators.py          # GeneticOperators
        fitness.py            # FitnessEvaluator, FitnessProfile

      # CLI/Output layer
      cli/
        __init__.py
        main.py               # Entry point
        commands.py           # run, resume, explain, etc.

      tracking/
        __init__.py
        tracker.py            # ExperimentTracker
        analyzer.py           # Analysis utilities

      rules/
        __init__.py
        generator.py          # RuleGenerator
        llm_client.py         # LLM API abstraction

  tests/
    unit/
    integration/
    e2e/
    fixtures/                 # Sample genomes, test data

  experiments/                # Output directory (gitignored)

  docs/
    plans/                    # Design documents
```

### Key Dependencies

```toml
[tool.poetry.dependencies]
python = "^3.11"
numpy = "^1.26"              # Numerical operations
pydantic = "^2.5"            # Data validation
click = "^8.1"               # CLI framework
pyyaml = "^6.0"              # Config files
anthropic = "^0.18"          # Claude API (or openai)
tqdm = "^4.66"               # Progress bars
pytest = "^7.4"              # Testing
hypothesis = "^6.92"         # Property-based testing
```

### Additional Considerations

**Entry Point:** `cards-evolve` command installed via Poetry/pip, points to `cli.main:main()`.

**Configuration:** Support config file hierarchy: system defaults → project config → CLI args. CLI args override config file.

**Logging:** Structured logging with different levels for console vs file. Log all simulation parameters for reproducibility.

---

## Next Steps

With this design in place, implementation should follow this sequence:

1. **Foundation** (Week 1-2)
   - Project scaffolding and dependencies
   - GameGenome dataclass definitions
   - GenomeValidator with basic rules

2. **Simulation Core** (Week 2-3)
   - GameState and GameLogic interfaces
   - GenomeInterpreter (structured data → executable)
   - GameEngine with RandomPlayer
   - Basic fitness metrics

3. **Evolution Engine** (Week 3-4)
   - GeneticOperators (mutation, crossover, selection)
   - FitnessEvaluator with FitnessProfile
   - EvolutionEngine orchestration
   - Initial population generation

4. **AI Players** (Week 4-5)
   - GreedyPlayer with heuristics
   - MCTSPlayer variants (weak/medium/strong)
   - Skill gradient measurement

5. **CLI & Tracking** (Week 5-6)
   - Command interface (run, resume, explain, export)
   - ExperimentTracker persistence
   - Progress display and logging

6. **Rule Generation** (Week 6)
   - LLM client abstraction
   - RuleGenerator with two-pass system (content generation + Elements of Style copyediting)
   - Caching for both draft and final rules
   - Fallback strategies for LLM failures

7. **Testing & Polish** (Week 7-8)
   - Comprehensive test suite
   - Performance optimization
   - Documentation and examples

---

**End of Design Document**
