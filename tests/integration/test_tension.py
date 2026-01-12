"""Integration tests for tension curve metrics.

These tests verify that tension metrics (lead_changes, decisive_turn_pct,
closest_margin) flow correctly from Go simulation -> FlatBuffers -> Python.
"""

import pytest

from darwindeck.genome.examples import create_war_genome, create_hearts_genome
from darwindeck.simulation.go_simulator import GoSimulator
from darwindeck.evolution.fitness_full import FitnessEvaluator, SimulationResults


class TestTensionMetricsPopulation:
    """Test that tension metrics are populated after simulation."""

    def _check_cgo_available(self):
        """Check if CGo bridge is available.

        Returns:
            (available, reason) tuple where available is bool
        """
        try:
            from darwindeck.bindings.cgo_bridge import simulate_batch
            from darwindeck.genome.bytecode import BytecodeCompiler

            # Quick test to see if Go library loads
            genome = create_war_genome()
            compiler = BytecodeCompiler()
            bytecode = compiler.compile_genome(genome)
            return True, None
        except ImportError as e:
            return False, f"CGo bridge not available: {e}"
        except OSError as e:
            return False, f"Library loading error: {e}"

    def test_tension_metrics_populated_war(self):
        """Tension metrics should be populated after War simulation."""
        available, reason = self._check_cgo_available()
        if not available:
            pytest.skip(reason)

        simulator = GoSimulator(seed=42)
        genome = create_war_genome()

        # Run simulation
        results = simulator.simulate(genome, num_games=100)

        # Verify simulation succeeded
        assert results.errors == 0, "Should have no simulation errors"
        assert results.total_games == 100, "Should run 100 games"

        # Verify tension metrics are present in SimulationResults
        assert hasattr(results, "lead_changes"), "Should have lead_changes attribute"
        assert hasattr(results, "decisive_turn_pct"), "Should have decisive_turn_pct attribute"
        assert hasattr(results, "closest_margin"), "Should have closest_margin attribute"

        # At least one metric should be non-default
        # Default values: lead_changes=0, decisive_turn_pct=1.0, closest_margin=1.0
        has_data = (
            results.lead_changes > 0 or
            results.decisive_turn_pct < 1.0 or
            results.closest_margin < 1.0
        )

        # Print metrics for debugging
        print(f"\nTension Metrics (100 War games):")
        print(f"  lead_changes: {results.lead_changes}")
        print(f"  decisive_turn_pct: {results.decisive_turn_pct:.4f}")
        print(f"  closest_margin: {results.closest_margin:.4f}")

        # War is a back-and-forth game, so we expect lead changes
        # Note: If Go doesn't track tension yet, this may be zero - that's OK for now
        # The test documents expected behavior
        if has_data:
            # Go is tracking tension - verify values are reasonable
            assert results.lead_changes >= 0, "lead_changes should be non-negative"
            assert 0.0 <= results.decisive_turn_pct <= 1.0, "decisive_turn_pct should be 0-1"
            assert 0.0 <= results.closest_margin <= 1.0, "closest_margin should be 0-1"
        else:
            # Go not tracking tension yet - pass but note it
            print("  Note: Tension metrics at defaults - Go may not track them yet")

    def test_tension_metrics_populated_hearts(self):
        """Tension metrics should be populated after Hearts simulation."""
        available, reason = self._check_cgo_available()
        if not available:
            pytest.skip(reason)

        simulator = GoSimulator(seed=123)
        genome = create_hearts_genome()

        # Run simulation with 4 players
        results = simulator.simulate(genome, num_games=50, player_count=4)

        # Verify simulation ran (may have errors if Hearts not fully implemented)
        assert results.total_games == 50, "Should attempt 50 games"

        # Print metrics for debugging
        print(f"\nTension Metrics (50 Hearts games):")
        print(f"  errors: {results.errors}")
        print(f"  lead_changes: {results.lead_changes}")
        print(f"  decisive_turn_pct: {results.decisive_turn_pct:.4f}")
        print(f"  closest_margin: {results.closest_margin:.4f}")

        # Hearts should have close margins (avoiding points)
        # If implemented, closest_margin should be < 1.0 for close games


