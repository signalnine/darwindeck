package fitness

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// BenchmarkMCTSBatch20 times the Tier 2 MCTS skill batch exactly as
// EvaluateWithMCTS runs it: tier2MCTSGames (20) games of gin rummy, seat 0 at
// production search strength (zero-value MCTSEvalConfig = 200 iterations, 10
// determinizations), other seats random. This is the long serial pole Wave I
// parallelized inside sim.RunBatch (the per-genome MCTS grant cost the
// evolution engine pays once per top-decile candidate per generation); run it
// with -benchtime=1x before/after any batch-runner change. Task 19's serial
// measurement was ~14.5s per batch on this workload.
func BenchmarkMCTSBatch20(b *testing.B) {
	g := seeds.GinRummy()
	runner := GetRunner(g)
	hooks := mechanic.HooksFor(g)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		runMCTSBatch(g, runner, tier2MCTSGames, 12345, MCTSEvalConfig{}, hooks...)
	}
}
