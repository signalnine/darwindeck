package grammar

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/sim"
)

// This file holds the grammar's whole-space invariant tests (2026-10 bughunt):
// the properties that must hold for EVERY well-typed spec at EVERY state, not
// just one representative per family under a harness.

// allWellTypedSpecs returns every well-typed point of the full domain: each base
// spec (players x deal x target) crossed with every modifier subset up to modCap.
// EnumerateModified keeps one representative per family; this keeps them all.
func allWellTypedSpecs() []GameSpec {
	all := make([]Modifier, 0, modifierCount)
	for m := Modifier(0); m < modifierCount; m++ {
		all = append(all, m)
	}
	subsets := modSubsets(all, modCap)
	seen := map[string]bool{}
	var out []GameSpec
	for _, base := range EnumerateAll() {
		for _, sub := range subsets {
			s := base
			s.Mods = sub
			if !s.WellTyped() {
				continue
			}
			if k := s.String(); !seen[k] {
				seen[k] = true
				out = append(out, s)
			}
		}
	}
	return out
}

// zoneCensus counts every card across every zone a card can live in and reports
// the total plus how many distinct cards appear more than once.
func zoneCensus(gs *sim.GameState) (total, dups int) {
	cnt := map[sim.Card]int{}
	add := func(cs []sim.Card) {
		for _, c := range cs {
			cnt[c]++
		}
	}
	add(gs.Deck)
	add(gs.Discard)
	add(gs.TrickCards)
	for _, h := range gs.Hands {
		add(h)
	}
	for _, t := range gs.Tableau {
		add(t)
	}
	for _, n := range cnt {
		total += n
		if n > 1 {
			dups++
		}
	}
	return total, dups
}

// playRandomGame drives one game through the Adapter exactly as the real engine
// loop does (Upkeep, CheckEnd, GenerateMoves, ApplyMove) under uniform-random
// play, calling visit (when non-nil) on every pre-move state. It returns the
// final state and the winner (-1 if the game ran past the iteration backstop).
func playRandomGame(s GameSpec, seed uint64, visit func(gs *sim.GameState, moves []sim.Move)) (*sim.GameState, int) {
	a := Adapter{s}
	g := SpecGenome(s)
	rng := rand.New(rand.NewPCG(seed, 0x5eed))
	gs := a.Setup(g, rng)
	for i := 0; i < 20000; i++ {
		a.Upkeep(gs, g)
		if w := a.CheckEnd(gs, g); w >= 0 {
			return gs, w
		}
		moves := a.GenerateMoves(gs, g)
		if visit != nil {
			visit(gs, moves)
		}
		if len(moves) == 0 {
			return gs, -1
		}
		a.ApplyMove(gs, moves[rng.IntN(len(moves))], g)
	}
	return gs, -1
}

// TestGrammarFuzzInvariants is the property fuzz over the WHOLE well-typed space
// (every players x deal x target x modifier-subset point, ~680 specs) through the
// Adapter, i.e. the surface the real engine drives. At every state of every game:
//
//   - safety: GenerateMoves is non-empty, every move belongs to the active seat,
//     every card a move plays is in that seat's hand, and EVERY legal move applies
//     to a clone without panicking or duplicating a card;
//   - conservation: the card census over deck/hands/discard/table/piles never
//     changes after the deal and never holds a duplicate;
//   - Progress has one entry per seat, each within [0,1] and never NaN;
//   - liveness: the game ends with a real seat as winner, within the turn cap the
//     real engine enforces (SpecGenome(s).MaxTurns()) -- termination is the
//     runner's property, so this must hold with no harness stalemate net;
//   - determinism: the same seed replays to the same winner, turn count and census.
func TestGrammarFuzzInvariants(t *testing.T) {
	seeds := uint64(10)
	if testing.Short() {
		seeds = 3
	}
	specs := allWellTypedSpecs()
	if len(specs) < 600 {
		t.Fatalf("only %d well-typed specs enumerated; the fuzz is not covering the space", len(specs))
	}
	for _, s := range specs {
		a := Adapter{s}
		g := SpecGenome(s)
		maxTurns := g.MaxTurns()
		for seed := uint64(1); seed <= seeds; seed++ {
			want := -1
			gs, w := playRandomGame(s, seed, func(gs *sim.GameState, moves []sim.Move) {
				total, dups := zoneCensus(gs)
				if want < 0 {
					want = total
				}
				if total != want || dups != 0 {
					t.Fatalf("%s seed %d turn %d: census %d (dups %d), want %d / 0", s, seed, gs.Turn, total, dups, want)
				}
				if len(moves) == 0 {
					t.Fatalf("SAFETY: %s seed %d turn %d: empty move set", s, seed, gs.Turn)
				}
				pr := a.Progress(gs, g)
				if len(pr) != gs.NumPlayers {
					t.Fatalf("%s: Progress has %d entries, want %d", s, len(pr), gs.NumPlayers)
				}
				for p, v := range pr {
					if !(v >= 0 && v <= 1) { // also rejects NaN
						t.Fatalf("%s seed %d turn %d: Progress[%d] = %v, outside [0,1]", s, seed, gs.Turn, p, v)
					}
				}
				inHand := map[sim.Card]bool{}
				for _, c := range gs.Hands[gs.Active] {
					inHand[c] = true
				}
				for _, m := range moves {
					if m.PlayerID != gs.Active {
						t.Fatalf("%s seed %d: move %+v is not the active seat's (%d)", s, seed, m, gs.Active)
					}
					fromHand := m.Cards
					if m.Type == sim.MoveCapture {
						fromHand = m.Cards[:1] // Cards[1:] are the captured TABLE cards
					}
					if s.Move == Accumulate {
						fromHand = nil // banking takes from the table, not a hand
					}
					played := map[sim.Card]bool{}
					for _, c := range fromHand {
						if !inHand[c] || played[c] {
							t.Fatalf("%s seed %d: move %+v plays %v, not (uniquely) in hand %v", s, seed, m, c, gs.Hands[gs.Active])
						}
						played[c] = true
					}
					cl := gs.Clone()
					a.ApplyMove(cl, m, g) // a panic here fails the test with the stack
					if ct, cd := zoneCensus(cl); ct != want || cd != 0 {
						t.Fatalf("%s seed %d: applying %+v leaves census %d (dups %d), want %d / 0", s, seed, m, ct, cd, want)
					}
				}
			})
			if w < 0 || w >= s.Players {
				t.Fatalf("LIVENESS: %s seed %d: no winner (got %d) after %d turns", s, seed, w, gs.Turn)
			}
			if gs.Turn > maxTurns {
				t.Fatalf("LIVENESS: %s seed %d: ended at turn %d, past the engine cap %d", s, seed, gs.Turn, maxTurns)
			}
			gs2, w2 := playRandomGame(s, seed, nil)
			t1, _ := zoneCensus(gs)
			t2, _ := zoneCensus(gs2)
			if w2 != w || gs2.Turn != gs.Turn || t1 != t2 {
				t.Fatalf("DETERMINISM: %s seed %d: replay gave winner %d turn %d, first run %d / %d", s, seed, w2, gs2.Turn, w, gs.Turn)
			}
		}
	}
}

