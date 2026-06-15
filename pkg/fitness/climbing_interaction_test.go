package fitness

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// TestClimbingInteractionMeasured locks in deltaModeClimbing (pkg/sim/batch.go):
// the Interaction metric must SEE the climbing skeleton's beat/pass constraint.
// Before the fix Big Two scored Interaction 0.000 (climbing fell to
// deltaModeNone) and only skimmed the fitness floor as an artifact; the
// calibration gate's >=0.40 floor would NOT catch a regression (Big Two would
// still clear at ~0.401), so this targeted threshold is the real guard.
func TestClimbingInteractionMeasured(t *testing.T) {
	g := seeds.BigTwo()
	var sum float64
	n := 0
	for _, seed := range CalibrationSeeds {
		sum += Evaluate(g, seed).Metrics.Interaction
		n++
	}
	avg := sum / float64(n)
	if avg < 0.3 {
		t.Fatalf("climbing Interaction not measured: Big Two interaction %.3f < 0.3 -- deltaModeClimbing regressed?", avg)
	}
	t.Logf("Big Two interaction (climbing measured) = %.3f", avg)
}
