"""Interactive genome picker."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Optional

from darwindeck.genome.schema import GameGenome
from darwindeck.genome.serialization import genome_from_dict


class GenomePicker:
    """Picks genomes from evolution runs."""

    def __init__(self, output_dir: Path | str = Path("output")):
        """Initialize with output directory."""
        self.output_dir = Path(output_dir)

    def list_runs(self) -> list[dict]:
        """List available evolution runs.

        Returns:
            List of dicts with 'name', 'path', 'top_genomes'
        """
        runs: list[dict] = []

        if not self.output_dir.exists():
            return runs

        # Find evolution-* directories
        for path in sorted(self.output_dir.glob("evolution-*"), reverse=True):
            if path.is_dir():
                genomes = self.list_genomes(path)
                top_names = [g["name"] for g in genomes[:3]]

                runs.append({
                    "name": path.name,
                    "path": path,
                    "top_genomes": top_names,
                })

        return runs[:10]  # Limit to recent 10

    def list_genomes(self, run_path: Path) -> list[dict]:
        """List genomes in a run directory.

        Returns:
            List of dicts with 'name', 'path', 'fitness'
        """
        genomes: list[dict] = []

        for path in sorted(run_path.glob("rank*.json")):
            try:
                with open(path) as f:
                    data = json.load(f)

                genomes.append({
                    "name": data.get("genome_id", path.stem),
                    "path": path,
                    "fitness": data.get("fitness", 0.0),
                })
            except (json.JSONDecodeError, KeyError):
                continue

        return genomes

    def load_genome(self, path: Path) -> tuple[GameGenome, Path]:
        """Load genome from file.

        Returns:
            Tuple of (GameGenome, path)
        """
        with open(path) as f:
            data = json.load(f)

        return genome_from_dict(data), path

    def interactive_pick(
        self,
        output_fn=print,
        input_fn=input,
    ) -> Optional[tuple[GameGenome, Path]]:
        """Interactive genome selection.

        Returns:
            Tuple of (GameGenome, path) or None if cancelled
        """
        runs = self.list_runs()

        if not runs:
            output_fn("No evolution runs found in output/")
            output_fn("Enter genome path manually:")
            try:
                path_str = input_fn("> ").strip()
                if path_str:
                    return self.load_genome(Path(path_str))
            except (EOFError, KeyboardInterrupt):
                pass
            return None

        # Show runs
        output_fn("\nRecent evolution runs:")
        for i, run in enumerate(runs):
            output_fn(f"  [{i+1}] {run['name']}")
            if run["top_genomes"]:
                output_fn(f"      Top: {', '.join(run['top_genomes'])}")

        output_fn(f"  [{len(runs)+1}] Enter path manually")
        output_fn("")

        try:
            choice_str = input_fn("Select run: ").strip()
            choice = int(choice_str)
        except (ValueError, EOFError, KeyboardInterrupt):
            return None

        if choice == len(runs) + 1:
            # Manual path
            try:
                path_str = input_fn("Enter path: ").strip()
                return self.load_genome(Path(path_str))
            except (EOFError, KeyboardInterrupt):
                return None

        if choice < 1 or choice > len(runs):
            output_fn("Invalid choice")
            return None

        # Show genomes in selected run
        run = runs[choice - 1]
        genomes = self.list_genomes(run["path"])

        output_fn(f"\nGenomes in {run['name']}:")
        for i, g in enumerate(genomes[:10]):
            output_fn(f"  [{i+1}] {g['name']} (fitness: {g['fitness']:.3f})")
        output_fn("")

        try:
            g_choice_str = input_fn("Select genome: ").strip()
            g_choice = int(g_choice_str)
        except (ValueError, EOFError, KeyboardInterrupt):
            return None

        if g_choice < 1 or g_choice > len(genomes):
            output_fn("Invalid choice")
            return None

        return self.load_genome(genomes[g_choice - 1]["path"])
