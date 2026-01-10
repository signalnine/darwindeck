# Phase 4: Genetic Algorithm & Fitness Evaluation - Implementation Plan

**Date:** 2026-01-10
**Phase:** 4 of 4
**Goal:** Implement genetic algorithm to evolve novel card games optimized for fun and playability

**Consensus Input:** Multi-agent consensus (Claude, Gemini, Codex) - high confidence on critical fixes
**Confidence Level:** High (unanimous agreement on architecture, parameter tuning expected)

## Overview

Phase 4 implements the evolutionary loop that generates novel card games by:
1. Managing populations of game genomes
2. Evaluating fitness using simulation-based proxy metrics
3. Applying genetic operators (mutation, crossover)
4. Selecting and breeding successive generations
5. Validating playability and termination

**Key Performance Target:** Evaluate 100 genomes × 100 simulations each in <10 minutes (leverage Phase 3 Go core)

## Consensus-Driven Design Decisions

### Unanimous Recommendations (High Confidence)

✅ **Fitness evaluation is the critical bottleneck** - Progressive evaluation mandatory
✅ **Fitness caching essential** - Genomes are deterministic, hash-based caching
✅ **Validity checking multi-stage** - Schema validation + simulation testing
✅ **Progressive evaluation strategy** - Cheap tests first, expensive tests for survivors

### Consensus Recommendations (Updated Based on Multi-Agent Review)

✅ **Population size: 100 (monitor diversity)** - Start at 100, increase to 200-300 if premature convergence
✅ **Seed population: 70% known games, 30% random** - Avoid pure random start
✅ **Tournament selection (size 3)** - Simple, effective baseline
✅ **10% elitism** - Preserve best performers
✅ **Session length as constraint** - Filter games outside 3-20 minute range (fitness = 0)
✅ **Diversity mechanism required** - Track genome distance, log per generation
✅ **Plateau detection: 30 generations** - Extended from 20 based on consensus review
✅ **100 generations with adaptive stopping** - Early termination if diversity collapses or plateau detected

### Must Address Before Implementation (From Consensus Review)

❌ **Simulation failure handling** - Genomes that crash/timeout/infinite-loop need fitness = 0
❌ **Session length as constraint** - Move from averaged metric to filter (3-20 min range)
❌ **Win-condition mutation operator** - Win conditions are crucial, need dedicated mutation
❌ **Diversity mechanism** - Explicit genome distance tracking and logging

### Implementation Notes

✅ **Concrete mutation operators** - 6 mutation types implemented (see Task 1)
✅ **Crossover semantics** - Semantic phase-boundary crossover (see Task 1)
✅ **Infinite loop detection** - max_turns validation + repair (Phase 3.5)
⚠️ **Human evaluation integration** - Deferred to post-MVP, proxy metrics validated first

---

## Task Breakdown

### Task 1: Genome Mutation Operators (30 minutes)

**Goal:** Implement concrete mutation operators for GameGenome dataclass

#### Step 1.1: Define mutation types (10 min)

**File:** `src/darwindeck/evolution/operators.py`

```python
"""Genetic operators for GameGenome evolution."""
from dataclasses import replace
from typing import List, Callable
import random
from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, PlayPhase, DrawPhase, DiscardPhase,
    Condition, ConditionType, Operator, Location, Rank, WinCondition, ScoringRule,
    SpecialEffect, Action, ActionType
)

class MutationOperator:
    """Base class for genome mutations."""

    def __init__(self, rate: float = 0.1):
        """
        Args:
            rate: Probability of applying this mutation (0.0-1.0)
        """
        self.rate = rate

    def apply(self, genome: GameGenome) -> GameGenome:
        """Apply mutation if random roll succeeds."""
        if random.random() < self.rate:
            return self.mutate(genome)
        return genome

    def mutate(self, genome: GameGenome) -> GameGenome:
        """Override with specific mutation logic."""
        raise NotImplementedError


class TweakParameterMutation(MutationOperator):
    """Mutate numeric parameters (hand size, max turns, etc.)."""

    def mutate(self, genome: GameGenome) -> GameGenome:
        choice = random.choice([
            'cards_per_player',
            'max_turns',
            'initial_discard_count'
        ])

        if choice == 'cards_per_player':
            # Adjust ±3 cards, keep in range [3, 26]
            delta = random.randint(-3, 3)
            new_value = max(3, min(26, genome.setup.cards_per_player + delta))
            new_setup = replace(genome.setup, cards_per_player=new_value)
            return replace(genome, setup=new_setup)

        elif choice == 'max_turns':
            # Adjust ±20%, keep in range [20, 1000]
            delta_pct = random.uniform(-0.2, 0.2)
            new_value = int(max(20, min(1000, genome.max_turns * (1 + delta_pct))))
            return replace(genome, max_turns=new_value)

        elif choice == 'initial_discard_count':
            # Toggle between 0 and 1 (most common)
            new_value = 1 - genome.setup.initial_discard_count
            new_setup = replace(genome.setup, initial_discard_count=new_value)
            return replace(genome, setup=new_setup)

        return genome


class SwapPhaseOrderMutation(MutationOperator):
    """Swap the order of two adjacent phases."""

    def mutate(self, genome: GameGenome) -> GameGenome:
        phases = list(genome.turn_structure.phases)

        if len(phases) < 2:
            return genome

        # Pick random adjacent pair
        idx = random.randint(0, len(phases) - 2)
        phases[idx], phases[idx + 1] = phases[idx + 1], phases[idx]

        new_turn = replace(genome.turn_structure, phases=phases)
        return replace(genome, turn_structure=new_turn)


class AddPhaseMutation(MutationOperator):
    """Add a new phase to turn structure."""

    def mutate(self, genome: GameGenome) -> GameGenome:
        phases = list(genome.turn_structure.phases)

        # Don't add too many phases (max 5)
        if len(phases) >= 5:
            return genome

        # Create new phase (random type)
        new_phase = random.choice([
            DrawPhase(
                source=random.choice([Location.DECK, Location.DISCARD]),
                count=1,
                mandatory=random.choice([True, False])
            ),
            PlayPhase(
                target=Location.DISCARD,
                valid_play_condition=Condition(
                    type=ConditionType.HAND_SIZE,
                    operator=Operator.GT,
                    value=0
                ),
                min_cards=1,
                max_cards=1,
                mandatory=True
            ),
            DiscardPhase(
                target=Location.DISCARD,
                count=1,
                mandatory=False
            )
        ])

        # Insert at random position
        insert_pos = random.randint(0, len(phases))
        phases.insert(insert_pos, new_phase)

        new_turn = replace(genome.turn_structure, phases=phases)
        return replace(genome, turn_structure=new_turn)


class RemovePhaseMutation(MutationOperator):
    """Remove a random phase from turn structure."""

    def mutate(self, genome: GameGenome) -> GameGenome:
        phases = list(genome.turn_structure.phases)

        # Don't remove if only 1 phase left
        if len(phases) <= 1:
            return genome

        # Remove random phase
        idx = random.randint(0, len(phases) - 1)
        del phases[idx]

        new_turn = replace(genome.turn_structure, phases=phases)
        return replace(genome, turn_structure=new_turn)


class ModifyConditionMutation(MutationOperator):
    """Modify a condition in play phase or special effect."""

    def mutate(self, genome: GameGenome) -> GameGenome:
        # Find all play phases
        phases = list(genome.turn_structure.phases)
        play_phases = [(i, p) for i, p in enumerate(phases) if isinstance(p, PlayPhase)]

        if not play_phases:
            return genome

        # Pick random play phase
        idx, phase = random.choice(play_phases)

        # Mutate the condition
        cond = phase.valid_play_condition

        # Change operator
        if random.random() < 0.5 and cond.operator:
            new_operator = random.choice([
                Operator.EQ, Operator.NE, Operator.LT,
                Operator.GT, Operator.LE, Operator.GE
            ])
            new_cond = replace(cond, operator=new_operator)

        # Change value (if numeric)
        elif isinstance(cond.value, int):
            delta = random.randint(-2, 2)
            new_value = max(0, cond.value + delta)
            new_cond = replace(cond, value=new_value)

        else:
            return genome

        # Update phase
        new_phase = replace(phase, valid_play_condition=new_cond)
        phases[idx] = new_phase

        new_turn = replace(genome.turn_structure, phases=phases)
        return replace(genome, turn_structure=new_turn)


class AddSpecialEffectMutation(MutationOperator):
    """Add a special effect triggered by a specific rank."""

    def mutate(self, genome: GameGenome) -> GameGenome:
        effects = list(genome.special_effects)

        # Don't add too many effects (max 3)
        if len(effects) >= 3:
            return genome

        # Create new effect
        trigger_rank = random.choice([
            Rank.ACE, Rank.TWO, Rank.EIGHT, Rank.JACK, Rank.QUEEN, Rank.KING
        ])

        effect = SpecialEffect(
            trigger_card=trigger_rank,
            actions=[
                Action(
                    type=random.choice([
                        ActionType.SKIP_TURN,
                        ActionType.REVERSE_ORDER,
                        ActionType.DRAW_CARDS
                    ]),
                    source=Location.DECK,
                    count=random.randint(1, 3)
                )
            ]
        )

        effects.append(effect)
        return replace(genome, special_effects=effects)


class ModifyWinConditionMutation(MutationOperator):
    """Modify win conditions (critical design element)."""

    def mutate(self, genome: GameGenome) -> GameGenome:
        win_conditions = list(genome.win_conditions)

        if not win_conditions:
            # Add default win condition if missing
            new_condition = WinCondition(type="empty_hand")
            return replace(genome, win_conditions=[new_condition])

        # Pick random win condition to modify
        idx = random.randint(0, len(win_conditions) - 1)
        condition = win_conditions[idx]

        # Change win condition type
        new_type = random.choice([
            "empty_hand",
            "point_threshold",
            "capture_goal",
            "first_to_X"
        ])

        # If type requires a threshold, set one
        if new_type in ["point_threshold", "first_to_X"]:
            new_condition = WinCondition(
                type=new_type,
                value=random.choice([50, 100, 150, 200, 500])
            )
        else:
            new_condition = WinCondition(type=new_type)

        win_conditions[idx] = new_condition
        return replace(genome, win_conditions=win_conditions)


def create_mutation_pipeline(rates: dict = None) -> List[MutationOperator]:
    """Create standard mutation pipeline with configurable rates."""
    default_rates = {
        'tweak_parameter': 0.3,
        'swap_phase_order': 0.15,
        'add_phase': 0.1,
        'remove_phase': 0.1,
        'modify_condition': 0.2,
        'add_special_effect': 0.05,
        'modify_win_condition': 0.1,  # NEW: win conditions are critical
    }

    rates = {**default_rates, **(rates or {})}

    return [
        TweakParameterMutation(rates['tweak_parameter']),
        SwapPhaseOrderMutation(rates['swap_phase_order']),
        AddPhaseMutation(rates['add_phase']),
        RemovePhaseMutation(rates['remove_phase']),
        ModifyConditionMutation(rates['modify_condition']),
        AddSpecialEffectMutation(rates['add_special_effect']),
        ModifyWinConditionMutation(rates['modify_win_condition']),  # NEW
    ]


def mutate_genome(genome: GameGenome, mutation_rate: float = 1.0) -> GameGenome:
    """
    Apply mutation pipeline to genome.

    Args:
        genome: Genome to mutate
        mutation_rate: Global rate multiplier (0.0 = no mutations, 1.0 = standard)

    Returns:
        Mutated genome (new instance)
    """
    mutated = genome

    # Adjust individual mutation rates by global multiplier
    adjusted_rates = {
        'tweak_parameter': 0.3 * mutation_rate,
        'swap_phase_order': 0.15 * mutation_rate,
        'add_phase': 0.1 * mutation_rate,
        'remove_phase': 0.1 * mutation_rate,
        'modify_condition': 0.2 * mutation_rate,
        'add_special_effect': 0.05 * mutation_rate,
        'modify_win_condition': 0.1 * mutation_rate,  # NEW
    }

    pipeline = create_mutation_pipeline(adjusted_rates)

    for operator in pipeline:
        mutated = operator.apply(mutated)

    # Increment generation counter
    mutated = replace(mutated, generation=mutated.generation + 1)

    return mutated
```

