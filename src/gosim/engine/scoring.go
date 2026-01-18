package engine

// EvaluateContracts scores all teams based on their bids and tricks won.
func EvaluateContracts(state *GameState, scoring *ContractScoring) {
	numTeams := len(state.TeamScores)
	if numTeams == 0 {
		return
	}

	for teamIdx := 0; teamIdx < numTeams; teamIdx++ {
		// Sum tricks won by team members
		tricksWon := int32(0)
		teamPlayers := getTeamPlayers(state, teamIdx)

		// Score Nil bids first
		for _, playerIdx := range teamPlayers {
			player := &state.Players[playerIdx]
			if player.IsNilBid {
				if player.TricksWon == 0 {
					state.TeamScores[teamIdx] += int32(scoring.NilBonus)
				} else {
					state.TeamScores[teamIdx] -= int32(scoring.NilPenalty)
				}
			}
			tricksWon += int32(player.TricksWon)
		}

		// Score team contract (non-Nil bids)
		contract := int32(state.TeamContracts[teamIdx])

		if tricksWon >= contract {
			// Made contract
			state.TeamScores[teamIdx] += contract * int32(scoring.PointsPerTrickBid)
			overtricks := int(tricksWon - contract)
			state.TeamScores[teamIdx] += int32(overtricks * scoring.OvertrickPoints)

			// Accumulate bags
			state.AccumulatedBags[teamIdx] += int8(overtricks)
			if state.AccumulatedBags[teamIdx] >= int8(scoring.BagLimit) {
				state.TeamScores[teamIdx] -= int32(scoring.BagPenalty)
				state.AccumulatedBags[teamIdx] -= int8(scoring.BagLimit)
			}
		} else {
			// Failed contract
			state.TeamScores[teamIdx] -= contract * int32(scoring.FailedContractPenalty)
		}
	}
}

// getTeamPlayers returns player indices for a team.
func getTeamPlayers(state *GameState, teamIdx int) []int {
	players := []int{}
	for i, team := range state.PlayerToTeam {
		if int(team) == teamIdx {
			players = append(players, i)
		}
	}
	return players
}

// ResetHandState clears per-hand state for the next hand.
// AccumulatedBags and TeamScores persist across hands.
func ResetHandState(state *GameState) {
	for i := range state.Players {
		state.Players[i].CurrentBid = -1
		state.Players[i].IsNilBid = false
		state.Players[i].TricksWon = 0
	}
	state.BiddingComplete = false

	// Reset team contracts but keep scores and bags
	for i := range state.TeamContracts {
		state.TeamContracts[i] = 0
	}
}
