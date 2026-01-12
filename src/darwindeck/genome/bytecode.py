"""Bytecode compiler for genome serialization."""

from enum import IntEnum
from dataclasses import dataclass
from typing import List, Union
import struct

from darwindeck.genome.schema import (
    GameGenome,
    SetupRules,
    TurnStructure,
    PlayPhase,
    DrawPhase,
    DiscardPhase,
    TrickPhase,
    ClaimPhase,
    BettingPhase,
    WinCondition,
    Location,
    EffectType,
    TargetSelector,
    Rank,
)
from darwindeck.genome.conditions import Condition, ConditionType, Operator, CompoundCondition, ConditionOrCompound


class OpCode(IntEnum):
    """Bytecode instructions for genome execution."""
    # Conditions (0-19)
    CHECK_HAND_SIZE = 0
    CHECK_CARD_RANK = 1
    CHECK_CARD_SUIT = 2
    CHECK_LOCATION_SIZE = 3
    CHECK_SEQUENCE = 4
    # Optional extensions: set/collection detection
    CHECK_HAS_SET_OF_N = 5
    CHECK_HAS_RUN_OF_N = 6
    CHECK_HAS_MATCHING_PAIR = 7
    # Optional extensions: betting conditions
    CHECK_CHIP_COUNT = 8
    CHECK_POT_SIZE = 9
    CHECK_CURRENT_BET = 10
    CHECK_CAN_AFFORD = 11
    # Actions (20-39)
    DRAW_CARDS = 20
    PLAY_CARD = 21
    DISCARD_CARD = 22
    SKIP_TURN = 23
    REVERSE_ORDER = 24
    # Optional extensions: opponent interaction
    DRAW_FROM_OPPONENT = 25
    DISCARD_PAIRS = 26
    # Optional extensions: betting actions
    BET = 27
    CALL = 28
    RAISE = 29
    FOLD = 30
    CHECK = 31
    ALL_IN = 32
    # Optional extensions: bluffing actions
    CLAIM = 33
    CHALLENGE = 34
    REVEAL = 35
    # Control flow (40-49)
    AND = 40
    OR = 41
    # Operators (50-55)
    OP_EQ = 50
    OP_NE = 51
    OP_LT = 52
    OP_GT = 53
    OP_LE = 54
    OP_GE = 55
    # Special Effects (60-69)
    EFFECT_HEADER = 60
    EFFECT_ENTRY = 61


# Rank to bytecode mapping
RANK_TO_BYTE = {
    Rank.TWO: 0, Rank.THREE: 1, Rank.FOUR: 2, Rank.FIVE: 3,
    Rank.SIX: 4, Rank.SEVEN: 5, Rank.EIGHT: 6, Rank.NINE: 7,
    Rank.TEN: 8, Rank.JACK: 9, Rank.QUEEN: 10, Rank.KING: 11, Rank.ACE: 12,
}

# EffectType to bytecode mapping
EFFECT_TYPE_TO_BYTE = {
    EffectType.SKIP_NEXT: 0,
    EffectType.REVERSE_DIRECTION: 1,
    EffectType.DRAW_CARDS: 2,
    EffectType.EXTRA_TURN: 3,
    EffectType.FORCE_DISCARD: 4,
}

# TargetSelector to bytecode mapping
TARGET_TO_BYTE = {
    TargetSelector.NEXT_PLAYER: 0,
    TargetSelector.PREV_PLAYER: 1,
    TargetSelector.PLAYER_CHOICE: 2,
    TargetSelector.RANDOM_OPPONENT: 3,
    TargetSelector.ALL_OPPONENTS: 4,
    TargetSelector.LEFT_OPPONENT: 5,
    TargetSelector.RIGHT_OPPONENT: 6,
}


def compile_effects(effects: list) -> bytes:
    """Compile special effects to bytecode.

    Format:
    - EFFECT_HEADER opcode (1 byte)
    - effect_count (1 byte)
    - For each effect (4 bytes):
      - trigger_rank (1 byte)
      - effect_type (1 byte)
      - target (1 byte)
      - value (1 byte)
    """
    if not effects:
        return bytes()

    result = bytes([OpCode.EFFECT_HEADER.value, len(effects)])
    for effect in effects:
        result += bytes([
            RANK_TO_BYTE[effect.trigger_rank],
            EFFECT_TYPE_TO_BYTE[effect.effect_type],
            TARGET_TO_BYTE[effect.target],
            effect.value,
        ])
    return result


