package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// Wave M publication-integrity fix tests (audit Task 28/29 follow-up).
//
// The r4 flagship published a shedding genome (rank02) that fails its own
// greedy_longest_run veto on 1/10 seeds -- a publication-integrity hole, since
// production does ONE eval per genome. SaveResults now re-evaluates each
// top-N genome K times at fresh seeds and demotes any that is not veto-stable.

// mkInd builds a published-style individual whose greedy-only running mean is
// fit -- the OutputRank leaderboard key the stability re-rank starts from.
func mkInd(g *genome.Genome, fit float64) *evolution.Individual {
	return &evolution.Individual{
		Genome:     g,
		Valid:      true,
		Fitness:    fitness.Metrics{TotalFitness: fit},
		EvalCount:  1,
		FitnessSum: fit,
	}
}

// TestStabilityCleanGenomeKeepsRank: a healthy classic seed is valid on every
// re-eval, so it stays stable and keeps its place above an unstable genome.
func TestStabilityCleanGenomeKeepsRank(t *testing.T) {
	clean := seeds.CrazyEights()
	st := EvaluateStability(clean, 12345)
	if !st.Stable {
		t.Fatalf("crazy-eights should be veto-stable, got %d/%d (%v)",
			st.ValidCount, st.Total, st.Reasons)
	}
	if st.ValidCount != st.Total {
		t.Errorf("expected all %d re-evals valid for a classic, got %d", st.Total, st.ValidCount)
	}
}

// TestStabilityPlantedDegenerateIsUnstable: a known degenerate fixture (it
// fails Tier-1 / a veto on most seeds) must be flagged unstable.
func TestStabilityPlantedDegenerateIsUnstable(t *testing.T) {
	degen := seeds.WildUnionShedding() // vetoed 10/10 in calibration
	st := EvaluateStability(degen, 12345)
	if st.Stable {
		t.Fatalf("wild-union shedding should be UNSTABLE, got %d/%d valid", st.ValidCount, st.Total)
	}
}

// TestStabilityDeterministic: same genome + same base seed => identical result.
func TestStabilityDeterministic(t *testing.T) {
	g := seeds.GinRummy()
	a := EvaluateStability(g, 999)
	b := EvaluateStability(g, 999)
	if a.ValidCount != b.ValidCount || a.Stable != b.Stable || a.Total != b.Total {
		t.Fatalf("stability check not deterministic: %+v vs %+v", a, b)
	}
}

// TestStabilityReRanksDemoteUnstable: a stable genome with a LOWER greedy mean
// must outrank an unstable genome with a higher greedy mean after the
// stability re-rank, and the unstable one must be flagged in genome.json.
func TestStabilityReRanksDemoteUnstable(t *testing.T) {
	unstable := mkInd(seeds.WildUnionShedding(), 0.95) // high fitness, but degenerate
	stable := mkInd(seeds.CrazyEights(), 0.45)         // lower fitness, healthy

	top := []*evolution.Individual{unstable, stable}
	dir := t.TempDir()
	cfg := evolution.Config{
		PopulationSize: 10,
		Generations:    5,
		BaseSeed:       42,
		OutputDir:      dir,
	}
	if err := SaveResults(dir, top, cfg, time.Second); err != nil {
		t.Fatalf("SaveResults: %v", err)
	}

	rank01 := readPublishedGenome(t, dir, 1)
	rank02 := readPublishedGenome(t, dir, 2)

	if !rank01.VetoStable {
		t.Errorf("rank01 should be the veto-stable genome, got %+v stable=%v evals=%s",
			rank01.ID, rank01.VetoStable, rank01.StableEvals)
	}
	if rank02.VetoStable {
		t.Errorf("rank02 should be the UNSTABLE (demoted) genome, got stable=%v evals=%s",
			rank02.VetoStable, rank02.StableEvals)
	}
	// The clean genome (crazy-eights) must have risen to rank01 despite its
	// lower fitness; the degenerate must have sunk to rank02.
	if rank01.ID == unstable.Genome.ID {
		t.Errorf("the unstable high-fitness genome was NOT demoted -- still rank01")
	}
}

// TestStabilityMajorityBoundary pins the documented policy: a genome valid on
// a MAJORITY (>= 3/5) of fresh seeds is stable; below that it is demoted. This
// is the "2/5 demoted" case from the Wave M brief and the exact threshold the
// flag publishes.
func TestStabilityMajorityBoundary(t *testing.T) {
	cases := []struct {
		valid int
		want  bool
	}{
		{0, false}, {1, false}, {2, false}, {3, true}, {4, true}, {5, true},
	}
	for _, c := range cases {
		got := StabilityResult{ValidCount: c.valid, Total: stabilityEvals, Stable: c.valid >= stabilityMajority}
		if got.Stable != c.want {
			t.Errorf("%d/%d: stable=%v want %v", c.valid, stabilityEvals, got.Stable, c.want)
		}
		if got.Label() != labelFor(c.valid) {
			t.Errorf("label %q want %q", got.Label(), labelFor(c.valid))
		}
	}
	if stabilityMajority != 3 {
		t.Fatalf("majority constant drifted: %d (expected 3 for K=5)", stabilityMajority)
	}
}

func labelFor(n int) string {
	return string(rune('0'+n)) + "/5"
}

func readPublishedGenome(t *testing.T, dir string, rank int) *genome.Genome {
	t.Helper()
	gamesDir := filepath.Join(dir, "games")
	entries, err := os.ReadDir(gamesDir)
	if err != nil {
		t.Fatalf("reading games dir: %v", err)
	}
	prefix := "rank0" + string(rune('0'+rank))
	for _, e := range entries {
		if len(e.Name()) >= len(prefix) && e.Name()[:len(prefix)] == prefix {
			data, err := os.ReadFile(filepath.Join(gamesDir, e.Name(), "genome.json"))
			if err != nil {
				t.Fatalf("reading genome.json: %v", err)
			}
			var g genome.Genome
			if err := json.Unmarshal(data, &g); err != nil {
				t.Fatalf("unmarshal genome.json: %v", err)
			}
			return &g
		}
	}
	t.Fatalf("no game dir for rank %d in %v", rank, entries)
	return nil
}
