package engine

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// OpCode matches Python bytecode.py
type OpCode uint8

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

// BytecodeHeader matches Python (36 bytes, not 32!)
type BytecodeHeader struct {
	Version             uint32
	GenomeIDHash        uint64
	PlayerCount         uint32
	MaxTurns            uint32
	SetupOffset         int32
	TurnStructureOffset int32
	WinConditionsOffset int32
	ScoringOffset       int32
}

// ParseHeader extracts header from bytecode
func ParseHeader(bytecode []byte) (*BytecodeHeader, error) {
	if len(bytecode) < 36 {
		return nil, errors.New("bytecode too short for header")
	}

	h := &BytecodeHeader{}
	h.Version = binary.BigEndian.Uint32(bytecode[0:4])
	h.GenomeIDHash = binary.BigEndian.Uint64(bytecode[4:12])
	h.PlayerCount = binary.BigEndian.Uint32(bytecode[12:16])
	h.MaxTurns = binary.BigEndian.Uint32(bytecode[16:20])
	h.SetupOffset = int32(binary.BigEndian.Uint32(bytecode[20:24]))
	h.TurnStructureOffset = int32(binary.BigEndian.Uint32(bytecode[24:28]))
	h.WinConditionsOffset = int32(binary.BigEndian.Uint32(bytecode[28:32]))
	h.ScoringOffset = int32(binary.BigEndian.Uint32(bytecode[32:36]))

	return h, nil
}

// Genome holds parsed bytecode sections
type Genome struct {
	Header        *BytecodeHeader
	Bytecode      []byte
	TurnPhases    []PhaseDescriptor
	WinConditions []WinCondition
}

type PhaseDescriptor struct {
	PhaseType uint8  // 1=Draw, 2=Play, 3=Discard, 4=Betting, 5=Claim
	Data      []byte // Raw bytes for this phase
}

type WinCondition struct {
	WinType   uint8
	Threshold int32
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
	if err := genome.parseWinConditions(); err != nil {
		return nil, err
	}

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
		case 1: // DrawPhase: source:1 + count:4 + mandatory:1 + has_condition:1 = 7 bytes
			baseLen := 7
			if offset+int32(baseLen) > int32(len(g.Bytecode)) {
				return errors.New("invalid draw phase data")
			}
			hasCondition := g.Bytecode[offset+6]
			phaseLen = baseLen
			if hasCondition == 1 {
				phaseLen += 7 // Add condition bytes
			}
		case 2: // PlayPhase: target:1 + min:1 + max:1 + mandatory:1 + conditionLen:4 + condition
			if offset+8 > int32(len(g.Bytecode)) {
				return errors.New("invalid play phase header")
			}
			conditionLen := int(binary.BigEndian.Uint32(g.Bytecode[offset+4 : offset+8]))
			phaseLen = 8 + conditionLen
		case 3: // DiscardPhase: target:1 + count:4 + mandatory:1 = 6 bytes
			phaseLen = 6
		case 4: // BettingPhase (optional)
			phaseLen = 21
		case 5: // ClaimPhase (optional)
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

func (g *Genome) parseWinConditions() error {
	offset := g.Header.WinConditionsOffset
	if offset < 0 || offset >= int32(len(g.Bytecode)) {
		return errors.New("invalid win conditions offset")
	}

	count := int(binary.BigEndian.Uint32(g.Bytecode[offset : offset+4]))
	offset += 4

	g.WinConditions = make([]WinCondition, count)

	for i := 0; i < count; i++ {
		if offset+5 > int32(len(g.Bytecode)) {
			return errors.New("win condition data exceeds bytecode length")
		}

		winType := g.Bytecode[offset]
		threshold := int32(binary.BigEndian.Uint32(g.Bytecode[offset+1 : offset+5]))

		g.WinConditions[i] = WinCondition{
			WinType:   winType,
			Threshold: threshold,
		}

		offset += 5
	}

	return nil
}
