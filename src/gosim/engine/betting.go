package engine

// BettingAction represents a betting action type
type BettingAction int

const (
	BettingCheck BettingAction = iota
	BettingBet
	BettingCall
	BettingRaise
	BettingAllIn
	BettingFold
)

// GenerateBettingMoves returns all valid betting actions for a player
func GenerateBettingMoves(gs *GameState, phase *BettingPhaseData, playerID int) []BettingAction {
	player := &gs.Players[playerID]
	moves := make([]BettingAction, 0, 4)

	// Can't act if folded, all-in, or no chips
	if player.HasFolded || player.IsAllIn || player.Chips <= 0 {
		return moves
	}

	toCall := gs.CurrentBet - player.CurrentBet

	if toCall == 0 {
		// No bet to match
		moves = append(moves, BettingCheck)
		if player.Chips >= int64(phase.MinBet) {
			moves = append(moves, BettingBet)
		} else if player.Chips > 0 {
			// Can't afford min bet, but can go all-in
			moves = append(moves, BettingAllIn)
		}
	} else {
		// Must match, raise, all-in, or fold
		if player.Chips >= toCall {
			moves = append(moves, BettingCall)
			if player.Chips >= toCall+int64(phase.MinBet) && gs.RaiseCount < phase.MaxRaises {
				moves = append(moves, BettingRaise)
			}
		}
		if player.Chips > 0 && player.Chips < toCall {
			// Can't afford call, but can go all-in
			moves = append(moves, BettingAllIn)
		}
		moves = append(moves, BettingFold)
	}

	return moves
}

// ApplyBettingAction executes a betting action, mutating the game state
func ApplyBettingAction(gs *GameState, phase *BettingPhaseData, playerID int, action BettingAction) {
	player := &gs.Players[playerID]

	switch action {
	case BettingCheck:
		// No change
	case BettingBet:
		player.Chips -= int64(phase.MinBet)
		player.CurrentBet += int64(phase.MinBet)
		gs.Pot += int64(phase.MinBet)
		gs.CurrentBet = int64(phase.MinBet)
	case BettingCall:
		toCall := gs.CurrentBet - player.CurrentBet
		player.Chips -= toCall
		player.CurrentBet = gs.CurrentBet
		gs.Pot += toCall
	case BettingRaise:
		toCall := gs.CurrentBet - player.CurrentBet
		raiseAmount := toCall + int64(phase.MinBet)
		player.Chips -= raiseAmount
		player.CurrentBet = gs.CurrentBet + int64(phase.MinBet)
		gs.Pot += raiseAmount
		gs.CurrentBet = player.CurrentBet
		gs.RaiseCount++
	case BettingAllIn:
		amount := player.Chips
		player.Chips = 0
		player.CurrentBet += amount
		gs.Pot += amount
		player.IsAllIn = true
		if player.CurrentBet > gs.CurrentBet {
			gs.CurrentBet = player.CurrentBet
		}
	case BettingFold:
		player.HasFolded = true
	}
}

// CountActivePlayers returns the number of players who haven't folded
func CountActivePlayers(gs *GameState) int {
	count := 0
	for _, p := range gs.Players {
		if !p.HasFolded {
			count++
		}
	}
	return count
}

// CountActingPlayers returns the number of players who can still act
// (not folded, not all-in, and have chips)
func CountActingPlayers(gs *GameState) int {
	count := 0
	for _, p := range gs.Players {
		if !p.HasFolded && !p.IsAllIn && p.Chips > 0 {
			count++
		}
	}
	return count
}

// AllBetsMatched returns true if all active players have matched the current bet
// or are all-in/folded
func AllBetsMatched(gs *GameState) bool {
	for _, p := range gs.Players {
		if !p.HasFolded && !p.IsAllIn && p.CurrentBet != gs.CurrentBet {
			return false
		}
	}
	return true
}

