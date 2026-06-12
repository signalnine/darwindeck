// Wave I (RunBatch game-parallelism) contract tests.
//
// RunBatch plays its games concurrently but must be BIT-IDENTICAL to the
// serial implementation it replaced: per-game results land in index-addressed
// slots and every aggregate is reduced sequentially in game order afterwards.
// runBatchSerial (the pre-Wave-I code) is kept permanently as the golden
// reference; TestRunBatchMatchesSerialGolden is the regression net, not
// scaffolding.
package sim_test

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/mechanic"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/rummy"
	"github.com/darwindeck/darwindeck/pkg/skeleton/shedding"
	"github.com/darwindeck/darwindeck/pkg/skeleton/tricktaking"
)

// goldenCase pairs a genome with its skeleton runner and the AI under test.
type goldenCase struct {
	name   string
	g      *genome.Genome
	runner sim.GenericRunner
	ai     sim.AIPlayer
}

func goldenCases() []goldenCase {
	greedyShed := seeds.MauMau()
	return []goldenCase{
		{"shedding/crazy-eights/random", seeds.CrazyEights(), &shedding.Runner{}, &sim.RandomAI{}},
		{"shedding/mau-mau/greedy", greedyShed, &shedding.Runner{},
			&sim.GreedyAI{Scorer: sim.NewSheddingScorer(greedyShed)}},
		// Borrow-carrying genome: MechAvoidance + MechMeldBonus, so
		// mechanic.HooksFor attaches real hooks to the batch.
		{"shedding/catch-all-skip-borrows/random", seeds.CatchAllSkipShedding(), &shedding.Runner{}, &sim.RandomAI{}},
		{"tricktaking/whist/random", seeds.Whist(), &tricktaking.Runner{}, &sim.RandomAI{}},
		{"tricktaking/hearts/random", seeds.Hearts(), &tricktaking.Runner{}, &sim.RandomAI{}},
		{"rummy/gin/random", seeds.GinRummy(), &rummy.Runner{}, &sim.RandomAI{}},
		{"rummy/knock/random", seeds.KnockRummy(), &rummy.Runner{}, &sim.RandomAI{}},
	}
}

// TestRunBatchMatchesSerialGolden is the Wave I golden test: across all three
// skeletons x several genomes (including a borrow-carrying one with hooks
// attached) x 5 base seeds x n=20, the parallel RunBatch must produce a
// BatchResult deeply equal to the serial reference -- every per-game stream
// (AllEvents/AllTurns/AllLeaders/AllWinners) in the same order, every
// aggregate (WinCounts, Min/Max/Avg/TotalTurns, TurnsList, error counters)
// identical. PERMANENT: this is the bit-identical contract, do not delete
// when the serial reference looks "redundant".
func TestRunBatchMatchesSerialGolden(t *testing.T) {
	baseSeeds := []uint64{1, 7, 40, 1234, 99999}
	const n = 20
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			hooks := mechanic.HooksFor(tc.g)
			for _, seed := range baseSeeds {
				par := sim.RunBatch(tc.g, tc.runner, tc.ai, n, seed, hooks...)
				ser := sim.RunBatchSerialForTest(tc.g, tc.runner, tc.ai, n, seed, hooks...)
				if !reflect.DeepEqual(par, ser) {
					t.Fatalf("seed %d: parallel RunBatch != serial reference\nparallel: %+v\nserial:   %+v",
						seed, summarize(par), summarize(ser))
				}
			}
		})
	}
	// Premise check: the borrow-carrying case must actually attach hooks,
	// otherwise the golden suite silently stops covering hooked batches.
	if len(mechanic.HooksFor(seeds.CatchAllSkipShedding())) == 0 {
		t.Fatal("premise broken: catch-all-skip seed no longer carries hook-building borrows")
	}
}

// summarize keeps golden-mismatch output readable: the aggregates, not the
// full event streams.
func summarize(r sim.BatchResult) string {
	return fmt.Sprintf(
		"{Games:%d Completions:%d Errors:%d Timeouts:%d Wins:%v TotalTurns:%d Min:%d Max:%d Avg:%.3f Turns:%v Winners:%v}",
		r.GamesPlayed, r.Completions, r.Errors, r.Timeouts, r.WinCounts,
		r.TotalTurns, r.MinTurns, r.MaxTurns, r.AvgTurns, r.TurnsList, r.AllWinners)
}

