package fitness

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

func TestSessionLengthScoring(t *testing.T) {
	tests := []struct {
		avg      float64
		expected float64
	}{
		{3, 0},    // Too short
		{5, 0},    // Minimum
		{10, 0.5}, // Ramping up
		{15, 1.0}, // Target start
		{25, 1.0}, // In range
		{40, 1.0}, // Target end
		{70, 0.5}, // Ramping down
		{100, 0},  // Maximum
		{150, 0},  // Way too long
	}

	for _, tt := range tests {
		result := sim.BatchResult{AvgTurns: tt.avg}
		score := computeSessionLength(result)
		if diff := score - tt.expected; diff > 0.01 || diff < -0.01 {
			t.Errorf("avg=%.0f: expected %.2f, got %.2f", tt.avg, tt.expected, score)
		}
	}
}

func TestFullEvaluation(t *testing.T) {
	allSeeds := []*genome.Genome{
		seeds.CrazyEights(),
		seeds.Whist(),
		seeds.Hearts(),
		seeds.GinRummy(),
	}

	for _, g := range allSeeds {
		result := Evaluate(g, 0)

		if !result.Valid {
			t.Errorf("%s: evaluation invalid. Tier0: %v, Tier1: %s",
				g.ID, result.Tier0Errors, result.Tier1.Reason)
			continue
		}

		m := result.Metrics
		t.Logf("%s: fitness=%.3f decisions=%.3f arc=%.3f interact=%.3f skill=%.3f length=%.3f",
			g.ID, m.TotalFitness, m.MeaningfulDecisions, m.GameArc,
			m.Interaction, m.SkillGradient, m.SessionLength)

		if m.TotalFitness <= 0 {
			t.Errorf("%s: total fitness should be > 0, got %.3f", g.ID, m.TotalFitness)
		}
		if m.TotalFitness > 1.0 {
			t.Errorf("%s: total fitness should be <= 1.0, got %.3f", g.ID, m.TotalFitness)
		}
	}
}

func TestInvalidGenomeGetsZeroFitness(t *testing.T) {
	g := &genome.Genome{
		ID:       "broken",
		Skeleton: genome.Shedding,
		Players:  10, // Invalid
		HandSize: 7,
	}

	result := Evaluate(g, 0)
	if result.Valid {
		t.Fatal("broken genome should not be valid")
	}
	if result.Metrics.TotalFitness != 0 {
		t.Fatalf("broken genome should have 0 fitness, got %.3f", result.Metrics.TotalFitness)
	}
}

// TestGameArcTurnScoreIsScaleInvariant verifies cards-6n3: two batches with
// the same relative turn-count spread should produce similar GameArc
// turn-score contributions, regardless of absolute game length. The previous
// "/100" normalization meant a long-running genome saturated the term on
// trivial absolute spread while a short genome with the same proportional
// spread got near-zero credit.
func TestGameArcTurnScoreIsScaleInvariant(t *testing.T) {
	// Same coefficient of variation (~0.2), different magnitudes.
	shortTurns := []int{10, 12, 12, 14, 10, 14}            // mean=12, stddev≈1.8
	longTurns := []int{70, 80, 80, 90, 70, 90}             // mean=80, stddev≈8.6 (same CV)
	evenWinShort := []int{1, 1}                            // 2 players, even wins
	evenWinLong := []int{1, 1}

	shortBatch := sim.BatchResult{
		Completions: len(shortTurns),
		WinCounts:   evenWinShort,
		TurnsList:   shortTurns,
		AvgTurns:    average(shortTurns),
	}
	longBatch := sim.BatchResult{
		Completions: len(longTurns),
		WinCounts:   evenWinLong,
		TurnsList:   longTurns,
		AvgTurns:    average(longTurns),
	}

	shortArc := computeGameArc(shortBatch)
	longArc := computeGameArc(longBatch)

	// With variance-only normalization (the bug), longArc swamps shortArc
	// because variance scales with mean^2. After the fix the two batches
	// should score within ~10% of each other.
	diff := longArc - shortArc
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.1 {
		t.Fatalf("GameArc turn-score must be scale-invariant: short=%.3f long=%.3f diff=%.3f (want diff<=0.1)",
			shortArc, longArc, diff)
	}
}

func average(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return float64(sum) / float64(len(xs))
}

func TestSkillGradient(t *testing.T) {
	// Greedy should beat random in trick-taking (play to win tricks).
	// Player 0 uses greedy; the other seats stay random.
	g := seeds.Whist()
	runner := GetRunner(g)
	randomAI := &sim.RandomAI{}

	randomResult := sim.RunBatch(g, runner, randomAI, 100, 0)
	greedyResult := runGreedyBatch(g, runner, 100, 500)

	skill := computeSkillGradient(randomResult, greedyResult, g.Players)
	t.Logf("Whist skill gradient: %.3f (random wins=%v, greedy wins=%v)",
		skill, randomResult.WinCounts, greedyResult.WinCounts)

	// With only P0 greedy vs random opponents, P0 should win noticeably
	// more than the 1/N baseline. If this drops to zero the mix is broken.
	if skill <= 0 {
		t.Errorf("expected positive skill gradient for Whist greedy vs random, got %.3f", skill)
	}
}
