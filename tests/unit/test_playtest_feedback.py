"""Tests for FeedbackCollector."""

import json
import pytest
from pathlib import Path
from darwindeck.playtest.feedback import FeedbackCollector, PlaytestResult


class TestPlaytestResult:
    """Tests for PlaytestResult dataclass."""

    def test_to_dict(self):
        """Converts to dict correctly."""
        result = PlaytestResult(
            genome_id="TestGame",
            genome_path="path/to/genome.json",
            difficulty="greedy",
            seed=12345,
            winner="human",
            turns=23,
            rating=4,
            comment="Fun game",
        )

        d = result.to_dict()

        assert d["genome_id"] == "TestGame"
        assert d["seed"] == 12345
        assert d["rating"] == 4
        assert "timestamp" in d

    def test_optional_fields(self):
        """Handles optional fields."""
        result = PlaytestResult(
            genome_id="Test",
            genome_path="test.json",
            difficulty="random",
            seed=1,
            winner="ai",
            turns=10,
        )

        d = result.to_dict()

        assert d["rating"] is None
        assert d["comment"] == ""


class TestFeedbackCollector:
    """Tests for FeedbackCollector."""

    def test_saves_to_jsonl(self, tmp_path: Path):
        """Saves result as JSONL line."""
        output_file = tmp_path / "results.jsonl"
        collector = FeedbackCollector(output_file)

        result = PlaytestResult(
            genome_id="Test",
            genome_path="test.json",
            difficulty="random",
            seed=1,
            winner="human",
            turns=10,
            rating=5,
        )

        collector.save(result)

        # Read back
        lines = output_file.read_text().strip().split("\n")
        assert len(lines) == 1

        data = json.loads(lines[0])
        assert data["genome_id"] == "Test"
        assert data["rating"] == 5

    def test_appends_multiple(self, tmp_path: Path):
        """Appends multiple results."""
        output_file = tmp_path / "results.jsonl"
        collector = FeedbackCollector(output_file)

        for i in range(3):
            result = PlaytestResult(
                genome_id=f"Game{i}",
                genome_path=f"game{i}.json",
                difficulty="random",
                seed=i,
                winner="human",
                turns=10,
            )
            collector.save(result)

        lines = output_file.read_text().strip().split("\n")
        assert len(lines) == 3

    def test_creates_directory(self, tmp_path: Path):
        """Creates parent directory if needed."""
        output_file = tmp_path / "subdir" / "results.jsonl"
        collector = FeedbackCollector(output_file)

        result = PlaytestResult(
            genome_id="Test",
            genome_path="test.json",
            difficulty="random",
            seed=1,
            winner="human",
            turns=10,
        )

        collector.save(result)

        assert output_file.exists()
