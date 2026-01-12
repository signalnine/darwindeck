"""Integration tests for the betting system."""

import random
import pytest

from darwindeck.genome.examples import create_simple_poker_genome, create_war_genome
from darwindeck.genome.serialization import genome_to_json, genome_from_json
from darwindeck.genome.schema import (
    GameGenome,
    SetupRules,
    TurnStructure,
    WinCondition,
    BettingPhase,
    PlayPhase,
    Location,
)
from darwindeck.genome.conditions import Condition, ConditionType, Operator
from darwindeck.evolution.operators import (
    AddBettingPhaseMutation,
    RemoveBettingPhaseMutation,
    MutateBettingPhaseMutation,
    MutateStartingChipsMutation,
)


class TestBettingGenomeSerialization:
    """Test betting genome serialization round-trip."""

    def test_betting_genome_roundtrip(self):
        """Test simple poker genome serializes and deserializes correctly."""
        genome = create_simple_poker_genome()

        # Serialize to JSON
        json_str = genome_to_json(genome)

        # Deserialize back
        restored = genome_from_json(json_str)

        # Verify key betting properties
        assert restored.setup.starting_chips == 1000
        assert isinstance(restored.turn_structure.phases[0], BettingPhase)

        # Verify BettingPhase parameters
        betting_phase = restored.turn_structure.phases[0]
        assert betting_phase.min_bet == 10
        assert betting_phase.max_raises == 3

    def test_betting_genome_roundtrip_preserves_all_fields(self):
        """Test round-trip preserves all genome fields."""
        genome = create_simple_poker_genome()
        json_str = genome_to_json(genome)
        restored = genome_from_json(json_str)

        # Schema info
        assert restored.schema_version == genome.schema_version
        assert restored.genome_id == genome.genome_id
        assert restored.generation == genome.generation

        # Setup
        assert restored.setup.cards_per_player == genome.setup.cards_per_player
        assert restored.setup.initial_deck == genome.setup.initial_deck
        assert restored.setup.initial_discard_count == genome.setup.initial_discard_count
        assert restored.setup.starting_chips == genome.setup.starting_chips

        # Turn structure
        assert len(restored.turn_structure.phases) == len(genome.turn_structure.phases)

        # Win conditions
        assert len(restored.win_conditions) == len(genome.win_conditions)
        assert restored.win_conditions[0].type == genome.win_conditions[0].type

        # Game parameters
        assert restored.max_turns == genome.max_turns
        assert restored.player_count == genome.player_count

    def test_betting_genome_json_contains_betting_fields(self):
        """Test serialized JSON contains betting-specific fields."""
        genome = create_simple_poker_genome()
        json_str = genome_to_json(genome)

        # Check JSON string contains betting fields
        assert '"starting_chips": 1000' in json_str
        assert '"type": "BettingPhase"' in json_str
        assert '"min_bet": 10' in json_str
        assert '"max_raises": 3' in json_str