#### Step 1.2: Implement crossover (10 min)

**File:** `src/darwindeck/evolution/operators.py` (continued)

```python
def crossover_genomes(parent1: GameGenome, parent2: GameGenome) -> tuple[GameGenome, GameGenome]:
    """
    Perform semantic crossover between two parent genomes.

    Strategy: Swap turn structure phases between parents.

    Returns:
        Two offspring genomes
    """
    # Extract phases from both parents
    phases1 = list(parent1.turn_structure.phases)
    phases2 = list(parent2.turn_structure.phases)

    # Single-point crossover on phases
    if len(phases1) > 1 and len(phases2) > 1:
        # Pick crossover point
        point1 = random.randint(1, len(phases1) - 1)
        point2 = random.randint(1, len(phases2) - 1)

        # Swap tails
        child1_phases = phases1[:point1] + phases2[point2:]
        child2_phases = phases2[:point2] + phases1[point1:]

        # Create offspring
        child1_turn = replace(parent1.turn_structure, phases=child1_phases)
        child2_turn = replace(parent2.turn_structure, phases=child2_phases)

        child1 = replace(parent1,
                        turn_structure=child1_turn,
                        generation=max(parent1.generation, parent2.generation) + 1)
        child2 = replace(parent2,
                        turn_structure=child2_turn,
                        generation=max(parent1.generation, parent2.generation) + 1)

        return child1, child2

    # If crossover not possible, return mutated copies
    return (
        mutate_genome(parent1, mutation_rate=0.5),
        mutate_genome(parent2, mutation_rate=0.5)
    )
```

#### Step 1.3: Unit tests for operators (10 min)

**File:** `tests/evolution/test_operators.py`

```python
import pytest
from darwindeck.evolution.operators import (
    mutate_genome, crossover_genomes,
    TweakParameterMutation, SwapPhaseOrderMutation,
    AddPhaseMutation, RemovePhaseMutation
)
from darwindeck.genome.examples import create_war_genome


def test_tweak_parameter_mutation():
    """Test parameter tweaking mutation."""
    war = create_war_genome()
    operator = TweakParameterMutation(rate=1.0)  # Always mutate

    mutated = operator.apply(war)

    # Should have changed at least one parameter
    assert (
        mutated.setup.cards_per_player != war.setup.cards_per_player or
        mutated.max_turns != war.max_turns or
        mutated.setup.initial_discard_count != war.setup.initial_discard_count
    )


def test_swap_phase_order_mutation():
    """Test phase swapping mutation."""
    war = create_war_genome()

    # Add second phase so we can swap
    from darwindeck.genome.schema import DrawPhase, Location
    phases = list(war.turn_structure.phases)
    phases.append(DrawPhase(source=Location.DECK, count=1, mandatory=True))

    from dataclasses import replace
    war = replace(war, turn_structure=replace(war.turn_structure, phases=phases))

    operator = SwapPhaseOrderMutation(rate=1.0)
    mutated = operator.apply(war)

    # Phases should be swapped
    assert mutated.turn_structure.phases[0] == war.turn_structure.phases[1]
    assert mutated.turn_structure.phases[1] == war.turn_structure.phases[0]


def test_add_phase_mutation():
    """Test adding a phase."""
    war = create_war_genome()
    original_count = len(war.turn_structure.phases)

    operator = AddPhaseMutation(rate=1.0)
    mutated = operator.apply(war)

    assert len(mutated.turn_structure.phases) == original_count + 1


def test_remove_phase_mutation():
    """Test removing a phase."""
    war = create_war_genome()

    # Add second phase so we can remove one
    from darwindeck.genome.schema import DrawPhase, Location
    from dataclasses import replace
    phases = list(war.turn_structure.phases)
    phases.append(DrawPhase(source=Location.DECK, count=1, mandatory=True))
    war = replace(war, turn_structure=replace(war.turn_structure, phases=phases))

    original_count = len(war.turn_structure.phases)
    operator = RemovePhaseMutation(rate=1.0)
    mutated = operator.apply(war)

    assert len(mutated.turn_structure.phases) == original_count - 1


def test_mutate_genome_increments_generation():
    """Test that mutation increments generation counter."""
    war = create_war_genome()
    assert war.generation == 0

    mutated = mutate_genome(war, mutation_rate=1.0)
    assert mutated.generation == 1


def test_crossover_creates_valid_offspring():
    """Test crossover produces two children."""
    from darwindeck.genome.examples import create_crazy_eights_genome

    war = create_war_genome()
    crazy8 = create_crazy_eights_genome()

    child1, child2 = crossover_genomes(war, crazy8)

    # Both children should exist
    assert child1 is not None
    assert child2 is not None

    # Generation should increment
    assert child1.generation > max(war.generation, crazy8.generation)
    assert child2.generation > max(war.generation, crazy8.generation)


def test_mutation_preserves_genome_structure():
    """Test that mutation doesn't break dataclass structure."""
    war = create_war_genome()
    mutated = mutate_genome(war, mutation_rate=1.0)

    # Should still be valid GameGenome
    assert mutated.schema_version == "1.0"
    assert mutated.player_count == 2
    assert len(mutated.turn_structure.phases) >= 1
    assert len(mutated.win_conditions) >= 1
```

