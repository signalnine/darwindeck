"""Tests for genome bytecode compiler."""

import struct
import pytest
from darwindeck.genome.bytecode import BytecodeCompiler, BytecodeHeader, OpCode
from darwindeck.genome.examples import create_war_genome
from darwindeck.genome.schema import (
    BettingPhase,
    SetupRules,
    GameGenome,
    TurnStructure,
    DrawPhase,
    Location,
    WinCondition,
)


def test_header_serialization() -> None:
    """Test header round-trip."""
    header = BytecodeHeader(
        version=1,
        genome_id_hash=12345678901234567890,
        player_count=2,
        max_turns=100,
        setup_offset=53,  # Updated for new header size (53 bytes with team fields)
        turn_structure_offset=65,
        win_conditions_offset=121,
        scoring_offset=141,
        tableau_mode=1,  # WAR
        sequence_direction=0,  # ASCENDING
        card_scoring_offset=156,
        hand_evaluation_offset=176,
        team_mode=True,
        team_count=2,
        team_data_offset=190,
    )

    serialized = header.to_bytes()
    assert len(serialized) == 53  # Header size with team fields

    # First byte should be bytecode version 2
    assert serialized[0] == 2

    deserialized = BytecodeHeader.from_bytes(serialized)
    assert deserialized.version == 1
    assert deserialized.player_count == 2
    assert deserialized.max_turns == 100
    assert deserialized.tableau_mode == 1
    assert deserialized.sequence_direction == 0
    assert deserialized.card_scoring_offset == 156
    assert deserialized.hand_evaluation_offset == 176
    assert deserialized.team_mode is True
    assert deserialized.team_count == 2
    assert deserialized.team_data_offset == 190


def test_compile_war_genome() -> None:
    """Test compiling War genome to bytecode."""
    war = create_war_genome()
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(war)

    # Should be compact (< 500 bytes for War)
    assert len(bytecode) < 500

    # Header should parse
    header = BytecodeHeader.from_bytes(bytecode)
    assert header.version == 1
    assert header.player_count == 2
    assert header.max_turns == 1000


def test_opcode_values() -> None:
    """Test OpCode enum values are in expected ranges."""
    # Conditions: 0-19
    assert 0 <= OpCode.CHECK_HAND_SIZE <= 19
    assert 0 <= OpCode.CHECK_CARD_RANK <= 19

    # Actions: 20-39
    assert 20 <= OpCode.DRAW_CARDS <= 39
    assert 20 <= OpCode.PLAY_CARD <= 39

    # Control flow: 40-49
    assert 40 <= OpCode.AND <= 49
    assert 40 <= OpCode.OR <= 49

    # Operators: 50-55
    assert 50 <= OpCode.OP_EQ <= 55
    assert 50 <= OpCode.OP_NE <= 55


def test_effect_opcodes_exist():
    """Effect-related opcodes are defined."""
    from darwindeck.genome.bytecode import OpCode

    assert OpCode.EFFECT_HEADER.value == 60
    assert OpCode.EFFECT_ENTRY.value == 61


def test_compile_effects():
    """compile_effects produces correct bytecode."""
    from darwindeck.genome.bytecode import compile_effects
    from darwindeck.genome.schema import SpecialEffect, EffectType, Rank, TargetSelector

    effects = [
        SpecialEffect(Rank.TWO, EffectType.DRAW_CARDS, TargetSelector.NEXT_PLAYER, 2),
        SpecialEffect(Rank.JACK, EffectType.SKIP_NEXT, TargetSelector.NEXT_PLAYER, 1),
    ]

    bytecode = compile_effects(effects)

    # Header: opcode (60), count (2)
    assert bytecode[0] == 60  # EFFECT_HEADER
    assert bytecode[1] == 2   # count

    # Effect 1: TWO (0), DRAW_CARDS (2), NEXT_PLAYER (0), value (2)
    assert bytecode[2] == 0   # Rank.TWO -> 0
    assert bytecode[3] == 2   # EffectType.DRAW_CARDS -> 2
    assert bytecode[4] == 0   # TARGET_NEXT_PLAYER -> 0
    assert bytecode[5] == 2   # value

    # Effect 2: JACK (9), SKIP_NEXT (0), NEXT_PLAYER (0), value (1)
    assert bytecode[6] == 9   # Rank.JACK -> 9
    assert bytecode[7] == 0   # EffectType.SKIP_NEXT -> 0
    assert bytecode[8] == 0   # TARGET_NEXT_PLAYER -> 0
    assert bytecode[9] == 1   # value


