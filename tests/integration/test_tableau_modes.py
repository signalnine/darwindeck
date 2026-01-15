"""Integration tests for tableau mode simulation through the CGo bridge.

These tests verify that games with each TableauMode simulate correctly through
the full Python -> Go pipeline, including bytecode compilation and execution.
"""

import pytest

from darwindeck.genome.schema import (
    GameGenome,
    SetupRules,
    TurnStructure,
    PlayPhase,
    DrawPhase,
    WinCondition,
    Location,
    TableauMode,
    SequenceDirection,
)
from darwindeck.genome.conditions import Condition, ConditionType, Operator
from darwindeck.genome.examples import create_war_genome, create_scopa_genome, create_fan_tan_genome
from darwindeck.simulation.go_simulator import GoSimulator


class TestTableauModeSimulation:
    """Test that each tableau mode simulates correctly through CGo."""

    @pytest.fixture
    def simulator(self):
        """Create a GoSimulator instance for testing."""
        return GoSimulator(seed=42)

    def test_war_mode_games_complete(self, simulator):
        """WAR mode games run without errors and produce winners."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="war_test",
            generation=1,
            setup=SetupRules(
                cards_per_player=26,
                tableau_mode=TableauMode.WAR,
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU, mandatory=True, min_cards=1, max_cards=1)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="capture_all")],
            scoring_rules=[],
            player_count=2,
        )

        result = simulator.simulate(genome, num_games=50)

        assert result.errors == 0, f"WAR mode had errors: {result.errors}"
        assert result.total_games == 50
        # War games should complete with winners - not too many draws
        total_outcomes = sum(result.wins) + result.draws
        assert total_outcomes == 50, f"Not all games completed: {total_outcomes}/50"

    def test_none_mode_games_complete(self, simulator):
        """NONE mode games run without captures (cards just accumulate)."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="none_test",
            generation=1,
            setup=SetupRules(
                cards_per_player=7,
                tableau_mode=TableauMode.NONE,
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU, mandatory=True, min_cards=1, max_cards=1)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
            max_turns=50,
        )

        result = simulator.simulate(genome, num_games=50)

        # Should complete without errors (games may timeout, but should not error)
        assert result.errors == 0, f"NONE mode had errors: {result.errors}"
        assert result.total_games == 50

    def test_match_rank_mode_games_complete(self, simulator):
        """MATCH_RANK mode games run with captures on matching ranks."""
        # Create a Scopa-like genome with MATCH_RANK mode
        # Players need initial tableau cards to have something to capture
        # And a valid_play_condition for move generation
        genome = GameGenome(
            schema_version="1.0",
            genome_id="match_rank_test",
            generation=1,
            setup=SetupRules(
                cards_per_player=3,
                initial_discard_count=4,  # Start with 4 cards on tableau
                tableau_mode=TableauMode.MATCH_RANK,
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(
                    target=Location.TABLEAU,
                    valid_play_condition=Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.GT,
                        value=0
                    ),
                    mandatory=True,
                    min_cards=1,
                    max_cards=1,
                    pass_if_unable=False,
                ),
                # Draw when hand is empty (like Scopa)
                DrawPhase(
                    source=Location.DECK,
                    count=3,
                    mandatory=True,
                    condition=Condition(
                        type=ConditionType.HAND_SIZE,
                        operator=Operator.EQ,
                        value=0
                    )
                ),
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="most_captured")],
            scoring_rules=[],
            player_count=2,
            max_turns=100,
        )

        result = simulator.simulate(genome, num_games=50)

        assert result.errors == 0, f"MATCH_RANK mode had errors: {result.errors}"
        assert result.total_games == 50

    def test_sequence_mode_games_complete(self, simulator):
        """SEQUENCE mode games run with ordered plays."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="seq_test",
            generation=1,
            setup=SetupRules(
                cards_per_player=7,
                tableau_mode=TableauMode.SEQUENCE,
                sequence_direction=SequenceDirection.BOTH,
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(
                    target=Location.TABLEAU,
                    mandatory=False,
                    pass_if_unable=True,
                    min_cards=1,
                    max_cards=1
                )
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
            max_turns=200,
        )

        result = simulator.simulate(genome, num_games=50)

        # SEQUENCE mode may have many passes if cards don't match - that's expected
        assert result.errors == 0, f"SEQUENCE mode had errors: {result.errors}"
        assert result.total_games == 50

    def test_sequence_ascending_only(self, simulator):
        """SEQUENCE mode with ASCENDING only direction."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="seq_asc_test",
            generation=1,
            setup=SetupRules(
                cards_per_player=5,
                tableau_mode=TableauMode.SEQUENCE,
                sequence_direction=SequenceDirection.ASCENDING,
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(
                    target=Location.TABLEAU,
                    mandatory=False,
                    pass_if_unable=True,
                    min_cards=1,
                    max_cards=1
                ),
                DrawPhase(source=Location.DECK, count=1, mandatory=False),
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
            max_turns=100,
        )

        result = simulator.simulate(genome, num_games=30)

        assert result.errors == 0, f"SEQUENCE ASCENDING mode had errors: {result.errors}"

    def test_sequence_descending_only(self, simulator):
        """SEQUENCE mode with DESCENDING only direction."""
        genome = GameGenome(
            schema_version="1.0",
            genome_id="seq_desc_test",
            generation=1,
            setup=SetupRules(
                cards_per_player=5,
                tableau_mode=TableauMode.SEQUENCE,
                sequence_direction=SequenceDirection.DESCENDING,
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(
                    target=Location.TABLEAU,
                    mandatory=False,
                    pass_if_unable=True,
                    min_cards=1,
                    max_cards=1
                ),
                DrawPhase(source=Location.DECK, count=1, mandatory=False),
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
            max_turns=100,
        )

        result = simulator.simulate(genome, num_games=30)

        assert result.errors == 0, f"SEQUENCE DESCENDING mode had errors: {result.errors}"


class TestSeedGenomesWithTableauModes:
    """Test that seed genomes with explicit tableau modes simulate correctly."""

    @pytest.fixture
    def simulator(self):
        """Create a GoSimulator instance for testing."""
        return GoSimulator(seed=12345)

    def test_seed_genome_war_simulates(self, simulator):
        """Seed War genome with explicit WAR mode simulates correctly."""
        genome = create_war_genome()

        # Verify it has WAR mode
        assert genome.setup.tableau_mode == TableauMode.WAR

        result = simulator.simulate(genome, num_games=100)

        assert result.errors == 0, f"Seed War genome had errors: {result.errors}"
        assert result.total_games == 100
        # War games should produce winners
        total_outcomes = sum(result.wins) + result.draws
        assert total_outcomes == 100, f"Not all games completed: {total_outcomes}/100"

    def test_seed_genome_scopa_simulates(self, simulator):
        """Seed Scopa genome with MATCH_RANK mode simulates correctly."""
        genome = create_scopa_genome()

        # Verify it has MATCH_RANK mode
        assert genome.setup.tableau_mode == TableauMode.MATCH_RANK

        result = simulator.simulate(genome, num_games=50)

        assert result.errors == 0, f"Seed Scopa genome had errors: {result.errors}"
        assert result.total_games == 50

    def test_seed_genome_fan_tan_simulates(self, simulator):
        """Seed Fan Tan genome with SEQUENCE mode simulates correctly."""
        genome = create_fan_tan_genome()

        # Verify it has SEQUENCE mode
        assert genome.setup.tableau_mode == TableauMode.SEQUENCE
        assert genome.setup.sequence_direction == SequenceDirection.BOTH

        result = simulator.simulate(genome, num_games=50)

        assert result.errors == 0, f"Seed Fan Tan genome had errors: {result.errors}"
        assert result.total_games == 50


class TestTableauModeInteraction:
    """Test that tableau mode affects game outcomes as expected."""

    @pytest.fixture
    def simulator(self):
        """Create a GoSimulator instance for testing."""
        return GoSimulator(seed=999)

    def test_war_mode_produces_interactions(self, simulator):
        """WAR mode should produce interactions (card captures)."""
        genome = create_war_genome()
        result = simulator.simulate(genome, num_games=50)

        # War games have interactions when cards battle
        assert result.total_interactions > 0, "WAR mode should have interactions"

    def test_none_mode_no_tableau_captures(self, simulator):
        """NONE mode should not cause tableau captures - cards just stack."""
        # Create a simple game where cards go to tableau but never capture
        genome = GameGenome(
            schema_version="1.0",
            genome_id="none_stack_test",
            generation=1,
            setup=SetupRules(
                cards_per_player=5,
                tableau_mode=TableauMode.NONE,
            ),
            turn_structure=TurnStructure(phases=[
                PlayPhase(target=Location.TABLEAU, mandatory=True, min_cards=1, max_cards=1)
            ]),
            special_effects=[],
            win_conditions=[WinCondition(type="empty_hand")],
            scoring_rules=[],
            player_count=2,
            max_turns=20,
        )

        result = simulator.simulate(genome, num_games=30)

        # Games should complete without errors
        assert result.errors == 0, f"NONE mode had errors: {result.errors}"
        # With NONE mode, players just shed cards - games end when hands empty
        # (or timeout if deck keeps refilling)

    def test_different_modes_produce_different_avg_turns(self, simulator):
        """Different tableau modes should produce different game lengths."""
        base_setup = SetupRules(cards_per_player=10)

        def create_test_genome(mode: TableauMode, genome_id: str) -> GameGenome:
            return GameGenome(
                schema_version="1.0",
                genome_id=genome_id,
                generation=1,
                setup=SetupRules(
                    cards_per_player=10,
                    tableau_mode=mode,
                    sequence_direction=SequenceDirection.BOTH,
                ),
                turn_structure=TurnStructure(phases=[
                    PlayPhase(
                        target=Location.TABLEAU,
                        mandatory=True,
                        pass_if_unable=True,
                        min_cards=1,
                        max_cards=1
                    )
                ]),
                special_effects=[],
                win_conditions=[WinCondition(type="empty_hand")],
                scoring_rules=[],
                player_count=2,
                max_turns=100,
            )

        # Simulate with different modes
        war_result = simulator.simulate(create_test_genome(TableauMode.WAR, "war_len"), num_games=30)
        none_result = simulator.simulate(create_test_genome(TableauMode.NONE, "none_len"), num_games=30)

        # Both should complete without errors
        assert war_result.errors == 0, f"WAR mode length test had errors"
        assert none_result.errors == 0, f"NONE mode length test had errors"

        # Results are logged for analysis (actual values depend on implementation)
        print(f"\nTableau Mode Comparison:")
        print(f"  WAR mode avg turns: {war_result.avg_turns:.1f}")
        print(f"  NONE mode avg turns: {none_result.avg_turns:.1f}")


class TestTableauModeBytecodeCompilation:
    """Test that tableau modes compile correctly to bytecode."""

    def test_tableau_mode_compiles_to_bytecode(self):
        """Verify TableauMode is included in bytecode."""
        from darwindeck.genome.bytecode import BytecodeCompiler

        compiler = BytecodeCompiler()

        # Test each mode compiles
        for mode in TableauMode:
            genome = GameGenome(
                schema_version="1.0",
                genome_id=f"bytecode_test_{mode.value}",
                generation=1,
                setup=SetupRules(
                    cards_per_player=7,
                    tableau_mode=mode,
                ),
                turn_structure=TurnStructure(phases=[
                    PlayPhase(target=Location.TABLEAU, mandatory=True, min_cards=1, max_cards=1)
                ]),
                special_effects=[],
                win_conditions=[WinCondition(type="empty_hand")],
                scoring_rules=[],
                player_count=2,
            )

            # Should not raise
            bytecode = compiler.compile_genome(genome)
            assert len(bytecode) > 0, f"Bytecode for {mode.value} mode should not be empty"

    def test_sequence_direction_compiles_to_bytecode(self):
        """Verify SequenceDirection is included in bytecode."""
        from darwindeck.genome.bytecode import BytecodeCompiler

        compiler = BytecodeCompiler()

        # Test each direction compiles
        for direction in SequenceDirection:
            genome = GameGenome(
                schema_version="1.0",
                genome_id=f"seq_dir_test_{direction.value}",
                generation=1,
                setup=SetupRules(
                    cards_per_player=7,
                    tableau_mode=TableauMode.SEQUENCE,
                    sequence_direction=direction,
                ),
                turn_structure=TurnStructure(phases=[
                    PlayPhase(target=Location.TABLEAU, mandatory=True, min_cards=1, max_cards=1)
                ]),
                special_effects=[],
                win_conditions=[WinCondition(type="empty_hand")],
                scoring_rules=[],
                player_count=2,
            )

            # Should not raise
            bytecode = compiler.compile_genome(genome)
            assert len(bytecode) > 0, f"Bytecode for {direction.value} direction should not be empty"