// ResolveShowdown determines which players are eligible to win the pot
// Returns a slice of player IDs that are still in the hand (not folded)
// If only one player remains, they win automatically
// If multiple players remain, actual hand comparison is done elsewhere
func ResolveShowdown(gs *GameState) []int {
	activePlayers := []int{}
	for i, p := range gs.Players {
		if !p.HasFolded {
			activePlayers = append(activePlayers, i)
		}
	}

	return activePlayers
}

// AwardPot distributes the pot to the winner(s)
// If multiple winners, pot is split evenly with remainder going to first winner
func AwardPot(gs *GameState, winnerIDs []int) {
	if len(winnerIDs) == 0 {
		return
	}

	// Split pot evenly among winners
	share := gs.Pot / int64(len(winnerIDs))
	remainder := gs.Pot % int64(len(winnerIDs))

	for i, winnerID := range winnerIDs {
		gs.Players[winnerID].Chips += share
		if i == 0 {
			gs.Players[winnerID].Chips += remainder
		}
	}
	gs.Pot = 0
}

// ============================================================================
// AI Betting Action Selection
// ============================================================================

// SelectRandomBettingAction picks a random action from available moves.
func SelectRandomBettingAction(moves []BettingAction, rngIntn func(n int) int) BettingAction {
	if len(moves) == 0 {
		return BettingFold // Fallback
	}
	return moves[rngIntn(len(moves))]
}

// SelectGreedyBettingAction picks action based on hand strength heuristic.
// strongThreshold = 0.7, mediumThreshold = 0.3
func SelectGreedyBettingAction(gs *GameState, moves []BettingAction, handStrength float64) BettingAction {
	// Strong hand (>0.7): Raise > Bet > AllIn
	if handStrength > 0.7 {
		if containsBettingAction(moves, BettingRaise) {
			return BettingRaise
		}
		if containsBettingAction(moves, BettingBet) {
			return BettingBet
		}
		if containsBettingAction(moves, BettingAllIn) {
			return BettingAllIn
		}
	}

	// Medium hand (>0.3): Call > Check
	if handStrength > 0.3 {
		if containsBettingAction(moves, BettingCall) {
			return BettingCall
		}
		if containsBettingAction(moves, BettingCheck) {
			return BettingCheck
		}
	}

	// Weak hand: Check > Fold
	if containsBettingAction(moves, BettingCheck) {
		return BettingCheck
	}
	return BettingFold
}

// containsBettingAction checks if action is in moves
func containsBettingAction(moves []BettingAction, target BettingAction) bool {
	for _, m := range moves {
		if m == target {
			return true
		}
	}
	return false
}

// EvaluateHandStrength returns a 0-1 score based on poker hand ranking heuristics.
// Simple implementation: based on high cards and pairs.
// Rank values: 0=Ace, 1-9=2-10, 10=Jack, 11=Queen, 12=King
// For scoring, Ace is high (treated as 13), King is 12, etc.
func EvaluateHandStrength(hand []Card) float64 {
	if len(hand) == 0 {
		return 0.0
	}

	// Count pairs, trips, etc.
	rankCounts := make(map[uint8]int)
	for _, card := range hand {
		rankCounts[card.Rank]++
	}

	maxCount := 0
	highRank := uint8(0)
	for rank, count := range rankCounts {
		if count > maxCount {
			maxCount = count
		}
		// Convert rank for comparison: Ace (0) becomes highest (13)
		effectiveRank := rank
		if rank == 0 {
			effectiveRank = 13 // Ace high
		}
		if effectiveRank > highRank {
			highRank = effectiveRank
		}
	}

	// Score components
	// pairScore: 0 for no pair, 0.2 for pair, 0.4 for trips, 0.6 for quads
	pairScore := float64(maxCount-1) * 0.2
	// highCardScore: 0-0.4 based on highest card (Ace = 13, King = 12)
	highCardScore := float64(highRank) / 13.0 * 0.4

	return minFloat64(pairScore+highCardScore, 1.0)
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