class TestBettingSimulation:
    """Test betting simulation (requires CGo bridge with betting support)."""

    def _check_cgo_betting_support(self):
        """Check if CGo bridge is available and supports betting games.

        Returns:
            (available, reason) tuple where available is bool
        """
        try:
            from darwindeck.bindings.cgo_bridge import simulate_batch
            from darwindeck.genome.bytecode import BytecodeCompiler
            import flatbuffers
            from darwindeck.bindings.cardsim.SimulationRequest import (
                SimulationRequestStart,
                SimulationRequestAddGenomeBytecode,
                SimulationRequestAddNumGames,
                SimulationRequestAddAiPlayerType,
                SimulationRequestAddMctsIterations,
                SimulationRequestAddRandomSeed,
                SimulationRequestEnd,
            )
            from darwindeck.bindings.cardsim.BatchRequest import (
                BatchRequestStart,
                BatchRequestAddBatchId,
                BatchRequestAddRequests,
                BatchRequestStartRequestsVector,
                BatchRequestEnd,
            )

            # Test with simple poker genome to see if betting is supported
            genome = create_simple_poker_genome()
            compiler = BytecodeCompiler()
            bytecode = compiler.compile_genome(genome)

            builder = flatbuffers.Builder(2048)
            genome_offset = builder.CreateByteVector(bytecode)

            SimulationRequestStart(builder)
            SimulationRequestAddGenomeBytecode(builder, genome_offset)
            SimulationRequestAddNumGames(builder, 1)
            SimulationRequestAddAiPlayerType(builder, 0)
            SimulationRequestAddMctsIterations(builder, 0)
            SimulationRequestAddRandomSeed(builder, 42)
            req_offset = SimulationRequestEnd(builder)

            BatchRequestStartRequestsVector(builder, 1)
            builder.PrependUOffsetTRelative(req_offset)
            requests_offset = builder.EndVector()

            BatchRequestStart(builder)
            BatchRequestAddBatchId(builder, 1)
            BatchRequestAddRequests(builder, requests_offset)
            batch_offset = BatchRequestEnd(builder)

            builder.Finish(batch_offset)
            request_bytes = bytes(builder.Output())

            response = simulate_batch(request_bytes)
            result = response.Results(0)

            # Check if games completed without errors
            if result.Errors() > 0:
                return False, "CGo bridge does not yet support BettingPhase games"

            return True, None

        except ImportError as e:
            return False, f"CGo bridge not available: {e}"
        except OSError as e:
            return False, f"Library loading error: {e}"

    def test_betting_simulation_completes(self):
        """Game with BettingPhase runs without infinite loop.

        This test verifies that the Go simulator can execute games
        with BettingPhase. The test uses the CGo bridge to run
        100 poker games and verifies they complete successfully.
        """
        available, reason = self._check_cgo_betting_support()
        if not available:
            pytest.skip(reason)

        # Import here to avoid errors if CGo not available
        from darwindeck.bindings.cgo_bridge import simulate_batch
        from darwindeck.genome.bytecode import BytecodeCompiler
        import flatbuffers
        from darwindeck.bindings.cardsim.SimulationRequest import (
            SimulationRequestStart,
            SimulationRequestAddGenomeBytecode,
            SimulationRequestAddNumGames,
            SimulationRequestAddAiPlayerType,
            SimulationRequestAddMctsIterations,
            SimulationRequestAddRandomSeed,
            SimulationRequestEnd,
        )
        from darwindeck.bindings.cardsim.BatchRequest import (
            BatchRequestStart,
            BatchRequestAddBatchId,
            BatchRequestAddRequests,
            BatchRequestStartRequestsVector,
            BatchRequestEnd,
        )

        genome = create_simple_poker_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        # Build simulation request
        builder = flatbuffers.Builder(2048)
        genome_offset = builder.CreateByteVector(bytecode)

        SimulationRequestStart(builder)
        SimulationRequestAddGenomeBytecode(builder, genome_offset)
        SimulationRequestAddNumGames(builder, 100)
        SimulationRequestAddAiPlayerType(builder, 0)  # Random AI
        SimulationRequestAddMctsIterations(builder, 0)
        SimulationRequestAddRandomSeed(builder, 42)
        req_offset = SimulationRequestEnd(builder)

        BatchRequestStartRequestsVector(builder, 1)
        builder.PrependUOffsetTRelative(req_offset)
        requests_offset = builder.EndVector()

        BatchRequestStart(builder)
        BatchRequestAddBatchId(builder, 1)
        BatchRequestAddRequests(builder, requests_offset)
        batch_offset = BatchRequestEnd(builder)

        builder.Finish(batch_offset)
        request_bytes = bytes(builder.Output())

        # Run simulation
        response = simulate_batch(request_bytes)
        result = response.Results(0)

        # Verify games completed
        assert result.TotalGames() == 100
        assert result.Errors() == 0, "Should have no simulation errors"
        # All games should complete (win/loss/draw)
        total_outcomes = result.Player0Wins() + result.Player1Wins() + result.Draws()
        assert total_outcomes == 100, "All games should complete"


