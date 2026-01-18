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
