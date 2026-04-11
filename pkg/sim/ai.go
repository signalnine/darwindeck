package sim

import "math/rand/v2"

// AIPlayer selects moves from available options.
type AIPlayer interface {
	SelectMove(moves []Move, state *GameState, rng *rand.Rand) Move
}

// RandomAI picks a uniformly random legal move.
type RandomAI struct{}

func (ai *RandomAI) SelectMove(moves []Move, state *GameState, rng *rand.Rand) Move {
	if len(moves) == 0 {
		return Move{Type: MovePass}
	}
	return moves[rng.IntN(len(moves))]
}
