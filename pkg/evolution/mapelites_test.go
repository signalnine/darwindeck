package evolution

import (
	"fmt"
	"math"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

// TestAllQualifiedDeterministicOrder pins the ordering contract for
// AllQualified: occupants are returned in (Shedding, TrickTaking, Rummy)
// skeleton order, then row-major within each archive. Iterating
// e.Archives directly is non-deterministic because Go randomizes map
// iteration order per range loop, so the slice returned to callers
// (SaveTopN, top-K comparisons) would otherwise shuffle across runs even
// with the same seed.
func TestAllQualifiedDeterministicOrder(t *testing.T) {
	e := NewMAPElitesEngine(Config{BaseSeed: 1, Workers: 1}, allSeeds())

	// Populate one cell per skeleton with a uniquely identifiable genome
	// so we can observe ordering. Use cells in different (r, c) positions
	// to also exercise the inner row-major iteration. Fixtures carry an
	// above-floor fitness because AllQualified now floor-filters output
	// (archive ADMISSION is floor-free; see TestInsertAdmitsBelowFloor).
	populate := func(skel genome.SkeletonType, r, c int, id string) {
		g := &genome.Genome{ID: id, Skeleton: skel}
		e.Archives[skel].Cells[r][c] = &ArchiveCell{
			Individual: &Individual{Genome: g, Valid: true,
				Fitness: fitness.Metrics{TotalFitness: FitnessFloor + 0.1}},
		}
	}

	populate(genome.Rummy, 0, 0, "rummy-A")
	populate(genome.Rummy, 5, 5, "rummy-B")
	populate(genome.Shedding, 3, 7, "shedding-A")
	populate(genome.TrickTaking, 9, 9, "trick-A")
	populate(genome.TrickTaking, 0, 1, "trick-B")

	want := []string{
		"shedding-A",
		"trick-B",
		"trick-A",
		"rummy-A",
		"rummy-B",
	}

	// Call AllQualified many times: each range over e.Archives uses a
	// fresh map iteration order, so a non-deterministic implementation
	// would diverge from `want` within a handful of calls.
	for i := 0; i < 50; i++ {
		got := e.AllQualified()
		if len(got) != len(want) {
			t.Fatalf("call %d: len(AllQualified())=%d, want %d", i, len(got), len(want))
		}
		for j, ind := range got {
			if ind.Genome.ID != want[j] {
				ids := make([]string, len(got))
				for k, x := range got {
					ids[k] = x.Genome.ID
				}
				t.Fatalf("call %d: order=%v, want %v", i, ids, want)
			}
		}
	}
}

// TestAllQualifiedEmptyArchives verifies the contract for the empty case:
// no occupants returns a nil/empty slice without panicking on missing
// skeleton keys.
func TestAllQualifiedEmptyArchives(t *testing.T) {
	e := NewMAPElitesEngine(Config{BaseSeed: 1, Workers: 1}, allSeeds())
	got := e.AllQualified()
	if len(got) != 0 {
		t.Fatalf("AllQualified() on empty archives returned %d items, want 0", len(got))
	}
}

// TestAllQualifiedRowMajorWithinArchive confirms that within a single
// archive, cells are emitted in row-major order regardless of insertion
// sequence. Row-major iteration is array-indexed (deterministic by
// construction), but this test guards against accidental refactors to
// map-backed cell storage.
func TestAllQualifiedRowMajorWithinArchive(t *testing.T) {
	e := NewMAPElitesEngine(Config{BaseSeed: 1, Workers: 1}, allSeeds())

	// Insert in reverse row-major order; expect output in forward order.
	cells := []struct {
		r, c int
		id   string
	}{
		{9, 9, "z"},
		{5, 2, "m"},
		{0, 0, "a"},
		{2, 5, "b"},
	}
	for _, c := range cells {
		g := &genome.Genome{ID: c.id, Skeleton: genome.Shedding}
		e.Archives[genome.Shedding].Cells[c.r][c.c] = &ArchiveCell{
			Individual: &Individual{Genome: g, Valid: true,
				Fitness: fitness.Metrics{TotalFitness: FitnessFloor + 0.1}},
		}
	}

	want := []string{"a", "b", "m", "z"}
	got := e.AllQualified()
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d", len(got), len(want))
	}
	for i, ind := range got {
		if ind.Genome.ID != want[i] {
			ids := make([]string, len(got))
			for k, x := range got {
				ids[k] = x.Genome.ID
			}
			t.Fatalf("order=%v, want %v", ids, want)
		}
	}

	// Sanity: emitting a print-friendly description doesn't allocate or
	// crash on the populated archive (regression guard for nil dereference
	// during refactors).
	_ = fmt.Sprintf("%d cells", len(got))
}

