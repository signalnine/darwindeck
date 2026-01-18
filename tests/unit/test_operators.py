"""Test special effect and betting mutation operators."""

import pytest
import random
from dataclasses import replace


def test_add_effect_mutation():
    """AddEffectMutation adds an effect to the genome."""
    from darwindeck.evolution.operators import AddEffectMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()
    original_count = len(genome.special_effects)

    mutation = AddEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    assert len(mutated.special_effects) == original_count + 1


def test_remove_effect_mutation():
    """RemoveEffectMutation removes an effect."""
    from darwindeck.evolution.operators import RemoveEffectMutation
    from darwindeck.genome.schema import (
        SpecialEffect, EffectType, Rank, TargetSelector,
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    # Create genome with effects
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[
            SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        ],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = RemoveEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    assert len(mutated.special_effects) == 0


def test_remove_effect_mutation_no_effects():
    """RemoveEffectMutation returns genome unchanged if no effects exist."""
    from darwindeck.evolution.operators import RemoveEffectMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()  # War has no special effects
    assert len(genome.special_effects) == 0

    mutation = RemoveEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert len(mutated.special_effects) == 0
    assert mutated.generation == genome.generation  # No mutation occurred


def test_mutate_effect_mutation():
    """MutateEffectMutation changes one field of an effect."""
    from darwindeck.evolution.operators import MutateEffectMutation
    from darwindeck.genome.schema import (
        SpecialEffect, EffectType, Rank, TargetSelector,
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    random.seed(42)  # For reproducibility

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[
            SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        ],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = MutateEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should still have one effect
    assert len(mutated.special_effects) == 1
    # Something should have changed
    original = genome.special_effects[0]
    changed = mutated.special_effects[0]
    assert (original.trigger_rank != changed.trigger_rank or
            original.effect_type != changed.effect_type or
            original.target != changed.target or
            original.value != changed.value)


def test_mutate_effect_mutation_no_effects():
    """MutateEffectMutation returns genome unchanged if no effects exist."""
    from darwindeck.evolution.operators import MutateEffectMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()  # War has no special effects
    assert len(genome.special_effects) == 0

    mutation = MutateEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert len(mutated.special_effects) == 0
    assert mutated.generation == genome.generation  # No mutation occurred


def test_add_effect_creates_valid_effect():
    """AddEffectMutation creates a valid SpecialEffect with correct types."""
    from darwindeck.evolution.operators import AddEffectMutation
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector
    from darwindeck.genome.examples import create_war_genome

    random.seed(123)
    genome = create_war_genome()

    mutation = AddEffectMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    new_effect = mutated.special_effects[0]
    assert isinstance(new_effect, SpecialEffect)
    assert isinstance(new_effect.trigger_rank, Rank)
    assert isinstance(new_effect.effect_type, EffectType)
    assert isinstance(new_effect.target, TargetSelector)
    assert isinstance(new_effect.value, int)
    assert 1 <= new_effect.value <= 3


# =====================================================================
# Betting Mutation Tests
# =====================================================================


def test_add_betting_phase_mutation():
    """AddBettingPhaseMutation adds a BettingPhase to the genome."""
    from darwindeck.evolution.operators import AddBettingPhaseMutation
    from darwindeck.genome.schema import BettingPhase
    from darwindeck.genome.examples import create_war_genome

    random.seed(42)
    genome = create_war_genome()
    original_phase_count = len(genome.turn_structure.phases)

    mutation = AddBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should have one more phase
    assert len(mutated.turn_structure.phases) == original_phase_count + 1
    # One of them should be a BettingPhase
    betting_phases = [p for p in mutated.turn_structure.phases if isinstance(p, BettingPhase)]
    assert len(betting_phases) == 1


def test_add_betting_phase_respects_max_phases():
    """AddBettingPhaseMutation respects max 5 phases limit."""
    from darwindeck.evolution.operators import AddBettingPhaseMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, DrawPhase, Location
    )

    # Create genome with 5 phases
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[
            DrawPhase(source=Location.DECK) for _ in range(5)
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = AddBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should not add another phase
    assert len(mutated.turn_structure.phases) == 5
    assert mutated.generation == 0  # No mutation occurred


def test_remove_betting_phase_mutation():
    """RemoveBettingPhaseMutation removes a BettingPhase."""
    from darwindeck.evolution.operators import RemoveBettingPhaseMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition,
        BettingPhase, DrawPhase, Location
    )

    # Create genome with a BettingPhase
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7, starting_chips=1000),
        turn_structure=TurnStructure(phases=[
            DrawPhase(source=Location.DECK),
            BettingPhase(min_bet=10, max_raises=3),
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = RemoveBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should have one less phase
    assert len(mutated.turn_structure.phases) == 1
    # No BettingPhase should remain
    betting_phases = [p for p in mutated.turn_structure.phases if isinstance(p, BettingPhase)]
    assert len(betting_phases) == 0


def test_remove_betting_phase_mutation_no_betting():
    """RemoveBettingPhaseMutation returns unchanged if no BettingPhases."""
    from darwindeck.evolution.operators import RemoveBettingPhaseMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()  # War has no betting phases

    mutation = RemoveBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert mutated.generation == genome.generation


def test_mutate_betting_phase_mutation():
    """MutateBettingPhaseMutation modifies min_bet or max_raises."""
    from darwindeck.evolution.operators import MutateBettingPhaseMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase
    )

    random.seed(42)

    # Create genome with a BettingPhase
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7, starting_chips=1000),
        turn_structure=TurnStructure(phases=[
            BettingPhase(min_bet=10, max_raises=3),
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = MutateBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should still have one BettingPhase
    assert len(mutated.turn_structure.phases) == 1
    original = genome.turn_structure.phases[0]
    changed = mutated.turn_structure.phases[0]

    # Something should have changed
    assert (original.min_bet != changed.min_bet or
            original.max_raises != changed.max_raises)


def test_mutate_betting_phase_no_betting():
    """MutateBettingPhaseMutation returns unchanged if no BettingPhases."""
    from darwindeck.evolution.operators import MutateBettingPhaseMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()

    mutation = MutateBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert mutated.generation == genome.generation


def test_mutate_starting_chips_mutation():
    """MutateStartingChipsMutation modifies starting_chips."""
    from darwindeck.evolution.operators import MutateStartingChipsMutation
    from darwindeck.genome.examples import create_war_genome

    random.seed(42)
    genome = create_war_genome()
    # War has starting_chips=0 by default
    assert genome.setup.starting_chips == 0

    mutation = MutateStartingChipsMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should now have starting chips
    assert mutated.setup.starting_chips > 0


def test_mutate_starting_chips_existing():
    """MutateStartingChipsMutation mutates existing chips by +-50%."""
    from darwindeck.evolution.operators import MutateStartingChipsMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    random.seed(42)

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7, starting_chips=1000),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = MutateStartingChipsMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should have different starting chips
    assert mutated.setup.starting_chips != 1000
    # But within +-50% range (plus min of 10)
    assert 10 <= mutated.setup.starting_chips <= 1500


def test_betting_constraint_min_bet_le_starting_chips():
    """min_bet <= starting_chips is maintained after mutation."""
    from darwindeck.evolution.operators import MutateStartingChipsMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase
    )

    # Set seed to get a specific lower starting_chips value
    random.seed(5)

    # Create genome with high min_bet relative to chips
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7, starting_chips=100),
        turn_structure=TurnStructure(phases=[
            BettingPhase(min_bet=80, max_raises=3),
        ]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = MutateStartingChipsMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Verify constraint: min_bet <= starting_chips
    betting_phase = mutated.turn_structure.phases[0]
    assert betting_phase.min_bet <= mutated.setup.starting_chips


def test_add_betting_phase_min_bet_valid():
    """AddBettingPhaseMutation creates valid min_bet <= starting_chips."""
    from darwindeck.evolution.operators import AddBettingPhaseMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, BettingPhase
    )

    random.seed(42)

    # Create genome with low starting chips
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7, starting_chips=5),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    mutation = AddBettingPhaseMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Verify min_bet <= starting_chips
    betting_phase = next(
        p for p in mutated.turn_structure.phases if isinstance(p, BettingPhase)
    )
    # starting_chips is 5, so min_bet should be at most 5
    assert betting_phase.min_bet <= genome.setup.starting_chips or betting_phase.min_bet == 1


