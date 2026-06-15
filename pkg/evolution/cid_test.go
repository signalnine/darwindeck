package evolution

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// sheddingBase is a plain single-round Crazy-Eights-like shedding genome.
func sheddingBase() *genome.Genome {
	return &genome.Genome{
		ID: "cid-base", Skeleton: genome.Shedding, Players: 2, HandSize: 6,
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
}

func withBorrows(g *genome.Genome, bs ...genome.BorrowedMechanic) *genome.Genome {
	g.Borrowed = bs
	return g
}

func cidOf(t *testing.T, g *genome.Genome) float64 {
	t.Helper()
	hooked, ok := BehaviorBatch(g, 12345)
	if !ok {
		t.Fatalf("no runner for %s", g.ID)
	}
	return CounterfactualIntegration(g, hooked, 12345)
}

// TestCIDDiscriminatesDeepFromShallow is the core property: the deep move+win
// recipe (run_play + multi-round meld-points) has HIGH counterfactual
// integration, while a borrowless genome has ZERO and an inert single-round
// meld-bonus (banks scores nothing reads) is ~0. This is the mechanic-aware
// signal the 2-D behavior descriptor lacks.
func TestCIDDiscriminatesDeepFromShallow(t *testing.T) {
	// borrowless -> exactly 0 (no counterfactual)
	base := sheddingBase()
	if c := cidOf(t, base); c != 0 {
		t.Fatalf("borrowless genome must have CID 0, got %.3f", c)
	}

	// inert single-round meld-bonus: banking borrow with rounds_per_game 1, so
	// nothing reads the banked score and the borrow changes ~nothing -> low CID.
	inert := withBorrows(sheddingBase(),
		genome.BorrowedMechanic{Source: genome.Rummy, Mechanic: genome.MechMeldBonus})
	inert.Shedding.RoundsPerGame = 1
	inertCID := cidOf(t, inert)

	// deep recipe: run_play (move-change) + meld_bonus over 3 rounds (win-condition
	// change). Should move play a lot when removed -> high CID.
	deep := withBorrows(sheddingBase(),
		genome.BorrowedMechanic{Source: genome.Climbing, Mechanic: genome.MechRunPlay},
		genome.BorrowedMechanic{Source: genome.Rummy, Mechanic: genome.MechMeldBonus})
	deep.Shedding.RoundsPerGame = 3
	deepCID := cidOf(t, deep)

	t.Logf("CID: borrowless=0.000  inert-meldbonus=%.3f  deep-recipe=%.3f", inertCID, deepCID)
	if deepCID < 0.10 {
		t.Fatalf("deep recipe should have substantial CID (>=0.10), got %.3f", deepCID)
	}
	if deepCID <= inertCID {
		t.Fatalf("deep recipe CID (%.3f) must exceed the inert bolt-on CID (%.3f) -- the whole point of the signal", deepCID, inertCID)
	}
}

// TestCIDLeaveOneOutIgnoresInertBorrow: carrying an INERT borrow alongside a
// deep one must NOT inflate CID. Leave-one-out takes the max single-borrow
// marginal, so an inert borrow (marginal ~0) is ignored: CID(run_play + inert
// single-round meld_bonus) should be ~ CID(run_play alone). This is the
// pile-on guard a blind-judge test motivated (a dead borrow inflating apparent
// depth).
func TestCIDLeaveOneOutIgnoresInertBorrow(t *testing.T) {
	solo := withBorrows(sheddingBase(),
		genome.BorrowedMechanic{Source: genome.Climbing, Mechanic: genome.MechRunPlay})
	soloCID := cidOf(t, solo)

	withInert := withBorrows(sheddingBase(),
		genome.BorrowedMechanic{Source: genome.Climbing, Mechanic: genome.MechRunPlay},
		genome.BorrowedMechanic{Source: genome.Rummy, Mechanic: genome.MechMeldBonus})
	withInert.Shedding.RoundsPerGame = 1 // meld_bonus single-round = inert (nothing reads the banked score)
	withInertCID := cidOf(t, withInert)

	t.Logf("CID: run_play solo=%.3f  run_play+inert-meldbonus=%.3f", soloCID, withInertCID)
	if soloCID <= 0 {
		t.Fatalf("run_play alone should have positive CID (it changes the move set), got %.3f", soloCID)
	}
	if withInertCID > soloCID+0.10 {
		t.Fatalf("inert borrow inflated CID via pile-on: solo=%.3f vs with-inert=%.3f (leave-one-out should ignore it)", soloCID, withInertCID)
	}
}

// TestCIDBoundedAndStable: CID stays in [0,1] and is deterministic for a fixed
// seed (no map-iteration leakage).
func TestCIDBoundedAndStable(t *testing.T) {
	deep := withBorrows(sheddingBase(),
		genome.BorrowedMechanic{Source: genome.Climbing, Mechanic: genome.MechRunPlay},
		genome.BorrowedMechanic{Source: genome.Rummy, Mechanic: genome.MechMeldBonus})
	deep.Shedding.RoundsPerGame = 3
	c1 := cidOf(t, deep)
	c2 := cidOf(t, deep)
	if c1 < 0 || c1 > 1 {
		t.Fatalf("CID out of [0,1]: %.3f", c1)
	}
	if c1 != c2 {
		t.Fatalf("CID not deterministic for fixed seed: %.3f vs %.3f", c1, c2)
	}
}
