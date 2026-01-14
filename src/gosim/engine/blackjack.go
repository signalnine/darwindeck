package engine

// CalculateBlackjackValue calculates the value of a blackjack hand
// Returns the best value (using Ace as 11 if it doesn't bust, otherwise 1)
// Card.Rank encoding: 0=Ace, 1-9=2-10, 10-12=J,Q,K
func CalculateBlackjackValue(cards []Card) int {
	if len(cards) == 0 {
		return 0
	}

	total := 0
	aceCount := 0

	for _, card := range cards {
		switch {
		case card.Rank == 0: // Ace
			aceCount++
			total += 11 // Initially count Ace as 11
		case card.Rank >= 10: // J, Q, K (ranks 10, 11, 12)
			total += 10
		default: // 2-10 (ranks 1-9)
			total += int(card.Rank) + 1 // rank 1 = 2, rank 9 = 10
		}
	}

	// Convert Aces from 11 to 1 as needed to avoid bust
	for aceCount > 0 && total > 21 {
		total -= 10 // Change an Ace from 11 to 1
		aceCount--
	}

	return total
}

// FindBestBlackjackWinner finds the player with the best blackjack hand
// Returns player ID or -1 for tie
// In blackjack, closest to 21 without going over wins
// Bust (over 21) loses to any non-bust hand
func FindBestBlackjackWinner(state *GameState, numPlayers int) int8 {
	if numPlayers == 0 {
		numPlayers = 2
	}

	bestPlayer := int8(-1)
	bestValue := 0

	for playerID := 0; playerID < numPlayers && playerID < len(state.Players); playerID++ {
		// Skip folded players
		if state.Players[playerID].HasFolded {
			continue
		}

		hand := state.Players[playerID].Hand
		if len(hand) == 0 {
			continue // Skip players with no cards
		}

		value := CalculateBlackjackValue(hand)

		// Skip busted hands
		if value > 21 {
			continue
		}

		if value > bestValue {
			bestValue = value
			bestPlayer = int8(playerID)
		} else if value == bestValue && bestPlayer >= 0 {
			// Tie - for simplicity, first player wins ties
			// In real blackjack, would be a push
		}
	}

	return bestPlayer
}

// IsBlackjackGame checks if the genome uses blackjack-style scoring
// (high_score win condition with threshold around 21)
func IsBlackjackGame(genome *Genome) bool {
	for _, wc := range genome.WinConditions {
		if wc.WinType == 1 { // high_score
			// Threshold of 21 strongly suggests blackjack
			if wc.Threshold == 21 {
				return true
			}
		}
	}
	return false
}

// SelectBlackjackMove implements basic blackjack strategy
// Hit on < 17, stand on >= 17 (or if already busted)
// Returns the index into the moves slice
func SelectBlackjackMove(state *GameState, moves []LegalMove) int {
	if len(moves) == 0 {
		return -1
	}

	// Calculate current hand value
	hand := state.Players[state.CurrentPlayer].Hand
	handValue := CalculateBlackjackValue(hand)

	// Find hit and stand moves
	hitIdx := -1
	standIdx := -1
	for i, move := range moves {
		if move.CardIndex == MoveDraw { // -1 = hit
			hitIdx = i
		} else if move.CardIndex == MoveDrawPass { // -3 = stand
			standIdx = i
		}
	}

	// Basic strategy: hit on < 17, stand on >= 17
	// Also stand if busted (>=22) to avoid making it worse
	if handValue >= 17 && standIdx >= 0 {
		return standIdx
	}
	if handValue < 17 && hitIdx >= 0 {
		return hitIdx
	}

	// Fallback: return first available move
	return 0
}

// IsBlackjackDrawMove checks if a move is a hit/stand from blackjack
func IsBlackjackDrawMove(move *LegalMove) bool {
	return move.CardIndex == MoveDraw || move.CardIndex == MoveDrawPass
}