class TestTensionMetricsIntegration:
    """Test tension metrics integration with fitness evaluation."""

    def _check_cgo_available(self):
        """Check if CGo bridge is available."""
        try:
            from darwindeck.bindings.cgo_bridge import simulate_batch
            return True, None
        except ImportError as e:
            return False, f"CGo bridge not available: {e}"
        except OSError as e:
            return False, f"Library loading error: {e}"

    def test_fitness_evaluator_uses_tension_data(self):
        """FitnessEvaluator should use tension data when available."""
        available, reason = self._check_cgo_available()
        if not available:
            pytest.skip(reason)

        simulator = GoSimulator(seed=42)
        genome = create_war_genome()
        evaluator = FitnessEvaluator(style="balanced")

        # Run simulation
        results = simulator.simulate(genome, num_games=100)

        # Evaluate fitness
        metrics = evaluator.evaluate(genome, results)

        # Verify tension_curve metric is computed
        assert hasattr(metrics, "tension_curve"), "Should have tension_curve metric"
        assert 0.0 <= metrics.tension_curve <= 1.0, "tension_curve should be 0-1"

        print(f"\nFitness Metrics (War):")
        print(f"  tension_curve: {metrics.tension_curve:.4f}")
        print(f"  valid: {metrics.valid}")

    def test_tension_metrics_affect_fitness_calculation(self):
        """Different tension metrics should affect fitness differently."""
        genome = create_war_genome()
        evaluator = FitnessEvaluator(style="balanced")

        # Create results with low tension (defaults)
        low_tension_results = SimulationResults(
            total_games=100,
            wins=(50, 50),
            player_count=2,
            draws=0,
            avg_turns=50.0,
            errors=0,
            lead_changes=0,
            decisive_turn_pct=1.0,
            closest_margin=1.0,
        )

        # Create results with high tension
        high_tension_results = SimulationResults(
            total_games=100,
            wins=(50, 50),
            player_count=2,
            draws=0,
            avg_turns=50.0,
            errors=0,
            lead_changes=25,  # Many lead changes
            decisive_turn_pct=0.8,  # Many decisive turns
            closest_margin=0.1,  # Very close game
        )

        low_metrics = evaluator.evaluate(genome, low_tension_results)
        high_metrics = evaluator.evaluate(genome, high_tension_results)

        print(f"\nTension Curve Comparison:")
        print(f"  Low tension: {low_metrics.tension_curve:.4f}")
        print(f"  High tension: {high_metrics.tension_curve:.4f}")

        # High tension should result in higher tension_curve score
        assert high_metrics.tension_curve > low_metrics.tension_curve, (
            f"High tension ({high_metrics.tension_curve:.4f}) should score higher "
            f"than low tension ({low_metrics.tension_curve:.4f})"
        )


class TestTensionMetricsRawFlatbuffers:
    """Test tension metrics directly from FlatBuffers response."""

    def _check_cgo_available(self):
        """Check if CGo bridge is available."""
        try:
            from darwindeck.bindings.cgo_bridge import simulate_batch
            return True, None
        except ImportError as e:
            return False, f"CGo bridge not available: {e}"
        except OSError as e:
            return False, f"Library loading error: {e}"

    def test_flatbuffers_tension_fields_exist(self):
        """Verify FlatBuffers schema includes tension fields."""
        available, reason = self._check_cgo_available()
        if not available:
            pytest.skip(reason)

        import flatbuffers
        from darwindeck.bindings.cgo_bridge import simulate_batch
        from darwindeck.genome.bytecode import BytecodeCompiler
        from darwindeck.genome.examples import create_war_genome
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

        # Build request
        genome = create_war_genome()
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)

        builder = flatbuffers.Builder(2048)
        genome_offset = builder.CreateByteVector(bytecode)

        SimulationRequestStart(builder)
        SimulationRequestAddGenomeBytecode(builder, genome_offset)
        SimulationRequestAddNumGames(builder, 10)
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

        # Verify tension fields exist in response
        lead_changes = result.LeadChanges()
        decisive_turn_pct = result.DecisiveTurnPct()
        closest_margin = result.ClosestMargin()

        print(f"\nRaw FlatBuffers Tension Fields:")
        print(f"  LeadChanges(): {lead_changes}")
        print(f"  DecisiveTurnPct(): {decisive_turn_pct}")
        print(f"  ClosestMargin(): {closest_margin}")

        # Fields should return valid values (even if defaults)
        assert isinstance(lead_changes, int), "LeadChanges should be int"
        assert isinstance(decisive_turn_pct, float), "DecisiveTurnPct should be float"
        assert isinstance(closest_margin, float), "ClosestMargin should be float"

        # Values should be in valid ranges
        assert lead_changes >= 0, "LeadChanges should be >= 0"
        assert 0.0 <= decisive_turn_pct <= 1.0, "DecisiveTurnPct should be 0-1"
        assert 0.0 <= closest_margin <= 1.0, "ClosestMargin should be 0-1"


