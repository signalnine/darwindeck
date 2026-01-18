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
    BiddingPhase,
    ContractScoring,
    WinCondition,
    Location,
    EffectType,
    TargetSelector,
    Rank,
    TableauMode,
    SequenceDirection,
    ScoringTrigger,
    CardCondition,
    CardScoringRule,
    Suit,
    HandEvaluationMethod,
    CardValue,
    HandEvaluation,
    HandPattern,
)
from darwindeck.genome.conditions import Condition, ConditionType, Operator, CompoundCondition, ConditionOrCompound

# Bytecode format version
BYTECODE_VERSION = 2

# TableauMode encoding
TABLEAU_MODE_MAP = {
    TableauMode.NONE: 0,
    TableauMode.WAR: 1,
    TableauMode.MATCH_RANK: 2,
    TableauMode.SEQUENCE: 3,
}

# SequenceDirection encoding
SEQUENCE_DIRECTION_MAP = {
    SequenceDirection.ASCENDING: 0,
    SequenceDirection.DESCENDING: 1,
    SequenceDirection.BOTH: 2,
}


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
    # Card matching conditions (for valid_play_condition)
    CHECK_CARD_MATCHES_RANK = 12  # Candidate card matches reference card's rank
    CHECK_CARD_MATCHES_SUIT = 13  # Candidate card matches reference card's suit
    CHECK_CARD_BEATS_TOP = 14     # Candidate card beats reference card (President)
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
    # Phase-specific opcodes (70-79)
    BIDDING_PHASE = 70


# Opcode constant for external use
OPCODE_BIDDING_PHASE = 70


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

# ScoringTrigger encoding
SCORING_TRIGGER_MAP = {
    ScoringTrigger.TRICK_WIN: 0,
    ScoringTrigger.CAPTURE: 1,
    ScoringTrigger.PLAY: 2,
    ScoringTrigger.HAND_END: 3,
    ScoringTrigger.SET_COMPLETE: 4,
}

# Suit encoding (for CardCondition)
SUIT_TO_BYTE = {
    Suit.HEARTS: 0,
    Suit.DIAMONDS: 1,
    Suit.CLUBS: 2,
    Suit.SPADES: 3,
}

# HandEvaluationMethod encoding
HAND_EVAL_METHOD_MAP = {
    HandEvaluationMethod.NONE: 0,
    HandEvaluationMethod.HIGH_CARD: 1,
    HandEvaluationMethod.POINT_TOTAL: 2,
    HandEvaluationMethod.PATTERN_MATCH: 3,
    HandEvaluationMethod.CARD_COUNT: 4,
}


def compile_card_scoring(rules: tuple) -> bytes:
    """Compile card scoring rules to bytecode.

    Format:
    - rule_count (2 bytes, big-endian)
    - For each rule (5 bytes):
      - suit (1 byte): 0-3 for H/D/C/S, 255 for "any"
      - rank (1 byte): 0-12 for 2-A, 255 for "any"
      - points (2 bytes, big-endian, signed)
      - trigger (1 byte): ScoringTrigger enum value
    """
    if not rules:
        return struct.pack("!H", 0)  # 0 rules

    result = struct.pack("!H", len(rules))
    for rule in rules:
        # Encode condition
        suit = SUIT_TO_BYTE.get(rule.condition.suit, 255) if rule.condition.suit else 255
        rank = RANK_TO_BYTE.get(rule.condition.rank, 255) if rule.condition.rank else 255

        # Encode points (signed 16-bit)
        points = rule.points

        # Encode trigger
        trigger = SCORING_TRIGGER_MAP.get(rule.trigger, 0)

        result += struct.pack("!BBhB", suit, rank, points, trigger)

    return result


def compile_card_values(values: tuple) -> bytes:
    """Compile card values to bytecode.

    Format:
    - value_count (1 byte)
    - For each value (3 bytes):
      - rank (1 byte): 0-12 for 2-A
      - value (1 byte): primary point value
      - alternate_value (1 byte): 0 if none, else alternate value
    """
    if not values:
        return bytes([0])

    result = bytes([len(values)])
    for cv in values:
        rank = RANK_TO_BYTE.get(cv.rank, 0)
        value = cv.value & 0xFF
        alt = cv.alternate_value if cv.alternate_value else 0
        result += bytes([rank, value, alt])

    return result