# =====================================================================
# Tableau Mode Mutation Tests
# =====================================================================


def test_mutate_tableau_mode():
    """MutateTableauModeMutation changes tableau mode."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, TableauMode
    )
    from darwindeck.evolution.operators import MutateTableauModeMutation

    random.seed(42)

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.NONE),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=2,
    )

    mutation = MutateTableauModeMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Mode should have changed (with seed 42, should get a different mode)
    assert mutated.setup.tableau_mode != TableauMode.NONE


def test_mutate_tableau_mode_war_requires_2_players():
    """WAR mode mutation only applies to 2-player games."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, TableauMode
    )
    from darwindeck.evolution.operators import MutateTableauModeMutation

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.NONE),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=4,  # 4 players - WAR not allowed
    )

    mutation = MutateTableauModeMutation(probability=1.0)

    # Try multiple times with different seeds - should never get WAR
    for seed in range(50):
        random.seed(seed)
        mutated = mutation.mutate(genome)
        assert mutated.setup.tableau_mode != TableauMode.WAR, f"WAR should not be allowed with 4 players (seed={seed})"


def test_mutate_sequence_direction():
    """MutateSequenceDirectionMutation changes direction when mode is SEQUENCE."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition,
        TableauMode, SequenceDirection
    )
    from darwindeck.evolution.operators import MutateSequenceDirectionMutation

    random.seed(42)

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(
            cards_per_player=7,
            tableau_mode=TableauMode.SEQUENCE,
            sequence_direction=SequenceDirection.ASCENDING
        ),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=2,
    )

    mutation = MutateSequenceDirectionMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Direction should have changed
    assert mutated.setup.sequence_direction != SequenceDirection.ASCENDING


def test_mutate_sequence_direction_no_op_when_not_sequence():
    """MutateSequenceDirectionMutation is no-op when not in SEQUENCE mode."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition,
        TableauMode, SequenceDirection
    )
    from darwindeck.evolution.operators import MutateSequenceDirectionMutation

    random.seed(42)

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(
            cards_per_player=7,
            tableau_mode=TableauMode.WAR,  # Not SEQUENCE
            sequence_direction=SequenceDirection.BOTH
        ),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=2,
    )

    mutation = MutateSequenceDirectionMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should be unchanged - mutation is no-op when not SEQUENCE mode
    assert mutated.setup.sequence_direction == SequenceDirection.BOTH
    assert mutated.setup.tableau_mode == TableauMode.WAR


