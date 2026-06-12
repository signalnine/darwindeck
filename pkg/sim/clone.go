package sim

import "math/rand/v2"

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
	cp.Scores = cloneInts(gs.Scores)

	if gs.TopCard != nil {
		c := *gs.TopCard
		cp.TopCard = &c
	}

	cp.TrickCards = cloneCards(gs.TrickCards)
	cp.TrickPlayers = cloneInts(gs.TrickPlayers)

	cp.Melds = cloneCardMatrix(gs.Melds)
	cp.MeldOwner = cloneInts(gs.MeldOwner)

	cp.Events = nil
	cp.RNG = nil
	return &cp
}

// Determinize returns a clone of gs in which all information hidden from
// player p is resampled (audit Task 19 step 2). The hidden pool is the
// face-down deck plus every OTHER player's hand; it is shuffled with rng and
// redealt back into those zones preserving each zone's exact size. Everything
// player p can know stays byte-identical: p's own hand, the discard pile, all
// tableaus, table melds, the current trick, TopCard, Scores, and every scalar
// field.
//
// This is the information-set boundary for ISMCTS. v1's MCTS cloned hidden
// hands verbatim (omniscient search) -- the audit explicitly forbids copying
// that. Note the model is deliberately simple: cards an opponent visibly took
// from the discard pile are still treated as hidden, an acceptable looseness
// the determinization tests pin (zone identity/sizes/conservation), not a
// leak in the unsafe direction.
//
// Preconditions: 0 <= p < gs.NumPlayers. Like Clone, the returned state has a
// nil RNG; simulation callers must assign their own source.
func Determinize(gs *GameState, p int, rng *rand.Rand) *GameState {
	cp := gs.Clone()

	poolSize := len(cp.Deck)
	for i := range cp.Hands {
		if i != p {
			poolSize += len(cp.Hands[i])
		}
	}
	pool := make([]Card, 0, poolSize)
	pool = append(pool, cp.Deck...)
	for i := range cp.Hands {
		if i != p {
			pool = append(pool, cp.Hands[i]...)
		}
	}

	ShuffleDeck(pool, rng)

	// Redeal in zone order, preserving each zone's size. The clone's slices
	// already have the right lengths and are not aliased to the original, so
	// copying over them in place is safe.
	idx := copy(cp.Deck, pool)
	for i := range cp.Hands {
		if i != p {
			idx += copy(cp.Hands[i], pool[idx:])
		}
	}

	return cp
}

// The clone helpers preserve nil-ness exactly: runners reset some zones with
// s[:0] (empty, non-nil) and reflect.DeepEqual distinguishes that from nil,
// so a clone that collapsed empty to nil would fail state-equality checks.
func cloneCards(src []Card) []Card {
	if src == nil {
		return nil
	}
	out := make([]Card, len(src))
	copy(out, src)
	return out
}

func cloneInts(src []int) []int {
	if src == nil {
		return nil
	}
	out := make([]int, len(src))
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
