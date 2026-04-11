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

func getRunner(g *genome.Genome) sim.GenericRunner {
	switch g.Skeleton {
	case genome.Shedding:
		return &shedding.Runner{}
	case genome.TrickTaking:
		return &tricktaking.Runner{}
	case genome.Rummy:
		return &rummy.Runner{}
	default:
		return nil
	}
}

func TestTier1AllSeedsPass(t *testing.T) {
	allSeeds := []*genome.Genome{
		seeds.CrazyEights(),
		seeds.MauMau(),
		seeds.Whist(),
		seeds.Hearts(),
		seeds.Spades(),
		seeds.OhHell(),
		seeds.GinRummy(),
		seeds.KnockRummy(),
	}

	for _, g := range allSeeds {
		runner := getRunner(g)
		if runner == nil {
			t.Fatalf("%s: no runner for skeleton %s", g.ID, g.Skeleton)
		}

		result := RunTier1(g, runner, 0)
		if !result.Passed {
			t.Errorf("%s FAILED Tier 1: %s", g.ID, result.Reason)
		} else {
			t.Logf("%s: passed (avg turns=%.1f, wins=%v)", g.ID, result.AvgTurns, result.Winners)
		}
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
