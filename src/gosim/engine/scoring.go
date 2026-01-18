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