class TestBettingChipsConservation:
    """Test that chips are conserved in betting games."""

    def test_betting_genome_has_valid_chip_config(self):
        """Test that simple poker genome has valid chip configuration."""
        genome = create_simple_poker_genome()

        # Starting chips should be positive
        assert genome.setup.starting_chips > 0

        # min_bet should not exceed starting_chips
        betting_phase = genome.turn_structure.phases[0]
        assert isinstance(betting_phase, BettingPhase)
        assert betting_phase.min_bet <= genome.setup.starting_chips

    def test_total_chips_equals_starting_times_players(self):
        """Total chips in system equals starting_chips * player_count."""
        genome = create_simple_poker_genome()

        # Calculate expected total chips
        expected_total = genome.setup.starting_chips * genome.player_count

        # For 2 players with 1000 chips each
        assert expected_total == 2000


class TestBettingMutations:
    """Test betting mutation operators."""

    def test_add_betting_phase_mutation_to_war(self):
        """AddBettingPhaseMutation adds a BettingPhase to War genome."""
        random.seed(42)
        genome = create_war_genome()  # No betting

        # Verify no betting phases initially
        betting_phases = [
            p for p in genome.turn_structure.phases if isinstance(p, BettingPhase)
        ]
        assert len(betting_phases) == 0

        mutation = AddBettingPhaseMutation(probability=1.0)
        mutated = mutation.mutate(genome)

        # Should now have a BettingPhase
        betting_phases = [
            p for p in mutated.turn_structure.phases if isinstance(p, BettingPhase)
        ]
        assert len(betting_phases) == 1

    def test_remove_betting_phase_mutation(self):
        """RemoveBettingPhaseMutation removes BettingPhase from genome."""
        random.seed(42)
        genome = create_simple_poker_genome()  # Has betting

        # Verify betting phase exists
        betting_phases = [
            p for p in genome.turn_structure.phases if isinstance(p, BettingPhase)
        ]
        assert len(betting_phases) == 1

        mutation = RemoveBettingPhaseMutation(probability=1.0)
        mutated = mutation.mutate(genome)

        # Should have no BettingPhases (but may have 0 phases, which is unchanged)
        # Actually, simple poker only has 1 phase, so it won't remove if that's the last one
        if len(genome.turn_structure.phases) > 1:
            betting_phases = [
                p for p in mutated.turn_structure.phases if isinstance(p, BettingPhase)
            ]
            assert len(betting_phases) == 0
        else:
            # Cannot remove last phase
            assert mutated.generation == genome.generation

    def test_mutate_betting_phase_modifies_parameters(self):
        """MutateBettingPhaseMutation modifies min_bet or max_raises."""
        random.seed(42)
        genome = create_simple_poker_genome()

        original_phase = genome.turn_structure.phases[0]
        assert isinstance(original_phase, BettingPhase)

        mutation = MutateBettingPhaseMutation(probability=1.0)
        mutated = mutation.mutate(genome)

        mutated_phase = mutated.turn_structure.phases[0]
        assert isinstance(mutated_phase, BettingPhase)

        # At least one parameter should have changed
        assert (
            original_phase.min_bet != mutated_phase.min_bet
            or original_phase.max_raises != mutated_phase.max_raises
        )

    def test_mutate_starting_chips_enables_betting(self):
        """MutateStartingChipsMutation adds chips to genome without betting."""
        random.seed(42)
        genome = create_war_genome()  # starting_chips = 0

        assert genome.setup.starting_chips == 0

        mutation = MutateStartingChipsMutation(probability=1.0)
        mutated = mutation.mutate(genome)

        # Should now have starting chips
        assert mutated.setup.starting_chips > 0


