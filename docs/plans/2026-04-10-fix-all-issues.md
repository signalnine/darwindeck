# Fix All 8 Issues Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use conclave:executing-plans to implement this plan task-by-task.

**Goal:** Fix all 8 open bd issues: CGo memory leak, process pool recreation, TrickPhase fitness exploit, fitness ceiling, skill eval caching, crossover limitation, card count validation, and inert effects inflation.

**Architecture:** Eight independent fixes across the Python codebase (no Go changes needed except confirming the FreeResponse export). Fixes are grouped by dependency: the CGo leak is standalone, parallel_fitness and skill_eval are standalone, and the 4 fitness/crossover fixes can be done independently but should be tested together.

**Tech Stack:** Python 3.11+, ctypes (CGo bridge), multiprocessing, frozen dataclasses

---

### Task 1: Fix CGo memory leak (cards-evolve-49m)

**Files:**
- Modify: `src/darwindeck/bindings/cgo_bridge.py`
- Test: `tests/unit/test_cgo_bridge.py` (create)

**Dependencies:** none

**Step 1: Write the failing test**

Create `tests/unit/test_cgo_bridge.py`:

```python
"""Tests for CGo bridge memory management."""

import pytest
from darwindeck.bindings.cgo_bridge import _lib


def test_free_response_exported():
    """FreeResponse function is available in the shared library."""
    assert hasattr(_lib, 'FreeResponse'), "FreeResponse not exported from libcardsim.so"
```

**Step 2: Run test to verify it passes** (this confirms the export exists)

Run: `uv run pytest tests/unit/test_cgo_bridge.py -v`

**Step 3: Fix the memory leak**

In `src/darwindeck/bindings/cgo_bridge.py`, add the FreeResponse signature and call it after copying bytes:

```python
# Add after existing function signatures (after line 14):
_lib.FreeResponse.argtypes = [ctypes.c_void_p]
_lib.FreeResponse.restype = None
```

Then in `simulate_batch()`, after copying result bytes (after line 40), add:

```python
    # Free Go-allocated response buffer
    _lib.FreeResponse(result_ptr)
```

**Step 4: Run tests**

Run: `uv run pytest tests/unit/test_cgo_bridge.py tests/unit/test_fitness.py -v`

**Step 5: Commit**

```bash
git add src/darwindeck/bindings/cgo_bridge.py tests/unit/test_cgo_bridge.py
git commit -m "fix: free CGo response buffer to prevent memory leak"
bd close cards-evolve-49m
```

---

### Task 2: Fix process pool recreation (cards-evolve-88x)

**Files:**
- Modify: `src/darwindeck/evolution/parallel_fitness.py`
- Test: `tests/unit/test_parallel_fitness.py` (existing)

**Dependencies:** none

**Step 1: Write the failing test**

Add to `tests/unit/test_parallel_fitness.py`:

```python
def test_pool_reuse_across_evaluations():
    """Pool should be created once and reused, not recreated each call."""
    evaluator = ParallelFitnessEvaluator(
        evaluator_factory=lambda: FitnessEvaluator(),
        num_workers=2
    )
    # Pool should not exist until first call
    assert evaluator._pool is None

    # After evaluate, pool should exist
    genome = create_war_genome()
    evaluator.evaluate_population([genome], num_simulations=10)
    assert evaluator._pool is not None

    # Second call should reuse same pool
    pool_ref = evaluator._pool
    evaluator.evaluate_population([genome], num_simulations=10)
    assert evaluator._pool is pool_ref

    # Cleanup
    evaluator.shutdown()
    assert evaluator._pool is None
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_parallel_fitness.py::test_pool_reuse_across_evaluations -v`
Expected: FAIL (no `_pool` attribute, no `shutdown()` method)

**Step 3: Implement pool reuse**

Modify `ParallelFitnessEvaluator` in `parallel_fitness.py`:

Replace `evaluate_population` method (lines 80-111) with:

