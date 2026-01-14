"""Feedback collection and storage."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Optional


@dataclass
class PlaytestResult:
    """Result of a playtest session."""

    genome_id: str
    genome_path: str
    difficulty: str
    seed: int
    winner: str  # "human", "ai", "stuck", "quit"
    turns: int
    rating: Optional[int] = None
    comment: str = ""
    quit_early: bool = False
    felt_broken: bool = False
    stuck_reason: Optional[str] = None
    replay_path: Optional[str] = None
    timestamp: str = field(default_factory=lambda: datetime.now().isoformat())

    def to_dict(self) -> dict:
        """Convert to dictionary for JSON serialization."""
        return {
            "timestamp": self.timestamp,
            "genome_id": self.genome_id,
            "genome_path": self.genome_path,
            "difficulty": self.difficulty,
            "seed": self.seed,
            "winner": self.winner,
            "turns": self.turns,
            "rating": self.rating,
            "comment": self.comment,
            "quit_early": self.quit_early,
            "felt_broken": self.felt_broken,
            "stuck_reason": self.stuck_reason,
            "replay_path": self.replay_path,
        }


class FeedbackCollector:
    """Collects and saves playtest feedback."""

    def __init__(self, output_path: Path | str):
        """Initialize with output file path."""
        self.output_path = Path(output_path)

    def save(self, result: PlaytestResult) -> None:
        """Save result to JSONL file (append)."""
        # Ensure parent directory exists
        self.output_path.parent.mkdir(parents=True, exist_ok=True)

        # Append as JSONL
        with open(self.output_path, "a") as f:
            f.write(json.dumps(result.to_dict()) + "\n")