@dataclass
class BytecodeHeader:
    """Fixed-size header for bytecode blob."""
    version: int  # 4 bytes
    genome_id_hash: int  # 8 bytes (hash of genome_id)
    player_count: int  # 4 bytes
    max_turns: int  # 4 bytes
    setup_offset: int  # 4 bytes (offset to setup section)
    turn_structure_offset: int  # 4 bytes
    win_conditions_offset: int  # 4 bytes
    scoring_offset: int  # 4 bytes

    STRUCT_FORMAT = "!IQIIiiii"  # Big-endian, 36 bytes total

    def to_bytes(self) -> bytes:
        return struct.pack(
            self.STRUCT_FORMAT,
            self.version,
            self.genome_id_hash,
            self.player_count,
            self.max_turns,
            self.setup_offset,
            self.turn_structure_offset,
            self.win_conditions_offset,
            self.scoring_offset
        )

    @classmethod
    def from_bytes(cls, data: bytes) -> "BytecodeHeader":
        unpacked = struct.unpack(cls.STRUCT_FORMAT, data[:36])
        return cls(*unpacked)


class BytecodeCompiler:
    """Compiles GameGenome to bytecode."""

    def __init__(self):
        self.offset = 36  # After header

    def compile_genome(self, genome: GameGenome) -> bytes:
        """Convert genome to bytecode blob."""
        # Reset offset for each genome (instance is reused across compilations)
        self.offset = 36  # After header

        # Compile sections
        setup_offset = self.offset
        setup_bytes = self._compile_setup(genome.setup)
        self.offset += len(setup_bytes)

        turn_offset = self.offset
        turn_bytes = self._compile_turn_structure(genome.turn_structure)
        self.offset += len(turn_bytes)

        win_offset = self.offset
        win_bytes = self._compile_win_conditions(genome.win_conditions)
        self.offset += len(win_bytes)

        score_offset = self.offset
        score_bytes = self._compile_scoring(genome.scoring_rules)
        self.offset += len(score_bytes)

        # Create header
        header = BytecodeHeader(
            version=1,
            genome_id_hash=hash(genome.genome_id) & 0xFFFFFFFFFFFFFFFF,
            player_count=genome.player_count,
            max_turns=genome.max_turns,
            setup_offset=setup_offset,
            turn_structure_offset=turn_offset,
            win_conditions_offset=win_offset,
            scoring_offset=score_offset
        )

        # Combine all sections
        return header.to_bytes() + setup_bytes + turn_bytes + win_bytes + score_bytes

    def _compile_setup(self, setup: SetupRules) -> bytes:
        """Encode setup rules.

        Format: cards_per_player:4 + initial_discard_count:4 + starting_chips:4
        """
        return struct.pack("!iii", setup.cards_per_player, setup.initial_discard_count, setup.starting_chips)

    def _compile_turn_structure(self, turn: TurnStructure) -> bytes:
        """Encode turn phases.

        Format: phase_count:4 + [phase_type:1 + phase_data]...
        Go reads phase_type first, then phase_data based on type.
        """
        phase_count = len(turn.phases)
        result = struct.pack("!I", phase_count)  # Use unsigned int

        for phase in turn.phases:
            if isinstance(phase, DrawPhase):
                result += self._compile_draw_phase(phase)
            elif isinstance(phase, PlayPhase):
                result += self._compile_play_phase(phase)
            elif isinstance(phase, DiscardPhase):
                result += self._compile_discard_phase(phase)
            elif isinstance(phase, TrickPhase):
                result += self._compile_trick_phase(phase)
            elif isinstance(phase, ClaimPhase):
                result += self._compile_claim_phase(phase)
            elif isinstance(phase, BettingPhase):
                result += self._compile_betting_phase(phase)
            else:
                # Unknown phase type, skip
                pass

        return result

    def _compile_condition(self, cond: ConditionOrCompound) -> bytes:
        """Encode condition to bytecode."""
        if isinstance(cond, CompoundCondition):
            # Compound condition: logic + count + nested conditions
            logic_op = OpCode.AND if cond.logic == "AND" else OpCode.OR
            count = len(cond.conditions)
            result = struct.pack("!BI", logic_op, count)
            for nested in cond.conditions:
                result += self._compile_condition(nested)
            return result
        else:
            # Simple condition: [OpCode:1][Operator:1][Value:4][Reference:1]
            opcode = self._condition_type_to_opcode(cond.type)
            operator = self._operator_to_code(cond.operator) if cond.operator else 0
            value = cond.value if isinstance(cond.value, int) else 0
            ref = self._reference_to_code(cond.reference) if cond.reference else 0
            return struct.pack("!BBiB", opcode, operator, value, ref)

    def _compile_draw_phase(self, phase: DrawPhase) -> bytes:
        """Encode DrawPhase to bytecode.

        Go format: phase_type:1 + source:1 + count:4 + mandatory:1 + has_condition:1 [+ condition:7]
        Note: Go expects phaseLen=5, but that seems wrong. Using actual format from comment.
        """
        phase_type = 1  # DrawPhase
        source = self._location_to_code(phase.source)
        count = phase.count
        mandatory = 1 if phase.mandatory else 0
        has_condition = 1 if phase.condition else 0

        result = struct.pack("!BBIB", phase_type, source, count, mandatory)
        result += struct.pack("!B", has_condition)

        if phase.condition:
            result += self._compile_condition(phase.condition)

        return result

    def _compile_play_phase(self, phase: PlayPhase) -> bytes:
        """Encode PlayPhase to bytecode.

        Go format: phase_type:1 + target:1 + min:1 + max:1 + mandatory:1 + conditionLen:4 + condition
        """
        phase_type = 2  # PlayPhase
        target = self._location_to_code(phase.target)
        min_cards = phase.min_cards
        max_cards = phase.max_cards
        mandatory = 1 if phase.mandatory else 0

        condition_bytes = self._compile_condition(phase.valid_play_condition)

        # Go reads: target:1 + min:1 + max:1 + mandatory:1 + conditionLen:4 = 8 bytes header
        header = struct.pack("!BBBBBI", phase_type, target, min_cards, max_cards, mandatory, len(condition_bytes))
        return header + condition_bytes

    def _compile_discard_phase(self, phase: DiscardPhase) -> bytes:
        """Encode DiscardPhase to bytecode.

        Go format: phase_type:1 + target:1 + count:4 + mandatory:1
        """
        phase_type = 3  # DiscardPhase
        target = self._location_to_code(phase.target)
        count = phase.count
        mandatory = 1 if phase.mandatory else 0

        return struct.pack("!BBIB", phase_type, target, count, mandatory)

    def _compile_trick_phase(self, phase: TrickPhase) -> bytes:
        """Encode TrickPhase to bytecode.

        Go format: phase_type:1 + lead_suit_required:1 + trump_suit:1 + high_card_wins:1 + breaking_suit:1
        Total: 5 bytes
        """
        phase_type = 4  # TrickPhase
        lead_suit_required = 1 if phase.lead_suit_required else 0
        trump_suit = self._suit_to_code(phase.trump_suit) if phase.trump_suit else 255  # 255 = None
        high_card_wins = 1 if phase.high_card_wins else 0
        breaking_suit = self._suit_to_code(phase.breaking_suit) if phase.breaking_suit else 255  # 255 = None

        return struct.pack("!BBBBB", phase_type, lead_suit_required, trump_suit, high_card_wins, breaking_suit)

    def _compile_claim_phase(self, phase: ClaimPhase) -> bytes:
        """Encode ClaimPhase to bytecode.

        Go format: phase_type:1 + min_cards:1 + max_cards:1 + sequential_rank:1 +
                   allow_challenge:1 + pile_penalty:1 + reserved:5
        Total: 11 bytes (1 type + 10 data)
        """
        phase_type = 6  # ClaimPhase
        min_cards = phase.min_cards
        max_cards = phase.max_cards
        sequential_rank = 1 if phase.sequential_rank else 0
        allow_challenge = 1 if phase.allow_challenge else 0
        pile_penalty = 1 if phase.pile_penalty else 0

        # Pack: type + 5 data bytes + 5 reserved bytes = 11 total
        return struct.pack("!BBBBBBBBBBB", phase_type, min_cards, max_cards,
                          sequential_rank, allow_challenge, pile_penalty,
                          0, 0, 0, 0, 0)  # 5 reserved bytes

    def _compile_betting_phase(self, phase: BettingPhase) -> bytes:
        """Encode BettingPhase to bytecode.

        Go format: phase_type:1 + min_bet:4 + max_raises:4
        Total: 9 bytes (1 type + 8 data)
        """
        phase_type = 5  # BettingPhase (matches PhaseTypeBetting in Go)
        min_bet = phase.min_bet
        max_raises = phase.max_raises

        return struct.pack("!BII", phase_type, min_bet, max_raises)

    def _suit_to_code(self, suit) -> int:
        """Map Suit enum to code."""
        from darwindeck.genome.schema import Suit
        mapping = {
            Suit.HEARTS: 0,
            Suit.DIAMONDS: 1,
            Suit.CLUBS: 2,
            Suit.SPADES: 3,
        }
        return mapping.get(suit, 255)

    def _compile_win_conditions(self, conditions: List[WinCondition]) -> bytes:
        """Encode win conditions."""
        result = struct.pack("!i", len(conditions))

        for cond in conditions:
            win_type = self._win_type_to_code(cond.type)
            threshold = cond.threshold if cond.threshold else 0
            result += struct.pack("!Bi", win_type, threshold)

        return result

    def _compile_scoring(self, rules: List) -> bytes:
        """Encode scoring rules."""
        # For now, just encode count (War has no scoring rules)
        result = struct.pack("!i", len(rules))
        # TODO: Implement when ScoringRule class is added to schema
        return result

    # Helper mappings
    def _condition_type_to_opcode(self, cond_type: ConditionType) -> int:
        """Map ConditionType to OpCode."""
        mapping = {
            ConditionType.HAND_SIZE: OpCode.CHECK_HAND_SIZE,
            ConditionType.CARD_MATCHES_RANK: OpCode.CHECK_CARD_RANK,
            ConditionType.CARD_MATCHES_SUIT: OpCode.CHECK_CARD_SUIT,
            ConditionType.LOCATION_SIZE: OpCode.CHECK_LOCATION_SIZE,
            ConditionType.SEQUENCE_ADJACENT: OpCode.CHECK_SEQUENCE,
        }
        return mapping.get(cond_type, 0)

    def _operator_to_code(self, op: Operator) -> int:
        """Map Operator to code."""
        mapping = {
            Operator.EQ: OpCode.OP_EQ - 50,
            Operator.NE: OpCode.OP_NE - 50,
            Operator.LT: OpCode.OP_LT - 50,
            Operator.GT: OpCode.OP_GT - 50,
            Operator.LE: OpCode.OP_LE - 50,
            Operator.GE: OpCode.OP_GE - 50,
        }
        return mapping.get(op, 0)

    def _location_to_code(self, loc: Location) -> int:
        """Map Location to code."""
        mapping = {
            Location.DECK: 0,
            Location.HAND: 1,
            Location.DISCARD: 2,
            Location.TABLEAU: 3,
        }
        return mapping.get(loc, 0)

    def _reference_to_code(self, ref: str) -> int:
        """Map reference string to code."""
        mapping = {
            "top_discard": 1,
            "last_played": 2,
            "valid_plays": 3,
        }
        return mapping.get(ref, 0)

    def _win_type_to_code(self, win_type: str) -> int:
        """Map win condition type to code."""
        mapping = {
            "empty_hand": 0,
            "high_score": 1,
            "first_to_score": 2,
            "capture_all": 3,
            "low_score": 4,        # Hearts: lowest score wins
            "all_hands_empty": 5,  # Trick-taking: hand ends when all empty
            "best_hand": 6,        # Poker: best poker hand wins
            "most_captured": 7,    # Scopa: most captured cards wins
        }
        return mapping.get(win_type, 0)
