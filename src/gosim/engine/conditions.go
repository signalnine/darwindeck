package engine

import (
	"encoding/binary"
	"sort"
)

// EvaluateCondition checks if condition is true for given state
func EvaluateCondition(state *GameState, playerID uint8, conditionBytes []byte) bool {
	if len(conditionBytes) < 7 {
		return false
	}

	opcode := OpCode(conditionBytes[0])
	operator := conditionBytes[1]
	value := int32(binary.BigEndian.Uint32(conditionBytes[2:6]))
	reference := conditionBytes[6]

	var actual int32

	switch opcode {
	case OpCheckHandSize:
		actual = int32(len(state.Players[playerID].Hand))

	case OpCheckLocationSize:
		switch Location(reference) {
		case LocationDeck:
			actual = int32(len(state.Deck))
		case LocationDiscard:
			actual = int32(len(state.Discard))
		case LocationTableau:
			if len(state.Tableau) > 0 {
				actual = int32(len(state.Tableau[0]))
			}
		}

	case OpCheckCardRank:
		// Check if card at index matches rank
		refCard := getReferencedCard(state, reference)
		if refCard != nil && int(refCard.Rank) == int(value) {
			return true
		}
		return false

	case OpCheckCardSuit:
		refCard := getReferencedCard(state, reference)
		if refCard != nil && int(refCard.Suit) == int(value) {
			return true
		}
		return false

	// Optional extensions: betting conditions (use int64)
	case OpCheckChipCount:
		actual64 := state.Players[playerID].Chips
		return compareInt64(actual64, operator, int64(value))

	case OpCheckPotSize:
		actual64 := state.Pot
		return compareInt64(actual64, operator, int64(value))

	case OpCheckCurrentBet:
		actual64 := state.CurrentBet
		return compareInt64(actual64, operator, int64(value))

	case OpCheckCanAfford:
		actual64 := state.Players[playerID].Chips
		// Check if player can afford the value
		return actual64 >= int64(value)

	// Optional extensions: pattern matching
	case OpCheckHasSetOfN:
		// Detect N cards of same rank in player's hand
		requiredCount := int(value)
		rankCounts := make(map[uint8]int)

		for _, card := range state.Players[playerID].Hand {
			rankCounts[card.Rank]++
			if rankCounts[card.Rank] >= requiredCount {
				return true
			}
		}
		return false

	case OpCheckHasRunOfN:
		// Detect N cards in sequence (any suit, sequential ranks)
		requiredLength := int(value)
		hand := state.Players[playerID].Hand

		if len(hand) < requiredLength {
			return false
		}

		// Sort by rank
		sorted := make([]Card, len(hand))
		copy(sorted, hand)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Rank < sorted[j].Rank
		})

		// Find sequential run
		runLength := 1
		for i := 1; i < len(sorted); i++ {
			if sorted[i].Rank == sorted[i-1].Rank+1 {
				runLength++
				if runLength >= requiredLength {
					return true
				}
			} else if sorted[i].Rank != sorted[i-1].Rank {
				// Different rank, not sequential - reset counter
				runLength = 1
			}
			// Same rank = continue current run length
		}
		return false

	case OpCheckHasMatchingPair:
		// Detect two cards with matching rank and color (Old Maid style)
		hand := state.Players[playerID].Hand

		for i := 0; i < len(hand); i++ {
			for j := i + 1; j < len(hand); j++ {
				// Check if same rank and same color
				if hand[i].Rank == hand[j].Rank {
					color1 := hand[i].Suit % 2 // 0=red (H,D), 1=black (C,S)
					color2 := hand[j].Suit % 2
					if color1 == color2 {
						return true
					}
				}
			}
		}
		return false

	default:
		return false
	}

	// Apply operator
	switch OpCode(operator + 50) {
	case OpEQ:
		return actual == value
	case OpNE:
		return actual != value
	case OpLT:
		return actual < value
	case OpGT:
		return actual > value
	case OpLE:
		return actual <= value
	case OpGE:
		return actual >= value
	default:
		return false
	}
}

// compareInt64 applies comparison operator to int64 values
func compareInt64(actual int64, operator uint8, value int64) bool {
	switch OpCode(operator + 50) {
	case OpEQ:
		return actual == value
	case OpNE:
		return actual != value
	case OpLT:
		return actual < value
	case OpGT:
		return actual > value
	case OpLE:
		return actual <= value
	case OpGE:
		return actual >= value
	default:
		return false
	}
}

func getReferencedCard(state *GameState, reference uint8) *Card {
	switch reference {
	case 1: // top_discard
		if len(state.Discard) > 0 {
			return &state.Discard[len(state.Discard)-1]
		}
	case 2: // last_played (tableau top)
		if len(state.Tableau) > 0 && len(state.Tableau[0]) > 0 {
			pile := state.Tableau[0]
			return &pile[len(pile)-1]
		}
	}
	return nil
}