// TestClimbingBeatenCardsGoToDiscard: a beaten combination (and a table cleared
// by an all-pass) used to be overwritten / nil'ed, so cards silently left the
// game from turn 2 on (52 -> 51 -> ...). They now go to the discard pile, so the
// census is constant -- anything that reads the zones (determinization, a future
// "cards seen" feature) sees a whole deck.
func TestClimbingBeatenCardsGoToDiscard(t *testing.T) {
	for _, mods := range [][]Modifier{nil, {ModRunPlay}, {ModRunPlay, ModKnock}} {
		for _, players := range enumPlayers {
			s := GameSpec{Players: players, Deal: 7, Move: BeatOrPass, End: EmptyHand, Score: FirstOut, Mods: mods}
			for seed := uint64(1); seed <= 25; seed++ {
				steps := 0
				gs, w := playRandomGame(s, seed, func(gs *sim.GameState, _ []sim.Move) {
					steps++
					if total, dups := zoneCensus(gs); total != 52 || dups != 0 {
						t.Fatalf("%s seed %d step %d: census = %d cards (%d duplicated), want 52 / 0", s, seed, steps, total, dups)
					}
				})
				if total, _ := zoneCensus(gs); total != 52 || w < 0 {
					t.Fatalf("%s seed %d: final census = %d (winner %d), want 52", s, seed, total, w)
				}
			}
		}
	}
}

// TestBankingTakenCardsKept: a card a banking player takes (face-up or blind)
// used to vanish into the running total. It now sits in the taker's pile, which
// both conserves the deck and gives the tie-break its "fewest cards taken" signal.
func TestBankingTakenCardsKept(t *testing.T) {
	for _, players := range enumPlayers {
		for _, target := range targetsFor(Accumulate) {
			s := GameSpec{Players: players, Shared: 3, Move: Accumulate, Target: target, End: Bust, Score: ClosestTarget}
			for seed := uint64(1); seed <= 25; seed++ {
				gs, w := playRandomGame(s, seed, func(gs *sim.GameState, _ []sim.Move) {
					if total, dups := zoneCensus(gs); total != 52 || dups != 0 {
						t.Fatalf("%s seed %d: census = %d cards (%d duplicated), want 52 / 0", s, seed, total, dups)
					}
				})
				if w < 0 {
					t.Fatalf("%s seed %d: did not terminate", s, seed)
				}
				for p := 0; p < gs.NumPlayers; p++ {
					sum := 0
					for _, c := range gs.Tableau[p] {
						sum += cardValue(c.Rank)
					}
					if sum != gs.Scores[p] {
						t.Fatalf("%s seed %d: seat %d pile %v sums to %d but its total is %d", s, seed, p, gs.Tableau[p], sum, gs.Scores[p])
					}
				}
			}
		}
	}
}