```python
    def __init__(
        self,
        evaluator_factory: Callable[[], FitnessEvaluator],
        simulator_factory: Optional[Callable[[], GoSimulator]] = None,
        num_workers: Optional[int] = None
    ):
        self.evaluator_factory = evaluator_factory
        self.simulator_factory = simulator_factory or _create_simulator
        self.num_workers = num_workers or mp.cpu_count()
        self._pool = None

    def _get_pool(self):
        """Get or create the process pool."""
        if self._pool is None:
            self._pool = _mp_context.Pool(
                processes=self.num_workers,
                initializer=_worker_init,
                initargs=(self.evaluator_factory, self.simulator_factory)
            )
        return self._pool

    def evaluate_population(
        self,
        genomes: List[GameGenome],
        num_simulations: int = 100,
        use_mcts: bool = False
    ) -> List[FitnessMetrics]:
        if not genomes:
            return []

        tasks = [
            EvaluationTask(genome, num_simulations, use_mcts)
            for genome in genomes
        ]

        pool = self._get_pool()
        results = pool.map(_evaluate_task, tasks)
        return results

    def shutdown(self):
        """Shut down the process pool."""
        if self._pool is not None:
            self._pool.terminate()
            self._pool.join()
            self._pool = None
```

**Step 4: Run tests**

Run: `uv run pytest tests/unit/test_parallel_fitness.py -v`

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/parallel_fitness.py tests/unit/test_parallel_fitness.py
git commit -m "fix: reuse process pool across generations instead of recreating"
bd close cards-evolve-88x
```

---

### Task 3: Fix TrickPhase free interaction score (cards-evolve-iol)

**Files:**
- Modify: `src/darwindeck/evolution/fitness_full.py`
- Test: `tests/unit/test_fitness.py` (existing)

**Dependencies:** none

**Step 1: Write the failing test**

Add to `tests/unit/test_fitness.py`:

```python
def test_interaction_frequency_uses_real_data_not_heuristic():
    """When real instrumentation data is available, interaction_frequency should use it,
    not fall back to heuristic that gives TrickPhase games free 1.0 score."""
    evaluator = FitnessEvaluator(style='balanced')

    # Simulate a TrickPhase game with real instrumentation showing low interaction
    results = SimulationResults(
        total_games=100,
        wins=(50, 50),
        player_count=2,
        draws=0,
        avg_turns=40,
        errors=0,
        total_decisions=200,
        total_valid_moves=400,
        forced_decisions=50,
        total_hand_size=800,
        total_interactions=20,  # Low interaction
        total_actions=400,      # Many actions
    )

    # Create a trick-based genome
    from darwindeck.genome.examples import create_hearts_genome
    genome = create_hearts_genome()

    metrics = evaluator.evaluate(genome, results)
    # With real data showing 20/400 = 5% interaction, score should be ~0.05, not 1.0
    assert metrics.interaction_frequency < 0.2, (
        f"Expected low interaction_frequency from real data, got {metrics.interaction_frequency}"
    )
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_fitness.py::test_interaction_frequency_uses_real_data_not_heuristic -v`

Note: This test may already pass if the real instrumentation path works correctly. If it passes, the issue is that the heuristic path (when `total_actions == 0`) gives inflated scores. In that case, also add:

```python
def test_interaction_heuristic_does_not_give_trick_phase_free_score():
    """Heuristic interaction score should not give TrickPhase 1.0 for free."""
    evaluator = FitnessEvaluator(style='balanced')

    # No instrumentation data -- falls back to heuristic
    results = SimulationResults(
        total_games=100,
        wins=(50, 50),
        player_count=2,
        draws=0,
        avg_turns=40,
        errors=0,
    )

    from darwindeck.genome.examples import create_hearts_genome
    genome = create_hearts_genome()

    metrics = evaluator.evaluate(genome, results)
    # Heuristic should not give > 0.7 just for being trick-based
    assert metrics.interaction_frequency < 0.7, (
        f"Trick-based heuristic too generous: {metrics.interaction_frequency}"
    )
```

**Step 3: Fix the heuristic**

In `fitness_full.py`, modify the heuristic fallback for interaction_frequency (lines 285-294):

```python
        else:
            # Fallback to heuristic
            special_effects_score = min(1.0, len(genome.special_effects) / 3.0)
            # Trick-based gets a moderate bonus, not a huge one
            trick_based_score = 0.15 if genome.turn_structure.is_trick_based else 0.0
            multi_phase_score = min(0.2, len(genome.turn_structure.phases) / 10.0)

            interaction_frequency = min(1.0,
                special_effects_score * 0.4 +
                trick_based_score +
                multi_phase_score
            )
