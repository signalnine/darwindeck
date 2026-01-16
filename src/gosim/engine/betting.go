package engine

import "sort"

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

// ============================================================================
// Explicit Hand Pattern Evaluation (for poker-style games)
// ============================================================================

// EvaluateHandPattern returns the rank_priority of the best matching pattern.
// Patterns are expected to be sorted by priority (highest first).
// Returns 0 if no pattern matches or evaluation is nil/wrong method.
func EvaluateHandPattern(hand []Card, eval *HandEvaluation) uint8 {
	if eval == nil || eval.Method != EvalMethodPatternMatch || len(eval.Patterns) == 0 {
		return 0
	}

	// Try each pattern (they should be sorted by priority, highest first)
	for _, pattern := range eval.Patterns {
		if matchesPattern(hand, pattern) {
			return pattern.RankPriority
		}
	}

	return 0
}

// matchesPattern checks if a hand matches a pattern
func matchesPattern(hand []Card, p HandPattern) bool {
	// Check required count
	if p.RequiredCount > 0 && len(hand) != int(p.RequiredCount) {
		return false
	}

	// Check same suit count (for flush)
	if p.SameSuitCount > 0 {
		suitCounts := make(map[uint8]int)
		for _, c := range hand {
			suitCounts[c.Suit]++
		}
		maxSuit := 0
		for _, count := range suitCounts {
			if count > maxSuit {
				maxSuit = count
			}
		}
		if maxSuit < int(p.SameSuitCount) {
			return false
		}
	}

	// Check same rank groups (for pairs, trips, full house)
	if len(p.SameRankGroups) > 0 {
		rankCounts := make(map[uint8]int)
		for _, c := range hand {
			rankCounts[c.Rank]++
		}

		// Sort counts descending
		counts := make([]int, 0, len(rankCounts))
		for _, count := range rankCounts {
			counts = append(counts, count)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(counts)))

		// Check if groups match
		for i, required := range p.SameRankGroups {
			if i >= len(counts) || counts[i] < int(required) {
				return false
			}
		}
	}

	// Check sequence (for straight)
	if p.SequenceLength > 0 {
		if !isSequence(hand, int(p.SequenceLength), p.SequenceWrap) {
			return false
		}
	}

	// Check required ranks (for specific hands like royal flush)
	if len(p.RequiredRanks) > 0 {
		rankSet := make(map[uint8]bool)
		for _, c := range hand {
			rankSet[c.Rank] = true
		}
		for _, r := range p.RequiredRanks {
			if !rankSet[r] {
				return false
			}
		}
	}

	return true
}

// isSequence checks if hand contains a sequence of given length
func isSequence(hand []Card, length int, wrap bool) bool {
	if len(hand) < length {
		return false
	}

	// Get unique ranks sorted
	rankSet := make(map[uint8]bool)
	for _, c := range hand {
		rankSet[c.Rank] = true
	}

	ranks := make([]int, 0, len(rankSet))
	for r := range rankSet {
		ranks = append(ranks, int(r))
	}
	sort.Ints(ranks)

	// Check for consecutive run
	for i := 0; i <= len(ranks)-length; i++ {
		consecutive := true
		for j := 1; j < length; j++ {
			if ranks[i+j] != ranks[i]+j {
				consecutive = false
				break
			}
		}
		if consecutive {
			return true
		}
	}

	// Check wrap-around (Ace-low: A-2-3-4-5)
	if wrap && len(ranks) >= length {
		// Check if we have Ace (rank 12) and low cards
		hasAce := rankSet[12]
		if hasAce {
			// Check for A-2-3-4-5 (ranks: 12, 0, 1, 2, 3)
			// In our representation: 0=2, 1=3, 2=4, 3=5, ... 12=Ace
			wrapRanks := []uint8{0, 1, 2, 3} // 2,3,4,5
			allPresent := true
			for _, r := range wrapRanks[:length-1] {
				if !rankSet[r] {
					allPresent = false
					break
				}
			}
			if allPresent {
				return true
			}
		}
	}

	return false
}
