package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestCrossoverWithFlagOffNilOnCrossSkeleton: with the flag OFF, crossing two
// different-skeleton parents returns nil (the historical hard-disabled
// behavior the caller falls back to mutation on). This is the safety contract
// that v2 shipped with.
func TestCrossoverWithFlagOffNilOnCrossSkeleton(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 0))
	for seed := uint64(0); seed < 50; seed++ {
		if child := CrossoverWith(seeds.CrazyEights(), seeds.Whist(), rng, false); child != nil {
			t.Fatalf("seed %d: cross-skeleton crossover with flag OFF must return nil", seed)
		}
	}
}

// TestCrossoverWithSameSkeletonUnaffectedByFlag: same-skeleton crossover is
// identical whether the flag is on or off.
func TestCrossoverWithSameSkeletonUnaffectedByFlag(t *testing.T) {
	for _, flag := range []bool{false, true} {
		rng := rand.New(rand.NewPCG(7, 0))
		child := CrossoverWith(seeds.CrazyEights(), seeds.MauMau(), rng, flag)
		if child == nil {
			t.Fatalf("flag=%v: same-skeleton crossover must succeed", flag)
		}
		if child.Skeleton != genome.Shedding {
			t.Fatalf("flag=%v: child skeleton = %s, want shedding", flag, child.Skeleton)
		}
		if errs := genome.Validate(child); len(errs) > 0 {
			t.Errorf("flag=%v: same-skeleton child invalid: %v", flag, errs)
		}
	}
}

// TestHybridCrossoverProducesValidHybrid: with the flag ON, crossing two
// different-skeleton parents yields a HYBRID -- host = parent A's skeleton, and
// it carries an active cross-family borrow from parent B's family. The child
// must validate, must carry at least one borrow, and that borrow must be LIVE
// (outcome-affecting) under the genome's own liveness rules.
func TestHybridCrossoverProducesValidHybrid(t *testing.T) {
	pairs := []struct {
		name string
		a, b *genome.Genome
	}{
		{"shedding x trick", seeds.CrazyEights(), seeds.Whist()},
		{"trick x shedding", seeds.Whist(), seeds.CrazyEights()},
		{"trick x rummy", seeds.Hearts(), seeds.GinRummy()},
		{"rummy x trick", seeds.GinRummy(), seeds.Hearts()},
		{"shedding x rummy", seeds.MauMau(), seeds.KnockRummy()},
		{"rummy x shedding", seeds.KnockRummy(), seeds.MauMau()},
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			sawBorrow := false
			for seed := uint64(0); seed < 60; seed++ {
				rng := rand.New(rand.NewPCG(seed, 0))
				child := CrossoverWith(p.a, p.b, rng, true)
				if child == nil {
					t.Fatalf("seed %d: hybrid crossover returned nil with flag ON", seed)
				}
				if child.Skeleton != p.a.Skeleton {
					t.Fatalf("seed %d: hybrid host skeleton = %s, want parent A's %s", seed, child.Skeleton, p.a.Skeleton)
				}
				if errs := genome.Validate(child); len(errs) > 0 {
					t.Fatalf("seed %d: hybrid child invalid: %v", seed, errs)
				}
				if len(child.Borrowed) > 0 {
					sawBorrow = true
					// Every borrow on the hybrid must be cross-family (not the
					// host's own skeleton) and LIVE.
					for _, bm := range child.Borrowed {
						if bm.Source == child.Skeleton {
							t.Fatalf("seed %d: hybrid borrow sourced from host's own skeleton %s", seed, bm.Source)
						}
					}
					if len(child.LiveBorrows()) == 0 {
						t.Fatalf("seed %d: hybrid carries borrows but none are live (outcome-affecting): %+v", seed, child.Borrowed)
					}
				}
			}
			if !sawBorrow {
				t.Errorf("%s: no hybrid across 60 seeds carried a cross-family borrow", p.name)
			}
		})
	}
}