```

Change `trick_based_score` from 0.3 to 0.15. This halves the free bonus. Games must earn interaction score from actual special effects or phase complexity.

**Step 4: Run tests**

Run: `uv run pytest tests/unit/test_fitness.py -v`

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/fitness_full.py tests/unit/test_fitness.py
git commit -m "fix: reduce TrickPhase heuristic interaction bonus from 0.3 to 0.15"
bd close cards-evolve-iol
```

---

### Task 4: Fix fitness ceiling from skill_vs_luck heuristic (cards-evolve-527)

**Files:**
- Modify: `src/darwindeck/evolution/fitness_full.py`
- Test: `tests/unit/test_fitness.py` (existing)

**Dependencies:** none

**Step 1: Write the failing test**

Add to `tests/unit/test_fitness.py`:

```python
def test_skill_vs_luck_heuristic_can_reach_high_values():
    """skill_vs_luck should be able to reach 0.85+ for well-designed games
    without requiring MCTS evaluation."""
    evaluator = FitnessEvaluator(style='balanced')

    # A well-designed game: long, balanced, complex
    results = SimulationResults(
        total_games=100,
        wins=(50, 50),
        player_count=2,
        draws=0,
        avg_turns=60,
        errors=0,
        total_decisions=300,
        total_valid_moves=900,
        forced_decisions=30,
        total_hand_size=1200,
        total_interactions=150,
        total_actions=600,
    )

    from darwindeck.genome.examples import create_hearts_genome
    genome = create_hearts_genome()

    metrics = evaluator.evaluate(genome, results)
    assert metrics.skill_vs_luck >= 0.75, (
        f"skill_vs_luck too low for well-designed game: {metrics.skill_vs_luck}"
    )
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_fitness.py::test_skill_vs_luck_heuristic_can_reach_high_values -v`

**Step 3: Recalibrate the heuristic**

In `fitness_full.py`, replace the skill_vs_luck heuristic (lines 356-374) with a version that uses decision density from the real instrumentation data as an input, and has a higher ceiling:

```python
        else:
            # Without MCTS, estimate skill potential from game structure + instrumentation
            length_factor = min(1.0, results.avg_turns / 50.0)  # Cap at 50 turns (was 80)

            # Use real decision density if available, else estimate from structure
            if hasattr(results, 'total_decisions') and results.total_decisions > 0:
                forced_ratio = results.forced_decisions / results.total_decisions
                decision_factor = 1.0 - forced_ratio  # More non-forced = more skill
            else:
                complexity_factor = min(1.0, (
                    len(genome.turn_structure.phases) +
                    len(genome.special_effects) +
                    (1 if genome.turn_structure.is_trick_based else 0)
                ) / 6.0)
                decision_factor = complexity_factor

            balance_factor = comeback_potential

            # Weighted combination with higher ceiling
            skill_vs_luck = min(1.0,
                length_factor * 0.3 +
                decision_factor * 0.4 +
                balance_factor * 0.3
            )
```

Key changes:
- Lowered turn cap from 80 to 50 (games reach full length_factor sooner)
- Uses actual forced_decision ratio when instrumentation available (instead of structural heuristic)
- `decision_factor` gets highest weight (0.4) since real decision data is the best skill proxy
- Removed double-counting of comeback_potential by reducing its weight

**Step 4: Run tests**

Run: `uv run pytest tests/unit/test_fitness.py -v`

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/fitness_full.py tests/unit/test_fitness.py
git commit -m "fix: recalibrate skill_vs_luck heuristic to use real decision data"
bd close cards-evolve-527
```

---

### Task 5: Fix skill eval GoSimulator recreation (cards-evolve-f75)

**Files:**
- Modify: `src/darwindeck/evolution/skill_evaluation.py`
- Test: `tests/unit/test_skill_evaluation.py` (create)

**Dependencies:** none

**Step 1: Write the failing test**

Create `tests/unit/test_skill_evaluation.py`:

```python
"""Tests for skill evaluation caching."""

import pytest
from unittest.mock import patch, MagicMock
from darwindeck.evolution.skill_evaluation import _evaluate_skill_task, _SkillEvalTask, _worker_simulator


def test_worker_simulator_used_when_available():
    """Skill eval should use module-level worker simulator when initialized."""
    # This tests that the worker path exists
    from darwindeck.evolution.skill_evaluation import _worker_simulator
    # Default is None (no pool initialized)
    assert _worker_simulator is None