def test_compile_betting_phase() -> None:
    """Test BettingPhase compilation produces correct bytes."""
    compiler = BytecodeCompiler()
    phase = BettingPhase(min_bet=25, max_raises=4)

    bytecode = compiler._compile_betting_phase(phase)

    # Should be 9 bytes: phase_type:1 + min_bet:4 + max_raises:4
    assert len(bytecode) == 9

    # Parse the bytecode
    phase_type = bytecode[0]
    min_bet = struct.unpack("!I", bytecode[1:5])[0]
    max_raises = struct.unpack("!I", bytecode[5:9])[0]

    assert phase_type == 5  # PhaseTypeBetting
    assert min_bet == 25
    assert max_raises == 4


def test_compile_setup_includes_starting_chips() -> None:
    """Test that starting_chips is included in setup bytes."""
    compiler = BytecodeCompiler()
    setup = SetupRules(
        cards_per_player=5,
        initial_discard_count=1,
        starting_chips=1000,
    )

    bytecode = compiler._compile_setup(setup)

    # Should be 12 bytes: cards_per_player:4 + initial_discard_count:4 + starting_chips:4
    assert len(bytecode) == 12

    # Parse the bytecode
    cards_per_player, initial_discard_count, starting_chips = struct.unpack("!iii", bytecode)

    assert cards_per_player == 5
    assert initial_discard_count == 1
    assert starting_chips == 1000


def test_compile_genome_with_betting_phase() -> None:
    """Test genome with BettingPhase compiles correctly."""
    from darwindeck.genome.conditions import Condition, ConditionType, Operator

    # Create a simple genome with a betting phase
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test-betting-genome",
        generation=0,
        player_count=2,
        max_turns=100,
        setup=SetupRules(
            cards_per_player=5,
            initial_discard_count=0,
            starting_chips=500,
        ),
        turn_structure=TurnStructure(
            phases=[
                DrawPhase(source=Location.DECK, count=1),
                BettingPhase(min_bet=10, max_raises=3),
            ]
        ),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    # Should compile without error
    assert len(bytecode) > 47  # Header + data

    # Header should parse
    header = BytecodeHeader.from_bytes(bytecode)
    assert header.version == 1
    assert header.player_count == 2
    assert header.max_turns == 100

    # Setup should contain starting_chips
    setup_start = header.setup_offset
    setup_bytes = bytecode[setup_start:setup_start + 12]
    _, _, starting_chips = struct.unpack("!iii", setup_bytes)
    assert starting_chips == 500


