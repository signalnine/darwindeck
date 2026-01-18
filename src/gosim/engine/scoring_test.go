package engine

import "testing"

func TestEvaluateContractsMade(t *testing.T) {
	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{CurrentBid: 3, IsNilBid: false, TricksWon: 4},
			{CurrentBid: 2, IsNilBid: false, TricksWon: 3},
			{CurrentBid: 4, IsNilBid: false, TricksWon: 4},
			{CurrentBid: 4, IsNilBid: false, TricksWon: 2},
		},
		TeamScores:      []int32{0, 0},
		TeamContracts:   []int8{5, 8}, // Team 0: 3+2=5, Team 1: 4+4=8
		AccumulatedBags: []int8{0, 0},
		PlayerToTeam:    []int8{0, 1, 0, 1},
	}

	scoring := ContractScoring{
		PointsPerTrickBid:     10,
		OvertrickPoints:       1,
		FailedContractPenalty: 10,
		NilBonus:              100,
		NilPenalty:            100,
		BagLimit:              10,
		BagPenalty:            100,
	}

	EvaluateContracts(state, &scoring)

	// Team 0: bid 5, got 8 (4+4), made +50 +3 bags = 53
	if state.TeamScores[0] != 53 {
		t.Errorf("Team 0 expected 53, got %d", state.TeamScores[0])
	}
	if state.AccumulatedBags[0] != 3 {
		t.Errorf("Team 0 expected 3 bags, got %d", state.AccumulatedBags[0])
	}

	// Team 1: bid 8, got 5 (3+2), failed -80
	if state.TeamScores[1] != -80 {
		t.Errorf("Team 1 expected -80, got %d", state.TeamScores[1])
	}
}

func TestEvaluateContractsNilSuccess(t *testing.T) {
	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{CurrentBid: 0, IsNilBid: true, TricksWon: 0}, // Nil success
			{CurrentBid: 5, IsNilBid: false, TricksWon: 6},
			{CurrentBid: 5, IsNilBid: false, TricksWon: 5},
			{CurrentBid: 3, IsNilBid: false, TricksWon: 2},
		},
		TeamScores:      []int32{0, 0},
		TeamContracts:   []int8{5, 8}, // Team 0: 0(Nil)+5=5, Team 1: 5+3=8
		AccumulatedBags: []int8{0, 0},
		PlayerToTeam:    []int8{0, 1, 0, 1},
	}

	scoring := ContractScoring{
		PointsPerTrickBid:     10,
		OvertrickPoints:       1,
		FailedContractPenalty: 10,
		NilBonus:              100,
		NilPenalty:            100,
		BagLimit:              10,
		BagPenalty:            100,
	}

	EvaluateContracts(state, &scoring)

	// Team 0: Nil success +100, contract 5 made with 5 tricks = +50, total = 150
	if state.TeamScores[0] != 150 {
		t.Errorf("Team 0 expected 150, got %d", state.TeamScores[0])
	}
}

func TestEvaluateContractsBagPenalty(t *testing.T) {
	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{CurrentBid: 3, IsNilBid: false, TricksWon: 10},
			{CurrentBid: 3, IsNilBid: false, TricksWon: 3},
		},
		TeamScores:      []int32{0, 0},
		TeamContracts:   []int8{3, 3},
		AccumulatedBags: []int8{5, 0}, // Team 0 already has 5 bags
		PlayerToTeam:    []int8{0, 1},
	}

	scoring := ContractScoring{
		PointsPerTrickBid:     10,
		OvertrickPoints:       1,
		FailedContractPenalty: 10,
		NilBonus:              100,
		NilPenalty:            100,
		BagLimit:              10,
		BagPenalty:            100,
	}

	EvaluateContracts(state, &scoring)

	// Team 0: bid 3, got 10, +30 +7 bags = 37
	// But 5+7=12 bags >= 10 limit, -100 penalty, bags reset to 2
	// Total: 37 - 100 = -63
	if state.TeamScores[0] != -63 {
		t.Errorf("Team 0 expected -63, got %d", state.TeamScores[0])
	}
	if state.AccumulatedBags[0] != 2 {
		t.Errorf("Team 0 expected 2 bags after penalty, got %d", state.AccumulatedBags[0])
	}
}

