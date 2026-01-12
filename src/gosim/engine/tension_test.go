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
