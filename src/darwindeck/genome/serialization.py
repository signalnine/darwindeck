"""JSON serialization for GameGenome."""

import json
from enum import Enum
from typing import Any, Dict, List, Optional
from dataclasses import asdict

from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    PlayPhase, DrawPhase, DiscardPhase, TrickPhase, Location, Suit, Rank
)
from darwindeck.genome.conditions import (
    Condition, CompoundCondition, ConditionType, Operator
)


def _serialize_value(value: Any) -> Any:
    """Serialize a value, converting enums to their name or value."""
    if value is None:
        return None
    if isinstance(value, Enum):
        # For Rank/Suit, use the name (e.g., "ACE", "HEARTS")
        # For other enums, use the name as well
        return value.name
    return value


def genome_to_dict(genome: GameGenome) -> Dict[str, Any]:
    """Convert GameGenome to JSON-serializable dict."""
    return {
        "schema_version": genome.schema_version,
        "genome_id": genome.genome_id,
        "generation": genome.generation,
        "setup": _setup_to_dict(genome.setup),
        "turn_structure": _turn_structure_to_dict(genome.turn_structure),
        "special_effects": list(genome.special_effects),
        "win_conditions": [_win_condition_to_dict(wc) for wc in genome.win_conditions],
        "scoring_rules": list(genome.scoring_rules),
        "max_turns": genome.max_turns,
        "min_turns": genome.min_turns,
        "player_count": genome.player_count,
    }


def genome_to_json(genome: GameGenome, indent: int = 2) -> str:
    """Serialize GameGenome to JSON string."""
    return json.dumps(genome_to_dict(genome), indent=indent)


def genome_from_dict(data: Dict[str, Any]) -> GameGenome:
    """Create GameGenome from dict."""
    return GameGenome(
        schema_version=data["schema_version"],
        genome_id=data["genome_id"],
        generation=data["generation"],
        setup=_setup_from_dict(data["setup"]),
        turn_structure=_turn_structure_from_dict(data["turn_structure"]),
        special_effects=data.get("special_effects", []),
        win_conditions=[_win_condition_from_dict(wc) for wc in data["win_conditions"]],
        scoring_rules=data.get("scoring_rules", []),
        max_turns=data["max_turns"],
        min_turns=data.get("min_turns", 1),
        player_count=data.get("player_count", 2),
    )


def genome_from_json(json_str: str) -> GameGenome:
    """Deserialize GameGenome from JSON string."""
    return genome_from_dict(json.loads(json_str))


def _setup_to_dict(setup: SetupRules) -> Dict[str, Any]:
    """Convert SetupRules to dict."""
    d = {
        "cards_per_player": setup.cards_per_player,
        "initial_deck": setup.initial_deck,
        "initial_discard_count": setup.initial_discard_count,
    }
    if setup.trump_suit is not None:
        d["trump_suit"] = setup.trump_suit.name
    return d


def _setup_from_dict(data: Dict[str, Any]) -> SetupRules:
    """Create SetupRules from dict."""
    trump_suit = None
    if data.get("trump_suit"):
        trump_suit = Suit[data["trump_suit"]]
    return SetupRules(
        cards_per_player=data["cards_per_player"],
        initial_deck=data.get("initial_deck", "standard_52"),
        initial_discard_count=data.get("initial_discard_count", 0),
        trump_suit=trump_suit,
    )


def _turn_structure_to_dict(ts: TurnStructure) -> Dict[str, Any]:
    """Convert TurnStructure to dict."""
    return {
        "phases": [_phase_to_dict(p) for p in ts.phases],
        "is_trick_based": ts.is_trick_based,
        "tricks_per_hand": ts.tricks_per_hand,
    }


def _turn_structure_from_dict(data: Dict[str, Any]) -> TurnStructure:
    """Create TurnStructure from dict."""
    return TurnStructure(
        phases=[_phase_from_dict(p) for p in data["phases"]],
        is_trick_based=data.get("is_trick_based", False),
        tricks_per_hand=data.get("tricks_per_hand"),
    )


