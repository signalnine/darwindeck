package sim

// Clone returns a deep copy of the game state for tree search (audit Task 19).
//
// Every gameplay-relevant field is copied so mutating the clone (via
// ApplyMove/Upkeep) can never disturb the original. Two fields are
// deliberately NOT carried over:
//
//   - Events: the log is observational -- nothing in any runner's
//     GenerateMoves/ApplyMove/Upkeep/CheckEnd reads it -- and MCTS performs
//     ~40k clone+rollout cycles per genome, so copying a growing event log
//     into every rollout would dominate allocation for zero behavioral gain.
//     The clone starts with a nil log; runners may append to it freely.
//
//   - RNG: sharing the *rand.Rand would let rollouts on clones advance the
//     REAL game's randomness stream (shedding deck recycling, tricktaking
//     redeals, and rummy reshuffles all draw from state.RNG), silently
//     coupling search to gameplay. The clone's RNG is nil; all runner RNG
//     uses are nil-guarded, and simulation callers (MCTS) must assign their
//     own source before stepping the clone.
//
// Maintenance: TestGameStateFieldCountPinsClone pins GameState's field count;
// adding a field forces an update here (and in Determinize, if hidden).
func (gs *GameState) Clone() *GameState {
	cp := *gs // scalars: Turn, Active, Phase, NumPlayers, Direction, Round,
	// MaxRound, TrickLeader, TrumpSuit, TrickBroken

	cp.Deck = cloneCards(gs.Deck)
	cp.Hands = cloneCardMatrix(gs.Hands)
	cp.Discard = cloneCards(gs.Discard)
	cp.Tableau = cloneCardMatrix(gs.Tableau)
	if gs.Scores != nil {
		cp.Scores = append([]int(nil), gs.Scores...)
	}

	if gs.TopCard != nil {
		c := *gs.TopCard
		cp.TopCard = &c
	}

	cp.TrickCards = cloneCards(gs.TrickCards)
	if gs.TrickPlayers != nil {
		cp.TrickPlayers = append([]int(nil), gs.TrickPlayers...)
	}

	cp.Melds = cloneCardMatrix(gs.Melds)
	if gs.MeldOwner != nil {
		cp.MeldOwner = append([]int(nil), gs.MeldOwner...)
	}

	cp.Events = nil
	cp.RNG = nil
	return &cp
}

func cloneCards(src []Card) []Card {
	if src == nil {
		return nil
	}
	out := make([]Card, len(src))
	copy(out, src)
	return out
}

func cloneCardMatrix(src [][]Card) [][]Card {
	if src == nil {
		return nil
	}
	out := make([][]Card, len(src))
	for i := range src {
		out[i] = cloneCards(src[i])
	}
	return out
}