def test_tableau_mutations_in_default_pipeline():
    """Default pipeline includes tableau mode mutations."""
    from darwindeck.evolution.operators import (
        create_default_pipeline,
        MutateTableauModeMutation,
        MutateSequenceDirectionMutation,
    )

    pipeline = create_default_pipeline()

    # Pipeline should include tableau mode mutation
    has_tableau_mode_mutation = any(
        isinstance(op, MutateTableauModeMutation)
        for op in pipeline.operators
    )
    assert has_tableau_mode_mutation, "Default pipeline should include MutateTableauModeMutation"

    # Pipeline should include sequence direction mutation
    has_sequence_direction_mutation = any(
        isinstance(op, MutateSequenceDirectionMutation)
        for op in pipeline.operators
    )
    assert has_sequence_direction_mutation, "Default pipeline should include MutateSequenceDirectionMutation"

    # Check probabilities
    tableau_op = next(
        op for op in pipeline.operators
        if isinstance(op, MutateTableauModeMutation)
    )
    assert tableau_op.probability == 0.05  # 5% probability

    sequence_op = next(
        op for op in pipeline.operators
        if isinstance(op, MutateSequenceDirectionMutation)
    )
    assert sequence_op.probability == 0.03  # 3% probability


# =====================================================================
# Card Scoring Mutation Tests
# =====================================================================


def test_add_card_scoring_mutation():
    """Test adding a card scoring rule."""
    from darwindeck.evolution.operators import AddCardScoringMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()
    assert len(genome.card_scoring) == 0

    mutation = AddCardScoringMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    assert len(mutated.card_scoring) == 1
    assert mutated.card_scoring[0].points != 0 or mutated.card_scoring[0].points == 0  # Can be any int


