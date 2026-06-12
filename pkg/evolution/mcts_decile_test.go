package evolution

import (
	"math"
	"testing"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
)

// reducedMCTS keeps decile-pass tests fast. Production strength is the
// MCTSEvalConfig zero value (200 iterations / 10 determinizations); these
// knobs match the calibration suite's reduced settings.
var reducedMCTS = fitness.MCTSEvalConfig{Iterations: 50, Determinizations: 5}

// TestTopDecileMCTSPublishesGapGenome (audit Task 20b): the decile pass must
// grant the MCTS term to exactly the top-decile individual (by greedy-only
// running mean) and publish its fitness as the MCTS running mean, while
// below-decile individuals keep their greedy-only published fitness.
//
// Whist at seed 44 is the gap genome: ISMCTS outplays the greedy scorer there
// (measured: skill 0.065 greedy-only vs 0.206 two-tier at these knobs), so
// EvaluateWithMCTS produces a visibly different TotalFitness than the
// greedy-only Evaluate at the same seed. (ROUND 3: the previous gap genome,
// InstantKnockRummy at seed 44, is now killed by the greedy_timeout veto
// before the MCTS tier -- see TestMCTSTierRewardsDegenKnockTiming -- so a
// VALID classic carries this plumbing test instead.) BaseSeed is chosen so
// the decile pass's derived seed for population index 0 lands exactly on 44
// (uint64 wrap-around is well-defined).
func TestTopDecileMCTSPublishesGapGenome(t *testing.T) {
	// Runtime subtraction so the uint64 wrap is legal (a constant expression
	// would be rejected at compile time).
	var targetSeed, offset uint64 = 44, mctsSeedOffset
	cfg := Config{
		PopulationSize: 3,
		EliteSize:      1,
		TournamentSize: 1,
		Workers:        1,
		BaseSeed:       targetSeed - offset, // gen 0, idx 0 => seed 44
		MCTSDecile:     0.1,                 // ceil(0.1*3) = 1: top individual only
		MCTSEval:       reducedMCTS,
	}
	e := NewEngine(cfg, allSeeds())

	gap := &Individual{Genome: seeds.Whist(), Valid: true, EvalCount: 1, FitnessSum: 0.9}
	gap.Fitness.TotalFitness = 0.9
	mid := &Individual{Genome: seeds.CrazyEights(), Valid: true, EvalCount: 2, FitnessSum: 1.0}
	mid.Fitness.TotalFitness = 0.5
	low := &Individual{Genome: seeds.MauMau(), Valid: true, EvalCount: 1, FitnessSum: 0.4}
	low.Fitness.TotalFitness = 0.4
	e.Population = []*Individual{gap, mid, low}
	e.Generation = 0

	e.runMCTSTopDecile()

	full := fitness.EvaluateWithMCTS(seeds.Whist(), 44, reducedMCTS)
	if !full.Valid {
		t.Fatal("reference EvaluateWithMCTS(whist, 44) must pass tiers 0-2")
	}
	base := fitness.Evaluate(seeds.Whist(), 44)
	if !base.Valid {
		t.Fatal("reference Evaluate(whist, 44) must pass tiers 0-2")
	}
	// Precondition for "includes the MCTS term": the two-tier eval differs
	// from the greedy-only eval at the same seed via the skill gradient.
	if full.Metrics.SkillGradient <= base.Metrics.SkillGradient {
		t.Fatalf("fixture lost its MCTS-vs-greedy gap: two-tier skill %.3f <= greedy-only %.3f",
			full.Metrics.SkillGradient, base.Metrics.SkillGradient)
	}

	if gap.MctsCount != 1 {
		t.Fatalf("top-decile individual MctsCount = %d, want 1", gap.MctsCount)
	}
	if math.Abs(gap.MctsSum-full.Metrics.TotalFitness) > 1e-12 {
		t.Errorf("MctsSum = %.6f, want the EvaluateWithMCTS result %.6f", gap.MctsSum, full.Metrics.TotalFitness)
	}
	if math.Abs(gap.Fitness.TotalFitness-full.Metrics.TotalFitness) > 1e-12 {
		t.Errorf("published TotalFitness = %.6f, want MCTS mean %.6f (must include the MCTS term)",
			gap.Fitness.TotalFitness, full.Metrics.TotalFitness)
	}
	if gap.Fitness.TotalFitness == 0.9 {
		t.Error("published fitness is still the greedy mean; the MCTS term never entered")
	}

	// Running-mean purity: the greedy-only accumulator is the decile RANKING
	// key and must never absorb MCTS-mode samples (mode mixing).
	if gap.EvalCount != 1 || gap.FitnessSum != 0.9 {
		t.Errorf("greedy accumulator corrupted by MCTS eval: EvalCount=%d FitnessSum=%.3f, want 1/0.900",
			gap.EvalCount, gap.FitnessSum)
	}

	// Below-decile individuals: untouched, published fitness stays greedy.
	for _, tc := range []struct {
		name string
		ind  *Individual
		fit  float64
	}{{"mid", mid, 0.5}, {"low", low, 0.4}} {
		if tc.ind.MctsCount != 0 || tc.ind.MctsSum != 0 {
			t.Errorf("%s: below-decile individual got an MCTS eval: count=%d sum=%.3f",
				tc.name, tc.ind.MctsCount, tc.ind.MctsSum)
		}
		if tc.ind.Fitness.TotalFitness != tc.fit {
			t.Errorf("%s: published fitness changed to %.3f, want greedy mean %.3f",
				tc.name, tc.ind.Fitness.TotalFitness, tc.fit)
		}
	}
}