```

**Step 2: Implement worker-level simulator for skill eval**

In `skill_evaluation.py`, add worker initialization similar to `parallel_fitness.py`:

After the existing imports (around line 20), add:

```python
# Worker-level simulator for parallel evaluation (initialized per-process)
_worker_simulator: Optional[GoSimulator] = None


def _skill_worker_init():
    """Initialize worker process with its own GoSimulator."""
    global _worker_simulator
    _worker_simulator = GoSimulator()
```

Modify `evaluate_skill()` to accept an optional simulator parameter (line 54-77):

```python
def evaluate_skill(
    genome: GameGenome,
    num_games: int = 100,
    mcts_iterations: int = 100,
    timeout_sec: float = 60.0,
    progress_callback: Optional[Callable[[str], None]] = None,
    simulator: Optional[GoSimulator] = None
) -> SkillEvalResult:
    start_time = time.time()
    sim = simulator or _worker_simulator or GoSimulator()
    ...
```

Replace `simulator = GoSimulator()` on line 77 with `sim = simulator or _worker_simulator or GoSimulator()` and use `sim` throughout the function instead of `simulator`.

Modify `_evaluate_skill_task` to use the worker simulator:

```python
def _evaluate_skill_task(task: _SkillEvalTask) -> SkillEvalResult:
    """Worker function for parallel evaluation."""
    return evaluate_skill(
        genome=task.genome,
        num_games=task.num_games,
        mcts_iterations=task.mcts_iterations,
        timeout_sec=task.timeout_sec,
        simulator=_worker_simulator
    )
```

Modify `evaluate_batch_skill` to use worker init (around line 280):

```python
    with _mp_context.Pool(
        processes=num_workers,
        initializer=_skill_worker_init
    ) as pool:
```

**Step 3: Run tests**

Run: `uv run pytest tests/unit/test_skill_evaluation.py tests/unit/test_fitness.py -v`

**Step 4: Commit**

```bash
git add src/darwindeck/evolution/skill_evaluation.py tests/unit/test_skill_evaluation.py
git commit -m "fix: reuse GoSimulator in skill eval workers for bytecode caching"
bd close cards-evolve-f75
```

---

### Task 6: Expand crossover to swap win conditions and effects (cards-evolve-18v)

**Files:**
- Modify: `src/darwindeck/evolution/operators.py`
- Test: `tests/unit/test_operators.py` (existing)

**Dependencies:** none

**Step 1: Write the failing test**

Add to `tests/unit/test_operators.py`:

```python
def test_crossover_can_swap_win_conditions():
    """Crossover should sometimes produce offspring with mixed win conditions."""
    from darwindeck.genome.examples import create_war_genome, create_hearts_genome

    crossover = CrossoverOperator(probability=1.0)  # Always apply
    parent1 = create_war_genome()
    parent2 = create_hearts_genome()

    # Run many crossovers and check if win conditions ever differ from both parents
    swapped = False
    for _ in range(50):
        child1, child2 = crossover.crossover(parent1, parent2)
        if child1.win_conditions != parent1.win_conditions:
            swapped = True
            break
        if child2.win_conditions != parent2.win_conditions:
            swapped = True
            break

    assert swapped, "Crossover never swapped win conditions in 50 attempts"


def test_crossover_can_swap_special_effects():
    """Crossover should sometimes produce offspring with mixed special effects."""
    from darwindeck.genome.examples import create_war_genome, create_uno_genome

    crossover = CrossoverOperator(probability=1.0)
    parent1 = create_war_genome()  # No effects
    parent2 = create_uno_genome()  # Has effects

    swapped = False
    for _ in range(50):
        child1, child2 = crossover.crossover(parent1, parent2)
        # child1 inherits from parent1 base, should sometimes get parent2's effects
        if child1.special_effects != parent1.special_effects:
            swapped = True
            break

    assert swapped, "Crossover never swapped special effects in 50 attempts"
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_operators.py::test_crossover_can_swap_win_conditions -v`

**Step 3: Implement multi-component crossover**

In `operators.py`, modify the `crossover` method (lines 975-1034) to also swap win conditions, special effects, and setup with 50% probability each:

After creating the offspring phase lists (around line 1016), add component swaps:

```python
        # Component-level crossover: swap win conditions, effects, setup with 50% chance each
        child1_win = parent1.win_conditions
        child2_win = parent2.win_conditions
        if random.random() < 0.5:
            child1_win, child2_win = child2_win, child1_win

        child1_effects = parent1.special_effects
        child2_effects = parent2.special_effects
        if random.random() < 0.5:
            child1_effects, child2_effects = child2_effects, child1_effects

        child1_setup = parent1.setup
        child2_setup = parent2.setup
        if random.random() < 0.5:
            child1_setup, child2_setup = child2_setup, child1_setup

        # Create offspring genomes
        offspring1 = replace(
            parent1,
            setup=child1_setup,
            turn_structure=replace(parent1.turn_structure, phases=tuple(offspring1_phases)),
            win_conditions=child1_win,
            special_effects=child1_effects,
            generation=parent1.generation + 1,
            genome_id=generate_name()
        )

        offspring2 = replace(
            parent2,
            setup=child2_setup,
            turn_structure=replace(parent2.turn_structure, phases=tuple(offspring2_phases)),
            win_conditions=child2_win,
            special_effects=child2_effects,
            generation=parent2.generation + 1,
            genome_id=generate_name()
        )