// TestInsertSameCellKeepsFitter pins the cell-elitism contract (audit Task
// 18, engine previously 0% covered): when two genomes map to the same
// behavior cell, the fitter one holds the cell regardless of insertion
// order, Occupied counts the cell once, and QDScore reflects only the
// winner. Ties keep the incumbent (strict > comparison). Since the
// winner's-curse fix the comparison runs against the incumbent's
// running mean after a challenge re-evaluation; the eval seam is stubbed to
// return the incumbent's first value so the mean stays put and this test
// pins pure comparison semantics (TestChallengeReevaluation* pin the
// mean-movement behavior).
func TestInsertSameCellKeepsFitter(t *testing.T) {
	b := BehaviorDescriptor{0.55, 0.25} // cell (2,5), pinned by TestGridCellBounds
	cases := []struct {
		name          string
		first, second float64
		wantID        string
	}{
		{"fitter second replaces", 0.50, 0.60, "B"},
		{"fitter first holds", 0.60, 0.50, "A"},
		{"tie keeps incumbent", 0.50, 0.50, "A"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewMAPElitesEngine(Config{BaseSeed: 1, Workers: 1}, allSeeds())
			e.evaluate = stubEval(tc.first, nil)
			gA := &genome.Genome{ID: "A", Skeleton: genome.Shedding}
			gB := &genome.Genome{ID: "B", Skeleton: genome.Shedding}
			e.insert(gA, fitness.Metrics{TotalFitness: tc.first}, b)
			e.insert(gB, fitness.Metrics{TotalFitness: tc.second}, b)

			arch := e.Archives[genome.Shedding]
			if arch.Occupied != 1 {
				t.Fatalf("Occupied = %d, want 1 (same cell)", arch.Occupied)
			}
			cell := arch.Cells[2][5]
			if cell == nil {
				t.Fatal("cell (2,5) empty after two insertions")
			}
			if cell.Individual.Genome.ID != tc.wantID {
				t.Errorf("cell holds %q, want %q", cell.Individual.Genome.ID, tc.wantID)
			}
			wantFit := math.Max(tc.first, tc.second)
			if got := cell.Individual.Fitness.TotalFitness; got != wantFit {
				t.Errorf("cell fitness = %v, want %v", got, wantFit)
			}
			if math.Abs(arch.QDScore-wantFit) > 1e-12 {
				t.Errorf("QDScore = %v, want %v (loser must not be double-counted)", arch.QDScore, wantFit)
			}
			if e.BestFitness != wantFit {
				t.Errorf("BestFitness = %v, want %v", e.BestFitness, wantFit)
			}
		})
	}
}

// TestInsertAdmitsBelowFloor pins audit Task 18 / carried finding (a): the
// FitnessFloor must NOT gate archive ADMISSION -- cells hold their best
// occupant regardless, because sub-floor occupants are stepping stones for
// parent selection. The floor applies to OUTPUT only: AllQualified excludes
// sub-floor occupants.
func TestInsertAdmitsBelowFloor(t *testing.T) {
	e := NewMAPElitesEngine(Config{BaseSeed: 1, Workers: 1}, allSeeds())

	sub := FitnessFloor - 0.2
	gSub := &genome.Genome{ID: "sub-floor", Skeleton: genome.Rummy}
	if !e.insert(gSub, fitness.Metrics{TotalFitness: sub}, BehaviorDescriptor{0.15, 0.85}) {
		t.Fatal("insert rejected a sub-floor genome; admission must ignore FitnessFloor")
	}
	if e.Archives[genome.Rummy].Occupied != 1 {
		t.Fatalf("Occupied = %d, want 1", e.Archives[genome.Rummy].Occupied)
	}
	if got := e.AllQualified(); len(got) != 0 {
		t.Fatalf("AllQualified returned %d sub-floor occupants, want 0 (output keeps the floor)", len(got))
	}

	gOK := &genome.Genome{ID: "above-floor", Skeleton: genome.Shedding}
	e.insert(gOK, fitness.Metrics{TotalFitness: FitnessFloor + 0.1}, BehaviorDescriptor{0.55, 0.25})
	got := e.AllQualified()
	if len(got) != 1 || got[0].Genome.ID != "above-floor" {
		t.Fatalf("AllQualified = %v entries, want exactly the above-floor occupant", len(got))
	}

	// Sub-floor occupants must be reachable as parents (stepping stones).
	foundSub := false
	for i := 0; i < 100; i++ {
		if p := e.randomArchiveOccupant(); p != nil && p.ID == "sub-floor" {
			foundSub = true
			break
		}
	}
	if !foundSub {
		t.Error("sub-floor occupant never selected as parent in 100 draws; stepping stones must be selectable")
	}
}

// TestMAPElitesAdmissionIgnoresFloorEndToEnd drives the real Run path with
// an impossible FitnessFloor (1.0): cells must still fill (admission is
// floor-free) while AllQualified stays empty (output respects the floor).
// Against the pre-Task-18 code this fails: evaluateAndInsert carried a
// hardcoded 0.70 admission gate, which after the Task 14 recalibration
// (classics 0.43-0.65) silently emptied the MAP-Elites archive.
func TestMAPElitesAdmissionIgnoresFloorEndToEnd(t *testing.T) {
	oldFloor := FitnessFloor
	FitnessFloor = 1.0
	defer func() { FitnessFloor = oldFloor }()

	e := NewMAPElitesEngine(Config{
		PopulationSize: 10,
		Generations:    1,
		Workers:        4,
		BaseSeed:       7,
	}, allSeeds())
	e.Run(nil)

	occupied, _ := e.totalStats()
	if occupied == 0 {
		t.Fatal("no cells occupied after Run; archive admission must not be gated by FitnessFloor")
	}
	if got := e.AllQualified(); len(got) != 0 {
		t.Fatalf("AllQualified returned %d entries with floor 1.0, want 0 (output keeps the floor)", len(got))
	}
	if e.BestGenome == nil || e.BestFitness <= 0 {
		t.Errorf("BestFitness/BestGenome not tracked: %v %v", e.BestFitness, e.BestGenome)
	}
}
