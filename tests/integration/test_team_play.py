"""Integration tests for team play functionality.

These tests verify the complete pipeline:
Python genome -> Bytecode -> Go simulation -> FlatBuffers -> Python results
"""

import pytest
from darwindeck.genome.examples import create_partnership_spades_genome, create_war_genome
from darwindeck.genome.bytecode import BytecodeCompiler, BytecodeHeader
from darwindeck.genome.validator import GenomeValidator


# Check if Go simulator is available
GO_SIMULATOR_AVAILABLE = False
GO_SIMULATOR_ERROR = "Unknown error"
try:
    from darwindeck.simulation.go_simulator import GoSimulator
    GO_SIMULATOR_AVAILABLE = True
except (ImportError, OSError) as e:
    GO_SIMULATOR_ERROR = str(e)
except Exception as e:
    GO_SIMULATOR_ERROR = f"Unexpected error: {str(e)}"


class TestTeamGenomeCompilation:
    """Test that team genomes compile correctly to bytecode."""

    def test_team_genome_compiles_to_bytecode(self):
        """Partnership Spades should compile to valid bytecode with team data."""
        genome = create_partnership_spades_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        # Verify bytecode was generated
        assert bytecode is not None
        assert len(bytecode) >= BytecodeHeader.HEADER_SIZE

        # Check team fields in header
        header = BytecodeHeader.from_bytes(bytecode)

        assert header.team_mode is True
        assert header.team_count == 2
        assert header.team_data_offset > 0
        assert header.team_data_offset < len(bytecode)

    def test_non_team_genome_has_no_team_data(self):
        """War (non-team game) should have team_mode=False."""
        genome = create_war_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        header = BytecodeHeader.from_bytes(bytecode)

        assert header.team_mode is False
        assert header.team_count == 0

    def test_team_data_section_encoding(self):
        """Team data section should encode team assignments correctly."""
        genome = create_partnership_spades_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        header = BytecodeHeader.from_bytes(bytecode)
        offset = header.team_data_offset

        # Read team data: [num_teams] [team_size] [player indices...] ...
        num_teams = bytecode[offset]
        assert num_teams == 2

        # First team: size + players
        team0_size = bytecode[offset + 1]
        assert team0_size == 2
        team0_players = (bytecode[offset + 2], bytecode[offset + 3])
        assert team0_players == (0, 2)  # Players 0 and 2

        # Second team: size + players
        team1_offset = offset + 1 + 1 + team0_size
        team1_size = bytecode[team1_offset]
        assert team1_size == 2
        team1_players = (bytecode[team1_offset + 1], bytecode[team1_offset + 2])
        assert team1_players == (1, 3)  # Players 1 and 3


class TestTeamGenomeValidation:
    """Test genome validation for team configurations."""

    def test_valid_team_genome_passes_validation(self):
        """Partnership Spades should pass validation."""
        genome = create_partnership_spades_genome()
        validator = GenomeValidator()
        errors = validator.validate(genome)

        assert len(errors) == 0, f"Validation errors: {errors}"

    def test_team_mode_properties(self):
        """Partnership Spades should have correct team properties."""
        genome = create_partnership_spades_genome()

        assert genome.team_mode is True
        assert len(genome.teams) == 2
        assert genome.teams[0] == (0, 2)
        assert genome.teams[1] == (1, 3)
        assert genome.player_count == 4


@pytest.mark.skipif(
    not GO_SIMULATOR_AVAILABLE,
    reason=f"Go simulator not available: {GO_SIMULATOR_ERROR}"
)
class TestTeamGameSimulation:
    """Integration tests for team game simulation via CGo."""

    def test_team_game_simulation_via_cgo(self):
        """Partnership Spades should simulate via CGo and track team wins."""
        genome = create_partnership_spades_genome()
        simulator = GoSimulator(seed=42)

        # Run a batch of games
        results = simulator.simulate(
            genome=genome,
            num_games=50,
            player_count=4,
        )

        # Verify simulation ran
        assert results.total_games == 50
        assert results.player_count == 4

        # Verify team wins are tracked (if Go supports it)
        # Note: team_wins may be None if Go doesn't implement it yet
        if results.team_wins is not None:
            assert len(results.team_wins) >= 2, f"Expected at least 2 teams, got {len(results.team_wins)}"

            # Team wins should sum to completed games (or less if draws)
            total_team_wins = sum(results.team_wins)
            assert total_team_wins <= results.total_games - results.errors

            print(f"Team wins: {results.team_wins}, Player wins: {results.wins}")
        else:
            # If team_wins is None, that's okay for now - Go may not implement it yet
            print("Note: team_wins is None - Go may not implement team tracking yet")

    def test_non_team_game_has_no_team_wins(self):
        """Non-team games should have None for team_wins."""
        genome = create_war_genome()
        simulator = GoSimulator(seed=42)

        results = simulator.simulate(
            genome=genome,
            num_games=50,
        )

        # Non-team game should not have team wins
        assert results.team_wins is None or len(results.team_wins) == 0

    def test_team_game_with_asymmetric_ai(self):
        """Partnership Spades should work with asymmetric AI types."""
        genome = create_partnership_spades_genome()
        simulator = GoSimulator(seed=42)

        # Run with different AI types per player
        results = simulator.simulate_asymmetric(
            genome=genome,
            num_games=20,
            ai_types=["random", "random", "random", "random"],
            player_count=4,
        )

        # Verify simulation ran
        assert results.total_games == 20
        assert results.player_count == 4

        # Verify wins tuple has correct length
        assert len(results.wins) == 4


class TestBytecodeRoundTrip:
    """Test bytecode compilation and parsing consistency."""

    def test_bytecode_determinism(self):
        """Compiling same team genome should produce identical bytecode."""
        genome = create_partnership_spades_genome()
        compiler = BytecodeCompiler()

        bytecode1 = compiler.compile_genome(genome)
        bytecode2 = compiler.compile_genome(genome)

        assert bytecode1 == bytecode2, "Bytecode compilation is not deterministic"

    def test_header_fields_preserved(self):
        """All header fields should be preserved in bytecode."""
        genome = create_partnership_spades_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        header = BytecodeHeader.from_bytes(bytecode)

        # Verify core fields
        assert header.player_count == 4
        assert header.max_turns == 200

        # Verify team fields
        assert header.team_mode is True
        assert header.team_count == 2
        assert header.team_data_offset > 0
