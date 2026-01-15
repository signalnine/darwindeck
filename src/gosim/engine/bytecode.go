package engine

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// OpCode matches Python bytecode.py
type OpCode uint8

// Phase type constants
const (
	PhaseTypeDraw    = 1
	PhaseTypePlay    = 2
	PhaseTypeDiscard = 3
	PhaseTypeTrick   = 4
	PhaseTypeBetting = 5
	PhaseTypeClaim   = 6
)

const (
	// Conditions
	OpCheckHandSize OpCode = 0
	OpCheckCardRank OpCode = 1
	OpCheckCardSuit OpCode = 2
	OpCheckLocationSize OpCode = 3
	OpCheckSequence OpCode = 4
	// Optional extensions
	OpCheckHasSetOfN       OpCode = 5
	OpCheckHasRunOfN       OpCode = 6
	OpCheckHasMatchingPair OpCode = 7
	OpCheckChipCount       OpCode = 8
	OpCheckPotSize         OpCode = 9
	OpCheckCurrentBet      OpCode = 10
	OpCheckCanAfford       OpCode = 11
	// Card matching conditions (for valid_play_condition)
	OpCheckCardMatchesRank OpCode = 12 // Candidate card matches reference card's rank
	OpCheckCardMatchesSuit OpCode = 13 // Candidate card matches reference card's suit
	OpCheckCardBeatsTop    OpCode = 14 // Candidate card beats reference card (President)

	// Actions
	OpDrawCards        OpCode = 20
	OpPlayCard         OpCode = 21
	OpDiscardCard      OpCode = 22
	OpSkipTurn         OpCode = 23
	OpReverseOrder     OpCode = 24
	OpDrawFromOpponent OpCode = 25
	OpDiscardPairs     OpCode = 26
	OpBet              OpCode = 27
	OpCall             OpCode = 28
	OpRaise            OpCode = 29
	OpFold             OpCode = 30
	OpCheck            OpCode = 31
	OpAllIn            OpCode = 32
	OpClaim            OpCode = 33
	OpChallenge        OpCode = 34
	OpReveal           OpCode = 35

	// Control flow
	OpAnd OpCode = 40
	OpOr  OpCode = 41

	// Operators
	OpEQ OpCode = 50
	OpNE OpCode = 51
	OpLT OpCode = 52
	OpGT OpCode = 53
	OpLE OpCode = 54
	OpGE OpCode = 55
)

// BytecodeHeader matches Python bytecode format
// V1 format: 36 bytes (no version byte prefix)
// V2 format: 39 bytes (version byte at offset 0, struct at 1-36, tableau fields at 37-38)
type BytecodeHeader struct {
	BytecodeVersion     uint8  // V2+: bytecode format version (at byte 0)
	Version             uint32 // Legacy version field
	GenomeIDHash        uint64
	PlayerCount         uint32
	MaxTurns            uint32
	SetupOffset         int32
	TurnStructureOffset int32
	WinConditionsOffset int32
	ScoringOffset       int32
	TableauMode         uint8 // V2+: tableau mode (0=none, 1=war, 2=klondike, 3=build_sequences)
	SequenceDirection   uint8 // V2+: sequence direction (0=ascending, 1=descending, 2=both)
}

// ParseHeader extracts header from bytecode
// Supports both V1 (36 bytes, no version prefix) and V2 (39 bytes, version at byte 0)
func ParseHeader(bytecode []byte) (*BytecodeHeader, error) {
	if len(bytecode) < 36 {
		return nil, errors.New("bytecode too short for header")
	}

	// Check if this is V2 format (version byte at offset 0)
	// V2 bytecode has version == 2 at byte 0
	// V1 bytecode has the legacy version field (uint32) at bytes 0-3
	// We can distinguish because V1's legacy version is typically 1,
	// which would have bytes [0,0,0,1] - the first byte is 0, not 2
	if bytecode[0] == 2 {
		return parseV2Header(bytecode)
	}

	// V1 format (backward compatible)
	return parseV1Header(bytecode)
}

// parseV1Header parses the original 36-byte header format (no version byte prefix)
func parseV1Header(bytecode []byte) (*BytecodeHeader, error) {
	h := &BytecodeHeader{}
	h.BytecodeVersion = 1 // Assume V1 for legacy bytecode
	h.Version = binary.BigEndian.Uint32(bytecode[0:4])
	h.GenomeIDHash = binary.BigEndian.Uint64(bytecode[4:12])
	h.PlayerCount = binary.BigEndian.Uint32(bytecode[12:16])
	h.MaxTurns = binary.BigEndian.Uint32(bytecode[16:20])
	h.SetupOffset = int32(binary.BigEndian.Uint32(bytecode[20:24]))
	h.TurnStructureOffset = int32(binary.BigEndian.Uint32(bytecode[24:28]))
	h.WinConditionsOffset = int32(binary.BigEndian.Uint32(bytecode[28:32]))
	h.ScoringOffset = int32(binary.BigEndian.Uint32(bytecode[32:36]))
	// V1 has no tableau fields - leave as defaults (0)
	h.TableauMode = 0
	h.SequenceDirection = 0

	return h, nil
}

