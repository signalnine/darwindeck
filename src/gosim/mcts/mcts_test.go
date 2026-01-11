package mcts

import (
	"testing"

	"github.com/signalnine/darwindeck/gosim/engine"
)

func TestNodePool(t *testing.T) {
	// Acquire and release
	n1 := GetNode()
	if cap(n1.Children) == 0 {
		t.Error("Expected pre-allocated children slice")
	}

	PutNode(n1)

	// Should get same instance back
	n2 := GetNode()
	if &n1.Children != &n2.Children {
		t.Error("Pool did not reuse memory")
	}

	PutNode(n2)
}

func TestNodeReset(t *testing.T) {
	node := GetNode()
	node.Visits = 100
	node.Wins = 50.0
	node.PlayerID = 1

	node.Reset()

	if node.Visits != 0 || node.Wins != 0 || node.PlayerID != 0 {
		t.Error("Reset did not clear node state")
	}

	PutNode(node)
}

func TestUCB1Calculation(t *testing.T) {
	parent := GetNode()
	parent.Visits = 100

	child := GetNode()
	child.Parent = parent
	child.Visits = 10
	child.Wins = 7.0

	ucb := child.UCB1(1.414)

	// UCB1 = exploitation + exploration
	// exploitation = 7/10 = 0.7
	// exploration = 1.414 * sqrt(ln(100)/10) ≈ 1.414 * sqrt(0.46) ≈ 0.96
	// total ≈ 1.66

	if ucb < 1.5 || ucb > 1.8 {
		t.Errorf("UCB1 value out of expected range: %f", ucb)
	}

	PutNode(parent)
	PutNode(child)
}

func TestBestChild(t *testing.T) {
	parent := GetNode()
	parent.Visits = 100

	child1 := GetNode()
	child1.Parent = parent
	child1.Visits = 40
	child1.Wins = 20.0 // Win rate: 0.50

	child2 := GetNode()
	child2.Parent = parent
	child2.Visits = 50
	child2.Wins = 40.0 // Win rate: 0.80 (much higher)

	parent.Children = append(parent.Children, child1, child2)

	best := parent.BestChild(1.414)

	// child2 should have higher UCB1 due to significantly better win rate
	// child1 UCB1 ≈ 0.50 + 1.414*sqrt(ln(100)/40) ≈ 0.50 + 0.68 ≈ 1.18
	// child2 UCB1 ≈ 0.80 + 1.414*sqrt(ln(100)/50) ≈ 0.80 + 0.60 ≈ 1.40
	if best != child2 {
		t.Error("BestChild did not select highest UCB1 child")
	}

	PutNode(parent) // Recursively returns children
}

func TestMostVisitedChild(t *testing.T) {
	parent := GetNode()

	child1 := GetNode()
	child1.Visits = 10

	child2 := GetNode()
	child2.Visits = 25 // Most visited

	child3 := GetNode()
	child3.Visits = 15

	parent.Children = append(parent.Children, child1, child2, child3)

	most := parent.MostVisitedChild()

	if most != child2 {
		t.Error("MostVisitedChild did not select child with most visits")
	}

	PutNode(parent)
}

func TestIsFullyExpanded(t *testing.T) {
	node := GetNode()
	node.UntriedMoves = []engine.LegalMove{
		{PhaseIndex: 0, CardIndex: 0},
	}

	if node.IsFullyExpanded() {
		t.Error("Node should not be fully expanded with untried moves")
	}

	node.UntriedMoves = node.UntriedMoves[:0]

	if !node.IsFullyExpanded() {
		t.Error("Node should be fully expanded with no untried moves")
	}

	PutNode(node)
}

func TestIsTerminal(t *testing.T) {
	node := GetNode()
	node.State = engine.GetState()
	node.State.WinnerID = -1

	if node.IsTerminal() {
		t.Error("Node should not be terminal with winner -1")
	}

	node.State.WinnerID = 0

	if !node.IsTerminal() {
		t.Error("Node should be terminal with winner 0")
	}

	engine.PutState(node.State)
	PutNode(node)
}

func TestMCTSSearch(t *testing.T) {
	// Create a simple War genome for testing
	state := engine.GetState()
	defer engine.PutState(state)

	// Initialize a simple game state
	state.Deck = append(state.Deck,
		engine.Card{Rank: 5, Suit: 0},
		engine.Card{Rank: 3, Suit: 1},
		engine.Card{Rank: 8, Suit: 2},
	)
	state.CurrentPlayer = 0
	state.WinnerID = -1

	// Create minimal genome
	genome := &engine.Genome{
		Header: &engine.BytecodeHeader{
			PlayerCount: 2,
			MaxTurns:    100,
		},
		TurnPhases: []engine.PhaseDescriptor{
			{
				PhaseType: 1, // Draw phase
				Data: []byte{
					0,          // source: deck
					0, 0, 0, 1, // count: 1
					1, // mandatory: true
					0, // has_condition: false
				},
			},
		},
		WinConditions: []engine.WinCondition{
			{
				WinType:   0, // empty_hand
				Threshold: 0,
			},
		},
	}

	// Run MCTS search
	move := Search(state, genome, 100, 1.414)

	if move == nil {
		t.Error("MCTS returned nil move")
	}

	if move.PhaseIndex != 0 {
		t.Errorf("Expected move for phase 0, got %d", move.PhaseIndex)
	}
}

func BenchmarkMCTSSearch(b *testing.B) {
	state := engine.GetState()
	defer engine.PutState(state)

	state.Deck = make([]engine.Card, 52)
	for i := 0; i < 52; i++ {
		state.Deck[i] = engine.Card{Rank: uint8(i % 13), Suit: uint8(i / 13)}
	}
	state.CurrentPlayer = 0
	state.WinnerID = -1

	genome := &engine.Genome{
		Header: &engine.BytecodeHeader{
			PlayerCount: 2,
			MaxTurns:    100,
		},
		TurnPhases: []engine.PhaseDescriptor{
			{
				PhaseType: 1,
				Data: []byte{
					0,          // source
					0, 0, 0, 1, // count
					1, // mandatory
					0, // has_condition
				},
			},
		},
		WinConditions: []engine.WinCondition{
			{WinType: 0, Threshold: 0},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Search(state, genome, 100, 1.414)
	}
}