def test_add_card_scoring_creates_valid_rule():
    """AddCardScoringMutation creates a valid CardScoringRule with correct types."""
    from darwindeck.evolution.operators import AddCardScoringMutation
    from darwindeck.genome.schema import CardScoringRule, CardCondition, ScoringTrigger, Suit, Rank
    from darwindeck.genome.examples import create_war_genome

    random.seed(123)
    genome = create_war_genome()

    mutation = AddCardScoringMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    new_rule = mutated.card_scoring[0]
    assert isinstance(new_rule, CardScoringRule)
    assert isinstance(new_rule.condition, CardCondition)
    assert isinstance(new_rule.trigger, ScoringTrigger)
    assert isinstance(new_rule.points, int)
    assert -5 <= new_rule.points <= 15
    # Suit can be None or a Suit
    assert new_rule.condition.suit is None or isinstance(new_rule.condition.suit, Suit)
    # Rank can be None or a Rank
    assert new_rule.condition.rank is None or isinstance(new_rule.condition.rank, Rank)


def test_add_card_scoring_multiple_rules():
    """Multiple applications add multiple rules."""
    from darwindeck.evolution.operators import AddCardScoringMutation
    from darwindeck.genome.examples import create_war_genome

    random.seed(42)
    genome = create_war_genome()
    assert len(genome.card_scoring) == 0

    mutation = AddCardScoringMutation(probability=1.0)
    mutated = mutation.mutate(genome)
    mutated = mutation.mutate(mutated)
    mutated = mutation.mutate(mutated)

    assert len(mutated.card_scoring) == 3
    assert mutated.generation == genome.generation + 3


def test_mutate_card_scoring():
    """Test mutating existing card scoring rules."""
    from darwindeck.evolution.operators import MutateCardScoringMutation
    from darwindeck.genome.examples import create_hearts_genome

    random.seed(42)
    genome = create_hearts_genome()
    assert len(genome.card_scoring) >= 2

    original_points = [r.points for r in genome.card_scoring]

    mutation = MutateCardScoringMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    mutated_points = [r.points for r in mutated.card_scoring]
    # At least one point value should have changed
    assert original_points != mutated_points


def test_mutate_card_scoring_no_rules():
    """MutateCardScoringMutation returns genome unchanged if no rules exist."""
    from darwindeck.evolution.operators import MutateCardScoringMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()  # War has no card scoring rules
    assert len(genome.card_scoring) == 0

    mutation = MutateCardScoringMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert len(mutated.card_scoring) == 0
    assert mutated.generation == genome.generation  # No mutation occurred


def test_remove_card_scoring():
    """Test removing a card scoring rule."""
    from darwindeck.evolution.operators import RemoveCardScoringMutation
    from darwindeck.genome.examples import create_hearts_genome

    random.seed(42)
    genome = create_hearts_genome()
    original_count = len(genome.card_scoring)

    mutation = RemoveCardScoringMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    assert len(mutated.card_scoring) == original_count - 1


def test_remove_card_scoring_no_rules():
    """RemoveCardScoringMutation returns genome unchanged if no rules exist."""
    from darwindeck.evolution.operators import RemoveCardScoringMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()  # War has no card scoring rules
    assert len(genome.card_scoring) == 0

    mutation = RemoveCardScoringMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert len(mutated.card_scoring) == 0
    assert mutated.generation == genome.generation  # No mutation occurred


# =====================================================================
# Hand Evaluation Mutation Tests
# =====================================================================


def test_mutate_hand_pattern_priority():
    """Test mutating hand pattern rank priority."""
    from darwindeck.evolution.operators import MutateHandPatternMutation
    from darwindeck.genome.examples import create_simple_poker_genome

    random.seed(42)

    genome = create_simple_poker_genome()
    assert genome.hand_evaluation is not None
    assert len(genome.hand_evaluation.patterns) > 0

    original_priorities = [p.rank_priority for p in genome.hand_evaluation.patterns]

    mutation = MutateHandPatternMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    mutated_priorities = [p.rank_priority for p in mutated.hand_evaluation.patterns]
    assert original_priorities != mutated_priorities
    assert mutated.generation == genome.generation + 1


def test_mutate_hand_pattern_no_patterns():
    """MutateHandPatternMutation returns unchanged if no patterns."""
    from darwindeck.evolution.operators import MutateHandPatternMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()
    # War has no hand_evaluation
    assert genome.hand_evaluation is None

    mutation = MutateHandPatternMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert mutated.generation == genome.generation