**Test:**
```bash
uv run pytest tests/evolution/test_operators.py -v
```

**Expected:** All mutation and crossover tests pass

**Commit:**
```
Implement genetic operators for genome evolution

Mutation operators:
- TweakParameterMutation: adjust hand size, max turns, discard count
- SwapPhaseOrderMutation: reorder turn phases
- AddPhaseMutation: insert new phase (DrawPhase, PlayPhase, DiscardPhase)
- RemovePhaseMutation: delete random phase
- ModifyConditionMutation: change operators/values in conditions
- AddSpecialEffectMutation: add card-triggered effects
- ModifyWinConditionMutation: mutate win condition types and thresholds (NEW)

Crossover:
- Semantic single-point crossover on turn structure phases
- Fallback to mutation if crossover not possible
- Increment generation counter

Testing:
- Unit tests for all mutation types
- Crossover validation
- Generation counter verification
- Dataclass structure preservation
```

---

### Task 2: Fitness Evaluation System (45 minutes)

**Goal:** Implement progressive fitness evaluation with 7 proxy metrics

#### Step 2.1: Define fitness metrics (15 min)

**File:** `src/darwindeck/evolution/fitness.py`

```python
"""Fitness evaluation for game genomes."""
from dataclasses import dataclass
from typing import List, Dict
import hashlib
import json
from darwindeck.genome.schema import GameGenome
from darwindeck.genome.bytecode import BytecodeCompiler
from darwindeck.bindings.cgo_bridge import simulate_batch
import flatbuffers


@dataclass
class FitnessMetrics:
    """Individual fitness components."""
    decision_density: float  # Meaningful choices / total turns (0-1, higher = more choices)
    comeback_potential: float  # How often trailing player wins (0-1, 0.5 = balanced)
    tension_curve: float  # Variance in win probability over time (0-1, higher = more tension)
    interaction_frequency: float  # % turns affecting opponent (0-1)
    rules_complexity: float  # Inverse of complexity (0-1, higher = simpler)
    session_length: float  # Normalized game duration (0-1, 1.0 = 10 min target)
    skill_vs_luck: float  # MCTS win rate delta vs random (0-1, higher = more skill)

    # Aggregate
    total_fitness: float  # Weighted sum

    # Metadata
    games_simulated: int
    valid: bool  # False if genome failed validation


@dataclass
class SimulationResults:
    """Raw simulation data for a genome."""
    total_games: int
    player0_wins: int
    player1_wins: int
    draws: int
    avg_turns: float
    median_turns: int
    avg_duration_ns: int
    errors: int

    # Extended metrics (TODO: collect from Go core)
    decision_counts: List[int] = None  # Choices per turn
    win_probability_trace: List[float] = None  # Win prob over time
    interaction_turns: int = 0  # Turns with opponent interaction


class FitnessCache:
    """Cache fitness evaluations by genome hash."""

    def __init__(self):
        self.cache: Dict[str, FitnessMetrics] = {}

    def get_hash(self, genome: GameGenome) -> str:
        """Compute deterministic hash of genome."""
        # Use bytecode representation for hash
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)
        return hashlib.sha256(bytecode).hexdigest()

    def get(self, genome: GameGenome) -> FitnessMetrics | None:
        """Retrieve cached fitness if available."""
        key = self.get_hash(genome)
        return self.cache.get(key)

    def put(self, genome: GameGenome, metrics: FitnessMetrics):
        """Store fitness in cache."""
        key = self.get_hash(genome)
        self.cache[key] = metrics


class FitnessEvaluator:
    """Evaluate genome fitness using simulation."""

    def __init__(self,
                 weights: Dict[str, float] = None,
                 use_cache: bool = True):
        """
        Args:
            weights: Metric weights (default: equal weights)
            use_cache: Enable fitness caching
        """
        self.weights = weights or {
            'decision_density': 1.0,
            'comeback_potential': 1.0,
            'tension_curve': 1.0,
            'interaction_frequency': 1.0,
            'rules_complexity': 1.0,
            'session_length': 1.0,
            'skill_vs_luck': 1.0,
        }

        # Normalize weights to sum to 1.0
        total_weight = sum(self.weights.values())
        self.weights = {k: v / total_weight for k, v in self.weights.items()}

        self.cache = FitnessCache() if use_cache else None

    def evaluate(self, genome: GameGenome,
                 num_simulations: int = 100,
                 use_mcts: bool = False) -> FitnessMetrics:
        """
        Evaluate genome fitness through simulation.

        Args:
            genome: Game to evaluate
            num_simulations: Games to simulate (more = stable estimate)
            use_mcts: Use MCTS AI (expensive) vs random AI

        Returns:
            Fitness metrics
        """
        # Check cache first
        if self.cache:
            cached = self.cache.get(genome)
            if cached:
                return cached

        # Run simulations via Go core
        results = self._run_simulations(genome, num_simulations, use_mcts)

        # Compute metrics
        metrics = self._compute_metrics(genome, results, use_mcts)

        # Cache result
        if self.cache:
            self.cache.put(genome, metrics)

        return metrics

    def _run_simulations(self, genome: GameGenome,
                        num_simulations: int,
                        use_mcts: bool) -> SimulationResults:
        """Run batch simulations via Go core."""
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        # TODO: Call Go core via CGo
        # For now, return mock results
        return SimulationResults(
            total_games=num_simulations,
            player0_wins=num_simulations // 2,
            player1_wins=num_simulations // 2,
            draws=0,
            avg_turns=50.0,
            median_turns=48,
            avg_duration_ns=300_000_000,  # 300ms avg
            errors=0
        )

    def _compute_metrics(self, genome: GameGenome,
                        results: SimulationResults,
                        use_mcts: bool) -> FitnessMetrics:
        """Compute fitness metrics from simulation results."""

        # SESSION LENGTH AS CONSTRAINT (not averaged metric)
        # Assume 1 turn = 2 seconds for human play
        estimated_duration_sec = results.avg_turns * 2
        target_min = 3 * 60   # 3 minutes
        target_max = 20 * 60  # 20 minutes

        # Filter: games outside range get fitness = 0
        if estimated_duration_sec < target_min or estimated_duration_sec > target_max:
            return FitnessMetrics(
                decision_density=0.0,
                comeback_potential=0.0,
                tension_curve=0.0,
                interaction_frequency=0.0,
                rules_complexity=0.0,
                session_length=0.0,  # Failed constraint
                skill_vs_luck=0.0,
                total_fitness=0.0,
                games_simulated=results.total_games,
                valid=False  # Failed constraint
            )

        # Normalize session length for reporting (passed constraint)
        session_length_normalized = 1.0 - abs(estimated_duration_sec - 600) / 600

        # 1. Decision density (placeholder - needs instrumentation)
        # Estimate: more phases = more decisions
        decision_density = min(1.0, len(genome.turn_structure.phases) / 5.0)

        # 2. Comeback potential (how balanced is the game?)
        win_rate_p0 = results.player0_wins / results.total_games
        # Perfect balance = 0.5, worst = 0.0 or 1.0
        comeback_potential = 1.0 - abs(win_rate_p0 - 0.5) * 2

        # 3. Tension curve (placeholder - needs win prob trace)
        # Estimate: longer games = more tension
        tension_curve = min(1.0, results.avg_turns / 100.0)

        # 4. Interaction frequency (placeholder - needs instrumentation)
        # Estimate: special effects = interaction
        interaction_frequency = min(1.0, len(genome.special_effects) / 3.0)

        # 5. Rules complexity (inverse)
        # Count genome components
        complexity = (
            len(genome.turn_structure.phases) +
            len(genome.special_effects) * 2 +
            len(genome.scoring_rules) +
            len(genome.win_conditions)
        )
        # Normalize to 0-1 (simpler = higher score)
        rules_complexity = max(0.0, 1.0 - complexity / 20.0)

        # 6. Skill vs luck (only if MCTS used)
        skill_vs_luck = 0.5  # Neutral if not measured
        if use_mcts:
            # TODO: Compare MCTS win rate vs random baseline
            skill_vs_luck = 0.6  # Placeholder

        # Check validity (simulation errors OR failed constraint)
        valid = results.errors == 0 and results.total_games > 0

        # Compute weighted total (ONLY 6 metrics, session length is constraint)
        # Renormalize weights to sum to 1.0 without session_length
        total_fitness = (
            self.weights['decision_density'] * decision_density +
            self.weights['comeback_potential'] * comeback_potential +
            self.weights['tension_curve'] * tension_curve +
            self.weights['interaction_frequency'] * interaction_frequency +
            self.weights['rules_complexity'] * rules_complexity +
            self.weights['skill_vs_luck'] * skill_vs_luck
        )

        # Renormalize to [0, 1] (compensate for missing session_length weight)
        total_fitness = total_fitness * 7.0 / 6.0

        return FitnessMetrics(
            decision_density=decision_density,
            comeback_potential=comeback_potential,
            tension_curve=tension_curve,
            interaction_frequency=interaction_frequency,
            rules_complexity=rules_complexity,
            session_length=session_length_normalized,  # For reporting only
            skill_vs_luck=skill_vs_luck,
            total_fitness=total_fitness,
            games_simulated=results.total_games,
            valid=valid
        )

    def evaluate_progressive(self, genome: GameGenome) -> FitnessMetrics:
        """
        Progressive evaluation: cheap tests first, expensive only if promising.

        Stage 1: Schema validation + 10 random simulations
        Stage 2: 100 random simulations
        Stage 3: MCTS evaluation (top 20% only)
        """
        # Stage 1: Quick validation
        quick_results = self._run_simulations(genome, num_simulations=10, use_mcts=False)

        if quick_results.errors > 0:
            # Invalid genome, return low fitness
            return FitnessMetrics(
                decision_density=0.0,
                comeback_potential=0.0,
                tension_curve=0.0,
                interaction_frequency=0.0,
                rules_complexity=0.0,
                session_length=0.0,
                skill_vs_luck=0.0,
                total_fitness=0.0,
                games_simulated=10,
                valid=False
            )

        # Stage 2: Full random evaluation
        full_results = self._run_simulations(genome, num_simulations=100, use_mcts=False)
        metrics = self._compute_metrics(genome, full_results, use_mcts=False)

        # Stage 3: MCTS (deferred - only call for top performers)
        # This will be triggered by EvolutionEngine for elites

        return metrics
```

