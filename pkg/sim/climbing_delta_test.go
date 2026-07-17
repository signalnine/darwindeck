// External test package: the climbing skeleton imports sim, so an in-package
// test would be an import cycle.
package sim_test

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/climbing"
)

// TestClimbingOptionDeltaNeverPositive pins deltaModeClimbing's documented
// "always <= 0" contract. The delta once compared a raw GenerateMoves count
// on both sides of the counterfactual, but the follow position always carries
// an extra Pass and a free lead never does -- a +1 floor bias that produced
// impossible positive deltas (a follower whose every combo beats the table
// read +1) and read "removed exactly one option" as 0. The play-only probe
// removes the bias; every beat is also a legal lead, so no delta can be
// positive.
func TestClimbingOptionDeltaNeverPositive(t *testing.T) {
	g := seeds.BigTwo()
	result := sim.RunBatch(g, &climbing.Runner{}, &sim.RandomAI{}, 50, 42)
	turns, positives := 0, 0
	for _, game := range result.AllTurns {
		for _, tr := range game {
			turns++
			if tr.OptionDelta > 0 {
				positives++
			}
		}
	}
	if turns == 0 {
		t.Fatal("no turn records produced")
	}
	if positives > 0 {
		t.Fatalf("%d/%d climbing turn records have OptionDelta > 0; the climb-constraint delta must be <= 0", positives, turns)
	}
}
