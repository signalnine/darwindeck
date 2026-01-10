"""Schema versioning for genome compatibility."""

from typing import Set
from darwindeck.genome.schema import GameGenome


class SchemaVersion:
    """Schema version constants."""

    CURRENT = "1.0"
    COMPATIBLE: Set[str] = {"1.0"}


def validate_schema_version(genome: GameGenome) -> None:
    """Validate genome schema version is compatible.

    Args:
        genome: The genome to validate

    Raises:
        ValueError: If schema version is not compatible
    """
    if genome.schema_version not in SchemaVersion.COMPATIBLE:
        raise ValueError(
            f"Incompatible schema version: {genome.schema_version}. "
            f"Compatible versions: {SchemaVersion.COMPATIBLE}"
        )