// parseV2Header parses the new 39-byte header format (version byte at offset 0)
// Format:
// - Byte 0: bytecode version (2)
// - Bytes 1-4: legacy version (uint32)
// - Bytes 5-12: genome_id_hash (uint64)
// - Bytes 13-16: player_count (uint32)
// - Bytes 17-20: max_turns (uint32)
// - Bytes 21-24: setup_offset (int32)
// - Bytes 25-28: turn_structure_offset (int32)
// - Bytes 29-32: win_conditions_offset (int32)
// - Bytes 33-36: scoring_offset (int32)
// - Byte 37: tableau_mode (uint8)
// - Byte 38: sequence_direction (uint8)
func parseV2Header(bytecode []byte) (*BytecodeHeader, error) {
	if len(bytecode) < 39 {
		return nil, fmt.Errorf("v2 bytecode too short: %d < 39", len(bytecode))
	}

	h := &BytecodeHeader{}
	h.BytecodeVersion = bytecode[0]
	h.Version = binary.BigEndian.Uint32(bytecode[1:5])
	h.GenomeIDHash = binary.BigEndian.Uint64(bytecode[5:13])
	h.PlayerCount = binary.BigEndian.Uint32(bytecode[13:17])
	h.MaxTurns = binary.BigEndian.Uint32(bytecode[17:21])
	h.SetupOffset = int32(binary.BigEndian.Uint32(bytecode[21:25]))
	h.TurnStructureOffset = int32(binary.BigEndian.Uint32(bytecode[25:29]))
	h.WinConditionsOffset = int32(binary.BigEndian.Uint32(bytecode[29:33]))
	h.ScoringOffset = int32(binary.BigEndian.Uint32(bytecode[33:37]))
	h.TableauMode = bytecode[37]
	h.SequenceDirection = bytecode[38]

	return h, nil
}

// Genome holds parsed bytecode sections
type Genome struct {
	Header        *BytecodeHeader
	Bytecode      []byte
	TurnPhases    []PhaseDescriptor
	WinConditions []WinCondition
	Effects       map[uint8]SpecialEffect // rank -> effect lookup
}

type PhaseDescriptor struct {
	PhaseType uint8  // 1=Draw, 2=Play, 3=Discard, 4=Trick, 5=Betting, 6=Claim
	Data      []byte // Raw bytes for this phase
}

// BettingPhaseData holds parsed betting phase parameters
type BettingPhaseData struct {
	MinBet    int // Minimum bet/raise amount
	MaxRaises int // Maximum raises per round (prevents infinite loops)
}

type WinCondition struct {
	WinType   uint8
	Threshold int32
}

// ParseBettingPhaseData extracts betting phase parameters from raw phase data.
// Expected format: min_bet:4 + max_raises:4 = 8 bytes
func ParseBettingPhaseData(data []byte) (*BettingPhaseData, error) {
	if len(data) < 8 {
		return nil, errors.New("betting phase data too short: need at least 8 bytes")
	}

	return &BettingPhaseData{
		MinBet:    int(binary.BigEndian.Uint32(data[0:4])),
		MaxRaises: int(binary.BigEndian.Uint32(data[4:8])),
	}, nil
}

// ParseGenome parses full bytecode into structured Genome
func ParseGenome(bytecode []byte) (*Genome, error) {
	header, err := ParseHeader(bytecode)
	if err != nil {
		return nil, err
	}

	genome := &Genome{
		Header:   header,
		Bytecode: bytecode,
	}

	// Parse turn structure
	if err := genome.parseTurnStructure(); err != nil {
		return nil, err
	}

	// Parse win conditions
	offset, err := genome.parseWinConditions()
	if err != nil {
		return nil, err
	}

	// Parse effects section (at end of bytecode)
	effects, _, err := parseEffects(bytecode, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to parse effects: %w", err)
	}
	genome.Effects = effects

	return genome, nil
}

