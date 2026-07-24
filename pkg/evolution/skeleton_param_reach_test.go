package evolution

import (
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// Every skeleton param a runner reads must be reachable by BOTH operators.
// Casino and Vying were the newest skeletons and got missed in the per-skeleton
// switches: tweakParameter/flipBool had no Casino case (TableSize and
// AllowSumCapture never changed in 20k mutations) and Crossover's switch covered
// only the first four skeletons (casino and vying params never recombined in
// 4000 crossovers). Same class as the genomeHash / MAP-Elites-archives / TopN
// gaps: a per-skeleton switch that stopped being exhaustive.

func TestCasinoParamsReachableByMutation(t *testing.T) {
	all := seeds.All()
	rng := rand.New(rand.NewPCG(1, 2))
	tableSizes := map[int]bool{}
	sumCapture := map[bool]bool{}

	g := seeds.Casino()
	for i := 0; i < 8000; i++ {
		g = MutateWith(g, rng, all, true)
		if g.Skeleton != genome.Casino || g.Casino == nil {
			g = seeds.Casino() // changeSkeleton jumped families; restart
			continue
		}
		if errs := genome.Validate(g); len(errs) > 0 {
			t.Fatalf("mutation produced an invalid casino genome: %v (%s)", errs, g.ActiveParams())
		}
		tableSizes[g.Casino.TableSize] = true
		sumCapture[g.Casino.AllowSumCapture] = true
	}
	if len(tableSizes) < 2 {
		t.Errorf("casino TableSize never mutated (only saw %v); tweakParameter needs a Casino case", tableSizes)
	}
	if len(sumCapture) < 2 {
		t.Errorf("casino AllowSumCapture never flipped (only saw %v); flipBool needs a Casino case", sumCapture)
	}
}

func TestCasinoParamsReachableByCrossover(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	a := seeds.Casino()
	b := seeds.Casino()
	b.Casino.TableSize = 6
	b.Casino.AllowSumCapture = !a.Casino.AllowSumCapture

	gotTable, gotSum := false, false
	for i := 0; i < 2000; i++ {
		c := CrossoverWith(a, b, rng, true)
		if c == nil || c.Casino == nil {
			t.Fatal("same-skeleton casino crossover returned nil")
		}
		if errs := genome.Validate(c); len(errs) > 0 {
			t.Fatalf("crossover produced an invalid casino genome: %v (%s)", errs, c.ActiveParams())
		}
		if c.Casino.TableSize == b.Casino.TableSize {
			gotTable = true
		}
		if c.Casino.AllowSumCapture == b.Casino.AllowSumCapture {
			gotSum = true
		}
	}
	if !gotTable || !gotSum {
		t.Errorf("casino params never recombined (table=%v sum=%v); Crossover needs a Casino case", gotTable, gotSum)
	}
}

func TestVyingParamsReachableByCrossover(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	a := seeds.SimplePoker()
	b := seeds.SimplePoker()
	b.Vying.MinBet = a.Vying.MinBet * 2
	b.Vying.MaxRaises = a.Vying.MaxRaises + 1
	b.Vying.RoundsPerGame = a.Vying.RoundsPerGame + 3
	b.Vying.StartingChips = 2 * b.Vying.RoundsPerGame * b.Vying.MinBet * (b.Vying.MaxRaises + 1)
	if errs := genome.Validate(b); len(errs) > 0 {
		t.Fatalf("parent b invalid: %v", errs)
	}

	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		c := CrossoverWith(a, b, rng, true)
		if c == nil || c.Vying == nil {
			t.Fatal("same-skeleton vying crossover returned nil")
		}
		// Valid-in/valid-out: the four betting coin flips are independent, so
		// the stack-sufficiency repair must run.
		if errs := genome.Validate(c); len(errs) > 0 {
			t.Fatalf("crossover produced an invalid vying genome: %v (%s)", errs, c.ActiveParams())
		}
		if c.Vying.MinBet == b.Vying.MinBet {
			seen["min_bet"] = true
		}
		if c.Vying.MaxRaises == b.Vying.MaxRaises {
			seen["max_raises"] = true
		}
		if c.Vying.RoundsPerGame == b.Vying.RoundsPerGame {
			seen["rounds"] = true
		}
	}
	for _, want := range []string{"min_bet", "max_raises", "rounds"} {
		if !seen[want] {
			t.Errorf("vying %s never recombined; Crossover needs a Vying case", want)
		}
	}
}

