#!/usr/bin/env python3
"""One-time migration: Convert Python v1 format genomes to Go canonical format.

Creates .bak backup files before modifying. Run with --dry-run first to preview changes.

Usage:
    # Preview changes (no modifications)
    uv run python scripts/migrate_genomes.py --dry-run --path output/

    # Migrate all genomes in output directory
    uv run python scripts/migrate_genomes.py --path output/

    # Migrate specific pattern
    uv run python scripts/migrate_genomes.py --path seeds/ --pattern "*.json"
"""

import json
import shutil
import sys
from pathlib import Path
from typing import Any, Optional


def detect_format(data: dict) -> str:
    """Detect genome format version.

    Returns:
        "go": Go format (already has genome wrapper and/or schema_version 2.0)
        "python_v1": Python v1 format (schema_version 1.0, PascalCase types)
        "unknown": Cannot determine format
    """
    # Go wrapped format has "genome" key at root with nested genome data
    if "genome" in data and isinstance(data.get("genome"), dict):
        return "go"

    # Go format uses schema_version 2.0
    if data.get("schema_version") == "2.0":
        return "go"

    # Python v1 format has schema_version 1.0 and genome_id at root
    if data.get("schema_version") == "1.0" and data.get("genome_id"):
        return "python_v1"

    # Check phase types to distinguish - Go uses lowercase, Python uses PascalCase
    phases = data.get("turn_structure", {}).get("phases", [])
    if phases:
        first_phase_type = phases[0].get("type", "")
        # PascalCase types indicate Python v1
        if first_phase_type and first_phase_type[0].isupper() and "Phase" in first_phase_type:
            return "python_v1"
        # Lowercase types or "data" key indicate Go format
        if first_phase_type.islower() or "data" in phases[0]:
            return "go"

    return "unknown"


def convert_python_to_go(data: dict) -> dict:
    """Convert Python v1 format to Go canonical format."""
    # Extract fitness info from root if present (preserve during migration)
    fitness = data.get("fitness", 0.0)
    fitness_metrics = data.get("fitness_metrics", {})
    skill_evaluation = data.get("skill_evaluation", {})

    # Build Go-style genome
    go_genome = {
        "schema_version": "2.0",
        "genome": {
            "name": data.get("genome_id", "Unknown"),
            "metadata": {
                "genome_id": data.get("genome_id", "unknown"),
                "generation": data.get("generation", 0),
                "parent_ids": []
            },
            "setup": _convert_setup(data.get("setup", {})),
            "turn_structure": _convert_turn_structure(data.get("turn_structure", {}), data.get("max_turns", 100)),
            "win_conditions": data.get("win_conditions", [{"type": "empty_hand"}]),
            "effects": [_convert_effect(e) for e in data.get("special_effects", [])],
            "player_count": data.get("player_count", 2),
        },
        "fitness": fitness if isinstance(fitness, (int, float)) else 0.0,
        "fitness_metrics": fitness_metrics
    }

    # Preserve skill evaluation if present
    if skill_evaluation:
        go_genome["skill_evaluation"] = skill_evaluation

    # Preserve scoring rules if present
    if data.get("scoring_rules"):
        go_genome["genome"]["scoring_rules"] = data["scoring_rules"]

    return go_genome


def _convert_setup(setup: dict) -> dict:
    """Convert setup to Go format."""
    result = {
        "cards_per_player": setup.get("cards_per_player", 5),
        "starting_chips": setup.get("starting_chips", 0),
    }

    # Optional fields - only include if non-default
    if setup.get("initial_deck") and setup["initial_deck"] != "standard_52":
        result["initial_deck"] = setup["initial_deck"]
    if setup.get("initial_discard_count", 0) > 0:
        result["initial_discard_count"] = setup["initial_discard_count"]

    # Convert trump_suit from UPPERCASE to lowercase
    if setup.get("trump_suit"):
        result["trump_suit"] = setup["trump_suit"].lower()

    return result


def _convert_turn_structure(ts: dict, max_turns: int) -> dict:
    """Convert turn structure to Go format with nested phase data."""
    result = {
        "phases": [_convert_phase(p) for p in ts.get("phases", [])],
        "max_turns": max_turns,
    }

    # Optional fields
    if ts.get("is_trick_based"):
        result["is_trick_based"] = True
    if ts.get("tricks_per_hand"):
        result["tricks_per_hand"] = ts["tricks_per_hand"]

    # Tableau mode from setup (in Python format) goes to turn_structure in Go
    return result


