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
		case LocationHand:
			actual = int32(len(state.Players[playerID].Hand))
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
	case 2, 3: // last_played / tableau_top (top of tableau pile)
		// Reference 2 = "last_played", Reference 3 = "tableau" (both mean top of tableau)
		if len(state.Tableau) > 0 && len(state.Tableau[0]) > 0 {
			pile := state.Tableau[0]
			return &pile[len(pile)-1]
		}
	}
	return nil
}

// EvaluateCardCondition checks if a candidate card satisfies a condition.
// Used for valid_play_condition evaluation in PlayPhase.
func EvaluateCardCondition(state *GameState, playerID uint8, candidateCard Card, conditionBytes []byte) bool {
	if len(conditionBytes) < 7 {
		return false
	}

	opcode := OpCode(conditionBytes[0])
	// operator is at conditionBytes[1] - unused for card matching
	value := int32(binary.BigEndian.Uint32(conditionBytes[2:6]))
	reference := conditionBytes[6]

	switch opcode {
	case OpCheckCardRank:
		// CARD_IS_RANK: Check if candidate card is a specific rank (for wild cards)
		return int32(candidateCard.Rank) == value

	case OpCheckCardSuit:
		// CARD_IS_SUIT: Check if candidate card is a specific suit
		return int32(candidateCard.Suit) == value

	case OpCheckCardMatchesRank:
		// CARD_MATCHES_RANK: Check if candidate matches reference card's rank
		refCard := getReferencedCard(state, reference)
		if refCard == nil {
			return true // No reference card = any card valid
		}
		return candidateCard.Rank == refCard.Rank

	case OpCheckCardMatchesSuit:
		// CARD_MATCHES_SUIT: Check if candidate matches reference card's suit
		refCard := getReferencedCard(state, reference)
		if refCard == nil {
			return true // No reference card = any card valid
		}
		return candidateCard.Suit == refCard.Suit

	case OpCheckCardBeatsTop:
		// CARD_BEATS_TOP: Check if candidate beats reference card (President/Daifugo)
		// Higher rank wins, same rank is allowed (multiple cards of same rank can be played)
		refCard := getReferencedCard(state, reference)
		if refCard == nil {
			return true // No reference card = any card valid
		}
		return candidateCard.Rank >= refCard.Rank

	case OpAnd:
		// Compound AND: all nested conditions must be true
		return evaluateCompoundCardCondition(state, playerID, candidateCard, conditionBytes, true)

	case OpOr:
		// Compound OR: at least one nested condition must be true
		return evaluateCompoundCardCondition(state, playerID, candidateCard, conditionBytes, false)

	default:
		// For non-card conditions, delegate to EvaluateCondition
		return EvaluateCondition(state, playerID, conditionBytes)
	}
}

// evaluateCompoundCardCondition evaluates compound AND/OR conditions for a card
func evaluateCompoundCardCondition(state *GameState, playerID uint8, candidateCard Card, conditionBytes []byte, isAnd bool) bool {
	if len(conditionBytes) < 5 {
		return false
	}

	// Format: [OpCode:1][Count:4][nested conditions...]
	count := binary.BigEndian.Uint32(conditionBytes[1:5])
	offset := 5

	for i := uint32(0); i < count; i++ {
		if offset+7 > len(conditionBytes) {
			return false
		}

		// Determine the size of this condition
		nestedOpcode := OpCode(conditionBytes[offset])
		var nestedLen int

		if nestedOpcode == OpAnd || nestedOpcode == OpOr {
			// Compound condition - need to calculate size
			nestedLen = calculateCompoundConditionSize(conditionBytes[offset:])
		} else {
			// Simple condition is 7 bytes
			nestedLen = 7
		}

		if offset+nestedLen > len(conditionBytes) {
			return false
		}

		result := EvaluateCardCondition(state, playerID, candidateCard, conditionBytes[offset:offset+nestedLen])

		if isAnd && !result {
			return false // AND: any false = false
		}
		if !isAnd && result {
			return true // OR: any true = true
		}

		offset += nestedLen
	}

	return isAnd // AND returns true if all passed, OR returns false if none passed
}

// calculateCompoundConditionSize returns the total byte size of a compound condition
func calculateCompoundConditionSize(conditionBytes []byte) int {
	if len(conditionBytes) < 5 {
		return 0
	}

	count := binary.BigEndian.Uint32(conditionBytes[1:5])
	size := 5 // header

	offset := 5
	for i := uint32(0); i < count; i++ {
		if offset >= len(conditionBytes) {
			break
		}

		nestedOpcode := OpCode(conditionBytes[offset])
		if nestedOpcode == OpAnd || nestedOpcode == OpOr {
			nestedLen := calculateCompoundConditionSize(conditionBytes[offset:])
			size += nestedLen
			offset += nestedLen
		} else {
			size += 7
			offset += 7
		}
	}

	return size
}
