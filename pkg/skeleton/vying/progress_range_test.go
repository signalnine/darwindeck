package vying

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// scoredPokerGenome is a VyingScored game: the avoidance borrow banks a penalty
// into the chip stacks at each showdown, which is exactly what can drive a stack
// negative.
func scoredPokerGenome(t *testing.T) *genome.Genome {
	t.Helper()
	g := seeds.SimplePoker()
	g.Borrowed = []genome.BorrowedMechanic{{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance}}
	g.Scoring.CardPoints = []genome.CardScoring{{Suit: 3, Points: 40}}
	g.Vying.RoundsPerGame = 12
	g.Vying.StartingChips = g.Vying.RoundsPerGame * g.Vying.MinBet * (g.Vying.MaxRaises + 1)
	if errs := genome.Validate(g); len(errs) > 0 {
		t.Fatalf("scored poker genome invalid: %v", errs)
	}
	return g
}

// TestProgressStaysInRangeWithNegativeStacks pins the GenericRunner contract:
// Progress must report [0,1] for every player. A VyingScored avoidance penalty
// can push a stack below zero, and the old share-of-total form then returned a
// negative value.
func TestProgressStaysInRangeWithNegativeStacks(t *testing.T) {
	g := scoredPokerGenome(t)
	r := &Runner{}
	st := r.Setup(g, rand.New(rand.NewPCG(1, 1)))
	st.Scores[0] = -50

	p := r.Progress(st, g)
	for i, x := range p {
		if x < 0 || x > 1 {
			t.Errorf("Progress[%d] = %v with scores %v; must stay in [0,1]", i, x, st.Scores)
		}
	}
	// The negative stack must still rank last.
	for i := 1; i < len(p); i++ {
		if p[0] >= p[i] {
			t.Errorf("negative stack ranks %v vs %v; it must be the lowest", p[0], p[i])
		}
	}
}

// TestProgressInRangeAcrossHookedPlay is the end-to-end form: play scored poker
// with the avoidance hook live and assert no sample ever leaves [0,1].
func TestProgressInRangeAcrossHookedPlay(t *testing.T) {
	g := scoredPokerGenome(t)
	r := &Runner{}
	hooks := mechanic.HooksFor(g)
	samples := 0
	for s := uint64(0); s < 40; s++ {
		rng := rand.New(rand.NewPCG(s+1, 0))
		st := r.Setup(g, rng)
		ai := &sim.RandomAI{}
		for i := 0; i < 3000; i++ {
			r.Upkeep(st, g)
			if r.CheckEnd(st, g) >= 0 {
				break
			}
			mv := ai.SelectMove(r.GenerateMoves(st, g), st, rng)
			for _, e := range r.ApplyMove(st, mv, g) {
				for _, h := range hooks {
					h(st, g, e)
				}
			}
			for pi, x := range r.Progress(st, g) {
				samples++
				if x < 0 || x > 1 {
					t.Fatalf("seed %d: Progress[%d] = %v (scores %v) outside [0,1]", s, pi, x, st.Scores)
				}
			}
		}
	}
	if samples == 0 {
		t.Fatal("no Progress samples taken")
	}
	t.Logf("%d in-range Progress samples across hooked scored-poker play", samples)
}

// TestProgressArgmaxIsChipLeader pins that the normalization change did not move
// the leader track: argmax(Progress) must still be argmax(chips), which is the
// rule CheckEnd uses.
func TestProgressArgmaxIsChipLeader(t *testing.T) {
	g := seeds.SimplePoker()
	r := &Runner{}
	rng := rand.New(rand.NewPCG(4, 4))
	for trial := 0; trial < 500; trial++ {
		st := r.Setup(g, rng)
		for i := range st.Scores {
			st.Scores[i] = rng.IntN(400) - 100 // include negative stacks
		}
		p := r.Progress(st, g)
		bestChips, bestProg := 0, 0
		for i := range st.Scores {
			if st.Scores[i] > st.Scores[bestChips] {
				bestChips = i
			}
			if p[i] > p[bestProg] {
				bestProg = i
			}
		}
		if st.Scores[bestChips] != st.Scores[bestProg] {
			t.Fatalf("trial %d: Progress leader %d (%v) is not the chip leader %d (%v); scores %v",
				trial, bestProg, p[bestProg], bestChips, p[bestChips], st.Scores)
		}
	}
}