def _convert_phase(phase: dict) -> dict:
    """Convert phase to Go format with nested 'data' field."""
    phase_type = phase.get("type", "Unknown")

    # Remove "Phase" suffix and convert to lowercase
    # "DrawPhase" -> "draw", "PlayPhase" -> "play", etc.
    go_type = phase_type.replace("Phase", "").lower()

    if go_type == "draw":
        return {
            "type": "draw",
            "data": {
                "source": _to_lowercase(phase.get("source", "DECK")),
                "count": phase.get("count", 1),
                "mandatory": phase.get("mandatory", True),
                "condition": _convert_condition(phase.get("condition"))
            }
        }

    elif go_type == "play":
        return {
            "type": "play",
            "data": {
                "target": _to_lowercase(phase.get("target", "DISCARD")),
                "min_cards": phase.get("min_cards", 1),
                "max_cards": phase.get("max_cards", 1),
                "mandatory": phase.get("mandatory", True),
                "pass_if_unable": phase.get("pass_if_unable", True),
                "valid_play_condition": _convert_condition(phase.get("valid_play_condition"))
            }
        }

    elif go_type == "discard":
        return {
            "type": "discard",
            "data": {
                "target": _to_lowercase(phase.get("target", "DISCARD")),
                "count": phase.get("count", 1),
                "mandatory": phase.get("mandatory", False),
            }
        }

    elif go_type == "trick":
        data = {
            "lead_suit_required": phase.get("lead_suit_required", True),
            "high_card_wins": phase.get("high_card_wins", True),
        }
        if phase.get("trump_suit"):
            data["trump_suit"] = _to_lowercase(phase["trump_suit"])
        if phase.get("breaking_suit"):
            data["breaking_suit"] = _to_lowercase(phase["breaking_suit"])
        return {"type": "trick", "data": data}

    elif go_type == "claim":
        return {
            "type": "claim",
            "data": {
                "min_cards": phase.get("min_cards", 1),
                "max_cards": phase.get("max_cards", 4),
                "sequential_rank": phase.get("sequential_rank", True),
                "allow_challenge": phase.get("allow_challenge", True),
                "pile_penalty": phase.get("pile_penalty", True),
            }
        }

    elif go_type == "betting":
        return {
            "type": "betting",
            "data": {
                "min_bet": phase.get("min_bet", 10),
                "max_raises": phase.get("max_raises", 3),
            }
        }

    else:
        # Unknown phase type - preserve as-is
        return {"type": go_type, "data": phase}


def _convert_condition(cond: Optional[dict]) -> Optional[dict]:
    """Convert Python condition to Go format with op_code."""
    if cond is None:
        return None

    # Handle compound conditions
    if cond.get("type") == "compound":
        return {
            "type": "compound",
            "logic": cond.get("logic", "AND").lower(),
            "conditions": [_convert_condition(c) for c in cond.get("conditions", [])]
        }

    # Handle simple conditions
    if cond.get("type") == "simple":
        # Map Python condition_type to Go op_code
        condition_type = cond.get("condition_type", "HAND_SIZE")
        op_code_map = {
            "HAND_SIZE": "check_hand_size",
            "LOCATION_SIZE": "check_location_size",
            "LOCATION_EMPTY": "check_location_empty",
            "CARD_IS_RANK": "check_card_rank",
            "CARD_MATCHES_SUIT": "check_card_suit",
            "CARD_MATCHES_RANK": "check_rank_match",
            "CARD_MATCHES_COLOR": "check_color_match",
            "SEQUENCE_ADJACENT": "check_sequence",
            "HAS_SET_OF_N": "check_set",
        }

        result = {
            "op_code": op_code_map.get(condition_type, "check_hand_size"),
        }

        # Add operator if present (convert to lowercase)
        if cond.get("operator"):
            result["operator"] = cond["operator"].lower()

        # Add value if present
        if cond.get("value") is not None:
            result["value"] = cond["value"]

        # Add reference location if present
        if cond.get("reference"):
            result["ref_loc"] = _to_lowercase(cond["reference"])

        return result

    return None


