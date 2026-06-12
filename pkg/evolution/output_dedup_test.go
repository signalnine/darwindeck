package evolution

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

// Output-pipeline regression tests (Task 28 step 4 round 2, commit 5): the
// post-fix flagship's published top 30 contained byte-identical genomes under
// different IDs (ranks 2/3 identical; ranks 11-20 held a 6-way clone group),
// and genome.json's fitness field stored the novelty-blended SharedFitness
// while report.md showed raw TotalFitness (0.41 vs 0.94 confusion).

// cloneWithID returns a copy of g distinguishable only by ID -- the exact
// shape of the flagship's published duplicates (IDs differ, content does
// not; genomeHash ignores ID).
func cloneWithID(g *genome.Genome, id string) *genome.Genome {
	c := g.Clone()
	c.ID = id
	return c
}

// distinctShedding returns a shedding genome whose DrawPenalty varies with n,
// so each n yields a distinct genomeHash.
func distinctShedding(n int) *genome.Genome {
	return &genome.Genome{
		ID:       fmt.Sprintf("distinct-%d", n),
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 5 + n, // distinct content
		Shedding: &genome.SheddingParams{MatchRule: genome.MatchEither, DrawPenalty: 1},
	}
}

func mkInd(g *genome.Genome, fit float64) *Individual {
	return &Individual{
		Genome:  g,
		Valid:   true,
		Fitness: fitness.Metrics{TotalFitness: fit, SharedFitness: fit},
	}
}

// TestTopNDedupsByteIdenticalGenomes: planted duplicates must yield a
// distinct top-N, with the next-best DISTINCT genome filling the freed slot.
func TestTopNDedupsByteIdenticalGenomes(t *testing.T) {
	base := distinctShedding(0)
	pop := []*Individual{
		mkInd(cloneWithID(base, "dup-a"), 0.90),
		mkInd(cloneWithID(base, "dup-b"), 0.89), // byte-identical to dup-a
		mkInd(cloneWithID(base, "dup-c"), 0.88), // byte-identical to dup-a
		mkInd(distinctShedding(1), 0.80),
		mkInd(distinctShedding(2), 0.70),
		mkInd(distinctShedding(3), 0.60),
	}
	e := &Engine{Population: pop}

	top := e.TopN(4)
	if len(top) != 4 {
		t.Fatalf("TopN(4) returned %d individuals", len(top))
	}
	seen := map[string]bool{}
	for _, ind := range top {
		h := genomeHash(ind.Genome)
		if seen[h] {
			t.Fatalf("TopN emitted byte-identical genomes (id %s)", ind.Genome.ID)
		}
		seen[h] = true
	}
	// The freed slots must be filled by the next-best distinct genomes.
	if top[0].Genome.ID != "dup-a" {
		t.Errorf("top[0] = %s, want the best duplicate kept (dup-a)", top[0].Genome.ID)
	}
	for i, want := range []string{"distinct-1", "distinct-2", "distinct-3"} {
		if top[i+1].Genome.ID != want {
			t.Errorf("top[%d] = %s, want %s (next-best distinct)", i+1, top[i+1].Genome.ID, want)
		}
	}
}

// TestNoveltyAllQualifiedDedupsByteIdenticalGenomes: the novelty engine's
// output union (population + archive) must collapse clone groups to their
// best-fitness member, keeping behaviors parallel.
func TestNoveltyAllQualifiedDedupsByteIdenticalGenomes(t *testing.T) {
	base := distinctShedding(0)
	mkNov := func(g *genome.Genome, fit float64, b BehaviorDescriptor) *NoveltyIndividual {
		return &NoveltyIndividual{Individual: *mkInd(g, fit), Behavior: b}
	}
	e := &NoveltyEngine{
		Population: []*NoveltyIndividual{
			mkNov(cloneWithID(base, "dup-a"), 0.80, BehaviorDescriptor{0.1, 0.1}),
			mkNov(cloneWithID(base, "dup-b"), 0.90, BehaviorDescriptor{0.2, 0.2}), // best clone
			mkNov(distinctShedding(1), 0.70, BehaviorDescriptor{0.3, 0.3}),
		},
		Archive: []*NoveltyIndividual{
			mkNov(cloneWithID(base, "dup-c"), 0.85, BehaviorDescriptor{0.4, 0.4}),
			mkNov(distinctShedding(2), 0.65, BehaviorDescriptor{0.5, 0.5}),
		},
	}

	inds, behaviors := e.AllQualified()
	if len(inds) != len(behaviors) {
		t.Fatalf("individuals (%d) and behaviors (%d) must stay parallel", len(inds), len(behaviors))
	}
	if len(inds) != 3 {
		t.Fatalf("want 3 distinct genomes (clone group collapsed), got %d", len(inds))
	}
	seen := map[string]*Individual{}
	for _, ind := range inds {
		h := genomeHash(ind.Genome)
		if seen[h] != nil {
			t.Fatalf("AllQualified emitted byte-identical genomes (ids %s, %s)", seen[h].Genome.ID, ind.Genome.ID)
		}
		seen[h] = ind
	}
	if kept := seen[genomeHash(base)]; kept == nil || kept.Genome.ID != "dup-b" {
		t.Errorf("clone group must keep its best-fitness member dup-b, kept %v", kept)
	}
}