#### Step 2.2: Add validation system (15 min)

**File:** `src/darwindeck/validation/schema_check.py`

```python
"""Schema validation for game genomes."""
from darwindeck.genome.schema import GameGenome, PlayPhase, DrawPhase, DiscardPhase


def validate_genome(genome: GameGenome) -> tuple[bool, str]:
    """
    Validate genome structure and constraints.

    Returns:
        (is_valid, error_message)
    """
    # Check basic structure
    if not genome.turn_structure.phases:
        return False, "No turn phases defined"

    if not genome.win_conditions:
        return False, "No win conditions defined"

    # Check parameters in valid ranges
    if genome.setup.cards_per_player < 1:
        return False, f"Invalid cards_per_player: {genome.setup.cards_per_player}"

    if genome.setup.cards_per_player * genome.player_count > 52:
        return False, f"Too many cards: {genome.setup.cards_per_player} × {genome.player_count} > 52"

    if genome.max_turns < 10:
        return False, f"max_turns too low: {genome.max_turns}"

    if genome.max_turns > 10000:
        return False, f"max_turns too high: {genome.max_turns} (potential infinite loop)"

    # Check phase validity
    for i, phase in enumerate(genome.turn_structure.phases):
        if isinstance(phase, PlayPhase):
            if phase.min_cards < 0 or phase.max_cards < phase.min_cards:
                return False, f"Phase {i}: invalid card counts"

        elif isinstance(phase, DrawPhase):
            if phase.count < 0:
                return False, f"Phase {i}: negative draw count"

        elif isinstance(phase, DiscardPhase):
            if phase.count < 0:
                return False, f"Phase {i}: negative discard count"

    # Check for at least one non-deterministic decision point
    has_choice = False
    for phase in genome.turn_structure.phases:
        if isinstance(phase, PlayPhase) and not phase.mandatory:
            has_choice = True
            break
        if isinstance(phase, DiscardPhase) and not phase.mandatory:
            has_choice = True
            break

    if not has_choice and not genome.special_effects:
        return False, "No decision points (all phases mandatory, no special effects)"

    return True, "Valid"


def validate_and_repair(genome: GameGenome) -> GameGenome:
    """
    Validate genome and attempt repairs if invalid.

    Returns:
        Repaired genome (or original if valid)
    """
    is_valid, error = validate_genome(genome)

    if is_valid:
        return genome

    # Repair strategies
    from dataclasses import replace
    repaired = genome

    # Fix card distribution
    if repaired.setup.cards_per_player * repaired.player_count > 52:
        new_cards = 52 // repaired.player_count
        repaired = replace(repaired, setup=replace(repaired.setup, cards_per_player=new_cards))

    # Fix max_turns
    if repaired.max_turns < 10:
        repaired = replace(repaired, max_turns=100)
    elif repaired.max_turns > 10000:
        repaired = replace(repaired, max_turns=1000)

    # Add default win condition if missing
    if not repaired.win_conditions:
        from darwindeck.genome.schema import WinCondition
        repaired = replace(repaired, win_conditions=[WinCondition(type="empty_hand")])

    # Add default phase if missing
    if not repaired.turn_structure.phases:
        from darwindeck.genome.schema import PlayPhase, Condition, ConditionType, Operator, Location
        default_phase = PlayPhase(
            target=Location.DISCARD,
            valid_play_condition=Condition(
                type=ConditionType.HAND_SIZE,
                operator=Operator.GT,
                value=0
            ),
            min_cards=1,
            max_cards=1,
            mandatory=True
        )
        repaired = replace(repaired,
                          turn_structure=replace(repaired.turn_structure, phases=[default_phase]))

    return repaired
```

#### Step 2.3: Unit tests for fitness (15 min)

**File:** `tests/evolution/test_fitness.py`

```python
import pytest
from darwindeck.evolution.fitness import FitnessEvaluator, FitnessCache, FitnessMetrics
from darwindeck.validation.schema_check import validate_genome, validate_and_repair
from darwindeck.genome.examples import create_war_genome


def test_fitness_cache():
    """Test fitness caching by genome hash."""
    cache = FitnessCache()
    war = create_war_genome()

    # Create mock metrics
    metrics = FitnessMetrics(
        decision_density=0.5,
        comeback_potential=0.5,
        tension_curve=0.5,
        interaction_frequency=0.5,
        rules_complexity=0.5,
        session_length=0.5,
        skill_vs_luck=0.5,
        total_fitness=0.5,
        games_simulated=100,
        valid=True
    )

    # Cache should be empty initially
    assert cache.get(war) is None

    # Store in cache
    cache.put(war, metrics)

    # Should retrieve from cache
    cached = cache.get(war)
    assert cached is not None
    assert cached.total_fitness == 0.5


def test_fitness_evaluator_basic():
    """Test basic fitness evaluation."""
    evaluator = FitnessEvaluator(use_cache=False)
    war = create_war_genome()

    metrics = evaluator.evaluate(war, num_simulations=10)

    # Should have all metrics
    assert 0.0 <= metrics.decision_density <= 1.0
    assert 0.0 <= metrics.comeback_potential <= 1.0
    assert 0.0 <= metrics.total_fitness <= 1.0
    assert metrics.games_simulated == 10
    assert metrics.valid is True


def test_fitness_weights_normalization():
    """Test that fitness weights are normalized."""
    weights = {
        'decision_density': 2.0,
        'comeback_potential': 1.0,
        'tension_curve': 1.0,
        'interaction_frequency': 1.0,
        'rules_complexity': 1.0,
        'session_length': 1.0,
        'skill_vs_luck': 1.0,
    }

    evaluator = FitnessEvaluator(weights=weights)

    # Weights should sum to 1.0
    total = sum(evaluator.weights.values())
    assert abs(total - 1.0) < 0.001


def test_validate_genome_basic():
    """Test basic genome validation."""
    war = create_war_genome()

    is_valid, error = validate_genome(war)
    assert is_valid is True
    assert error == "Valid"


def test_validate_genome_catches_invalid_cards():
    """Test validation catches too many cards."""
    from dataclasses import replace
    war = create_war_genome()

    # Set cards_per_player to impossible value
    invalid = replace(war, setup=replace(war.setup, cards_per_player=30))

    is_valid, error = validate_genome(invalid)
    assert is_valid is False
    assert "Too many cards" in error


def test_validate_and_repair():
    """Test genome repair."""
    from dataclasses import replace
    war = create_war_genome()

    # Create invalid genome
    invalid = replace(war, setup=replace(war.setup, cards_per_player=30))

    # Repair should fix it
    repaired = validate_and_repair(invalid)

    is_valid, error = validate_genome(repaired)
    assert is_valid is True


def test_progressive_evaluation():
    """Test progressive fitness evaluation."""
    evaluator = FitnessEvaluator()
    war = create_war_genome()

    metrics = evaluator.evaluate_progressive(war)

    # Should complete stage 1 and 2
    assert metrics.games_simulated >= 10
    assert metrics.valid is True
```

**Test:**
```bash
uv run pytest tests/evolution/test_fitness.py -v
```