// TestTopDecileMCTSZeroDecileDisables: MCTSDecile == 0 is the off switch --
// no individual receives an MCTS evaluation and published fitness stays the
// greedy-only mean (zero-value Config and pre-Task-20b behavior).
func TestTopDecileMCTSZeroDecileDisables(t *testing.T) {
	e := NewEngine(Config{PopulationSize: 2, Workers: 1, BaseSeed: 1}, allSeeds())
	a := &Individual{Genome: seeds.CrazyEights(), Valid: true, EvalCount: 1, FitnessSum: 0.6}
	a.Fitness.TotalFitness = 0.6
	b := &Individual{Genome: seeds.Whist(), Valid: true, EvalCount: 1, FitnessSum: 0.5}
	b.Fitness.TotalFitness = 0.5
	e.Population = []*Individual{a, b}

	e.runMCTSTopDecile()

	for i, ind := range e.Population {
		if ind.MctsCount != 0 || ind.MctsSum != 0 {
			t.Errorf("individual %d received an MCTS eval with MCTSDecile=0", i)
		}
	}
	if a.Fitness.TotalFitness != 0.6 || b.Fitness.TotalFitness != 0.5 {
		t.Error("published fitness changed with MCTSDecile=0")
	}
}

// TestEvaluatePopulationGrantsMCTSToTopDecileOnly drives the real engine
// evaluation path end-to-end: greedy pass, decile ranking on greedy-only
// running means, one MCTS eval for the top individual, published fitness
// switching to the MCTS mean for it and staying the greedy mean for the
// rest.
func TestEvaluatePopulationGrantsMCTSToTopDecileOnly(t *testing.T) {
	cfg := Config{
		PopulationSize: 3,
		EliteSize:      1,
		TournamentSize: 1,
		Workers:        2,
		BaseSeed:       42,
		MCTSDecile:     0.1, // ceil(0.1*3) = 1
		MCTSEval:       fitness.MCTSEvalConfig{Iterations: 10, Determinizations: 2},
	}
	e := NewEngine(cfg, allSeeds())
	e.Population = []*Individual{
		{Genome: seeds.CrazyEights()},
		{Genome: seeds.MauMau()},
		{Genome: seeds.Whist()},
	}
	e.Generation = 0
	e.EvaluatePopulation()

	var valid []*Individual
	for _, ind := range e.Population {
		if ind.Valid {
			valid = append(valid, ind)
		}
	}
	if len(valid) < 2 {
		t.Skipf("only %d/3 classics passed Tier 1 at gen-0 seeds; cannot exercise ranking", len(valid))
	}

	top := valid[0]
	for _, ind := range valid[1:] {
		if ind.greedyMean() > top.greedyMean() {
			top = ind
		}
	}

	granted := 0
	for _, ind := range valid {
		if ind.MctsCount > 0 {
			granted++
			if ind != top {
				t.Errorf("MCTS granted to %s (greedy mean %.3f), but the top greedy mean is %s (%.3f)",
					ind.Genome.ID, ind.greedyMean(), top.Genome.ID, top.greedyMean())
			}
			if math.Abs(ind.Fitness.TotalFitness-ind.MctsSum/float64(ind.MctsCount)) > 1e-12 {
				t.Errorf("granted individual publishes %.6f, want MCTS mean %.6f",
					ind.Fitness.TotalFitness, ind.MctsSum/float64(ind.MctsCount))
			}
		} else {
			if math.Abs(ind.Fitness.TotalFitness-ind.greedyMean()) > 1e-12 {
				t.Errorf("%s publishes %.6f, want greedy mean %.6f",
					ind.Genome.ID, ind.Fitness.TotalFitness, ind.greedyMean())
			}
		}
	}
	if granted != 1 {
		t.Errorf("MCTS granted to %d individuals, want exactly 1 (ceil(0.1*%d))", granted, len(valid))
	}
}