def compile_hand_patterns(patterns: tuple) -> bytes:
    """Compile hand patterns to bytecode.

    Format:
    - pattern_count (1 byte)
    - For each pattern:
      - rank_priority (1 byte)
      - required_count (1 byte, 0 if None)
      - same_suit_count (1 byte, 0 if None)
      - sequence_length (1 byte, 0 if None)
      - sequence_wrap (1 byte, 0 or 1)
      - group_count (1 byte)
      - same_rank_groups (group_count bytes)
      - required_ranks_count (1 byte)
      - required_ranks (required_ranks_count bytes)
    """
    if not patterns:
        return bytes([0])

    result = bytes([len(patterns)])
    for pattern in patterns:
        result += bytes([
            # Clamp rank_priority to 0-255 for single-byte encoding
            pattern.rank_priority & 0xFF,
            pattern.required_count or 0,
            pattern.same_suit_count or 0,
            pattern.sequence_length or 0,
            1 if pattern.sequence_wrap else 0,
        ])

        # Encode same_rank_groups (clamp each value to 0-255)
        groups = pattern.same_rank_groups or ()
        result += bytes([len(groups)])
        result += bytes([g & 0xFF for g in groups])

        # Encode required_ranks
        ranks = pattern.required_ranks or ()
        result += bytes([len(ranks)])
        for r in ranks:
            result += bytes([RANK_TO_BYTE.get(r, 0)])

    return result


def compile_hand_evaluation(eval: HandEvaluation) -> bytes:
    """Compile hand evaluation to bytecode.

    Format:
    - method (1 byte): HandEvaluationMethod enum
    - target_value (1 byte, 0 if None)
    - bust_threshold (1 byte, 0 if None)
    - card_values section (variable)
    - patterns section (variable)
    """
    if eval is None:
        return bytes([0])  # NONE method

    method = HAND_EVAL_METHOD_MAP.get(eval.method, 0)
    target = eval.target_value or 0
    bust = eval.bust_threshold or 0

    result = bytes([method, target & 0xFF, bust & 0xFF])
    result += compile_card_values(eval.card_values or ())
    result += compile_hand_patterns(eval.patterns or ())

    return result


def compile_teams(teams: tuple[tuple[int, ...], ...]) -> bytes:
    """Compile team assignments to bytecode.

    Format:
    [num_teams: 1 byte]
    For each team:
        [team_size: 1 byte]
        [player_indices: team_size bytes]

    Example for 2v2 teams ((0, 2), (1, 3)):
    [2][2][0][2][2][1][3] = 7 bytes
    """
    if not teams:
        return bytes([0])  # No teams

    result = [len(teams)]  # num_teams
    for team in teams:
        result.append(len(team))  # team_size
        result.extend(team)  # player indices
    return bytes(result)