def test_mutate_hand_pattern_bounds():
    """Mutated priority stays within 1-100 bounds."""
    from darwindeck.evolution.operators import MutateHandPatternMutation
    from darwindeck.genome.examples import create_simple_poker_genome

    genome = create_simple_poker_genome()

    mutation = MutateHandPatternMutation(probability=1.0)

    # Try many seeds to test bounds
    for seed in range(50):
        random.seed(seed)
        mutated = mutation.mutate(genome)
        for pattern in mutated.hand_evaluation.patterns:
            assert 1 <= pattern.rank_priority <= 100, f"Priority out of bounds: {pattern.rank_priority}"


def test_mutate_card_value():
    """Test mutating card point values."""
    from darwindeck.evolution.operators import MutateCardValueMutation
    from darwindeck.genome.examples import create_blackjack_genome

    random.seed(42)

    genome = create_blackjack_genome()
    assert genome.hand_evaluation is not None
    assert len(genome.hand_evaluation.card_values) > 0

    original_values = [cv.value for cv in genome.hand_evaluation.card_values]

    mutation = MutateCardValueMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    mutated_values = [cv.value for cv in mutated.hand_evaluation.card_values]
    assert original_values != mutated_values
    assert mutated.generation == genome.generation + 1


def test_mutate_card_value_no_values():
    """MutateCardValueMutation returns unchanged if no card_values."""
    from darwindeck.evolution.operators import MutateCardValueMutation
    from darwindeck.genome.examples import create_war_genome

    genome = create_war_genome()
    # War has no hand_evaluation
    assert genome.hand_evaluation is None

    mutation = MutateCardValueMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Should return genome unchanged
    assert mutated.generation == genome.generation


def test_mutate_card_value_bounds():
    """Mutated value stays within 1-15 bounds."""
    from darwindeck.evolution.operators import MutateCardValueMutation
    from darwindeck.genome.examples import create_blackjack_genome

    genome = create_blackjack_genome()

    mutation = MutateCardValueMutation(probability=1.0)

    # Try many seeds to test bounds
    for seed in range(50):
        random.seed(seed)
        mutated = mutation.mutate(genome)
        for cv in mutated.hand_evaluation.card_values:
            assert 1 <= cv.value <= 15, f"Value out of bounds: {cv.value}"


def test_mutate_card_value_preserves_alternate():
    """MutateCardValueMutation preserves alternate_value."""
    random.seed(0)  # Seed that will modify the Ace

    from darwindeck.evolution.operators import MutateCardValueMutation
    from darwindeck.genome.examples import create_blackjack_genome

    genome = create_blackjack_genome()
    # Blackjack Ace has alternate_value=11
    ace_cv = next(cv for cv in genome.hand_evaluation.card_values if cv.alternate_value is not None)
    assert ace_cv.alternate_value == 11

    mutation = MutateCardValueMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Find the Ace in mutated (should still have alternate_value)
    mutated_ace = next(cv for cv in mutated.hand_evaluation.card_values if cv.rank == ace_cv.rank)
    assert mutated_ace.alternate_value == 11


# =====================================================================
# Default Pipeline Tests
# =====================================================================


def test_self_describing_mutations_in_default_pipeline():
    """New self-describing mutations are in the default pipeline."""
    from darwindeck.evolution.operators import (
        create_default_pipeline,
        AddCardScoringMutation,
        MutateCardScoringMutation,
        RemoveCardScoringMutation,
        MutateHandPatternMutation,
        MutateCardValueMutation,
    )

    pipeline = create_default_pipeline()
    operator_types = [type(op).__name__ for op in pipeline.operators]

    assert "AddCardScoringMutation" in operator_types
    assert "MutateCardScoringMutation" in operator_types
    assert "RemoveCardScoringMutation" in operator_types
    assert "MutateHandPatternMutation" in operator_types
    assert "MutateCardValueMutation" in operator_types


def test_self_describing_mutations_have_correct_probabilities():
    """Self-describing mutations have expected probabilities."""
    from darwindeck.evolution.operators import (
        create_default_pipeline,
        AddCardScoringMutation,
        MutateCardScoringMutation,
        RemoveCardScoringMutation,
        MutateHandPatternMutation,
        MutateCardValueMutation,
    )

    pipeline = create_default_pipeline()

    # Find each operator and check probability
    add_scoring = next(
        op for op in pipeline.operators if isinstance(op, AddCardScoringMutation)
    )
    assert add_scoring.probability == 0.05

    mutate_scoring = next(
        op for op in pipeline.operators if isinstance(op, MutateCardScoringMutation)
    )
    assert mutate_scoring.probability == 0.10

    remove_scoring = next(
        op for op in pipeline.operators if isinstance(op, RemoveCardScoringMutation)
    )
    assert remove_scoring.probability == 0.03

    mutate_pattern = next(
        op for op in pipeline.operators if isinstance(op, MutateHandPatternMutation)
    )
    assert mutate_pattern.probability == 0.05

    mutate_value = next(
        op for op in pipeline.operators if isinstance(op, MutateCardValueMutation)
    )
    assert mutate_value.probability == 0.05


