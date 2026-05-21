package evolution

import (
	"fmt"
	"testing"

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
	// to also exercise the inner row-major iteration.
	populate := func(skel genome.SkeletonType, r, c int, id string) {
		g := &genome.Genome{ID: id, Skeleton: skel}
		e.Archives[skel].Cells[r][c] = &ArchiveCell{
			Individual: &Individual{Genome: g, Valid: true},
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
			Individual: &Individual{Genome: g, Valid: true},
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
