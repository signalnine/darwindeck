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

// TestCrossoverKeepsCompatibleMods: Crossover picks the modifiers and THEN may
// take the other parent's player count. A players-dependent modifier (teams needs
// 4 seats, reverse needs 3+) can become ill-typed at that point; the child must
// lose only THAT modifier, not its whole modifier set. Each inheritable modifier
// is kept with probability 1/2, so a players-independent one must still show up
// in about half the children that switched player count (wiping the set left it
// in a quarter: only when the dependent modifier happened not to be picked).
func TestCrossoverKeepsCompatibleMods(t *testing.T) {
	for _, tc := range []struct {
		name    string
		a, b    GameSpec
		keep    Modifier // players-independent: must survive the switch
		dropped Modifier // ill-typed at b's player count
	}{
		{
			"teams at 3 players",
			GameSpec{Players: 4, Deal: 13, Move: Trick, End: DeckOut, Score: MostCaptured, Mods: []Modifier{ModTrump, ModTeams}},
			GameSpec{Players: 3, Deal: 13, Move: Trick, End: DeckOut, Score: MostCaptured},
			ModTrump, ModTeams,
		},
		{
			"reverse at 2 players",
			GameSpec{Players: 3, Deal: 7, Shared: 1, Move: PlayMatch, Match: MatchEither, End: EmptyHand, Score: FirstOut, Mods: []Modifier{ModSkip, ModReverse}},
			GameSpec{Players: 2, Deal: 7, Shared: 1, Move: PlayMatch, Match: MatchEither, End: EmptyHand, Score: FirstOut},
			ModSkip, ModReverse,
		},
	} {
		rng := testRNG(5)
		switched, kept := 0, 0
		for i := 0; i < 4000; i++ {
			child := Crossover(tc.a, tc.b, rng)
			if !child.WellTyped() {
				t.Fatalf("%s: Crossover produced a non-well-typed child %s", tc.name, child)
			}
			if child.Players != tc.b.Players {
				continue
			}
			switched++
			if child.hasMod(tc.dropped) {
				t.Fatalf("%s: child %s kept a modifier that is ill-typed at %d players", tc.name, child, child.Players)
			}
			if child.hasMod(tc.keep) {
				kept++
			}
		}
		if share := float64(kept) / float64(switched); share < 0.42 {
			t.Errorf("%s: only %.0f%% of %d player-switched children kept %s (want ~50%%: the incompatible modifier must not take the rest of the set with it)",
				tc.name, 100*share, switched, tc.keep)
		}
	}
}
