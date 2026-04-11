package fitness

import (
	"fmt"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// Tier1Result holds the result of quick validation.
type Tier1Result struct {
	Passed   bool
	Reason   string // Why it failed (if applicable)
	Games    int
	Winners  []int  // Win count per player
	AvgTurns float64
}

// RunTier1 performs quick validation: 5 games with random AI.
// Kills the genome if any game is broken.
func RunTier1(g *genome.Genome, runner sim.GenericRunner, baseSeed uint64) Tier1Result {
	const numGames = 5
	ai := &sim.RandomAI{}

	result := sim.RunBatch(g, runner, ai, numGames, baseSeed)

	// Any game errors = fail
	if result.Errors > 0 {
		return Tier1Result{
			Passed: false,
			Reason: fmt.Sprintf("%d/%d games errored", result.Errors, numGames),
		}
	}

	// Any timeouts = fail
	if result.Timeouts > 0 {
		return Tier1Result{
			Passed: false,
			Reason: fmt.Sprintf("%d/%d games timed out (hit max turns)", result.Timeouts, numGames),
		}
	}

	// All games must complete with a winner
	if result.Completions < numGames {
		return Tier1Result{
			Passed: false,
			Reason: fmt.Sprintf("only %d/%d games completed with a winner", result.Completions, numGames),
		}
	}

	// Same player wins all games = degenerate
	// With only 2 players, one player winning 5/5 has a 3% chance randomly.
	// We only flag this as degenerate for 3+ player games where it's much
	// less likely (0.4% for 3 players, 0.1% for 4).
	if g.Players >= 3 {
		for i, wins := range result.WinCounts {
			if wins == numGames {
				return Tier1Result{
					Passed: false,
					Reason: fmt.Sprintf("player %d won all %d games (degenerate)", i, numGames),
				}
			}
		}
	}

	// Average turns < 3 = game ends instantly
	if result.AvgTurns < 3 {
		return Tier1Result{
			Passed: false,
			Reason: fmt.Sprintf("avg turns %.1f < 3 (game ends too quickly)", result.AvgTurns),
		}
	}

	return Tier1Result{
		Passed:   true,
		Games:    numGames,
		Winners:  result.WinCounts,
		AvgTurns: result.AvgTurns,
	}
}
