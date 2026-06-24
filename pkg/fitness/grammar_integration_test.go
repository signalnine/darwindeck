package fitness_test

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/grammar"
)

// TestGrammarRunsThroughFitness: a grammar GameSpec produces the 5 metrics via the
// REAL pipeline (fitness.EvaluateWithRunner, the injected-runner plug-in point) --
// step 5 of the rearchitecture. Every canonical spec must pass Tier 1 and yield a
// sane total fitness; the shedding/climbing specs (whose generic state aligns to
// the skeleton-conventional fields TopCard/TrickCards) must be Valid.
func TestGrammarRunsThroughFitness(t *testing.T) {
	mustBeValid := map[string]bool{
		"play_match/either|empty_hand|first_out": true, // shedding
		"beat_or_pass|empty_hand|first_out":      true, // climbing
	}
	for _, spec := range grammar.Canonical() {
		g := grammar.SpecGenome(spec)
		r := fitness.EvaluateWithRunner(g, grammar.Adapter{Spec: spec}, fitness.GetGreedyAI(g), 1)
		if !r.Tier1.Passed {
			t.Errorf("%s: Tier 1 failed -- spec does not run through the pipeline", spec.Family())
			continue
		}
		if r.Metrics.TotalFitness <= 0 || r.Metrics.TotalFitness > 1 {
			t.Errorf("%s: TotalFitness %.3f out of (0,1]", spec.Family(), r.Metrics.TotalFitness)
		}
		if mustBeValid[spec.Family()] && !r.Valid {
			t.Errorf("%s: expected Valid, got veto=%q", spec.Family(), r.DegenerateReason)
		}
	}
}