// TestVyingCrossoverRepairsStackSufficiency targets the repair directly: a child
// that takes a long game and a big raise cap from one parent and a small stack
// from the other must have its stack raised, not be handed to Tier 0 to kill.
func TestVyingCrossoverRepairsStackSufficiency(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	a := seeds.SimplePoker()
	a.Vying.RoundsPerGame = 30
	a.Vying.MaxRaises = 6
	a.Vying.MinBet = 50
	a.Vying.StartingChips = 30 * 50 * 7

	b := seeds.SimplePoker()
	b.Vying.RoundsPerGame = 2
	b.Vying.MaxRaises = 1
	b.Vying.MinBet = 5
	b.Vying.StartingChips = 200

	for _, g := range []*genome.Genome{a, b} {
		if errs := genome.Validate(g); len(errs) > 0 {
			t.Fatalf("parent invalid: %v", errs)
		}
	}
	for i := 0; i < 1000; i++ {
		c := CrossoverWith(a, b, rng, true)
		worst := c.Vying.RoundsPerGame * c.Vying.MinBet * (c.Vying.MaxRaises + 1)
		if c.Vying.StartingChips < worst {
			t.Fatalf("child stack %d < worst-case commitment %d (rounds %d, min_bet %d, max_raises %d)",
				c.Vying.StartingChips, worst, c.Vying.RoundsPerGame, c.Vying.MinBet, c.Vying.MaxRaises)
		}
	}
}

// TestEverySkeletonHasCrossoverAndTweakCoverage is the drift guard: it walks
// genome.AllSkeletons() and asserts that for each one, mutation and crossover
// can actually move SOMETHING skeleton-specific. A seventh skeleton added
// without touching the operator switches fails here instead of silently
// evolving with its params frozen.
func TestEverySkeletonHasCrossoverAndTweakCoverage(t *testing.T) {
	bySkeleton := map[genome.SkeletonType]*genome.Genome{}
	for _, s := range seeds.All() {
		if _, ok := bySkeleton[s.Skeleton]; !ok {
			bySkeleton[s.Skeleton] = s
		}
	}
	all := seeds.All()

	for _, skel := range genome.AllSkeletons() {
		base, ok := bySkeleton[skel]
		if !ok {
			t.Errorf("%s: no seed to probe with", skel)
			continue
		}
		rng := rand.New(rand.NewPCG(uint64(skel)+41, 17))

		mutated := false
		g := base.Clone()
		for i := 0; i < 6000 && !mutated; i++ {
			g = MutateWith(g, rng, all, true)
			if g.Skeleton != skel {
				g = base.Clone()
				continue
			}
			if g.ActiveParams() != base.ActiveParams() {
				mutated = true
			}
		}
		if !mutated {
			t.Errorf("%s: mutation never changed any skeleton-specific param (frozen search axis); "+
				"tweakParameter/flipBool are missing a case", skel)
		}

		crossed := false
		other := base.Clone()
		// Drive the other parent somewhere different via mutation, then check
		// crossover can pull that difference across.
		for i := 0; i < 6000; i++ {
			other = MutateWith(other, rng, all, true)
			if other.Skeleton != skel {
				other = base.Clone()
				continue
			}
			if other.ActiveParams() != base.ActiveParams() {
				break
			}
		}
		if other.ActiveParams() == base.ActiveParams() {
			continue // covered by the mutation failure above
		}
		for i := 0; i < 3000 && !crossed; i++ {
			c := CrossoverWith(base, other, rng, true)
			if c != nil && c.ActiveParams() != base.ActiveParams() {
				crossed = true
			}
		}
		if !crossed {
			t.Errorf("%s: crossover never pulled a skeleton-specific param from the second parent; "+
				"Crossover's switch is missing a case", skel)
		}
	}
}