def _convert_effect(effect: dict) -> dict:
    """Convert Python effect to Go format (lowercase strings)."""
    # Map Python UPPERCASE rank names to Go lowercase
    rank_map = {
        "ACE": "ace", "TWO": "two", "THREE": "three", "FOUR": "four",
        "FIVE": "five", "SIX": "six", "SEVEN": "seven", "EIGHT": "eight",
        "NINE": "nine", "TEN": "ten", "JACK": "jack", "QUEEN": "queen", "KING": "king"
    }

    # Map Python UPPERCASE effect types to Go lowercase
    effect_map = {
        "SKIP_NEXT": "skip_next",
        "REVERSE_DIRECTION": "reverse_direction",
        "DRAW_CARDS": "draw_cards",
        "EXTRA_TURN": "extra_turn",
        "FORCE_DISCARD": "force_discard",
        "WILD_CARD": "wild_card",
        "BLOCK_NEXT": "block_next",
        "SWAP_HANDS": "swap_hands",
        "STEAL_CARD": "steal_card",
        "PEEK_HAND": "peek_hand",
    }

    # Map Python UPPERCASE targets to Go lowercase
    target_map = {
        "SELF": "self",
        "NEXT_PLAYER": "next_player",
        "PREV_PLAYER": "prev_player",
        "PLAYER_CHOICE": "player_choice",
        "RANDOM_OPPONENT": "random_opponent",
        "ALL_OPPONENTS": "all_opponents",
        "LEFT_OPPONENT": "left_opponent",
        "RIGHT_OPPONENT": "right_opponent",
    }

    trigger_rank = effect.get("trigger_rank", "ACE")
    effect_type = effect.get("effect_type", "SKIP_NEXT")
    target = effect.get("target", "NEXT_PLAYER")

    return {
        "trigger_rank": rank_map.get(trigger_rank, trigger_rank.lower() if isinstance(trigger_rank, str) else "ace"),
        "effect_type": effect_map.get(effect_type, effect_type.lower() if isinstance(effect_type, str) else "skip_next"),
        "target": target_map.get(target, target.lower() if isinstance(target, str) else "next_player"),
        "value": effect.get("value", 1),
    }


def _to_lowercase(value: Any) -> Any:
    """Convert value to lowercase if it's a string."""
    if isinstance(value, str):
        return value.lower()
    return value


def migrate_file(path: Path, dry_run: bool = False) -> tuple[bool, str]:
    """Migrate a single file.

    Args:
        path: Path to the JSON file
        dry_run: If True, don't actually modify files

    Returns:
        Tuple of (was_migrated, status_message)
    """
    try:
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
    except json.JSONDecodeError as e:
        return False, f"invalid_json: {e}"
    except Exception as e:
        return False, f"read_error: {e}"

    fmt = detect_format(data)

    if fmt == "go":
        return False, "already_go_format"

    if fmt == "python_v1":
        if dry_run:
            return True, "would_migrate"

        try:
            # Create backup
            backup_path = path.with_suffix(".json.bak")
            shutil.copy(path, backup_path)

            # Convert and write
            new_data = convert_python_to_go(data)
            with open(path, "w", encoding="utf-8") as f:
                json.dump(new_data, f, indent=2)

            return True, "migrated"
        except Exception as e:
            return False, f"write_error: {e}"

    return False, f"unknown_format"


def main():
    import argparse

    parser = argparse.ArgumentParser(
        description="Migrate Python v1 format genomes to Go canonical format",
        epilog="Creates .bak backup files before modifying. Run with --dry-run first."
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show what would be migrated without making changes"
    )
    parser.add_argument(
        "--pattern",
        default="**/*.json",
        help="Glob pattern for files to process (default: **/*.json)"
    )
    parser.add_argument(
        "--path",
        default=".",
        help="Base path to search (default: current directory)"
    )
    parser.add_argument(
        "--verbose",
        "-v",
        action="store_true",
        help="Show all files, not just migrated ones"
    )
    args = parser.parse_args()

    base = Path(args.path)
    if not base.exists():
        print(f"Error: Path does not exist: {base}")
        sys.exit(1)

    results = {
        "migrated": 0,
        "skipped": 0,
        "already_go_format": 0,
        "failed": 0
    }

    # Find all matching files
    files = list(base.glob(args.pattern))
    if not files:
        print(f"No files found matching pattern '{args.pattern}' in {base}")
        sys.exit(0)

    for path in sorted(files):
        # Skip backup files and non-files
        if ".bak" in path.suffixes or not path.is_file():
            continue

        # Skip checkpoint files and converted files
        if "checkpoint" in path.name or ".converted" in path.name:
            continue

        migrated, status = migrate_file(path, dry_run=args.dry_run)

        if status == "migrated":
            results["migrated"] += 1
            print(f"Migrated: {path}")
        elif status == "would_migrate":
            results["migrated"] += 1
            print(f"Would migrate: {path}")
        elif status == "already_go_format":
            results["already_go_format"] += 1
            if args.verbose:
                print(f"Already Go format: {path}")
        elif status.startswith("invalid_json") or status.startswith("read_error") or status.startswith("write_error"):
            results["failed"] += 1
            print(f"Error ({status}): {path}")
        else:
            results["skipped"] += 1
            if args.verbose:
                print(f"Skipped ({status}): {path}")

    # Print summary
    print()
    print("=" * 50)
    print("Migration Summary")
    print("=" * 50)
    print(f"  Files migrated:      {results['migrated']}")
    print(f"  Already Go format:   {results['already_go_format']}")
    print(f"  Skipped/unknown:     {results['skipped']}")
    print(f"  Failed:              {results['failed']}")
    print()

    if args.dry_run:
        print("(Dry run - no files were modified)")
    else:
        print(f"Backup files created with .bak extension")


if __name__ == "__main__":
    main()
