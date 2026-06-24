package grammar

import (
	"math/rand/v2"
	"testing"
)

func testRNG(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 0x2545F4914F6CDD1D)) }

// TestOperatorsStayWellTyped: the genetic operators NEVER leave the well-typed
// (playable-by-construction) manifold -- the property that makes evolution over
// the grammar safe by construction. RandomSpec, Mutate, and Crossover all produce
// only well-typed specs.
func TestOperatorsStayWellTyped(t *testing.T) {
	rng := testRNG(1)
	pop := make([]GameSpec, 200)
	for i := range pop {
		pop[i] = RandomSpec(rng)
		if !pop[i].WellTyped() {
			t.Fatalf("RandomSpec produced a non-well-typed spec: %s", pop[i])
		}
	}
	for i := 0; i < 2000; i++ {
		a := pop[rng.IntN(len(pop))]
		b := pop[rng.IntN(len(pop))]
		if m := Mutate(a, rng); !m.WellTyped() {
			t.Fatalf("Mutate produced a non-well-typed spec: %s (from %s)", m, a)
		}
		if c := Crossover(a, b, rng); !c.WellTyped() {
			t.Fatalf("Crossover produced a non-well-typed spec: %s (from %s x %s)", c, a, b)
		}
	}
}

// TestMutateActuallyMutates: Mutate should usually return a structurally
// different spec, not get stuck returning its input.
func TestMutateActuallyMutates(t *testing.T) {
	rng := testRNG(7)
	changed := 0
	const trials = 500
	for i := 0; i < trials; i++ {
		s := RandomSpec(rng)
		if !sameSpec(Mutate(s, rng), s) {
			changed++
		}
	}
	if changed < trials*8/10 {
		t.Errorf("Mutate changed only %d/%d specs; expected the large majority", changed, trials)
	}
}

// TestCompositionIsFamily pins the verdict-table key.
func TestCompositionIsFamily(t *testing.T) {
	rng := testRNG(3)
	for i := 0; i < 50; i++ {
		s := RandomSpec(rng)
		if s.Composition() != s.Family() {
			t.Errorf("Composition %q != Family %q", s.Composition(), s.Family())
		}
	}
}

// TestOperatorsReachModifierFamilies: the search can actually reach modified
// families (not just the 4 bare bases), or it would only ever rediscover the
// skeletons.
func TestOperatorsReachModifierFamilies(t *testing.T) {
	rng := testRNG(11)
	withMods := 0
	for i := 0; i < 500; i++ {
		if len(RandomSpec(rng).Mods) > 0 {
			withMods++
		}
	}
	if withMods == 0 {
		t.Error("RandomSpec never produced a modified spec -- search can't reach novelty")
	}
}
