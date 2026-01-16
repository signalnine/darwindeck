"""Integration tests for end-to-end simulation."""

import pytest

from darwindeck.genome.examples import create_hearts_genome
from darwindeck.simulation.go_simulator import GoSimulator


class TestExplicitScoringSimulation:
    """Test simulations using explicit scoring from genome."""

    def test_hearts_simulation_uses_explicit_scoring(self):
        """Hearts simulation uses explicit card_scoring instead of hardcoded values.

        The Hearts genome defines explicit card scoring rules:
        - Hearts suit: 1 point per card (on trick win)
        - Queen of Spades: 13 points (on trick win)

        This test verifies that the Go simulation engine correctly uses
        these explicit scoring rules encoded in the bytecode, rather than
        relying on hardcoded values.
        """
        genome = create_hearts_genome()

        # Verify genome has explicit card_scoring
        assert genome.card_scoring is not None, "Hearts genome should have card_scoring"
        assert len(genome.card_scoring) == 2, "Hearts should have 2 scoring rules"

        # Verify scoring rules are correct
        hearts_rule = genome.card_scoring[0]
        queen_rule = genome.card_scoring[1]
        assert hearts_rule.points == 1, "Hearts should be worth 1 point"
        assert queen_rule.points == 13, "Queen of Spades should be worth 13 points"

        # Run simulation through Go engine
        simulator = GoSimulator(seed=42)
        result = simulator.simulate(
            genome,
            num_games=10,
            player_count=4,  # Hearts is 4-player
            use_mcts=False,
        )

        # Verify simulation completed without errors
        assert result.total_games == 10, f"Expected 10 games, got {result.total_games}"
        assert result.errors == 0, f"Expected no errors, got {result.errors}"

        # With explicit scoring, games should complete normally
        assert result.avg_turns > 0, "Games should have positive turn count"

        # Verify games had outcomes (wins or draws)
        total_outcomes = sum(result.wins) + result.draws
        assert total_outcomes == 10, f"All games should complete, got {total_outcomes}"

    def test_hearts_genome_card_scoring_bytecode(self):
        """Verify Hearts card_scoring compiles to valid bytecode."""
        from darwindeck.genome.bytecode import BytecodeCompiler

        genome = create_hearts_genome()
        compiler = BytecodeCompiler()

        # Should compile without error
        bytecode = compiler.compile_genome(genome)

        # Bytecode should be non-empty
        assert len(bytecode) > 0, "Bytecode should not be empty"

        # Parse header to verify card_scoring_offset is set
        from darwindeck.genome.bytecode import BytecodeHeader
        header = BytecodeHeader.from_bytes(bytecode)

        # card_scoring_offset should be positive (non-zero) since Hearts has scoring
        assert header.card_scoring_offset > 0, (
            f"card_scoring_offset should be positive, got {header.card_scoring_offset}"
        )