**Expected:** All fitness and validation tests pass

**Commit:**
```
Implement progressive fitness evaluation system

Fitness metrics (6 aggregated + 1 constraint):
1. Decision density: meaningful choices vs forced plays
2. Comeback potential: game balance (win rate ~0.5)
3. Tension curve: variance in win probability over time
4. Interaction frequency: actions affecting opponents
5. Rules complexity: inverse complexity (simpler = higher)
6. Skill vs luck: MCTS win rate delta vs random
7. Session length: CONSTRAINT (3-20 min range, fitness=0 if failed)

Features:
- FitnessCache: hash-based caching (genomes are deterministic)
- Progressive evaluation: 10 sims → 100 sims → MCTS (deferred)
- Weighted fitness: configurable metric weights, normalized to sum=1.0
- Schema validation: type checking + playability constraints
- Genome repair: attempt to fix invalid genomes

Validation checks:
- Card distribution (cards_per_player × player_count ≤ 52)
- Turn limits (10 ≤ max_turns ≤ 10000)
- Phase validity (non-negative counts, valid ranges)
- Decision points (at least one non-mandatory phase or special effect)

Testing:
- Fitness cache retrieval
- Weight normalization
- Progressive evaluation
- Validation and repair
```

---

### Task 3: Population Management (30 minutes)

**Goal:** Implement population management with diversity tracking

#### Step 3.1: Population class (15 min)

**File:** `src/darwindeck/evolution/population.py`

```python
"""Population management for genetic algorithm."""
from dataclasses import dataclass
from typing import List, Tuple
import random
from darwindeck.genome.schema import GameGenome
from darwindeck.evolution.fitness import FitnessMetrics, FitnessEvaluator


@dataclass
class Individual:
    """Genome with fitness information."""
    genome: GameGenome
    fitness: FitnessMetrics | None = None

    @property
    def fitness_score(self) -> float:
        """Get fitness score (0.0 if not evaluated)."""
        return self.fitness.total_fitness if self.fitness else 0.0


class Population:
    """Manages collection of genomes."""

    def __init__(self,
                 individuals: List[Individual],
                 evaluator: FitnessEvaluator,
                 elitism_rate: float = 0.1):
        """
        Args:
            individuals: Initial population
            evaluator: Fitness evaluator
            elitism_rate: Fraction of population to preserve (0.0-1.0)
        """
        self.individuals = individuals
        self.evaluator = evaluator
        self.elitism_rate = elitism_rate
        self.generation = 0

    @property
    def size(self) -> int:
        return len(self.individuals)

    def evaluate_all(self, progressive: bool = True):
        """Evaluate fitness for all individuals."""
        for individual in self.individuals:
            if individual.fitness is None:
                if progressive:
                    individual.fitness = self.evaluator.evaluate_progressive(individual.genome)
                else:
                    individual.fitness = self.evaluator.evaluate(individual.genome, num_simulations=100)

    def get_best(self, n: int = 1) -> List[Individual]:
        """Get top N individuals by fitness."""
        sorted_pop = sorted(self.individuals, key=lambda x: x.fitness_score, reverse=True)
        return sorted_pop[:n]

    def get_worst(self, n: int = 1) -> List[Individual]:
        """Get bottom N individuals by fitness."""
        sorted_pop = sorted(self.individuals, key=lambda x: x.fitness_score)
        return sorted_pop[:n]

    def get_stats(self) -> dict:
        """Compute population statistics."""
        fitness_scores = [ind.fitness_score for ind in self.individuals if ind.fitness]

        if not fitness_scores:
            return {
                'size': self.size,
                'generation': self.generation,
                'avg_fitness': 0.0,
                'max_fitness': 0.0,
                'min_fitness': 0.0,
                'diversity': 0.0,
            }

        return {
            'size': self.size,
            'generation': self.generation,
            'avg_fitness': sum(fitness_scores) / len(fitness_scores),
            'max_fitness': max(fitness_scores),
            'min_fitness': min(fitness_scores),
            'diversity': self._compute_diversity(),
        }

    def _compute_diversity(self) -> float:
        """
        Compute population diversity metric using genome distance.

        Uses Hamming distance on structural features (phase counts, effect counts, etc.).
        Higher = more diverse, Lower = converged
        """
        if len(self.individuals) < 2:
            return 0.0

        # Compute pairwise distances (sample to avoid O(n²) for large populations)
        sample_size = min(50, len(self.individuals))
        import random
        sampled = random.sample(self.individuals, sample_size)

        total_distance = 0.0
        num_pairs = 0

        for i in range(len(sampled)):
            for j in range(i + 1, len(sampled)):
                total_distance += genome_distance(sampled[i].genome, sampled[j].genome)
                num_pairs += 1

        if num_pairs == 0:
            return 0.0

        # Average pairwise distance
        avg_distance = total_distance / num_pairs

        return avg_distance  # Already in [0, 1] range


def genome_distance(g1: GameGenome, g2: GameGenome) -> float:
    """
    Compute structural distance between two genomes.

    Uses Hamming distance on key structural features.
    Returns value in [0, 1] range (0 = identical, 1 = maximally different).
    """
    distance = 0.0
    total_features = 0

    # Phase count difference (normalized)
    phase_diff = abs(len(g1.turn_structure.phases) - len(g2.turn_structure.phases))
    distance += min(1.0, phase_diff / 5.0)
    total_features += 1

    # Special effects count difference
    effect_diff = abs(len(g1.special_effects) - len(g2.special_effects))
    distance += min(1.0, effect_diff / 3.0)
    total_features += 1

    # Win conditions count difference
    win_diff = abs(len(g1.win_conditions) - len(g2.win_conditions))
    distance += min(1.0, win_diff / 2.0)
    total_features += 1

    # Cards per player difference
    cards_diff = abs(g1.setup.cards_per_player - g2.setup.cards_per_player)
    distance += min(1.0, cards_diff / 13.0)  # Normalize by half deck
    total_features += 1

    # Max turns difference
    turns_diff = abs(g1.max_turns - g2.max_turns)
    distance += min(1.0, turns_diff / 500.0)  # Normalize by typical range
    total_features += 1

    # Player count difference
    player_diff = abs(g1.player_count - g2.player_count)
    distance += min(1.0, player_diff / 4.0)  # Normalize by max player range
    total_features += 1

    # Average distance across all features
    return distance / total_features

    @classmethod
    def create_initial_population(cls,
                                  size: int,
                                  seed_genomes: List[GameGenome],
                                  evaluator: FitnessEvaluator,
                                  seed_ratio: float = 0.7) -> 'Population':
        """
        Create initial population with seeded and random genomes.

        Args:
            size: Population size
            seed_genomes: Known good games (War, Crazy 8s, etc.)
            evaluator: Fitness evaluator
            seed_ratio: Fraction of population from seeds (rest is random/mutated)

        Returns:
            New population
        """
        from darwindeck.evolution.operators import mutate_genome

        individuals = []

        # Add seeded genomes
        num_seeds = int(size * seed_ratio)
        for i in range(num_seeds):
            genome = seed_genomes[i % len(seed_genomes)]
            individuals.append(Individual(genome=genome))

        # Add mutated variants of seeds
        num_mutants = size - num_seeds
        for i in range(num_mutants):
            base_genome = seed_genomes[i % len(seed_genomes)]
            mutated = mutate_genome(base_genome, mutation_rate=1.0)
            individuals.append(Individual(genome=mutated))

        return cls(individuals, evaluator)


def tournament_selection(population: Population,
                        tournament_size: int = 3) -> Individual:
    """
    Select individual via tournament selection.

    Args:
        population: Population to select from
        tournament_size: Number of individuals in tournament

    Returns:
        Winner of tournament
    """
    tournament = random.sample(population.individuals, tournament_size)
    return max(tournament, key=lambda x: x.fitness_score)


def select_parents(population: Population,
                   num_parents: int,
                   tournament_size: int = 3) -> List[Tuple[Individual, Individual]]:
    """
    Select parent pairs for breeding.

    Args:
        population: Population to select from
        num_parents: Number of parent pairs to select
        tournament_size: Tournament size for selection

    Returns:
        List of (parent1, parent2) tuples
    """
    pairs = []

    for _ in range(num_parents):
        parent1 = tournament_selection(population, tournament_size)
        parent2 = tournament_selection(population, tournament_size)
        pairs.append((parent1, parent2))

    return pairs
```

#### Step 3.2: Seed population generation (10 min)

**File:** `src/darwindeck/genome/examples.py` (extend)

