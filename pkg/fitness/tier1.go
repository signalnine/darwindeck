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
	Winners  []int // Win count per player
	AvgTurns float64
}

// Tier 1 fail thresholds (audit Task 16). The old gate (5 games, kill on a
// single timeout) rejected healthy rummy seeds 13-20% of the time: one
// unlucky random-AI deal hitting max turns was enough. 10 games with a
// tolerance band keeps the gate cheap while pricing in that noise.
const (
	tier1MaxTimeouts    = 3 // timeouts >= this = fail
	tier1MinCompletions = 7 // completions < this = fail
	// tier1MinAvgTurns: average turns of COMPLETED games below this =
	// degenerate ("game ends instantly"). Two Task 16 changes, both measured
	// against all 8 classics x 30 base seeds:
	//
	// 1. Basis: completed games only. Timeouts are tolerated separately as
	//    noise (above), but a timeout counts maxTurns (208 for rummy) toward
	//    the all-games average -- one tolerated timeout among ten 2-turn
	//    coin-flip games yields an average of ~22, indistinguishable from
	//    mau-mau (~23). No cutoff on the all-games basis separates the
	//    instant-knock fixture from the classics; on the completed basis
	//    they separate cleanly (fixture max 14.9 vs classic min 22.7).
	// 2. Cutoff: raised 3 -> 5. An instant-knock game averaging 3-6 turns
	//    across 2 players is one or two decisions each before a coin flip --
	//    still degenerate. Kills 23/30 fixture trials; the slowest classic
	//    (mau-mau, completed-avg 22.7) keeps a 4.5x margin.
	tier1MinAvgTurns = 5.0
)

// RunTier1 performs quick validation: tier1Games games with random AI
// (tier1Games lives in calibration.go so GamesPerEvaluation cannot drift
// from the pipeline). Kills the genome if the games are broken or degenerate.
//
// Hooks MUST be the same borrowed-mechanic hooks Tier 2 runs with (build
// them via buildHookFuncs) -- a hook-less Tier 1 validates a different game
// than the one being evolved, which is how the dd-wfi borrow that timed out
// 198/200 Tier 2 games sailed through Tier 1.
func RunTier1(g *genome.Genome, runner sim.GenericRunner, baseSeed uint64, hooks ...sim.HookFunc) Tier1Result {
	ai := &sim.RandomAI{}

	result := sim.RunBatch(g, runner, ai, tier1Games, baseSeed, hooks...)

	// Any game errors = fail (errors are deterministic brokenness, not noise)
	if result.Errors > 0 {
		return Tier1Result{
			Passed: false,
			Reason: fmt.Sprintf("%d/%d games errored", result.Errors, tier1Games),
		}
	}

	// A couple of timeouts is random-AI noise (healthy rummy games hit max
	// turns occasionally); three or more is a pattern.
	if result.Timeouts >= tier1MaxTimeouts {
		return Tier1Result{
			Passed: false,
			Reason: fmt.Sprintf("%d/%d games timed out (hit max turns)", result.Timeouts, tier1Games),
		}
	}

	// Most games must complete with a winner.
	if result.Completions < tier1MinCompletions {
		return Tier1Result{
			Passed: false,
			Reason: fmt.Sprintf("only %d/%d games completed with a winner (need %d)", result.Completions, tier1Games, tier1MinCompletions),
		}
	}

	// Same player wins every completed game = degenerate.
	// Under random play among k players, the chance that any player sweeps
	// c completed games is k*(1/k)^c = (1/k)^(c-1). At the minimum passing
	// completion count (c=7) that is ~1.6% for 2 players -- too likely to
	// flag -- but <= 0.14% for 3 players and <= 0.02% for 4 (at c=10:
	// ~0.005% and ~0.0004%), so the sweep check applies to 3+ player games
	// only.
	if g.Players >= 3 {
		for i, wins := range result.WinCounts {
			if wins == result.Completions {
				return Tier1Result{
					Passed: false,
					Reason: fmt.Sprintf("player %d won all %d completed games (degenerate)", i, result.Completions),
				}
			}
		}
	}

	// Average turns of completed games below the floor = game ends
	// essentially instantly (see tier1MinAvgTurns for why timeouts are
	// excluded from this average). Completions >= tier1MinCompletions here,
	// so the mean is over at least 7 games.
	completedAvg := completedAvgTurns(result)
	if completedAvg < tier1MinAvgTurns {
		return Tier1Result{
			Passed: false,
			Reason: fmt.Sprintf("completed games avg %.1f turns < %.0f (game ends too quickly)", completedAvg, tier1MinAvgTurns),
		}
	}

	return Tier1Result{
		Passed:   true,
		Games:    tier1Games,
		Winners:  result.WinCounts,
		AvgTurns: result.AvgTurns,
	}
}

// completedAvgTurns returns the mean turn count over games that ended with a
// winner (TurnsList and AllWinners are parallel), or 0 if none completed.
func completedAvgTurns(result sim.BatchResult) float64 {
	sum, n := 0, 0
	for i, w := range result.AllWinners {
		if w >= 0 {
			sum += result.TurnsList[i]
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}
