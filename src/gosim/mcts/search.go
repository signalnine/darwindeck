package mcts

import (
	"math/rand"

	"github.com/signalnine/cards-evolve/gosim/engine"
)

const (
	DefaultExplorationParam = 1.414 // sqrt(2)
)

// Search performs MCTS from the given state and returns the best move
func Search(state *engine.GameState, genome *engine.Genome, iterations int, explorationParam float64) *engine.LegalMove {
	if explorationParam == 0 {
		explorationParam = DefaultExplorationParam
	}

	// Create root node
	root := GetNode()
	defer PutNode(root)

	root.State = state.Clone()
	root.PlayerID = state.CurrentPlayer
	root.UntriedMoves = engine.GenerateLegalMoves(root.State, genome)

	// Run MCTS iterations
	for i := 0; i < iterations; i++ {
		node := root

		// 1. Selection - traverse tree using UCB1
		for !node.IsTerminal() && node.IsFullyExpanded() {
			node = node.BestChild(explorationParam)
			if node == nil {
				break
			}
		}

		// If selection resulted in nil node, skip this iteration
		if node == nil {
			continue
		}

		// 2. Expansion - add a new child node
		if !node.IsTerminal() && len(node.UntriedMoves) > 0 {
			node = expand(node, genome)
		}

		// 3. Simulation - play out randomly to terminal state
		winner := simulate(node.State, genome)

		// 4. Backpropagation - update statistics
		backpropagate(node, winner)
	}

	// Return most visited child's move
	bestChild := root.MostVisitedChild()
	if bestChild == nil || bestChild.Move == nil {
		// Fallback to first legal move if MCTS fails
		moves := engine.GenerateLegalMoves(state, genome)
		if len(moves) > 0 {
			return &moves[0]
		}
		return nil
	}

	// Create a copy of the move to return
	moveCopy := *bestChild.Move
	return &moveCopy
}

// expand adds a new child node for an untried move
func expand(node *MCTSNode, genome *engine.Genome) *MCTSNode {
	// Pick a random untried move
	moveIndex := rand.Intn(len(node.UntriedMoves))
	move := node.UntriedMoves[moveIndex]

	// Remove from untried moves
	node.UntriedMoves[moveIndex] = node.UntriedMoves[len(node.UntriedMoves)-1]
	node.UntriedMoves = node.UntriedMoves[:len(node.UntriedMoves)-1]

	// Create child state
	childState := node.State.Clone()
	engine.ApplyMove(childState, &move, genome)

	// Create child node
	child := GetNode()
	child.State = childState
	child.Move = &move
	child.Parent = node
	child.PlayerID = childState.CurrentPlayer
	child.UntriedMoves = engine.GenerateLegalMoves(childState, genome)

	node.Children = append(node.Children, child)

	return child
}

// simulate plays out the game randomly from the current state
func simulate(state *engine.GameState, genome *engine.Genome) int8 {
	simState := state.Clone()
	defer engine.PutState(simState)

	maxSimulationTurns := int(genome.Header.MaxTurns) * 2 // Safety limit

	for i := 0; i < maxSimulationTurns; i++ {
		// Check win conditions
		winner := engine.CheckWinConditions(simState, genome)
		if winner >= 0 {
			return winner
		}

		// Generate legal moves
		moves := engine.GenerateLegalMoves(simState, genome)
		if len(moves) == 0 {
			// No legal moves - game is stuck
			return -1
		}

		// Pick a random move
		move := moves[rand.Intn(len(moves))]
		engine.ApplyMove(simState, &move, genome)
	}

	// Timeout - return draw
	return -1
}

// backpropagate updates node statistics up the tree
func backpropagate(node *MCTSNode, winner int8) {
	for node != nil {
		node.Visits++

		// Award wins based on perspective
		if winner >= 0 {
			if uint8(winner) == node.PlayerID {
				node.Wins += 1.0
			}
			// Could add partial credit for draws:
			// else if winner == -1 {
			//     node.Wins += 0.5
			// }
		}

		node = node.Parent
	}
}

// SearchWithVariant allows specifying different MCTS variants
type SearchParams struct {
	Iterations       int
	ExplorationParam float64
	// Future extensions:
	// UseRAVE         bool
	// UseProgWiden    bool
	// ParallelWorkers int
}

// SearchWithParams runs MCTS with custom parameters
func SearchWithParams(state *engine.GameState, genome *engine.Genome, params SearchParams) *engine.LegalMove {
	return Search(state, genome, params.Iterations, params.ExplorationParam)
}
