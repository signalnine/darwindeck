package grammar

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// turnCap backstops a non-terminating composition (the v1 failure mode).
const turnCap = 2000

// Result of one random-AI game.
type Result struct {
	Terminated   bool // ended before the cap (by the spec's end OR the stalemate rule)
	ViaStalemate bool // ended only because progress plateaued (the spec's own end never fired)
	Stuck        bool // LegalMoves returned empty -- a playable-by-construction VIOLATION
	HitCap       bool // ran to turnCap without ending -- non-termination
	Turns        int
	Decisions    int // turns offering >1 legal move (agency)
}

// progressSig fingerprints the card distribution: the deck count, each hand
// count (with the seat's fold bit), and the shared/discard count are mixed as
// SEPARATE positional terms, so any real step (a card moves between zones, a
// player folds) changes the signature. A plain sum is insensitive to a draw
// (deck -> hand leaves the aggregate unchanged), which made a 12-draw streak
// trip the stalemate net on normally-progressing games. Only true no-progress
// loops (everyone passing forever) leave the signature flat; card motion is
// finite, so a flat signature under the stalemate rule still guarantees
// termination (the turn cap backstops the rest).
func progressSig(gs *sim.GameState) int {
	sig := len(gs.Deck)
	for p := 0; p < gs.NumPlayers; p++ {
		h := 2 * len(gs.Hands[p])
		if p < len(gs.Folded) && !gs.Folded[p] {
			h++
		}
		sig = sig*31 + h
	}
	return sig*31 + len(gs.Discard)
}

// PlayRandom runs one game of the spec under uniform-random play. Termination is
// guaranteed two ways: the spec's own CheckEnd, or a universal stalemate rule
// (no progress for staleLimit turns) that ends the game by the score rule. The
// stalemate rule is the grammar-level termination guarantee that pairs with the
// never-empty move set to make EVERY composition playable-by-construction.
func (rr Runner) PlayRandom(seed uint64) Result {
	rng := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	gs := rr.Setup(rng)
	ai := &sim.RandomAI{}
	res := Result{}
	staleLimit := 3*gs.NumPlayers + 6
	lastSig, stale := -1, 0
	for iter := 0; iter < turnCap; iter++ {
		rr.Upkeep(gs)
		if _, done := rr.CheckEnd(gs); done {
			res.Terminated = true
			res.Turns = gs.Turn
			return res
		}
		if sig := progressSig(gs); sig == lastSig {
			if stale++; stale >= staleLimit {
				res.Terminated, res.ViaStalemate = true, true
				res.Turns = gs.Turn
				return res
			}
		} else {
			lastSig, stale = sig, 0
		}
		moves := rr.LegalMoves(gs)
		if len(moves) == 0 {
			res.Stuck = true
			res.Turns = gs.Turn
			return res
		}
		if len(moves) > 1 {
			res.Decisions++
		}
		rr.Apply(gs, ai.SelectMove(moves, gs, rng))
	}
	res.HitCap = true
	res.Turns = gs.Turn
	return res
}

// Summary aggregates many random games of one spec.
type Summary struct {
	Spec       GameSpec
	Trials     int
	Terminated int
	Stalemate  int // terminations that fired only via the stalemate rule
	Stuck      int
	HitCap     int
	MeanTurns  float64
	AgencyFrac float64 // mean fraction of turns with a real choice
}

func (rr Runner) Playability(trials int, seedBase uint64) Summary {
	sm := Summary{Spec: rr.Spec, Trials: trials}
	var totTurns, totDec int
	for t := 0; t < trials; t++ {
		r := rr.PlayRandom(seedBase + uint64(t)*0x100)
		if r.Terminated {
			sm.Terminated++
		}
		if r.ViaStalemate {
			sm.Stalemate++
		}
		if r.Stuck {
			sm.Stuck++
		}
		if r.HitCap {
			sm.HitCap++
		}
		totTurns += r.Turns
		if r.Turns > 0 {
			totDec += r.Decisions
		}
	}
	if trials > 0 {
		sm.MeanTurns = float64(totTurns) / float64(trials)
	}
	if totTurns > 0 {
		sm.AgencyFrac = float64(totDec) / float64(totTurns)
	}
	return sm
}

// NaturalEnd reports whether the spec mostly ends by its OWN end condition (not
// the stalemate fallback). A spec that only ever stalemates is mis-typed: its
// end condition is unreachable by its move dynamics.
func (sm Summary) NaturalEnd() bool { return sm.Stalemate*2 <= sm.Trials }

// Playable: terminates every trial, never gets stuck, and shows agency. This is
// the bar for "stayed in the playable manifold."
func (sm Summary) Playable() bool {
	return sm.Stuck == 0 && sm.HitCap == 0 && sm.Terminated == sm.Trials && sm.AgencyFrac > 0.05
}