// TestShedToWinByTricksHybridIsReachableAndOutcomeAffecting: the headline
// hybrid. Crossing a shedding parent with a trick-taking parent under the flag
// must, on at least one seed, produce a shed-to-win game scored by tricks --
// a shedding host carrying a live MechTrickScoring borrow in multi-round mode
// -- and that genome must evaluate as VALID through the real pipeline (Tier 0
// -> Tier 1 -> degeneracy vetoes), confirming the hybrid is playable, not just
// statically valid.
func TestShedToWinByTricksHybridIsReachableAndOutcomeAffecting(t *testing.T) {
	var champion *genome.Genome
	for seed := uint64(0); seed < 200 && champion == nil; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0))
		child := CrossoverWith(seeds.CrazyEights(), seeds.Whist(), rng, true)
		if child == nil || child.Skeleton != genome.Shedding {
			continue
		}
		if !child.SheddingTrickScored() {
			continue
		}
		if !child.SheddingMultiRound() {
			t.Fatalf("seed %d: trick-scored shedding hybrid is not multi-round (the borrow would be inert)", seed)
		}
		champion = child
	}
	if champion == nil {
		t.Fatal("no shed-to-win-by-tricks hybrid produced across 200 seeds")
	}

	if errs := genome.Validate(champion); len(errs) > 0 {
		t.Fatalf("shed-to-win-by-tricks hybrid invalid: %v", errs)
	}

	// Must be a LIVE borrow (the rulebook/report may advertise it).
	live := champion.LiveBorrows()
	foundTS := false
	for _, bm := range live {
		if bm.Mechanic == genome.MechTrickScoring {
			foundTS = true
		}
	}
	if !foundTS {
		t.Fatalf("MechTrickScoring not live on the hybrid: live=%+v", live)
	}

	// Playable through the real pipeline on at least one calibration seed
	// (degeneracy vetoes are the safety net, not an outright ban).
	survived := 0
	for _, s := range fitness.CalibrationSeeds {
		if fitness.Evaluate(champion, s).Valid {
			survived++
		}
	}
	if survived == 0 {
		t.Errorf("shed-to-win-by-tricks hybrid was killed on every calibration seed -- a playable hybrid must survive at least one")
	}
	t.Logf("shed-to-win-by-tricks hybrid survived %d/%d calibration seeds", survived, len(fitness.CalibrationSeeds))
}

// TestCrossFamilyMutationBorrowsReachableOnlyWithFlag: mutation may add the new
// cross-family active borrows (shedding -> MechTrickScoring, trick-taking ->
// MechAvoidance) ONLY when the flag is on. With the flag off they never appear.
func TestCrossFamilyMutationBorrowsReachableOnlyWithFlag(t *testing.T) {
	check := func(host *genome.Genome, mech genome.MechanicType, crossSkeleton bool) bool {
		seen := false
		for seed := uint64(0); seed < 4000; seed++ {
			rng := rand.New(rand.NewPCG(seed, 0))
			child := MutateWith(host, rng, seeds.All(), crossSkeleton)
			for _, bm := range child.Borrowed {
				if bm.Mechanic == mech {
					// changeSkeleton (2% rate) can swap the host; only count when
					// the mechanic landed on a host that should carry it.
					if mech == genome.MechTrickScoring && child.Skeleton == genome.Shedding {
						seen = true
					}
					if mech == genome.MechAvoidance && child.Skeleton == genome.TrickTaking {
						seen = true
					}
				}
			}
			if seen {
				break
			}
		}
		return seen
	}

	// Shedding -> MechTrickScoring
	if check(seeds.CrazyEights(), genome.MechTrickScoring, false) {
		t.Error("MechTrickScoring borrow reached on shedding with flag OFF")
	}
	if !check(seeds.CrazyEights(), genome.MechTrickScoring, true) {
		t.Error("MechTrickScoring borrow never reached on shedding with flag ON")
	}

	// Trick-taking -> MechAvoidance
	if check(seeds.Whist(), genome.MechAvoidance, false) {
		t.Error("MechAvoidance borrow reached on trick-taking with flag OFF")
	}
	if !check(seeds.Whist(), genome.MechAvoidance, true) {
		t.Error("MechAvoidance borrow never reached on trick-taking with flag ON")
	}
}

// TestEngineCrossSkeletonProducesHybridsInPopulation: an end-to-end smoke test
// that an Engine run with CrossSkeleton ON yields at least one genome carrying
// a cross-family borrow whose host differs from the borrow's source family
// (a genuine hybrid), and that the population stays all-valid.
func TestEngineCrossSkeletonProducesHybridsInPopulation(t *testing.T) {
	cfg := Config{
		PopulationSize: 40,
		Generations:    4,
		EliteSize:      4,
		TournamentSize: 3,
		Workers:        4,
		BaseSeed:       99,
		CrossSkeleton:  true,
		// MCTSDecile 0 -> greedy-only, faster.
	}
	eng := NewEngine(cfg, seeds.All())
	eng.Run(nil)

	hybrids := 0
	for _, ind := range eng.Population {
		g := ind.Genome
		if g == nil {
			continue
		}
		if errs := genome.Validate(g); len(errs) > 0 {
			t.Errorf("population genome %s invalid: %v", g.ID, errs)
		}
		for _, bm := range g.Borrowed {
			if bm.Source != g.Skeleton {
				hybrids++
				break
			}
		}
	}
	if hybrids == 0 {
		t.Error("cross-skeleton engine run produced no genome with a cross-family borrow")
	}
	t.Logf("cross-skeleton run: %d/%d genomes carry a cross-family borrow", hybrids, len(eng.Population))
}