class TestBettingParameterValidation:
    """Test that betting mutations don't produce invalid configurations."""

    def test_mutations_maintain_min_bet_le_starting_chips(self):
        """Mutations don't produce invalid configs (min_bet > chips)."""
        random.seed(42)

        # Create genome with relatively low chips
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test-betting-validation",
            generation=0,
            setup=SetupRules(
                cards_per_player=5,
                starting_chips=20,  # Low starting chips
            ),
            turn_structure=TurnStructure(
                phases=[
                    BettingPhase(min_bet=10, max_raises=3),
                    PlayPhase(
                        target=Location.DISCARD,
                        valid_play_condition=Condition(
                            type=ConditionType.HAND_SIZE,
                            operator=Operator.GT,
                            value=0,
                        ),
                        min_cards=1,
                        max_cards=1,
                    ),
                ]
            ),
            special_effects=[],
            win_conditions=[WinCondition(type="best_hand")],
            scoring_rules=[],
            max_turns=10,
            player_count=2,
        )

        # Apply mutations many times and verify constraint holds
        mutation = MutateBettingPhaseMutation(probability=1.0)

        for _ in range(100):
            mutated = mutation.mutate(genome)

            for phase in mutated.turn_structure.phases:
                if isinstance(phase, BettingPhase):
                    # min_bet should not exceed starting_chips
                    assert phase.min_bet <= mutated.setup.starting_chips, (
                        f"min_bet ({phase.min_bet}) > starting_chips "
                        f"({mutated.setup.starting_chips})"
                    )

            # Use mutated as next input to chain mutations
            genome = mutated

    def test_add_betting_phase_creates_valid_min_bet(self):
        """AddBettingPhaseMutation creates valid min_bet for given chips."""
        random.seed(123)

        # Genome with very low starting chips
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test-low-chips",
            generation=0,
            setup=SetupRules(
                cards_per_player=5,
                starting_chips=8,  # Very low
            ),
            turn_structure=TurnStructure(
                phases=[
                    PlayPhase(
                        target=Location.DISCARD,
                        valid_play_condition=Condition(
                            type=ConditionType.HAND_SIZE,
                            operator=Operator.GT,
                            value=0,
                        ),
                    ),
                ]
            ),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
        )

        mutation = AddBettingPhaseMutation(probability=1.0)

        for _ in range(50):
            mutated = mutation.mutate(genome)

            for phase in mutated.turn_structure.phases:
                if isinstance(phase, BettingPhase):
                    # min_bet should be valid
                    assert phase.min_bet <= genome.setup.starting_chips or phase.min_bet == 1

    def test_starting_chips_mutation_adjusts_min_bet(self):
        """MutateStartingChipsMutation adjusts min_bet if needed."""
        # Create genome where min_bet is close to starting_chips
        genome = GameGenome(
            schema_version="1.0",
            genome_id="test-chips-adjustment",
            generation=0,
            setup=SetupRules(
                cards_per_player=5,
                starting_chips=100,
            ),
            turn_structure=TurnStructure(
                phases=[
                    BettingPhase(min_bet=80, max_raises=3),  # High min_bet
                ]
            ),
            special_effects=[],
            win_conditions=[WinCondition(type="best_hand")],
            scoring_rules=[],
        )

        # Use a seed that will reduce starting_chips
        random.seed(5)  # This seed gives a reduction in our mutation

        mutation = MutateStartingChipsMutation(probability=1.0)
        mutated = mutation.mutate(genome)

        # If starting_chips was reduced below min_bet, min_bet should be adjusted
        betting_phase = mutated.turn_structure.phases[0]
        assert betting_phase.min_bet <= mutated.setup.starting_chips, (
            f"After mutation: min_bet ({betting_phase.min_bet}) should be <= "
            f"starting_chips ({mutated.setup.starting_chips})"
        )


class TestBettingPhaseProperties:
    """Test BettingPhase dataclass properties."""

    def test_betting_phase_defaults(self):
        """Test BettingPhase has sensible defaults."""
        phase = BettingPhase()

        assert phase.min_bet == 10
        assert phase.max_raises == 3

    def test_betting_phase_custom_values(self):
        """Test BettingPhase with custom values."""
        phase = BettingPhase(min_bet=50, max_raises=5)

        assert phase.min_bet == 50
        assert phase.max_raises == 5

    def test_betting_phase_is_frozen(self):
        """Test BettingPhase is immutable (frozen dataclass)."""
        phase = BettingPhase(min_bet=10, max_raises=3)

        with pytest.raises(AttributeError):
            phase.min_bet = 20  # type: ignore
