package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// assertNoDuplicateMechanics fails if g carries two borrows of the same
// Mechanic under ANY pair of Sources. BuildHooks builds one hook per Borrowed
// entry, so a duplicate mechanic double-applies its effect per event -- the
// bug the Mechanic-keyed dedupe exists to prevent.
func assertNoDuplicateMechanics(t *testing.T, g *genome.Genome, ctx string) {
	t.Helper()
	seen := map[genome.MechanicType]bool{}
	for _, b := range g.Borrowed {
		if seen[b.Mechanic] {
			t.Fatalf("%s: duplicate borrowed mechanic %d (borrows: %v)", ctx, b.Mechanic, g.Borrowed)
		}
		seen[b.Mechanic] = true
	}
}

// TestHybridCrossoverNeverDuplicatesMechanic: a parent already carrying a
// mechanic under one Source must not gain a second copy of the same mechanic
// under a different Source. The crossFamilyBorrow table (and its fallback
// Source re-stamping) can propose e.g. (TrickTaking, MechTrickScoring) to a
// host that already carries (Rummy, MechTrickScoring); the old dup check keyed
// on the full (Source, Mechanic) pair let that through and the hooks then
// applied the trick-scoring bonus twice per event.
func TestHybridCrossoverNeverDuplicatesMechanic(t *testing.T) {
	for seed := uint64(0); seed < 300; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))

		// Shedding host that already borrows trick scoring, sourced from
		// Rummy -- valid (whitelist checks Mechanic; Source only must differ
		// from the host), and mechanically identical to the TrickTaking-
		// sourced entry the cross table proposes.
		a := seeds.CrazyEights()
		a.Borrowed = []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechTrickScoring},
		}
		a.Shedding.RoundsPerGame = 2
		b := seeds.Whist()

		child := CrossoverWith(a, b, rng, true)
		if child == nil {
			t.Fatalf("seed %d: cross-skeleton crossover returned nil with flag on", seed)
		}
		assertNoDuplicateMechanics(t, child, "hybridCrossover")
		if errs := genome.Validate(child); len(errs) > 0 {
			t.Fatalf("seed %d: hybrid child failed validation: %v", seed, errs)
		}
	}
}

// TestAddBorrowedMechanicNeverDuplicatesMechanic: mutation must not stack a
// second copy of a mechanic the genome already carries under a different
// Source (same double-apply failure as the crossover path).
func TestAddBorrowedMechanicNeverDuplicatesMechanic(t *testing.T) {
	for seed := uint64(0); seed < 300; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))

		g := seeds.CrazyEights()
		g.Borrowed = []genome.BorrowedMechanic{
			{Source: genome.Rummy, Mechanic: genome.MechTrickScoring},
		}
		g.Shedding.RoundsPerGame = 2

		// Several attempts per seed: the borrow cap is 3, so repeated adds
		// exercise both the dup check and the cap.
		for i := 0; i < 5; i++ {
			addBorrowedMechanic(g, rng, true)
		}
		assertNoDuplicateMechanics(t, g, "addBorrowedMechanic")
		if errs := genome.Validate(g); len(errs) > 0 {
			t.Fatalf("seed %d: mutated genome failed validation: %v", seed, errs)
		}
	}
}
