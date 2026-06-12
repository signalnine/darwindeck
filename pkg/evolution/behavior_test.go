package evolution

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// TestDescriptorSpread is the empirical anti-degeneracy gate for the behavior
// descriptor (audit Task 17). The old Y axis (win entropy) put 91% of mutants
// in the top row of the 10x10 grid, leaving 16/100 MAP-Elites cells reachable.
// The gate: descriptors of the 8 classic seeds plus 50 real-operator mutants
// must occupy at least 4 distinct rows AND 4 distinct columns. Mutants are
// generated with the real Mutate operator over a seeded RNG and Tier 0
// filtered, mirroring the evaluation pipeline; batches mirror the descriptor
// call sites (50 random-AI games per genome).
func TestDescriptorSpread(t *testing.T) {
	classics := seeds.All()
	genomes := make([]*genome.Genome, 0, len(classics)+50)
	genomes = append(genomes, classics...)

	rng := rand.New(rand.NewPCG(17, 0))
	const wantMutants = 50
	mutants := 0
	for draws := 0; mutants < wantMutants && draws < 500; draws++ {
		parent := classics[rng.IntN(len(classics))]
		m := Mutate(parent, rng, classics)
		if errs := genome.Validate(m); len(errs) > 0 {
			continue // Tier 0 reject, same as the evaluation pipeline
		}
		genomes = append(genomes, m)
		mutants++
	}
	if mutants < wantMutants {
		t.Fatalf("only %d/%d valid mutants after 500 draws", mutants, wantMutants)
	}

	rows := make(map[int]bool)
	cols := make(map[int]bool)
	for i, g := range genomes {
		runner := fitness.GetRunner(g)
		if runner == nil {
			t.Fatalf("genome %d (skeleton %v): no runner", i, g.Skeleton)
		}
		result := sim.RunBatch(g, runner, &sim.RandomAI{}, 50, 1000+uint64(i))
		row, col := ComputeBehavior(result).GridCell(10)
		rows[row] = true
		cols[col] = true
	}

	if len(rows) < 4 || len(cols) < 4 {
		t.Errorf("descriptor degenerate: %d distinct rows, %d distinct cols over %d genomes (want >= 4 each)",
			len(rows), len(cols), len(genomes))
	}
}

// TestGridCellBounds pins corner mapping and out-of-range clamping for
// GridCell. Row comes from b[1] (Y), col from b[0] (X); descriptors at or
// beyond 1.0 clamp into the last cell, negatives into cell 0.
func TestGridCellBounds(t *testing.T) {
	cases := []struct {
		name     string
		b        BehaviorDescriptor
		row, col int
	}{
		{"origin corner", BehaviorDescriptor{0, 0}, 0, 0},
		{"far corner clamps inside", BehaviorDescriptor{1, 1}, 9, 9},
		{"x-only corner", BehaviorDescriptor{1, 0}, 0, 9},
		{"y-only corner", BehaviorDescriptor{0, 1}, 9, 0},
		{"negative clamps to zero", BehaviorDescriptor{-0.5, -2}, 0, 0},
		{"overflow clamps to last", BehaviorDescriptor{1.7, 42}, 9, 9},
		{"interior", BehaviorDescriptor{0.55, 0.25}, 2, 5},
		{"exact midpoint boundary", BehaviorDescriptor{0.5, 0.5}, 5, 5},
	}
	for _, tc := range cases {
		row, col := tc.b.GridCell(10)
		if row != tc.row || col != tc.col {
			t.Errorf("%s: GridCell(10) = (%d,%d), want (%d,%d)", tc.name, row, col, tc.row, tc.col)
		}
	}
}

// TestDistanceMetric pins the Euclidean distance: identity, symmetry, and
// known values (a 3-4-5 triangle scaled by 0.1, and the unit diagonal).
func TestDistanceMetric(t *testing.T) {
	a := BehaviorDescriptor{0.1, 0.2}
	b := BehaviorDescriptor{0.4, 0.6}

	if d := a.Distance(a); d != 0 {
		t.Errorf("identity: Distance(a,a) = %v, want 0", d)
	}
	if d1, d2 := a.Distance(b), b.Distance(a); d1 != d2 {
		t.Errorf("symmetry: %v vs %v", d1, d2)
	}
	if got, want := a.Distance(b), 0.5; math.Abs(got-want) > 1e-12 {
		t.Errorf("3-4-5 triangle: Distance = %v, want %v", got, want)
	}
	zero := BehaviorDescriptor{0, 0}
	one := BehaviorDescriptor{1, 1}
	if got, want := zero.Distance(one), math.Sqrt2; math.Abs(got-want) > 1e-12 {
		t.Errorf("unit diagonal: Distance = %v, want %v", got, want)
	}
}