// TestRunBatchWorkerBoundIsBounded pins the Wave I nested-parallelism
// contract: engines already fan out across genomes (cfg.Workers goroutines,
// 256 on the EPYC), so the per-batch game parallelism must stay a SMALL
// bounded factor -- min(BatchGameParallelism, GOMAXPROCS, n) -- never
// per-game unbounded spawning.
func TestRunBatchWorkerBoundIsBounded(t *testing.T) {
	maxProcs := runtime.GOMAXPROCS(0)
	min3 := func(a, b, c int) int {
		m := a
		if b < m {
			m = b
		}
		if c < m {
			m = c
		}
		return m
	}
	cases := []struct{ n, want int }{
		{0, 0},
		{1, 1},
		{2, min3(sim.BatchGameParallelism, maxProcs, 2)},
		{20, min3(sim.BatchGameParallelism, maxProcs, 20)},
		{200, min3(sim.BatchGameParallelism, maxProcs, 200)},
		{100000, min3(sim.BatchGameParallelism, maxProcs, 100000)},
	}
	for _, tc := range cases {
		if got := sim.BatchWorkerCountForTest(tc.n); got != tc.want {
			t.Errorf("batchWorkerCount(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
	if sim.BatchGameParallelism != 8 {
		t.Errorf("BatchGameParallelism = %d, want 8 (the documented bounded factor; change deliberately, with the engine-level fan-out in mind)", sim.BatchGameParallelism)
	}
}

// TestRunBatchHookedBatchRaceClean runs a borrow-carrying batch large enough
// to saturate the worker bound. Its real assertion is the -race detector
// (requirement: go test -race ./pkg/sim/ must be clean): mechanic.HooksFor
// returns ONE set of closures shared by every concurrently-played game, which
// is safe because the audit of pkg/mechanic/hooks.go found all hook Apply
// functions to be stateless functions of (state, g, event) -- see the guard
// comment on HooksFor. If a future hook captures per-game mutable state, this
// test is the tripwire.
func TestRunBatchHookedBatchRaceClean(t *testing.T) {
	g := seeds.CatchAllSkipShedding()
	hooks := mechanic.HooksFor(g)
	if len(hooks) == 0 {
		t.Fatal("premise broken: genome must carry hook-building borrows")
	}
	res := sim.RunBatch(g, &shedding.Runner{}, &sim.RandomAI{}, 32, 7, hooks...)
	if res.GamesPlayed != 32 || len(res.AllEvents) != 32 {
		t.Fatalf("hooked batch: GamesPlayed = %d, len(AllEvents) = %d, want 32/32", res.GamesPlayed, len(res.AllEvents))
	}
}

// TestRunBatchSharedMCTSAIRaceClean shares ONE MCTSAI instance across all
// games of a parallel batch -- exactly how fitness.runMCTSBatch builds the
// Tier 2 MCTS skill batch. Safe because MCTSAI's fields are read-only
// configuration during SelectMove (every per-decision structure -- trees,
// visit maps, determinizations -- is local to the call; see the concurrency
// note on MCTSAI). Reduced search knobs keep the test fast; -race is the
// real assertion.
func TestRunBatchSharedMCTSAIRaceClean(t *testing.T) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	mcts := &sim.MCTSAI{Runner: runner, Genome: g, Iterations: 8, Determinizations: 2, RolloutCap: 20}
	random := &sim.RandomAI{}
	ai := &sim.PerPlayerAI{Players: []sim.AIPlayer{mcts, random}, Fallback: random}
	res := sim.RunBatch(g, runner, ai, 8, 3, mechanic.HooksFor(g)...)
	if res.GamesPlayed != 8 || len(res.AllEvents) != 8 {
		t.Fatalf("shared-MCTSAI batch: GamesPlayed = %d, len(AllEvents) = %d, want 8/8", res.GamesPlayed, len(res.AllEvents))
	}
}
