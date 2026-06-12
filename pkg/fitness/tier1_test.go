package fitness

import (
	"testing"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
	"github.com/darwindeck/darwindeck/pkg/skeleton/rummy"
	"github.com/darwindeck/darwindeck/pkg/skeleton/shedding"
	"github.com/darwindeck/darwindeck/pkg/skeleton/tricktaking"
)

// tier1Trials is the number of independent Tier 1 runs (distinct baseSeeds)
// used by the acceptance/rejection tests below. Base seeds are spaced 1000
// apart so the per-game seed ranges (baseSeed..baseSeed+numGames-1) never
// overlap between trials.
const tier1Trials = 30

func tier1TrialSeed(trial int) uint64 {
	return uint64(trial) * 1000
}

// TestTier1AcceptsClassics: every classic seed game must pass Tier 1 on at
// least 28 of 30 base seeds. This is the false-reject budget for the gate
// (audit Task 16): the old 5-game kill-on-single-timeout gate rejected
// healthy rummy seeds 13-20% of the time.
func TestTier1AcceptsClassics(t *testing.T) {
	for _, g := range seeds.All() {
		runner := GetRunner(g)
		if runner == nil {
			t.Fatalf("%s: no runner for skeleton %s", g.ID, g.Skeleton)
		}

		passes := 0
		for trial := 0; trial < tier1Trials; trial++ {
			result := RunTier1(g, runner, tier1TrialSeed(trial))
			if result.Passed {
				passes++
			} else {
				t.Logf("%s trial %d failed: %s", g.ID, trial, result.Reason)
			}
		}

		if passes < 28 {
			t.Errorf("%s: passed only %d/%d Tier 1 trials (need >= 28)", g.ID, passes, tier1Trials)
		} else {
			t.Logf("%s: passed %d/%d trials", g.ID, passes, tier1Trials)
		}
	}
}

// TestTier1KillsInstantKnock: the degenerate instant-knock fixture (the
// rank01_gen200_70015 flagship reproduction) must fail Tier 1 on a majority
// of base seeds. Tier 1's job is "kill if degenerate"; a gate that waves this
// through on most seeds is not doing it.
func TestTier1KillsInstantKnock(t *testing.T) {
	g := seeds.InstantKnockRummy()
	runner := GetRunner(g)
	if runner == nil {
		t.Fatalf("no runner for skeleton %s", g.Skeleton)
	}

	fails := 0
	for trial := 0; trial < tier1Trials; trial++ {
		result := RunTier1(g, runner, tier1TrialSeed(trial))
		if !result.Passed {
			fails++
		} else {
			t.Logf("trial %d passed (avg turns=%.1f, wins=%v)", trial, result.AvgTurns, result.Winners)
		}
	}

	if fails <= tier1Trials/2 {
		t.Errorf("instant-knock-rummy failed only %d/%d trials; Tier 1 must kill it on a majority", fails, tier1Trials)
	} else {
		t.Logf("instant-knock-rummy killed on %d/%d trials", fails, tier1Trials)
	}
}

func TestTier1RejectsDegenerate(t *testing.T) {
	// A game with MatchBoth and 1 card hand — ends too quickly
	g := &genome.Genome{
		ID:       "degenerate",
		Skeleton: genome.Shedding,
		Players:  2,
		HandSize: 3,
		Shedding: &genome.SheddingParams{
			MatchRule:   genome.MatchBoth,
			DrawPenalty: 1,
		},
	}

	runner := &shedding.Runner{}
	result := RunTier1(g, runner, 0)

	// This might pass or fail depending on how quickly games end
	// The point is it shouldn't crash
	t.Logf("Degenerate game: passed=%v reason=%q avg_turns=%.1f", result.Passed, result.Reason, result.AvgTurns)
}

func TestBatchRunnerStats(t *testing.T) {
	g := seeds.CrazyEights()
	runner := &shedding.Runner{}
	ai := &sim.RandomAI{}

	result := sim.RunBatch(g, runner, ai, 100, 0)

	if result.GamesPlayed != 100 {
		t.Fatalf("expected 100 games, got %d", result.GamesPlayed)
	}

	if result.Completions < 90 {
		t.Fatalf("too few completions: %d/100", result.Completions)
	}

	if result.Errors > 0 {
		t.Fatalf("unexpected errors: %d", result.Errors)
	}

	if result.AvgTurns < 5 || result.AvgTurns > 200 {
		t.Fatalf("suspicious avg turns: %.1f", result.AvgTurns)
	}

	t.Logf("Crazy Eights batch: completions=%d avg_turns=%.1f min=%d max=%d wins=%v",
		result.Completions, result.AvgTurns, result.MinTurns, result.MaxTurns, result.WinCounts)
}

func TestBatchRunnerTrickTaking(t *testing.T) {
	g := seeds.Hearts()
	runner := &tricktaking.Runner{}
	ai := &sim.RandomAI{}

	result := sim.RunBatch(g, runner, ai, 100, 0)

	if result.Completions < 95 {
		t.Fatalf("Hearts: too few completions %d/100", result.Completions)
	}

	t.Logf("Hearts batch: completions=%d avg_turns=%.1f wins=%v",
		result.Completions, result.AvgTurns, result.WinCounts)
}

func TestBatchRunnerRummy(t *testing.T) {
	g := seeds.GinRummy()
	runner := &rummy.Runner{}
	ai := &sim.RandomAI{}

	result := sim.RunBatch(g, runner, ai, 100, 0)

	if result.Completions < 80 {
		t.Fatalf("Gin Rummy: too few completions %d/100", result.Completions)
	}

	t.Logf("Gin Rummy batch: completions=%d avg_turns=%.1f wins=%v",
		result.Completions, result.AvgTurns, result.WinCounts)
}
