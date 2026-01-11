package engine

import "sort"

// HandRank represents poker hand rankings (higher = better)
type HandRank uint8

const (
	HighCard HandRank = iota
	OnePair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
	RoyalFlush
)

// PokerHand represents an evaluated poker hand
type PokerHand struct {
	Rank     HandRank
	Kickers  []uint8 // For tie-breaking (high cards)
}

// EvaluatePokerHand evaluates a 5-card poker hand
func EvaluatePokerHand(cards []Card) PokerHand {
	if len(cards) != 5 {
		return PokerHand{Rank: HighCard}
	}

	// Sort cards by rank descending
	sorted := make([]Card, 5)
	copy(sorted, cards)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Rank > sorted[j].Rank
	})

	// Check for flush (all same suit)
	isFlush := true
	for i := 1; i < 5; i++ {
		if sorted[i].Suit != sorted[0].Suit {
			isFlush = false
			break
		}
	}

	// Check for straight (5 consecutive ranks)
	isStraight := true
	for i := 1; i < 5; i++ {
		if sorted[i-1].Rank != sorted[i].Rank+1 {
			isStraight = false
			break
		}
	}

	// Special case: A-2-3-4-5 (wheel straight)
	// Ace is rank 12, so check for 12-3-2-1-0
	if !isStraight && sorted[0].Rank == 12 && sorted[1].Rank == 3 &&
		sorted[2].Rank == 2 && sorted[3].Rank == 1 && sorted[4].Rank == 0 {
		isStraight = true
		// Reorder for wheel: 3-2-1-0-12 becomes 5-high straight
		sorted = []Card{sorted[1], sorted[2], sorted[3], sorted[4], sorted[0]}
	}

	// Count ranks
	rankCounts := make(map[uint8]int)
	for _, card := range sorted {
		rankCounts[card.Rank]++
	}

	// Determine hand type based on rank counts
	var pairs, threes, fours int
	for _, count := range rankCounts {
		switch count {
		case 2:
			pairs++
		case 3:
			threes++
		case 4:
			fours++
		}
	}

	// Build kickers list (all ranks sorted descending)
	kickers := make([]uint8, 5)
	for i, card := range sorted {
		kickers[i] = card.Rank
	}

	// Determine hand rank
	if isStraight && isFlush {
		if sorted[0].Rank == 12 && sorted[1].Rank == 11 {
			// A-K-Q-J-10 of same suit
			return PokerHand{Rank: RoyalFlush, Kickers: kickers}
		}
		return PokerHand{Rank: StraightFlush, Kickers: kickers}
	}

	if fours == 1 {
		return PokerHand{Rank: FourOfAKind, Kickers: kickers}
	}

	if threes == 1 && pairs == 1 {
		return PokerHand{Rank: FullHouse, Kickers: kickers}
	}

	if isFlush {
		return PokerHand{Rank: Flush, Kickers: kickers}
	}

	if isStraight {
		return PokerHand{Rank: Straight, Kickers: kickers}
	}

	if threes == 1 {
		return PokerHand{Rank: ThreeOfAKind, Kickers: kickers}
	}

	if pairs == 2 {
		return PokerHand{Rank: TwoPair, Kickers: kickers}
	}

	if pairs == 1 {
		return PokerHand{Rank: OnePair, Kickers: kickers}
	}

	return PokerHand{Rank: HighCard, Kickers: kickers}
}

// ComparePokerHands compares two poker hands, returns:
// -1 if hand1 < hand2
//  0 if hand1 == hand2
//  1 if hand1 > hand2
func ComparePokerHands(hand1, hand2 PokerHand) int {
	if hand1.Rank > hand2.Rank {
		return 1
	}
	if hand1.Rank < hand2.Rank {
		return -1
	}

	// Same rank - compare kickers
	for i := 0; i < len(hand1.Kickers) && i < len(hand2.Kickers); i++ {
		if hand1.Kickers[i] > hand2.Kickers[i] {
			return 1
		}
		if hand1.Kickers[i] < hand2.Kickers[i] {
			return -1
		}
	}

	return 0 // Exact tie
}

// FindBestPokerWinner finds the player with the best poker hand
// Returns player ID or -1 for tie
func FindBestPokerWinner(state *GameState, numPlayers int) int8 {
	if numPlayers == 0 {
		numPlayers = 2
	}

	bestPlayer := int8(-1)
	var bestHand PokerHand

	for playerID := 0; playerID < numPlayers; playerID++ {
		hand := state.Players[playerID].Hand
		if len(hand) != 5 {
			continue // Skip players without exactly 5 cards
		}

		pokerHand := EvaluatePokerHand(hand)

		if bestPlayer == -1 {
			bestPlayer = int8(playerID)
			bestHand = pokerHand
		} else {
			cmp := ComparePokerHands(pokerHand, bestHand)
			if cmp > 0 {
				bestPlayer = int8(playerID)
				bestHand = pokerHand
			} else if cmp == 0 {
				// Tie - for simplicity, first player wins ties
				// In real poker, pot would be split
			}
		}
	}

	return bestPlayer
}