func TestEvaluateContractsAllNilTeam(t *testing.T) {
	// Edge case: Both players on a team bid Nil
	// Team contract should be 0, and any tricks won become overtricks (bags)
	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{CurrentBid: 0, IsNilBid: true, TricksWon: 0}, // Team 0, Nil success
			{CurrentBid: 5, IsNilBid: false, TricksWon: 6},
			{CurrentBid: 0, IsNilBid: true, TricksWon: 0}, // Team 0, Nil success
			{CurrentBid: 8, IsNilBid: false, TricksWon: 7},
		},
		TeamScores:      []int32{0, 0},
		TeamContracts:   []int8{0, 13}, // Team 0: 0+0=0 (both Nil), Team 1: 5+8=13
		AccumulatedBags: []int8{0, 0},
		PlayerToTeam:    []int8{0, 1, 0, 1},
	}

	scoring := ContractScoring{
		PointsPerTrickBid:     10,
		OvertrickPoints:       1,
		FailedContractPenalty: 10,
		NilBonus:              100,
		NilPenalty:            100,
		BagLimit:              10,
		BagPenalty:            100,
	}

	EvaluateContracts(state, &scoring)

	// Team 0: Both Nil success (+200), contract 0 made with 0 tricks = +0, total = 200
	if state.TeamScores[0] != 200 {
		t.Errorf("Team 0 expected 200 (2x Nil bonus), got %d", state.TeamScores[0])
	}
	if state.AccumulatedBags[0] != 0 {
		t.Errorf("Team 0 expected 0 bags, got %d", state.AccumulatedBags[0])
	}

	// Team 1: contract 13, got 13 (6+7), made +130, 0 bags
	if state.TeamScores[1] != 130 {
		t.Errorf("Team 1 expected 130, got %d", state.TeamScores[1])
	}
}

func TestEvaluateContractsAllNilTeamOneFails(t *testing.T) {
	// Edge case: Both players on team bid Nil, one fails
	// The failed Nil player's tricks become overtricks (bags)
	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{CurrentBid: 0, IsNilBid: true, TricksWon: 0}, // Team 0, Nil success
			{CurrentBid: 5, IsNilBid: false, TricksWon: 5},
			{CurrentBid: 0, IsNilBid: true, TricksWon: 3}, // Team 0, Nil FAIL (won 3 tricks)
			{CurrentBid: 8, IsNilBid: false, TricksWon: 5},
		},
		TeamScores:      []int32{0, 0},
		TeamContracts:   []int8{0, 13}, // Team 0: 0+0=0 (both Nil), Team 1: 5+8=13
		AccumulatedBags: []int8{0, 0},
		PlayerToTeam:    []int8{0, 1, 0, 1},
	}

	scoring := ContractScoring{
		PointsPerTrickBid:     10,
		OvertrickPoints:       1,
		FailedContractPenalty: 10,
		NilBonus:              100,
		NilPenalty:            100,
		BagLimit:              10,
		BagPenalty:            100,
	}

	EvaluateContracts(state, &scoring)

	// Team 0: Nil success +100, Nil fail -100, contract 0 made, 3 overtricks +3
	// Total: 100 - 100 + 0 + 3 = 3
	if state.TeamScores[0] != 3 {
		t.Errorf("Team 0 expected 3, got %d", state.TeamScores[0])
	}
	// The 3 tricks become bags
	if state.AccumulatedBags[0] != 3 {
		t.Errorf("Team 0 expected 3 bags from failed Nil, got %d", state.AccumulatedBags[0])
	}

	// Team 1: contract 13, got 10, failed -130
	if state.TeamScores[1] != -130 {
		t.Errorf("Team 1 expected -130, got %d", state.TeamScores[1])
	}
}

func TestResetHandState(t *testing.T) {
	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{CurrentBid: 3, IsNilBid: false, TricksWon: 4},
			{CurrentBid: 2, IsNilBid: true, TricksWon: 0},
			{CurrentBid: 4, IsNilBid: false, TricksWon: 5},
			{CurrentBid: 4, IsNilBid: false, TricksWon: 4},
		},
		BiddingComplete: true,
		TeamContracts:   []int8{5, 8},
		TeamScores:      []int32{100, 50},
		AccumulatedBags: []int8{3, 7},
	}

	ResetHandState(state)

	// Per-hand state should be reset
	for i, player := range state.Players {
		if player.CurrentBid != -1 {
			t.Errorf("Player %d CurrentBid should be -1, got %d", i, player.CurrentBid)
		}
		if player.IsNilBid {
			t.Errorf("Player %d IsNilBid should be false", i)
		}
		if player.TricksWon != 0 {
			t.Errorf("Player %d TricksWon should be 0, got %d", i, player.TricksWon)
		}
	}

	if state.BiddingComplete {
		t.Errorf("BiddingComplete should be false")
	}

	// TeamScores and AccumulatedBags should persist
	if state.TeamScores[0] != 100 || state.TeamScores[1] != 50 {
		t.Errorf("TeamScores should persist")
	}
	if state.AccumulatedBags[0] != 3 || state.AccumulatedBags[1] != 7 {
		t.Errorf("AccumulatedBags should persist")
	}
}