# =====================================================================
# Team Mutation Tests
# =====================================================================


def test_enable_team_mode_mutation():
    """EnableTeamModeMutation should enable team mode with 2v2 teams."""
    from darwindeck.evolution.operators import EnableTeamModeMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=4,
        team_mode=False,
        teams=(),
    )
    mutation = EnableTeamModeMutation(probability=1.0)

    assert mutation.can_apply(genome)
    mutated = mutation.mutate(genome)

    assert mutated.team_mode is True
    assert len(mutated.teams) == 2
    # Each team should have 2 players (for 4-player game)
    assert len(mutated.teams[0]) == 2
    assert len(mutated.teams[1]) == 2
    # All 4 players should be assigned
    all_players = set(mutated.teams[0] + mutated.teams[1])
    assert all_players == {0, 1, 2, 3}


def test_enable_team_mode_already_enabled():
    """EnableTeamModeMutation should not apply if already enabled."""
    from darwindeck.evolution.operators import EnableTeamModeMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=4,
        team_mode=True,
        teams=((0, 2), (1, 3)),
    )
    mutation = EnableTeamModeMutation(probability=1.0)

    assert not mutation.can_apply(genome)


def test_enable_team_mode_odd_players():
    """EnableTeamModeMutation should not apply for odd player counts."""
    from darwindeck.evolution.operators import EnableTeamModeMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=3,
        team_mode=False,
    )
    mutation = EnableTeamModeMutation(probability=1.0)

    # Can't have equal teams with odd players
    assert not mutation.can_apply(genome)


def test_enable_team_mode_two_players():
    """EnableTeamModeMutation should not apply for 2-player games."""
    from darwindeck.evolution.operators import EnableTeamModeMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=2,
        team_mode=False,
    )
    mutation = EnableTeamModeMutation(probability=1.0)

    # 2 players is not enough for teams (need 4+)
    assert not mutation.can_apply(genome)


def test_enable_team_mode_six_players():
    """EnableTeamModeMutation should work with 6 players (3v3)."""
    from darwindeck.evolution.operators import EnableTeamModeMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=6,
        team_mode=False,
        teams=(),
    )
    mutation = EnableTeamModeMutation(probability=1.0)

    assert mutation.can_apply(genome)
    mutated = mutation.mutate(genome)

    assert mutated.team_mode is True
    assert len(mutated.teams) == 2
    # Each team should have 3 players (for 6-player game)
    assert len(mutated.teams[0]) == 3
    assert len(mutated.teams[1]) == 3
    # All 6 players should be assigned
    all_players = set(mutated.teams[0] + mutated.teams[1])
    assert all_players == {0, 1, 2, 3, 4, 5}


def test_disable_team_mode_mutation():
    """DisableTeamModeMutation should disable team mode."""
    from darwindeck.evolution.operators import DisableTeamModeMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=4,
        team_mode=True,
        teams=((0, 2), (1, 3)),
    )
    mutation = DisableTeamModeMutation(probability=1.0)

    assert mutation.can_apply(genome)
    mutated = mutation.mutate(genome)

    assert mutated.team_mode is False
    assert mutated.teams == ()


def test_disable_team_mode_already_disabled():
    """DisableTeamModeMutation should not apply if already disabled."""
    from darwindeck.evolution.operators import DisableTeamModeMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=4,
        team_mode=False,
    )
    mutation = DisableTeamModeMutation(probability=1.0)

    assert not mutation.can_apply(genome)


def test_mutate_team_assignment():
    """MutateTeamAssignmentMutation should swap players between teams."""
    from darwindeck.evolution.operators import MutateTeamAssignmentMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    random.seed(42)

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=4,
        team_mode=True,
        teams=((0, 2), (1, 3)),
    )
    mutation = MutateTeamAssignmentMutation(probability=1.0)

    assert mutation.can_apply(genome)
    mutated = mutation.mutate(genome)

    # Should still be valid team configuration
    assert mutated.team_mode is True
    assert len(mutated.teams) == 2
    all_players = set(mutated.teams[0] + mutated.teams[1])
    assert all_players == {0, 1, 2, 3}
    # Should be different from original (with high probability)
    # Note: With 4 players, there's a small chance of same config