def compile_bidding_phase(phase: BiddingPhase, scoring: ContractScoring = None) -> bytes:
    """Compile BiddingPhase to bytecode.

    Format: [opcode=70] [min_bid] [max_bid] [flags] [scoring_data...]
    flags byte: bit 0 = allow_nil
    scoring_data: 12 bytes of ContractScoring

    Total: 16 bytes (4 header + 12 scoring)
    """
    if scoring is None:
        scoring = ContractScoring()

    flags = 0
    if phase.allow_nil:
        flags |= 0x01

    result = bytes([
        OPCODE_BIDDING_PHASE,
        phase.min_bid,
        phase.max_bid,
        flags,
    ])

    # Append ContractScoring (12 bytes)
    result += bytes([
        scoring.points_per_trick_bid,
        scoring.overtrick_points,
        scoring.failed_contract_penalty,
    ])
    # nil_bonus as uint16 little-endian
    result += scoring.nil_bonus.to_bytes(2, 'little')
    # nil_penalty as uint16 little-endian
    result += scoring.nil_penalty.to_bytes(2, 'little')
    result += bytes([scoring.bag_limit])
    # bag_penalty as uint16 little-endian
    result += scoring.bag_penalty.to_bytes(2, 'little')
    # reserved 2 bytes
    result += bytes([0, 0])

    return result


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
    """Fixed-size header for bytecode blob.

    Format (53 bytes total):
    - Byte 0: bytecode version (single byte, currently 2)
    - Bytes 1-4: legacy version field (4 bytes, for compatibility)
    - Bytes 5-12: genome_id_hash (8 bytes)
    - Bytes 13-16: player_count (4 bytes)
    - Bytes 17-20: max_turns (4 bytes)
    - Bytes 21-24: setup_offset (4 bytes)
    - Bytes 25-28: turn_structure_offset (4 bytes)
    - Bytes 29-32: win_conditions_offset (4 bytes)
    - Bytes 33-36: scoring_offset (4 bytes)
    - Byte 37: tableau_mode (1 byte)
    - Byte 38: sequence_direction (1 byte)
    - Bytes 39-42: card_scoring_offset (4 bytes)
    - Bytes 43-46: hand_evaluation_offset (4 bytes)
    - Byte 47: team_mode (1 byte, 0 or 1)
    - Byte 48: team_count (1 byte)
    - Bytes 49-52: team_data_offset (4 bytes)
    """
    version: int  # 4 bytes (legacy, kept for compatibility)
    genome_id_hash: int  # 8 bytes (hash of genome_id)
    player_count: int  # 4 bytes
    max_turns: int  # 4 bytes
    setup_offset: int  # 4 bytes (offset to setup section)
    turn_structure_offset: int  # 4 bytes
    win_conditions_offset: int  # 4 bytes
    scoring_offset: int  # 4 bytes
    tableau_mode: int = 0  # 1 byte (TableauMode enum value)
    sequence_direction: int = 0  # 1 byte (SequenceDirection enum value)
    card_scoring_offset: int = 0  # 4 bytes (offset to card scoring section)
    hand_evaluation_offset: int = 0  # 4 bytes (offset to hand evaluation section)
    team_mode: bool = False  # 1 byte (0 or 1)
    team_count: int = 0  # 1 byte
    team_data_offset: int = 0  # 4 bytes (offset to team data in bytecode)

    # Inner struct format (bytes 1-36): legacy version + core fields
    INNER_STRUCT_FORMAT = "!IQIIiiii"  # 36 bytes

    HEADER_SIZE = 53  # Total header size including version byte, tableau fields, and team fields

    def to_bytes(self) -> bytes:
        # Byte 0: bytecode version
        result = bytes([BYTECODE_VERSION])
        # Bytes 1-36: legacy struct fields
        result += struct.pack(
            self.INNER_STRUCT_FORMAT,
            self.version,
            self.genome_id_hash,
            self.player_count,
            self.max_turns,
            self.setup_offset,
            self.turn_structure_offset,
            self.win_conditions_offset,
            self.scoring_offset
        )
        # Bytes 37-38: tableau_mode and sequence_direction
        result += bytes([self.tableau_mode, self.sequence_direction])
        # Bytes 39-46: card_scoring_offset and hand_evaluation_offset
        result += struct.pack("!ii", self.card_scoring_offset, self.hand_evaluation_offset)
        # Bytes 47-52: team_mode, team_count, team_data_offset
        result += bytes([1 if self.team_mode else 0, self.team_count])
        result += struct.pack("!i", self.team_data_offset)
        return result

    @classmethod
    def from_bytes(cls, data: bytes) -> "BytecodeHeader":
        # Skip byte 0 (bytecode version), parse bytes 1-36
        unpacked = struct.unpack(cls.INNER_STRUCT_FORMAT, data[1:37])
        # Parse bytes 37-38
        tableau_mode = data[37] if len(data) > 37 else 0
        sequence_direction = data[38] if len(data) > 38 else 0
        # Parse bytes 39-46: card_scoring_offset and hand_evaluation_offset
        card_scoring_offset = struct.unpack("!i", data[39:43])[0] if len(data) > 42 else 0
        hand_evaluation_offset = struct.unpack("!i", data[43:47])[0] if len(data) > 46 else 0
        # Parse bytes 47-52: team_mode, team_count, team_data_offset
        team_mode = bool(data[47]) if len(data) > 47 else False
        team_count = data[48] if len(data) > 48 else 0
        team_data_offset = struct.unpack("!i", data[49:53])[0] if len(data) > 52 else 0
        return cls(
            *unpacked,
            tableau_mode=tableau_mode,
            sequence_direction=sequence_direction,
            card_scoring_offset=card_scoring_offset,
            hand_evaluation_offset=hand_evaluation_offset,
            team_mode=team_mode,
            team_count=team_count,
            team_data_offset=team_data_offset,
        )


