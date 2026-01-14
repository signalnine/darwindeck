"""Tests for genome picker."""

import json
import pytest
from pathlib import Path
from darwindeck.playtest.picker import GenomePicker


class TestGenomePicker:
    """Tests for GenomePicker."""

    def test_finds_evolution_runs(self, tmp_path: Path):
        """Finds evolution run directories."""
        # Create test directories
        run1 = tmp_path / "evolution-20260114-120000"
        run1.mkdir()
        (run1 / "rank01_TestGame.json").write_text(json.dumps({
            "genome_id": "TestGame",
            "fitness": 0.85
        }))

        picker = GenomePicker(tmp_path)
        runs = picker.list_runs()

        assert len(runs) >= 1
        assert "20260114" in runs[0]["name"]

    def test_lists_genomes_in_run(self, tmp_path: Path):
        """Lists genomes within a run directory."""
        run_dir = tmp_path / "evolution-20260114-120000"
        run_dir.mkdir()

        # Create genome files
        for i, name in enumerate(["Alpha", "Beta", "Gamma"]):
            genome = {"genome_id": name, "fitness": 0.9 - i * 0.1}
            (run_dir / f"rank0{i+1}_{name}.json").write_text(json.dumps(genome))

        picker = GenomePicker(tmp_path)
        genomes = picker.list_genomes(run_dir)

        assert len(genomes) == 3
        assert genomes[0]["name"] == "Alpha"

    def test_loads_genome_file(self, tmp_path: Path):
        """Loads genome from JSON file."""
        genome_data = {
            "schema_version": "1.0",
            "genome_id": "TestGame",
            "generation": 1,
            "setup": {"cards_per_player": 5},
            "turn_structure": {"phases": []},
            "special_effects": [],
            "win_conditions": [{"type": "empty_hand"}],
            "scoring_rules": [],
            "max_turns": 100,
            "min_turns": 1,
            "player_count": 2,
        }
        genome_file = tmp_path / "test.json"
        genome_file.write_text(json.dumps(genome_data))

        picker = GenomePicker(tmp_path)
        genome, path = picker.load_genome(genome_file)

        assert genome.genome_id == "TestGame"
        assert path == genome_file

    def test_handles_no_runs(self, tmp_path: Path):
        """Returns empty list when no runs found."""
        picker = GenomePicker(tmp_path)
        runs = picker.list_runs()

        assert runs == []