// TestEvaluatePopulationResetsMctsStateOnInvalid: the MCTS accumulator
// resets wherever EvalCount resets -- here, the invalid-re-evaluation path
// (a flaky genome must re-qualify from scratch in BOTH modes).
func TestEvaluatePopulationResetsMctsStateOnInvalid(t *testing.T) {
	e := NewEngine(Config{PopulationSize: 1, Workers: 1, BaseSeed: 1}, allSeeds())
	bad := &genome.Genome{ID: "broken", Skeleton: genome.Shedding, Players: 10}
	ind := &Individual{Genome: bad, Valid: true, EvalCount: 3, FitnessSum: 1.5, MctsSum: 1.2, MctsCount: 2}
	e.Population = []*Individual{ind}

	e.EvaluatePopulation()

	if ind.Valid {
		t.Fatal("tier-0-invalid genome must come back invalid")
	}
	if ind.EvalCount != 0 || ind.FitnessSum != 0 {
		t.Errorf("greedy accumulator not reset: EvalCount=%d FitnessSum=%.2f", ind.EvalCount, ind.FitnessSum)
	}
	if ind.MctsCount != 0 || ind.MctsSum != 0 {
		t.Errorf("MCTS accumulator not reset: MctsCount=%d MctsSum=%.2f", ind.MctsCount, ind.MctsSum)
	}
}

// TestNoveltyEvaluatePopulationGrantsMCTSToTopDecile: the novelty engine
// gets the identical decile pass (the prompt's mode applies to engine.go AND
// novelty.go; MAP-Elites differs -- it re-evaluates on cell challenge
// instead).
func TestNoveltyEvaluatePopulationGrantsMCTSToTopDecile(t *testing.T) {
	cfg := Config{
		PopulationSize: 3,
		EliteSize:      1,
		TournamentSize: 1,
		Workers:        2,
		BaseSeed:       42,
		MCTSDecile:     0.1,
		MCTSEval:       fitness.MCTSEvalConfig{Iterations: 10, Determinizations: 2},
	}
	e := NewNoveltyEngine(cfg, allSeeds())
	e.Population = []*NoveltyIndividual{
		{Individual: Individual{Genome: seeds.CrazyEights()}},
		{Individual: Individual{Genome: seeds.MauMau()}},
		{Individual: Individual{Genome: seeds.Whist()}},
	}
	e.Generation = 0
	e.evaluatePopulation()

	var valid []*NoveltyIndividual
	for _, ind := range e.Population {
		if ind.Valid {
			valid = append(valid, ind)
		}
	}
	if len(valid) < 2 {
		t.Skipf("only %d/3 classics passed Tier 1 at gen-0 seeds; cannot exercise ranking", len(valid))
	}

	top := valid[0]
	for _, ind := range valid[1:] {
		if ind.greedyMean() > top.greedyMean() {
			top = ind
		}
	}

	granted := 0
	for _, ind := range valid {
		if ind.MctsCount > 0 {
			granted++
			if ind != top {
				t.Errorf("MCTS granted to %s, but top greedy mean is %s", ind.Genome.ID, top.Genome.ID)
			}
			if math.Abs(ind.Fitness.TotalFitness-ind.MctsSum/float64(ind.MctsCount)) > 1e-12 {
				t.Errorf("granted individual publishes %.6f, want MCTS mean %.6f",
					ind.Fitness.TotalFitness, ind.MctsSum/float64(ind.MctsCount))
			}
		} else if math.Abs(ind.Fitness.TotalFitness-ind.greedyMean()) > 1e-12 {
			t.Errorf("%s publishes %.6f, want greedy mean %.6f",
				ind.Genome.ID, ind.Fitness.TotalFitness, ind.greedyMean())
		}
	}
	if granted != 1 {
		t.Errorf("MCTS granted to %d individuals, want exactly 1", granted)
	}
}

// TestDefaultConfigMCTSDecile pins the production default: MCTS-for-top-
// decile is ON at 0.10 (the plan's fallback mode after Task 19's 2s budget
// failed); 0 must remain expressible as the off switch.
func TestDefaultConfigMCTSDecile(t *testing.T) {
	if got := DefaultConfig().MCTSDecile; got != 0.10 {
		t.Errorf("DefaultConfig().MCTSDecile = %v, want 0.10", got)
	}
}
