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

// PerPlayerAI dispatches move selection to a distinct AIPlayer for each
// player index. Used for skill-gradient evaluation where player 0 is greedy
// and the rest are random. Out-of-range indices fall back to Fallback.
type PerPlayerAI struct {
	Players  []AIPlayer
	Fallback AIPlayer
}

func (ai *PerPlayerAI) SelectMove(moves []Move, state *GameState, rng *rand.Rand) Move {
	idx := state.Active
	if idx >= 0 && idx < len(ai.Players) && ai.Players[idx] != nil {
		return ai.Players[idx].SelectMove(moves, state, rng)
	}
	if ai.Fallback != nil {
		return ai.Fallback.SelectMove(moves, state, rng)
	}
	if len(moves) == 0 {
		return Move{Type: MovePass}
	}
	return moves[rng.IntN(len(moves))]
}