// TestMAPElitesAllQualifiedDedupsByteIdenticalGenomes: the same genome
// occupying two cells (clones admitted into neighboring niches) must be
// published once.
func TestMAPElitesAllQualifiedDedupsByteIdenticalGenomes(t *testing.T) {
	base := distinctShedding(0)
	archive := &Archive{}
	archive.Cells[0][0] = &ArchiveCell{Individual: mkInd(cloneWithID(base, "dup-a"), 0.90)}
	archive.Cells[1][1] = &ArchiveCell{Individual: mkInd(cloneWithID(base, "dup-b"), 0.95)}
	archive.Cells[2][2] = &ArchiveCell{Individual: mkInd(distinctShedding(1), 0.80)}
	e := &MAPElitesEngine{Archives: map[genome.SkeletonType]*Archive{genome.Shedding: archive}}

	out := e.AllQualified()
	if len(out) != 2 {
		t.Fatalf("want 2 distinct genomes from 3 cells with a clone pair, got %d", len(out))
	}
	seen := map[string]*Individual{}
	for _, ind := range out {
		h := genomeHash(ind.Genome)
		if seen[h] != nil {
			t.Fatalf("AllQualified emitted byte-identical genomes (ids %s, %s)", seen[h].Genome.ID, ind.Genome.ID)
		}
		seen[h] = ind
	}
	if kept := seen[genomeHash(base)]; kept == nil || kept.Genome.ID != "dup-b" {
		t.Errorf("clone pair must keep its best-fitness occupant dup-b, kept %v", kept)
	}
}

// TestGenomeJSONStoresRawAndSharedFitness pins the genome.json contract: the
// fitness field is the RAW TotalFitness (what report.md shows) and the
// novelty/sharing-blended score is a separate explicit shared_fitness field.
// The flagship published genome.json files whose fitness (0.41-class,
// SharedFitness) contradicted their own report.md (0.94-class raw).
func TestGenomeJSONStoresRawAndSharedFitness(t *testing.T) {
	// Engine path: applyFitnessSharing must not clobber Genome.Fitness with
	// the shared value.
	popA := mkInd(distinctShedding(1), 0)
	popA.Fitness = fitness.Metrics{TotalFitness: 0.94}
	popB := mkInd(&genome.Genome{
		ID: "tt", Skeleton: genome.TrickTaking, Players: 4, HandSize: 13,
		TrickTaking: &genome.TrickTakingParams{MustFollowSuit: true, RoundsPerGame: 1},
	}, 0)
	popB.Fitness = fitness.Metrics{TotalFitness: 0.80}
	e := &Engine{Population: []*Individual{popA, popB}}
	e.applyFitnessSharing()

	for _, ind := range e.Population {
		if ind.Fitness.SharedFitness == 0 {
			t.Fatalf("premise broken: sharing must set SharedFitness")
		}
		if ind.Genome.Fitness != ind.Fitness.TotalFitness {
			t.Errorf("%s: genome.Fitness = %.3f, want raw TotalFitness %.3f",
				ind.Genome.ID, ind.Genome.Fitness, ind.Fitness.TotalFitness)
		}
		if ind.Genome.SharedFitness != ind.Fitness.SharedFitness {
			t.Errorf("%s: genome.SharedFitness = %.3f, want %.3f",
				ind.Genome.ID, ind.Genome.SharedFitness, ind.Fitness.SharedFitness)
		}
	}

	// JSON field names are the published contract.
	data, err := json.Marshal(e.Population[0].Genome)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if got := fields["fitness"]; got != e.Population[0].Fitness.TotalFitness {
		t.Errorf(`json "fitness" = %v, want raw TotalFitness %v`, got, e.Population[0].Fitness.TotalFitness)
	}
	if got := fields["shared_fitness"]; got != e.Population[0].Fitness.SharedFitness {
		t.Errorf(`json "shared_fitness" = %v, want %v`, got, e.Population[0].Fitness.SharedFitness)
	}
}
