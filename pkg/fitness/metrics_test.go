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
