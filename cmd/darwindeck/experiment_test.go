package main

// Tests for the experiment harness's aggregation math and statistics (audit
// Task 27). These functions back published config comparisons; every fixture
// below is hand-computed (Mann-Whitney p-values additionally cross-checked
// against scipy.stats.mannwhitneyu(method='asymptotic'), which uses the same
// normal approximation with tie + continuity correction).

import (
	"math"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/evolution"
	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
)

func approxEq(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestMedian(t *testing.T) {
	cases := []struct {
		name string
		in   []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5}, 5},
		{"odd unsorted", []float64{3, 1, 2}, 2},
		// Even n: standard convention, mean of the two middle values.
		{"even", []float64{1, 2, 3, 4}, 2.5},
		{"two", []float64{2, 1}, 1.5},
		{"even unsorted", []float64{4, 1, 3, 2, 6, 5}, 3.5},
	}
	for _, c := range cases {
		if got := median(c.in); !approxEq(got, c.want, 1e-12) {
			t.Errorf("median(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestMedianDoesNotMutateInput(t *testing.T) {
	in := []float64{3, 1, 2}
	median(in)
	if in[0] != 3 || in[1] != 1 || in[2] != 2 {
		t.Errorf("median mutated its input: %v", in)
	}
}

// iqr uses the documented index-based convention: Q1 = sorted[n/4],
// Q3 = sorted[3n/4]. At the harness default n=15 this gives the 4th and 12th
// order statistics -- the Tukey quartiles for n=15.
func TestIQR(t *testing.T) {
	cases := []struct {
		name string
		in   []float64
		want float64
	}{
		{"too small", []float64{1, 2, 3}, 0},
		// n=4: Q1=sorted[1]=2, Q3=sorted[3]=4
		{"n4", []float64{4, 2, 1, 3}, 2},
		// n=8: Q1=sorted[2]=3, Q3=sorted[6]=7
		{"n8", []float64{8, 1, 7, 2, 6, 3, 5, 4}, 4},
		// n=15 (harness default): Q1=sorted[3]=4, Q3=sorted[11]=12
		{"n15", []float64{15, 1, 14, 2, 13, 3, 12, 4, 11, 5, 10, 6, 9, 7, 8}, 8},
	}
	for _, c := range cases {
		if got := iqr(c.in); !approxEq(got, c.want, 1e-12) {
			t.Errorf("iqr(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"baseline,map-elites,novelty,random", []string{"baseline", "map-elites", "novelty", "random"}},
		{"baseline", []string{"baseline"}},
		{"", nil},
		{"a,,b", []string{"a", "b"}},
		{",a,", []string{"a"}},
	}
	for _, c := range cases {
		got := splitCSV(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitCSV(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// TestComputeMetricsFixture hand-computes every aggregate for a 4-individual
// fixture spanning two skeletons and three grid cells (GridSize=10):
//
//	A shedding fit=0.5 behavior=(0.05,0.05) -> cell row0,col0
//	B shedding fit=0.7 behavior=(0.07,0.04) -> cell row0,col0 (beats A)
//	C shedding fit=0.6 behavior=(0.15,0.05) -> cell row0,col1
//	D trick   fit=0.8 behavior=(0.55,0.95) -> cell row9,col5
//
// Coverage: 3 occupied / 300 total cells = 0.01.
// QD-score: best per cell = 0.7 + 0.6 + 0.8 = 2.1.
// Median fitness over [0.5,0.6,0.7,0.8] = 0.65.
// Pairwise distance: mean of the 6 Euclidean distances
//
//	d(A,B)=sqrt(0.0005), d(A,C)=0.1, d(A,D)=sqrt(1.06),
//	d(B,C)=sqrt(0.0065), d(B,D)=sqrt(1.0585), d(C,D)=sqrt(0.97)
//	mean = 0.5410443906...
func TestComputeMetricsFixture(t *testing.T) {
	mk := func(skel genome.SkeletonType, fit float64) *evolution.Individual {
		return &evolution.Individual{
			Genome:  &genome.Genome{Skeleton: skel},
			Fitness: fitness.Metrics{TotalFitness: fit},
			Valid:   true,
		}
	}
	individuals := []*evolution.Individual{
		mk(genome.Shedding, 0.5),
		mk(genome.Shedding, 0.7),
		mk(genome.Shedding, 0.6),
		mk(genome.TrickTaking, 0.8),
	}
	behaviors := []evolution.BehaviorDescriptor{
		{0.05, 0.05},
		{0.07, 0.04},
		{0.15, 0.05},
		{0.55, 0.95},
	}

	r := computeMetrics(individuals, behaviors)

	if r.NumGames != 4 {
		t.Errorf("NumGames = %d, want 4", r.NumGames)
	}
	if !approxEq(r.Coverage, 3.0/300.0, 1e-12) {
		t.Errorf("Coverage = %v, want %v", r.Coverage, 3.0/300.0)
	}
	if !approxEq(r.QDScore, 2.1, 1e-9) {
		t.Errorf("QDScore = %v, want 2.1", r.QDScore)
	}
	if !approxEq(r.MedianFit, 0.65, 1e-9) {
		t.Errorf("MedianFit = %v, want 0.65", r.MedianFit)
	}
	if !approxEq(r.PairwiseDist, 0.5410443906, 1e-9) {
		t.Errorf("PairwiseDist = %v, want 0.5410443906", r.PairwiseDist)
	}

	shed := r.PerSkeleton["shedding"]
	if !approxEq(shed.Coverage, 0.02, 1e-12) || !approxEq(shed.QDScore, 1.3, 1e-9) || shed.NumGames != 3 {
		t.Errorf("shedding = %+v, want coverage=0.02 qd=1.3 games=3", shed)
	}
	trick := r.PerSkeleton["trick_taking"]
	if !approxEq(trick.Coverage, 0.01, 1e-12) || !approxEq(trick.QDScore, 0.8, 1e-9) || trick.NumGames != 1 {
		t.Errorf("trick_taking = %+v, want coverage=0.01 qd=0.8 games=1", trick)
	}
	rummy := r.PerSkeleton["rummy"]
	if rummy.Coverage != 0 || rummy.QDScore != 0 || rummy.NumGames != 0 {
		t.Errorf("rummy = %+v, want all zero", rummy)
	}
}

func TestComputeMetricsEmpty(t *testing.T) {
	r := computeMetrics(nil, nil)
	if r.NumGames != 0 || r.Coverage != 0 || r.QDScore != 0 || r.MedianFit != 0 || r.PairwiseDist != 0 {
		t.Errorf("computeMetrics(nil, nil) = %+v, want zero values", r)
	}
}

// Mann-Whitney U fixtures. All hand-computed; p-values cross-checked against
// scipy.stats.mannwhitneyu(alternative='two-sided', method='asymptotic'),
// which applies the identical tie + continuity correction.

// x=[1,2,3], y=[4,5,6]: no ties. R1=6, U1 = 6 - 3*4/2 = 0, mu = 4.5,
// sigma = sqrt(9*7/12) = 2.2912878..., z_cc = (4.5-0.5)/sigma = 1.7457431...,
// p = erfc(z/sqrt 2) = 0.0808556 (scipy: 0.08085559837).
// Rank-biserial r = 2*0/9 - 1 = -1 (x stochastically smaller).
func TestMannWhitneyUNoTies(t *testing.T) {
	r := mannWhitneyU([]float64{1, 2, 3}, []float64{4, 5, 6})
	if r.N1 != 3 || r.N2 != 3 {
		t.Fatalf("n = %d,%d, want 3,3", r.N1, r.N2)
	}
	if r.U != 0 {
		t.Errorf("U = %v, want 0", r.U)
	}
	if !approxEq(r.Z, -1.7457431219, 1e-9) {
		t.Errorf("Z = %v, want -1.7457431219", r.Z)
	}
	if !approxEq(r.P, 0.0808555984, 1e-9) {
		t.Errorf("P = %v, want 0.0808555984", r.P)
	}
	if !approxEq(r.RankBiserial, -1, 1e-12) {
		t.Errorf("RankBiserial = %v, want -1", r.RankBiserial)
	}
	if r.Degenerate {
		t.Error("Degenerate = true, want false")
	}
}

// x=[1,2,2,5], y=[2,3,4,5]: ties. Pooled sorted = 1,2,2,2,3,4,5,5 with the
// three 2s at ranks (2,3,4) -> 3 each and the two 5s at (7,8) -> 7.5 each.
// R1 = 1 + 3 + 3 + 7.5 = 14.5, U1 = 14.5 - 10 = 4.5, mu = 8.
// Tie groups t=3 and t=2: sum(t^3 - t) = 24 + 6 = 30.
// sigma^2 = (16/12) * (9 - 30/56) = 11.2857142857...,
// z_cc = (|4.5-8| - 0.5)/sigma = 3/3.3594217... = 0.8930108...,
// p = 0.3718514 (scipy: 0.37185136942).
// Rank-biserial r = 2*4.5/16 - 1 = -0.4375.
func TestMannWhitneyUTies(t *testing.T) {
	r := mannWhitneyU([]float64{1, 2, 2, 5}, []float64{2, 3, 4, 5})
	if r.U != 4.5 {
		t.Errorf("U = %v, want 4.5", r.U)
	}
	if !approxEq(r.Z, -0.8930108367, 1e-9) {
		t.Errorf("Z = %v, want -0.8930108367", r.Z)
	}
	if !approxEq(r.P, 0.3718513694, 1e-9) {
		t.Errorf("P = %v, want 0.3718513694", r.P)
	}
	if !approxEq(r.RankBiserial, -0.4375, 1e-12) {
		t.Errorf("RankBiserial = %v, want -0.4375", r.RankBiserial)
	}
}

// x=1..8, y=9..16: n=8 per side (the documented validity threshold for the
// normal approximation). U1=0, mu=32, sigma=sqrt(64*17/12)=9.5219045...,
// z_cc = 31.5/sigma = 3.3081617..., p = 0.00093911 (scipy: 0.00093910570).
func TestMannWhitneyUSeparatedN8(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	y := []float64{9, 10, 11, 12, 13, 14, 15, 16}
	r := mannWhitneyU(x, y)
	if r.N1 != 8 || r.N2 != 8 {
		t.Fatalf("n = %d,%d, want 8,8", r.N1, r.N2)
	}
	if r.U != 0 {
		t.Errorf("U = %v, want 0", r.U)
	}
	if !approxEq(r.Z, -3.3081616985, 1e-9) {
		t.Errorf("Z = %v, want -3.3081616985", r.Z)
	}
	if !approxEq(r.P, 0.0009391057, 1e-9) {
		t.Errorf("P = %v, want 0.0009391057", r.P)
	}
}

// All pooled values tied: sigma^2 = 0, no ordering information. The test must
// report p=1 rather than dividing by zero.
func TestMannWhitneyUDegenerate(t *testing.T) {
	r := mannWhitneyU([]float64{5, 5, 5, 5}, []float64{5, 5, 5, 5})
	if !r.Degenerate {
		t.Error("Degenerate = false, want true")
	}
	if r.P != 1 || r.Z != 0 {
		t.Errorf("P=%v Z=%v, want P=1 Z=0", r.P, r.Z)
	}
	// U1 = R1 - n1(n1+1)/2 = 4*4.5 - 10 = 8 = mu: no shift either way.
	if r.U != 8 {
		t.Errorf("U = %v, want 8", r.U)
	}
	if r.RankBiserial != 0 {
		t.Errorf("RankBiserial = %v, want 0", r.RankBiserial)
	}
}

// Identical distributions: x=[1,2], y=[1,2]. Ranks 1.5,1.5,3.5,3.5;
// R1 = 5, U1 = 2 = mu. Continuity correction clamps |0|-0.5 at zero, so
// z=0 and p=1 exactly.
func TestMannWhitneyUIdenticalSamples(t *testing.T) {
	r := mannWhitneyU([]float64{1, 2}, []float64{1, 2})
	if r.U != 2 {
		t.Errorf("U = %v, want 2", r.U)
	}
	if r.Z != 0 || r.P != 1 {
		t.Errorf("Z=%v P=%v, want Z=0 P=1", r.Z, r.P)
	}
	if r.Degenerate {
		t.Error("Degenerate = true, want false (sigma > 0 here)")
	}
}

func TestMannWhitneyUEmptySide(t *testing.T) {
	for _, pair := range [][2][]float64{
		{nil, {1, 2, 3}},
		{{1, 2, 3}, nil},
		{nil, nil},
	} {
		r := mannWhitneyU(pair[0], pair[1])
		if !r.Degenerate || r.P != 1 {
			t.Errorf("mannWhitneyU(%v, %v) = %+v, want degenerate with P=1", pair[0], pair[1], r)
		}
	}
}

// U1 + U2 = n1*n2, and the two-sided p must not depend on argument order.
func TestMannWhitneyUSymmetry(t *testing.T) {
	x := []float64{0.31, 0.40, 0.40, 0.55, 0.62, 0.62, 0.70, 0.81}
	y := []float64{0.28, 0.40, 0.47, 0.55, 0.55, 0.66, 0.74, 0.90, 0.93}
	a := mannWhitneyU(x, y)
	b := mannWhitneyU(y, x)
	if !approxEq(a.U+b.U, float64(len(x)*len(y)), 1e-9) {
		t.Errorf("U1 + U2 = %v, want %d", a.U+b.U, len(x)*len(y))
	}
	if !approxEq(a.P, b.P, 1e-12) {
		t.Errorf("two-sided p depends on order: %v vs %v", a.P, b.P)
	}
	if !approxEq(a.Z, -b.Z, 1e-12) {
		t.Errorf("z not antisymmetric: %v vs %v", a.Z, b.Z)
	}
	if !approxEq(a.RankBiserial, -b.RankBiserial, 1e-12) {
		t.Errorf("rank-biserial not antisymmetric: %v vs %v", a.RankBiserial, b.RankBiserial)
	}
}

func TestValidateConfigs(t *testing.T) {
	if err := validateConfigs([]string{"baseline", "map-elites", "mapelites", "novelty", "hybrid", "random"}); err != nil {
		t.Errorf("known configs rejected: %v", err)
	}
	if err := validateConfigs([]string{"baseline", "bogus"}); err == nil {
		t.Error("unknown config accepted, want error")
	}
	if err := validateConfigs(nil); err == nil {
		t.Error("empty config list accepted, want error")
	}
}

// TestRunRandomSearchSmall is a small deterministic integration run of the
// null-hypothesis config: 6 mutants x (1+1) waves = 12 evaluations -- the
// same (Generations+1)*PopulationSize budget shape as the other engines.
func TestRunRandomSearchSmall(t *testing.T) {
	config := evolution.Config{
		PopulationSize: 6,
		Generations:    1,
		Workers:        4,
		BaseSeed:       42,
	}
	allSeeds := getAllSeeds()

	inds, behaviors := runRandomSearch(config, allSeeds)
	if len(inds) != len(behaviors) {
		t.Fatalf("len(individuals)=%d != len(behaviors)=%d", len(inds), len(behaviors))
	}
	// Deterministic at BaseSeed 42: at least one mutant of the classic seeds
	// qualifies. If this starts failing after a fitness recalibration, re-pin.
	if len(inds) == 0 {
		t.Fatal("no qualified individuals at the pinned seed; expected >= 1")
	}

	type cell struct {
		skel     genome.SkeletonType
		row, col int
	}
	seen := make(map[cell]bool)
	for i, ind := range inds {
		if !ind.Valid {
			t.Errorf("individual %d not Valid", i)
		}
		if ind.Fitness.TotalFitness < evolution.FitnessFloor {
			t.Errorf("individual %d fitness %v below floor %v", i, ind.Fitness.TotalFitness, evolution.FitnessFloor)
		}
		row, col := behaviors[i].GridCell(evolution.GridSize)
		key := cell{ind.Genome.Skeleton, row, col}
		if seen[key] {
			t.Errorf("cell %+v occupied twice: archive must keep one best occupant per cell", key)
		}
		seen[key] = true
	}

	// Determinism: a second run with the same config reproduces the metrics.
	inds2, behaviors2 := runRandomSearch(config, allSeeds)
	m1 := computeMetrics(inds, behaviors)
	m2 := computeMetrics(inds2, behaviors2)
	if m1.Coverage != m2.Coverage || m1.QDScore != m2.QDScore ||
		m1.NumGames != m2.NumGames || m1.MedianFit != m2.MedianFit ||
		m1.PairwiseDist != m2.PairwiseDist {
		t.Errorf("random search not deterministic:\n  first  %+v\n  second %+v", m1, m2)
	}
}
