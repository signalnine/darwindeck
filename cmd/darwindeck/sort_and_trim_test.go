package main

import (
	"fmt"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// rankedInd wraps g as an evaluated individual whose leaderboard rank
// (greedy-only running mean) is fit.
func rankedInd(g *genome.Genome, id string, fit float64) *evolution.Individual {
	g = g.Clone()
	g.ID = id
	return &evolution.Individual{
		Genome: g, Valid: true, EvalCount: 1, FitnessSum: fit,
		Fitness: fitness.Metrics{TotalFitness: fit, SharedFitness: fit},
	}
}

// sortAndTrim used to deduplicate by Genome.ID BEFORE sorting. IDs were
// "gen<G>_<random 0..99999>", so two different games could share one, and the
// first-seen entry won regardless of fitness: a 0.45 shedding game shadowed a
// 0.90 trick-taker with the same ID. Identity is the game's rules
// (evolution.OutputHash), never its label.
func TestSortAndTrimKeepsDistinctGamesSharingAnID(t *testing.T) {
	low := rankedInd(seeds.CrazyEights(), "gen5_123", 0.45)
	high := rankedInd(seeds.Whist(), "gen5_123", 0.90) // a DIFFERENT game, same ID

	out := sortAndTrim([]*evolution.Individual{low, high}, 20, nil)

	if len(out) != 2 {
		t.Fatalf("kept %d of 2 distinct games sharing an ID, want both", len(out))
	}
	if out[0] != high {
		t.Errorf("out[0] = %s (%.2f), want the 0.90 game first", out[0].Genome.Skeleton, out[0].OutputRank())
	}
}

// Functionally identical genomes under different IDs are one published game:
// the clone group must collapse, keeping its BEST member (dedup after the
// sort, not before).
func TestSortAndTrimDedupsClonesKeepingBest(t *testing.T) {
	worse := rankedInd(seeds.CrazyEights(), "clone-a", 0.50)
	better := rankedInd(seeds.CrazyEights(), "clone-b", 0.70)
	other := rankedInd(seeds.Whist(), "other", 0.60)

	out := sortAndTrim([]*evolution.Individual{worse, other, better}, 20, nil)

	if len(out) != 2 {
		t.Fatalf("got %d entries, want 2 (the clone group collapses to one)", len(out))
	}
	if out[0] != better || out[1] != other {
		t.Errorf("out = [%s, %s], want [clone-b, other]", out[0].Genome.ID, out[1].Genome.ID)
	}
}

// The per-skeleton reservation divides by the real skeleton count. It was a
// hardcoded 3 from the three-skeleton era (the same stale divisor Engine.TopN
// carried until the 2026-07-17 bughunt), so three strong skeletons took 6
// reserved slots each and a qualified climbing game never reached a top-20.
func TestSortAndTrimReservesSlotsForEverySkeleton(t *testing.T) {
	var inds []*evolution.Individual
	n := 0
	for _, g := range []*genome.Genome{seeds.SimplePoker(), seeds.Casino(), seeds.CrazyEights()} {
		for i := 0; i < 10; i++ {
			c := g.Clone()
			c.HandSize = 3 + i // distinct rules, so no clone-group collapse
			inds = append(inds, rankedInd(c, fmt.Sprintf("strong%d", n), 0.9-float64(n)*0.001))
			n++
		}
	}
	for i, g := range []*genome.Genome{seeds.Whist(), seeds.GinRummy(), seeds.BigTwo()} {
		inds = append(inds, rankedInd(g, fmt.Sprintf("weak%d", i), 0.6))
	}

	out := sortAndTrim(inds, 20, nil)

	if len(out) != 20 {
		t.Fatalf("got %d entries, want 20", len(out))
	}
	count := map[genome.SkeletonType]int{}
	for _, ind := range out {
		count[ind.Genome.Skeleton]++
	}
	for _, skel := range genome.AllSkeletons() {
		if count[skel] == 0 {
			t.Errorf("no %s game in the top 20 although a qualified one exists (per-skeleton counts: %v)", skel, count)
		}
	}
}

// TestSortAndTrimPublishedOrderIsMonotone (2026-10-02 bughunt review): the
// selection is two passes (per-skeleton reservation, then best-of-the-rest),
// and the result used to be returned in pass order -- so a reserved 0.60 game
// landed at a better rank than a 0.89 fill game, and "rankNN" did not mean
// rank. Which games are published is unchanged; they are returned best first.
func TestSortAndTrimPublishedOrderIsMonotone(t *testing.T) {
	var inds []*evolution.Individual
	n := 0
	for _, g := range []*genome.Genome{seeds.SimplePoker(), seeds.Casino(), seeds.CrazyEights()} {
		for i := 0; i < 10; i++ {
			c := g.Clone()
			c.HandSize = 3 + i
			inds = append(inds, rankedInd(c, fmt.Sprintf("strong%d", n), 0.9-float64(n)*0.001))
			n++
		}
	}
	for i, g := range []*genome.Genome{seeds.Whist(), seeds.GinRummy(), seeds.BigTwo()} {
		inds = append(inds, rankedInd(g, fmt.Sprintf("weak%d", i), 0.6))
	}

	out := sortAndTrim(inds, 20, nil)

	for i := 1; i < len(out); i++ {
		if out[i].OutputRank() > out[i-1].OutputRank() {
			t.Fatalf("rank %d (%.3f) outranks rank %d (%.3f): the published order is not best-first",
				i+1, out[i].OutputRank(), i, out[i-1].OutputRank())
		}
	}
	// The reservation still decides WHO is published.
	count := map[genome.SkeletonType]int{}
	for _, ind := range out {
		count[ind.Genome.Skeleton]++
	}
	for _, skel := range genome.AllSkeletons() {
		if count[skel] == 0 {
			t.Errorf("no %s game in the top 20 after re-ordering (counts: %v)", skel, count)
		}
	}
}
