package evolution

import (
	"math"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestSeedDescriptorsAreClassicCount verifies the engine computes a behavior
// descriptor for each of the 8 classic seeds, computed once and cached.
func TestSeedDescriptorsAreClassicCount(t *testing.T) {
	e := NewNoveltyEngine(Config{Workers: 1, BaseSeed: 7}, allSeeds())
	ds := e.seedDescriptors()
	if len(ds) != len(seeds.All()) {
		t.Fatalf("seedDescriptors len = %d, want %d (the classic seeds)", len(ds), len(seeds.All()))
	}
	// Cached: a second call returns the identical slice.
	ds2 := e.seedDescriptors()
	if &ds[0] != &ds2[0] {
		t.Fatalf("seedDescriptors not cached: distinct backing arrays across calls")
	}
}

// TestSeedDistanceTermRewardsDistantBehavior pins the core selection effect:
// with -novelty-select ON, a valid above-floor individual whose behavior is
// FAR from every classic seed must get a higher pre-normalization novelty
// score than one near a seed, holding the within-population k-NN term equal.
func TestSeedDistanceTermRewardsDistantBehavior(t *testing.T) {
	e := NewNoveltyEngine(Config{Workers: 1, BaseSeed: 7, NoveltySelect: true}, allSeeds())

	// Stub the seed descriptors so the test does not depend on simulation:
	// the only seed sits at the origin.
	e.seedDescCache = []BehaviorDescriptor{{0, 0}}
	e.seedDescOnce = true

	near := e.seedDistanceTerm(BehaviorDescriptor{0.01, 0.0})
	far := e.seedDistanceTerm(BehaviorDescriptor{0.9, 0.0})
	if !(far > near) {
		t.Fatalf("seedDistanceTerm: far=%.3f must exceed near=%.3f", far, near)
	}
	if math.Abs(near-0.01) > 1e-9 {
		t.Fatalf("seedDistanceTerm near = %.6f, want min distance 0.01", near)
	}
}

// TestSeedDistanceTermDisabledWhenFlagOff verifies the seed term contributes
// nothing when NoveltySelect is off -- the pre-existing within-population
// behavior is preserved exactly.
func TestSeedDistanceTermDisabledWhenFlagOff(t *testing.T) {
	e := NewNoveltyEngine(Config{Workers: 1, BaseSeed: 7, NoveltySelect: false}, allSeeds())
	e.seedDescCache = []BehaviorDescriptor{{0, 0}}
	e.seedDescOnce = true
	if got := e.seedDistanceTerm(BehaviorDescriptor{0.9, 0.0}); got != 0 {
		t.Fatalf("seedDistanceTerm with flag off = %.3f, want 0", got)
	}
}

// TestSeedDistanceNeverResurrectsInvalid is the CRITICAL playability guard: an
// individual that is NOT Valid, or is Valid but below FitnessFloor, must end
// the novelty pass with zero novelty and zero shared fitness regardless of how
// behaviorally far it sits from every seed. A degenerate game must never win
// on novelty alone.
func TestSeedDistanceNeverResurrectsInvalid(t *testing.T) {
	e := NewNoveltyEngine(Config{Workers: 1, BaseSeed: 7, NoveltySelect: true}, allSeeds())
	e.seedDescCache = []BehaviorDescriptor{{0, 0}} // seeds at origin
	e.seedDescOnce = true

	// Behavior maximally far from the only seed.
	farBehavior := BehaviorDescriptor{1.0, 1.0}

	invalid := &NoveltyIndividual{
		Individual: Individual{
			Genome:  &genome.Genome{ID: "broken", Skeleton: genome.Shedding},
			Valid:   false,
			Fitness: fitness.Metrics{TotalFitness: 0.99},
		},
		Behavior: farBehavior,
	}
	belowFloor := &NoveltyIndividual{
		Individual: Individual{
			Genome:  &genome.Genome{ID: "weak", Skeleton: genome.Shedding},
			Valid:   true,
			Fitness: fitness.Metrics{TotalFitness: FitnessFloor - 0.01},
		},
		Behavior: farBehavior,
	}
	good := &NoveltyIndividual{
		Individual: Individual{
			Genome:  &genome.Genome{ID: "good", Skeleton: genome.Shedding},
			Valid:   true,
			Fitness: fitness.Metrics{TotalFitness: FitnessFloor + 0.10},
		},
		Behavior: farBehavior,
	}

	e.Population = []*NoveltyIndividual{invalid, belowFloor, good}
	e.computeNovelty()

	if invalid.Novelty != 0 || invalid.Fitness.SharedFitness != 0 {
		t.Errorf("invalid individual got Novelty=%.3f SharedFitness=%.3f, want 0/0 (seed novelty must not resurrect it)",
			invalid.Novelty, invalid.Fitness.SharedFitness)
	}
	if belowFloor.Novelty != 0 || belowFloor.Fitness.SharedFitness != 0 {
		t.Errorf("below-floor individual got Novelty=%.3f SharedFitness=%.3f, want 0/0",
			belowFloor.Novelty, belowFloor.Fitness.SharedFitness)
	}
	if good.Novelty <= 0 {
		t.Errorf("valid above-floor individual got Novelty=%.3f, want > 0 (it is far from the seed)", good.Novelty)
	}
}

// TestSeedDistanceLiftsNoveltyForValid verifies that turning NoveltySelect ON
// raises a valid, above-floor, seed-distant individual's raw Novelty score
// relative to the flag-OFF baseline (same population, same behaviors).
func TestSeedDistanceLiftsNoveltyForValid(t *testing.T) {
	build := func(noveltySelect bool) float64 {
		e := NewNoveltyEngine(Config{Workers: 1, BaseSeed: 7, NoveltySelect: noveltySelect}, allSeeds())
		e.seedDescCache = []BehaviorDescriptor{{0, 0}}
		e.seedDescOnce = true

		// Two clustered individuals (so within-pop k-NN is small) sitting far
		// from the seed at origin.
		a := &NoveltyIndividual{
			Individual: Individual{
				Genome:  &genome.Genome{ID: "a", Skeleton: genome.Shedding},
				Valid:   true,
				Fitness: fitness.Metrics{TotalFitness: FitnessFloor + 0.2},
			},
			Behavior: BehaviorDescriptor{0.8, 0.8},
		}
		b := &NoveltyIndividual{
			Individual: Individual{
				Genome:  &genome.Genome{ID: "b", Skeleton: genome.Shedding},
				Valid:   true,
				Fitness: fitness.Metrics{TotalFitness: FitnessFloor + 0.2},
			},
			Behavior: BehaviorDescriptor{0.81, 0.81},
		}
		e.Population = []*NoveltyIndividual{a, b}
		e.computeNovelty()
		return a.Novelty
	}

	off := build(false)
	on := build(true)
	if !(on > off) {
		t.Fatalf("NoveltySelect must lift raw Novelty for a seed-distant valid individual: on=%.4f off=%.4f", on, off)
	}
}

// TestNoveltySelectSeedsAtDistanceZero verifies the calibration anchor: each
// classic seed's own descriptor sits at distance 0 from the seed set, so the
// seed term adds nothing for a seed itself. This is why the calibration gate
// (which evaluates seeds) is unaffected by -novelty-select.
func TestNoveltySelectSeedsAtDistanceZero(t *testing.T) {
	e := NewNoveltyEngine(Config{Workers: 1, BaseSeed: 7, NoveltySelect: true}, allSeeds())
	for i, d := range e.seedDescriptors() {
		if got := e.minSeedDistance(d); got != 0 {
			t.Errorf("seed %d descriptor %v: minSeedDistance = %.6f, want 0 (a seed is distance 0 from itself)", i, d, got)
		}
	}
}