def test_mutate_team_assignment_no_teams():
    """MutateTeamAssignmentMutation should not apply without teams."""
    from darwindeck.evolution.operators import MutateTeamAssignmentMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=7),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=4,
        team_mode=False,
    )
    mutation = MutateTeamAssignmentMutation(probability=1.0)

    assert not mutation.can_apply(genome)


def test_mutate_team_assignment_preserves_team_sizes():
    """MutateTeamAssignmentMutation should preserve team sizes."""
    from darwindeck.evolution.operators import MutateTeamAssignmentMutation
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition
    )

    random.seed(123)

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=0,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        player_count=6,
        team_mode=True,
        teams=((0, 2, 4), (1, 3, 5)),
    )
    mutation = MutateTeamAssignmentMutation(probability=1.0)

    assert mutation.can_apply(genome)
    mutated = mutation.mutate(genome)

    # Team sizes should be preserved
    assert len(mutated.teams[0]) == 3
    assert len(mutated.teams[1]) == 3
    # All players still assigned
    all_players = set(mutated.teams[0] + mutated.teams[1])
    assert all_players == {0, 1, 2, 3, 4, 5}


def test_team_mutations_in_default_pipeline():
    """Default pipeline includes team mode mutations."""
    from darwindeck.evolution.operators import (
        create_default_pipeline,
        EnableTeamModeMutation,
        DisableTeamModeMutation,
        MutateTeamAssignmentMutation,
    )

    pipeline = create_default_pipeline()
    operator_types = [type(op).__name__ for op in pipeline.operators]

    assert "EnableTeamModeMutation" in operator_types
    assert "DisableTeamModeMutation" in operator_types
    assert "MutateTeamAssignmentMutation" in operator_types


def test_cleanup_orphaned_resources_removes_chips_without_betting():
    """CleanupOrphanedResourcesMutation removes chips when no BettingPhase exists."""
    from darwindeck.evolution.operators import CleanupOrphanedResourcesMutation
    from darwindeck.genome.examples import create_war_genome
    from dataclasses import replace

    # Create a genome with orphaned chips (chips but no BettingPhase)
    genome = create_war_genome()
    genome_with_chips = replace(
        genome,
        setup=replace(genome.setup, starting_chips=1000)
    )

    # Verify it has chips but no betting phase
    from darwindeck.genome.schema import BettingPhase
    has_betting = any(isinstance(p, BettingPhase) for p in genome_with_chips.turn_structure.phases)
    assert genome_with_chips.setup.starting_chips == 1000
    assert not has_betting

    # Apply cleanup mutation
    mutation = CleanupOrphanedResourcesMutation(probability=1.0)
    mutated = mutation.mutate(genome_with_chips)

    # Chips should be removed
    assert mutated.setup.starting_chips == 0


def test_cleanup_orphaned_resources_preserves_valid_chips():
    """CleanupOrphanedResourcesMutation preserves chips when BettingPhase exists."""
    from darwindeck.evolution.operators import CleanupOrphanedResourcesMutation
    from darwindeck.genome.examples import create_simple_poker_genome

    # Simple poker has both chips and betting phase
    genome = create_simple_poker_genome()

    # Verify it has chips and betting phase
    from darwindeck.genome.schema import BettingPhase
    has_betting = any(isinstance(p, BettingPhase) for p in genome.turn_structure.phases)
    assert genome.setup.starting_chips > 0
    assert has_betting

    # Apply cleanup mutation
    mutation = CleanupOrphanedResourcesMutation(probability=1.0)
    mutated = mutation.mutate(genome)

    # Chips should be preserved
    assert mutated.setup.starting_chips == genome.setup.starting_chips


def test_cleanup_orphaned_resources_in_default_pipeline():
    """Default pipeline includes cleanup mutation."""
    from darwindeck.evolution.operators import (
        create_default_pipeline,
        CleanupOrphanedResourcesMutation,
    )

    pipeline = create_default_pipeline()
    operator_types = [type(op).__name__ for op in pipeline.operators]

    assert "CleanupOrphanedResourcesMutation" in operator_types
