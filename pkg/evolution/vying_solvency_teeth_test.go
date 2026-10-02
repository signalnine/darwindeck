package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestGiveBorrowTeethRepairsVyingAvoidanceSolvency: validateVying now counts the
// avoidance borrow's worst-case showdown penalty in the stack-sufficiency bound
// (genome.VyingWorstCaseCommitment; 2026-10 bughunt, vying insolvency). The
// operators must stay valid-in/valid-out, so the shared teeth wiring -- run
// after every mutation and every crossover -- tops the stack up to the bound.
func TestGiveBorrowTeethRepairsVyingAvoidanceSolvency(t *testing.T) {
	g := seeds.SimplePoker()
	bm := genome.BorrowedMechanic{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance}
	g.Borrowed = []genome.BorrowedMechanic{bm}
	g.Scoring.CardPoints = []genome.CardScoring{{Suit: 3, Points: 20}}
	g.Vying.RoundsPerGame = 12
	// Covers the betting commitment only -- the pre-fix bound.
	g.Vying.StartingChips = g.Vying.RoundsPerGame * g.Vying.MinBet * (g.Vying.MaxRaises + 1)
	if errs := genome.Validate(g); len(errs) == 0 {
		t.Fatal("fixture must violate the penalty-aware solvency bound before repair")
	}

	giveBorrowTeeth(g, bm)

	if errs := genome.Validate(g); len(errs) != 0 {
		t.Fatalf("giveBorrowTeeth left a vying+avoidance genome invalid: %v", errs)
	}
	if got, want := g.Vying.StartingChips, g.VyingWorstCaseCommitment(); got < want {
		t.Errorf("StartingChips = %d, want >= worst case %d", got, want)
	}
}

// TestMutationKeepsVyingAvoidanceSolvent is the property form: thousands of
// cross-skeleton mutations of a vying+avoidance genome (scoring tweaks, betting
// tweaks, hand-size tweaks) never emit a child that fails Tier 0.
func TestMutationKeepsVyingAvoidanceSolvent(t *testing.T) {
	bm := genome.BorrowedMechanic{Source: genome.TrickTaking, Mechanic: genome.MechAvoidance}
	parent := seeds.SimplePoker()
	parent.Borrowed = []genome.BorrowedMechanic{bm}
	parent.Scoring.CardPoints = []genome.CardScoring{{Suit: 3, Points: 20}}
	giveBorrowTeeth(parent, bm)
	if errs := genome.Validate(parent); len(errs) != 0 {
		t.Fatalf("parent invalid: %v", errs)
	}
	// Vying-only seed pool so changeSkeleton keeps the lineage on the host
	// under test.
	pool := []*genome.Genome{parent}
	rng := rand.New(rand.NewPCG(20261001, 7))
	g := parent
	for i := 0; i < 4000; i++ {
		child := MutateWith(g, rng, pool, true)
		if errs := genome.Validate(child); len(errs) != 0 {
			t.Fatalf("mutation %d emitted an invalid child: %v\n%+v vying=%+v", i, errs, child, child.Vying)
		}
		g = child
		if i%50 == 49 {
			// Cross with the parent too: the betting/scoring coin flips are
			// independent, so crossover needs the same repair.
			c := Crossover(g, parent, rng)
			if errs := genome.Validate(c); len(errs) != 0 {
				t.Fatalf("crossover %d emitted an invalid child: %v", i, errs)
			}
		}
	}
}