func (g *Genome) parseTurnStructure() error {
	offset := g.Header.TurnStructureOffset
	if offset < 0 || offset >= int32(len(g.Bytecode)) {
		return errors.New("invalid turn structure offset")
	}

	phaseCount := int(binary.BigEndian.Uint32(g.Bytecode[offset : offset+4]))
	offset += 4

	g.TurnPhases = make([]PhaseDescriptor, 0, phaseCount)

	for i := 0; i < phaseCount; i++ {
		if offset >= int32(len(g.Bytecode)) {
			return errors.New("unexpected end of bytecode in turn structure")
		}
		phaseType := g.Bytecode[offset]
		offset++

		// Read phase data (format depends on phase type)
		// Python bytecode format (phase_type already read):
		var phaseLen int
		switch phaseType {
		case PhaseTypeDraw: // DrawPhase: source:1 + count:4 + mandatory:1 + has_condition:1 = 7 bytes
			baseLen := 7
			if offset+int32(baseLen) > int32(len(g.Bytecode)) {
				return errors.New("invalid draw phase data")
			}
			hasCondition := g.Bytecode[offset+6]
			phaseLen = baseLen
			if hasCondition == 1 {
				phaseLen += 7 // Add condition bytes
			}
		case PhaseTypePlay: // PlayPhase: target:1 + min:1 + max:1 + mandatory:1 + pass_if_unable:1 + conditionLen:4 + condition
			if offset+9 > int32(len(g.Bytecode)) {
				return errors.New("invalid play phase header")
			}
			conditionLen := int(binary.BigEndian.Uint32(g.Bytecode[offset+5 : offset+9]))
			phaseLen = 9 + conditionLen
		case PhaseTypeDiscard: // DiscardPhase: target:1 + count:4 + mandatory:1 = 6 bytes
			phaseLen = 6
		case PhaseTypeTrick: // TrickPhase: lead_suit_required:1 + trump_suit:1 + high_card_wins:1 + breaking_suit:1 = 4 bytes
			phaseLen = 4
		case PhaseTypeBetting: // BettingPhase: min_bet:4 + max_raises:4 = 8 bytes
			phaseLen = 8
		case PhaseTypeClaim: // ClaimPhase
			phaseLen = 10
		default:
			return fmt.Errorf("unknown phase type: %d", phaseType)
		}

		phaseEnd := offset + int32(phaseLen)
		if phaseEnd > int32(len(g.Bytecode)) {
			return errors.New("phase data exceeds bytecode length")
		}

		phaseData := make([]byte, phaseLen)
		copy(phaseData, g.Bytecode[offset:phaseEnd])
		offset = phaseEnd

		g.TurnPhases = append(g.TurnPhases, PhaseDescriptor{
			PhaseType: phaseType,
			Data:      phaseData,
		})
	}

	return nil
}

const OP_EFFECT_HEADER = 60

// parseEffects extracts special effects from bytecode
func parseEffects(data []byte, offset int) (map[uint8]SpecialEffect, int, error) {
	effects := make(map[uint8]SpecialEffect)

	// Bounds check: need at least 1 byte
	if offset >= len(data) {
		return effects, offset, nil // No effects section
	}

	if data[offset] != OP_EFFECT_HEADER {
		return effects, offset, nil // No effects section
	}
	offset++

	// Bounds check: need count byte
	if offset >= len(data) {
		return nil, offset, fmt.Errorf("truncated effects section: missing count")
	}

	count := int(data[offset])
	offset++

	// Bounds check: need 4 bytes per effect
	requiredBytes := count * 4
	if offset+requiredBytes > len(data) {
		return nil, offset, fmt.Errorf("truncated effects section: expected %d bytes, have %d",
			requiredBytes, len(data)-offset)
	}

	for i := 0; i < count; i++ {
		effect := SpecialEffect{
			TriggerRank: data[offset],
			EffectType:  data[offset+1],
			Target:      data[offset+2],
			Value:       data[offset+3],
		}
		// Note: Later effects with same rank overwrite earlier ones
		effects[effect.TriggerRank] = effect
		offset += 4
	}

	return effects, offset, nil
}

func (g *Genome) parseWinConditions() (int, error) {
	offset := g.Header.WinConditionsOffset
	if offset < 0 || offset >= int32(len(g.Bytecode)) {
		return 0, errors.New("invalid win conditions offset")
	}

	count := int(binary.BigEndian.Uint32(g.Bytecode[offset : offset+4]))
	offset += 4

	g.WinConditions = make([]WinCondition, count)

	for i := 0; i < count; i++ {
		if offset+5 > int32(len(g.Bytecode)) {
			return 0, errors.New("win condition data exceeds bytecode length")
		}

		winType := g.Bytecode[offset]
		threshold := int32(binary.BigEndian.Uint32(g.Bytecode[offset+1 : offset+5]))

		g.WinConditions[i] = WinCondition{
			WinType:   winType,
			Threshold: threshold,
		}

		offset += 5
	}

	return int(offset), nil
}