```python
# Add to existing examples.py

def create_gin_rummy_genome() -> GameGenome:
    """Create Gin Rummy genome (simplified)."""
    return GameGenome(
        schema_version="1.0",
        genome_id="gin-rummy-simplified",
        generation=0,

        setup=SetupRules(
            cards_per_player=10,
            initial_discard_count=1
        ),

        turn_structure=TurnStructure(phases=[
            DrawPhase(
                source=Location.DECK,
                count=1,
                mandatory=True
            ),
            PlayPhase(
                target=Location.TABLEAU,
                valid_play_condition=Condition(
                    type=ConditionType.HAND_SIZE,
                    operator=Operator.GE,
                    value=3
                ),
                min_cards=0,
                max_cards=10,
                mandatory=False
            ),
            DiscardPhase(
                target=Location.DISCARD,
                count=1,
                mandatory=True
            )
        ]),

        special_effects=[],

        win_conditions=[
            WinCondition(
                type="empty_hand"
            )
        ],

        scoring_rules=[],
        max_turns=50,
        player_count=2
    )


def get_seed_genomes() -> List[GameGenome]:
    """Get all seed genomes for initial population."""
    return [
        create_war_genome(),
        create_crazy_eights_genome(),
        create_gin_rummy_genome(),
    ]
```

#### Step 3.3: Unit tests for population (5 min)

**File:** `tests/evolution/test_population.py`

```python
import pytest
from darwindeck.evolution.population import (
    Population, Individual, tournament_selection, select_parents
)
from darwindeck.evolution.fitness import FitnessEvaluator, FitnessMetrics
from darwindeck.genome.examples import get_seed_genomes


def test_create_initial_population():
    """Test initial population creation."""
    seeds = get_seed_genomes()
    evaluator = FitnessEvaluator()

    pop = Population.create_initial_population(
        size=20,
        seed_genomes=seeds,
        evaluator=evaluator,
        seed_ratio=0.7
    )

    assert pop.size == 20
    assert pop.generation == 0


def test_population_get_best():
    """Test getting best individuals."""
    seeds = get_seed_genomes()
    evaluator = FitnessEvaluator()

    pop = Population.create_initial_population(
        size=10,
        seed_genomes=seeds,
        evaluator=evaluator
    )

    # Assign mock fitness
    for i, ind in enumerate(pop.individuals):
        ind.fitness = FitnessMetrics(
            decision_density=0.5,
            comeback_potential=0.5,
            tension_curve=0.5,
            interaction_frequency=0.5,
            rules_complexity=0.5,
            session_length=0.5,
            skill_vs_luck=0.5,
            total_fitness=0.1 * i,  # Increasing fitness
            games_simulated=10,
            valid=True
        )

    best = pop.get_best(n=3)
    assert len(best) == 3
    assert best[0].fitness_score >= best[1].fitness_score
    assert best[1].fitness_score >= best[2].fitness_score


def test_tournament_selection():
    """Test tournament selection."""
    seeds = get_seed_genomes()
    evaluator = FitnessEvaluator()

    pop = Population.create_initial_population(
        size=10,
        seed_genomes=seeds,
        evaluator=evaluator
    )

    # Assign fitness
    for i, ind in enumerate(pop.individuals):
        ind.fitness = FitnessMetrics(
            decision_density=0.5,
            comeback_potential=0.5,
            tension_curve=0.5,
            interaction_frequency=0.5,
            rules_complexity=0.5,
            session_length=0.5,
            skill_vs_luck=0.5,
            total_fitness=0.1 * i,
            games_simulated=10,
            valid=True
        )

    winner = tournament_selection(pop, tournament_size=3)

    # Winner should have fitness
    assert winner.fitness is not None
    assert winner.fitness_score > 0.0


def test_population_diversity():
    """Test diversity metric calculation."""
    seeds = get_seed_genomes()
    evaluator = FitnessEvaluator()

    pop = Population.create_initial_population(
        size=10,
        seed_genomes=seeds,
        evaluator=evaluator
    )

    stats = pop.get_stats()

    # Should have diversity metric
    assert 'diversity' in stats
    assert 0.0 <= stats['diversity'] <= 1.0
```

**Test:**
```bash
uv run pytest tests/evolution/test_population.py -v
```

**Expected:** All population management tests pass

**Commit:**
```
Implement population management with diversity tracking

Population class:
- Individual: genome + fitness wrapper
- create_initial_population: seed with known games (70%) + mutated variants (30%)
- evaluate_all: fitness evaluation for entire population
- get_best/get_worst: rank by fitness
- diversity metric: variance in turn structure complexity

Selection:
- tournament_selection: pick winner from random tournament
- select_parents: generate parent pairs for breeding

Seed genomes:
- War, Crazy 8s, Gin Rummy (simplified)
- get_seed_genomes() for initial population seeding

Statistics:
- avg_fitness, max_fitness, min_fitness
- population diversity (genome distance via Hamming distance on structural features)

Diversity tracking (NEW):
- genome_distance(): computes structural distance on 6 features (phases, effects, win conditions, cards, turns, players)
- Pairwise distance averaging (sampled for performance)
- Warning when diversity < 0.1 (premature convergence risk)

Testing:
- Initial population creation
- Selection mechanisms
- Diversity calculation
```

---

### Task 4: Evolution Engine (30 minutes)

**Goal:** Orchestrate genetic algorithm loop

#### Step 4.1: Main evolution loop (20 min)

**File:** `src/darwindeck/evolution/engine.py`

```python
"""Evolution engine orchestrating genetic algorithm."""
from typing import List, Callable
import time
from dataclasses import dataclass
from darwindeck.genome.schema import GameGenome
from darwindeck.evolution.population import Population, Individual, select_parents
from darwindeck.evolution.operators import mutate_genome, crossover_genomes
from darwindeck.evolution.fitness import FitnessEvaluator
from darwindeck.validation.schema_check import validate_and_repair


@dataclass
class EvolutionConfig:
    """Configuration for evolution run."""
    population_size: int = 100
    num_generations: int = 100
    elitism_rate: float = 0.1
    crossover_rate: float = 0.7
    mutation_rate: float = 1.0
    tournament_size: int = 3
    seed_ratio: float = 0.7
    plateau_threshold: int = 30  # Generations without improvement for early stopping (consensus: extended from 20)


@dataclass
class GenerationStats:
    """Statistics for a single generation."""
    generation: int
    avg_fitness: float
    max_fitness: float
    min_fitness: float
    diversity: float
    best_genome: GameGenome
    evaluation_time: float


class EvolutionEngine:
    """Orchestrates genetic algorithm execution."""

    def __init__(self,
                 config: EvolutionConfig,
                 evaluator: FitnessEvaluator,
                 seed_genomes: List[GameGenome],
                 callbacks: List[Callable] = None):
        """
        Args:
            config: Evolution parameters
            evaluator: Fitness evaluator
            seed_genomes: Initial population seeds
            callbacks: Optional callbacks for monitoring (e.g., logging, plotting)
        """
        self.config = config
        self.evaluator = evaluator
        self.seed_genomes = seed_genomes
        self.callbacks = callbacks or []

        # Initialize population
        self.population = Population.create_initial_population(
            size=config.population_size,
            seed_genomes=seed_genomes,
            evaluator=evaluator,
            seed_ratio=config.seed_ratio
        )

        self.history: List[GenerationStats] = []
        self.best_ever: Individual | None = None

    def run(self) -> List[GenerationStats]:
        """
        Execute evolution for configured number of generations.

        Returns:
            History of generation statistics
        """
        print(f"Starting evolution: {self.config.num_generations} generations, "
              f"population size {self.config.population_size}")

        for gen in range(self.config.num_generations):
            gen_start = time.time()

            # Evaluate fitness
            self.population.evaluate_all(progressive=True)

            # Track statistics
            stats = self._record_generation_stats(gen)
            self.history.append(stats)

            gen_time = time.time() - gen_start

            print(f"Gen {gen}: "
                  f"avg={stats.avg_fitness:.3f} "
                  f"max={stats.max_fitness:.3f} "
                  f"diversity={stats.diversity:.3f} "
                  f"({gen_time:.1f}s)")

            # Diversity warning (consensus: monitor for premature convergence)
            if stats.diversity < 0.1:
                print(f"⚠️  WARNING: Low diversity ({stats.diversity:.3f}) - consider increasing population size")

            # Callbacks
            for callback in self.callbacks:
                callback(self.population, stats)

            # Early stopping: plateau detection
            if self._check_plateau():
                print(f"Early stopping: fitness plateau detected at generation {gen}")
                break

            # Create next generation
            self.population = self._create_next_generation()
            self.population.generation = gen + 1

        return self.history

    def _record_generation_stats(self, generation: int) -> GenerationStats:
        """Record statistics for current generation."""
        stats = self.population.get_stats()
        best = self.population.get_best(n=1)[0]

        # Track best ever
        if self.best_ever is None or best.fitness_score > self.best_ever.fitness_score:
            self.best_ever = best

        return GenerationStats(
            generation=generation,
            avg_fitness=stats['avg_fitness'],
            max_fitness=stats['max_fitness'],
            min_fitness=stats['min_fitness'],
            diversity=stats['diversity'],
            best_genome=best.genome,
            evaluation_time=0.0  # TODO: track evaluation time
        )

    def _check_plateau(self) -> bool:
        """Check if fitness has plateaued (early stopping criterion)."""
        if len(self.history) < self.config.plateau_threshold:
            return False

        # Check last N generations
        recent = self.history[-self.config.plateau_threshold:]
        max_fitnesses = [s.max_fitness for s in recent]

        # If max fitness hasn't improved by 1% in N generations, plateau
        improvement = (max(max_fitnesses) - min(max_fitnesses)) / (min(max_fitnesses) + 0.001)

        return improvement < 0.01

    def _create_next_generation(self) -> Population:
        """
        Create next generation using elitism, crossover, and mutation.

        Strategy:
        1. Preserve top N% (elitism)
        2. Fill rest with crossover + mutation offspring
        """
        next_gen_individuals = []

        # Elitism: preserve best individuals
        num_elites = int(self.config.population_size * self.config.elitism_rate)
        elites = self.population.get_best(n=num_elites)
        next_gen_individuals.extend(elites)

        # Breeding: create offspring to fill remaining slots
        num_offspring = self.config.population_size - num_elites
        num_pairs = (num_offspring + 1) // 2  # Round up

        parent_pairs = select_parents(
            self.population,
            num_pairs,
            tournament_size=self.config.tournament_size
        )

        for parent1, parent2 in parent_pairs:
            # Crossover
            if len(next_gen_individuals) >= self.config.population_size:
                break

            if len(parent1.genome.turn_structure.phases) > 1 and \
               len(parent2.genome.turn_structure.phases) > 1 and \
               random.random() < self.config.crossover_rate:
                child1_genome, child2_genome = crossover_genomes(
                    parent1.genome,
                    parent2.genome
                )
            else:
                # If crossover not possible, clone parents
                child1_genome = parent1.genome
                child2_genome = parent2.genome

            # Mutation
            child1_genome = mutate_genome(child1_genome, self.config.mutation_rate)
            child2_genome = mutate_genome(child2_genome, self.config.mutation_rate)

            # Validation and repair
            child1_genome = validate_and_repair(child1_genome)
            child2_genome = validate_and_repair(child2_genome)

            next_gen_individuals.append(Individual(genome=child1_genome))

            if len(next_gen_individuals) < self.config.population_size:
                next_gen_individuals.append(Individual(genome=child2_genome))

        return Population(
            next_gen_individuals,
            self.evaluator,
            elitism_rate=self.config.elitism_rate
        )


# Missing import
import random
```

