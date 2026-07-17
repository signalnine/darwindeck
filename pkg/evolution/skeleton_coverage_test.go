package evolution

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

// TestGenomeHashCoversAllSkeletonParams pins the genomeHash contract ("every
// field that mutation or crossover can change") for the three skeletons whose
// param structs were once missing from the canon struct: two genomes differing
// only in a Climbing/Casino/Vying param must hash differently, or dedup
// destroys the variant and TopN collapses distinct games to one slot.
func TestGenomeHashCoversAllSkeletonParams(t *testing.T) {
	cases := []struct {
		name string
		a, b *genome.Genome
	}{
		{
			name: "Climbing AllowPairs",
			a: &genome.Genome{Skeleton: genome.Climbing, Players: 4, HandSize: 13,
				Climbing: &genome.ClimbingParams{AllowPairs: true}},
			b: &genome.Genome{Skeleton: genome.Climbing, Players: 4, HandSize: 13,
				Climbing: &genome.ClimbingParams{AllowPairs: false}},
		},
		{
			name: "Casino TableSize",
			a: &genome.Genome{Skeleton: genome.Casino, Players: 2, HandSize: 4,
				Casino: &genome.CasinoParams{TableSize: 4}},
			b: &genome.Genome{Skeleton: genome.Casino, Players: 2, HandSize: 4,
				Casino: &genome.CasinoParams{TableSize: 2}},
		},
		{
			name: "Vying MaxRaises",
			a: &genome.Genome{Skeleton: genome.Vying, Players: 4, HandSize: 5,
				Vying: &genome.VyingParams{StartingChips: 1000, MinBet: 10, MaxRaises: 3, RoundsPerGame: 2}},
			b: &genome.Genome{Skeleton: genome.Vying, Players: 4, HandSize: 5,
				Vying: &genome.VyingParams{StartingChips: 1000, MinBet: 10, MaxRaises: 1, RoundsPerGame: 2}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if genomeHash(c.a) == genomeHash(c.b) {
				t.Fatalf("genomes differing only in %s hash equal -- dedup would destroy the variant", c.name)
			}
		})
	}
}

// TestMAPElitesArchivesCoverAllSkeletons pins that the engine has an archive
// for EVERY skeleton: the archives map once covered only the original three,
// so the first valid Big Two / Casino / SimplePoker mutant reaching insert
// nil-pointer-panicked the whole mapelites run.
func TestMAPElitesArchivesCoverAllSkeletons(t *testing.T) {
	e := NewMAPElitesEngine(Config{BaseSeed: 1, Workers: 1}, allSeeds())
	for _, skel := range genome.AllSkeletons() {
		if e.Archives[skel] == nil {
			t.Errorf("no archive for skeleton %s", skel)
		}
	}

	// insert on a climbing genome must not panic and must land in its archive.
	g := &genome.Genome{Skeleton: genome.Climbing, Players: 4, HandSize: 13,
		Climbing: &genome.ClimbingParams{AllowPairs: true, MinRunLen: 3}}
	if !e.insert(g, fitness.Metrics{TotalFitness: 0.5}, BehaviorDescriptor{}) {
		t.Fatal("insert of a climbing genome into an empty archive failed")
	}
	if e.Archives[genome.Climbing].Occupied != 1 {
		t.Fatalf("climbing archive Occupied = %d, want 1", e.Archives[genome.Climbing].Occupied)
	}
}
