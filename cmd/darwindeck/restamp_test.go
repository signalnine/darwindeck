package main

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/output"
)

// TestRankRestampGamesDemotesUnstable: every veto-stable game must outrank
// every unstable game, regardless of fitness -- the publication-integrity
// invariant the Wave M restamp enforces (the r4 rank02 case: a high-fitness
// single-eval publication that fails its own veto on fresh seeds sinks to the
// bottom).
func TestRankRestampGamesDemotesUnstable(t *testing.T) {
	g := func(id string) *genome.Genome { return &genome.Genome{ID: id, Skeleton: genome.Shedding} }
	games := []restampGame{
		{genome: g("unstable-hi"), greedyMean: 0.95, stability: output.StabilityResult{ValidCount: 1, Total: 5, Stable: false}},
		{genome: g("stable-lo"), greedyMean: 0.40, stability: output.StabilityResult{ValidCount: 5, Total: 5, Stable: true}},
		{genome: g("stable-hi"), greedyMean: 0.70, stability: output.StabilityResult{ValidCount: 4, Total: 5, Stable: true}},
		{genome: g("unstable-lo"), greedyMean: 0.10, stability: output.StabilityResult{ValidCount: 0, Total: 5, Stable: false}},
	}
	rankRestampGames(games)

	wantOrder := []string{"stable-hi", "stable-lo", "unstable-hi", "unstable-lo"}
	for i, want := range wantOrder {
		if games[i].genome.ID != want {
			t.Errorf("rank %d: got %s want %s", i+1, games[i].genome.ID, want)
		}
	}
	// The high-fitness UNSTABLE game (0.95) must be below both stable games
	// (0.70, 0.40): stability outranks fitness.
	if games[0].stability.Stable != true || games[2].genome.ID != "unstable-hi" {
		t.Errorf("unstable-hi (0.95) was not demoted below the stable games")
	}
}