```

**Step 4: Run tests**

Run: `uv run pytest tests/unit/test_operators.py -v`

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/operators.py tests/unit/test_operators.py
git commit -m "feat: expand crossover to swap win conditions, effects, and setup"
bd close cards-evolve-18v
```

---

### Task 7: Add card count validation (cards-evolve-232)

**Files:**
- Modify: `src/darwindeck/evolution/operators.py`
- Modify: `src/darwindeck/evolution/fitness_full.py`
- Test: `tests/unit/test_operators.py` (existing)

**Dependencies:** none

**Step 1: Write the failing test**

Add to `tests/unit/test_operators.py`:

```python
def test_tweak_parameter_enforces_deck_size_constraint():
    """cards_per_player * player_count should not exceed 52."""
    from darwindeck.genome.examples import create_hearts_genome

    genome = create_hearts_genome()  # 4-player game

    # Manually create genome with invalid card count
    from dataclasses import replace
    invalid_setup = replace(genome.setup, cards_per_player=15)
    invalid_genome = replace(genome, setup=invalid_setup)

    # Validate function should catch this
    from darwindeck.evolution.operators import validate_card_count
    assert not validate_card_count(invalid_genome), (
        "15 cards * 4 players = 60 > 52, should be invalid"
    )

    # Valid case
    valid_setup = replace(genome.setup, cards_per_player=10)
    valid_genome = replace(genome, setup=valid_setup)
    assert validate_card_count(valid_genome), (
        "10 cards * 4 players = 40 <= 52, should be valid"
    )
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_operators.py::test_tweak_parameter_enforces_deck_size_constraint -v`

**Step 3: Add validation function and integrate it**

In `operators.py`, add a validation function near the top (after imports):

```python
STANDARD_DECK_SIZE = 52

def validate_card_count(genome: GameGenome) -> bool:
    """Check that cards_per_player * player_count fits in the deck."""
    player_count = genome.setup.player_count if hasattr(genome.setup, 'player_count') else 2
    # For genomes without explicit player_count, infer from win_conditions or default to 4
    # Use max expected players (4) as conservative check
    max_players = 4
    total_cards_needed = genome.setup.cards_per_player * max_players
    return total_cards_needed <= STANDARD_DECK_SIZE
```

Note: Check what field tracks player count. If `GameGenome` doesn't have a `player_count` field, use `max_players=4` as the conservative bound.

In `TweakParameterMutation.mutate()`, add a clamp after modifying `cards_per_player`:

```python
    # Clamp cards_per_player to deck constraint
    max_cards = STANDARD_DECK_SIZE // 4  # Conservative: assume up to 4 players
    new_cards = min(new_setup.cards_per_player, max_cards)
    if new_cards != new_setup.cards_per_player:
        new_setup = replace(new_setup, cards_per_player=new_cards)
```

Also add a penalty in `fitness_full.py` `_compute_metrics` for games with impossible card counts -- add before the final fitness computation (around line 420):

```python
        # Penalize impossible card counts
        max_players = 4
        if genome.setup.cards_per_player * max_players > 52:
            # Scale penalty by how far over the limit
            overflow = (genome.setup.cards_per_player * max_players - 52) / 52
            penalty = max(0.5, 1.0 - overflow)
            total_fitness *= penalty
```

**Step 4: Run tests**

