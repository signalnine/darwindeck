package evolution

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// MAP-Elites built its offspring with the cross-skeleton flag hard-off, so the
// cross-family borrow MUTATIONS were unreachable there. That is not a cosmetic
// gap: addBorrowedMechanic offers casino and vying hosts a candidate ONLY under
// crossSkeleton, so those two skeletons reached zero borrows under -algorithm
// mapelites, and the deep borrows (run_play / follow_suit / knock /
// trick_scoring) were unreachable on every host -- in the one algorithm whose
// job is to illuminate the behavior space.
func TestMapElitesMutationHonorsCrossSkeleton(t *testing.T) {
	cases := []struct {
		name string
		seed func() *genome.Genome
		skel genome.SkeletonType
	}{
		{"casino", seeds.Casino, genome.Casino},
		{"vying", seeds.SimplePoker, genome.Vying},
	}

	for _, tc := range cases {
		for _, cross := range []bool{false, true} {
			cfg := DefaultConfig()
			cfg.CrossSkeleton = cross
			cfg.BaseSeed = 99
			e := NewMAPElitesEngine(cfg, seeds.All())

			reached := map[genome.MechanicType]bool{}
			g := tc.seed()
			for i := 0; i < 8000; i++ {
				g = e.mutate(g)
				if g.Skeleton != tc.skel {
					g = tc.seed() // changeSkeleton jumped families; restart
					continue
				}
				for _, b := range g.Borrowed {
					reached[b.Mechanic] = true
				}
			}
			switch {
			case cross && len(reached) == 0:
				t.Errorf("%s: MAP-Elites mutation reached no borrow with -cross-skeleton ON; "+
					"the engine is not threading Config.CrossSkeleton into MutateWith", tc.name)
			case !cross && len(reached) != 0:
				t.Errorf("%s: MAP-Elites mutation reached borrows %v with -cross-skeleton OFF; "+
					"the flag must still gate them", tc.name, reached)
			}
		}
	}
}

// TestMapElitesDeepBorrowsReachable pins the other half: the runner-implemented
// deep borrows must be reachable on a shedding host under MAP-Elites too.
func TestMapElitesDeepBorrowsReachable(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CrossSkeleton = true
	cfg.BaseSeed = 7
	e := NewMAPElitesEngine(cfg, seeds.All())

	reached := map[genome.MechanicType]bool{}
	g := seeds.CrazyEights()
	for i := 0; i < 20000; i++ {
		g = e.mutate(g)
		if g.Skeleton != genome.Shedding {
			g = seeds.CrazyEights()
			continue
		}
		for _, b := range g.Borrowed {
			reached[b.Mechanic] = true
		}
	}
	for _, deep := range []genome.MechanicType{genome.MechRunPlay, genome.MechFollowSuit, genome.MechKnock} {
		if !reached[deep] {
			t.Errorf("deep borrow %s unreachable from MAP-Elites mutation with -cross-skeleton ON", deep)
		}
	}
}

// TestArchiveOrderIsDeterministic pins the fix for the QD-score float sum:
// ranging over the Archives map accumulated the floats in a per-range-random
// order, so the same seeded run reported last-bit-different QD scores -- the
// identical defect already fixed in cmd/darwindeck/experiment.go.
func TestArchiveOrderIsDeterministic(t *testing.T) {
	e := NewMAPElitesEngine(DefaultConfig(), seeds.All())
	// An archive for a skeleton outside AllSkeletons(), the case insert() can
	// create; it must still land in a deterministic position.
	e.Archives[genome.SkeletonType(200)] = &Archive{}

	want := e.archiveOrder()
	for i := 0; i < 200; i++ {
		got := e.archiveOrder()
		if len(got) != len(want) {
			t.Fatalf("archiveOrder length %d != %d", len(got), len(want))
		}
		for j := range got {
			if got[j] != want[j] {
				t.Fatalf("archiveOrder is not deterministic: %v then %v", want, got)
			}
		}
	}
	// The fixed skeleton list comes first, in its own order.
	for i, skel := range genome.AllSkeletons() {
		if want[i] != skel {
			t.Errorf("archiveOrder[%d] = %v, want %v (AllSkeletons order first)", i, want[i], skel)
		}
	}
}

// TestTotalStatsIsOrderStable exercises the sum itself with values chosen so
// that a different accumulation order gives a different float.
func TestTotalStatsIsOrderStable(t *testing.T) {
	e := NewMAPElitesEngine(DefaultConfig(), seeds.All())
	skels := genome.AllSkeletons()
	if len(skels) < 3 {
		t.Skip("need at least 3 archives")
	}
	// Catastrophic-cancellation values: (1e16 + 1) - 1e16 == 0 in float64 while
	// (1e16 - 1e16) + 1 == 1, so any order drift is visible, not sub-ULP.
	e.Archives[skels[0]].QDScore = 1e16
	e.Archives[skels[1]].QDScore = 1
	e.Archives[skels[2]].QDScore = -1e16
	for i := 3; i < len(skels); i++ {
		e.Archives[skels[i]].QDScore = 0
	}

	_, want := e.totalStats()
	for i := 0; i < 500; i++ {
		if _, got := e.totalStats(); got != want {
			t.Fatalf("totalStats QD score drifted with map iteration order: %v then %v", want, got)
		}
	}
}

// TestMapElitesRunReproducible is the end-to-end guard: two runs at the same
// seed must produce the same archive occupancy and QD score.
func TestMapElitesRunReproducible(t *testing.T) {
	run := func() (int, float64) {
		cfg := DefaultConfig()
		cfg.PopulationSize = 6
		cfg.Generations = 2
		cfg.Workers = 2
		cfg.BaseSeed = 2026
		cfg.MCTSDecile = 0
		cfg.CrossSkeleton = true
		e := NewMAPElitesEngine(cfg, seeds.All())
		e.Run(nil)
		return e.totalStats()
	}
	occA, qdA := run()
	occB, qdB := run()
	if occA != occB || qdA != qdB {
		t.Errorf("MAP-Elites run not reproducible at a fixed seed: (%d, %v) vs (%d, %v)", occA, qdA, occB, qdB)
	}
}