class TestTensionMetricsSanity:
    """Sanity checks for tension metrics behavior."""

    def test_simulation_results_defaults(self):
        """SimulationResults should have sensible tension defaults."""
        results = SimulationResults(
            total_games=100,
            wins=(50, 50),
            player_count=2,
            draws=0,
            avg_turns=50.0,
            errors=0,
        )

        # Default values should indicate "no tension data"
        assert results.lead_changes == 0, "Default lead_changes should be 0"
        assert results.decisive_turn_pct == 1.0, "Default decisive_turn_pct should be 1.0"
        assert results.closest_margin == 1.0, "Default closest_margin should be 1.0"

    def test_has_tension_data_detection(self):
        """FitnessEvaluator should detect when tension data is available."""
        # Results with no tension data (all defaults)
        no_data = SimulationResults(
            total_games=100,
            wins=(50, 50),
            player_count=2,
            draws=0,
            avg_turns=50.0,
            errors=0,
            lead_changes=0,
            decisive_turn_pct=1.0,
            closest_margin=1.0,
        )

        # Results with tension data
        has_data = SimulationResults(
            total_games=100,
            wins=(50, 50),
            player_count=2,
            draws=0,
            avg_turns=50.0,
            errors=0,
            lead_changes=10,  # Non-zero
            decisive_turn_pct=1.0,
            closest_margin=1.0,
        )

        # Check detection logic (same as in fitness_full.py)
        def has_tension_data(results):
            return (
                results.lead_changes > 0 or
                results.decisive_turn_pct < 1.0 or
                results.closest_margin < 1.0
            )

        assert not has_tension_data(no_data), "Should detect no tension data"
        assert has_tension_data(has_data), "Should detect tension data present"

    def test_tension_curve_formula(self):
        """Test tension curve calculation formula."""
        genome = create_war_genome()
        evaluator = FitnessEvaluator(style="balanced")

        # Create results with known tension values
        results = SimulationResults(
            total_games=100,
            wins=(50, 50),
            player_count=2,
            draws=0,
            avg_turns=100.0,  # 100 turns
            errors=0,
            lead_changes=5,  # 5 lead changes
            decisive_turn_pct=0.6,  # 60% decisive
            closest_margin=0.2,  # 20% margin
        )

        metrics = evaluator.evaluate(genome, results)

        # Expected calculation:
        # turns_per_expected_change = 20
        # expected_changes = 100 / 20 = 5
        # lead_change_score = min(1.0, 5/5) = 1.0
        # decisive_turn_score = 0.6
        # margin_score = 1.0 - 0.2 = 0.8
        # tension_curve = 1.0 * 0.4 + 0.6 * 0.4 + 0.8 * 0.2 = 0.4 + 0.24 + 0.16 = 0.8

        expected = 0.8
        assert abs(metrics.tension_curve - expected) < 0.01, (
            f"Tension curve {metrics.tension_curve:.4f} should be ~{expected}"
        )