Run: `uv run pytest tests/unit/test_operators.py tests/unit/test_fitness.py -v`

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/operators.py src/darwindeck/evolution/fitness_full.py tests/unit/test_operators.py
git commit -m "fix: validate card count against deck size, clamp mutations"
bd close cards-evolve-232
```

---

### Task 8: Fix inert effects inflating rules_complexity (cards-evolve-avn)

**Files:**
- Modify: `src/darwindeck/evolution/fitness_full.py`
- Test: `tests/unit/test_fitness.py` (existing)

**Dependencies:** none

**Step 1: Write the failing test**

Add to `tests/unit/test_fitness.py`:

```python
def test_rules_complexity_not_inflated_by_trick_phase_effects():
    """Special effects on trick-taking games should not inflate rules_complexity
    since effects don't functionally interact with TrickPhase."""
    evaluator = FitnessEvaluator(style='balanced')

    results = SimulationResults(
        total_games=100,
        wins=(50, 50),
        player_count=2,
        draws=0,
        avg_turns=40,
        errors=0,
        total_decisions=200,
        total_valid_moves=400,
        forced_decisions=50,
        total_hand_size=800,
        total_interactions=100,
        total_actions=400,
    )

    from darwindeck.genome.examples import create_hearts_genome
    from dataclasses import replace as dc_replace
    from darwindeck.genome.schema import SpecialEffect, EffectType, TargetSelector, Rank

    base_genome = create_hearts_genome()

    # Genome with no effects
    no_effects_genome = dc_replace(base_genome, special_effects=())
    no_effects_metrics = evaluator.evaluate(no_effects_genome, results)

    # Genome with 3 effects (but it's trick-based, so effects are inert)
    effects = (
        SpecialEffect(Rank.ACE, EffectType.SKIP_NEXT, TargetSelector.NEXT_PLAYER),
        SpecialEffect(Rank.KING, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        SpecialEffect(Rank.QUEEN, EffectType.REVERSE_DIRECTION, TargetSelector.ALL_OPPONENTS),
    )
    effects_genome = dc_replace(base_genome, special_effects=effects)
    effects_metrics = evaluator.evaluate(effects_genome, results)

    # Rules complexity should not increase significantly for inert effects
    diff = effects_metrics.rules_complexity - no_effects_metrics.rules_complexity
    assert diff < 0.1, (
        f"Inert effects inflated rules_complexity by {diff:.3f} "
        f"(no_effects={no_effects_metrics.rules_complexity:.3f}, "
        f"with_effects={effects_metrics.rules_complexity:.3f})"
    )
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/unit/test_fitness.py::test_rules_complexity_not_inflated_by_trick_phase_effects -v`

**Step 3: Discount effects for trick-based games**

In `fitness_full.py`, modify the effects_score calculation (around lines 304-315):

```python
        # Gameplay richness
        special_effects_count = len(genome.special_effects)
        scoring_rules_count = len(genome.scoring_rules)

        # Discount effects for trick-based games where they're functionally inert
        effective_effects_count = special_effects_count
        if genome.turn_structure.is_trick_based:
            effective_effects_count = 0  # Effects don't interact with TrickPhase

        # Reward 1-3 special effects, neutral at 0, penalty beyond 5
        effects_score = min(1.0, max(0.0, (
            0.7 +  # Baseline for no effects
            (effective_effects_count / 3.0) * 0.5 -
            max(0.0, (effective_effects_count - 3) * 0.1)
        )))
```

**Step 4: Run tests**

Run: `uv run pytest tests/unit/test_fitness.py -v`

**Step 5: Commit**

```bash
git add src/darwindeck/evolution/fitness_full.py tests/unit/test_fitness.py
git commit -m "fix: discount special effects in rules_complexity for trick-based games"
bd close cards-evolve-avn
```

---

## Verification

After all 8 tasks, run the full test suite:

```bash
uv run pytest tests/ -v
cd src/gosim && go test ./... -v
```

Then run a quick evolution to verify the fitness landscape has changed:

```bash
uv run python -m darwindeck.cli.evolve --population-size 50 --generations 10 --verbose
```

Expected changes:
- No more memory growth during long runs
- Faster generation times (pool reuse)
- More diverse evolved games (not all trick-taking)
- Fitness ceiling should be higher (new skill heuristic)
- No impossible card counts in top genomes
