package vying

import (
	"math/rand/v2"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Runner implements the vying / betting skeleton (poker family).
//
// Each deal: every player gets HandSize hidden cards, one rotating player posts
// a big blind (so the first actor always faces a bet), then a single betting
// round runs -- fold / call / raise, raises capped at MaxRaises -- and the best
// poker hand among the non-folded players takes the pot. Chips (state.Scores)
// carry across RoundsPerGame deals (state.Round = deal index, the big blind is
// Round % NumPlayers so positions rotate); the largest stack wins.
//
// State mapping onto sim.GameState: Scores = chip stacks; Hands = hidden hole
// cards; Pot / CurrentBet / Committed / Folded / RaiseCount / ToAct are the
// current betting round's transient state. Validation guarantees stacks always
// cover a call (no all-in / side pots), so the betting round is a clean
// fold/call/raise loop. Termination: MaxRaises bounds each round and
// RoundsPerGame bounds the game.
//
// Metric profile: Meaningful-Decisions reads structurally ~1.0 for vying, and
// that is correct rather than a pathology -- every betting action (fold / call /
// raise / check) is a distinct, materially different choice (it forfeits,
// commits, pressures, or stays for free), so a >= 2-move betting turn is always
// a real decision (the same accepted property trick-taking has, where plays are
// always meaningful). The vying genome params (chips / blind / raise cap /
// deals) cannot force a decisions-gamed degenerate, so there is no skeleton
// pin to break here. The discriminating signal between a good and a bad vying
// game lives in the OTHER metrics: Skill (the VyingScorer's tight-aggressive
// edge over random), Interaction (deltaModeVying scores raises and folds), and
// Game Arc (the chip-lead trajectory across deals).
type Runner struct{}

type Card = sim.Card

func (r *Runner) Setup(g *genome.Genome, rng *rand.Rand) *sim.GameState {
	state := sim.NewGameState(g.Players)
	p := g.Vying
	for i := 0; i < g.Players; i++ {
		state.Scores[i] = p.StartingChips
	}
	state.Committed = make([]int, g.Players)
	state.Folded = make([]bool, g.Players)
	state.Round = 0
	state.MaxRound = p.RoundsPerGame
	state.Phase = sim.PhasePlay
	state.RNG = rng
	r.beginDeal(state, g)
	return state
}

// beginDeal deals fresh hidden hands, resets the betting round, and posts the
// rotating big blind. The big blind seat is Round % NumPlayers; the first actor
// is the player after it, so every deal someone is forced to act and the
// positional burden rotates (no seat dominates -- the seat-participation veto).
func (r *Runner) beginDeal(state *sim.GameState, g *genome.Genome) {
	deck := sim.StandardDeck()
	sim.ShuffleDeck(deck, state.RNG)
	for i := 0; i < state.NumPlayers; i++ {
		hand, rest := sim.DrawN(deck, g.HandSize)
		state.Hands[i] = append(state.Hands[i][:0], hand...)
		deck = rest
	}

	state.Pot = 0
	state.CurrentBet = 0
	state.RaiseCount = 0
	for i := 0; i < state.NumPlayers; i++ {
		state.Committed[i] = 0
		state.Folded[i] = false
	}

	bb := state.Round % state.NumPlayers
	post := g.Vying.MinBet
	if post > state.Scores[bb] {
		post = state.Scores[bb] // validation prevents this, but stay solvent
	}
	state.Scores[bb] -= post
	state.Committed[bb] = post
	state.Pot += post
	state.CurrentBet = post

	state.Active = (bb + 1) % state.NumPlayers
	// Everyone must get an action this round; the big blind acts last and gets
	// the check-or-raise option once the bet is matched around.
	state.ToAct = state.NumPlayers
}

// Upkeep resolves a closed betting round (award the pot at showdown) and starts
// the next deal, until RoundsPerGame deals are done. Mutating; once Round
// reaches MaxRound it is a no-op so the resolved final deal is not double-paid.
func (r *Runner) Upkeep(state *sim.GameState, g *genome.Genome) {
	if state.Round >= state.MaxRound {
		return // game over; CheckEnd reports the chip winner
	}
	if !roundClosed(state) {
		return
	}
	r.resolveShowdown(state)
	state.Round++
	if state.Round < state.MaxRound {
		r.beginDeal(state, g)
	}
}

// roundClosed reports whether the betting round is over: every non-folded player
// has had their action since the last raise (ToAct drained), or only one
// non-folded player remains.
func roundClosed(state *sim.GameState) bool {
	return state.ToAct <= 0 || countNonFolded(state) <= 1
}

func countNonFolded(state *sim.GameState) int {
	n := 0
	for i := 0; i < state.NumPlayers; i++ {
		if !state.Folded[i] {
			n++
		}
	}
	return n
}

// resolveShowdown awards the pot. With one contender it goes uncontested;
// otherwise the best poker hand wins, splitting on an exact tie (remainder to
// the lowest seat for determinism).
func (r *Runner) resolveShowdown(state *sim.GameState) {
	var contenders []int
	for i := 0; i < state.NumPlayers; i++ {
		if !state.Folded[i] {
			contenders = append(contenders, i)
		}
	}
	if len(contenders) == 0 {
		state.Pot = 0
		return
	}
	if len(contenders) == 1 {
		state.Scores[contenders[0]] += state.Pot
		state.Pot = 0
		return
	}
	best := int64(-1)
	var winners []int
	for _, i := range contenders {
		s := HandStrength(state.Hands[i])
		if s > best {
			best = s
			winners = []int{i}
		} else if s == best {
			winners = append(winners, i)
		}
	}
	share := state.Pot / len(winners)
	rem := state.Pot - share*len(winners)
	for idx, w := range winners {
		state.Scores[w] += share
		if idx == 0 {
			state.Scores[w] += rem // lowest-seat winner takes the odd chip
		}
	}
	state.Pot = 0
}

// GenerateMoves returns the active player's legal betting actions. PURE. Never
// empty: facing a bet you can always fold; facing nothing you can always check.
func (r *Runner) GenerateMoves(state *sim.GameState, g *genome.Genome) []sim.Move {
	a := state.Active
	if state.Folded[a] {
		return nil // unreachable in a well-formed loop (we advance to non-folded)
	}
	facing := state.CurrentBet - state.Committed[a]
	canRaise := state.RaiseCount < g.Vying.MaxRaises &&
		state.Scores[a] >= facing+g.Vying.MinBet

	var moves []sim.Move
	if facing == 0 {
		moves = append(moves, sim.Move{Type: sim.MoveCheck, PlayerID: a})
		if canRaise {
			moves = append(moves, sim.Move{Type: sim.MoveRaise, PlayerID: a})
		}
	} else {
		moves = append(moves, sim.Move{Type: sim.MoveFold, PlayerID: a})
		if state.Scores[a] >= facing {
			moves = append(moves, sim.Move{Type: sim.MoveCall, PlayerID: a})
		}
		if canRaise {
			moves = append(moves, sim.Move{Type: sim.MoveRaise, PlayerID: a})
		}
	}
	return moves
}

func (r *Runner) ApplyMove(state *sim.GameState, move sim.Move, g *genome.Genome) []sim.Event {
	a := state.Active
	facing := state.CurrentBet - state.Committed[a]

	switch move.Type {
	case sim.MoveCheck:
		state.ToAct--

	case sim.MoveCall:
		state.Scores[a] -= facing
		state.Committed[a] += facing
		state.Pot += facing
		state.ToAct--

	case sim.MoveRaise:
		// Call the outstanding bet, then raise by MinBet.
		put := facing + g.Vying.MinBet
		state.Scores[a] -= put
		state.Committed[a] += put
		state.Pot += put
		state.CurrentBet += g.Vying.MinBet
		state.RaiseCount++
		// Every other non-folded player now owes a response to the raise.
		state.ToAct = countNonFolded(state) - 1

	case sim.MoveFold:
		state.Folded[a] = true
		state.ToAct--
	}

	state.Turn++
	state.Active = nextNonFolded(state, a)

	// Vying-as-scoring-host (VyingScored): when this move closes the betting
	// round, MUCK the folded hands (empty their card slices -- folded players
	// reveal nothing, standard poker) and emit ONE EventRoundEnd. The
	// MeldBonus / Avoidance hook fires on that event and scores state.Hands[i]
	// for EVERY i; mucking the folded hands is what makes them contribute zero,
	// so the hook stays host-agnostic (it must not read state.Folded, which is
	// nil on the non-vying hosts that share these hooks). The non-folded hands
	// are still populated (the redeal is in the next Upkeep), so only the SHOWN
	// hands are scored into state.Scores -- the chip stacks CheckEnd ranks.
	// Fires exactly once per deal (only on the closing move), so the accumulating
	// hook never double-banks; the next Upkeep then runs resolveShowdown (which
	// ranks the still-intact non-folded hands) and redeals. Entirely gated on
	// VyingScored, so an unscored vying game is byte-identical. NOTE the scored
	// game is deliberately NOT a closed chip economy: a meld bonus creates chips
	// and an avoidance penalty destroys them, both bounded (teeth-fixed
	// CardPoints, << StartingChips), and CheckEnd/Progress both tolerate a
	// player going negative.
	if g.VyingScored() && roundClosed(state) {
		for i := 0; i < state.NumPlayers; i++ {
			if state.Folded[i] {
				state.Hands[i] = state.Hands[i][:0]
			}
		}
		return []sim.Event{{Type: sim.EventRoundEnd, PlayerID: a, Detail: "showdown"}}
	}
	return nil
}

// nextNonFolded returns the next non-folded seat after `from`, or `from` if no
// other player is still in (the round is about to close in Upkeep).
func nextNonFolded(state *sim.GameState, from int) int {
	for step := 1; step <= state.NumPlayers; step++ {
		p := (from + step) % state.NumPlayers
		if !state.Folded[p] {
			return p
		}
	}
	return from
}

// CheckEnd returns the chip leader once all deals are played, else -1. PURE.
// Ties break to the lowest seat.
func (r *Runner) CheckEnd(state *sim.GameState, g *genome.Genome) int {
	if state.Round < state.MaxRound {
		return -1
	}
	winner := 0
	for i := 1; i < state.NumPlayers; i++ {
		if state.Scores[i] > state.Scores[winner] {
			winner = i
		}
	}
	return winner
}

// Progress is each player's share of all chips in [0,1]; argmax (most chips) is
// the winner rule, so the eventual winner's final Progress is the maximum. Chips
// committed to the current pot are momentarily excluded (they return to a stack
// at showdown), the same one-resolution-late skew the other banked-score
// skeletons carry. Pure and allocation-light.
func (r *Runner) Progress(state *sim.GameState, g *genome.Genome) []float64 {
	out := make([]float64, state.NumPlayers)
	total := 0
	for _, s := range state.Scores {
		total += s
	}
	if total <= 0 {
		return out
	}
	for i, s := range state.Scores {
		out[i] = float64(s) / float64(total)
	}
	return out
}
