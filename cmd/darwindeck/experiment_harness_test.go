package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
)

// TestPerSkeletonCoverageLineCoversAllSkeletons: the per-skeleton coverage
// report printed only shed/trick/rummy (a hand-coded list from the
// three-skeleton era), so climbing, casino and vying coverage -- computed and
// saved in results.json -- never reached the console summary.
func TestPerSkeletonCoverageLineCoversAllSkeletons(t *testing.T) {
	per := map[string]SkeletonMetrics{}
	for i, skel := range genome.AllSkeletons() {
		per[skel.String()] = SkeletonMetrics{Coverage: float64(i+1) / 100}
	}
	line := perSkeletonCoverageLine("hybrid", []ExperimentResult{{PerSkeleton: per}})

	if !strings.Contains(line, "hybrid") {
		t.Errorf("line %q does not name its config", line)
	}
	for i, skel := range genome.AllSkeletons() {
		want := fmt.Sprintf("%s=%.3f", skel, float64(i+1)/100)
		if !strings.Contains(line, want) {
			t.Errorf("coverage line %q is missing %q", line, want)
		}
	}
}

// The reported value is the MEDIAN across the config's runs.
func TestPerSkeletonCoverageLineIsMedian(t *testing.T) {
	var runs []ExperimentResult
	for _, cov := range []float64{0.10, 0.30, 0.20} {
		runs = append(runs, ExperimentResult{PerSkeleton: map[string]SkeletonMetrics{"climbing": {Coverage: cov}}})
	}
	if line := perSkeletonCoverageLine("baseline", runs); !strings.Contains(line, "climbing=0.200") {
		t.Errorf("line %q, want climbing=0.200 (median of 0.10/0.30/0.20)", line)
	}
}

// TestExperimentBaseSeedsAreDisjoint: replicate runs must draw from disjoint
// seed bands. BaseSeed was specSeed*1000 while the engines step their
// evaluation seed by 10000 per generation, so replicate s+10 at generation g
// reused replicate s's generation g+1 evaluation seeds (s=1,g=1 and s=11,g=0
// both evaluated slot 0 at seed 11000): the "independent" replicates the
// Mann-Whitney test compares shared game samples.
func TestExperimentBaseSeedsAreDisjoint(t *testing.T) {
	// The exact historical collision.
	const genStride = 10000 // pkg/evolution: BaseSeed + gen*10000 + idx
	if experimentBaseSeed(1)+1*genStride == experimentBaseSeed(11)+0*genStride {
		t.Error("replicate 1 at generation 1 and replicate 11 at generation 0 share evaluation seeds")
	}

	// Every offset an engine adds to BaseSeed must stay inside the run's own
	// band. The largest is MAP-Elites' challenge re-evaluation stream at
	// BaseSeed + 2^40 + challenge*10000; the evaluation stream (gen*10000 +
	// idx, plus the +5000 behavior/MCTS and +99999/+9,000,000 descriptor
	// offsets) sits far below 2^40 for any plausible run. Budget a billion
	// challenges on top of the 2^40 base.
	const maxOffset = uint64(1)<<40 + 1_000_000_000*10_000

	var bases []uint64
	for i := 0; i < 15; i++ { // the default 15 replicates ...
		for ci := 0; ci < 6; ci++ { // ... of every config
			bases = append(bases, experimentBaseSeed(experimentSpecSeed(i, ci)))
		}
	}
	sort.Slice(bases, func(a, b int) bool { return bases[a] < bases[b] })
	for k := 1; k < len(bases); k++ {
		if gap := bases[k] - bases[k-1]; gap <= maxOffset {
			t.Fatalf("base seeds %d and %d are only %d apart; a run's seed band spans up to %d", bases[k-1], bases[k], gap, maxOffset)
		}
	}
}

// TestExperimentParallelism: `-parallel 0` sized the run semaphore to zero,
// and the first `sem <- struct{}{}` blocked forever (the command hung with no
// output after the banner). Non-positive values mean one run at a time, and a
// run never gets fewer than one worker.
func TestExperimentParallelism(t *testing.T) {
	for in, want := range map[int]int{-3: 1, 0: 1, 1: 1, 4: 4} {
		if got := experimentParallelism(in); got != want {
			t.Errorf("experimentParallelism(%d) = %d, want %d", in, got, want)
		}
	}
	cases := []struct{ workers, parallel, want int }{
		{24, 3, 8},
		{8, 3, 2},
		{2, 8, 1}, // more parallel runs than workers: 1 each, never 0 (0 = auto = NumCPU per run)
		{1, 1, 1},
	}
	for _, c := range cases {
		if got := perRunWorkers(c.workers, c.parallel); got != c.want {
			t.Errorf("perRunWorkers(%d, %d) = %d, want %d", c.workers, c.parallel, got, c.want)
		}
	}
}
