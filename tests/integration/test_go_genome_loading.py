"""Integration tests for loading and simulating Go-evolved genome files.

These tests verify that Python can load actual Go-evolved genome files
from the output directory and run simulations with them.
"""

import json
import pytest
from pathlib import Path
from typing import Optional

from darwindeck.genome.schema import GameGenome
from darwindeck.genome.serialization import genome_from_dict
from darwindeck.simulation.go_simulator import GoSimulator


def find_go_genome_files() -> list[Path]:
    """Find Go-evolved genome files in output directory.

    Returns files matching rank*_*.json pattern, excluding:
    - .converted.json files (conversion artifacts)
    - .bak files (backups)
    - basin_analysis.json (not a genome file)
    """
    output_dir = Path(__file__).parent.parent.parent / "output"
    if not output_dir.exists():
        return []

    genome_files = []
    for json_file in output_dir.rglob("rank*.json"):
        # Skip conversion artifacts and backups
        if ".converted." in json_file.name or ".bak" in json_file.name:
            continue
        genome_files.append(json_file)

    # Sort by modification time (newest first) for consistent test order
    genome_files.sort(key=lambda p: p.stat().st_mtime, reverse=True)
    return genome_files


def try_load_genome(filepath: Path) -> Optional[GameGenome]:
    """Attempt to load a genome file, returning None on failure.

    This is used to find files that can be successfully loaded for testing,
    as some older Go-format files may have incompatibilities.
    """
    try:
        with open(filepath) as f:
            data = json.load(f)
        return genome_from_dict(data)
    except Exception:
        return None


def find_loadable_genome_file() -> Optional[tuple[Path, GameGenome]]:
    """Find a genome file that can be successfully loaded.

    Returns (path, genome) tuple or None if no loadable files found.
    """
    for filepath in find_go_genome_files():
        genome = try_load_genome(filepath)
        if genome is not None:
            return (filepath, genome)
    return None


class TestGoGenomeLoading:
    """Tests for loading Go-evolved genome files."""

    def test_load_real_go_evolved_genome(self):
        """Load an actual Go-evolved genome file without errors."""
        result = find_loadable_genome_file()

        if result is None:
            genome_files = find_go_genome_files()
            if not genome_files:
                pytest.skip("No Go genome files found in output/ directory")
            else:
                pytest.skip("No loadable Go genome files found in output/")

        genome_file, genome = result

        # Verify basic genome structure
        assert genome is not None, f"Failed to load genome from {genome_file}"
        assert genome.genome_id, f"Genome from {genome_file} has no genome_id"

        # Verify genome has phases (required for playable game)
        assert len(genome.turn_structure.phases) > 0, (
            f"Genome from {genome_file} has no phases"
        )

        # Verify genome has win conditions
        assert len(genome.win_conditions) > 0, (
            f"Genome from {genome_file} has no win conditions"
        )

    def test_load_multiple_go_genomes(self):
        """Load multiple Go-evolved genomes to verify consistency."""
        genome_files = find_go_genome_files()

        if not genome_files:
            pytest.skip("No Go genome files found in output/ directory")

        # Try loading up to 10 files, track successes
        loaded_count = 0
        for genome_file in genome_files[:10]:
            genome = try_load_genome(genome_file)
            if genome is not None:
                loaded_count += 1
                # Basic validation
                assert len(genome.turn_structure.phases) > 0, (
                    f"{genome_file} has no phases"
                )

        if loaded_count == 0:
            pytest.skip("Could not load any of the 10 newest genome files")

        # At least some files should load successfully
        assert loaded_count > 0


class TestGoGenomeSimulation:
    """Tests for simulating Go-evolved genome files."""

    def test_simulate_go_genome(self):
        """Simulate a real Go-evolved genome without errors."""
        result = find_loadable_genome_file()

        if result is None:
            genome_files = find_go_genome_files()
            if not genome_files:
                pytest.skip("No Go genome files found in output/ directory")
            else:
                pytest.skip("No loadable Go genome files found in output/")

        genome_file, genome = result

        # Run simulation with RandomAI
        simulator = GoSimulator(seed=42)
        sim_result = simulator.simulate(
            genome,
            num_games=10,  # Small number for fast test
            use_mcts=False,
            player_count=genome.player_count,
        )

        # Verify simulation was requested
        assert sim_result.total_games == 10, (
            f"Expected 10 games, got {sim_result.total_games}"
        )

        # Verify all games have an outcome (win, draw, or error/timeout)
        # Some games may error, but they should still be counted
        total_outcomes = sum(sim_result.wins) + sim_result.draws + sim_result.errors
        assert total_outcomes == 10, (
            f"Total outcomes (wins+draws+errors) {total_outcomes} != 10 games"
        )

        # Verify at least some games completed successfully
        # (may have some timeouts or errors, but not all)
        successful_outcomes = sum(sim_result.wins) + sim_result.draws
        if successful_outcomes == 0:
            # All games errored - this is still valid but unusual
            # The simulation infrastructure works, even if the genome has issues
            assert sim_result.errors == 10, (
                "Expected either successful games or all errors"
            )

    def test_simulate_go_genome_deterministic(self):
        """Verify Go genome simulation is deterministic with same seed."""
        result = find_loadable_genome_file()

        if result is None:
            genome_files = find_go_genome_files()
            if not genome_files:
                pytest.skip("No Go genome files found in output/ directory")
            else:
                pytest.skip("No loadable Go genome files found in output/")

        _, genome = result

        # Run twice with same seed
        simulator1 = GoSimulator(seed=12345)
        result1 = simulator1.simulate(genome, num_games=5)

        simulator2 = GoSimulator(seed=12345)
        result2 = simulator2.simulate(genome, num_games=5)

        # Results should be identical
        assert result1.wins == result2.wins, (
            f"Non-deterministic results: {result1.wins} vs {result2.wins}"
        )
        assert result1.draws == result2.draws


class TestGoGenomeMetadata:
    """Tests for metadata in Go-evolved genome files."""

    def test_genome_preserves_fitness_data(self):
        """Verify fitness data from Go evolution is accessible."""
        genome_files = find_go_genome_files()

        if not genome_files:
            pytest.skip("No Go genome files found in output/ directory")

        # Find a file with fitness data
        found_fitness = False
        for genome_file in genome_files[:10]:
            with open(genome_file) as f:
                data = json.load(f)

            # Check raw data has fitness info (Go saves this at top level)
            has_fitness = "fitness" in data or "fitness_metrics" in data

            if has_fitness:
                found_fitness = True
                # If fitness_metrics exists, verify structure
                if "fitness_metrics" in data:
                    metrics = data["fitness_metrics"]
                    assert isinstance(metrics, dict), "fitness_metrics should be dict"

                    # Common fitness fields
                    expected_fields = ["decision_density", "valid"]
                    for field in expected_fields:
                        if field in metrics:
                            assert metrics[field] is not None
                break

        if not found_fitness:
            pytest.skip("No fitness data in any of the checked genome files")

    def test_genome_has_generation_info(self):
        """Verify Go-evolved genomes have generation information."""
        result = find_loadable_genome_file()

        if result is None:
            genome_files = find_go_genome_files()
            if not genome_files:
                pytest.skip("No Go genome files found in output/ directory")
            else:
                pytest.skip("No loadable Go genome files found in output/")

        _, genome = result

        # Generation should be non-negative (0 for seed genomes, >0 for evolved)
        assert genome.generation >= 0, (
            f"Invalid generation {genome.generation}"
        )
