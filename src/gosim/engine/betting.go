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