def test_bytecode_version_header():
    """Bytecode starts with version byte."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition, TableauMode
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(cards_per_player=7, tableau_mode=TableauMode.WAR),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    # First byte should be version 2
    assert bytecode[0] == 2


def test_bytecode_tableau_mode_encoding():
    """Bytecode encodes tableau_mode at correct offset."""
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition,
        TableauMode, SequenceDirection
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test",
        generation=1,
        setup=SetupRules(
            cards_per_player=7,
            tableau_mode=TableauMode.SEQUENCE,
            sequence_direction=SequenceDirection.DESCENDING
        ),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
    )

    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    # Offset 37: tableau_mode (3=SEQUENCE)
    # Offset 38: sequence_direction (1=DESCENDING)
    assert bytecode[37] == 3  # SEQUENCE
    assert bytecode[38] == 1  # DESCENDING


def test_compile_card_scoring_hearts():
    """Test encoding Hearts-style card scoring rules."""
    from darwindeck.genome.schema import CardScoringRule, CardCondition, ScoringTrigger, Suit, Rank
    from darwindeck.genome.bytecode import compile_card_scoring

    rules = (
        CardScoringRule(
            condition=CardCondition(suit=Suit.HEARTS),
            points=1,
            trigger=ScoringTrigger.TRICK_WIN,
        ),
        CardScoringRule(
            condition=CardCondition(suit=Suit.SPADES, rank=Rank.QUEEN),
            points=13,
            trigger=ScoringTrigger.TRICK_WIN,
        ),
    )

    bytecode = compile_card_scoring(rules)

    # Format: count:2 + [suit:1, rank:1, points:2, trigger:1] * count
    assert len(bytecode) == 2 + (5 * 2)  # 12 bytes
    assert bytecode[0:2] == b'\x00\x02'  # 2 rules (big-endian)


def test_compile_card_values_blackjack():
    """Test encoding Blackjack card values."""
    from darwindeck.genome.schema import CardValue, Rank
    from darwindeck.genome.bytecode import compile_card_values

    values = (
        CardValue(rank=Rank.ACE, value=1, alternate_value=11),
        CardValue(rank=Rank.KING, value=10),
    )

    bytecode = compile_card_values(values)

    # Format: count:1 + [rank:1, value:1, alt_value:1] * count
    assert len(bytecode) == 1 + (3 * 2)  # 7 bytes
    assert bytecode[0] == 2  # 2 values


def test_compile_hand_patterns_poker():
    """Test encoding poker hand patterns."""
    from darwindeck.genome.schema import HandPattern
    from darwindeck.genome.bytecode import compile_hand_patterns

    patterns = (
        HandPattern(
            name="Full House",
            rank_priority=70,
            required_count=5,
            same_rank_groups=(3, 2),
        ),
        HandPattern(
            name="Flush",
            rank_priority=60,
            required_count=5,
            same_suit_count=5,
        ),
    )

    bytecode = compile_hand_patterns(patterns)

    # Format: count:1 + variable per pattern
    assert bytecode[0] == 2  # 2 patterns


def test_compile_hand_evaluation_blackjack():
    """Test encoding Blackjack hand evaluation."""
    from darwindeck.genome.schema import HandEvaluation, HandEvaluationMethod, CardValue, Rank
    from darwindeck.genome.bytecode import compile_hand_evaluation

    eval = HandEvaluation(
        method=HandEvaluationMethod.POINT_TOTAL,
        card_values=(
            CardValue(rank=Rank.ACE, value=1, alternate_value=11),
        ),
        target_value=21,
        bust_threshold=22,
    )

    bytecode = compile_hand_evaluation(eval)

    # Format: method:1 + target:1 + bust:1 + card_values + patterns
    assert bytecode[0] == 2  # POINT_TOTAL method
    assert bytecode[1] == 21  # target_value
    assert bytecode[2] == 22  # bust_threshold


def test_compile_genome_with_card_scoring():
    """Test that Hearts genome includes card_scoring in bytecode."""
    from darwindeck.genome.examples import create_hearts_genome
    from darwindeck.genome.bytecode import BytecodeCompiler, BytecodeHeader

    genome = create_hearts_genome()
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    header = BytecodeHeader.from_bytes(bytecode)

    # Verify card_scoring_offset is set and points to valid data
    assert header.card_scoring_offset > 0
    assert header.card_scoring_offset < len(bytecode)


def test_compile_genome_with_hand_evaluation():
    """Test that Blackjack genome includes hand_evaluation in bytecode."""
    from darwindeck.genome.examples import create_blackjack_genome
    from darwindeck.genome.bytecode import BytecodeCompiler, BytecodeHeader

    genome = create_blackjack_genome()
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    header = BytecodeHeader.from_bytes(bytecode)

    # Verify hand_evaluation_offset is set and points to valid data
    assert header.hand_evaluation_offset > 0
    assert header.hand_evaluation_offset < len(bytecode)


def test_bytecode_header_includes_team_mode():
    """Bytecode header should include team_mode flag."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="team-game",
        generation=0,
        player_count=4,
        max_turns=100,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        team_mode=True,
        teams=((0, 2), (1, 3)),
    )
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)
    header = BytecodeHeader.from_bytes(bytecode)
    assert header.team_mode is True