def _phase_to_dict(phase) -> Dict[str, Any]:
    """Convert phase to dict."""
    if isinstance(phase, DrawPhase):
        return {
            "type": "DrawPhase",
            "source": phase.source.name,
            "count": phase.count,
            "mandatory": phase.mandatory,
            "condition": _condition_to_dict(phase.condition) if phase.condition else None,
        }
    elif isinstance(phase, PlayPhase):
        return {
            "type": "PlayPhase",
            "target": phase.target.name,
            "valid_play_condition": _condition_to_dict(phase.valid_play_condition),
            "min_cards": phase.min_cards,
            "max_cards": phase.max_cards,
            "mandatory": phase.mandatory,
        }
    elif isinstance(phase, DiscardPhase):
        return {
            "type": "DiscardPhase",
            "target": phase.target.name,
            "count": phase.count,
            "mandatory": phase.mandatory,
        }
    elif isinstance(phase, TrickPhase):
        return {
            "type": "TrickPhase",
            "lead_suit_required": phase.lead_suit_required,
            "trump_suit": phase.trump_suit.name if phase.trump_suit else None,
            "high_card_wins": phase.high_card_wins,
            "breaking_suit": phase.breaking_suit.name if phase.breaking_suit else None,
        }
    else:
        return {"type": "Unknown"}


def _phase_from_dict(data: Dict[str, Any]):
    """Create phase from dict."""
    phase_type = data["type"]

    if phase_type == "DrawPhase":
        condition = _condition_from_dict(data["condition"]) if data.get("condition") else None
        return DrawPhase(
            source=Location[data["source"]],
            count=data["count"],
            mandatory=data["mandatory"],
            condition=condition,
        )
    elif phase_type == "PlayPhase":
        return PlayPhase(
            target=Location[data["target"]],
            valid_play_condition=_condition_from_dict(data["valid_play_condition"]),
            min_cards=data["min_cards"],
            max_cards=data["max_cards"],
            mandatory=data["mandatory"],
        )
    elif phase_type == "DiscardPhase":
        return DiscardPhase(
            target=Location[data["target"]],
            count=data["count"],
            mandatory=data["mandatory"],
        )
    elif phase_type == "TrickPhase":
        trump = Suit[data["trump_suit"]] if data.get("trump_suit") else None
        breaking = Suit[data["breaking_suit"]] if data.get("breaking_suit") else None
        return TrickPhase(
            lead_suit_required=data["lead_suit_required"],
            trump_suit=trump,
            high_card_wins=data["high_card_wins"],
            breaking_suit=breaking,
        )
    else:
        raise ValueError(f"Unknown phase type: {phase_type}")


def _condition_to_dict(cond) -> Optional[Dict[str, Any]]:
    """Convert condition to dict."""
    if cond is None:
        return None

    if isinstance(cond, CompoundCondition):
        return {
            "type": "compound",
            "logic": cond.logic,
            "conditions": [_condition_to_dict(c) for c in cond.conditions],
        }
    elif isinstance(cond, Condition):
        return {
            "type": "simple",
            "condition_type": cond.type.name,
            "operator": cond.operator.name if cond.operator else None,
            "value": _serialize_value(cond.value),
            "reference": _serialize_value(cond.reference),
        }
    return None


def _condition_from_dict(data: Optional[Dict[str, Any]]):
    """Create condition from dict."""
    if data is None:
        return None

    if data["type"] == "compound":
        return CompoundCondition(
            logic=data["logic"],
            conditions=[_condition_from_dict(c) for c in data["conditions"]],
        )
    elif data["type"] == "simple":
        return Condition(
            type=ConditionType[data["condition_type"]],
            operator=Operator[data["operator"]] if data.get("operator") else None,
            value=data.get("value"),
            reference=data.get("reference"),
        )
    return None


def _win_condition_to_dict(wc: WinCondition) -> Dict[str, Any]:
    """Convert WinCondition to dict."""
    return {
        "type": wc.type,
        "threshold": wc.threshold,
    }


def _win_condition_from_dict(data: Dict[str, Any]) -> WinCondition:
    """Create WinCondition from dict."""
    return WinCondition(
        type=data["type"],
        threshold=data.get("threshold"),
    )
