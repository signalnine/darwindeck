package engine

import "testing"

func TestNewTensionMetrics(t *testing.T) {
	tm := NewTensionMetrics(4)

	if tm.currentLeader != -1 {
		t.Errorf("expected currentLeader=-1, got %d", tm.currentLeader)
	}
	if tm.ClosestMargin != 1.0 {
		t.Errorf("expected ClosestMargin=1.0, got %f", tm.ClosestMargin)
	}
	if len(tm.leaderHistory) != 0 {
		t.Errorf("expected empty leaderHistory, got len=%d", len(tm.leaderHistory))
	}
	if cap(tm.leaderHistory) < 100 {
		t.Errorf("expected leaderHistory capacity >= 100, got %d", cap(tm.leaderHistory))
	}
}

func TestScoreLeaderDetector_GetLeader(t *testing.T) {
	detector := &ScoreLeaderDetector{}

	state := &GameState{
		NumPlayers: 3,
		Players: []PlayerState{
			{Score: 10},
			{Score: 25},
			{Score: 15},
		},
	}

	leader := detector.GetLeader(state)
	if leader != 1 {
		t.Errorf("expected leader=1 (highest score), got %d", leader)
	}
}

func TestScoreLeaderDetector_Tie(t *testing.T) {
	detector := &ScoreLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Score: 20},
			{Score: 20},
		},
	}

	leader := detector.GetLeader(state)
	if leader != -1 {
		t.Errorf("expected leader=-1 (tie), got %d", leader)
	}
}

func TestScoreLeaderDetector_GetMargin(t *testing.T) {
	detector := &ScoreLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Score: 100},
			{Score: 75},
		},
	}

	margin := detector.GetMargin(state)
	// (100-75)/100 = 0.25
	if margin < 0.24 || margin > 0.26 {
		t.Errorf("expected margin=0.25, got %f", margin)
	}
}

func TestHandSizeLeaderDetector_GetLeader(t *testing.T) {
	detector := &HandSizeLeaderDetector{}

	state := &GameState{
		NumPlayers: 3,
		Players: []PlayerState{
			{Hand: make([]Card, 5)},
			{Hand: make([]Card, 2)}, // Fewest cards = leader
			{Hand: make([]Card, 7)},
		},
	}

	leader := detector.GetLeader(state)
	if leader != 1 {
		t.Errorf("expected leader=1 (fewest cards), got %d", leader)
	}
}

func TestHandSizeLeaderDetector_Tie(t *testing.T) {
	detector := &HandSizeLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Hand: make([]Card, 3)},
			{Hand: make([]Card, 3)},
		},
	}

	leader := detector.GetLeader(state)
	if leader != -1 {
		t.Errorf("expected leader=-1 (tie), got %d", leader)
	}
}

func TestHandSizeLeaderDetector_GetMargin(t *testing.T) {
	detector := &HandSizeLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Hand: make([]Card, 2)},
			{Hand: make([]Card, 8)},
		},
	}

	margin := detector.GetMargin(state)
	// (8-2)/8 = 0.75
	if margin < 0.74 || margin > 0.76 {
		t.Errorf("expected margin=0.75, got %f", margin)
	}
}

func TestTrickLeaderDetector_GetLeader(t *testing.T) {
	detector := &TrickLeaderDetector{}

	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{}, {}, {}, {},
		},
		TricksWon: []uint8{3, 5, 2, 3}, // Player 1 has most tricks = leader
	}

	leader := detector.GetLeader(state)
	if leader != 1 {
		t.Errorf("expected leader=1 (most tricks), got %d", leader)
	}
}

func TestTrickLeaderDetector_Tie(t *testing.T) {
	detector := &TrickLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{}, {},
		},
		TricksWon: []uint8{5, 5}, // Tied
	}

	leader := detector.GetLeader(state)
	if leader != -1 {
		t.Errorf("expected leader=-1 (tie), got %d", leader)
	}
}

func TestTrickLeaderDetector_GetMargin(t *testing.T) {
	detector := &TrickLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{}, {},
		},
		TricksWon: []uint8{7, 6}, // Total 13 tricks
	}

	margin := detector.GetMargin(state)
	// (7-6)/13 ≈ 0.077
	if margin < 0.07 || margin > 0.08 {
		t.Errorf("expected margin≈0.077, got %f", margin)
	}
}

func TestTrickAvoidanceLeaderDetector_GetLeader(t *testing.T) {
	detector := &TrickAvoidanceLeaderDetector{}

	state := &GameState{
		NumPlayers: 4,
		Players: []PlayerState{
			{}, {}, {}, {},
		},
		TricksWon: []uint8{3, 5, 1, 4}, // Player 2 has fewest tricks = leader in Hearts
	}

	leader := detector.GetLeader(state)
	if leader != 2 {
		t.Errorf("expected leader=2 (fewest tricks), got %d", leader)
	}
}

func TestTrickAvoidanceLeaderDetector_Tie(t *testing.T) {
	detector := &TrickAvoidanceLeaderDetector{}

	state := &GameState{
		NumPlayers: 3,
		Players: []PlayerState{
			{}, {}, {},
		},
		TricksWon: []uint8{2, 5, 2}, // Players 0 and 2 tied for fewest
	}

	leader := detector.GetLeader(state)
	if leader != -1 {
		t.Errorf("expected leader=-1 (tie), got %d", leader)
	}
}

func TestTrickAvoidanceLeaderDetector_GetMargin(t *testing.T) {
	detector := &TrickAvoidanceLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{}, {},
		},
		TricksWon: []uint8{3, 10}, // Total 13 tricks, player 0 leads (fewer is better)
	}

	margin := detector.GetMargin(state)
	// (10-3)/13 ≈ 0.538
	if margin < 0.53 || margin > 0.55 {
		t.Errorf("expected margin≈0.538, got %f", margin)
	}
}

func TestChipLeaderDetector_GetLeader(t *testing.T) {
	detector := &ChipLeaderDetector{}

	state := &GameState{
		NumPlayers: 3,
		Players: []PlayerState{
			{Chips: 500},
			{Chips: 1200}, // Most chips = leader
			{Chips: 300},
		},
	}

	leader := detector.GetLeader(state)
	if leader != 1 {
		t.Errorf("expected leader=1 (most chips), got %d", leader)
	}
}

func TestChipLeaderDetector_Tie(t *testing.T) {
	detector := &ChipLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Chips: 1000},
			{Chips: 1000},
		},
	}

	leader := detector.GetLeader(state)
	if leader != -1 {
		t.Errorf("expected leader=-1 (tie), got %d", leader)
	}
}

func TestChipLeaderDetector_GetMargin(t *testing.T) {
	detector := &ChipLeaderDetector{}

	state := &GameState{
		NumPlayers: 2,
		Players: []PlayerState{
			{Chips: 1500},
			{Chips: 500},
		},
	}

	margin := detector.GetMargin(state)
	// (1500-500)/2000 = 0.5
	if margin < 0.49 || margin > 0.51 {
		t.Errorf("expected margin=0.5, got %f", margin)
	}
}