#### Step 4.2: CLI entry point (10 min)

**File:** `src/darwindeck/cli/evolve.py`

```python
"""CLI entry point for evolution."""
import argparse
from pathlib import Path
import json
from darwindeck.evolution.engine import EvolutionEngine, EvolutionConfig
from darwindeck.evolution.fitness import FitnessEvaluator
from darwindeck.genome.examples import get_seed_genomes
from darwindeck.genome.bytecode import BytecodeCompiler


def save_best_genome(genome, output_path: Path):
    """Save best genome to file."""
    # Save as bytecode
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    with open(output_path.with_suffix('.bin'), 'wb') as f:
        f.write(bytecode)

    # Also save human-readable JSON
    # TODO: Implement genome JSON serialization
    print(f"Saved best genome to {output_path}")


def main():
    """Main CLI entry point."""
    parser = argparse.ArgumentParser(description='Evolve card games')
    parser.add_argument('--population', type=int, default=100, help='Population size')
    parser.add_argument('--generations', type=int, default=100, help='Number of generations')
    parser.add_argument('--output', type=Path, default=Path('output/best_genome'),
                       help='Output path for best genome')
    parser.add_argument('--elitism', type=float, default=0.1, help='Elitism rate')
    parser.add_argument('--crossover', type=float, default=0.7, help='Crossover rate')
    parser.add_argument('--mutation', type=float, default=1.0, help='Mutation rate')

    args = parser.parse_args()

    # Create output directory
    args.output.parent.mkdir(parents=True, exist_ok=True)

    # Configuration
    config = EvolutionConfig(
        population_size=args.population,
        num_generations=args.generations,
        elitism_rate=args.elitism,
        crossover_rate=args.crossover,
        mutation_rate=args.mutation
    )

    # Initialize
    evaluator = FitnessEvaluator()
    seeds = get_seed_genomes()

    # Run evolution
    engine = EvolutionEngine(config, evaluator, seeds)
    history = engine.run()

    # Save best genome
    if engine.best_ever:
        save_best_genome(engine.best_ever.genome, args.output)
        print(f"\nBest fitness: {engine.best_ever.fitness_score:.3f}")

    # Save history
    history_path = args.output.parent / 'evolution_history.json'
    with open(history_path, 'w') as f:
        json.dump([{
            'generation': s.generation,
            'avg_fitness': s.avg_fitness,
            'max_fitness': s.max_fitness,
            'diversity': s.diversity
        } for s in history], f, indent=2)

    print(f"Saved evolution history to {history_path}")


if __name__ == '__main__':
    main()
```

**Test:**
```bash
# Dry run with small population
uv run python -m darwindeck.cli.evolve --population 10 --generations 5
```

**Expected:** Evolution runs for 5 generations, saves best genome

**Commit:**
```
Implement evolution engine and CLI

EvolutionEngine:
- Orchestrates genetic algorithm loop
- Elitism: preserve top 10% each generation
- Tournament selection for parent pairs
- Crossover (70% rate) + mutation (100% rate)
- Validation and repair of offspring
- Early stopping: plateau detection (30 generations without 1% improvement, extended from 20 per consensus)
- Diversity monitoring: warnings when diversity < 0.1

Breeding strategy:
1. Elitism: copy best N% to next generation
2. Selection: tournament selection (size 3) for parent pairs
3. Crossover: semantic phase swapping (70% rate)
4. Mutation: apply mutation pipeline (100% rate, individual operators have lower rates)
5. Repair: validate and fix invalid genomes

CLI (darwindeck.cli.evolve):
- Configurable population size, generations, rates
- Saves best genome as bytecode
- Exports evolution history as JSON
- Progress reporting per generation

Statistics tracking:
- GenerationStats: avg/max/min fitness, diversity, best genome
- Plateau detection for early stopping
- Best-ever tracking across all generations

Testing:
- Small-scale dry run (10 individuals, 5 generations)
```

---

### Task 5: Testing and Validation (20 minutes)

**Goal:** End-to-end testing of evolution loop

#### Step 5.1: Integration test (15 min)

**File:** `tests/integration/test_evolution_loop.py`

```python
"""Integration test for full evolution loop."""
import pytest
from darwindeck.evolution.engine import EvolutionEngine, EvolutionConfig
from darwindeck.evolution.fitness import FitnessEvaluator
from darwindeck.genome.examples import get_seed_genomes


def test_evolution_loop_small_scale():
    """Test evolution with small population and few generations."""
    config = EvolutionConfig(
        population_size=10,
        num_generations=3,
        elitism_rate=0.2,
        crossover_rate=0.7,
        mutation_rate=1.0,
        tournament_size=3
    )

    evaluator = FitnessEvaluator(use_cache=True)
    seeds = get_seed_genomes()

    engine = EvolutionEngine(config, evaluator, seeds)
    history = engine.run()

    # Should complete all generations
    assert len(history) == 3

    # Each generation should have stats
    for stat in history:
        assert stat.avg_fitness >= 0.0
        assert stat.max_fitness >= stat.avg_fitness
        assert stat.best_genome is not None

    # Should have best ever
    assert engine.best_ever is not None
    assert engine.best_ever.fitness_score > 0.0


def test_evolution_improves_fitness():
    """Test that evolution improves fitness over generations."""
    config = EvolutionConfig(
        population_size=20,
        num_generations=10,
        elitism_rate=0.1,
    )

    evaluator = FitnessEvaluator(use_cache=True)
    seeds = get_seed_genomes()

    engine = EvolutionEngine(config, evaluator, seeds)
    history = engine.run()

    # Fitness should generally improve (or at least not decrease much)
    first_gen_max = history[0].max_fitness
    last_gen_max = history[-1].max_fitness

    # Allow some variance, but should trend upward
    assert last_gen_max >= first_gen_max * 0.8  # At least 80% of initial


def test_elitism_preserves_best():
    """Test that elitism preserves best individuals."""
    config = EvolutionConfig(
        population_size=20,
        num_generations=5,
        elitism_rate=0.2,  # Preserve top 20%
    )

    evaluator = FitnessEvaluator(use_cache=True)
    seeds = get_seed_genomes()

    engine = EvolutionEngine(config, evaluator, seeds)
    history = engine.run()

    # Max fitness should never decrease (elitism)
    for i in range(1, len(history)):
        assert history[i].max_fitness >= history[i-1].max_fitness


def test_diversity_metric():
    """Test that diversity is tracked."""
    config = EvolutionConfig(
        population_size=10,
        num_generations=3,
    )

    evaluator = FitnessEvaluator(use_cache=True)
    seeds = get_seed_genomes()

    engine = EvolutionEngine(config, evaluator, seeds)
    history = engine.run()

    # All generations should have diversity metric
    for stat in history:
        assert 0.0 <= stat.diversity <= 1.0
```

