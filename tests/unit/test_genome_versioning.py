"""Tests for genome schema versioning."""

import pytest
from cards_evolve.genome.versioning import SchemaVersion, validate_schema_version
from cards_evolve.genome.examples import create_war_genome
from dataclasses import replace


def test_current_schema_version() -> None:
    """Test current schema version is 1.0."""
    assert SchemaVersion.CURRENT == "1.0"


def test_validate_compatible_version() -> None:
    """Test compatible schema versions pass validation."""
    genome = create_war_genome()

    # Should not raise
    validate_schema_version(genome)


def test_validate_incompatible_version_raises() -> None:
    """Test incompatible schema versions raise error."""
    genome = create_war_genome()
    # Create genome with incompatible version
    bad_genome = replace(genome, schema_version="2.0")

    with pytest.raises(ValueError, match="Incompatible schema version"):
        validate_schema_version(bad_genome)
