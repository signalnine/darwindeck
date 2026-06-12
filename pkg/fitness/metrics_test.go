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

// turnsFixture builds a single-game AllTurns batch with `forced` records of
// 1 legal move and `choice` records of >=2 legal moves.
func turnsFixture(forced, choice int) sim.BatchResult {
	records := make([]sim.TurnRecord, 0, forced+choice)
	for i := 0; i < forced; i++ {
		records = append(records, sim.TurnRecord{Player: i % 2, LegalMoves: 1})
	}
	for i := 0; i < choice; i++ {
		records = append(records, sim.TurnRecord{Player: i % 2, LegalMoves: 3})
	}
	return sim.BatchResult{AllTurns: [][]sim.TurnRecord{records}}
}

func TestDecisionDensityAllForcedIsZero(t *testing.T) {
	got := computeDecisionDensity(turnsFixture(10, 0))
	if got != 0.0 {
		t.Fatalf("all-forced fixture (every turn 1 legal move) must score 0.0, got %.3f", got)
	}
}

func TestDecisionDensityAllChoiceIsOne(t *testing.T) {
	got := computeDecisionDensity(turnsFixture(0, 10))
	if got != 1.0 {
		t.Fatalf("all-choice fixture (every turn >=2 legal moves) must score 1.0, got %.3f", got)
	}
}

func TestDecisionDensityMixed30_70(t *testing.T) {
	// 30% of turns offer a real choice, 70% are forced => density 0.3.
	got := computeDecisionDensity(turnsFixture(7, 3))
	if diff := got - 0.3; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("mixed 30/70 fixture must score 0.3, got %.3f", got)
	}
}

func TestDecisionDensityEmptyBatchIsZero(t *testing.T) {
	if got := computeDecisionDensity(sim.BatchResult{}); got != 0.0 {
		t.Fatalf("empty batch must score 0.0, got %.3f", got)
	}
}

// TestDecisionDensityWhistNotPinned: the old event-taxonomy metric scored every
// trick-taking game exactly 1.0 (every event was a "play"). Real whist forces
// follow-suit on many turns, so legal-move-count density must be strictly
// between 0 and 1.
func TestDecisionDensityWhistNotPinned(t *testing.T) {
	g := seeds.Whist()
	runner := GetRunner(g)
	result := sim.RunBatch(g, runner, &sim.RandomAI{}, 50, 0)

	density := computeDecisionDensity(result)
	t.Logf("Whist decision density: %.3f", density)

	if density <= 0.0 || density >= 1.0 {
		t.Fatalf("whist density must be strictly between 0 and 1 (old structural pin was 1.0), got %.3f", density)
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

func TestSkillGradientUsesEmpiricalBaseline(t *testing.T) {
	// Greedy always plays seat 0. A game with first-player advantage gives seat 0
	// a high random win rate; comparing greedy's seat-0 rate against the theoretical
	// 1/N rather than the *measured* seat-0 random rate miscredits that structural
	// edge as skill. The baseline must come from randomResult, not 1/numPlayers.
	mk := func(seat0Wins int) sim.BatchResult {
		return sim.BatchResult{Completions: 100, WinCounts: []int{seat0Wins, 100 - seat0Wins}}
	}

	// Zero true skill (greedy == random seat-0 rate) must score 0, regardless of
	// how strong the first-player advantage is.
	if got := computeSkillGradient(mk(60), mk(60), 2); got != 0 {
		t.Errorf("FPA game with greedy==random seat-0 rate (60%%) must score 0, got %.3f", got)
	}
	if got := computeSkillGradient(mk(50), mk(50), 2); got != 0 {
		t.Errorf("fair game with greedy==random seat-0 rate (50%%) must score 0, got %.3f", got)
	}

	// A genuine skill edge over the empirical baseline still scores positive.
	if got := computeSkillGradient(mk(50), mk(60), 2); got <= 0 {
		t.Errorf("greedy seat-0 rate (60%%) above random baseline (50%%) must score >0, got %.3f", got)
	}

	// Greedy worse than the empirical baseline clamps to 0.
	if got := computeSkillGradient(mk(60), mk(40), 2); got != 0 {
		t.Errorf("greedy seat-0 rate (40%%) below random baseline (60%%) must score 0, got %.3f", got)
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
