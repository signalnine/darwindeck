// Canonical calibration constants (audit remediation Tasks 2 + 13.5).
//
// This file is deliberately NOT behind the `calibration` build tag: the
// pinned seed list is consumed by both the gated calibration suite
// (calibration_test.go) and the `calibrate` CLI subcommand
// (cmd/darwindeck/calibrate.go), so it must be importable by non-test code.
package fitness

// CalibrationSeeds is the canonical pinned seed list for ALL calibration
// evaluations. Every task that measures seed-game fitness uses this list --
// never ad-hoc seeds -- so numbers are comparable across the whole plan
// (plan "Seed discipline" note).
var CalibrationSeeds = []uint64{11, 22, 33, 44, 55, 66, 77, 88, 99, 110}

// tier1Games is the Tier 1 quick-batch size, consumed directly by RunTier1
// (tier1.go) and by GamesPerEvaluation below -- a single constant so the
// throughput accounting can never drift from the pipeline. Raised from 5 to
// 10 in Task 16: at 5 games a kill-on-single-timeout gate rejected healthy
// rummy seeds 13-20% of the time.
const tier1Games = 10

// GamesPerEvaluation returns the number of simulated games one Evaluate call
// plays: the Tier 1 quick batch always runs; Tier 2's random + greedy batches
// run only when Tier 1 passes. The calibrate subcommand uses this for
// throughput accounting so the count cannot drift from the pipeline.
func GamesPerEvaluation(tier1Passed bool) int {
	games := tier1Games
	if tier1Passed {
		games += tier2RandomGames + tier2GreedyGames
	}
	return games
}