class BytecodeCompiler:
    """Compiles GameGenome to bytecode."""

    def __init__(self):
        self.offset = BytecodeHeader.HEADER_SIZE  # After header (39 bytes)

    def compile_genome(self, genome: GameGenome) -> bytes:
        """Convert genome to bytecode blob."""
        # Reset offset for each genome (instance is reused across compilations)
        self.offset = BytecodeHeader.HEADER_SIZE  # After header (53 bytes)

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

        # Compile effects (Go expects them right after win conditions)
        effects_bytes = compile_effects(genome.special_effects)
        self.offset += len(effects_bytes)

        score_offset = self.offset
        score_bytes = self._compile_scoring(genome.scoring_rules)
        self.offset += len(score_bytes)

        # Compile card_scoring section
        card_scoring_offset = self.offset
        card_scoring_bytes = compile_card_scoring(genome.card_scoring or ())
        self.offset += len(card_scoring_bytes)

        # Compile hand_evaluation section
        hand_eval_offset = self.offset
        hand_eval_bytes = compile_hand_evaluation(genome.hand_evaluation)
        self.offset += len(hand_eval_bytes)

        # Compile team data section
        team_data_offset = self.offset
        team_bytes = compile_teams(genome.teams)
        self.offset += len(team_bytes)

        # Encode tableau_mode and sequence_direction
        tableau_mode = TABLEAU_MODE_MAP.get(genome.setup.tableau_mode, 0)
        sequence_direction = SEQUENCE_DIRECTION_MAP.get(
            genome.setup.sequence_direction, 0
        ) if genome.setup.sequence_direction else 0

        # Create header with all offsets including new sections
        header = BytecodeHeader(
            version=1,
            genome_id_hash=hash(genome.genome_id) & 0xFFFFFFFFFFFFFFFF,
            player_count=genome.player_count,
            max_turns=genome.max_turns,
            setup_offset=setup_offset,
            turn_structure_offset=turn_offset,
            win_conditions_offset=win_offset,
            scoring_offset=score_offset,
            tableau_mode=tableau_mode,
            sequence_direction=sequence_direction,
            card_scoring_offset=card_scoring_offset,
            hand_evaluation_offset=hand_eval_offset,
            team_mode=genome.team_mode,
            team_count=len(genome.teams),
            team_data_offset=team_data_offset,
        )

        # Combine all sections (effects come right after win conditions, before scoring)
        return (header.to_bytes() + setup_bytes + turn_bytes + win_bytes +
                effects_bytes + score_bytes + card_scoring_bytes + hand_eval_bytes +
                team_bytes)

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
            elif isinstance(phase, BiddingPhase):
                result += self._compile_bidding_phase_internal(phase)
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
            value = self._value_to_int(cond.value)
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

        Go format: phase_type:1 + target:1 + min:1 + max:1 + mandatory:1 + pass_if_unable:1 + conditionLen:4 + condition
        """
        phase_type = 2  # PlayPhase
        target = self._location_to_code(phase.target)
        min_cards = phase.min_cards
        max_cards = phase.max_cards
        mandatory = 1 if phase.mandatory else 0
        pass_if_unable = 1 if phase.pass_if_unable else 0

        # Compile condition if present, otherwise empty
        condition_bytes = b""
        if phase.valid_play_condition is not None:
            condition_bytes = self._compile_condition(phase.valid_play_condition)

        # Go reads: target:1 + min:1 + max:1 + mandatory:1 + pass_if_unable:1 + conditionLen:4 = 9 bytes header
        header = struct.pack("!BBBBBBI", phase_type, target, min_cards, max_cards, mandatory, pass_if_unable, len(condition_bytes))
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

    def _compile_bidding_phase_internal(self, phase: BiddingPhase) -> bytes:
        """Encode BiddingPhase to bytecode using module-level function.

        Go format: phase_type:1 + bidding_data:16
        Total: 17 bytes (1 type + 16 bidding phase data)
        """
        phase_type = 7  # BiddingPhase (new phase type)
        # Use the module-level compile_bidding_phase function
        bidding_data = compile_bidding_phase(phase)
        return bytes([phase_type]) + bidding_data

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
            ConditionType.CARD_IS_RANK: OpCode.CHECK_CARD_RANK,  # Card is specific rank (wild cards)
            ConditionType.CARD_MATCHES_RANK: OpCode.CHECK_CARD_MATCHES_RANK,  # Card matches reference's rank
            ConditionType.CARD_MATCHES_SUIT: OpCode.CHECK_CARD_MATCHES_SUIT,  # Card matches reference's suit
            ConditionType.CARD_BEATS_TOP: OpCode.CHECK_CARD_BEATS_TOP,  # Card beats reference (President)
            ConditionType.LOCATION_SIZE: OpCode.CHECK_LOCATION_SIZE,
            ConditionType.SEQUENCE_ADJACENT: OpCode.CHECK_SEQUENCE,
            ConditionType.HAS_SET_OF_N: OpCode.CHECK_HAS_SET_OF_N,
            ConditionType.HAS_RUN_OF_N: OpCode.CHECK_HAS_RUN_OF_N,
            ConditionType.HAS_MATCHING_PAIR: OpCode.CHECK_HAS_MATCHING_PAIR,
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
            Location.OPPONENT_HAND: 4,
            Location.OPPONENT_DISCARD: 5,
        }
        return mapping.get(loc, 0)

    def _value_to_int(self, value) -> int:
        """Convert condition value to integer.

        Handles:
        - int: pass through
        - Rank enum: convert to 0-12 (Ace=0, King=12)
        - Suit enum: convert to 0-3 (Hearts=0, Spades=3)
        - None/other: return 0
        """
        from darwindeck.genome.schema import Rank, Suit

        if isinstance(value, int):
            return value

        if isinstance(value, Rank):
            rank_order = [Rank.ACE, Rank.TWO, Rank.THREE, Rank.FOUR, Rank.FIVE,
                          Rank.SIX, Rank.SEVEN, Rank.EIGHT, Rank.NINE, Rank.TEN,
                          Rank.JACK, Rank.QUEEN, Rank.KING]
            try:
                return rank_order.index(value)
            except ValueError:
                return 0

        if isinstance(value, Suit):
            suit_order = [Suit.HEARTS, Suit.DIAMONDS, Suit.CLUBS, Suit.SPADES]
            try:
                return suit_order.index(value)
            except ValueError:
                return 0

        return 0

    def _reference_to_code(self, ref: str) -> int:
        """Map reference string to code.

        Two contexts use references differently:
        - Card comparisons (CARD_BEATS_TOP, etc.): Reference identifies which card to compare
          - 1 = top_discard (top of discard pile)
          - 2 = last_played / tableau_top (top of tableau)
        - Location checks (LOCATION_SIZE): Reference identifies which location to check
          - Uses Location codes: 0=deck, 1=hand, 2=discard, 3=tableau
        """
        mapping = {
            # Card references (for CARD_BEATS_TOP, CARD_MATCHES_RANK, etc.)
            "top_discard": 1,
            "last_played": 2,
            "tableau_top": 2,  # Alias for last_played
            "valid_plays": 3,
            # Location references (for LOCATION_SIZE)
            "deck": 0,
            "hand": 1,
            "discard": 2,
            "tableau": 3,
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