def test_bytecode_header_includes_team_count():
    """Bytecode header should include team count."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="team-game",
        generation=0,
        player_count=4,
        max_turns=100,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        team_mode=True,
        teams=((0, 2), (1, 3)),
    )
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)
    header = BytecodeHeader.from_bytes(bytecode)
    assert header.team_count == 2


def test_compile_teams_produces_valid_bytes():
    """compile_teams should produce parseable team data."""
    from darwindeck.genome.bytecode import compile_teams

    teams = ((0, 2), (1, 3))  # 2v2
    team_bytes = compile_teams(teams)
    # Format: [num_teams(1)][team0_size(1)][team0_players...][team1_size(1)][team1_players...]
    # Expected: [2][2][0][2][2][1][3] = 7 bytes
    assert len(team_bytes) >= 3  # At minimum: num_teams + 2x team_size bytes
    assert team_bytes[0] == 2  # 2 teams


def test_bytecode_no_teams_has_zero_team_mode():
    """Non-team genome should have team_mode=False in header."""
    genome = GameGenome(
        schema_version="1.0",
        genome_id="no-teams",
        generation=0,
        player_count=2,
        max_turns=100,
        setup=SetupRules(cards_per_player=5),
        turn_structure=TurnStructure(phases=[]),
        special_effects=[],
        win_conditions=[WinCondition(type="empty_hand")],
        scoring_rules=[],
        team_mode=False,
    )
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)
    header = BytecodeHeader.from_bytes(bytecode)
    assert header.team_mode is False
    assert header.team_count == 0


def test_compile_bidding_phase():
    """BiddingPhase compiles to correct bytecode."""
    from darwindeck.genome.schema import BiddingPhase, ContractScoring
    from darwindeck.genome.bytecode import compile_bidding_phase, OPCODE_BIDDING_PHASE

    phase = BiddingPhase(min_bid=1, max_bid=13, allow_nil=True)
    scoring = ContractScoring()

    bytecode = compile_bidding_phase(phase, scoring)

    # Opcode 70 for BiddingPhase
    assert bytecode[0] == OPCODE_BIDDING_PHASE
    # min_bid = 1
    assert bytecode[1] == 1
    # max_bid = 13
    assert bytecode[2] == 13
    # flags: bit 0 = allow_nil (1)
    assert bytecode[3] == 0x01
    # ContractScoring follows (12 bytes)
    assert len(bytecode) == 4 + 12  # 16 bytes total


def test_compile_genome_with_custom_contract_scoring():
    """Verify genome's contract_scoring is used during BiddingPhase compilation."""
    from darwindeck.genome.bytecode import BytecodeCompiler, BytecodeHeader, OPCODE_BIDDING_PHASE
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition,
        BiddingPhase, TrickPhase, ContractScoring, Suit
    )

    # Create custom scoring with non-default values
    custom_scoring = ContractScoring(
        points_per_trick_bid=20,  # Non-default (default is 10)
        nil_bonus=200,            # Non-default (default is 100)
    )

    genome = GameGenome(
        schema_version="1.0",
        genome_id="test_bidding",
        generation=0,
        setup=SetupRules(cards_per_player=13),
        turn_structure=TurnStructure(
            phases=(BiddingPhase(), TrickPhase(trump_suit=Suit.SPADES)),
            is_trick_based=True,
        ),
        special_effects=[],
        win_conditions=(WinCondition(type="score_threshold", threshold=500),),
        scoring_rules=[],
        contract_scoring=custom_scoring,
    )

    # Test using constructor-based compilation
    compiler = BytecodeCompiler(genome)
    bytecode = compiler.compile()

    assert bytecode is not None

    # Find the BiddingPhase in the bytecode
    # The phase type byte (7) precedes the OPCODE_BIDDING_PHASE (70)
    # Turn structure: phase_count:4 + [phase_type:1 + phase_data]...
    header = BytecodeHeader.from_bytes(bytecode)
    turn_offset = header.turn_structure_offset

    # Read phase count (4 bytes)
    phase_count = struct.unpack("!I", bytecode[turn_offset:turn_offset + 4])[0]
    assert phase_count == 2  # BiddingPhase + TrickPhase

    # First phase is BiddingPhase
    # Format: phase_type:1 + bidding_data (17 bytes: opcode + min + max + flags + scoring)
    phase_start = turn_offset + 4
    phase_type = bytecode[phase_start]
    assert phase_type == 7  # BiddingPhase type

    # The bidding data follows immediately
    bidding_data_start = phase_start + 1
    assert bytecode[bidding_data_start] == OPCODE_BIDDING_PHASE  # opcode 70

    # ContractScoring starts at offset 4 within bidding_data
    # Format: [opcode:1][min_bid:1][max_bid:1][flags:1][scoring:12]
    scoring_start = bidding_data_start + 4

    # First byte of scoring is points_per_trick_bid (should be 20, not default 10)
    points_per_trick_bid = bytecode[scoring_start]
    assert points_per_trick_bid == 20, f"Expected 20, got {points_per_trick_bid} - custom contract_scoring not used"

    # nil_bonus is a uint16 little-endian at offset +3 and +4 (after points, overtrick, failed_penalty)
    nil_bonus_low = bytecode[scoring_start + 3]
    nil_bonus_high = bytecode[scoring_start + 4]
    nil_bonus = nil_bonus_low + (nil_bonus_high << 8)
    assert nil_bonus == 200, f"Expected 200, got {nil_bonus} - custom contract_scoring not used"


