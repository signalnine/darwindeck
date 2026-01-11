package mcts

import (
	"math"
	"sync"

	"github.com/signalnine/cards-evolve/gosim/engine"
)

// MCTSNode represents a node in the Monte Carlo search tree
type MCTSNode struct {
	State        *engine.GameState
	Move         *engine.LegalMove
	Parent       *MCTSNode
	Children     []*MCTSNode
	Visits       int
	Wins         float64
	UntriedMoves []engine.LegalMove
	PlayerID     uint8
}

// NodePool provides memory pooling for MCTS nodes
var NodePool = sync.Pool{
	New: func() interface{} {
		return &MCTSNode{
			Children:     make([]*MCTSNode, 0, 10),
			UntriedMoves: make([]engine.LegalMove, 0, 20),
		}
	},
}

// GetNode acquires a node from the pool
func GetNode() *MCTSNode {
	node := NodePool.Get().(*MCTSNode)
	node.Reset()
	return node
}

// PutNode returns a node to the pool
func PutNode(node *MCTSNode) {
	if node == nil {
		return
	}
	// Return children to pool recursively
	for _, child := range node.Children {
		PutNode(child)
	}
	NodePool.Put(node)
}

// Reset clears a node for reuse
func (n *MCTSNode) Reset() {
	n.State = nil
	n.Move = nil
	n.Parent = nil
	n.Children = n.Children[:0]
	n.Visits = 0
	n.Wins = 0
	n.UntriedMoves = n.UntriedMoves[:0]
	n.PlayerID = 0
}

// UCB1 calculates the Upper Confidence Bound for Trees value
func (n *MCTSNode) UCB1(explorationParam float64) float64 {
	if n.Visits == 0 {
		return math.Inf(1)
	}

	exploitation := n.Wins / float64(n.Visits)
	exploration := explorationParam * math.Sqrt(math.Log(float64(n.Parent.Visits))/float64(n.Visits))

	return exploitation + exploration
}

// BestChild returns the child with the highest UCB1 value
func (n *MCTSNode) BestChild(explorationParam float64) *MCTSNode {
	if len(n.Children) == 0 {
		return nil
	}

	bestChild := n.Children[0]
	bestValue := bestChild.UCB1(explorationParam)

	for _, child := range n.Children[1:] {
		value := child.UCB1(explorationParam)
		if value > bestValue {
			bestValue = value
			bestChild = child
		}
	}

	return bestChild
}

// MostVisitedChild returns the child with the most visits
func (n *MCTSNode) MostVisitedChild() *MCTSNode {
	if len(n.Children) == 0 {
		return nil
	}

	bestChild := n.Children[0]
	maxVisits := bestChild.Visits

	for _, child := range n.Children[1:] {
		if child.Visits > maxVisits {
			maxVisits = child.Visits
			bestChild = child
		}
	}

	return bestChild
}

// IsFullyExpanded checks if all possible moves have been tried
func (n *MCTSNode) IsFullyExpanded() bool {
	return len(n.UntriedMoves) == 0
}

// IsTerminal checks if this node represents a terminal game state
func (n *MCTSNode) IsTerminal() bool {
	if n.State == nil {
		return true // Treat nil state as terminal to prevent crashes
	}
	return n.State.WinnerID >= 0
}