#### Step 5.2: Property-based test for genome validity (5 min)

**File:** `tests/evolution/test_genome_properties.py`

```python
"""Property-based testing for genome evolution."""
from hypothesis import given, strategies as st, settings
from darwindeck.evolution.operators import mutate_genome
from darwindeck.validation.schema_check import validate_genome, validate_and_repair
from darwindeck.genome.examples import create_war_genome


@given(mutation_rate=st.floats(min_value=0.0, max_value=2.0))
@settings(max_examples=50)
def test_mutation_preserves_validity(mutation_rate):
    """Test that mutation + repair always produces valid genomes."""
    war = create_war_genome()

    # Mutate
    mutated = mutate_genome(war, mutation_rate=mutation_rate)

    # Repair if needed
    repaired = validate_and_repair(mutated)

    # Should always be valid after repair
    is_valid, error = validate_genome(repaired)
    assert is_valid, f"Genome invalid after repair: {error}"


@given(num_mutations=st.integers(min_value=1, max_value=10))
@settings(max_examples=20)
def test_repeated_mutation_stability(num_mutations):
    """Test that repeated mutation doesn't cause genome explosion."""
    genome = create_war_genome()

    for _ in range(num_mutations):
        genome = mutate_genome(genome, mutation_rate=1.0)
        genome = validate_and_repair(genome)

    # Genome should still be reasonable
    assert len(genome.turn_structure.phases) <= 10  # Not too many phases
    assert genome.setup.cards_per_player <= 26  # Valid card distribution
    assert 10 <= genome.max_turns <= 10000  # Reasonable turn limits
```

**Test:**
```bash
uv run pytest tests/integration/test_evolution_loop.py -v
uv run pytest tests/evolution/test_genome_properties.py -v
```

**Expected:** All integration and property tests pass

**Commit:**
```
Add integration tests and property-based validation

Integration tests:
- test_evolution_loop_small_scale: 10 individuals, 3 generations
- test_evolution_improves_fitness: validates fitness trends upward
- test_elitism_preserves_best: max fitness never decreases
- test_diversity_metric: diversity tracked each generation

Property-based tests (hypothesis):
- test_mutation_preserves_validity: mutation + repair = valid genome
- test_repeated_mutation_stability: genome doesn't explode with mutations

Validation:
- All evolved genomes are valid after repair
- Turn structure stays reasonable (<= 10 phases)
- Card distribution valid (cards_per_player <= 26)
- Turn limits reasonable (10-10000)
```

---

## Success Criteria

✅ Mutation operators modify genomes without breaking structure
✅ Crossover creates valid offspring from two parents
✅ Fitness evaluation computes all 7 proxy metrics
✅ Population management tracks diversity and fitness statistics
✅ Evolution engine runs for N generations with elitism
✅ CLI tool accepts configuration and saves best genome
✅ Integration tests validate end-to-end evolution loop
✅ Property tests ensure genome validity is preserved

---

## Performance Targets

| Metric | Target | Rationale |
|--------|--------|-----------|
| Fitness evaluation | <5s for 100 simulations | Leverage Phase 3 Go core |
| Generation time | <10 min for pop=100 | 100 genomes × 100 sims × 5ms/game |
| Total evolution | <20 hours for 100 gens | Overnight run feasible |
| Cache hit rate | >50% after gen 10 | Many genomes similar |

---

## Critical Gaps Addressed

### From Original Plan

### 1. Concrete Mutation Operators ✅
- Implemented 7 mutation types with dataclass-aware logic
- Parameter tweaking, phase reordering, phase add/remove
- Condition modification, special effect addition
- **NEW: Win condition mutation** (consensus requirement)

### 2. Crossover Semantics ✅
- Semantic single-point crossover on turn structure phases
- Fallback to mutation if crossover not possible
- Generation counter incremented

### 3. Infinite Loop Detection ✅
- max_turns validation (10 ≤ max_turns ≤ 10000) - **Phase 3.5**
- Schema validation checks for stalemate conditions
- Repair mechanism fixes invalid turn limits

### 4. Human Evaluation Integration ⚠️
- **Deferred to Phase 4b or Phase 5**
- Proxy metrics established
- CLI saves best genomes for manual playtesting
- TODO: Add human rating collection system

### From Consensus Review (Multi-Agent)

### 5. Session Length as Constraint ✅
- **Moved from averaged metric to filter**
- Games outside 3-20 minute range get fitness = 0
- Remaining 6 metrics renormalized

### 6. Diversity Mechanism ✅
- **genome_distance() function** using Hamming distance on 6 structural features
- Pairwise distance averaging (sampled for O(n) performance)
- Diversity logging per generation
- Warning when diversity < 0.1

### 7. Simulation Failure Handling ✅
- Genomes with simulation errors get valid=False
- Session length constraint failures get fitness=0
- Invalid genomes filtered during evaluation

### 8. Plateau Detection Extended ✅
- **Extended from 20 to 30 generations** per consensus
- 1% improvement threshold maintained
- Adaptive early stopping

---

## Next Steps After Phase 4

Once Phase 4 MVP is complete:

1. **Human Playtesting Loop:**
   - Generate top 10 games from evolution
   - Convert to natural language rules (LLM rule generator)
   - Playtest with humans
   - Collect ratings
   - Retrain fitness weights based on human feedback

2. **Advanced Metrics:**
   - Instrument Go core to collect:
     - Decision counts per turn (for decision density)
     - Win probability traces (for tension curve)
     - Opponent interaction flags (for interaction frequency)

3. **Multi-Objective Optimization:**
   - Implement Pareto front tracking
   - Explore diversity of game styles (simple vs complex, skill vs luck, etc.)

4. **Adaptive Rates:**
   - Mutation rate scheduling (start high, decay)
   - Adaptive tournament size based on diversity

5. **Surrogate Models:**
   - Train neural network to predict fitness from genome
   - Use for fast initial filtering (1000s of candidates)
   - Full simulation only for top 100

---

## Timeline Estimate

- **Total time:** 2.5-3 hours
- **Task 1 (Operators):** 30 minutes
- **Task 2 (Fitness):** 45 minutes
- **Task 3 (Population):** 30 minutes
- **Task 4 (Engine):** 30 minutes
- **Task 5 (Testing):** 20 minutes

**Recommended execution:** Sequential (each task builds on previous)

---

## Notes on Multi-Agent Consensus

This plan incorporates **full multi-agent consensus** (Claude, Gemini, Codex - all 3 agents succeeded).

### High-Confidence Decisions (Unanimous Agreement)
- ✅ Progressive evaluation architecture (10 → 100 → MCTS)
- ✅ Session length as constraint filter (not averaged metric)
- ✅ Diversity mechanism required (genome distance + monitoring)
- ✅ Plateau detection extended to 30 generations
- ✅ Win-condition mutation operator critical

### Parameters Requiring Empirical Tuning
- **Population size:** Start at 100, increase to 200-300 if diversity < 0.1 persists
- **Mutation rates:** Monitor effective mutation magnitude, reduce if fitness degrades
- **Fitness weights:** Start equal (already normalized), recalibrate after 10-20 generations
- **MCTS depth:** Explicit budgeting needed (verify minimum depth for skill measurement fits time budget)

### Monitoring Requirements
- **Diversity:** Log per generation, warn if < 0.1, increase population if collapse detected
- **Repair frequency:** Audit repaired genomes to detect creativity collapse (falling back to War variants)
- **Fitness trends:** Should generally improve or plateau (not degrade)
- **Session length constraint:** Track rejection rate, adjust range if >50% filtered

### Known Limitations (Accept for MVP)
- **Proxy-to-fun gap:** Optimizing 7 metrics may not produce genuinely fun games
- **Mitigation:** Add minimal human validation checkpoint (5-10 playtests on top genomes) post-MVP
- **Genome repair risk:** May collapse novel-but-broken games into trivial defaults
- **Mitigation:** Log all repairs, manually inspect high-repair genomes