def test_compile_genome_with_default_contract_scoring():
    """Verify default contract_scoring is used when genome has None."""
    from darwindeck.genome.bytecode import BytecodeCompiler, BytecodeHeader, OPCODE_BIDDING_PHASE
    from darwindeck.genome.schema import (
        GameGenome, SetupRules, TurnStructure, WinCondition,
        BiddingPhase, TrickPhase, Suit
    )

    # No contract_scoring specified - should use defaults
    genome = GameGenome(
        schema_version="1.0",
        genome_id="test_bidding_defaults",
        generation=0,
        setup=SetupRules(cards_per_player=13),
        turn_structure=TurnStructure(
            phases=(BiddingPhase(), TrickPhase(trump_suit=Suit.SPADES)),
            is_trick_based=True,
        ),
        special_effects=[],
        win_conditions=(WinCondition(type="score_threshold", threshold=500),),
        scoring_rules=[],
        # contract_scoring is None by default
    )

    compiler = BytecodeCompiler(genome)
    bytecode = compiler.compile()

    header = BytecodeHeader.from_bytes(bytecode)
    turn_offset = header.turn_structure_offset

    # First phase is BiddingPhase
    phase_start = turn_offset + 4
    bidding_data_start = phase_start + 1
    scoring_start = bidding_data_start + 4

    # Should have default points_per_trick_bid = 10
    points_per_trick_bid = bytecode[scoring_start]
    assert points_per_trick_bid == 10, f"Expected default 10, got {points_per_trick_bid}"

    # Should have default nil_bonus = 100
    nil_bonus_low = bytecode[scoring_start + 3]
    nil_bonus_high = bytecode[scoring_start + 4]
    nil_bonus = nil_bonus_low + (nil_bonus_high << 8)
    assert nil_bonus == 100, f"Expected default 100, got {nil_bonus}"
